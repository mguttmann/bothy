// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/oidcflow"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The causes of a rejected oidcCompletion in audit details (codes, SEC-AUD-5).
const (
	causeLoginCodeUnknown = "login_code_unknown"
	causeVerifierMismatch = "verifier_mismatch"
	causeNoMapping        = "no_mapping"
	causeProviderOff      = "provider_disabled"
	causePersonInactive   = "person_not_active"
	causeOtherPerson      = "other_person"
)

// flowRejectedError is a refused oidcCompletion: answer is the error of the response, cause the audit
// cause, and fail asks to end the flow as failed after the rollback.
type flowRejectedError struct {
	answer error
	cause  string
	fail   bool
	codeID ids.ID
}

func (r *flowRejectedError) Error() string { return "identity: login flow rejected (" + r.cause + ")" }

// oidcCompletion is what a completing unit of work learned before it failed, for the answer: the
// flow and the enrollment code it was about to consume.
type oidcCompletion struct {
	flow   LoginFlowRecord
	codeID ids.ID
}

// CompleteOIDCLogin implements identity.OIDCLogin (brief M1-41 "completeOidcLogin"): authorize,
// the DPoP proof as a login proof without ath (a rejected proof never looks the code up), then one
// unit of work: the flow by its login_code (locked), the PKCE check, the mapping of 08 section 3.1
// (identity, enrollment code, provisioning), device, session and tokens, and the flow completed
// with its external identity dropped.
func (o *OIDC) CompleteOIDCLogin(ctx context.Context, in identity.OIDCComplete) (identity.RedeemResult, error) {
	if err := o.authorize(ctx, actor.Anonymous(), authorize.ActionAuthLoginComplete, instanceResource); err != nil {
		return identity.RedeemResult{}, err
	}
	proof, err := o.s.d.DPoP.Verify(ctx, in.Proof)
	if err != nil {
		return identity.RedeemResult{}, err
	}
	hash := oidcflow.Hash(o.pepper, oidcflow.HashLoginCode, in.LoginCode)
	var (
		out LoginResult
		st  oidcCompletion
	)
	err = o.s.d.UoW.Do(ctx, func(ctx context.Context) error {
		st = oidcCompletion{}
		f, err := o.redeemable(ctx, hash, oidcflow.ModeLogin, errs.New(errs.CodeUnauthenticated, ""))
		if err != nil {
			return err
		}
		st.flow = f
		if !oidcflow.VerifierMatches(in.ClientVerifier, f.ClientChallenge) {
			return &flowRejectedError{answer: errs.New(errs.CodeUnauthenticated, ""), cause: causeVerifierMismatch, fail: true}
		}
		out, err = o.login(ctx, f, in, proof.JKT, &st)
		if err != nil {
			return err
		}
		return o.completeFlow(ctx, f.ID)
	})
	if err != nil {
		return identity.RedeemResult{}, o.completeFailed(ctx, err, st, authorize.ActionAuthLoginComplete)
	}
	res := identity.RedeemResult{Tokens: out.Tokens}
	if proof.NonceStale {
		res.Nonce = o.s.d.DPoP.CurrentNonce()
	}
	return res, nil
}

// redeemable locks the flow of a login_code hash and returns it when it has mode, the status
// code_issued and neither the code nor the flow has expired; otherwise a rejection with answer
// and no write.
func (o *OIDC) redeemable(ctx context.Context, hash []byte, mode oidcflow.Mode, answer error) (LoginFlowRecord, error) {
	f, found, err := o.d.Flows.LockLoginFlowByCodeHash(ctx, hash)
	if err != nil {
		return LoginFlowRecord{}, err
	}
	now := o.s.d.Clock.Now()
	if !found || f.Mode != mode || f.Status != oidcflow.StatusCodeIssued || f.External == nil ||
		!now.Before(f.LoginCodeExpiresAt) || !now.Before(f.ExpiresAt) {
		return LoginFlowRecord{}, &flowRejectedError{answer: answer, cause: causeLoginCodeUnknown}
	}
	return f, nil
}

// completeFlow marks the flow id completed; a flow that changed under the row lock is a server
// error.
func (o *OIDC) completeFlow(ctx context.Context, id ids.ID) error {
	ok, err := o.d.Flows.CompleteLoginFlow(ctx, id)
	if err == nil && !ok {
		err = errs.New(errs.CodeInternalError, "identity: the login flow changed under its lock")
	}
	return err
}

// login maps the external identity of f (08 section 3.1) and completes the login.
func (o *OIDC) login(ctx context.Context, f LoginFlowRecord, in identity.OIDCComplete, jkt string, st *oidcCompletion) (LoginResult, error) {
	p, found, err := o.s.d.Providers.EnabledProvider(ctx, f.ProviderID)
	if err != nil {
		return LoginResult{}, err
	}
	if !found {
		return LoginResult{}, &flowRejectedError{answer: errs.New(errs.CodeUnauthenticated, ""), cause: causeProviderOff, fail: true}
	}
	ext := *f.External
	base := LoginCompletion{
		AuthMethod: identity.AuthOIDC, Device: in.Device, ProofJKT: jkt, ClientIP: in.ClientIP, AllowPending: true, Trust: TrustUnconfirmed,
	}
	rec, known, err := o.s.d.Identities.ActiveIdentityBySubject(ctx, p.ID, ext.Subject)
	if err != nil {
		return LoginResult{}, err
	}
	switch {
	case known && rec.Issuer == ext.Issuer:
		base.UserID, base.IdentityID = rec.UserID, &rec.ID
		out, err := o.s.completeLogin(ctx, base)
		if errs.KindOf(err) == errs.KindUnauthenticated && !isDPoPError(err) {
			// A disabled or deleted person never logs in; nothing is written (08 section 3.1 step 5).
			return LoginResult{}, &flowRejectedError{answer: err, cause: causePersonInactive}
		}
		return out, err
	case known:
		// The subject is linked under another issuer of a multi-tenant provider: it can be neither
		// mapped nor linked a second time.
		return LoginResult{}, &flowRejectedError{answer: errs.New(errs.CodeUnauthenticated, ""), cause: causeNoMapping, fail: true}
	case !f.EnrollmentCodeID.IsZero():
		return o.linkByCode(ctx, f, p, base, st)
	case p.AllowProvisioning:
		return o.provision(ctx, f, p, base)
	}
	return LoginResult{}, &flowRejectedError{answer: errs.New(errs.CodeUnauthenticated, ""), cause: causeNoMapping, fail: true}
}

// isDPoPError reports a refused DPoP proof key, which is not a refusal of the person.
func isDPoPError(err error) bool {
	var d *identity.DPoPError
	return errors.As(err, &d)
}

// linkByCode links the identity to the person of the enrollment code the flow checked at begin,
// consumes the code with the registering device, makes that device active (08 section 3.2) and
// records identity.link with the person as actor.
func (o *OIDC) linkByCode(ctx context.Context, f LoginFlowRecord, p identity.Provider, base LoginCompletion, st *oidcCompletion) (LoginResult, error) {
	m, err := o.s.d.Enrollment.MatchID(ctx, f.EnrollmentCodeID, domain.ConsumablePurposes(domain.ConsumerIdentityLink)...)
	var rej *EnrollmentRejectedError
	if errors.As(err, &rej) {
		return LoginResult{}, &flowRejectedError{answer: rej.Unwrap(), cause: rej.Cause, fail: true, codeID: rej.CodeID}
	}
	if err != nil {
		return LoginResult{}, err
	}
	st.codeID = m.ID
	linked, err := o.insertIdentity(ctx, m.UserID, p, *f.External)
	if err != nil {
		return LoginResult{}, err
	}
	base.UserID, base.IdentityID, base.Trust = m.UserID, &linked.ID, TrustConfirmed
	out, err := o.s.completeLogin(ctx, base)
	if err != nil {
		return LoginResult{}, err
	}
	if err := o.s.d.Enrollment.MarkUsed(ctx, m.ID, out.Device.ID); err != nil {
		return LoginResult{}, err
	}
	return out, o.s.d.Audit.Record(loginActor(ctx, m.UserID, out.Tokens), auditing.Entry{
		Action: string(authorize.ActionIdentityLink), Decision: auditing.DecisionAllow, ResourceKind: resourceUser, ResourceID: m.UserID,
		Attrs: auditing.Attrs{
			attrProviderID: auditing.ID(p.ID), attrIdentityID: auditing.ID(linked.ID), attrEnrollmentCodeID: auditing.ID(m.ID),
			attrLoginAttemptID: auditing.ID(f.ID),
		},
	})
}

// insertIdentity links ext at p to userID: issuer, subject and, when the hint is one plain
// address, email_at_link (a display snapshot).
func (o *OIDC) insertIdentity(ctx context.Context, userID ids.ID, p identity.Provider, ext oidcflow.ExternalIdentity) (identity.Identity, error) {
	rec := identity.Identity{
		ID: ids.New(), UserID: userID, ProviderID: p.ID, ProviderKind: p.Kind, Subject: ext.Subject, Issuer: ext.Issuer,
		LinkedAt: o.s.d.Clock.Now(),
	}
	if ext.EmailHint != "" && domain.CheckEmail(ext.EmailHint) == nil {
		rec.EmailAtLink = ext.EmailHint
	}
	return o.s.d.Identities.InsertIdentity(ctx, rec)
}

// handleAttempts bounds the search for a free handle of a provisioned person.
const handleAttempts = 10

// provision creates a person in status pending without grants and without e-mail (users.email
// stays NULL: it is unique and would let the provider steer logins), links the identity and
// completes the login with a pending device (08 section 3.1 step 3).
func (o *OIDC) provision(ctx context.Context, f LoginFlowRecord, p identity.Provider, base LoginCompletion) (LoginResult, error) {
	ext := *f.External
	handle, err := o.freeHandle(ctx, oidcflow.ProvisionedHandle(ext.PreferredUsername, ext.DisplayName, o.cfg.HandleMaxChars))
	if err != nil {
		return LoginResult{}, err
	}
	name := oidcflow.SanitizeDisplayName(ext.DisplayName, o.cfg.DisplayNameMaxChars)
	dn, err := domain.NewDisplayName(name, o.cfg.DisplayNameMaxChars)
	if err != nil {
		if dn, err = domain.NewDisplayName(handle.String(), o.cfg.DisplayNameMaxChars); err != nil {
			return LoginResult{}, err
		}
	}
	pr, err := domain.NewPrincipal(ids.New(), domain.KindUser, handle, dn)
	if err != nil {
		return LoginResult{}, err
	}
	pr.Status = domain.StatusPending
	u, err := domain.NewUser(pr, "", "", "")
	if err != nil {
		return LoginResult{}, err
	}
	if err := o.s.d.People.InsertPrincipal(ctx, &u.Principal); err != nil {
		return LoginResult{}, err
	}
	if err := o.s.d.People.InsertUser(ctx, u); err != nil {
		return LoginResult{}, err
	}
	linked, err := o.insertIdentity(ctx, pr.ID, p, ext)
	if err != nil {
		return LoginResult{}, err
	}
	base.UserID, base.IdentityID, base.Provisioned = pr.ID, &linked.ID, true
	return o.s.completeLogin(ctx, base)
}

// freeHandle returns base when no principal has it (without regard to case), else base with a
// random suffix that is free; after handleAttempts tries the login fails with conflict.
func (o *OIDC) freeHandle(ctx context.Context, base string) (domain.Handle, error) {
	candidate := base
	for range handleAttempts {
		_, taken, err := o.s.d.People.PrincipalByHandle(ctx, candidate)
		if err != nil {
			return domain.Handle{}, err
		}
		if !taken {
			return domain.NewHandle(candidate, o.cfg.HandleMaxChars)
		}
		suffix, err := oidcflow.NewHandleSuffix(o.d.Random)
		if err != nil {
			return domain.Handle{}, errs.Wrap(errs.CodeInternalError, err, "identity: random source")
		}
		candidate = oidcflow.HandleWithSuffix(base, suffix, o.cfg.HandleMaxChars)
	}
	return domain.Handle{}, errs.New(errs.CodeConflict, "").
		WithFields(errs.FieldError{Pointer: "/handle", Code: "taken", Detail: "No free handle was found."})
}

// completeFailed is the answer of a failed oidcCompletion of act: a rejected flow is ended as failed
// when asked (own unit of work after the rollback) and audited, then answers with its answer; a
// rejected login with an enrollment code about to be consumed counts a failed attempt of the code
// (08 section 3.2); a server failure is returned unchanged.
func (o *OIDC) completeFailed(ctx context.Context, err error, st oidcCompletion, act authorize.Action) error {
	var rej *flowRejectedError
	if errors.As(err, &rej) {
		return o.rejected(ctx, rej, st, act)
	}
	if !countsAsAttempt(err) {
		return err
	}
	if !st.codeID.IsZero() {
		if aerr := o.s.d.Enrollment.RecordFailedAttempt(context.WithoutCancel(ctx), st.codeID); aerr != nil {
			return aerr
		}
	}
	if errs.KindOf(err) == errs.KindConflict && act == authorize.ActionIdentityLink && !st.flow.ID.IsZero() {
		if ferr := o.failFlow(ctx, st.flow.ID); ferr != nil {
			return ferr
		}
	}
	_ = o.s.d.Audit.RecordDenial(ctx, completeDenial(act, causeLoginRejected, st.flow, st.codeID))
	return err
}

// rejected answers a rejected flow: it is ended as failed when the rejection asks so (own unit
// of work after the rollback), audited, and answered with its answer.
func (o *OIDC) rejected(ctx context.Context, rej *flowRejectedError, st oidcCompletion, act authorize.Action) error {
	if rej.fail && !st.flow.ID.IsZero() {
		if err := o.failFlow(ctx, st.flow.ID); err != nil {
			return err
		}
	}
	codeID := rej.codeID
	if codeID.IsZero() {
		codeID = st.codeID
	}
	_ = o.s.d.Audit.RecordDenial(ctx, completeDenial(act, rej.cause, st.flow, codeID))
	return rej.answer
}

// failFlow ends the flow id as login_failed in an own unit of work; the count must not depend on
// the client, so a dropped connection does not cancel it.
func (o *OIDC) failFlow(ctx context.Context, id ids.ID) error {
	return o.s.d.UoW.Do(context.WithoutCancel(ctx), func(ctx context.Context) error {
		_, err := o.d.Flows.FailLoginFlow(ctx, id, oidcflow.ErrorLoginFailed)
		return err
	})
}

// completeDenial is the deny entry of a failed oidcCompletion: cause, provider, flow and code ids,
// never a code, subject, e-mail or name.
func completeDenial(act authorize.Action, cause string, f LoginFlowRecord, codeID ids.ID) auditing.Entry {
	attrs := auditing.Attrs{attrCause: auditing.Code(cause)}
	if !f.ID.IsZero() {
		attrs[attrLoginAttemptID] = auditing.ID(f.ID)
		attrs[attrProviderID] = auditing.ID(f.ProviderID)
	}
	if !codeID.IsZero() {
		attrs[attrEnrollmentCodeID] = auditing.ID(codeID)
	}
	return auditing.Entry{
		Action: string(act), Decision: auditing.DecisionDeny, Reason: auditing.ReasonUnauthenticated,
		ResourceKind: string(authorize.ResourceInstance), Attrs: attrs,
	}
}

// CompleteIdentityLink implements identity.OIDCLogin (brief M1-41 "Link mode"): authorize
// identity.link with step-up first, so a 401 step_up_required leaves the login_code usable; then
// one unit of work: the link flow of the caller (another person's or an unknown code is 404), the
// PKCE check, the identity row and the allow entry.
func (o *OIDC) CompleteIdentityLink(ctx context.Context, a actor.Actor, loginCode, clientVerifier string) (identity.Identity, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.Identity{}, err
	}
	if err := o.authorize(ctx, a, authorize.ActionIdentityLink, userResource(a.ID)); err != nil {
		return identity.Identity{}, err
	}
	hash := oidcflow.Hash(o.pepper, oidcflow.HashLoginCode, loginCode)
	var (
		out identity.Identity
		st  oidcCompletion
	)
	err = o.s.d.UoW.Do(ctx, func(ctx context.Context) error {
		st = oidcCompletion{}
		var err error
		out, err = o.link(ctx, a, hash, clientVerifier, &st)
		return err
	})
	if err != nil {
		return identity.Identity{}, o.completeFailed(ctx, err, st, authorize.ActionIdentityLink)
	}
	return out, nil
}

// link is CompleteIdentityLink inside its unit of work: the link flow of a by the code hash, the
// PKCE check, the identity row, the completed flow and the allow entry.
func (o *OIDC) link(ctx context.Context, a actor.Actor, hash []byte, clientVerifier string, st *oidcCompletion) (identity.Identity, error) {
	f, err := o.redeemable(ctx, hash, oidcflow.ModeLink, invisible())
	if err != nil {
		return identity.Identity{}, err
	}
	if f.UserID != a.ID {
		return identity.Identity{}, &flowRejectedError{answer: invisible(), cause: causeOtherPerson}
	}
	st.flow = f
	if !oidcflow.VerifierMatches(clientVerifier, f.ClientChallenge) {
		return identity.Identity{}, &flowRejectedError{answer: errs.New(errs.CodeUnauthenticated, ""), cause: causeVerifierMismatch, fail: true}
	}
	p, found, err := o.s.d.Providers.EnabledProvider(ctx, f.ProviderID)
	if err != nil {
		return identity.Identity{}, err
	}
	if !found {
		return identity.Identity{}, &flowRejectedError{answer: invisible(), cause: causeProviderOff, fail: true}
	}
	out, err := o.insertIdentity(ctx, a.ID, p, *f.External)
	if err != nil {
		return identity.Identity{}, err
	}
	if err := o.completeFlow(ctx, f.ID); err != nil {
		return identity.Identity{}, err
	}
	return out, o.s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionIdentityLink), Decision: auditing.DecisionAllow, ResourceKind: resourceUser, ResourceID: a.ID,
		Attrs: auditing.Attrs{
			attrStep: auditing.Code("complete"), attrProviderID: auditing.ID(p.ID), attrIdentityID: auditing.ID(out.ID),
			attrLoginAttemptID: auditing.ID(f.ID),
		},
	})
}
