// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
)

// The unit tests of the job runs.app_dispatch (brief M2-25 design note 8) with fakes for the
// dispatcher of apps, the authorizer and the component rows.

// dispatchComponents is an in-memory app.ComponentRepository with the rows the job reads; every
// other method is not used (nil interface, a call panics).
type dispatchComponents struct {
	app.ComponentRepository
	mu           sync.Mutex
	interactions map[ids.ID]app.InteractionRow
	components   map[ids.ID]app.ComponentRow
	// states records every state the job wrote.
	states []string
}

func (c *dispatchComponents) Interaction(_ context.Context, id ids.ID) (app.InteractionRow, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.interactions[id]
	return r, ok, nil
}

func (c *dispatchComponents) LockInteraction(ctx context.Context, id ids.ID) (app.InteractionRow, bool, error) {
	return c.Interaction(ctx, id)
}

func (c *dispatchComponents) Component(_ context.Context, id ids.ID) (app.ComponentRow, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.components[id]
	return r, ok, nil
}

func (c *dispatchComponents) SetInteractionState(_ context.Context, id ids.ID, state string, _ []byte, at time.Time) (app.InteractionRow, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r := c.interactions[id]
	r.State, r.UpdatedAt = state, at
	c.interactions[id] = r
	c.states = append(c.states, state)
	return r, nil
}

// state returns the stored state of id.
func (c *dispatchComponents) state(id ids.ID) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.interactions[id].State
}

// dispatchCards is the card port; the job does not use it.
type dispatchCards struct{ app.CardMessages }

// dispatchThreads is the thread lock; the job does not use it.
type dispatchThreads struct{}

func (dispatchThreads) LockForMessageChange(context.Context, ids.ID) error { return nil }

// fakeDispatcher is a scripted runs.AppDispatcher.
type fakeDispatcher struct {
	mu    sync.Mutex
	way   runs.AppWay
	out   runs.AppOutcome
	err   error
	posts []runs.AppPost
	// during runs inside Post, after it was recorded (a concurrent respondToInteraction).
	during func()
	// ask is the outcome of Ask; asks records its requests.
	ask  runs.AppOutcome
	asks []runs.AppAsk
}

func (f *fakeDispatcher) Way(context.Context, ids.ID) (runs.AppWay, error) { return f.way, nil }

func (f *fakeDispatcher) Post(_ context.Context, p runs.AppPost) (runs.AppOutcome, error) {
	f.mu.Lock()
	f.posts = append(f.posts, p)
	during := f.during
	f.mu.Unlock()
	if during != nil {
		during()
	}
	return f.out, f.err
}

func (f *fakeDispatcher) Ask(_ context.Context, a runs.AppAsk) (runs.AppOutcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asks = append(f.asks, a)
	return f.ask, nil
}

func (f *fakeDispatcher) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.posts)
}

// appCommands is a command register with one app command.
type appCommands struct{ c agents.Command }

func (a appCommands) Resolve(_ context.Context, id, _ ids.ID) (agents.Command, error) {
	if id != a.c.ID {
		return agents.Command{}, errors.New("unknown")
	}
	return a.c, nil
}

// dispatchWorld is the world of the job tests: the runs world with components on, an app card
// of Tracker in the main thread and one interaction of Alex on it.
type dispatchWorld struct {
	*world
	comps                *dispatchComponents
	disp                 *fakeDispatcher
	tracker, card, comp  ids.ID
	interaction, eventID ids.ID
	cmd                  agents.Command
}

func newDispatchWorld(t *testing.T) *dispatchWorld {
	t.Helper()
	w := newWorld(t)
	dw := &dispatchWorld{world: w, tracker: ids.New(), card: ids.New(), comp: ids.New(), interaction: ids.New(), eventID: ids.New(),
		disp: &fakeDispatcher{way: runs.AppWayInteractionsURL}}
	dw.comps = &dispatchComponents{
		interactions: map[ids.ID]app.InteractionRow{dw.interaction: {ID: dw.interaction, ComponentID: dw.comp, MessageID: dw.card,
			PrincipalID: w.alex, State: "received"}},
		components: map[ids.ID]app.ComponentRow{dw.comp: {ID: dw.comp, MessageID: dw.card, CustomID: "ok", Type: "button", State: "used",
			RestrictedTo: "any", ExpiresAt: t0.Add(time.Hour)}},
	}
	w.msgFacts.m[dw.card] = messaging.TriggerMessage{ID: dw.card, Kind: "card", Origin: "app", ThreadID: w.main, ChannelID: w.channel,
		Author: messaging.PrincipalRef{ID: dw.tracker, Kind: "app", Handle: "tracker", DisplayName: "Tracker"}}
	d := w.deps()
	d.Components, d.Cards, d.ThreadLocks = dw.comps, dispatchCards{}, dispatchThreads{}
	svc, err := app.New(w.config(), d)
	if err != nil {
		t.Fatal(err)
	}
	w.svc = svc
	if err := svc.BindAppDispatcher(dw.disp); err != nil {
		t.Fatal(err)
	}
	dw.cmd = agents.Command{ID: ids.New(), OwnerPrincipalID: dw.tracker, OwnerKind: "app", Name: "ticket", RequiredLevel: "trigger"}
	if err := svc.BindCommands(appCommands{dw.cmd}); err != nil {
		t.Fatal(err)
	}
	return dw
}

// interactionJob is the job of the interaction of the world.
func (dw *dispatchWorld) interactionJob() app.DispatchJob {
	return app.DispatchJob{Kind: app.AppDispatchInteraction, EventID: dw.eventID, AppID: dw.tracker, PersonID: dw.alex,
		ChannelID: dw.channel, ThreadID: dw.main, InteractionID: dw.interaction}
}

// commandJob is the job of an invocation of the command of the world.
func (dw *dispatchWorld) commandJob() app.DispatchJob {
	return app.DispatchJob{Kind: app.AppDispatchCommand, EventID: dw.eventID, AppID: dw.tracker, PersonID: dw.alex,
		ChannelID: dw.channel, ThreadID: dw.main, CommandID: dw.cmd.ID}
}

// TestAppDispatch_GatewayWayDoesNothing: an app without interactions_url gets its event through
// its connection; the job sends nothing, checks nothing and the interaction stays received.
func TestAppDispatch_GatewayWayDoesNothing(t *testing.T) {
	dw := newDispatchWorld(t)
	dw.disp.way = runs.AppWayGateway
	for _, j := range []app.DispatchJob{dw.interactionJob(), dw.commandJob()} {
		if err := dw.svc.DispatchToApp(context.Background(), j); err != nil {
			t.Fatal(err)
		}
	}
	if dw.disp.count() != 0 || dw.comps.state(dw.interaction) != "received" || dw.j.count("authz:interaction.create:filter") != 0 {
		t.Fatalf("posts %d, state %s", dw.disp.count(), dw.comps.state(dw.interaction))
	}
}

// TestAppDispatch_InactiveFailsInteraction: a suspended or uninstalled app gets no request and the
// interaction becomes failed; a command stores nothing.
func TestAppDispatch_InactiveFailsInteraction(t *testing.T) {
	dw := newDispatchWorld(t)
	dw.disp.way = runs.AppWayInactive
	if err := dw.svc.DispatchToApp(context.Background(), dw.commandJob()); err != nil {
		t.Fatal(err)
	}
	if err := dw.svc.DispatchToApp(context.Background(), dw.interactionJob()); err != nil {
		t.Fatal(err)
	}
	if dw.disp.count() != 0 || dw.comps.state(dw.interaction) != "failed" {
		t.Fatalf("posts %d, state %s", dw.disp.count(), dw.comps.state(dw.interaction))
	}
}

// TestAppDispatch_RecheckDeniedNoRequest: the re-check with the rights of this moment runs in
// filter mode for the person as the principal of the check without a request (R-AZ-29); a denial
// sends nothing and fails the interaction. The interaction is checked at stage deliver without
// trigger facts, the command with the app owner of the command.
func TestAppDispatch_RecheckDeniedNoRequest(t *testing.T) {
	dw := newDispatchWorld(t)
	dw.authz.decide = func(act authorize.Action, filter bool, r authorize.Resource) decision {
		return decision{reason: "component_inactive"}
	}
	if err := dw.svc.DispatchToApp(context.Background(), dw.interactionJob()); err != nil {
		t.Fatal(err)
	}
	if err := dw.svc.DispatchToApp(context.Background(), dw.commandJob()); err != nil {
		t.Fatal(err)
	}
	if dw.disp.count() != 0 || dw.comps.state(dw.interaction) != "failed" {
		t.Fatalf("posts %d, state %s", dw.disp.count(), dw.comps.state(dw.interaction))
	}
	var sawInteraction, sawCommand bool
	for _, c := range dw.j.list() {
		switch c.what {
		case "authz:interaction.create:filter":
			r := c.arg.(authorize.Resource)
			sawInteraction = r.Invoke == nil && r.Interaction != nil && r.Interaction.Stage == authorize.InteractionStageDeliver &&
				r.Interaction.OwnerKind == "app" && r.OwnerID == dw.tracker
		case "authz:command.invoke:filter":
			r := c.arg.(authorize.Resource)
			sawCommand = r.Command != nil && r.Command.OwnerID == dw.tracker && r.Command.OwnerKind == "app" && r.ThreadID == dw.main
		case "authz:interaction.create:request", "authz:command.invoke:request":
			t.Fatalf("a re-check in request mode: %s", c.what)
		}
	}
	if !sawInteraction || !sawCommand {
		t.Fatalf("re-checks interaction %v command %v", sawInteraction, sawCommand)
	}
	for _, p := range dw.authz.principals {
		if !authorize.IsRecheckPrincipal(p) || p.PrincipalID() != dw.alex {
			t.Fatalf("principal %+v is not the re-check principal of Alex", p)
		}
	}
}

// TestAppDispatch_OutcomesToStates: answered gives acknowledged, deferred keeps dispatched, every
// other reason gives failed; the request names the stored event and the card's thread.
func TestAppDispatch_OutcomesToStates(t *testing.T) {
	cases := []struct {
		out  runs.AppOutcome
		want string
	}{{runs.AppOutcome{Answered: true}, "acknowledged"}, {runs.AppOutcome{Deferred: true}, "dispatched"}}
	for _, r := range runs.AppReasons {
		cases = append(cases, struct {
			out  runs.AppOutcome
			want string
		}{runs.AppOutcome{Reason: r}, "failed"})
	}
	for _, c := range cases {
		dw := newDispatchWorld(t)
		dw.disp.out = c.out
		if err := dw.svc.DispatchToApp(context.Background(), dw.interactionJob()); err != nil {
			t.Fatal(err)
		}
		if got := dw.comps.state(dw.interaction); got != c.want {
			t.Errorf("%+v: state %s, want %s", c.out, got, c.want)
		}
		if dw.disp.count() != 1 || dw.disp.posts[0] != (runs.AppPost{AppID: dw.tracker, EventID: dw.eventID, ReplyThreadID: dw.main}) {
			t.Fatalf("posts %+v", dw.disp.posts)
		}
	}
}

// TestAppDispatch_RespondWinsOverJob: respondToInteraction that leaves dispatched while the
// request runs keeps its state; an interaction already answered gets no request at all.
func TestAppDispatch_RespondWinsOverJob(t *testing.T) {
	dw := newDispatchWorld(t)
	dw.disp.out = runs.AppOutcome{Reason: runs.AppReasonTimeout}
	dw.disp.during = func() {
		_, _ = dw.comps.SetInteractionState(context.Background(), dw.interaction, "acknowledged", nil, t0)
	}
	if err := dw.svc.DispatchToApp(context.Background(), dw.interactionJob()); err != nil {
		t.Fatal(err)
	}
	if got := dw.comps.state(dw.interaction); got != "acknowledged" {
		t.Fatalf("state %s, want the app's acknowledged", got)
	}
	for _, st := range []string{"acknowledged", "rejected", "failed"} {
		dw := newDispatchWorld(t)
		_, _ = dw.comps.SetInteractionState(context.Background(), dw.interaction, st, nil, t0)
		if err := dw.svc.DispatchToApp(context.Background(), dw.interactionJob()); err != nil {
			t.Fatal(err)
		}
		if dw.disp.count() != 0 || dw.comps.state(dw.interaction) != st {
			t.Fatalf("%s: posts %d, state %s", st, dw.disp.count(), dw.comps.state(dw.interaction))
		}
	}
	// A rerun of a job whose interaction is dispatched sends again (same event, same webhook-id).
	dw = newDispatchWorld(t)
	_, _ = dw.comps.SetInteractionState(context.Background(), dw.interaction, "dispatched", nil, t0)
	dw.disp.out = runs.AppOutcome{Answered: true}
	if err := dw.svc.DispatchToApp(context.Background(), dw.interactionJob()); err != nil {
		t.Fatal(err)
	}
	if dw.disp.count() != 1 || dw.comps.state(dw.interaction) != "acknowledged" {
		t.Fatalf("rerun: posts %d, state %s", dw.disp.count(), dw.comps.state(dw.interaction))
	}
}

// TestAppDispatch_ReturnsNilAfterRequest: one request, no repetition: a fault after the request
// (runs.ErrAfterRequest) returns nil and fails the interaction; a fault before the request returns
// its error, so the job system runs the job again, which proceeds from dispatched and sends; only
// the end of the job's context returns its error after a request.
func TestAppDispatch_ReturnsNilAfterRequest(t *testing.T) {
	dw := newDispatchWorld(t)
	dw.disp.err = fmt.Errorf("%w: the reply could not be stored", runs.ErrAfterRequest)
	if err := dw.svc.DispatchToApp(context.Background(), dw.interactionJob()); err != nil {
		t.Fatalf("after the request: %v", err)
	}
	if dw.disp.count() != 1 || dw.comps.state(dw.interaction) != "failed" {
		t.Fatalf("posts %d, state %s", dw.disp.count(), dw.comps.state(dw.interaction))
	}

	dw = newDispatchWorld(t)
	before := errors.New("the database is not reachable")
	dw.disp.err = before
	if err := dw.svc.DispatchToApp(context.Background(), dw.interactionJob()); !errors.Is(err, before) {
		t.Fatalf("before the request: %v, want the fault for a retry", err)
	}
	if st := dw.comps.state(dw.interaction); st != "dispatched" {
		t.Fatalf("state after a fault before the request: %s", st)
	}
	dw.disp.err, dw.disp.out = nil, runs.AppOutcome{Answered: true}
	if err := dw.svc.DispatchToApp(context.Background(), dw.interactionJob()); err != nil {
		t.Fatal(err)
	}
	if dw.disp.count() != 2 || dw.comps.state(dw.interaction) != "acknowledged" {
		t.Fatalf("retry: posts %d, state %s", dw.disp.count(), dw.comps.state(dw.interaction))
	}

	dw = newDispatchWorld(t)
	ctx, cancel := context.WithCancel(context.Background())
	dw.disp.during = cancel
	dw.disp.out = runs.AppOutcome{Reason: runs.AppReasonTransport}
	if err := dw.svc.DispatchToApp(ctx, dw.commandJob()); !errors.Is(err, context.Canceled) {
		t.Fatalf("shutdown: %v", err)
	}
}
