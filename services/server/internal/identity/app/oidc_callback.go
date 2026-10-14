// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log/slog"
	"net/url"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/oidcflow"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// The internal causes of a failed callback, logged with the provider id and the flow id only; the
// app sees one of the three error codes of oidcflow.
const (
	causeProviderError  = "provider_error"
	causeProviderGone   = "provider_unavailable"
	causeMetadata       = "metadata"
	causeIssParameter   = "iss_parameter"
	causeIssMissing     = "iss_parameter_missing"
	causeCodeMissing    = "code_missing"
	causeExchange       = "token_exchange"
	causeClaims         = "id_token_claims"
	causeIssMismatch    = "iss_mismatch"
	causeFlowExpired    = "flow_expired"
	causeUnknownFlow    = "unknown_flow"
	logCallbackCause    = "bothy.identity.oidc.cause"
	logLoginAttemptID   = "bothy.identity.login_attempt_id"
	fieldState          = "/state"
	queryLoginCode      = "login_code"
	queryLoginAttemptID = "login_attempt_id"
	queryError          = "error"
)

// HandleOIDCCallback implements identity.OIDCLogin (brief M1-41 "handleOidcCallback"). The flow
// is read without a lock and the provider is called outside any transaction (02 section 8.5); the
// only write is conditional on the status started, so of two callbacks with one state at most one
// issues a login_code. The callback changes no account (R-DM-30).
func (o *OIDC) HandleOIDCCallback(ctx context.Context, in identity.OIDCCallback) (string, error) {
	if err := o.authorize(ctx, actor.Anonymous(), authorize.ActionAuthLoginBegin, instanceResource); err != nil {
		return "", err
	}
	f, found, err := LoginFlowRecord{}, false, error(nil)
	if in.State != "" {
		if f, found, err = o.d.Flows.LoginFlowByStateHash(ctx, oidcflow.Hash(o.pepper, oidcflow.HashState, in.State)); err != nil {
			return "", err
		}
	}
	if !found || f.Status != oidcflow.StatusStarted || !o.s.d.Clock.Now().Before(f.ExpiresAt) {
		return "", o.unknownFlow(ctx)
	}
	ext, code, cause := o.verifyCallback(ctx, f, in)
	if code == "" {
		redirect, issued, err := o.issueLoginCode(ctx, f, ext)
		if err != nil || issued {
			return redirect, err
		}
		// Another callback with the same state was faster: this one has no trusted answer.
		return "", o.unknownFlow(ctx)
	}
	return o.failCallback(ctx, f, code, cause)
}

// unknownFlow is the answer to a state without a usable flow: 400 without a redirect and a deny
// entry without details (there is no trusted flow to name).
func (o *OIDC) unknownFlow(ctx context.Context) error {
	_ = o.s.d.Audit.RecordDenial(ctx, auditing.Entry{
		Action: string(authorize.ActionAuthLoginBegin), Decision: auditing.DecisionDeny, Reason: auditing.ReasonUnauthenticated,
		ResourceKind: string(authorize.ResourceInstance), Attrs: auditing.Attrs{attrCause: auditing.Code(causeUnknownFlow)},
	})
	return errs.New(errs.CodeValidationFailed, "").
		WithFields(errs.FieldError{Pointer: fieldState, Code: "invalid", Detail: "The login flow is unknown, used or expired."})
}

// verifyCallback runs the checks of steps 3 to 6 and returns the verified identity, or the error
// code for the app and the internal cause.
func (o *OIDC) verifyCallback(ctx context.Context, f LoginFlowRecord, in identity.OIDCCallback) (oidcflow.ExternalIdentity, string, string) {
	if in.Error != "" {
		return oidcflow.ExternalIdentity{}, oidcflow.ErrorAccessDenied, causeProviderError
	}
	p, conf, err := o.provider(ctx, f.ProviderID)
	if err != nil {
		return oidcflow.ExternalIdentity{}, oidcflow.ErrorLoginFailed, causeProviderGone
	}
	meta, err := o.d.Client.Metadata(ctx, p.ID)
	if err != nil {
		return oidcflow.ExternalIdentity{}, oidcflow.ErrorLoginFailed, causeMetadata
	}
	switch {
	case in.HasIss && !oidcflow.IssuerMatches(conf.Issuer, conf.AllowedTenantIDs, in.Iss):
		return oidcflow.ExternalIdentity{}, oidcflow.ErrorLoginFailed, causeIssParameter
	case !in.HasIss && meta.IssParameterSupported:
		return oidcflow.ExternalIdentity{}, oidcflow.ErrorLoginFailed, causeIssMissing
	case in.Code == "":
		return oidcflow.ExternalIdentity{}, oidcflow.ErrorLoginFailed, causeCodeMissing
	}
	claims, err := o.d.Client.ExchangeCode(ctx, p.ID, in.Code, oidcflow.ServerVerifier(o.pepper, f.ID), o.callback)
	if err != nil {
		return oidcflow.ExternalIdentity{}, oidcflow.ErrorLoginFailed, causeExchange
	}
	now := o.s.d.Clock.Now()
	issuer, err := oidcflow.CheckIDToken(claims, oidcflow.Expectation{
		Issuer: conf.Issuer, AllowedTenants: conf.AllowedTenantIDs, ClientID: conf.ClientID, NonceHash: f.NonceHash, Pepper: o.pepper,
		FlowCreatedAt: f.CreatedAt, Now: now, Leeway: o.cfg.ClockLeeway,
	})
	switch {
	case err != nil:
		return oidcflow.ExternalIdentity{}, oidcflow.ErrorLoginFailed, causeClaims
	case in.HasIss && claims.Issuer != in.Iss:
		return oidcflow.ExternalIdentity{}, oidcflow.ErrorLoginFailed, causeIssMismatch
	case !now.Before(f.ExpiresAt):
		return oidcflow.ExternalIdentity{}, oidcflow.ErrorFlowExpired, causeFlowExpired
	}
	return oidcflow.ExternalFromClaims(claims, issuer), "", ""
}

// issueLoginCode stores the verified identity with a new login_code and returns the redirect with
// login_code and login_attempt_id; issued is false when the flow was no longer started.
func (o *OIDC) issueLoginCode(ctx context.Context, f LoginFlowRecord, ext oidcflow.ExternalIdentity) (string, bool, error) {
	code, err := oidcflow.NewLoginCode(o.d.Random)
	if err != nil {
		return "", false, errs.Wrap(errs.CodeInternalError, err, "identity: random source")
	}
	expires := o.s.d.Clock.Now().Add(o.cfg.ServerCodeTTL)
	var issued bool
	err = o.s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		issued, err = o.d.Flows.IssueLoginCode(ctx, f.ID, ext, oidcflow.Hash(o.pepper, oidcflow.HashLoginCode, code), expires)
		return err
	})
	if err != nil || !issued {
		return "", false, err
	}
	redirect, err := withQuery(f.AppRedirectURI, url.Values{queryLoginCode: {code}, queryLoginAttemptID: {f.ID.String()}})
	return redirect, err == nil, err
}

// failCallback ends the flow with code, writes the deny entry {provider_id, login_attempt_id,
// error}, logs the internal cause and returns the redirect with error and login_attempt_id.
func (o *OIDC) failCallback(ctx context.Context, f LoginFlowRecord, code, cause string) (string, error) {
	err := o.s.d.UoW.Do(ctx, func(ctx context.Context) error {
		_, err := o.d.Flows.FailLoginFlow(ctx, f.ID, code)
		return err
	})
	if err != nil {
		return "", err
	}
	o.d.Log.InfoContext(ctx, "identity: an OIDC callback failed", slog.String(logProviderID, f.ProviderID.String()),
		slog.String(logLoginAttemptID, f.ID.String()), slog.String(logCallbackCause, cause))
	_ = o.s.d.Audit.RecordDenial(ctx, auditing.Entry{
		Action: string(authorize.ActionAuthLoginBegin), Decision: auditing.DecisionDeny, Reason: auditing.ReasonUnauthenticated,
		ResourceKind: string(authorize.ResourceInstance),
		Attrs: auditing.Attrs{
			attrProviderID: auditing.ID(f.ProviderID), attrLoginAttemptID: auditing.ID(f.ID), attrError: auditing.Code(code),
		},
	})
	return withQuery(f.AppRedirectURI, url.Values{queryError: {code}, queryLoginAttemptID: {f.ID.String()}})
}

// appRedirect appends params to the app redirect URI, keeping its own query.
func withQuery(base string, params url.Values) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternalError, err, "identity: a stored URL does not parse")
	}
	if u.RawQuery != "" {
		u.RawQuery += "&"
	}
	u.RawQuery += params.Encode()
	return u.String(), nil
}
