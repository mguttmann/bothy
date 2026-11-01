// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/oidcflow"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/stepup"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// fakeChallenges is a ChallengeStore with one canned challenge.
type fakeChallenges struct {
	ChallengeStore
	ch       ChallengeRecord
	found    bool
	lockErr  error
	failures []time.Time
	finished []ChallengeStatus
	methods  []string
}

func (f *fakeChallenges) LockChallenge(context.Context, ids.ID) (ChallengeRecord, bool, error) {
	return f.ch, f.found, f.lockErr
}

func (f *fakeChallenges) FinishChallenge(_ context.Context, _ ids.ID, status ChallengeStatus, _ time.Time) error {
	f.finished = append(f.finished, status)
	return nil
}

func (f *fakeChallenges) FailedStepUpTimes(context.Context, ids.ID, time.Time) ([]time.Time, error) {
	return f.failures, nil
}

func (f *fakeChallenges) SetSessionStepUp(_ context.Context, _ ids.ID, _ time.Time, method string) error {
	f.methods = append(f.methods, method)
	return nil
}

// fakeAuth is an identity.Auth that records the revoked sessions.
type fakeAuth struct {
	identity.Auth
	revoked []ids.ID
}

func (f *fakeAuth) RevokeSession(_ context.Context, sessionID ids.ID, _ identity.SessionRevokeReason, _ *ids.ID) error {
	f.revoked = append(f.revoked, sessionID)
	return nil
}

// stepUpFakes are the collaborators of a test console with its step-up use cases.
type stepUpFakes struct {
	*consoleFakes
	challenges *fakeChallenges
	auth       *fakeAuth
}

// newTestStepUpConsole returns a test console bound to step-up use cases over the same fakes.
func newTestStepUpConsole(t *testing.T) (*Console, *stepUpFakes) {
	t.Helper()
	c, cf := newTestConsole(t)
	f := &stepUpFakes{consoleFakes: cf, challenges: &fakeChallenges{}, auth: &fakeAuth{}}
	c.stepUps = &StepUps{
		cfg: Config{StepUpMaxAge: 5 * time.Minute, LoginFailWindow: 15 * time.Minute, StepUpFailLimit: 3},
		d: StepUpDeps{
			Challenges: f.challenges, Sessions: cf.sessions, Devices: cf.devices, Auth: f.auth, UoW: runUoW{},
			Authorizer: allowAll, Audit: cf.audit, Clock: c.d.Clock, Console: c,
		},
	}
	return c, f
}

// wantDenied fails t unless err is a refused step-up whose answer has code want.
func wantDenied(t *testing.T, err error, want errs.Code) {
	t.Helper()
	var d *stepUpDeniedError
	if !errors.As(err, &d) {
		t.Fatalf("err = %v, want a refused step-up", err)
	}
	wantCode(t, d.answer, want)
}

// TestConsoleStepUpBeginErrors: every console step-up refuses a foreign principal in ctx, an
// unbound step-up, a session that is no console session and a failing admit.
func TestConsoleStepUpBeginErrors(t *testing.T) {
	t.Parallel()
	t.Run("foreign principal", func(t *testing.T) {
		t.Parallel()
		c, _ := newTestStepUpConsole(t)
		c.oidc = &OIDC{}
		_, err := c.BeginStepUpOIDC(otherCallerContext(), consoleActor(), identity.StepUpOIDCBegin{})
		wantCode(t, err, errs.CodeInternalError)
	})
	t.Run("unbound", func(t *testing.T) {
		t.Parallel()
		c, _ := newTestConsole(t)
		c.oidc = &OIDC{}
		_, err := c.CompleteStepUpOIDC(context.Background(), consoleActor(), "code", "verifier")
		wantCode(t, err, errs.CodeInternalError)
	})
	t.Run("not a console session", func(t *testing.T) {
		t.Parallel()
		c, f := newTestStepUpConsole(t)
		c.oidc = &OIDC{}
		a := consoleActor()
		a.ClientComponent = string(identity.ComponentApp)
		_, err := c.CompleteStepUpOIDC(context.Background(), a, "code", "verifier")
		if e := problemOf(t, err, errs.CodeForbidden); e.Reason != reasonStepUpNotPossible {
			t.Fatalf("reason = %q", e.Reason)
		}
		if len(f.audit.denials) != 1 {
			t.Fatalf("denials = %v, want one", f.audit.denials)
		}
	})
	t.Run("admit fails", func(t *testing.T) {
		t.Parallel()
		c, f := newTestStepUpConsole(t)
		c.local = &LocalCredentials{enabled: true}
		f.devices.err = errStoreDown
		_, err := c.StepUpWithPassword(context.Background(), consoleActor(), ids.New(), "password", "123456")
		wantStoreDown(t, err)
	})
}

// TestConsoleStepUpUnboundMethods: a method whose use case is not bound is not found.
func TestConsoleStepUpUnboundMethods(t *testing.T) {
	t.Parallel()
	c, _ := newTestStepUpConsole(t)
	_, err := c.StepUpWithPassword(context.Background(), consoleActor(), ids.New(), "password", "123456")
	wantCode(t, err, errs.CodeNotFound)
	c.local = &LocalCredentials{}
	_, err = c.StepUpWithPassword(context.Background(), consoleActor(), ids.New(), "password", "123456")
	wantCode(t, err, errs.CodeNotFound)
	_, err = c.BeginStepUpOIDC(context.Background(), consoleActor(), identity.StepUpOIDCBegin{})
	wantCode(t, err, errs.CodeNotFound)
	_, err = c.CompleteStepUpOIDC(context.Background(), consoleActor(), "code", "verifier")
	wantCode(t, err, errs.CodeNotFound)
}

// TestConsoleStepUpWithPasswordWithoutLocalAccount: a person without a local account gets 403
// step_up_not_possible, audited.
func TestConsoleStepUpWithPasswordWithoutLocalAccount(t *testing.T) {
	t.Parallel()
	c, f := newTestStepUpConsole(t)
	c.local = &LocalCredentials{enabled: true, d: LocalDeps{Identities: f.identities}}
	_, err := c.StepUpWithPassword(context.Background(), consoleActor(), ids.New(), "password", "123456")
	if e := problemOf(t, err, errs.CodeForbidden); e.Reason != reasonStepUpNotPossible {
		t.Fatalf("reason = %q", e.Reason)
	}
	if len(f.audit.denials) != 1 || f.audit.denials[0].Attrs[attrCause].Data() != causeNoLocalAccount {
		t.Fatalf("denials = %+v", f.audit.denials)
	}
}

// TestConsoleStepUpWithPasswordRefusedRowCountsNothing (08 6.2 and 13, R-ID-10,
// SEC-STEPUP-LOCKOUT): a locked password path and a password changed during the attempt are
// refusals of the login rules, not wrong confirmations: 401 step_up_required with reason
// password_locked or password_changed, audited with cause locked or credential_changed, the
// challenge is neither locked nor failed and the session is never revoked, even with
// auth.step_up_fail_limit failures already in the window.
func TestConsoleStepUpWithPasswordRefusedRowCountsNothing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, reason, cause string
		change              func(*fakeCredentials)
	}{
		{"locked", reasonPasswordLocked, causeLocked, func(c *fakeCredentials) {
			c.rec.Lockout.LockedUntil = consoleNow.Add(time.Minute)
		}},
		{"changed", reasonPasswordChanged, causeChanged, func(c *fakeCredentials) {
			c.lockSame = false
			c.locked = c.rec
			c.locked.PasswordHash = "$argon2id$new"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, f := newTestStepUpConsole(t)
			l, creds := localForStepUp()
			tc.change(creds)
			c.local = l
			f.challenges.found = true
			f.challenges.failures = []time.Time{consoleNow, consoleNow, consoleNow}
			_, err := c.StepUpWithPassword(context.Background(), consoleActor(), ids.New(), "password", validTOTP())
			if e := problemOf(t, err, errs.CodeStepUpRequired); e.Reason != tc.reason {
				t.Fatalf("reason = %q, want %q", e.Reason, tc.reason)
			}
			if len(f.challenges.finished) != 0 || len(f.auth.revoked) != 0 {
				t.Fatalf("finished %v, revoked %v: a refused row must count nothing", f.challenges.finished, f.auth.revoked)
			}
			if len(f.audit.denials) != 1 || f.audit.denials[0].Attrs[attrCause].Data() != tc.cause ||
				f.audit.denials[0].Reason != auditing.ReasonStepUpRequired {
				t.Fatalf("denials = %+v", f.audit.denials)
			}
		})
	}
}

// TestConsoleBeginStepUpOIDCErrors: the start of a re-login refuses a failing admit, another
// redirect and a challenge that cannot be read.
func TestConsoleBeginStepUpOIDCErrors(t *testing.T) {
	t.Parallel()
	t.Run("admit fails", func(t *testing.T) {
		t.Parallel()
		c, f := newTestStepUpConsole(t)
		c.oidc = &OIDC{}
		f.sessions.sessErr = errStoreDown
		_, err := c.BeginStepUpOIDC(context.Background(), consoleActor(), identity.StepUpOIDCBegin{})
		wantStoreDown(t, err)
	})
	t.Run("other redirect", func(t *testing.T) {
		t.Parallel()
		c, _ := newTestStepUpConsole(t)
		c.oidc = &OIDC{}
		_, err := c.BeginStepUpOIDC(context.Background(), consoleActor(), identity.StepUpOIDCBegin{AppRedirectURI: "https://other.example.com/cb"})
		if e := problemOf(t, err, errs.CodeValidationFailed); len(e.Fields) != 1 || e.Fields[0].Pointer != fieldAppRedirectURI {
			t.Fatalf("fields = %+v", e.Fields)
		}
	})
	t.Run("challenge read", func(t *testing.T) {
		t.Parallel()
		c, f := newTestStepUpConsole(t)
		c.oidc = &OIDC{}
		f.challenges.lockErr = errStoreDown
		_, err := c.BeginStepUpOIDC(context.Background(), consoleActor(), identity.StepUpOIDCBegin{AppRedirectURI: c.stepUpCallback()})
		wantStoreDown(t, err)
	})
}

// TestConsoleReauthChallenge: only the console step-up callback with a well-formed challenge id
// names a challenge.
func TestConsoleReauthChallenge(t *testing.T) {
	t.Parallel()
	c, _ := newTestConsole(t)
	id := ids.New()
	if got, ok := c.reauthChallenge(c.stepUpCallback() + "?" + queryChallengeID + "=" + id.String()); !ok || got != id {
		t.Fatalf("reauthChallenge = %v, %v, want %v", got, ok, id)
	}
	for _, redirect := range []string{
		c.stepUpCallback(),
		"https://other.example.com" + consoleStepUpCallback + "?" + queryChallengeID + "=" + id.String(),
		c.stepUpCallback() + "?%zz",
		c.stepUpCallback() + "?" + queryChallengeID + "=not-an-id",
	} {
		if _, ok := c.reauthChallenge(redirect); ok {
			t.Errorf("reauthChallenge(%q) names a challenge", redirect)
		}
	}
}

// fakeFlows is a LoginFlowStore with one canned flow.
type fakeFlows struct {
	LoginFlowStore
	flow   LoginFlowRecord
	found  bool
	failed []ids.ID
}

func (f *fakeFlows) LockLoginFlowByCodeHash(context.Context, []byte) (LoginFlowRecord, bool, error) {
	return f.flow, f.found, nil
}

func (f *fakeFlows) FailLoginFlow(_ context.Context, id ids.ID, _ string) (bool, error) {
	f.failed = append(f.failed, id)
	return true, nil
}

// reauthConsole is a test console with an OIDC use case over a flow store that holds a
// redeemable re-login flow of a, whose redirect names a challenge.
func reauthConsole(t *testing.T, a actor.Actor) (*Console, *stepUpFakes, *fakeFlows) {
	t.Helper()
	c, f := newTestStepUpConsole(t)
	flows := &fakeFlows{found: true, flow: LoginFlowRecord{
		ID: ids.New(), ProviderID: ids.New(), Mode: oidcflow.ModeLink, UserID: a.ID, Status: oidcflow.StatusCodeIssued,
		AppRedirectURI:  c.stepUpCallback() + "?" + queryChallengeID + "=" + ids.New().String(),
		ClientChallenge: "challenge of another verifier", External: &oidcflow.ExternalIdentity{Issuer: "https://idp.example.com", Subject: "sam"},
		LoginCodeExpiresAt: consoleNow.Add(time.Minute), ExpiresAt: consoleNow.Add(time.Minute),
	}}
	c.oidc = &OIDC{pepper: []byte("pepper"), d: OIDCDeps{Flows: flows}, s: &Service{d: ServiceDeps{Clock: c.d.Clock, Audit: f.audit, UoW: runUoW{}}}}
	return c, f, flows
}

// TestConsoleCompleteStepUpOIDCRefusals: a re-login flow of another redirect or another person and
// a wrong PKCE verifier answer 401 step_up_required, audited; only the wrong verifier fails the
// flow.
func TestConsoleCompleteStepUpOIDCRefusals(t *testing.T) {
	t.Parallel()
	t.Run("admit fails", func(t *testing.T) {
		t.Parallel()
		a := consoleActor()
		c, f, _ := reauthConsole(t, a)
		f.sessions.sessErr = errStoreDown
		_, err := c.CompleteStepUpOIDC(context.Background(), a, "code", "verifier")
		wantStoreDown(t, err)
	})
	t.Run("other redirect", func(t *testing.T) {
		t.Parallel()
		a := consoleActor()
		c, f, flows := reauthConsole(t, a)
		flows.flow.AppRedirectURI = "https://bothy.example.com/console/login/callback"
		_, err := c.CompleteStepUpOIDC(context.Background(), a, "code", "verifier")
		wantCode(t, err, errs.CodeStepUpRequired)
		if len(flows.failed) != 0 || len(f.audit.denials) != 1 || f.audit.denials[0].Attrs[attrCause].Data() != causeReauthFlow {
			t.Fatalf("failed %v, denials %+v", flows.failed, f.audit.denials)
		}
	})
	t.Run("other person", func(t *testing.T) {
		t.Parallel()
		a := consoleActor()
		c, _, flows := reauthConsole(t, a)
		flows.flow.UserID = ids.New()
		_, err := c.CompleteStepUpOIDC(context.Background(), a, "code", "verifier")
		wantCode(t, err, errs.CodeStepUpRequired)
	})
	t.Run("wrong verifier", func(t *testing.T) {
		t.Parallel()
		a := consoleActor()
		c, _, flows := reauthConsole(t, a)
		_, err := c.CompleteStepUpOIDC(context.Background(), a, "code", "verifier")
		wantCode(t, err, errs.CodeStepUpRequired)
		if len(flows.failed) != 1 || flows.failed[0] != flows.flow.ID {
			t.Fatalf("failed = %v, want the flow failed", flows.failed)
		}
	})
}

// TestConsoleReauthCause: the re-login confirms only an identity of the person at the same issuer
// with an auth_time not before the second of the challenge's creation.
func TestConsoleReauthCause(t *testing.T) {
	t.Parallel()
	a := consoleActor()
	created := consoleNow.Add(-time.Minute)
	re := reauth{providerID: ids.New(), ext: oidcflow.ExternalIdentity{Issuer: "https://idp.example.com", Subject: "sam", AuthTime: created.Truncate(time.Second)}}
	ch := ChallengeRecord{CreatedAt: created.Add(500 * time.Millisecond)}
	mine := identity.Identity{UserID: a.ID, Issuer: re.ext.Issuer}
	cases := []struct {
		name  string
		ident *fakeIdentities
		ext   func(*oidcflow.ExternalIdentity)
		want  stepup.Cause
	}{
		{"confirmed", &fakeIdentities{bySubject: mine, found: true}, nil, ""},
		{"unknown identity", &fakeIdentities{}, nil, causeIdentityOther},
		{"other person", &fakeIdentities{bySubject: identity.Identity{UserID: ids.New(), Issuer: re.ext.Issuer}, found: true}, nil, causeIdentityOther},
		{"other issuer", &fakeIdentities{bySubject: identity.Identity{UserID: a.ID, Issuer: "https://other.example.com"}, found: true}, nil, causeIdentityOther},
		{"no auth_time", &fakeIdentities{bySubject: mine, found: true}, func(x *oidcflow.ExternalIdentity) { x.AuthTime = time.Time{} }, causeAuthTimeTooOld},
		{"auth_time too old", &fakeIdentities{bySubject: mine, found: true}, func(x *oidcflow.ExternalIdentity) { x.AuthTime = created.Add(-time.Second) }, causeAuthTimeTooOld},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, _ := newTestConsole(t)
			c.d.Identities = tc.ident
			r := re
			if tc.ext != nil {
				tc.ext(&r.ext)
			}
			got, err := c.reauthCause(context.Background(), a, r, ch)
			if err != nil || got != tc.want {
				t.Fatalf("reauthCause = %q, %v, want %q", got, err, tc.want)
			}
		})
	}
	c, _ := newTestConsole(t)
	c.d.Identities = &fakeIdentities{subjErr: errStoreDown}
	_, err := c.reauthCause(context.Background(), a, re, ch)
	wantStoreDown(t, err)
}

// openChallenge stores an issued step-up challenge of the session of a and the open console
// session itself in the fakes, and returns the challenge id.
func openChallenge(f *stepUpFakes, a actor.Actor) ids.ID {
	f.challenges.ch = ChallengeRecord{
		ID: ids.New(), SessionID: a.SessionID, Purpose: PurposeStepUp, Status: ChallengeIssued,
		CreatedAt: consoleNow.Add(-time.Minute), ExpiresAt: consoleNow.Add(time.Minute),
	}
	f.challenges.found = true
	f.sessions.tree = []SessionRecord{{
		ID: a.SessionID, UserID: a.ID, Component: identity.ComponentConsole, ExpiresAt: consoleNow.Add(time.Hour),
	}}
	return f.challenges.ch.ID
}

// TestStepUpsCompleteWith: the completion of a console step-up hands on the errors of the lock,
// the challenge and the check, fails and counts a refused check, revokes the session at the limit
// and rotates the cookie of a confirmation.
func TestStepUpsCompleteWith(t *testing.T) {
	t.Parallel()
	confirm := func(context.Context, ChallengeRecord) (stepup.Cause, error) { return "", nil }
	refuse := func(context.Context, ChallengeRecord) (stepup.Cause, error) { return causeIdentityOther, nil }
	t.Run("session lock", func(t *testing.T) {
		t.Parallel()
		c, f := newTestStepUpConsole(t)
		a := consoleActor()
		id := openChallenge(f, a)
		f.sessions.treeErr = errStoreDown
		_, err := c.stepUps.completeWith(context.Background(), a, id, identity.StepUpMethodOIDCReauth, confirm)
		wantStoreDown(t, err)
	})
	t.Run("session ended", func(t *testing.T) {
		t.Parallel()
		c, f := newTestStepUpConsole(t)
		a := consoleActor()
		id := openChallenge(f, a)
		f.sessions.tree = nil
		_, err := c.stepUps.completeWith(context.Background(), a, id, identity.StepUpMethodOIDCReauth, confirm)
		wantCode(t, err, errs.CodeSessionRevoked)
	})
	t.Run("challenge read", func(t *testing.T) {
		t.Parallel()
		c, f := newTestStepUpConsole(t)
		a := consoleActor()
		id := openChallenge(f, a)
		f.challenges.lockErr = errStoreDown
		_, err := c.stepUps.completeWith(context.Background(), a, id, identity.StepUpMethodOIDCReauth, confirm)
		wantStoreDown(t, err)
	})
	t.Run("check fails", func(t *testing.T) {
		t.Parallel()
		c, f := newTestStepUpConsole(t)
		a := consoleActor()
		id := openChallenge(f, a)
		_, err := c.stepUps.completeWith(context.Background(), a, id, identity.StepUpMethodOIDCReauth,
			func(context.Context, ChallengeRecord) (stepup.Cause, error) { return "", errStoreDown })
		wantStoreDown(t, err)
	})
	t.Run("refused", func(t *testing.T) {
		t.Parallel()
		c, f := newTestStepUpConsole(t)
		a := consoleActor()
		id := openChallenge(f, a)
		_, err := c.stepUps.completeWith(context.Background(), a, id, identity.StepUpMethodOIDCReauth, refuse)
		wantCode(t, err, errs.CodeStepUpRequired)
		if len(f.challenges.finished) != 1 || f.challenges.finished[0] != ChallengeFailed || len(f.auth.revoked) != 0 {
			t.Fatalf("finished %v, revoked %v", f.challenges.finished, f.auth.revoked)
		}
	})
	t.Run("refused at the limit", func(t *testing.T) {
		t.Parallel()
		c, f := newTestStepUpConsole(t)
		a := consoleActor()
		id := openChallenge(f, a)
		f.challenges.failures = []time.Time{consoleNow.Add(-2 * time.Minute), consoleNow.Add(-time.Minute), consoleNow}
		_, err := c.stepUps.completeWith(context.Background(), a, id, identity.StepUpMethodOIDCReauth, refuse)
		wantCode(t, err, errs.CodeSessionRevoked)
		if len(f.auth.revoked) != 1 || f.auth.revoked[0] != a.SessionID {
			t.Fatalf("revoked = %v, want the session", f.auth.revoked)
		}
	})
	t.Run("confirmed", func(t *testing.T) {
		t.Parallel()
		c, f := newTestStepUpConsole(t)
		a := consoleActor()
		id := openChallenge(f, a)
		f.store.rotated = true
		got, err := c.stepUps.completeWith(context.Background(), a, id, identity.StepUpMethodOIDCReauth, confirm)
		if err != nil {
			t.Fatal(err)
		}
		if got.Cookie.IsZero() || got.CookieMaxAge != time.Hour || len(f.challenges.finished) != 1 || f.challenges.finished[0] != ChallengeUsed {
			t.Fatalf("result %+v, finished %v", got, f.challenges.finished)
		}
	})
}

// fakeCredentials is a LocalCredentialStore with one canned row.
type fakeCredentials struct {
	LocalCredentialStore
	rec      LocalCredentialRecord
	found    bool
	readErr  error
	locked   LocalCredentialRecord
	lockErr  error
	lockSame bool
	saved    []LocalCredentialRecord
}

func (f *fakeCredentials) LocalCredential(context.Context, ids.ID) (LocalCredentialRecord, bool, error) {
	return f.rec, f.found, f.readErr
}

func (f *fakeCredentials) LockLocalCredential(context.Context, ids.ID) (LocalCredentialRecord, bool, error) {
	if f.lockSame {
		return f.rec, f.found, f.lockErr
	}
	return f.locked, f.found, f.lockErr
}

func (f *fakeCredentials) SaveLocalCredential(_ context.Context, rec LocalCredentialRecord) error {
	f.saved = append(f.saved, rec)
	return nil
}

// fakeProviders is a ProviderStore whose every provider is enabled.
type fakeProviders struct{ ProviderStore }

func (fakeProviders) EnabledProvider(context.Context, ids.ID) (identity.Provider, bool, error) {
	return identity.Provider{}, true, nil
}

// fakeHasher answers Verify with ok, rehash and err and Hash with phc and hashErr.
type fakeHasher struct {
	PasswordHasher
	ok, rehash   bool
	err, hashErr error
	phc          string
}

func (h fakeHasher) Verify(string, string) (bool, bool, error) { return h.ok, h.rehash, h.err }

func (h fakeHasher) Hash(string) (string, error) { return h.phc, h.hashErr }

// identityNormalizer leaves a password as it is.
type identityNormalizer struct{}

func (identityNormalizer) NFC(p string) string { return p }

// fakeSealer opens every ciphertext to secret, or fails with err.
type fakeSealer struct {
	SecretSealer
	secret []byte
	err    error
}

func (s fakeSealer) Open(ids.ID, []byte, string) ([]byte, error) { return s.secret, s.err }

// storedPHC is the stored hash of the local credential of these tests.
const storedPHC = "phc-before"

// totpSecret is the TOTP secret of the local credential of these tests (any bytes; the codes are
// derived from it).
var totpSecret = []byte("totp-seed-of-these-tests")

// localForStepUp returns local credentials over fakes with one local identity of an enabled
// provider and an established row with TOTP.
func localForStepUp() (*LocalCredentials, *fakeCredentials) {
	creds := &fakeCredentials{found: true, lockSame: true, rec: LocalCredentialRecord{
		IdentityID: ids.New(), PasswordHash: storedPHC, TOTPCiphertext: []byte("sealed"), TOTPEnabledAt: consoleNow.Add(-time.Hour),
	}}
	l := &LocalCredentials{
		enabled: true, policy: domain.LockoutPolicy{Limit: 5, Window: 15 * time.Minute, LockDuration: 15 * time.Minute},
		d: LocalDeps{
			Credentials: creds, Providers: fakeProviders{},
			Identities: &fakeIdentities{list: []IdentityRecord{{Identity: identity.Identity{ID: creds.rec.IdentityID, ProviderKind: identity.ProviderLocal}}}},
			Hasher:     fakeHasher{ok: true}, Normalizer: identityNormalizer{}, Sealer: fakeSealer{secret: totpSecret},
			UoW: runUoW{}, Audit: &fakeAudit{}, Clock: clock.NewFake(consoleNow),
		},
	}
	return l, creds
}

// validTOTP is the code of totpSecret at consoleNow.
func validTOTP() string { return domain.TOTPCode(totpSecret, domain.TOTPStep(consoleNow), 6) }

// TestStepUpPasswordErrors: the password step-up refuses a missing local account and a row without
// password, hands on the store errors and answers a failing hasher or sealer with internal_error.
func TestStepUpPasswordErrors(t *testing.T) {
	t.Parallel()
	userID := ids.New()
	t.Run("identity read", func(t *testing.T) {
		t.Parallel()
		l, _ := localForStepUp()
		l.d.Identities = &fakeIdentities{listErr: errStoreDown}
		_, err := l.stepUpPassword(context.Background(), userID, "password", validTOTP())
		wantDenied(t, err, errs.CodeForbidden)
	})
	t.Run("row read", func(t *testing.T) {
		t.Parallel()
		l, creds := localForStepUp()
		creds.readErr = errStoreDown
		_, err := l.stepUpPassword(context.Background(), userID, "password", validTOTP())
		wantStoreDown(t, err)
	})
	t.Run("no row", func(t *testing.T) {
		t.Parallel()
		l, creds := localForStepUp()
		creds.found = false
		_, err := l.stepUpPassword(context.Background(), userID, "password", validTOTP())
		wantDenied(t, err, errs.CodeForbidden)
	})
	t.Run("no password", func(t *testing.T) {
		t.Parallel()
		l, creds := localForStepUp()
		creds.rec.PasswordHash = ""
		_, err := l.stepUpPassword(context.Background(), userID, "password", validTOTP())
		wantDenied(t, err, errs.CodeForbidden)
	})
	t.Run("hasher", func(t *testing.T) {
		t.Parallel()
		l, _ := localForStepUp()
		l.d.Hasher = fakeHasher{err: errors.New("broken phc")}
		_, err := l.stepUpPassword(context.Background(), userID, "password", validTOTP())
		wantCode(t, err, errs.CodeInternalError)
	})
	t.Run("row lock", func(t *testing.T) {
		t.Parallel()
		l, creds := localForStepUp()
		creds.lockErr = errStoreDown
		_, err := l.stepUpPassword(context.Background(), userID, "password", validTOTP())
		wantStoreDown(t, err)
	})
	t.Run("sealer", func(t *testing.T) {
		t.Parallel()
		l, _ := localForStepUp()
		l.d.Sealer = fakeSealer{err: errors.New("unknown key")}
		_, err := l.stepUpPassword(context.Background(), userID, "password", validTOTP())
		wantCode(t, err, errs.CodeInternalError)
	})
	t.Run("rehash", func(t *testing.T) {
		t.Parallel()
		l, _ := localForStepUp()
		l.d.Hasher = fakeHasher{ok: true, rehash: true, hashErr: errors.New("no memory")}
		_, err := l.stepUpPassword(context.Background(), userID, "password", validTOTP())
		wantCode(t, err, errs.CodeInternalError)
	})
}

// TestStepUpPasswordLockedRow: a row that changed since the unlocked read or is locked is refused
// without counting; a wrong password is counted; a success rehashes and resets the counter.
func TestStepUpPasswordLockedRow(t *testing.T) {
	t.Parallel()
	userID := ids.New()
	t.Run("changed", func(t *testing.T) {
		t.Parallel()
		l, creds := localForStepUp()
		creds.lockSame = false
		creds.locked = creds.rec
		creds.locked.PasswordHash = "$argon2id$new"
		cause, err := l.stepUpPassword(context.Background(), userID, "password", validTOTP())
		if err != nil || cause != causeChanged || len(creds.saved) != 0 {
			t.Fatalf("cause %q, err %v, saved %d", cause, err, len(creds.saved))
		}
	})
	t.Run("locked", func(t *testing.T) {
		t.Parallel()
		l, creds := localForStepUp()
		creds.rec.Lockout.LockedUntil = consoleNow.Add(time.Minute)
		cause, err := l.stepUpPassword(context.Background(), userID, "password", validTOTP())
		if err != nil || cause != causeLocked || len(creds.saved) != 0 {
			t.Fatalf("cause %q, err %v, saved %d", cause, err, len(creds.saved))
		}
	})
	t.Run("wrong password", func(t *testing.T) {
		t.Parallel()
		l, creds := localForStepUp()
		l.d.Hasher = fakeHasher{ok: false}
		cause, err := l.stepUpPassword(context.Background(), userID, "password", validTOTP())
		if err != nil || cause != causeWrongPassword || len(creds.saved) != 1 || creds.saved[0].Lockout.FailedCount != 1 {
			t.Fatalf("cause %q, err %v, saved %+v", cause, err, creds.saved)
		}
	})
	t.Run("success with rehash", func(t *testing.T) {
		t.Parallel()
		l, creds := localForStepUp()
		creds.rec.Lockout.FailedCount = 2
		l.d.Hasher = fakeHasher{ok: true, rehash: true, phc: "$argon2id$rehashed"}
		cause, err := l.stepUpPassword(context.Background(), userID, "password", validTOTP())
		if err != nil || cause != "" || len(creds.saved) != 1 {
			t.Fatalf("cause %q, err %v, saved %+v", cause, err, creds.saved)
		}
		if s := creds.saved[0]; s.PasswordHash != "$argon2id$rehashed" || s.Lockout.FailedCount != 0 || s.TOTPLastStep == nil {
			t.Fatalf("saved = %+v", s)
		}
	})
}

// TestAuthenticatorConsoleCookieWithoutConsole: a console cookie on a server without the console
// is unauthenticated with the clearing cookie.
func TestAuthenticatorConsoleCookieWithoutConsole(t *testing.T) {
	t.Parallel()
	_, err := (&Authenticator{}).Authenticate(context.Background(), consoleCreds())
	wantClearingCookie(t, problemOf(t, err, errs.CodeUnauthenticated))
}

// TestAuthenticatorRevalidateConsole: a console principal passes while its session is an open
// console session of the person before its end on an active device.
func TestAuthenticatorRevalidateConsole(t *testing.T) {
	t.Parallel()
	p := consoleActor()
	open := AuthState{
		Session:     SessionRecord{ID: p.SessionID, UserID: p.ID, Component: identity.ComponentConsole, ExpiresAt: consoleNow.Add(time.Hour)},
		DeviceState: identity.DeviceStateActive,
	}
	cases := []struct {
		name   string
		change func(*fakeSessionStore)
		want   errs.Code
	}{
		{"open", func(*fakeSessionStore) {}, ""},
		{"gone", func(s *fakeSessionStore) { s.authFound = false }, errs.CodeSessionRevoked},
		{"other person", func(s *fakeSessionStore) { s.auth.Session.UserID = ids.New() }, errs.CodeSessionRevoked},
		{"app session", func(s *fakeSessionStore) { s.auth.Session.Component = identity.ComponentApp }, errs.CodeSessionRevoked},
		{"ended", func(s *fakeSessionStore) { s.auth.Session.ExpiresAt = consoleNow }, errs.CodeSessionRevoked},
		{"device revoked", func(s *fakeSessionStore) { s.auth.DeviceState = identity.DeviceStateRevoked }, errs.CodeSessionRevoked},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := &fakeSessionStore{auth: open, authFound: true}
			tc.change(store)
			err := (&Authenticator{store: store, clk: clock.NewFake(consoleNow)}).Revalidate(context.Background(), p)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			wantCode(t, err, tc.want)
		})
	}
	err := (&Authenticator{store: &fakeSessionStore{authErr: errStoreDown}, clk: clock.NewFake(consoleNow)}).Revalidate(context.Background(), p)
	wantStoreDown(t, err)
}

// markingIdentities is an IdentityStore whose MarkIdentityLogin answers found and err.
type markingIdentities struct {
	IdentityStore
	found bool
	err   error
}

func (m markingIdentities) MarkIdentityLogin(context.Context, ids.ID, time.Time) (bool, error) {
	return m.found, m.err
}

// consoleLoginService returns a login service over a test console for an active person.
func consoleLoginService(t *testing.T) (*Service, *consoleFakes) {
	t.Helper()
	c, f := newTestConsole(t)
	f.people.user = UserRecord{User: domain.User{Principal: domain.Principal{Status: domain.StatusActive}}}
	return &Service{d: ServiceDeps{People: f.people, Console: c, Identities: markingIdentities{found: true}, Audit: f.audit, Clock: c.d.Clock}}, f
}

// TestCompleteConsoleLogin: the console form needs the console and a valid method, refuses a person
// that may not log in, hands on the errors of the start and of the identity, and records the login
// with client_component console.
func TestCompleteConsoleLogin(t *testing.T) {
	t.Parallel()
	identityID := ids.New()
	in := LoginCompletion{UserID: ids.New(), AuthMethod: identity.AuthOIDC, IdentityID: &identityID, Console: true}
	t.Run("no console", func(t *testing.T) {
		t.Parallel()
		s, _ := consoleLoginService(t)
		s.d.Console = nil
		_, err := s.completeLogin(context.Background(), in)
		wantCode(t, err, errs.CodeInternalError)
	})
	t.Run("pending person", func(t *testing.T) {
		t.Parallel()
		s, f := consoleLoginService(t)
		f.people.user.User.Principal.Status = domain.StatusPending
		pending := in
		pending.AllowPending = true
		_, err := s.completeLogin(context.Background(), pending)
		wantCode(t, err, errs.CodeUnauthenticated)
	})
	t.Run("start", func(t *testing.T) {
		t.Parallel()
		s, f := consoleLoginService(t)
		f.devices.err = errStoreDown
		_, err := s.completeLogin(context.Background(), in)
		wantStoreDown(t, err)
	})
	t.Run("identity mark", func(t *testing.T) {
		t.Parallel()
		s, _ := consoleLoginService(t)
		s.d.Identities = markingIdentities{err: errStoreDown}
		_, err := s.completeLogin(context.Background(), in)
		wantStoreDown(t, err)
	})
	t.Run("identity gone", func(t *testing.T) {
		t.Parallel()
		s, _ := consoleLoginService(t)
		s.d.Identities = markingIdentities{}
		_, err := s.completeLogin(context.Background(), in)
		wantCode(t, err, errs.CodeInternalError)
	})
	t.Run("audit", func(t *testing.T) {
		t.Parallel()
		s, f := consoleLoginService(t)
		f.audit.err = errStoreDown
		_, err := s.completeLogin(context.Background(), in)
		wantStoreDown(t, err)
	})
	t.Run("logged in", func(t *testing.T) {
		t.Parallel()
		s, f := consoleLoginService(t)
		got, err := s.completeLogin(context.Background(), in)
		if err != nil {
			t.Fatal(err)
		}
		if got.Console == nil || got.Console.Cookie.IsZero() || !got.NewDevice {
			t.Fatalf("result = %+v", got)
		}
		if len(f.audit.entries) != 1 || f.audit.entries[0].Attrs[attrClientComponent].Data() != string(identity.ComponentConsole) {
			t.Fatalf("entries = %+v", f.audit.entries)
		}
	})
}

// TestConsoleStepUpEntries: the allow entry of a re-login start names step, challenge, provider
// and flow.
func TestConsoleStepUpEntries(t *testing.T) {
	t.Parallel()
	sid, challengeID, providerID, flowID := ids.New(), ids.New(), ids.New(), ids.New()
	e := reauthEntry(sid, challengeID, providerID, flowID)
	if e.Decision != auditing.DecisionAllow || e.ResourceID != sid || e.Attrs[attrStep].Data() != stepOIDCBegin {
		t.Fatalf("entry = %+v", e)
	}
}

// appActor is a person in an app session on a device (the Windows app of D-M4-11-2 when the device
// has no step_up key).
func appActor() actor.Actor {
	a := consoleActor()
	a.ClientComponent = string(identity.ComponentApp)
	return a
}

// passwordStepUpApp is a test console with the step-up use cases and local credentials, an issued
// challenge of the app session of a on an active device of platform, which has a step_up key when
// stepUpKey is true.
func passwordStepUpApp(t *testing.T, a actor.Actor, platform identity.Platform, stepUpKey bool) (*Console, *stepUpFakes, ids.ID) {
	t.Helper()
	c, f := newTestStepUpConsole(t)
	c.local, _ = localForStepUp()
	f.devices.dev = identity.Device{ID: a.DeviceID, UserID: a.ID, Platform: platform, State: identity.DeviceStateActive}
	f.devices.found = true
	f.devices.stepUpKey = stepUpKey
	id := openChallenge(f, a)
	f.sessions.tree[0].Component = identity.ClientComponent(a.ClientComponent)
	return c, f, id
}

// TestStepUpWithPasswordFromAppWithoutStepUpKey (FIX-stepup-macos): an app session on a device
// without an active step_up key confirms with password and TOTP: 200 with method password_totp,
// the auth.step_up confirm entry with that method and no cookie, because only a console session
// has one.
func TestStepUpWithPasswordFromAppWithoutStepUpKey(t *testing.T) {
	t.Parallel()
	a := appActor()
	c, f, id := passwordStepUpApp(t, a, identity.PlatformWindows, false)
	got, err := c.StepUpWithPassword(context.Background(), a, id, "password", validTOTP())
	if err != nil {
		t.Fatal(err)
	}
	// f.store.rotated is false, so a rotation of the cookie would have failed the step-up.
	if !got.Cookie.IsZero() || got.CookieMaxAge != 0 || got.StepUpAt.IsZero() {
		t.Fatalf("result %+v, want a step-up without a cookie", got)
	}
	if len(f.challenges.methods) != 1 || f.challenges.methods[0] != identity.StepUpMethodPasswordTOTP ||
		len(f.challenges.finished) != 1 || f.challenges.finished[0] != ChallengeUsed {
		t.Fatalf("methods %v, finished %v", f.challenges.methods, f.challenges.finished)
	}
	var confirmed int
	for _, e := range f.audit.entries {
		if e.Action == string(authorize.ActionAuthStepUp) && e.Attrs[attrStep].Data() == stepConfirm &&
			e.Attrs[attrMethod].Data() == identity.StepUpMethodPasswordTOTP && e.ResourceID == a.SessionID {
			confirmed++
		}
	}
	if confirmed != 1 || len(f.audit.denials) != 0 {
		t.Fatalf("entries %+v, denials %+v", f.audit.entries, f.audit.denials)
	}
}

// TestStepUpWithPasswordFromAppWithStepUpKey: an app session on a device with an active step_up
// key gets 403 step_up_not_possible (no downgrade from the device key), audited with cause
// step_up_key_present, and the challenge stays open.
func TestStepUpWithPasswordFromAppWithStepUpKey(t *testing.T) {
	t.Parallel()
	a := appActor()
	c, f, id := passwordStepUpApp(t, a, identity.PlatformMacOS, true)
	_, err := c.StepUpWithPassword(context.Background(), a, id, "password", validTOTP())
	if e := problemOf(t, err, errs.CodeForbidden); e.Reason != reasonStepUpNotPossible {
		t.Fatalf("reason = %q", e.Reason)
	}
	if len(f.audit.denials) != 1 || f.audit.denials[0].Attrs[attrCause].Data() != causeStepUpKeyPresent {
		t.Fatalf("denials = %+v", f.audit.denials)
	}
	if len(f.challenges.finished) != 0 {
		t.Fatalf("finished = %v, want the challenge untouched", f.challenges.finished)
	}
}

// TestStepUpWithPasswordFromCLI: a session of a cli device gets 403 step_up_not_possible (08 6.4).
func TestStepUpWithPasswordFromCLI(t *testing.T) {
	t.Parallel()
	a := appActor()
	c, f, id := passwordStepUpApp(t, a, identity.PlatformCLI, false)
	_, err := c.StepUpWithPassword(context.Background(), a, id, "password", validTOTP())
	if e := problemOf(t, err, errs.CodeForbidden); e.Reason != reasonStepUpNotPossible {
		t.Fatalf("reason = %q", e.Reason)
	}
	if len(f.audit.denials) != 1 || len(f.challenges.finished) != 0 {
		t.Fatalf("denials %+v, finished %v", f.audit.denials, f.challenges.finished)
	}
}

// TestStepUpWithPasswordOtherComponents: a share extension session gets 403 step_up_not_possible
// with cause not_console, and a failing key read is handed on.
func TestStepUpWithPasswordOtherComponents(t *testing.T) {
	t.Parallel()
	t.Run("share extension", func(t *testing.T) {
		t.Parallel()
		a := appActor()
		a.ClientComponent = string(identity.ComponentShareExt)
		c, f, id := passwordStepUpApp(t, a, identity.PlatformWindows, false)
		_, err := c.StepUpWithPassword(context.Background(), a, id, "password", validTOTP())
		if e := problemOf(t, err, errs.CodeForbidden); e.Reason != reasonStepUpNotPossible {
			t.Fatalf("reason = %q", e.Reason)
		}
		if len(f.audit.denials) != 1 || f.audit.denials[0].Attrs[attrCause].Data() != string(causeNotConsole) {
			t.Fatalf("denials = %+v", f.audit.denials)
		}
	})
	t.Run("key read", func(t *testing.T) {
		t.Parallel()
		a := appActor()
		c, f, id := passwordStepUpApp(t, a, identity.PlatformWindows, false)
		f.devices.keyErr = errStoreDown
		_, err := c.StepUpWithPassword(context.Background(), a, id, "password", validTOTP())
		wantStoreDown(t, err)
	})
	t.Run("oidc stays console-only", func(t *testing.T) {
		t.Parallel()
		a := appActor()
		c, f, _ := passwordStepUpApp(t, a, identity.PlatformWindows, false)
		c.oidc = &OIDC{}
		_, err := c.BeginStepUpOIDC(context.Background(), a, identity.StepUpOIDCBegin{AppRedirectURI: c.stepUpCallback()})
		if e := problemOf(t, err, errs.CodeForbidden); e.Reason != reasonStepUpNotPossible {
			t.Fatalf("reason = %q", e.Reason)
		}
		if len(f.audit.denials) != 1 || f.audit.denials[0].Attrs[attrCause].Data() != string(causeNotConsole) {
			t.Fatalf("denials = %+v", f.audit.denials)
		}
	})
}
