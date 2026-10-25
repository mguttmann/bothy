// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// The schemes of the Authorization header (06 section 3).
const (
	schemeDPoP   = "dpop"
	schemeBearer = "bearer"
)

// schemeChallenge is the WWW-Authenticate of a request whose credentials are not usable at all.
const schemeChallenge = `DPoP algs="` + identity.DPoPAlgs + `"`

// headerDPoPNonce carries the current nonce (06 section 3.1 item 2).
const headerDPoPNonce = "DPoP-Nonce"

// Log messages and keys of failed activity writes; the keys are constants for sloglint.
const (
	msgTouchFailed      = "recording session activity failed"
	msgTokenTouchFailed = "recording API token use failed"
	logKeyError         = "error_type"
	logKeyTokenID       = "token_id"
)

// Authenticator implements kernel/actor.Authenticator (brief M1-34 "Authenticator", brief M1-44
// "Authentication"): for people the access token, the DPoP proof, then one query for session,
// device and key; for API tokens the format and checksum, then one query by HMAC, then the DPoP
// binding. It does not reject on the state of a session, a device or a found API token: it
// reports it in the actor, and authorize decides (check_active), so the denial is audited with its
// reason (07 section 4.2; token_revoked for a revoked or expired token). Nothing about a token is
// cached here, so a revocation holds at the next request in every process (AZ-P9). It is safe for
// concurrent use.
type Authenticator struct {
	cfg     Config
	tokens  *AccessTokens
	dpop    identity.DPoP
	store   SessionStore
	api     APITokenStore
	pepper  []byte
	apps    identity.AppInstallations
	clk     clock.Clock
	log     *slog.Logger
	bind    SessionBinding
	revoke  identity.Auth
	uow     txn.UnitOfWork
	audit   auditing.Recorder
	console *Console
	acting  identity.ActingForSource
}

// Compile-time check: the Authenticator is the kernel port.
var _ actor.Authenticator = (*Authenticator)(nil)

// AuthenticatorDeps are the collaborators of the Authenticator.
type AuthenticatorDeps struct {
	// AccessTokens verifies the access tokens of people.
	AccessTokens *AccessTokens
	// DPoP verifies the proofs of access tokens and bound API tokens.
	DPoP identity.DPoP
	// Sessions reads session, device and key of a person's request.
	Sessions SessionStore
	// APITokens reads API tokens by hash and id (M1-44).
	APITokens APITokenStore
	// Pepper is the raw token pepper, the HMAC key of the API token lookup.
	Pepper []byte
	// Apps reads the installation of an app principal on every request of its token (M2-24);
	// nil means identity.NoAppInstallations: no app is installed.
	Apps identity.AppInstallations
	// Clock is the time of the state checks; nil means clock.System().
	Clock clock.Clock
	// Log receives failed activity writes; nil means slog.Default().
	Log *slog.Logger
	// Binding checks every request of a session beyond token and proof (M1-43: the header login
	// of trusted_header sessions); nil checks nothing. With it, Revoker, UoW and Audit are required.
	Binding SessionBinding
	// Revoker revokes a session whose binding failed.
	Revoker identity.Auth
	// UoW runs the revocation with its audit entry.
	UoW txn.UnitOfWork
	// Audit records the denial of a failed binding.
	Audit auditing.Recorder
	// Console authenticates the console cookie, the second credential kind of people (M6-01);
	// nil refuses every cookie with 401 unauthenticated.
	Console *Console
	// ActingFor resolves the Invocation-Token header of an agent's request (07 guard rule SR9,
	// brief M2-14 design note 2); nil means identity.NoActingForSource: every header is invalid.
	ActingFor identity.ActingForSource
}

// NewAuthenticator returns the Authenticator with identity.last_active_granularity of cfg (not
// negative) and the collaborators of d. Errors: a missing collaborator, an empty pepper or a
// negative granularity.
func NewAuthenticator(cfg Config, d AuthenticatorDeps) (*Authenticator, error) {
	if d.AccessTokens == nil || d.DPoP == nil || d.Sessions == nil || d.APITokens == nil || len(d.Pepper) == 0 {
		return nil, errors.New("identity: NewAuthenticator needs the access tokens, the DPoP verifier, the session and API token stores and the pepper")
	}
	if d.Binding != nil && (d.Revoker == nil || d.UoW == nil || d.Audit == nil) {
		return nil, errors.New("identity: a session binding needs the revoker, the unit of work and the recorder")
	}
	if cfg.LastActiveGranularity < 0 || cfg.DevicePendingTTL <= 0 {
		return nil, errors.New("identity: identity.last_active_granularity must not be negative and auth.device_pending_ttl must be positive")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if d.Apps == nil {
		d.Apps = identity.NoAppInstallations{}
	}
	if d.ActingFor == nil {
		d.ActingFor = identity.NoActingForSource{}
	}
	return &Authenticator{
		cfg: cfg, tokens: d.AccessTokens, dpop: d.DPoP, store: d.Sessions, api: d.APITokens, pepper: append([]byte(nil), d.Pepper...),
		apps: d.Apps, clk: d.Clock, log: d.Log, bind: d.Binding, revoke: d.Revoker, uow: d.UoW, audit: d.Audit, console: d.Console,
		acting: d.ActingFor,
	}, nil
}

// unauthenticated is the 401 of credentials the server cannot use.
func unauthenticated() error {
	return errs.New(errs.CodeUnauthenticated, "").WithHeader("WWW-Authenticate", schemeChallenge)
}

// Authenticate implements actor.Authenticator in the order of the brief: no credentials are
// anonymous; Invocation-Token values without an Authorization header are 401 unauthenticated (a
// run binding needs an authenticated agent, brief M2-14 design note 2 b), even next to the console
// cookie; an Authorization header together with the console cookie is 400 validation_failed
// (one credential per request, D-M6-01-4); the cookie alone takes the console path
// (Console.Authenticate, M6-01); more than one header or another scheme is unauthenticated; a
// token with the prefix bothy_ takes the API token path under either scheme
// (AuthenticateAPIToken); otherwise Bearer with this server's access token fails with
// invalid_dpop_proof (RFC 9449 section 7.2) and DPoP checks the access token, then the proof, then
// reads session, device and key. After a successful path the Invocation-Token header is resolved
// into actor.ActingFor (actingFor).
func (a *Authenticator) Authenticate(ctx context.Context, creds actor.Credentials) (actor.Actor, error) {
	if len(creds.InvocationTokens) > 0 && creds.Authorization == "" {
		return actor.Actor{}, unauthenticated()
	}
	out, err := a.authenticate(ctx, creds)
	if err != nil || len(creds.InvocationTokens) == 0 {
		return out, err
	}
	return a.actingFor(ctx, out, creds.InvocationTokens)
}

// actingFor sets out.ActingFor from the values of the Invocation-Token header (07 guard rule SR9,
// brief M2-14 design note 2 c to e): more than one value, an empty value or a principal that is
// not an agent is invalid without asking the source, because the token's aud can only name an
// agent (05 section 5); exactly one value of an agent takes the source's result. A failing source
// is 503 unavailable, never the actor without its binding (07 section 1 point 5, fail closed); the
// error carries no token. The status never rejects here, authorize decides and audits it, and
// nothing is cached, so the end of a run or a revocation counts at the next request.
func (a *Authenticator) actingFor(ctx context.Context, out actor.Actor, values []string) (actor.Actor, error) {
	if len(values) != 1 || values[0] == "" || out.Kind != actor.KindAgent {
		out.ActingFor = actor.ActingFor{Status: actor.ActingForInvalid}
		return out, nil
	}
	af, err := a.acting.ResolveActingFor(ctx, out.ID, values[0])
	if err != nil {
		return actor.Actor{}, errs.Wrap(errs.CodeUnavailable, err, "").WithCause(errs.CauseDependencyUnavailable)
	}
	out.ActingFor = af
	return out, nil
}

// authenticate is Authenticate without the Invocation-Token header: the credential paths of M1-34,
// M1-44 and M6-01.
func (a *Authenticator) authenticate(ctx context.Context, creds actor.Credentials) (actor.Actor, error) {
	if !creds.Present() {
		return actor.Anonymous(), nil
	}
	if !creds.ConsoleCookie.IsZero() {
		if creds.Authorization != "" {
			return actor.Actor{}, errs.New(errs.CodeValidationFailed, "Send either the console cookie or an Authorization header.")
		}
		if a.console == nil {
			return actor.Actor{}, cookieRefused(errs.CodeUnauthenticated)
		}
		return a.console.Authenticate(ctx, creds)
	}
	scheme, token, ok := splitAuthorization(creds.Authorization)
	if !ok || scheme != schemeBearer && scheme != schemeDPoP {
		return actor.Actor{}, unauthenticated()
	}
	if apitoken.HasPrefix(token) {
		return a.AuthenticateAPIToken(ctx, token, scheme == schemeDPoP, creds)
	}
	if scheme == schemeBearer {
		return actor.Actor{}, a.bearer(ctx, token)
	}
	return a.person(ctx, creds, token)
}

// splitAuthorization splits "<scheme> <token>" (scheme without regard to case, RFC 9110 section
// 11.1); a value with a comma is several headers joined by the chain, which is refused.
func splitAuthorization(v string) (scheme, token string, ok bool) {
	if strings.Contains(v, ",") {
		return "", "", false
	}
	scheme, token, ok = strings.Cut(v, " ")
	if !ok || token == "" || strings.ContainsAny(token, " \t") {
		return "", "", false
	}
	return strings.ToLower(scheme), token, true
}

// bearer is the Bearer branch of a value that is no API token: a DPoP-bound access token of this
// server is invalid_dpop_proof (ID-T14), anything else unauthenticated.
func (a *Authenticator) bearer(ctx context.Context, token string) error {
	if a.tokens.IsOwnAccessToken(ctx, token) {
		return &identity.DPoPError{Code: identity.DPoPInvalidProof, Nonce: a.dpop.CurrentNonce()}
	}
	return unauthenticated()
}

// proofs returns the DPoP values of creds: DPoPProofs, or DPoP alone for callers that fill only it.
func proofs(creds actor.Credentials) []string {
	if creds.DPoPProofs != nil {
		return creds.DPoPProofs
	}
	if creds.DPoP != "" {
		return []string{creds.DPoP}
	}
	return nil
}

// person authenticates a person with a DPoP-bound access token.
func (a *Authenticator) person(ctx context.Context, creds actor.Credentials, token string) (actor.Actor, error) {
	c, err := a.tokens.Verify(ctx, token)
	if err != nil {
		return actor.Actor{}, err
	}
	proof, err := a.dpop.Verify(ctx, identity.ProofRequest{
		Proofs: proofs(creds), Method: creds.Method, URL: creds.HTU, AccessToken: token, ExpectedJKT: c.Cnf.JKT,
	})
	if err != nil {
		return actor.Actor{}, err
	}
	if proof.NonceStale {
		reqmeta.SetResponseHeader(ctx, headerDPoPNonce, a.dpop.CurrentNonce())
	}
	sub, sid, did := ids.MustParse(c.Subject), ids.MustParse(c.SessionID), ids.MustParse(c.DeviceID)
	st, found, err := a.store.AuthState(ctx, sid)
	if err != nil {
		return actor.Actor{}, err
	}
	out := actor.Actor{ID: sub, Kind: actor.KindUser, SessionID: sid, DeviceID: did, KeyThumbprint: c.Cnf.JKT, DeviceState: actor.DeviceUnknown,
		CredentialExpiresAt: time.Unix(c.Expiry, 0).UTC()}
	if len(c.AMR) == 1 {
		out.AuthMethod = c.AMR[0]
	}
	if !found {
		out.CredentialInactive = true
		return out, nil
	}
	a.describe(&out, st)
	// A revoked session, device or key is a dead credential like a revoked API token: authorize
	// still refuses and audits it, but no stage before it replays a stored answer to it (07 SR7,
	// 06 section 7, M1-45).
	out.CredentialInactive = !out.SessionActive || !out.DeviceKeyActive ||
		out.DeviceState != string(identity.DeviceStateActive) && out.DeviceState != string(identity.DeviceStatePending)
	if out.SessionActive && a.bind != nil {
		ok, err := a.bind.Bound(ctx, st.Session)
		if err != nil {
			return actor.Actor{}, err
		}
		if !ok {
			return actor.Actor{}, a.unbind(ctx, out)
		}
	}
	// An inactive device stays inactive: recording this request as contact would revive it
	// before identity.auth_cleanup revokes it (08 section 5.4).
	if !a.inactive(st) {
		a.touch(ctx, st, creds)
	}
	return out, nil
}

// unbind revokes the session of p whose binding failed (reason trusted_header_mismatch, family
// revoked, session.revoked appended) and records the denial of the route's action with reason
// session_revoked in the same unit of work; it returns 401 session_revoked (08 section 2.4 rule
// 5, ID-T10). The entry carries ids and the cause, never a header value.
func (a *Authenticator) unbind(ctx context.Context, p actor.Actor) error {
	t, _ := reqmeta.TransportOf(ctx)
	act := t.Action
	if act == "" {
		// The gateway reaches the Authenticator without a route (M1-64 names its action).
		act = string(authorize.ActionAuthLoginComplete)
	}
	err := a.uow.Do(ctx, func(ctx context.Context) error {
		if err := a.revoke.RevokeSession(ctx, p.SessionID, identity.RevokeTrustedHeaderMismatch, nil); err != nil {
			return err
		}
		return a.audit.Record(actor.WithActor(ctx, p), auditing.Entry{
			Action: act, Decision: auditing.DecisionDeny, Reason: auditing.ReasonSessionRevoked,
			ResourceKind: resourceAuthSession, ResourceID: p.SessionID,
			Attrs: auditing.Attrs{attrCause: auditing.Code(string(identity.RevokeTrustedHeaderMismatch))},
		})
	})
	if err != nil {
		return err
	}
	return errs.New(errs.CodeSessionRevoked, "")
}

// inactive reports an active device without contact for auth.device_inactive_revoke_after
// (last_seen_at, else approved_at; 08 section 5.4): authentication treats it as inactive before
// the job revokes it, so no request waits for the job (M1-45). Zero switches the check off.
func (a *Authenticator) inactive(st AuthState) bool {
	if a.cfg.DeviceInactiveRevokeAfter <= 0 || st.DeviceState != identity.DeviceStateActive {
		return false
	}
	return inactiveSince(st.DeviceLastSeenAt, st.DeviceApprovedAt, time.Time{}, a.clk.Now().Add(-a.cfg.DeviceInactiveRevokeAfter))
}

// describe fills the actor from the session, device and key read for this request. The session is
// active only when it is not revoked, has not ended and belongs to sub and did; the device state
// counts only for the device of the token, and a pending device past auth.device_pending_ttl is
// revoked, so authorize answers 401 (device_inactive) even before the cleanup job revokes it
// (M1-38); the key is active only when cnf.jkt is the device's active dpop key and the device is
// not inactive (M1-45; authorize then answers device_inactive).
func (a *Authenticator) describe(out *actor.Actor, st AuthState) {
	s := st.Session
	now := a.clk.Now()
	own := s.UserID == out.ID && s.DeviceID == out.DeviceID
	out.SessionActive = own && s.Active(now)
	if own {
		out.DeviceState = string(st.DeviceState)
		if st.DeviceState == identity.DeviceStatePending && !now.Before(st.DeviceCreatedAt.Add(a.cfg.DevicePendingTTL)) {
			out.DeviceState = actor.DeviceRevoked
		}
	}
	out.DeviceKeyActive = own && st.DeviceKeyJKT != "" && st.DeviceKeyJKT == out.KeyThumbprint && !a.inactive(st)
	out.StepUpAt, out.StepUpMethod, out.StepUpConsumedAt = s.StepUpAt, s.StepUpMethod, s.StepUpConsumedAt
	out.AuthMethod, out.ClientComponent = string(s.AuthMethod), string(s.Component)
}

// touch records activity outside the request's unit of work when the session or the device was
// last seen longer than identity.last_active_granularity ago; a failure is logged, never returned.
// In maintenance mode it writes nothing.
func (a *Authenticator) touch(ctx context.Context, st AuthState, creds actor.Credentials) {
	if a.cfg.readOnly() {
		// Maintenance mode drops the side writes (ADR 0065 item 5).
		return
	}
	now := a.clk.Now()
	threshold := now.Add(-a.cfg.LastActiveGranularity)
	if !st.Session.LastSeenAt.IsZero() && !st.Session.LastSeenAt.Before(threshold) &&
		!st.DeviceLastSeenAt.IsZero() && !st.DeviceLastSeenAt.Before(threshold) {
		return
	}
	if err := a.store.Touch(ctx, st.Session.ID, st.Session.DeviceID, creds.ClientIP, now, threshold); err != nil {
		a.log.LogAttrs(ctx, slog.LevelWarn, msgTouchFailed, slog.String(logKeyError, errs.KindOf(err).String()),
			slog.String(logKeyCorrID, reqmeta.CorrelationID(ctx)))
	}
}

// Revalidate implements actor.Authenticator: the anonymous principal passes; a principal of an
// API token passes while the token exists, belongs to it, is not revoked and has not expired
// (else 401 unauthenticated); a person with a session passes while the session is active,
// belongs to it and its device is pending or active with the same key (else 401
// session_revoked); everything else is unauthenticated.
func (a *Authenticator) Revalidate(ctx context.Context, p actor.Actor) error {
	if p.IsAnonymous() {
		return nil
	}
	if !p.TokenID.IsZero() {
		return a.revalidateToken(ctx, p)
	}
	if p.Kind != actor.KindUser || p.SessionID.IsZero() {
		return unauthenticated()
	}
	if p.ClientComponent == string(identity.ComponentConsole) {
		return a.revalidateConsole(ctx, p)
	}
	st, found, err := a.store.AuthState(ctx, p.SessionID)
	if err != nil {
		return err
	}
	if !found {
		return errs.New(errs.CodeSessionRevoked, "")
	}
	check := p
	a.describe(&check, st)
	if !check.SessionActive || !check.DeviceKeyActive ||
		check.DeviceState != actor.DeviceActive && check.DeviceState != actor.DevicePending {
		return errs.New(errs.CodeSessionRevoked, "")
	}
	return nil
}

// revalidateConsole checks a console principal again (M6-01): its session must still be open,
// before its end and on an active device; the console holds no gateway connection, so this only
// guards a principal some caller kept.
func (a *Authenticator) revalidateConsole(ctx context.Context, p actor.Actor) error {
	st, found, err := a.store.AuthState(ctx, p.SessionID)
	if err != nil {
		return err
	}
	s := st.Session
	if !found || s.UserID != p.ID || s.Component != identity.ComponentConsole || !s.Active(a.clk.Now()) ||
		st.DeviceState != identity.DeviceStateActive {
		return errs.New(errs.CodeSessionRevoked, "")
	}
	return nil
}
