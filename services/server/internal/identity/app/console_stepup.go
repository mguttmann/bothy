// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/oidcflow"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/stepup"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The causes of a refused console step-up (audit details, codes only).
const (
	causeNotConsole       stepup.Cause = "not_console"
	causeStepUpKeyPresent              = "step_up_key_present"
	causeIdentityOther    stepup.Cause = "identity_mismatch"
	causeAuthTimeTooOld   stepup.Cause = "auth_time_before_challenge"
	causeNoLocalAccount                = "no_local_account"
	causeReauthFlow                    = "reauth_flow_mismatch"
	fieldAppRedirectURI                = "/app_redirect_uri"
	queryChallengeID                   = "challenge_id"
	detailConsoleRedirect              = "The redirect must be the console callback on the origin of the server."
	stepOIDCBegin                      = "oidc_begin"
	// fieldCodeInvalid is the field code of a value outside its rule.
	fieldCodeInvalid = "invalid"
)

// consoleOnly answers the 403 of the OIDC re-login called from a session other than a console
// session (08 section 6.2: its redirect is the console callback), with its audit denial.
func consoleOnly(_ context.Context, a actor.Actor) error {
	if a.ClientComponent == string(identity.ComponentConsole) {
		return nil
	}
	return notPossible(string(causeNotConsole), a.SessionID)
}

// passwordCaller answers the 403 of stepUpWithPassword from a session that may not use it (08
// section 6.2, FIX-stepup-macos): a console session may; an app session only while its device
// has no active step_up key (Windows, D-M4-11-2), so a device with one keeps the device key and
// gets no weaker method. A cli device is refused by admit (08 section 6.4).
func (c *Console) passwordCaller(ctx context.Context, a actor.Actor) error {
	switch a.ClientComponent {
	case string(identity.ComponentConsole):
		return nil
	case string(identity.ComponentApp):
	default:
		return notPossible(string(causeNotConsole), a.SessionID)
	}
	_, found, err := c.stepUps.d.Devices.ActiveKey(ctx, a.DeviceID, identity.KeyPurposeStepUp)
	if err != nil {
		return err
	}
	if found {
		return notPossible(causeStepUpKeyPresent, a.SessionID)
	}
	return nil
}

// begin runs the checks of every console step-up operation: the caller in ctx, the session check
// of the operation (allowed) and auth.step_up on the own session (StepUps.admit). Refusals are
// audited.
func (c *Console) begin(ctx context.Context, a actor.Actor, allowed func(context.Context, actor.Actor) error) (context.Context, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return nil, err
	}
	if c.stepUps == nil {
		return nil, errs.New(errs.CodeInternalError, "identity: the console step-up is not bound")
	}
	if err := allowed(ctx, a); err != nil {
		return nil, c.stepUps.finish(ctx, err)
	}
	if err := c.stepUps.admit(ctx, a); err != nil {
		return nil, c.stepUps.finish(ctx, err)
	}
	return ctx, nil
}

// StepUpWithPassword implements identity.Console (stepUpWithPassword, D-M6-01-6): a console
// session, or an app session on a device without an active step_up key (passwordCaller,
// FIX-stepup-macos); the password and the TOTP code are verified by the rules of the login (one
// Argon2id computation, the lockout of auth.login_fail_limit counted and committed); then one unit
// of work locks session and challenge and confirms with method password_totp, or fails the
// challenge, which counts toward auth.step_up_fail_limit. Only a console session gets a new
// cookie (confirmed).
func (c *Console) StepUpWithPassword(ctx context.Context, a actor.Actor, challengeID ids.ID, password, totpCode string) (identity.StepUpResult, error) {
	if c.local == nil || !c.local.enabled {
		return identity.StepUpResult{}, invisible()
	}
	ctx, err := c.begin(ctx, a, c.passwordCaller)
	if err != nil {
		return identity.StepUpResult{}, err
	}
	cause, err := c.local.stepUpPassword(ctx, a.ID, password, totpCode)
	if err != nil {
		return identity.StepUpResult{}, c.stepUps.finish(ctx, err)
	}
	return c.stepUps.completeWith(ctx, a, challengeID, identity.StepUpMethodPasswordTOTP,
		func(context.Context, ChallengeRecord) (stepup.Cause, error) { return stepup.Cause(cause), nil })
}

// BeginStepUpOIDC implements identity.Console (beginStepUpOidcReauth, D-M6-01-6): the redirect
// must be the console step-up callback, the challenge an issued, unexpired step-up challenge of
// the session; the flow runs in link mode for the person with prompt=login and max_age=0 and
// stores the challenge in its redirect, so the callback hands it back with the login_code.
func (c *Console) BeginStepUpOIDC(ctx context.Context, a actor.Actor, in identity.StepUpOIDCBegin) (identity.OIDCBeginResult, error) {
	if c.oidc == nil {
		return identity.OIDCBeginResult{}, invisible()
	}
	ctx, err := c.begin(ctx, a, consoleOnly)
	if err != nil {
		return identity.OIDCBeginResult{}, err
	}
	if in.AppRedirectURI != c.stepUpCallback() {
		return identity.OIDCBeginResult{}, consoleRedirectInvalid()
	}
	err = c.d.UoW.Do(ctx, func(ctx context.Context) error {
		_, err := c.stepUps.opened(ctx, a.SessionID, in.ChallengeID, c.stepUps.now())
		return err
	})
	if err != nil {
		return identity.OIDCBeginResult{}, c.stepUps.finish(ctx, err)
	}
	stored := c.stepUpCallback() + "?" + url.Values{queryChallengeID: {in.ChallengeID.String()}}.Encode()
	return c.oidc.begin(ctx, identity.OIDCBegin{ProviderID: in.ProviderID, AppRedirectURI: in.AppRedirectURI, ClientChallenge: in.ClientChallenge},
		oidcflow.ModeLink, a, beginOpts{redirect: stored, reauth: in.ChallengeID})
}

// consoleRedirectInvalid is the 400 of a console redirect other than the console callback.
func consoleRedirectInvalid() error {
	return errs.New(errs.CodeValidationFailed, "").
		WithFields(errs.FieldError{Pointer: fieldAppRedirectURI, Code: fieldCodeInvalid, Detail: detailConsoleRedirect})
}

// reauthChallenge returns the step-up challenge stored in the redirect of a re-login flow and
// true; false for a flow that is no console re-login.
func (c *Console) reauthChallenge(redirect string) (ids.ID, bool) {
	base, query, ok := strings.Cut(redirect, "?")
	if !ok || base != c.stepUpCallback() {
		return ids.ID{}, false
	}
	v, err := url.ParseQuery(query)
	if err != nil {
		return ids.ID{}, false
	}
	id, err := ids.Parse(v.Get(queryChallengeID))
	return id, err == nil
}

// CompleteStepUpOIDC implements identity.Console (completeStepUpOidcReauth, D-M6-01-6): one unit
// of work redeems the login_code of a re-login flow of the person with the verifier and completes
// the flow; then the challenge stored in the flow is confirmed with method oidc_reauth when the
// provider identity is one of the person's (same issuer and subject) and its auth_time is not
// before the second of the challenge's creation (auth_time has whole seconds); otherwise the
// challenge fails and counts.
func (c *Console) CompleteStepUpOIDC(ctx context.Context, a actor.Actor, loginCode, clientVerifier string) (identity.StepUpResult, error) {
	if c.oidc == nil {
		return identity.StepUpResult{}, invisible()
	}
	ctx, err := c.begin(ctx, a, consoleOnly)
	if err != nil {
		return identity.StepUpResult{}, err
	}
	re, err := c.redeemReauth(ctx, a, loginCode, clientVerifier)
	if err != nil {
		return identity.StepUpResult{}, err
	}
	return c.stepUps.completeWith(ctx, a, re.challengeID, identity.StepUpMethodOIDCReauth,
		func(ctx context.Context, ch ChallengeRecord) (stepup.Cause, error) {
			return c.reauthCause(ctx, a, re, ch)
		})
}

// reauth is a redeemed console re-login: the provider, the verified identity and the challenge
// its flow names.
type reauth struct {
	providerID, challengeID ids.ID
	ext                     oidcflow.ExternalIdentity
}

// redeemReauth redeems the login_code of a console re-login of a in one unit of work: the flow
// (link mode, the person's, a step-up redirect), the PKCE check and the completed flow. A refusal
// answers 401 step_up_required, audited like every refused flow.
func (c *Console) redeemReauth(ctx context.Context, a actor.Actor, loginCode, clientVerifier string) (reauth, error) {
	var (
		st  oidcCompletion
		out reauth
	)
	hash := oidcflow.Hash(c.oidc.pepper, oidcflow.HashLoginCode, loginCode)
	err := c.d.UoW.Do(ctx, func(ctx context.Context) error {
		st = oidcCompletion{}
		f, err := c.oidc.redeemable(ctx, hash, oidcflow.ModeLink, errs.New(errs.CodeStepUpRequired, ""))
		if err != nil {
			return err
		}
		id, ok := c.reauthChallenge(f.AppRedirectURI)
		if !ok || f.UserID != a.ID {
			return &flowRejectedError{answer: errs.New(errs.CodeStepUpRequired, ""), cause: causeReauthFlow}
		}
		st.flow, out = f, reauth{providerID: f.ProviderID, challengeID: id, ext: *f.External}
		if !oidcflow.VerifierMatches(clientVerifier, f.ClientChallenge) {
			return &flowRejectedError{answer: errs.New(errs.CodeStepUpRequired, ""), cause: causeVerifierMismatch, fail: true}
		}
		return c.oidc.completeFlow(ctx, f.ID)
	})
	if err != nil {
		return reauth{}, c.oidc.completeFailed(ctx, err, st, authorize.ActionAuthStepUp)
	}
	return out, nil
}

// reauthCause decides the re-login re on the locked challenge ch: the identity must be one of
// the person's (same issuer and subject) and auth_time not before the second of the challenge's
// creation; it returns the cause of a refusal, "" for a confirmation.
func (c *Console) reauthCause(ctx context.Context, a actor.Actor, re reauth, ch ChallengeRecord) (stepup.Cause, error) {
	rec, found, err := c.d.Identities.ActiveIdentityBySubject(ctx, re.providerID, re.ext.Subject)
	if err != nil {
		return "", err
	}
	if !found || rec.UserID != a.ID || rec.Issuer != re.ext.Issuer {
		return causeIdentityOther, nil
	}
	if re.ext.AuthTime.IsZero() || re.ext.AuthTime.Before(ch.CreatedAt.Truncate(time.Second)) {
		return causeAuthTimeTooOld, nil
	}
	return "", nil
}

// completeWith confirms the step-up challenge challengeID of the console session of a with method
// in one unit of work: the session rows and the challenge locked (03 section 6.3), check decides
// on the locked challenge; a cause fails the challenge and counts toward auth.step_up_fail_limit,
// none confirms the step-up and rotates the cookie (confirmed). The caller ran admit.
func (s *StepUps) completeWith(ctx context.Context, a actor.Actor, challengeID ids.ID, method string,
	check func(ctx context.Context, ch ChallengeRecord) (stepup.Cause, error)) (identity.StepUpResult, error) {
	var out completion
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		now := s.now()
		sess, err := s.lockSession(ctx, a.SessionID, now)
		if err != nil {
			return err
		}
		ch, err := s.opened(ctx, a.SessionID, challengeID, now)
		if err != nil {
			return err
		}
		cause, err := check(ctx, ch)
		if err != nil {
			return err
		}
		if cause != "" {
			out, err = s.fail(ctx, a, ch.ID, cause, now)
			return err
		}
		out, err = s.confirmed(ctx, sess, ch.ID, method, now)
		return err
	})
	if err != nil {
		return identity.StepUpResult{}, s.finish(ctx, err)
	}
	switch {
	case out.revoked:
		return identity.StepUpResult{}, errs.New(errs.CodeSessionRevoked, "")
	case out.failed:
		return identity.StepUpResult{}, errs.New(errs.CodeStepUpRequired, "")
	}
	return out.result, nil
}

// stepUpPassword checks password and totpCode of the local account of userID for a console
// step-up by the rules of the login (brief M1-39): one Argon2id computation without a lock, then
// one unit of work that locks the row, refuses a locked or changed row without counting, counts a
// wrong password or code toward auth.login_fail_limit (committed) and, on success, stores the used
// TOTP step, resets the counter and rehashes when the parameters changed. It returns the cause of
// a refusal ("" for a valid confirmation). Errors: 403 forbidden (step_up_not_possible) without a
// local account with an established password.
func (l *LocalCredentials) stepUpPassword(ctx context.Context, userID ids.ID, password, totpCode string) (string, error) {
	ident, err := l.ownLocalIdentity(ctx, userID)
	if err != nil {
		return "", notPossible(causeNoLocalAccount, ids.ID{})
	}
	rec, found, err := l.d.Credentials.LocalCredential(ctx, ident.ID)
	if err != nil {
		return "", err
	}
	if !found || rec.PasswordHash == "" {
		return "", notPossible(causeNoLocalAccount, ids.ID{})
	}
	nfc := l.d.Normalizer.NFC(password)
	ok, rehash, err := l.d.Hasher.Verify(rec.PasswordHash, nfc)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternalError, err, "identity: verifying a password")
	}
	v := verified{at: passwordAttempt{userID: userID, identity: ident.ID}, found: true, hash: rec.PasswordHash, ok: ok, rehash: rehash, nfc: nfc}
	var cause string
	err = l.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		cause, err = l.stepUpPasswordLocked(ctx, v, totpCode)
		return err
	})
	return cause, err
}

// stepUpPasswordLocked is the unit of work of stepUpPassword: the row locked, a changed or locked
// row refused without counting, a wrong password or code counted (and committed), a success with
// the used TOTP step, the reset counter and the new hash when the parameters changed.
func (l *LocalCredentials) stepUpPasswordLocked(ctx context.Context, v verified, totpCode string) (string, error) {
	rec, found, err := l.d.Credentials.LockLocalCredential(ctx, v.at.identity)
	if err != nil {
		return "", err
	}
	now := l.d.Clock.Now()
	switch {
	case !found || rec.PasswordHash != v.hash:
		return causeChanged, nil
	case rec.Lockout.Locked(now):
		return causeLocked, nil
	}
	cause, err := l.check(&rec, v.ok, totpCode, now)
	if err != nil {
		return "", err
	}
	if cause != "" {
		_, err := l.fail(ctx, v.at, rec, now)
		return cause, err
	}
	rec.Lockout = domain.Lockout{}
	if v.rehash {
		phc, err := l.d.Hasher.Hash(v.nfc)
		if err != nil {
			return "", errs.Wrap(errs.CodeInternalError, err, "identity: rehashing a password")
		}
		rec.PasswordHash = phc
	}
	return "", l.d.Credentials.SaveLocalCredential(ctx, rec)
}

// beginOpts are the variants of an OIDC flow start (M6-01): redirect, when set, is the stored
// redirect of a console flow (it replaces the check against the provider's app_redirect_uris);
// reauth names the step-up challenge of a console re-login, which asks the provider for
// prompt=login and max_age=0 and records auth.step_up instead of identity.link.
type beginOpts struct {
	redirect string
	reauth   ids.ID
}

// reauthEntry is the allow entry of the start of a console re-login.
func reauthEntry(sid, challengeID, providerID, flowID ids.ID) auditing.Entry {
	return auditing.Entry{
		Action: string(authorize.ActionAuthStepUp), Decision: auditing.DecisionAllow, ResourceKind: resourceAuthSession, ResourceID: sid,
		Attrs: auditing.Attrs{
			attrStep: auditing.Code(stepOIDCBegin), attrChallengeID: auditing.ID(challengeID), attrProviderID: auditing.ID(providerID),
			attrLoginAttemptID: auditing.ID(flowID),
		},
	}
}
