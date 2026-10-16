// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"net/url"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/consolesession"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// ConsoleAuthState is the result of the one authentication query of a console request: the
// session found by the HMAC of its cookie, the state of its device and whether its person is
// active.
type ConsoleAuthState struct {
	Session     SessionRecord
	DeviceState identity.DeviceState
	UserActive  bool
}

// ConsoleView is a "View as person" stored on a console session (R-DM-49).
type ConsoleView struct {
	UserID    ids.ID
	StartedAt time.Time
}

// ConsoleStore is the outgoing port of the console session over auth_sessions (implemented by
// adapters/postgres). Reads join the ambient transaction of ctx; writes run only inside a unit of
// work, except TouchConsole. Errors are kernel/errs errors without SQL, cookies or hashes.
type ConsoleStore interface {
	// InsertConsoleSession inserts the console session s with the HMAC of its cookie;
	// last_seen_at starts at s.CreatedAt.
	InsertConsoleSession(ctx context.Context, s SessionRecord, cookieHash []byte) error
	// ConsoleAuthState returns the session whose cookie_hash is hash with its device state and
	// the status of its person; found is false for an unknown hash.
	ConsoleAuthState(ctx context.Context, hash []byte) (st ConsoleAuthState, found bool, err error)
	// SetConsoleCookieHash replaces the cookie hash of the open console session sessionID;
	// rotated is false when there is none.
	SetConsoleCookieHash(ctx context.Context, sessionID ids.ID, hash []byte) (rotated bool, err error)
	// StoreConsoleView stores the view of userID started at at on the open console session
	// sessionID; stored is false when there is none.
	StoreConsoleView(ctx context.Context, sessionID, userID ids.ID, at time.Time) (stored bool, err error)
	// ClearConsoleView ends the stored view of sessionID (only the one started at startedAt when
	// that is not zero) and returns it; cleared is false when there was none.
	ClearConsoleView(ctx context.Context, sessionID ids.ID, startedAt time.Time) (v ConsoleView, cleared bool, err error)
	// TouchConsole sets last_seen_at = at (and last_ip, when valid) on the open session and its
	// device, each only where last_seen_at is unset or older than threshold; one statement
	// outside any unit of work.
	TouchConsole(ctx context.Context, sessionID, deviceID ids.ID, ip netip.Addr, at, threshold time.Time) error
	// DueConsoleSessions returns up to limit open console sessions past their end at now or
	// without a request since idleCutoff, with ids above after (zero for the first page),
	// ascending.
	DueConsoleSessions(ctx context.Context, now, idleCutoff time.Time, after ids.ID, limit int) ([]ids.ID, error)
}

// ConsoleDeps are the collaborators of the console session (M6-01).
type ConsoleDeps struct {
	// Store reads and writes the console columns of auth_sessions.
	Store ConsoleStore
	// Sessions revokes ended console sessions (identity.Auth of M1-34).
	Sessions *Sessions
	// Devices inserts the web device of a console login.
	Devices DeviceStore
	// People reads the person and its sign-in methods.
	People Store
	// Identities reads the linked identities for the step-up method oidc_reauth.
	Identities IdentityStore
	// UoW runs every write; Authorizer decides self.read and access.view_as.end.
	UoW        txn.UnitOfWork
	Authorizer authorize.Authorizer
	// Audit writes the entries of the end of a session and of a view.
	Audit auditing.Recorder
	// Binding checks every request of a trusted_header session (M1-43); nil checks nothing.
	Binding SessionBinding
	// PasskeysEnabled reports a usable WebAuthn relying party (M1-40), for the available methods.
	PasskeysEnabled bool
	// Clock is the time of every check; nil means clock.System(). Random is the source of the
	// cookie values; nil means crypto/rand. Log receives failed activity writes; nil discards.
	Clock  clock.Clock
	Random io.Reader
	Log    *slog.Logger
}

// Console implements the console session of the web console (docs/spec/08-identitaet.md section
// 7.5, brief M6-01): the session start of the console form of every login, the cookie as the
// second credential kind of the Authenticator, the rotation after a step-up, the stored "View as
// person" with its end, the state for getConsoleSession and the expiry of the cleanup job. The
// step-up methods password with TOTP and OIDC re-login come from LocalCredentials and OIDC
// (BindStepUps). It implements identity.Console and is safe for concurrent use once bound.
type Console struct {
	cfg     Config
	timing  consolesession.Timing
	pepper  consolesession.StaticPepper
	origin  string
	d       ConsoleDeps
	stepUps *StepUps
	local   *LocalCredentials
	oidc    *OIDC
}

// Compile-time check: the incoming port.
var _ identity.Console = (*Console)(nil)

// NewConsole returns the console session with auth.console_session_max_age,
// auth.console_session_idle_timeout, authz.view_as_max_duration, auth.step_up_max_age and
// server.public_url of cfg and the raw token pepper. Errors: a missing collaborator, an empty
// pepper, a duration out of range or a public URL without scheme and host.
func NewConsole(cfg Config, pepper []byte, d ConsoleDeps) (*Console, error) {
	if d.Store == nil || d.Sessions == nil || d.Devices == nil || d.People == nil || d.Identities == nil || d.UoW == nil ||
		d.Authorizer == nil || d.Audit == nil || len(pepper) == 0 {
		return nil, errors.New("identity: NewConsole needs every collaborator and the token pepper")
	}
	t := consolesession.Timing{MaxAge: cfg.ConsoleSessionMaxAge, IdleTimeout: cfg.ConsoleSessionIdleTimeout}
	if err := t.Validate(); err != nil {
		return nil, err
	}
	if cfg.ViewAsMaxDuration <= 0 || cfg.StepUpMaxAge <= 0 {
		return nil, errors.New("identity: the console needs authz.view_as_max_duration and auth.step_up_max_age above 0")
	}
	u, err := url.Parse(cfg.PublicURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("identity: server.public_url must be an absolute URL")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Random == nil {
		d.Random = rand.Reader
	}
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	return &Console{
		cfg: cfg, timing: t, pepper: consolesession.StaticPepper(append([]byte(nil), pepper...)), origin: u.Scheme + "://" + u.Host, d: d,
	}, nil
}

// BindStepUps hands the console its step-up use cases: the challenges and the rotation (StepUps),
// password with TOTP (LocalCredentials, nil while BindLocal has not run) and the OIDC re-login
// (OIDC, nil while BindOIDC has not run). internal/wire calls it once while it builds identity.
func (c *Console) BindStepUps(s *StepUps, local *LocalCredentials, o *OIDC) {
	if s != nil {
		c.stepUps = s
	}
	if local != nil {
		c.local = local
	}
	if o != nil {
		c.oidc = o
	}
}

// The console callback paths on the origin of server.public_url (D-M6-01-1, D-M6-01-6).
const (
	consoleLoginCallback  = "/console/login/callback"
	consoleStepUpCallback = "/console/step-up/callback"
)

// stepUpCallback returns the console step-up callback URL of this server.
func (c *Console) stepUpCallback() string { return c.origin + consoleStepUpCallback }

// Audit values of the console (codes and IDs only, SEC-AUD-5).
const (
	attrClientComponent = "client_component"
	attrRevokeReasonKey = "revoke_reason"
	attrEndCause        = "cause"
	attrTargetUserID    = "target_user_id"
	attrStartedAt       = "started_at"
	attrEndedReason     = "ended_reason"
	causeAbsolute       = "absolute"
	causeIdle           = "idle"
	viewEnded           = "ended"
	viewExpired         = "expired"
	// reasonConsoleStepUpRequired is the reason of the 401 of the pre-step-up gate (D-M6-01-8).
	reasonConsoleStepUpRequired = "console_step_up_required"
	// headerSetCookie carries the clearing cookie of a refused cookie.
	headerSetCookie = "Set-Cookie"
	// msgConsoleTouchFailed is logged when last_seen_at could not be written.
	msgConsoleTouchFailed = "recording console activity failed"
)

// ConsoleStart is the input of Start: who logged in and how.
type ConsoleStart struct {
	UserID     ids.ID
	AuthMethod identity.AuthMethod
	IdentityID *ids.ID
	ClientIP   netip.Addr
}

// Start registers the web device of a console login and starts its console session with a new
// cookie, inside the ambient unit of work (CompleteLogin calls it, D-M6-01-1). It issues no token
// and no refresh family. It returns the login result and the device. Errors: the store errors and
// internal for a failing random source.
func (c *Console) Start(ctx context.Context, in ConsoleStart) (ConsoleStarted, error) {
	now := c.d.Clock.Now().Truncate(time.Microsecond)
	dev, err := c.d.Devices.InsertDevice(ctx, consolesession.WebDevice(ids.New(), in.UserID, now))
	if err != nil {
		return ConsoleStarted{}, err
	}
	value, err := consolesession.NewCookieValue(c.d.Random)
	if err != nil {
		return ConsoleStarted{}, errs.Wrap(errs.CodeInternalError, err, "identity: random source")
	}
	sess := SessionRecord{
		ID: ids.New(), UserID: in.UserID, DeviceID: dev.ID, Component: identity.ComponentConsole, AuthMethod: in.AuthMethod,
		CreatedAt: now, ExpiresAt: c.timing.ExpiresAt(now), LastIP: in.ClientIP, LastSeenAt: now,
	}
	if in.IdentityID != nil {
		sess.IdentityID = *in.IdentityID
	}
	if err := c.d.Store.InsertConsoleSession(ctx, sess, consolesession.CookieHash(c.pepper, value)); err != nil {
		return ConsoleStarted{}, err
	}
	view, err := c.view(ctx, sess)
	if err != nil {
		return ConsoleStarted{}, err
	}
	return ConsoleStarted{
		Login:   identity.ConsoleLogin{Session: view, Cookie: identity.NewToken(value), CookieMaxAge: sess.ExpiresAt.Sub(now)},
		Session: sessionView(sess), Device: dev,
	}, nil
}

// ConsoleStarted is the result of Start: the answer of the login, the new session and the web
// device.
type ConsoleStarted struct {
	Login   identity.ConsoleLogin
	Session identity.Session
	Device  identity.Device
}

// rotate gives the open console session sess a new cookie value inside the ambient unit of work
// (session fixation, D-M6-01-2): the old value stops working at commit.
func (c *Console) rotate(ctx context.Context, sess SessionRecord) (identity.Token, time.Duration, error) {
	value, err := consolesession.NewCookieValue(c.d.Random)
	if err != nil {
		return identity.Token{}, 0, errs.Wrap(errs.CodeInternalError, err, "identity: random source")
	}
	ok, err := c.d.Store.SetConsoleCookieHash(ctx, sess.ID, consolesession.CookieHash(c.pepper, value))
	if err != nil {
		return identity.Token{}, 0, err
	}
	if !ok {
		return identity.Token{}, 0, errs.New(errs.CodeSessionRevoked, "")
	}
	return identity.NewToken(value), sess.ExpiresAt.Sub(c.d.Clock.Now()), nil
}

// cookieRefused is a 401 of a console cookie with the clearing cookie (D-M6-01-2).
func cookieRefused(code errs.Code) error {
	return errs.New(code, "").WithHeader(headerSetCookie, consolesession.ClearCookie())
}

// Authenticate authenticates the console cookie of creds (D-M6-01-4): one query by the HMAC of
// the value; the session must be a console session, open, before its absolute end, not idle, on
// an active device of an active person and, for trusted_header, still bound to its header login.
// An expired or idle session is revoked with reason expired and audited once. A stored view past
// authz.view_as_max_duration is ended with its audit entry and the request gets 401
// step_up_required; the pre-step-up gate refuses every action but the few of D-M6-01-8 with 401
// step_up_required. Unknown, revoked and ended cookies get 401 with the clearing cookie. The actor
// carries ClientComponent console, no key binding and the stored view.
func (c *Console) Authenticate(ctx context.Context, creds actor.Credentials) (actor.Actor, error) {
	st, found, err := c.d.Store.ConsoleAuthState(ctx, consolesession.CookieHash(c.pepper, creds.ConsoleCookie.Reveal()))
	if err != nil {
		return actor.Actor{}, err
	}
	if !found || st.Session.Component != identity.ComponentConsole {
		return actor.Actor{}, cookieRefused(errs.CodeUnauthenticated)
	}
	s := st.Session
	now := c.d.Clock.Now()
	switch {
	case !s.RevokedAt.IsZero():
		return actor.Actor{}, cookieRefused(errs.CodeSessionRevoked)
	case c.timing.Expired(s.ExpiresAt, now):
		return actor.Actor{}, c.expire(ctx, s.ID, causeAbsolute)
	case c.timing.Idle(s.LastSeenAt, now):
		return actor.Actor{}, c.expire(ctx, s.ID, causeIdle)
	case st.DeviceState != identity.DeviceStateActive || !st.UserActive:
		return actor.Actor{}, cookieRefused(errs.CodeSessionRevoked)
	}
	if err := c.bound(ctx, s); err != nil {
		return actor.Actor{}, err
	}
	if !s.ViewAsUserID.IsZero() && !consolesession.ViewActive(s.ViewAsStartedAt, now, c.cfg.ViewAsMaxDuration) {
		return actor.Actor{}, c.viewExpired(ctx, s)
	}
	if t, _ := reqmeta.TransportOf(ctx); !consolesession.PassesGate(t.Action, s.StepUpAt) {
		return actor.Actor{}, errs.New(errs.CodeStepUpRequired, "").WithReason(reasonConsoleStepUpRequired)
	}
	c.touch(ctx, s, creds.ClientIP, now)
	out := actor.Actor{
		ID: s.UserID, Kind: actor.KindUser, SessionID: s.ID, DeviceID: s.DeviceID, SessionActive: true,
		DeviceState: actor.DeviceActive, DeviceKeyActive: true, StepUpAt: s.StepUpAt, StepUpMethod: s.StepUpMethod,
		StepUpConsumedAt: s.StepUpConsumedAt, AuthMethod: string(s.AuthMethod), ClientComponent: string(identity.ComponentConsole),
		CredentialExpiresAt: s.ExpiresAt,
	}
	if !s.ViewAsUserID.IsZero() {
		out.ViewAs = &actor.ViewAs{UserID: s.ViewAsUserID, StartedAt: s.ViewAsStartedAt}
	}
	return out, nil
}

// bound checks the header login of a trusted_header console session on every request (08 section
// 2.4 rule 5, M1-43): a mismatch revokes the session with trusted_header_mismatch and answers 401
// session_revoked with the clearing cookie.
func (c *Console) bound(ctx context.Context, s SessionRecord) error {
	if c.d.Binding == nil || s.AuthMethod != identity.AuthTrustedHeader {
		return nil
	}
	ok, err := c.d.Binding.Bound(ctx, s)
	if err != nil || ok {
		return err
	}
	err = c.d.UoW.Do(context.WithoutCancel(ctx), func(ctx context.Context) error {
		n, err := c.revokeOne(ctx, s.ID, identity.RevokeTrustedHeaderMismatch)
		if err != nil || n == 0 {
			return err
		}
		return c.d.Audit.Record(ctx, auditing.Entry{
			Action: string(authorize.ActionSessionRevoke), Decision: auditing.DecisionAllow, ResourceKind: resourceAuthSession, ResourceID: s.ID,
			Attrs: auditing.Attrs{attrRevokeReasonKey: auditing.Code(string(identity.RevokeTrustedHeaderMismatch))},
		})
	})
	if err != nil {
		return err
	}
	return cookieRefused(errs.CodeSessionRevoked)
}

// revokeOne revokes the session id with reason when it is still open and returns how many
// sessions it revoked (0 or 1, the transition is guarded by revoked_at IS NULL).
func (c *Console) revokeOne(ctx context.Context, id ids.ID, reason identity.SessionRevokeReason) (int, error) {
	tree, err := c.d.Sessions.d.Store.LockSessionTree(ctx, id)
	if err != nil {
		return 0, err
	}
	return c.d.Sessions.revokeCounted(ctx, tree, reason, nil)
}

// expire revokes the console session id with reason expired and writes one audit entry of the
// system action session.revoke (cause absolute or idle) when this call made the transition; a
// concurrent request or the cleanup job that won writes it instead (D-M6-01-3). It returns the 401
// session_revoked with the clearing cookie, or the error of the unit of work.
func (c *Console) expire(ctx context.Context, id ids.ID, cause string) error {
	err := c.d.UoW.Do(context.WithoutCancel(ctx), func(ctx context.Context) error {
		_, err := c.expireLocked(ctx, id, cause)
		return err
	})
	if err != nil {
		return err
	}
	return cookieRefused(errs.CodeSessionRevoked)
}

// expireLocked is the body of expire inside the ambient unit of work; ended reports the
// transition.
func (c *Console) expireLocked(ctx context.Context, id ids.ID, cause string) (bool, error) {
	n, err := c.revokeOne(ctx, id, identity.RevokeExpired)
	if err != nil || n == 0 {
		return false, err
	}
	return true, c.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionSessionRevoke), Decision: auditing.DecisionAllow, ResourceKind: resourceAuthSession, ResourceID: id,
		Attrs: auditing.Attrs{attrRevokeReasonKey: auditing.Code(string(identity.RevokeExpired)), attrEndCause: auditing.Code(cause)},
	})
}

// viewExpired ends the stored view of s that reached authz.view_as_max_duration in its own short
// unit of work with the end entry (ended_reason expired) and answers 401 step_up_required, so no
// request sent from a page that showed the other person's view runs with the owner's rights (07
// section 9.2, D-M6-01-7).
func (c *Console) viewExpired(ctx context.Context, s SessionRecord) error {
	err := c.d.UoW.Do(context.WithoutCancel(ctx), func(ctx context.Context) error {
		return c.endView(actor.WithActor(ctx, consoleOwner(s)), s.ID, s.ViewAsStartedAt, viewExpired)
	})
	if err != nil {
		return err
	}
	return errs.New(errs.CodeStepUpRequired, "")
}

// consoleOwner is the principal of the console session s for the audit entries the authenticator
// writes before the chain has an actor.
func consoleOwner(s SessionRecord) actor.Actor {
	return actor.Actor{ID: s.UserID, Kind: actor.KindUser, SessionID: s.ID, DeviceID: s.DeviceID, ClientComponent: string(identity.ComponentConsole)}
}

// endView clears the view of the session sid (only the one started at startedAt when that is not
// zero) and, when there was one, writes the allow entry of access.view_as.end with the target, the
// start and the reason; IDs and codes only.
func (c *Console) endView(ctx context.Context, sid ids.ID, startedAt time.Time, reason string) error {
	v, cleared, err := c.d.Store.ClearConsoleView(ctx, sid, startedAt)
	if err != nil || !cleared {
		return err
	}
	return c.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionAccessViewAsEnd), Decision: auditing.DecisionAllow, ResourceKind: resourceAuthSession, ResourceID: sid,
		Attrs: auditing.Attrs{
			attrTargetUserID: auditing.ID(v.UserID), attrStartedAt: auditing.Time(v.StartedAt), attrEndedReason: auditing.Code(reason),
		},
	})
}

// touch writes last_seen_at outside the request's unit of work when it is older than the write
// bound (D-M6-01-3); a failure is logged with its kind only.
func (c *Console) touch(ctx context.Context, s SessionRecord, ip netip.Addr, now time.Time) {
	if !c.timing.NeedsTouch(s.LastSeenAt, now) {
		return
	}
	if err := c.d.Store.TouchConsole(ctx, s.ID, s.DeviceID, ip, now, now.Add(-c.timing.WriteBound())); err != nil {
		c.d.Log.LogAttrs(ctx, slog.LevelWarn, msgConsoleTouchFailed, slog.String(logKeyError, errs.KindOf(err).String()))
	}
}

// ConsoleViews implements authz.ConsoleViews over the console store (R-DM-49, M6-01):
// viewAsUser from a console session stores its view on the session.
type ConsoleViews struct{ store ConsoleStore }

// Compile-time check: the port of authz.
var _ authz.ConsoleViews = ConsoleViews{}

// NewConsoleViews returns the view store over store.
func NewConsoleViews(store ConsoleStore) ConsoleViews { return ConsoleViews{store: store} }

// StartView implements authz.ConsoleViews inside the ambient unit of work: it stores the view of
// userID, started at at, on the console session sessionID. Errors: 401 session_revoked when the
// session is no longer an open console session.
func (v ConsoleViews) StartView(ctx context.Context, sessionID, userID ids.ID, at time.Time) error {
	ok, err := v.store.StoreConsoleView(ctx, sessionID, userID, at)
	if err != nil {
		return err
	}
	if !ok {
		return errs.New(errs.CodeSessionRevoked, "")
	}
	return nil
}

// fieldEnrollmentCode is the field pointer of the enrollment code of a login request.
const fieldEnrollmentCode = "/enrollment_code"

// consoleFormRefused is the 400 of a member the console form does not take (enrollment codes and
// device pairing never take the console form, D-M6-01-1).
func consoleFormRefused(pointer string) error {
	return errs.New(errs.CodeValidationFailed, "").
		WithFields(errs.FieldError{Pointer: pointer, Code: "not_allowed", Detail: "The console form does not take this member."})
}

// notConsole is the 403 of a console operation called from any other session or credential.
func notConsole() error { return errs.New(errs.CodeForbidden, "") }

// own is the resource auth_session of the request's own session.
func own(a actor.Actor) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceAuthSession, ID: a.SessionID, OwnerID: a.ID}
}

// check authorizes act on r for a and returns the decision as an error.
func (c *Console) check(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := c.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// EndViewAs implements identity.Console (endViewAs, access.view_as.end on the own console
// session, R-AZ-32): authorize, then the stored view ends with the entry ended_reason ended; a
// session without a view changes nothing.
func (c *Console) EndViewAs(ctx context.Context, a actor.Actor) error {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return err
	}
	if err := c.check(ctx, a, authorize.ActionAccessViewAsEnd, own(a)); err != nil {
		return err
	}
	if a.ClientComponent != string(identity.ComponentConsole) {
		return notConsole()
	}
	return c.d.UoW.Do(ctx, func(ctx context.Context) error {
		return c.endView(ctx, a.SessionID, time.Time{}, viewEnded)
	})
}

// GetConsoleSession implements identity.Console (getConsoleSession, self.read).
func (c *Console) GetConsoleSession(ctx context.Context, a actor.Actor) (identity.ConsoleSession, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.ConsoleSession{}, err
	}
	if err := c.check(ctx, a, authorize.ActionSelfRead, authorize.Resource{Kind: authorize.ResourcePrincipal, ID: a.ID, OwnerID: a.ID}); err != nil {
		return identity.ConsoleSession{}, err
	}
	if a.ClientComponent != string(identity.ComponentConsole) {
		return identity.ConsoleSession{}, notConsole()
	}
	s, found, err := c.d.Sessions.d.Store.Session(ctx, a.SessionID)
	if err != nil {
		return identity.ConsoleSession{}, err
	}
	if !found {
		return identity.ConsoleSession{}, errs.New(errs.CodeSessionRevoked, "")
	}
	return c.view(ctx, s)
}

// view returns the state of the console session s as the console reads it.
func (c *Console) view(ctx context.Context, s SessionRecord) (identity.ConsoleSession, error) {
	rec, found, err := c.d.People.User(ctx, s.UserID)
	if err != nil {
		return identity.ConsoleSession{}, err
	}
	if !found {
		return identity.ConsoleSession{}, invisible()
	}
	now := c.d.Clock.Now()
	out := identity.ConsoleSession{
		UserID: s.UserID, DisplayName: rec.User.Principal.DisplayName.String(), ExpiresAt: s.ExpiresAt, IdleTimeout: c.timing.IdleTimeout,
		StepUpAt: s.StepUpAt, StepUpMethod: s.StepUpMethod,
	}
	if until := s.StepUpAt.Add(c.cfg.StepUpMaxAge); !s.StepUpAt.IsZero() && until.After(now) {
		out.StepUpValidUntil = until
	}
	if out.MethodsAvailable, err = c.methods(ctx, s.UserID); err != nil {
		return identity.ConsoleSession{}, err
	}
	if !s.ViewAsUserID.IsZero() {
		p, found, err := c.d.People.Principal(ctx, s.ViewAsUserID)
		if err != nil {
			return identity.ConsoleSession{}, err
		}
		v := &identity.ConsoleViewAs{UserID: s.ViewAsUserID, StartedAt: s.ViewAsStartedAt,
			EndsAt: consolesession.ViewEndsAt(s.ViewAsStartedAt, c.cfg.ViewAsMaxDuration)}
		if found {
			v.DisplayName = p.Principal.DisplayName.String()
		}
		out.ViewAs = v
	}
	return out, nil
}

// methods returns the step-up methods the person userID can use in the console (08 section 6.2):
// passkey with an active passkey and a relying party, oidc_reauth with an identity at an enabled
// OIDC provider, password_totp with an established local credential while
// auth.local_password_enabled is on.
func (c *Console) methods(ctx context.Context, userID ids.ID) ([]string, error) {
	out := []string{}
	if c.d.PasskeysEnabled {
		ok, err := c.d.People.HasActivePasskey(ctx, userID)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, identity.StepUpMethodPasskey)
		}
	}
	idents, err := c.d.Identities.ActiveIdentitiesOfUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, i := range idents {
		if i.ProviderKind == identity.ProviderOIDC && i.ProviderEnabled {
			out = append(out, identity.StepUpMethodOIDCReauth)
			break
		}
	}
	if c.cfg.LocalPasswordEnabled {
		ok, err := c.d.People.HasEstablishedLocalCredential(ctx, userID)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, identity.StepUpMethodPasswordTOTP)
		}
	}
	return out, nil
}

// consoleSweepBatch bounds one page of the console sweep of the cleanup job.
const consoleSweepBatch = 100

// ExpireConsoleSessions revokes every open console session past auth.console_session_max_age or
// idle by the rule of D-M6-01-3, each in its own unit of work with the same audit entry a request
// writes, and returns how many it revoked (the job identity.auth_cleanup). Errors: the first
// failing unit; the next run continues.
func (c *Console) ExpireConsoleSessions(ctx context.Context) (int, error) {
	now := c.d.Clock.Now()
	cutoff := now.Add(-(c.timing.IdleTimeout + c.timing.WriteBound()))
	total := 0
	var after ids.ID
	for {
		list, err := c.d.Store.DueConsoleSessions(ctx, now, cutoff, after, consoleSweepBatch)
		if err != nil {
			return total, err
		}
		for _, id := range list {
			ended, err := c.expireDue(ctx, id, now)
			if err != nil {
				return total, err
			}
			if ended {
				total++
			}
		}
		if len(list) < consoleSweepBatch {
			return total, nil
		}
		after = list[len(list)-1]
	}
}

// expireDue revokes the console session id in its own unit of work when it is still past its
// end or idle at now (a request may have touched it since the selection); ended reports the
// transition.
func (c *Console) expireDue(ctx context.Context, id ids.ID, now time.Time) (bool, error) {
	var ended bool
	err := c.d.UoW.Do(ctx, func(ctx context.Context) error {
		s, found, err := c.d.Sessions.d.Store.Session(ctx, id)
		if err != nil || !found {
			return err
		}
		cause := causeIdle
		switch {
		case c.timing.Expired(s.ExpiresAt, now):
			cause = causeAbsolute
		case !c.timing.Idle(s.LastSeenAt, now):
			return nil
		}
		ended, err = c.expireLocked(ctx, id, cause)
		return err
	})
	return ended, err
}
