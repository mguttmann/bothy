// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"log/slog"
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
)

// testFreshAge is terminal.step_up_max_age of the identity test worlds (M2-40).
const testFreshAge = time.Minute

// freshWorld is the session world with an Authorizer that knows one host: H, assigned to the
// person, with a reported session user of the person (ID-T28 without the console part, M2-40).
type freshWorld struct {
	*sworld
	az   *authzapp.Authorizer
	host ids.ID
	alex ids.ID
	c    *stepClient
}

// newFreshWorld builds the world with the owner person on a phone with a step_up key.
func newFreshWorld(t *testing.T) *freshWorld {
	t.Helper()
	w := &freshWorld{sworld: newSessionWorld(t), host: ids.New()}
	w.alex = w.person("alex", true)
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
	w.c = w.phone(w.alex)
	return w
}

// decideOpen authorizes terminal.open on H for a.
func (w *freshWorld) decideOpen(a actor.Actor) authorize.Decision {
	w.t.Helper()
	d, err := w.az.Authorize(context.Background(), a, authorize.ActionTerminalOpen, authorize.Resource{Kind: authorize.ResourceHost, ID: w.host,
		Terminal: &authorize.TerminalFacts{HostConnected: true, ProfileKnown: true}})
	if err != nil {
		w.t.Fatal(err)
	}
	return d
}

// consume runs ConsumeFresh for a in its own unit of work, the transaction of an opening.
func (w *freshWorld) consume(a actor.Actor) error {
	return w.uow.Do(context.Background(), func(ctx context.Context) error { return w.id.StepUps.ConsumeFresh(ctx, a) })
}

// openOnce is the open harness of M2-44: authorize, then the unit of work with ConsumeFresh.
func (w *freshWorld) openOnce(a actor.Actor) error {
	if err := w.decideOpen(a).Err(); err != nil {
		return err
	}
	return w.consume(a)
}

// stepUpAt reads step_up_at of the session of c.
func (w *freshWorld) stepUpAt() time.Time {
	w.t.Helper()
	var at time.Time
	w.scalar(&at, `SELECT step_up_at FROM auth_sessions WHERE id = $1`, w.c.session.String())
	return at.UTC()
}

// wantStale checks a decision step_up_required with terminal_step_up_not_fresh.
func wantStale(t *testing.T, d authorize.Decision) {
	t.Helper()
	if d.Outcome != authorize.OutcomeStepUpRequired || d.Reason != "terminal_step_up_not_fresh" {
		t.Fatalf("decision %+v, want step_up_required/terminal_step_up_not_fresh", d)
	}
}

// TestIDT28_PasswordTotpAndOidcStepUp (ID-T28, 08 section 6.3 point 2): a fresh step-up by
// password with TOTP or by OIDC, read through the real Authenticator, is not fresh for the
// terminal.
func TestIDT28_PasswordTotpAndOidcStepUp(t *testing.T) {
	t.Parallel()
	w := newFreshWorld(t)
	for _, method := range []string{"password_totp", "oidc_reauth"} {
		w.sql(`UPDATE auth_sessions SET step_up_at = $2, step_up_method = $3 WHERE id = $1`, w.c.session.String(), w.clk.Now(), method)
		a := w.c.actor()
		if a.StepUpMethod != method || a.StepUpAt.IsZero() {
			t.Fatalf("the Authenticator read %q at %v", a.StepUpMethod, a.StepUpAt)
		}
		wantStale(t, w.decideOpen(a))
	}
}

// TestIDT28_StepUpTooOld (ID-T28, 08 section 6.3 point 1): a step-up by the device key is fresh up
// to terminal.step_up_max_age inclusive and stale one nanosecond later.
func TestIDT28_StepUpTooOld(t *testing.T) {
	t.Parallel()
	w := newFreshWorld(t)
	w.c.stepUp()
	at := w.stepUpAt()
	w.clk.Set(at.Add(testFreshAge))
	if d := w.decideOpen(w.c.actor()); !d.Allowed() {
		t.Fatalf("at exactly terminal.step_up_max_age: %+v", d)
	}
	w.clk.Set(at.Add(testFreshAge + time.Nanosecond))
	wantStale(t, w.decideOpen(w.c.actor()))
}

// TestIDT28_TwoOpeningsNeedTwoConfirmations (ID-T28, AZ-M58, 08 section 6.3 point 4): one
// confirmation opens once; the next request's actor carries the consumption and fails at
// authorize; a new step-up opens again; other step-up actions ignore the consumption.
func TestIDT28_TwoOpeningsNeedTwoConfirmations(t *testing.T) {
	t.Parallel()
	w := newFreshWorld(t)
	w.c.stepUp()
	if err := w.openOnce(w.c.actor()); err != nil {
		t.Fatalf("first opening: %v", err)
	}
	second := w.c.actor()
	if second.StepUpConsumedAt.IsZero() {
		t.Fatal("the Authenticator does not read step_up_consumed_at")
	}
	wantStale(t, w.decideOpen(second))
	w.clk.Advance(time.Second)
	w.c.stepUp()
	if err := w.openOnce(w.c.actor()); err != nil {
		t.Fatalf("third opening after a new step-up: %v", err)
	}
	g := authorize.Resource{Kind: authorize.ResourceInstance, Grant: &authorize.GrantFacts{SubjectKind: "user", Level: "read", TargetKind: "instance"}}
	d, err := w.az.Authorize(context.Background(), w.c.actor(), authorize.ActionGrantCreate, g)
	if err != nil || !d.Allowed() {
		t.Fatalf("grant.create after a consumption: %+v %v", d, err)
	}
}

// TestIDT28_ConcurrentOpeningsOneConfirmation (ID-T28, T-TM-03): eight openings with one
// confirmation all pass authorize, and exactly one consumes the step-up.
func TestIDT28_ConcurrentOpeningsOneConfirmation(t *testing.T) {
	t.Parallel()
	w := newFreshWorld(t)
	w.c.stepUp()
	a := w.c.actor()
	const n = 8
	for range n {
		if d := w.decideOpen(a); !d.Allowed() {
			t.Fatalf("authorize: %+v", d)
		}
	}
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
	if n := w.count(`SELECT count(*) FROM auth_sessions WHERE id = $1 AND step_up_consumed_at IS NOT NULL`, w.c.session.String()); n != 1 {
		t.Fatal("the consumption is not stored")
	}
}

// TestConsumeFresh_Rules (Design note 4): outside a unit of work, a revoked session, a console
// session and a step-up that aged out between authorize and the lock.
func TestConsumeFresh_Rules(t *testing.T) {
	t.Parallel()
	w := newFreshWorld(t)
	w.c.stepUp()
	a := w.c.actor()
	if err := w.id.StepUps.ConsumeFresh(context.Background(), a); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("outside a unit of work: %v", err)
	}
	if d := w.decideOpen(a); !d.Allowed() {
		t.Fatalf("authorize: %+v", d)
	}
	w.clk.Advance(testFreshAge + time.Second)
	if err := w.consume(a); errs.KindOf(err) != errs.KindStepUpRequired {
		t.Fatalf("aged out before the lock: %v", err)
	}
	w.clk.Advance(time.Second)
	w.c.stepUp()
	a = w.c.actor()
	w.sql(`UPDATE auth_sessions SET client_component = 'console', cookie_hash = '\x01' WHERE id = $1`, w.c.session.String())
	if err := w.consume(a); errs.KindOf(err) != errs.KindStepUpRequired {
		t.Fatalf("console session: %v", err)
	}
	w.sql(`UPDATE auth_sessions SET client_component = 'app', cookie_hash = NULL, revoked_at = now(), revoke_reason = 'admin' WHERE id = $1`, w.c.session.String())
	if err := w.consume(a); errs.KindOf(err) != errs.KindUnauthenticated {
		t.Fatalf("revoked session: %v", err)
	} else if e, _ := errs.As(err); e.Code != errs.CodeSessionRevoked {
		t.Fatalf("revoked session: %v", err)
	}
	if n := w.count(`SELECT count(*) FROM auth_sessions WHERE id = $1 AND step_up_consumed_at IS NOT NULL`, w.c.session.String()); n != 0 {
		t.Fatal("a refused consumption was stored")
	}
}
