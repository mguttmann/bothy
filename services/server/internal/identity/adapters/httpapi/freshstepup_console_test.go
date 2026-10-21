// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	authzpg "github.com/OWNER/bothy/services/server/internal/authz/adapters/postgres"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// consoleFresh is the console world of M6-01 with an Authorizer that knows one host, H, assigned
// to Alex with a reported session user of Alex (ID-T28, TM-T07 and TM-T12, console part, M6-18).
// tab is Alex's console tab, logged in with password and TOTP.
type consoleFresh struct {
	*cworld
	az   *authzapp.Authorizer
	host ids.ID
	tab  *consoleClient
}

// newConsoleFresh builds the world: the real Authenticator, the real authorizer and the real
// consumption of identity over PostgreSQL, the passkey from virtualauthn.
func newConsoleFresh(t *testing.T) *consoleFresh {
	t.Helper()
	w := &consoleFresh{cworld: newConsoleWorld(t), host: ids.New()}
	hosts := fake.NewHosts()
	hosts.Put(authz.HostFacts{ID: w.host, State: "active", AssignedUserID: w.alex, LLMCredentials: "organization"})
	hosts.PutSessionUser(w.alex, authz.SessionUserFact{HostID: w.host, HostSessionUserID: ids.New(), OSUser: "bothy-s-alex", Reported: true})
	repo := postgres.NewRepository(w.pool)
	az, err := authzapp.NewAuthorizer(authzapp.AuthorizerConfig{SnapshotCacheMaxEntries: 100, StepUpMaxAge: sessCfg.StepUpMaxAge,
		TerminalStepUpMaxAge: testFreshAge, QuickReattachWindow: time.Minute}, authzapp.AuthorizerDeps{
		Catalog: authzapp.NewCatalog(slog.New(slog.DiscardHandler)), Store: authzpg.NewRepository(w.pool), Principals: w.principals,
		Credentials: app.NewCredentialSource(repo, repo, repo), Structure: w.structure, Policies: w.policies, Hosts: hosts,
		Versions: instanceapp.NewVersions(instancepg.NewRepository(w.pool)), Recorder: w.rec, Clock: w.clk, Log: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	w.az = az
	w.tab = w.newConsole()
	w.tab.login()
	return w
}

// consoleActor authenticates the tab's cookie as the request of action does (the pre-step-up gate
// of the console reads the action of the transport).
func (w *consoleFresh) consoleActor(action authorize.Action) (actor.Actor, error) {
	ctx := reqmeta.WithTransport(context.Background(), reqmeta.Transport{Action: string(action)})
	return w.id.Authenticator.Authenticate(ctx, actor.Credentials{ConsoleCookie: actor.NewCookieValue(w.tab.cookie())})
}

// mustActor is consoleActor that fails the test on an error.
func (w *consoleFresh) mustActor(action authorize.Action) actor.Actor {
	w.t.Helper()
	a, err := w.consoleActor(action)
	if err != nil {
		w.t.Fatalf("console actor: %v", err)
	}
	return a
}

// decideOpen authorizes terminal.open on H for a.
func (w *consoleFresh) decideOpen(a actor.Actor) authorize.Decision {
	w.t.Helper()
	d, err := w.az.Authorize(context.Background(), a, authorize.ActionTerminalOpen, authorize.Resource{Kind: authorize.ResourceHost, ID: w.host,
		Terminal: &authorize.TerminalFacts{HostConnected: true, ProfileKnown: true}})
	if err != nil {
		w.t.Fatal(err)
	}
	return d
}

// decideAttach authorizes terminal.attach for a on a session of Alex on H that the tab's console
// session and device opened and that was detached at detachedAt.
func (w *consoleFresh) decideAttach(a actor.Actor, deviceID, sessionID ids.ID, detachedAt time.Time) authorize.Decision {
	w.t.Helper()
	r := authorize.Resource{Kind: authorize.ResourceTerminalSession, ID: ids.New(), HostID: w.host, OwnerID: w.alex,
		Terminal: &authorize.TerminalFacts{SessionState: "detached", SessionDeviceID: deviceID, SessionAuthSessionID: sessionID,
			DetachedAt: detachedAt, HostConnected: true}}
	d, err := w.az.Authorize(context.Background(), a, authorize.ActionTerminalAttach, r)
	if err != nil {
		w.t.Fatal(err)
	}
	return d
}

// consume runs ConsumeFresh for a in its own unit of work, the transaction of an opening.
func (w *consoleFresh) consume(a actor.Actor) error {
	return w.uow.Do(context.Background(), func(ctx context.Context) error { return w.id.StepUps.ConsumeFresh(ctx, a) })
}

// wantPasskeyRequired checks a decision step_up_required whose problem is 401 step_up_required
// with reason passkey_required.
func wantPasskeyRequired(t *testing.T, what string, d authorize.Decision) {
	t.Helper()
	e, _ := errs.As(d.Err())
	if d.Outcome != authorize.OutcomeStepUpRequired || e == nil || e.Code != errs.CodeStepUpRequired || e.Kind().Status() != http.StatusUnauthorized ||
		e.Reason != "passkey_required" {
		t.Fatalf("%s: decision %+v problem %v, want 401 step_up_required with passkey_required", what, d, d.Err())
	}
}

// TestIDT28_Console (ID-T28 console part, 08 section 6.3 points 3 and 4, M6-18): a console session
// with a password-TOTP step-up, an OIDC step-up, a refused passkey without user verification and a
// passkey step-up whose stored WebAuthn session did not require user verification, a UV passkey
// older than terminal.step_up_max_age and a consumed one get 401 step_up_required with
// passkey_required (D-M6-18-1); a fresh UV passkey opens; two simultaneous openings with one
// confirmation consume it once.
func TestIDT28_Console(t *testing.T) {
	t.Parallel()
	w := newConsoleFresh(t)
	w.want(w.tab.passwordStepUp(testPassword), http.StatusOK, "")
	wantPasskeyRequired(t, "password_totp", w.decideOpen(w.mustActor(authorize.ActionTerminalOpen)))

	resp, _ := w.tab.oidcStepUp(0)
	w.want(resp, http.StatusOK, "")
	wantPasskeyRequired(t, "oidc_reauth", w.decideOpen(w.mustActor(authorize.ActionTerminalOpen)))

	w.passkey.NoUserVerification = true
	if r := w.tab.passkeyStepUp(); r.StatusCode == http.StatusOK {
		t.Fatal("a passkey assertion without user verification confirmed the step-up")
	}
	w.passkey.NoUserVerification = false
	a := w.mustActor(authorize.ActionTerminalOpen)
	if a.StepUpMethod == "passkey" {
		t.Fatal("the refused passkey set the step-up")
	}
	wantPasskeyRequired(t, "passkey without user verification", w.decideOpen(a))

	w.want(w.tab.passkeyStepUp(), http.StatusOK, "")
	a = w.mustActor(authorize.ActionTerminalOpen)
	if a.StepUpMethod != "passkey" || !a.StepUpUserVerified {
		t.Fatalf("the Authenticator read method %q, user verified %v", a.StepUpMethod, a.StepUpUserVerified)
	}
	sid := w.tab.sessionID()
	w.sql(`UPDATE auth_challenges SET webauthn_session = jsonb_set(webauthn_session, '{user_verification}', '"preferred"')
		WHERE session_id = $1 AND status = 'used'`, sid.String())
	a = w.mustActor(authorize.ActionTerminalOpen)
	if a.StepUpUserVerified {
		t.Fatal("a stored WebAuthn session without required user verification counts as verified")
	}
	wantPasskeyRequired(t, "stored session without userVerification required", w.decideOpen(a))
	if err := w.consume(a); errs.KindOf(err) != errs.KindStepUpRequired {
		t.Fatalf("ConsumeFresh of a passkey without user verification: %v", err)
	} else if e, _ := errs.As(err); e.Reason != "passkey_required" {
		t.Fatalf("ConsumeFresh reason %q, want passkey_required", e.Reason)
	}

	w.clk.Advance(time.Second)
	w.want(w.tab.passkeyStepUp(), http.StatusOK, "")
	a = w.mustActor(authorize.ActionTerminalOpen)
	if d := w.decideOpen(a); !d.Allowed() {
		t.Fatalf("a fresh UV passkey: %+v", d)
	}
	const n = 8
	var wg sync.WaitGroup
	results := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = w.consume(a)
		}()
	}
	wg.Wait()
	ok := 0
	for _, err := range results {
		switch {
		case err == nil:
			ok++
		case errs.KindOf(err) != errs.KindStepUpRequired:
			t.Fatalf("a loser failed with %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("%d openings consumed one confirmation", ok)
	}
	wantPasskeyRequired(t, "consumed UV passkey", w.decideOpen(w.mustActor(authorize.ActionTerminalOpen)))

	w.clk.Advance(time.Second)
	w.want(w.tab.passkeyStepUp(), http.StatusOK, "")
	w.clk.Advance(testFreshAge + time.Second)
	wantPasskeyRequired(t, "UV passkey older than terminal.step_up_max_age", w.decideOpen(w.mustActor(authorize.ActionTerminalOpen)))
}

// TestTMT07_Console (TM-T07 console part, M6-18): terminal.open from the console without a
// step-up is refused with 401 step_up_required (the pre-step-up gate of M6-01), and with a
// password step-up with 401 step_up_required and passkey_required.
func TestTMT07_Console(t *testing.T) {
	t.Parallel()
	w := newConsoleFresh(t)
	if _, err := w.consoleActor(authorize.ActionTerminalOpen); errs.KindOf(err) != errs.KindStepUpRequired {
		t.Fatalf("console without a step-up: %v", err)
	}
	w.want(w.tab.passwordStepUp(testPassword), http.StatusOK, "")
	wantPasskeyRequired(t, "password step-up", w.decideOpen(w.mustActor(authorize.ActionTerminalOpen)))
}

// TestTMT12_Console (TM-T12 console part, 24 section 6.2, M6-18): terminal.attach from the console
// session and device that opened the session, within terminal.quick_reattach_window and without a
// new passkey, gets 401 with passkey_required; with a new UV passkey it passes without the quick
// reattach; the app device within the window keeps the quick reattach (regression).
func TestTMT12_Console(t *testing.T) {
	t.Parallel()
	w := newConsoleFresh(t)
	w.want(w.tab.passkeyStepUp(), http.StatusOK, "")
	opener := w.mustActor(authorize.ActionTerminalAttach)
	if err := w.consume(opener); err != nil {
		t.Fatalf("the opening: %v", err)
	}
	detached := w.clk.Now()
	w.clk.Advance(time.Second)
	wantPasskeyRequired(t, "console attach within the window", w.decideAttach(w.mustActor(authorize.ActionTerminalAttach), opener.DeviceID, opener.SessionID, detached))
	w.want(w.tab.passkeyStepUp(), http.StatusOK, "")
	if d := w.decideAttach(w.mustActor(authorize.ActionTerminalAttach), opener.DeviceID, opener.SessionID, detached); !d.Allowed() || d.QuickReattach {
		t.Fatalf("console attach with a new UV passkey: %+v", d)
	}
	phone := w.phone.client.actor()
	if d := w.decideAttach(phone, phone.DeviceID, phone.SessionID, w.clk.Now()); !d.Allowed() || !d.QuickReattach {
		t.Fatalf("the app's quick reattach: %+v", d)
	}
}
