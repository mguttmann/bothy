// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/stats"
	"github.com/OWNER/bothy/services/server/internal/stats/app"
	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// liveAuthz decides by the action and the principal: owner may everything, manager may
// stats.read and cost.read.aggregate on the space support only, everyone else nothing (forbidden);
// an unknown space is not_found. Scope gives every channel to the owner and the support channel
// to the manager.
type liveAuthz struct {
	owner, manager, support, supportCh, devCh ids.ID
}

func (a *liveAuthz) Authorize(_ context.Context, p authorize.Principal, _ authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	id := p.PrincipalID()
	switch {
	case id == a.owner:
		return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
	case r.Kind == authorize.ResourceSpace && r.ID != a.support:
		return authorize.Decision{Outcome: authorize.OutcomeNotFound}, nil
	case id == a.manager && r.Kind == authorize.ResourceSpace:
		return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeForbidden}, nil
}

func (a *liveAuthz) Scope(_ context.Context, p authorize.Principal, _ authorize.Action) (authorize.Scope, error) {
	switch p.PrincipalID() {
	case a.owner:
		return authorize.Scope{ChannelIDs: []ids.ID{a.supportCh, a.devCh}}, nil
	case a.manager:
		return authorize.Scope{ChannelIDs: []ids.ID{a.supportCh}}, nil
	}
	return authorize.Scope{}, nil
}

func (*liveAuthz) Observe(context.Context, authorize.LogEvent) {}

// liveSources are the fakes of runs, chat, agents and the store; reads counts LiveRuns.
type liveSources struct {
	mu    sync.Mutex
	reads int
	runs  []runs.LiveRun
	place []chat.ChannelPlace
	rates []app.LiveRateRow
}

func (s *liveSources) LiveRuns(context.Context, int) ([]runs.LiveRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads++
	return s.runs, nil
}

func (s *liveSources) ChannelPlaces(context.Context) ([]chat.ChannelPlace, error) {
	return s.place, nil
}

func (s *liveSources) RunFacts(_ context.Context, list []ids.ID) (map[ids.ID]agents.RunFacts, error) {
	out := map[ids.ID]agents.RunFacts{}
	for _, id := range list {
		out[id] = agents.RunFacts{ID: id, BillingMode: agents.BillingOrganization}
	}
	return out, nil
}

func (s *liveSources) Heartbeat(context.Context, json.RawMessage) (agents.PresenceStatus, error) {
	return agents.PresenceStatus{}, nil
}

func (s *liveSources) Statuses(_ context.Context, list []ids.ID) ([]agents.PresenceStatus, error) {
	out := make([]agents.PresenceStatus, 0, len(list))
	for _, id := range list {
		out = append(out, agents.PresenceStatus{AgentID: id, State: agents.StateOnline})
	}
	return out, nil
}

func (s *liveSources) LiveRates(context.Context, time.Time, time.Time) ([]app.LiveRateRow, error) {
	return s.rates, nil
}

func (s *liveSources) LiveSLI(context.Context, time.Time, time.Time) ([]domain.SLICell, error) {
	return []domain.SLICell{{Metric: "sli.api_availability", Total: 10, Good: 9}}, nil
}

// fakeTicker is a Ticker the test drives: each start sends its channel on started, each stop
// signals stopped.
type fakeTicker struct {
	started chan chan time.Time
	stopped chan struct{}
}

func newFakeTicker() *fakeTicker {
	return &fakeTicker{started: make(chan chan time.Time, 4), stopped: make(chan struct{}, 4)}
}

func (f *fakeTicker) ticker(time.Duration) (<-chan time.Time, func()) {
	c := make(chan time.Time)
	f.started <- c
	return c, func() { f.stopped <- struct{}{} }
}

// sinkRecorder records the rounds the builder hands over.
type sinkRecorder struct{ rounds chan stats.LiveRound }

func (s *sinkRecorder) LiveBuilt(_ context.Context, r stats.LiveRound) { s.rounds <- r }

// liveEnv is a live stream over the fakes with a fake clock.
type liveEnv struct {
	live   *app.Live
	clk    *clock.Fake
	az     *liveAuthz
	src    *liveSources
	ticker *fakeTicker
	sink   *sinkRecorder
	gauge  *gauge
}

// gauge is the fake of bothy.stats.live.subscribers.
type gauge struct {
	mu sync.Mutex
	n  int64
}

func (g *gauge) Subscribers(_ context.Context, d int64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n += d
}

func (g *gauge) value() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.n
}

const liveInterval = time.Second

func newLiveEnv(t *testing.T, cfg app.LiveConfig) *liveEnv {
	t.Helper()
	az := &liveAuthz{owner: ids.New(), manager: ids.New(), support: ids.New(), supportCh: ids.New(), devCh: ids.New()}
	agent := ids.New()
	src := &liveSources{
		runs: []runs.LiveRun{{ID: ids.New(), ThreadID: ids.New(), AgentID: agent, ChannelID: az.supportCh, SpaceID: az.support,
			State: "running", QueuedAt: time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC), BillingMode: "organization",
			Cost: runs.Money{AmountMicros: 5, Currency: "USD"}}},
		place: []chat.ChannelPlace{{ChannelID: az.supportCh, SpaceID: az.support, OperatorID: agent}, {ChannelID: az.devCh, SpaceID: ids.New()}},
	}
	clk := clock.NewFake(time.Date(2026, 10, 10, 9, 30, 0, 0, time.UTC))
	src.rates = []app.LiveRateRow{
		{Minute: domain.Minute(clk.Now()), Metric: "messages_created", ChannelID: az.supportCh, Dimension: "human", Count: 2},
		{Minute: domain.Minute(clk.Now()).Add(-time.Minute), Metric: "messages_created", ChannelID: az.devCh, Dimension: "human", Count: 5},
		{Minute: domain.Minute(clk.Now()), Metric: "catchups", Count: 1},
	}
	if cfg.Interval == 0 {
		cfg = app.LiveConfig{Enabled: true, Interval: liveInterval, MaxRuns: 200, MaxSubscribers: 3, SLIWindow: 5 * time.Minute,
			PayloadMax: 1 << 20, Connections: true}
	}
	e := &liveEnv{clk: clk, az: az, src: src, ticker: newFakeTicker(), sink: &sinkRecorder{rounds: make(chan stats.LiveRound, 16)}, gauge: &gauge{}}
	l, err := app.NewLive(cfg, app.LiveDeps{Authorizer: az, Runs: src, Places: src, Facts: src, Presence: src, Store: src, Clock: clk,
		Ticker: e.ticker.ticker, Metrics: e.gauge})
	if err != nil {
		t.Fatal(err)
	}
	l.BindSink(e.sink)
	e.live = l
	return e
}

func (e *liveEnv) person(id ids.ID) actor.Actor { return actor.Actor{ID: id, Kind: actor.KindUser} }

// wantCode fails unless err carries code.
func wantCode(t *testing.T, err error, code errs.Code) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != code {
		t.Fatalf("error %v, want %s", err, code)
	}
}

// TestLive_Check (VZ-T10, 25 5.1 items 1 and 6): off is feature_disabled before any check; the
// owner reads the instance; a manager reads the space and is forbidden on the instance; an unknown
// space is not_found; an agent is forbidden even where authorize would allow.
func TestLive_Check(t *testing.T) {
	t.Parallel()
	e := newLiveEnv(t, app.LiveConfig{})
	ctx := context.Background()
	if err := e.live.Check(ctx, e.person(e.az.owner), stats.LiveQuery{}); err != nil {
		t.Fatalf("owner on the instance: %v", err)
	}
	if err := e.live.Check(ctx, e.person(e.az.manager), stats.LiveQuery{SpaceID: e.az.support}); err != nil {
		t.Fatalf("manager on support: %v", err)
	}
	wantCode(t, e.live.Check(ctx, e.person(e.az.manager), stats.LiveQuery{}), errs.CodeForbidden)
	wantCode(t, e.live.Check(ctx, e.person(e.az.manager), stats.LiveQuery{SpaceID: ids.New()}), errs.CodeNotFound)
	wantCode(t, e.live.Check(ctx, actor.Actor{ID: e.az.owner, Kind: actor.KindAgent}, stats.LiveQuery{}), errs.CodeForbidden)

	off := newLiveEnv(t, app.LiveConfig{Enabled: false, Interval: liveInterval, MaxRuns: 1, MaxSubscribers: 1, SLIWindow: time.Minute, PayloadMax: 1024})
	wantCode(t, off.live.Check(ctx, off.person(off.az.owner), stats.LiveQuery{}), errs.CodeFeatureDisabled)
	_, err := off.live.Current(actor.WithActor(ctx, off.person(off.az.owner)), stats.LiveQuery{})
	wantCode(t, err, errs.CodeFeatureDisabled)
}

// TestLive_MaxSubscribers: above stats.live_max_subscribers Acquire is rate_limited; the gauge
// follows Acquire and Release and never goes below zero.
func TestLive_MaxSubscribers(t *testing.T) {
	t.Parallel()
	e := newLiveEnv(t, app.LiveConfig{})
	for range 3 {
		if err := e.live.Acquire(); err != nil {
			t.Fatal(err)
		}
	}
	wantCode(t, e.live.Acquire(), errs.CodeRateLimited)
	if e.live.Subscribers() != 3 || e.gauge.value() != 3 {
		t.Fatalf("subscribers %d, gauge %d", e.live.Subscribers(), e.gauge.value())
	}
	for range 4 {
		e.live.Release()
	}
	if e.live.Subscribers() != 0 || e.gauge.value() != 0 {
		t.Fatalf("after release: subscribers %d, gauge %d", e.live.Subscribers(), e.gauge.value())
	}
}

// TestLive_BuilderStartsAndStops (25 5.1 item 4, R-VZ-05): without a subscription nothing is
// built and no ticker runs; the first subscription starts one ticker, each tick builds exactly one
// base frame for all subscribers and hands it to the sink; the first tick after the last release
// stops the ticker without a build.
func TestLive_BuilderStartsAndStops(t *testing.T) {
	t.Parallel()
	e := newLiveEnv(t, app.LiveConfig{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		e.live.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		testkit.Receive(t, "the end of Run", done)
	})
	if e.live.Tick(ctx) || e.live.Builds() != 0 {
		t.Fatalf("a tick without a subscription built %d frames", e.live.Builds())
	}
	for range 3 {
		if err := e.live.Acquire(); err != nil {
			t.Fatal(err)
		}
	}
	ticks := testkit.Receive(t, "the start of the ticker", e.ticker.started)
	for i := 1; i <= 2; i++ {
		ticks <- e.clk.Now()
		r := testkit.Receive(t, "the base frame of a tick", e.sink.rounds)
		if !r.At().Equal(e.clk.Now()) {
			t.Fatalf("round at %v", r.At())
		}
		if e.live.Builds() != int64(i) {
			t.Fatalf("after %d ticks %d builds", i, e.live.Builds())
		}
		e.clk.Advance(liveInterval)
	}
	for range 3 {
		e.live.Release()
	}
	ticks <- e.clk.Now()
	testkit.Receive(t, "the stop of the ticker", e.ticker.stopped)
	if e.live.Builds() != 2 {
		t.Fatalf("the idle tick built: %d builds", e.live.Builds())
	}
}

// TestLive_RoundFiltersPerReceiver: one base frame, filtered in memory for the owner (everything,
// sli, connections) and for the manager of support (only support, no instance rows, no sli).
func TestLive_RoundFiltersPerReceiver(t *testing.T) {
	t.Parallel()
	e := newLiveEnv(t, app.LiveConfig{})
	if err := e.live.Acquire(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if !e.live.Tick(ctx) {
		t.Fatal("no build with a subscription")
	}
	r := testkit.Receive(t, "the round", e.sink.rounds)
	owner, err := r.For(ctx, e.person(e.az.owner), stats.LiveQuery{})
	if err != nil {
		t.Fatal(err)
	}
	var of map[string]any
	if err := json.Unmarshal(owner.Payload, &of); err != nil {
		t.Fatal(err)
	}
	if _, ok := of["sli"]; !ok {
		t.Fatalf("owner frame without sli: %s", owner.Payload)
	}
	if len(of["rates"].([]any)) == 0 || len(of["runs"].([]any)) != 1 || len(of["agents"].([]any)) != 1 {
		t.Fatalf("owner frame %s", owner.Payload)
	}
	mgr, err := r.For(ctx, e.person(e.az.manager), stats.LiveQuery{SpaceID: e.az.support})
	if err != nil {
		t.Fatal(err)
	}
	var mf struct {
		Rates []struct {
			Metric    string `json:"metric"`
			Dimension string `json:"dimension"`
			Last      int64  `json:"last_minute"`
			Current   int64  `json:"current_minute"`
		} `json:"rates"`
		SLI  []any `json:"sli"`
		Runs []struct {
			Cost *int64 `json:"cost_micros"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(mgr.Payload, &mf); err != nil {
		t.Fatal(err)
	}
	if mf.SLI != nil || len(mf.Runs) != 1 || mf.Runs[0].Cost == nil {
		t.Fatalf("manager frame %s", mgr.Payload)
	}
	for _, rt := range mf.Rates {
		if rt.Metric != "messages_created" || rt.Last != 0 || rt.Current != 2 {
			t.Fatalf("manager rate %+v in %s", rt, mgr.Payload)
		}
	}
}

// TestLive_CurrentSingleFlight (R-VZ-06): ten concurrent polls on an idle node build one base
// frame; further polls within stats.live_interval build nothing; a poll after the interval builds
// again; a poll keeps the node active for one interval.
func TestLive_CurrentSingleFlight(t *testing.T) {
	t.Parallel()
	e := newLiveEnv(t, app.LiveConfig{})
	ctx := actor.WithActor(context.Background(), e.person(e.az.owner))
	var wg sync.WaitGroup
	errc := make(chan error, 10)
	for range 10 {
		wg.Go(func() {
			if _, err := e.live.Current(ctx, stats.LiveQuery{}); err != nil {
				errc <- err
			}
		})
	}
	wg.Wait()
	close(errc)
	for err := range errc {
		t.Fatal(err)
	}
	if e.live.Builds() != 1 {
		t.Fatalf("%d builds for ten polls", e.live.Builds())
	}
	e.clk.Advance(liveInterval / 2)
	if _, err := e.live.Current(ctx, stats.LiveQuery{}); err != nil || e.live.Builds() != 1 {
		t.Fatalf("poll within the interval: %v, %d builds", err, e.live.Builds())
	}
	if !e.live.Tick(context.Background()) {
		t.Fatal("the node is idle right after a poll")
	}
	e.clk.Advance(liveInterval)
	if e.live.Tick(context.Background()) {
		t.Fatal("the node stays active one interval after the last poll")
	}
	builds := e.live.Builds()
	if _, err := e.live.Current(ctx, stats.LiveQuery{}); err != nil || e.live.Builds() != builds+1 {
		t.Fatalf("poll after the interval: %v, %d builds", err, e.live.Builds())
	}
	if _, err := e.live.Current(actor.WithActor(context.Background(), e.person(e.az.manager)), stats.LiveQuery{}); err == nil {
		t.Fatal("the manager polls the instance")
	}
}
