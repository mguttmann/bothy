// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/url"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/oidcflow"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/provider"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ProviderMetadata is the checked discovery document of an OIDC provider (08 section 2.2): the
// endpoints the server uses and whether the provider sends the iss response parameter (RFC 9207).
type ProviderMetadata struct {
	Issuer                string
	AuthorizationEndpoint string
	TokenEndpoint         string
	JWKSURI               string
	// IssParameterSupported is authorization_response_iss_parameter_supported; then the callback
	// requires iss.
	IssParameterSupported bool
}

// IDTokenClaims are the claims of an ID token whose signature the provider client verified.
type IDTokenClaims = oidcflow.Claims

// OIDCProviderClient is the outgoing port towards the identity providers (implemented by
// adapters/oidc over platform/egress). Errors never carry a code, a token or a secret.
type OIDCProviderClient interface {
	// Metadata returns the checked discovery document of the provider providerID, cached for
	// auth.oidc_jwks_cache_ttl. Errors: the provider is unknown, unreachable, or its document
	// fails a check (08 section 2.2, brief M1-41 "beginOidcLogin" item 5).
	Metadata(ctx context.Context, providerID ids.ID) (ProviderMetadata, error)
	// ExchangeCode redeems code at the token endpoint with codeVerifier (PKCE S256), redirectURI
	// and the client authentication of the provider, verifies the JWS of the returned ID token
	// with the provider's keys and returns its parsed claims; the provider's access and refresh
	// tokens are dropped unread. The claim rules are the caller's (oidcflow.CheckIDToken).
	ExchangeCode(ctx context.Context, providerID ids.ID, code, codeVerifier, redirectURI string) (IDTokenClaims, error)
}

// OIDCDeps are the collaborators of the OIDC use cases.
type OIDCDeps struct {
	// Logins is the login service: its stores, unit of work, authorizer, audit, DPoP verifier,
	// enrollment codes and CompleteLogin.
	Logins *Service
	// Flows reads and writes auth_login_flows.
	Flows LoginFlowStore
	// Client talks to the providers.
	Client OIDCProviderClient
	// ClaimGroups reconciles the claim groups of authz at the completion of a login whose provider
	// has groups_claim (M1-42).
	ClaimGroups authz.ClaimSync
	// Random is the source of state, nonce, login_code and handle suffixes; nil means
	// crypto/rand.
	Random io.Reader
	// Log receives the warnings of unusable providers and the causes of failed callbacks (codes
	// and IDs only); nil discards.
	Log *slog.Logger
}

// OIDC implements identity.OIDCLogin (brief M1-41, 08 sections 2.2, 3.1 and 3.3). It is safe for
// concurrent use.
type OIDC struct {
	cfg      Config
	pepper   []byte
	callback string
	// consoleLogin and consoleStepUp are the console callbacks on the origin of server.public_url
	// (M6-01).
	consoleLogin, consoleStepUp string
	d                           OIDCDeps
	s                           *Service
}

// Compile-time check: the incoming port.
var _ identity.OIDCLogin = (*OIDC)(nil)

// callbackPath is the one redirect URI the provider knows (08 section 2.3).
const callbackPath = "/api/v1/auth/oidc/callback"

// NewOIDC returns the OIDC use cases with auth.login_flow_ttl, auth.server_code_ttl,
// auth.clock_leeway, auth.group_claim_max_age, server.public_url and the principal limits of cfg
// and the raw token pepper.
// Errors: a missing collaborator, an empty pepper, a lifetime that is not positive, a public URL
// without scheme and host.
func NewOIDC(cfg Config, pepper []byte, d OIDCDeps) (*OIDC, error) {
	if d.Logins == nil || d.Flows == nil || d.Client == nil || d.ClaimGroups == nil {
		return nil, errors.New("identity: NewOIDC needs the login service, the flow store, the provider client and the claim sync")
	}
	if cfg.GroupClaimMaxAge <= 0 {
		return nil, errors.New("identity: OIDC needs auth.group_claim_max_age above 0")
	}
	if len(pepper) == 0 || cfg.LoginFlowTTL <= 0 || cfg.ServerCodeTTL <= 0 || cfg.ClockLeeway < 0 {
		return nil, errors.New("identity: OIDC needs the token pepper, auth.login_flow_ttl and auth.server_code_ttl above 0")
	}
	u, err := url.Parse(cfg.PublicURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("identity: server.public_url must be an absolute URL")
	}
	if d.Random == nil {
		d.Random = rand.Reader
	}
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	return &OIDC{
		cfg: cfg, pepper: append([]byte(nil), pepper...), callback: u.Scheme + "://" + u.Host + callbackPath,
		consoleLogin: u.Scheme + "://" + u.Host + consoleLoginCallback, consoleStepUp: u.Scheme + "://" + u.Host + consoleStepUpCallback,
		d: d, s: d.Logins,
	}, nil
}

// Audit attribute names of the OIDC actions (codes and IDs only, SEC-AUD-5).
const (
	attrLoginAttemptID = "login_attempt_id"
	attrError          = "error"
	attrIdentityID     = "identity_id"
)

// logProviderID is the log attribute of the provider of a warning.
const logProviderID = "bothy.identity.provider_id"

// authorize asks for act on r and returns the refusal as an error.
func (o *OIDC) authorize(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := o.s.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// BeginOIDCLogin implements identity.OIDCLogin (brief M1-41 "beginOidcLogin").
// The console form (M6-01) needs the console login callback as redirect and no enrollment code
// (400 otherwise); its flow skips the provider's app_redirect_uris.
func (o *OIDC) BeginOIDCLogin(ctx context.Context, in identity.OIDCBegin) (identity.OIDCBeginResult, error) {
	if err := o.authorize(ctx, actor.Anonymous(), authorize.ActionAuthLoginBegin, instanceResource); err != nil {
		return identity.OIDCBeginResult{}, err
	}
	var opt beginOpts
	if in.Console {
		if in.AppRedirectURI != o.consoleLogin {
			return identity.OIDCBeginResult{}, consoleRedirectInvalid()
		}
		if in.EnrollmentCode != "" {
			return identity.OIDCBeginResult{}, consoleFormRefused(fieldEnrollmentCode)
		}
		opt.redirect = o.consoleLogin
	}
	return o.begin(ctx, in, oidcflow.ModeLogin, actor.Actor{}, opt)
}

// BeginIdentityLink implements identity.OIDCLogin (brief M1-41 "Link mode").
func (o *OIDC) BeginIdentityLink(ctx context.Context, a actor.Actor, in identity.OIDCBegin) (identity.OIDCBeginResult, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.OIDCBeginResult{}, err
	}
	if err := o.authorize(ctx, a, authorize.ActionIdentityLink, userResource(a.ID)); err != nil {
		return identity.OIDCBeginResult{}, err
	}
	in.EnrollmentCode = ""
	return o.begin(ctx, in, oidcflow.ModeLink, a, beginOpts{})
}

// begin checks the request, loads the provider metadata and inserts the flow (steps 2 to 7 of
// beginOidcLogin). A link flow belongs to linker and records its allow entry.
func (o *OIDC) begin(ctx context.Context, in identity.OIDCBegin, mode oidcflow.Mode, linker actor.Actor, opt beginOpts) (identity.OIDCBeginResult, error) {
	p, conf, err := o.provider(ctx, in.ProviderID)
	if err != nil {
		return identity.OIDCBeginResult{}, err
	}
	redirect, err := flowRedirect(in, conf, opt)
	if err != nil {
		return identity.OIDCBeginResult{}, err
	}
	if err := oidcflow.CheckClientChallenge(in.ClientChallenge); err != nil {
		return identity.OIDCBeginResult{}, err
	}
	var codeID ids.ID
	if in.EnrollmentCode != "" {
		if codeID, err = o.checkEnrollmentCode(ctx, in.EnrollmentCode); err != nil {
			return identity.OIDCBeginResult{}, err
		}
	}
	meta, err := o.d.Client.Metadata(ctx, p.ID)
	if err != nil {
		o.d.Log.WarnContext(ctx, "identity: the metadata of an OIDC provider is unusable", slog.String(logProviderID, p.ID.String()))
		return identity.OIDCBeginResult{}, errs.New(errs.CodeUnavailable, "").WithCause("dependency_unavailable")
	}
	state, err := oidcflow.NewSecret(o.d.Random)
	if err != nil {
		return identity.OIDCBeginResult{}, errs.Wrap(errs.CodeInternalError, err, "identity: random source")
	}
	nonce, err := oidcflow.NewSecret(o.d.Random)
	if err != nil {
		return identity.OIDCBeginResult{}, errs.Wrap(errs.CodeInternalError, err, "identity: random source")
	}
	now := o.s.d.Clock.Now()
	rec := LoginFlowRecord{
		ID: ids.New(), ProviderID: p.ID, Mode: mode, UserID: linker.ID, StateHash: oidcflow.Hash(o.pepper, oidcflow.HashState, state),
		NonceHash: oidcflow.Hash(o.pepper, oidcflow.HashNonce, nonce), ClientChallenge: in.ClientChallenge, AppRedirectURI: redirect,
		EnrollmentCodeID: codeID, Status: oidcflow.StatusStarted, CreatedAt: now, ExpiresAt: now.Add(o.cfg.LoginFlowTTL),
	}
	if err := o.insertFlow(ctx, rec, linker, opt, p.ID); err != nil {
		return identity.OIDCBeginResult{}, err
	}
	params := url.Values{
		"response_type": {"code"}, "client_id": {conf.ClientID}, "redirect_uri": {o.callback}, "scope": {"openid profile email"},
		"state": {state}, "nonce": {nonce}, "code_challenge": {oidcflow.S256(oidcflow.ServerVerifier(o.pepper, rec.ID))},
		"code_challenge_method": {"S256"},
	}
	if !opt.reauth.IsZero() {
		// A new sign-in at the provider, never a silent one (08 section 6.2, D-M6-01-6).
		params.Set("prompt", "login")
		params.Set("max_age", "0")
	}
	authURL, err := withQuery(meta.AuthorizationEndpoint, params)
	if err != nil {
		return identity.OIDCBeginResult{}, err
	}
	return identity.OIDCBeginResult{LoginAttemptID: rec.ID, AuthorizationURL: authURL, ExpiresAt: rec.ExpiresAt}, nil
}

// flowRedirect returns the redirect a flow stores: the console redirect of opt (M6-01), else the
// app redirect of in after the check against the provider's app_redirect_uris.
func flowRedirect(in identity.OIDCBegin, conf provider.OIDCConfig, opt beginOpts) (string, error) {
	if opt.redirect != "" {
		return opt.redirect, nil
	}
	if err := oidcflow.CheckAppRedirectURI(in.AppRedirectURI, conf.AppRedirectURIs); err != nil {
		return "", err
	}
	return in.AppRedirectURI, nil
}

// insertFlow inserts the flow rec and, for a link flow, its allow entry in one unit of work: the
// start of a console re-login records auth.step_up (M6-01), any other link identity.link.
func (o *OIDC) insertFlow(ctx context.Context, rec LoginFlowRecord, linker actor.Actor, opt beginOpts, providerID ids.ID) error {
	return o.s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := o.d.Flows.InsertLoginFlow(ctx, rec); err != nil {
			return err
		}
		switch {
		case rec.Mode != oidcflow.ModeLink:
			return nil
		case !opt.reauth.IsZero():
			return o.s.d.Audit.Record(ctx, reauthEntry(linker.SessionID, opt.reauth, providerID, rec.ID))
		}
		return o.s.d.Audit.Record(ctx, auditing.Entry{
			Action: string(authorize.ActionIdentityLink), Decision: auditing.DecisionAllow, ResourceKind: resourceUser, ResourceID: linker.ID,
			Attrs: auditing.Attrs{attrStep: auditing.Code("begin"), attrProviderID: auditing.ID(providerID), attrLoginAttemptID: auditing.ID(rec.ID)},
		})
	})
}

// provider returns the enabled OIDC provider id with its configuration; a missing, disabled or
// other provider is the same not_found.
func (o *OIDC) provider(ctx context.Context, id ids.ID) (identity.Provider, provider.OIDCConfig, error) {
	p, found, err := o.s.d.Providers.EnabledProvider(ctx, id)
	if err != nil {
		return identity.Provider{}, provider.OIDCConfig{}, err
	}
	if !found || p.Kind != identity.ProviderOIDC {
		return identity.Provider{}, provider.OIDCConfig{}, invisible()
	}
	var c provider.OIDCConfig
	if err := json.Unmarshal(p.Config, &c); err != nil {
		return identity.Provider{}, provider.OIDCConfig{}, errs.Wrap(errs.CodeInternalError, err, "identity: reading an OIDC configuration")
	}
	return p, c, nil
}

// checkEnrollmentCode verifies code at begin with the rules of M1-37 (first_login, recovery or
// identity_link; unused, unexpired, not exhausted, an active person) without consuming it. Every
// rejection is the same 401 with a deny entry of auth.login.begin.
func (o *OIDC) checkEnrollmentCode(ctx context.Context, code string) (ids.ID, error) {
	var id ids.ID
	err := o.s.d.UoW.Do(ctx, func(ctx context.Context) error {
		m, err := o.s.d.Enrollment.Match(ctx, code, domain.ConsumablePurposes(domain.ConsumerIdentityLink)...)
		id = m.ID
		return err
	})
	var rej *EnrollmentRejectedError
	if errors.As(err, &rej) {
		e := redeemDenial(rej.Cause, rej.CodeID)
		e.Action = string(authorize.ActionAuthLoginBegin)
		_ = o.s.d.Audit.RecordDenial(ctx, e)
		return ids.ID{}, rej.Unwrap()
	}
	return id, err
}
