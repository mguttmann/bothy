// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/netip"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/consolesession"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/device"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// errStoreDown is the store failure the fakes return; the use cases must hand it on unchanged.
var errStoreDown = errs.New(errs.CodeInternalError, "store down")

// consoleNow is the time of the fake clock of these tests.
var consoleNow = time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)

// fakeConsoleStore is a ConsoleStore with canned answers; DueConsoleSessions hands out pages.
type fakeConsoleStore struct {
	ConsoleStore
	st        ConsoleAuthState
	found     bool
	authErr   error
	insertErr error
	rotated   bool
	rotateErr error
	stored    bool
	storeErr  error
	view      ConsoleView
	cleared   bool
	clearErr  error
	touchErr  error
	touched   int
	pages     [][]ids.ID
	dueErr    error
	afters    []ids.ID
}

func (f *fakeConsoleStore) InsertConsoleSession(context.Context, SessionRecord, []byte) error {
	return f.insertErr
}

func (f *fakeConsoleStore) ConsoleAuthState(context.Context, []byte) (ConsoleAuthState, bool, error) {
	return f.st, f.found, f.authErr
}

func (f *fakeConsoleStore) SetConsoleCookieHash(context.Context, ids.ID, []byte) (bool, error) {
	return f.rotated, f.rotateErr
}

func (f *fakeConsoleStore) StoreConsoleView(context.Context, ids.ID, ids.ID, time.Time) (bool, error) {
	return f.stored, f.storeErr
}

func (f *fakeConsoleStore) ClearConsoleView(context.Context, ids.ID, time.Time) (ConsoleView, bool, error) {
	return f.view, f.cleared, f.clearErr
}

func (f *fakeConsoleStore) TouchConsole(context.Context, ids.ID, ids.ID, netip.Addr, time.Time, time.Time) error {
	f.touched++
	return f.touchErr
}

func (f *fakeConsoleStore) DueConsoleSessions(_ context.Context, _, _ time.Time, after ids.ID, _ int) ([]ids.ID, error) {
	f.afters = append(f.afters, after)
	if f.dueErr != nil {
		return nil, f.dueErr
	}
	if len(f.pages) == 0 {
		return nil, nil
	}
	page := f.pages[0]
	f.pages = f.pages[1:]
	return page, nil
}

// fakeSessionStore is a SessionStore with canned answers for the reads and the lock of the console.
type fakeSessionStore struct {
	SessionStore
	sess      SessionRecord
	found     bool
	sessErr   error
	tree      []SessionRecord
	treeErr   error
	treeCalls int
	auth      AuthState
	authFound bool
	authErr   error
}

func (f *fakeSessionStore) Session(context.Context, ids.ID) (SessionRecord, bool, error) {
	return f.sess, f.found, f.sessErr
}

func (f *fakeSessionStore) LockSessionTree(context.Context, ids.ID) ([]SessionRecord, error) {
	f.treeCalls++
	return f.tree, f.treeErr
}

func (f *fakeSessionStore) AuthState(context.Context, ids.ID) (AuthState, bool, error) {
	return f.auth, f.authFound, f.authErr
}

// fakeDeviceStore is a DeviceStore whose InsertDevice and Device answer canned values.
type fakeDeviceStore struct {
	DeviceStore
	dev   identity.Device
	found bool
	err   error
	// stepUpKey reports an active step_up key of the device (ActiveKey); keyErr fails the read.
	stepUpKey bool
	keyErr    error
}

func (f *fakeDeviceStore) ActiveKey(_ context.Context, deviceID ids.ID, purpose identity.KeyPurpose) (identity.DeviceKey, bool, error) {
	if f.keyErr != nil {
		return identity.DeviceKey{}, false, f.keyErr
	}
	if purpose != identity.KeyPurposeStepUp || !f.stepUpKey {
		return identity.DeviceKey{}, false, nil
	}
	return identity.DeviceKey{DeviceID: deviceID, Purpose: purpose, Alg: "ES256"}, true, nil
}

func (f *fakeDeviceStore) InsertDevice(_ context.Context, d device.Device) (identity.Device, error) {
	if f.err != nil {
		return identity.Device{}, f.err
	}
	return identity.Device{ID: d.ID}, nil
}

func (f *fakeDeviceStore) Device(context.Context, ids.ID) (identity.Device, bool, error) {
	return f.dev, f.found, f.err
}

// fakePeople is a Store with the reads of the console session view.
type fakePeople struct {
	Store
	user           UserRecord
	userFound      bool
	userErr        error
	principal      PrincipalRecord
	principalFound bool
	principalErr   error
	passkey        bool
	passkeyErr     error
	local          bool
	localErr       error
}

func (f *fakePeople) User(context.Context, ids.ID) (UserRecord, bool, error) {
	return f.user, f.userFound, f.userErr
}

func (f *fakePeople) Principal(context.Context, ids.ID) (PrincipalRecord, bool, error) {
	return f.principal, f.principalFound, f.principalErr
}

func (f *fakePeople) HasActivePasskey(context.Context, ids.ID) (bool, error) {
	return f.passkey, f.passkeyErr
}

func (f *fakePeople) HasEstablishedLocalCredential(context.Context, ids.ID) (bool, error) {
	return f.local, f.localErr
}

// fakeIdentities is an IdentityStore with the identity reads of the console.
type fakeIdentities struct {
	IdentityStore
	list      []IdentityRecord
	listErr   error
	bySubject identity.Identity
	found     bool
	subjErr   error
}

func (f *fakeIdentities) ActiveIdentitiesOfUser(context.Context, ids.ID) ([]IdentityRecord, error) {
	return f.list, f.listErr
}

func (f *fakeIdentities) ActiveIdentityBySubject(context.Context, ids.ID, string) (identity.Identity, bool, error) {
	return f.bySubject, f.found, f.subjErr
}

// runUoW runs fn directly, or fails with err without running it.
type runUoW struct{ err error }

func (u runUoW) Do(ctx context.Context, fn func(ctx context.Context) error, _ ...txn.Option) error {
	if u.err != nil {
		return u.err
	}
	return fn(ctx)
}

// fixedAuthorizer answers every Authorize call with d and err.
type fixedAuthorizer struct {
	authorize.Authorizer
	d   authorize.Decision
	err error
}

func (f fixedAuthorizer) Authorize(context.Context, authorize.Principal, authorize.Action, authorize.Resource) (authorize.Decision, error) {
	return f.d, f.err
}

// allowAll is the authorizer that allows everything.
var allowAll = fixedAuthorizer{d: authorize.Decision{Outcome: authorize.OutcomeAllow}}

// fakeAudit records the entries and denials; err fails Record.
type fakeAudit struct {
	entries []auditing.Entry
	denials []auditing.Entry
	err     error
}

func (f *fakeAudit) Record(_ context.Context, e auditing.Entry) error {
	if f.err != nil {
		return f.err
	}
	f.entries = append(f.entries, e)
	return nil
}

func (f *fakeAudit) RecordDenial(_ context.Context, e auditing.Entry) error {
	f.denials = append(f.denials, e)
	return nil
}

func (f *fakeAudit) RecordDetached(ctx context.Context, e auditing.Entry) error {
	return f.Record(ctx, e)
}

// fixedBinding answers Bound with ok and err.
type fixedBinding struct {
	ok  bool
	err error
}

func (b fixedBinding) Bound(context.Context, SessionRecord) (bool, error) { return b.ok, b.err }

// consoleFakes are the collaborators of a test console.
type consoleFakes struct {
	store      *fakeConsoleStore
	sessions   *fakeSessionStore
	devices    *fakeDeviceStore
	people     *fakePeople
	identities *fakeIdentities
	audit      *fakeAudit
	log        *bytes.Buffer
}

// newTestConsole returns a console over fresh fakes with a fake clock at consoleNow, a working
// unit of work and an authorizer that allows everything; change c.d before the call under test.
func newTestConsole(t *testing.T) (*Console, *consoleFakes) {
	t.Helper()
	f := &consoleFakes{
		store: &fakeConsoleStore{}, sessions: &fakeSessionStore{}, devices: &fakeDeviceStore{},
		people: &fakePeople{userFound: true}, identities: &fakeIdentities{}, audit: &fakeAudit{}, log: &bytes.Buffer{},
	}
	clk := clock.NewFake(consoleNow)
	c := &Console{
		cfg:    Config{ViewAsMaxDuration: time.Hour, StepUpMaxAge: 5 * time.Minute},
		timing: consolesession.Timing{MaxAge: 12 * time.Hour, IdleTimeout: 30 * time.Minute},
		pepper: consolesession.StaticPepper("console pepper"),
		origin: "https://bothy.example.com",
		d: ConsoleDeps{
			Store: f.store, Sessions: &Sessions{d: SessionsDeps{Store: f.sessions, Clock: clk}}, Devices: f.devices,
			People: f.people, Identities: f.identities, UoW: runUoW{}, Authorizer: allowAll, Audit: f.audit,
			Clock: clk, Random: rand.Reader, Log: slog.New(slog.NewTextHandler(f.log, nil)),
		},
	}
	return c, f
}

// consoleActor is a person on a console session.
func consoleActor() actor.Actor {
	return actor.Actor{ID: ids.New(), Kind: actor.KindUser, SessionID: ids.New(), DeviceID: ids.New(), ClientComponent: string(identity.ComponentConsole)}
}

// wantCode fails t unless err is a kernel error with code want.
func wantCode(t *testing.T, err error, want errs.Code) {
	t.Helper()
	_ = problemOf(t, err, want)
}

// problemOf fails t unless err is a kernel error with code want and returns it.
func problemOf(t *testing.T, err error, want errs.Code) *errs.Error {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != want {
		t.Fatalf("err = %v, want code %s", err, want)
	}
	return e
}

// wantStoreDown fails t unless err is errStoreDown handed on unchanged.
func wantStoreDown(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, errStoreDown) {
		t.Fatalf("err = %v, want the store error unchanged", err)
	}
}

// wantClearingCookie fails t unless e carries the clearing cookie.
func wantClearingCookie(t *testing.T, e *errs.Error) {
	t.Helper()
	if got := e.Headers.Get(headerSetCookie); got != consolesession.ClearCookie() {
		t.Fatalf("Set-Cookie = %q, want the clearing cookie", got)
	}
}

// validConsoleDeps are collaborators that pass the checks of NewConsole.
func validConsoleDeps() ConsoleDeps {
	return ConsoleDeps{
		Store: &fakeConsoleStore{}, Sessions: &Sessions{}, Devices: &fakeDeviceStore{}, People: &fakePeople{},
		Identities: &fakeIdentities{}, UoW: runUoW{}, Authorizer: allowAll, Audit: &fakeAudit{},
	}
}

// validConsoleConfig is a configuration that passes the checks of NewConsole.
func validConsoleConfig() Config {
	return Config{
		ConsoleSessionMaxAge: 12 * time.Hour, ConsoleSessionIdleTimeout: 30 * time.Minute, ViewAsMaxDuration: time.Hour,
		StepUpMaxAge: 5 * time.Minute, PublicURL: "https://bothy.example.com/base",
	}
}

// TestNewConsoleRefusesInvalidInput: a missing collaborator, an empty pepper, a duration out of
// range and a public URL without scheme and host are refused.
func TestNewConsoleRefusesInvalidInput(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*Config, *ConsoleDeps, *[]byte){
		"no store":         func(_ *Config, d *ConsoleDeps, _ *[]byte) { d.Store = nil },
		"no audit":         func(_ *Config, d *ConsoleDeps, _ *[]byte) { d.Audit = nil },
		"empty pepper":     func(_ *Config, _ *ConsoleDeps, p *[]byte) { *p = nil },
		"no max age":       func(c *Config, _ *ConsoleDeps, _ *[]byte) { c.ConsoleSessionMaxAge = 0 },
		"no view duration": func(c *Config, _ *ConsoleDeps, _ *[]byte) { c.ViewAsMaxDuration = 0 },
		"no step-up age":   func(c *Config, _ *ConsoleDeps, _ *[]byte) { c.StepUpMaxAge = 0 },
		"relative url":     func(c *Config, _ *ConsoleDeps, _ *[]byte) { c.PublicURL = "/console" },
		"broken url":       func(c *Config, _ *ConsoleDeps, _ *[]byte) { c.PublicURL = "http://[::1" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cfg, d, pepper := validConsoleConfig(), validConsoleDeps(), []byte("pepper")
			change(&cfg, &d, &pepper)
			if c, err := NewConsole(cfg, pepper, d); err == nil || c != nil {
				t.Fatalf("NewConsole = %v, %v, want an error", c, err)
			}
		})
	}
}

// TestNewConsoleDefaults: without clock, random source and log the console uses the system ones,
// and its step-up callback lies on the origin of server.public_url.
func TestNewConsoleDefaults(t *testing.T) {
	t.Parallel()
	c, err := NewConsole(validConsoleConfig(), []byte("pepper"), validConsoleDeps())
	if err != nil {
		t.Fatal(err)
	}
	if c.d.Clock == nil || c.d.Random == nil || c.d.Log == nil {
		t.Fatalf("defaults not set: %+v", c.d)
	}
	if got := c.stepUpCallback(); got != "https://bothy.example.com"+consoleStepUpCallback {
		t.Fatalf("stepUpCallback = %q", got)
	}
}

// TestConsoleStartErrors: Start hands on the store errors unchanged and answers a failing random
// source with internal_error.
func TestConsoleStartErrors(t *testing.T) {
	t.Parallel()
	in := ConsoleStart{UserID: ids.New(), AuthMethod: identity.AuthOIDC}
	t.Run("device insert", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		f.devices.err = errStoreDown
		_, err := c.Start(context.Background(), in)
		wantStoreDown(t, err)
	})
	t.Run("random source", func(t *testing.T) {
		t.Parallel()
		c, _ := newTestConsole(t)
		c.d.Random = iotest.ErrReader(errors.New("no entropy"))
		_, err := c.Start(context.Background(), in)
		wantCode(t, err, errs.CodeInternalError)
	})
	t.Run("session insert", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		f.store.insertErr = errStoreDown
		_, err := c.Start(context.Background(), in)
		wantStoreDown(t, err)
	})
	t.Run("view", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		f.people.userErr = errStoreDown
		_, err := c.Start(context.Background(), in)
		wantStoreDown(t, err)
	})
}

// TestConsoleStartWithIdentity: Start keeps the identity of the login on the session and answers
// a cookie that lives until the absolute end.
func TestConsoleStartWithIdentity(t *testing.T) {
	t.Parallel()
	c, _ := newTestConsole(t)
	identityID := ids.New()
	got, err := c.Start(context.Background(), ConsoleStart{UserID: ids.New(), AuthMethod: identity.AuthOIDC, IdentityID: &identityID})
	if err != nil {
		t.Fatal(err)
	}
	if got.Login.Cookie.IsZero() || got.Login.CookieMaxAge != c.timing.MaxAge {
		t.Fatalf("login = %+v, want a cookie for %v", got.Login, c.timing.MaxAge)
	}
}

// TestConsoleRotateErrors: the rotation answers a failing random source with internal_error, hands
// on a store error and answers a session that is no longer open with session_revoked.
func TestConsoleRotateErrors(t *testing.T) {
	t.Parallel()
	sess := SessionRecord{ID: ids.New(), ExpiresAt: consoleNow.Add(time.Hour)}
	t.Run("random source", func(t *testing.T) {
		t.Parallel()
		c, _ := newTestConsole(t)
		c.d.Random = iotest.ErrReader(errors.New("no entropy"))
		_, _, err := c.rotate(context.Background(), sess)
		wantCode(t, err, errs.CodeInternalError)
	})
	t.Run("store", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		f.store.rotateErr = errStoreDown
		_, _, err := c.rotate(context.Background(), sess)
		wantStoreDown(t, err)
	})
	t.Run("closed session", func(t *testing.T) {
		t.Parallel()
		c, _ := newTestConsole(t)
		_, _, err := c.rotate(context.Background(), sess)
		wantCode(t, err, errs.CodeSessionRevoked)
	})
}

// openConsoleState is the authentication state of an open console session with a step-up, seen
// a minute ago, on an active device of an active person.
func openConsoleState() ConsoleAuthState {
	return ConsoleAuthState{
		Session: SessionRecord{
			ID: ids.New(), UserID: ids.New(), DeviceID: ids.New(), Component: identity.ComponentConsole, AuthMethod: identity.AuthOIDC,
			CreatedAt: consoleNow.Add(-time.Hour), ExpiresAt: consoleNow.Add(time.Hour), LastSeenAt: consoleNow.Add(-time.Minute),
			StepUpAt: consoleNow.Add(-time.Minute),
		},
		DeviceState: identity.DeviceStateActive, UserActive: true,
	}
}

// consoleCreds are the credentials of a console request.
func consoleCreds() actor.Credentials {
	return actor.Credentials{ConsoleCookie: actor.NewCookieValue("cookie value")}
}

// TestConsoleAuthenticateErrors: the error and refusal paths of the cookie authentication.
func TestConsoleAuthenticateErrors(t *testing.T) {
	t.Parallel()
	t.Run("store", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		f.store.authErr = errStoreDown
		_, err := c.Authenticate(context.Background(), consoleCreds())
		wantStoreDown(t, err)
	})
	t.Run("inactive device", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		f.store.st, f.store.found = openConsoleState(), true
		f.store.st.DeviceState = identity.DeviceStateRevoked
		_, err := c.Authenticate(context.Background(), consoleCreds())
		wantClearingCookie(t, problemOf(t, err, errs.CodeSessionRevoked))
	})
	t.Run("inactive person", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		f.store.st, f.store.found = openConsoleState(), true
		f.store.st.UserActive = false
		_, err := c.Authenticate(context.Background(), consoleCreds())
		wantClearingCookie(t, problemOf(t, err, errs.CodeSessionRevoked))
	})
	t.Run("expire fails", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		f.store.st, f.store.found = openConsoleState(), true
		f.store.st.Session.ExpiresAt = consoleNow
		c.d.UoW = runUoW{err: errStoreDown}
		_, err := c.Authenticate(context.Background(), consoleCreds())
		wantStoreDown(t, err)
	})
	t.Run("expired by another request", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		f.store.st, f.store.found = openConsoleState(), true
		f.store.st.Session.LastSeenAt = consoleNow.Add(-2 * time.Hour)
		_, err := c.Authenticate(context.Background(), consoleCreds())
		wantClearingCookie(t, problemOf(t, err, errs.CodeSessionRevoked))
		if f.sessions.treeCalls != 1 || len(f.audit.entries) != 0 {
			t.Fatalf("lock calls %d, entries %v: want one lock and no entry for a session someone else ended", f.sessions.treeCalls, f.audit.entries)
		}
	})
	t.Run("revoke lock fails", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		f.store.st, f.store.found = openConsoleState(), true
		f.store.st.Session.ExpiresAt = consoleNow
		f.sessions.treeErr = errStoreDown
		_, err := c.Authenticate(context.Background(), consoleCreds())
		wantStoreDown(t, err)
	})
	t.Run("binding fails", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		f.store.st, f.store.found = openConsoleState(), true
		f.store.st.Session.AuthMethod = identity.AuthTrustedHeader
		c.d.Binding = fixedBinding{err: errStoreDown}
		_, err := c.Authenticate(context.Background(), consoleCreds())
		wantStoreDown(t, err)
	})
	t.Run("binding mismatch, lock fails", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		f.store.st, f.store.found = openConsoleState(), true
		f.store.st.Session.AuthMethod = identity.AuthTrustedHeader
		c.d.Binding = fixedBinding{ok: false}
		f.sessions.treeErr = errStoreDown
		_, err := c.Authenticate(context.Background(), consoleCreds())
		wantStoreDown(t, err)
	})
	t.Run("binding mismatch, already revoked", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		f.store.st, f.store.found = openConsoleState(), true
		f.store.st.Session.AuthMethod = identity.AuthTrustedHeader
		c.d.Binding = fixedBinding{ok: false}
		_, err := c.Authenticate(context.Background(), consoleCreds())
		wantClearingCookie(t, problemOf(t, err, errs.CodeSessionRevoked))
		if len(f.audit.entries) != 0 {
			t.Fatalf("entries = %v, want none without a revocation", f.audit.entries)
		}
	})
	t.Run("view end fails", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		f.store.st, f.store.found = openConsoleState(), true
		f.store.st.Session.ViewAsUserID = ids.New()
		f.store.st.Session.ViewAsStartedAt = consoleNow.Add(-2 * time.Hour)
		f.store.clearErr = errStoreDown
		_, err := c.Authenticate(context.Background(), consoleCreds())
		wantStoreDown(t, err)
	})
}

// TestConsoleAuthenticateTouchFailureIsLogged: a failed activity write does not fail the request;
// it is logged with its kind only.
func TestConsoleAuthenticateTouchFailureIsLogged(t *testing.T) {
	t.Parallel()
	c, f := newTestConsole(t)
	f.store.st, f.store.found = openConsoleState(), true
	f.store.st.Session.LastSeenAt = consoleNow.Add(-20 * time.Minute)
	f.store.touchErr = errStoreDown
	a, err := c.Authenticate(context.Background(), consoleCreds())
	if err != nil {
		t.Fatal(err)
	}
	if a.SessionID != f.store.st.Session.ID || f.store.touched != 1 {
		t.Fatalf("actor %+v, touched %d", a, f.store.touched)
	}
	if out := f.log.String(); !strings.Contains(out, msgConsoleTouchFailed) || !strings.Contains(out, errs.KindInternal.String()) {
		t.Fatalf("log = %q, want the touch failure with its kind", out)
	}
}

// TestConsoleViewsStartView: the view store hands on a store error and answers a session that is
// no longer an open console session with session_revoked.
func TestConsoleViewsStartView(t *testing.T) {
	t.Parallel()
	err := NewConsoleViews(&fakeConsoleStore{storeErr: errStoreDown}).StartView(context.Background(), ids.New(), ids.New(), consoleNow)
	wantStoreDown(t, err)
	err = NewConsoleViews(&fakeConsoleStore{}).StartView(context.Background(), ids.New(), ids.New(), consoleNow)
	wantCode(t, err, errs.CodeSessionRevoked)
	if err := NewConsoleViews(&fakeConsoleStore{stored: true}).StartView(context.Background(), ids.New(), ids.New(), consoleNow); err != nil {
		t.Fatal(err)
	}
}

// otherCallerContext is a context whose principal is someone else than the caller.
func otherCallerContext() context.Context {
	return actor.WithActor(context.Background(), actor.Actor{ID: ids.New(), Kind: actor.KindUser})
}

// TestConsoleEndViewAsErrors: EndViewAs refuses a foreign principal in ctx, hands on an
// authorizer error, answers a denial and refuses a session that is no console session.
func TestConsoleEndViewAsErrors(t *testing.T) {
	t.Parallel()
	t.Run("foreign principal", func(t *testing.T) {
		t.Parallel()
		c, _ := newTestConsole(t)
		wantCode(t, c.EndViewAs(otherCallerContext(), consoleActor()), errs.CodeInternalError)
	})
	t.Run("authorizer error", func(t *testing.T) {
		t.Parallel()
		c, _ := newTestConsole(t)
		c.d.Authorizer = fixedAuthorizer{err: errStoreDown}
		wantStoreDown(t, c.EndViewAs(context.Background(), consoleActor()))
	})
	t.Run("denied", func(t *testing.T) {
		t.Parallel()
		c, _ := newTestConsole(t)
		c.d.Authorizer = fixedAuthorizer{d: authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "insufficient_role"}}
		if got := errs.KindOf(c.EndViewAs(context.Background(), consoleActor())); got != errs.KindForbidden {
			t.Fatalf("kind = %v, want forbidden", got)
		}
	})
	t.Run("not a console session", func(t *testing.T) {
		t.Parallel()
		c, _ := newTestConsole(t)
		a := consoleActor()
		a.ClientComponent = string(identity.ComponentApp)
		wantCode(t, c.EndViewAs(context.Background(), a), errs.CodeForbidden)
	})
	t.Run("store", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		f.store.clearErr = errStoreDown
		wantStoreDown(t, c.EndViewAs(context.Background(), consoleActor()))
	})
}

// TestConsoleEndViewAsRecordsTheEnd: the end of a stored view writes the entry of
// access.view_as.end with the reason ended.
func TestConsoleEndViewAsRecordsTheEnd(t *testing.T) {
	t.Parallel()
	c, f := newTestConsole(t)
	f.store.view, f.store.cleared = ConsoleView{UserID: ids.New(), StartedAt: consoleNow.Add(-time.Minute)}, true
	if err := c.EndViewAs(context.Background(), consoleActor()); err != nil {
		t.Fatal(err)
	}
	if len(f.audit.entries) != 1 || f.audit.entries[0].Action != string(authorize.ActionAccessViewAsEnd) ||
		f.audit.entries[0].Attrs[attrEndedReason].Data() != viewEnded {
		t.Fatalf("entries = %+v", f.audit.entries)
	}
}

// TestConsoleGetConsoleSessionErrors: the error paths of getConsoleSession and its view.
func TestConsoleGetConsoleSessionErrors(t *testing.T) {
	t.Parallel()
	type setup func(c *Console, f *consoleFakes, a *actor.Actor) context.Context
	ok := func(*Console, *consoleFakes, *actor.Actor) context.Context { return context.Background() }
	found := func(f *consoleFakes) {
		f.sessions.sess, f.sessions.found = SessionRecord{ID: ids.New(), UserID: ids.New()}, true
	}
	cases := []struct {
		name  string
		setup setup
		check func(t *testing.T, err error)
	}{
		{"foreign principal", func(*Console, *consoleFakes, *actor.Actor) context.Context { return otherCallerContext() },
			func(t *testing.T, err error) { t.Helper(); wantCode(t, err, errs.CodeInternalError) }},
		{"authorizer error", func(c *Console, _ *consoleFakes, _ *actor.Actor) context.Context {
			c.d.Authorizer = fixedAuthorizer{err: errStoreDown}
			return context.Background()
		}, wantStoreDown},
		{"not a console session", func(_ *Console, _ *consoleFakes, a *actor.Actor) context.Context {
			a.ClientComponent = string(identity.ComponentApp)
			return context.Background()
		}, func(t *testing.T, err error) { t.Helper(); wantCode(t, err, errs.CodeForbidden) }},
		{"session read", func(_ *Console, f *consoleFakes, _ *actor.Actor) context.Context {
			f.sessions.sessErr = errStoreDown
			return context.Background()
		}, wantStoreDown},
		{"session gone", ok, func(t *testing.T, err error) { t.Helper(); wantCode(t, err, errs.CodeSessionRevoked) }},
		{"person read", func(_ *Console, f *consoleFakes, _ *actor.Actor) context.Context {
			found(f)
			f.people.userErr = errStoreDown
			return context.Background()
		}, wantStoreDown},
		{"person gone", func(_ *Console, f *consoleFakes, _ *actor.Actor) context.Context {
			found(f)
			f.people.userFound = false
			return context.Background()
		}, func(t *testing.T, err error) { t.Helper(); wantCode(t, err, errs.CodeNotFound) }},
		{"passkey read", func(c *Console, f *consoleFakes, _ *actor.Actor) context.Context {
			found(f)
			c.d.PasskeysEnabled = true
			f.people.passkeyErr = errStoreDown
			return context.Background()
		}, wantStoreDown},
		{"identity read", func(_ *Console, f *consoleFakes, _ *actor.Actor) context.Context {
			found(f)
			f.identities.listErr = errStoreDown
			return context.Background()
		}, wantStoreDown},
		{"local credential read", func(c *Console, f *consoleFakes, _ *actor.Actor) context.Context {
			found(f)
			c.cfg.LocalPasswordEnabled = true
			f.people.localErr = errStoreDown
			return context.Background()
		}, wantStoreDown},
		{"viewed person read", func(_ *Console, f *consoleFakes, _ *actor.Actor) context.Context {
			found(f)
			f.sessions.sess.ViewAsUserID = ids.New()
			f.people.principalErr = errStoreDown
			return context.Background()
		}, wantStoreDown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, f := newTestConsole(t)
			a := consoleActor()
			ctx := tc.setup(c, f, &a)
			_, err := c.GetConsoleSession(ctx, a)
			tc.check(t, err)
		})
	}
}

// TestConsoleGetConsoleSessionFull: the state names every available step-up method, the end of a
// valid step-up and the view with the viewed person's name.
func TestConsoleGetConsoleSessionFull(t *testing.T) {
	t.Parallel()
	c, f := newTestConsole(t)
	c.d.PasskeysEnabled, c.cfg.LocalPasswordEnabled = true, true
	f.people.passkey, f.people.local = true, true
	f.identities.list = []IdentityRecord{
		{Identity: identity.Identity{ProviderKind: identity.ProviderLocal}, ProviderEnabled: true},
		{Identity: identity.Identity{ProviderKind: identity.ProviderOIDC}, ProviderEnabled: true},
	}
	name, err := domain.NewDisplayName("Sam", 64)
	if err != nil {
		t.Fatal(err)
	}
	f.people.principal, f.people.principalFound = PrincipalRecord{Principal: domain.Principal{DisplayName: name}}, true
	stepUpAt := consoleNow.Add(-time.Minute)
	f.sessions.sess = SessionRecord{ID: ids.New(), UserID: ids.New(), StepUpAt: stepUpAt, ViewAsUserID: ids.New(), ViewAsStartedAt: consoleNow}
	f.sessions.found = true
	got, err := c.GetConsoleSession(context.Background(), consoleActor())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{identity.StepUpMethodPasskey, identity.StepUpMethodOIDCReauth, identity.StepUpMethodPasswordTOTP}
	if strings.Join(got.MethodsAvailable, ",") != strings.Join(want, ",") {
		t.Fatalf("methods = %v, want %v", got.MethodsAvailable, want)
	}
	if !got.StepUpValidUntil.Equal(stepUpAt.Add(c.cfg.StepUpMaxAge)) {
		t.Fatalf("StepUpValidUntil = %v", got.StepUpValidUntil)
	}
	if got.ViewAs == nil || got.ViewAs.DisplayName != "Sam" || !got.ViewAs.EndsAt.Equal(consoleNow.Add(time.Hour)) {
		t.Fatalf("view = %+v", got.ViewAs)
	}
}

// TestExpireConsoleSessionsErrors: the sweep hands on the error of the selection and of the first
// failing unit, with the count so far.
func TestExpireConsoleSessionsErrors(t *testing.T) {
	t.Parallel()
	t.Run("selection", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		f.store.dueErr = errStoreDown
		n, err := c.ExpireConsoleSessions(context.Background())
		wantStoreDown(t, err)
		if n != 0 {
			t.Fatalf("n = %d", n)
		}
	})
	t.Run("unit", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		f.store.pages = [][]ids.ID{{ids.New()}}
		f.sessions.sessErr = errStoreDown
		_, err := c.ExpireConsoleSessions(context.Background())
		wantStoreDown(t, err)
	})
}

// TestExpireConsoleSessionsPages: a full page asks for the next one after its last id.
func TestExpireConsoleSessionsPages(t *testing.T) {
	t.Parallel()
	c, f := newTestConsole(t)
	page := make([]ids.ID, consoleSweepBatch)
	for i := range page {
		page[i] = ids.New()
	}
	f.store.pages = [][]ids.ID{page}
	n, err := c.ExpireConsoleSessions(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("ExpireConsoleSessions = %d, %v", n, err)
	}
	if len(f.store.afters) != 2 || f.store.afters[1] != page[len(page)-1] {
		t.Fatalf("afters = %v, want a second page after the last id", f.store.afters)
	}
}

// TestExpireDueRechecksTheSession: a session a request touched since the selection stays open,
// and one past its absolute end is revoked.
func TestExpireDueRechecksTheSession(t *testing.T) {
	t.Parallel()
	t.Run("touched since", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		f.sessions.sess = SessionRecord{ID: ids.New(), ExpiresAt: consoleNow.Add(time.Hour), LastSeenAt: consoleNow}
		f.sessions.found = true
		ended, err := c.expireDue(context.Background(), f.sessions.sess.ID, consoleNow)
		if err != nil || ended || f.sessions.treeCalls != 0 {
			t.Fatalf("expireDue = %v, %v with %d locks, want no revocation", ended, err, f.sessions.treeCalls)
		}
	})
	t.Run("past the end", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		f.sessions.sess = SessionRecord{ID: ids.New(), ExpiresAt: consoleNow, LastSeenAt: consoleNow}
		f.sessions.found = true
		f.sessions.treeErr = errStoreDown
		_, err := c.expireDue(context.Background(), f.sessions.sess.ID, consoleNow)
		wantStoreDown(t, err)
		if f.sessions.treeCalls != 1 {
			t.Fatalf("locks = %d, want the revocation", f.sessions.treeCalls)
		}
	})
}
