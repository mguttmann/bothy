// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/stats"
	"github.com/OWNER/bothy/services/server/internal/stats/app"
)

// viewsChat is the chat source of the views tests: every channel lies in one space, and each
// channel has the threads of its map.
type viewsChat struct {
	*fakeSources
	threads map[ids.ID][]ids.ID
}

func (c viewsChat) StatsThreads(_ context.Context, chans []ids.ID, _ time.Time) ([]chat.StatsThread, error) {
	var out []chat.StatsThread
	for _, ch := range chans {
		for _, th := range c.threads[ch] {
			out = append(out, chat.StatsThread{ThreadID: th, ChannelID: ch})
		}
	}
	return out, nil
}

// StatsRunChats names every thread of the map "chat of <channel>" in channel "project of <channel>",
// within channelIDs only.
func (c viewsChat) StatsRunChats(_ context.Context, channelIDs, threadIDs []ids.ID) ([]chat.StatsRunChat, error) {
	var out []chat.StatsRunChat
	for _, ch := range channelIDs {
		for _, th := range c.threads[ch] {
			if slices.Contains(threadIDs, th) {
				out = append(out, chat.StatsRunChat{ThreadID: th, ChannelID: ch, ChannelName: "project of " + ch.String(),
					ThreadTitle: "chat of " + ch.String()})
			}
		}
	}
	return out, nil
}

// viewsFakes are the outgoing ports of the views: the rows and sums of stats_minutes, the SLI
// sums, the runs, the agents and the channel places, with the queries they received.
type viewsFakes struct {
	rows        []stats.TimelineBucket
	backend     []app.BackendRow
	sli         map[string][]stats.SLITotal
	sliCalls    int
	runs        []runs.TimelineRun
	runAgents   []ids.ID
	states      []agents.BackendState
	places      []chat.ChannelPlace
	rowsQuery   app.TimelineRowsQuery
	backendQ    app.BackendRowsQuery
	runsQuery   runs.TimelineQuery
	statesAsked []ids.ID
}

func (f *viewsFakes) TimelineRows(_ context.Context, q app.TimelineRowsQuery) ([]stats.TimelineBucket, error) {
	f.rowsQuery = q
	return f.rows, nil
}

func (f *viewsFakes) BackendRows(_ context.Context, q app.BackendRowsQuery) ([]app.BackendRow, error) {
	f.backendQ = q
	return f.backend, nil
}

func (f *viewsFakes) SLITotals(_ context.Context, metric string, _, _ time.Time) ([]stats.SLITotal, error) {
	f.sliCalls++
	return f.sli[metric], nil
}

func (f *viewsFakes) TimelineRuns(_ context.Context, q runs.TimelineQuery) ([]runs.TimelineRun, error) {
	f.runsQuery = q
	return f.runs, nil
}

func (f *viewsFakes) TimelineAgents(_ context.Context, q runs.TimelineQuery) ([]ids.ID, error) {
	f.runsQuery = q
	return f.runAgents, nil
}

func (f *viewsFakes) BackendStates(_ context.Context, list []ids.ID) ([]agents.BackendState, error) {
	f.statesAsked = list
	return f.states, nil
}

func (f *viewsFakes) ChannelPlaces(context.Context) ([]chat.ChannelPlace, error) {
	return f.places, nil
}

// viewsWorld is the use case over the fakes: the caller sees the channels visible (Scope of
// message.read), each with one thread.
type viewsWorld struct {
	views   *app.Views
	fakes   *viewsFakes
	az      *statsAuthorizer
	clk     *clock.Fake
	space   ids.ID
	visible []ids.ID
	thread  map[ids.ID]ids.ID
}

// viewsConfig are the defaults of the keys of 25 15 and retention.stats_minutes.
var viewsConfig = app.ViewsConfig{TimelineMaxRange: 24 * time.Hour, TimelineMaxRuns: 2, SLOWarnBudgetRatio: 0.25, SLOMinEvents: 100,
	Retention: 90 * 24 * time.Hour}

// viewsNow is the clock of the views tests; viewsFrom the start of their ranges.
var (
	viewsNow  = time.Date(2026, 10, 10, 12, 0, 30, 0, time.UTC)
	viewsFrom = time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
)

func newViewsWorld(t *testing.T, cfg app.ViewsConfig, channels int) *viewsWorld {
	t.Helper()
	w := &viewsWorld{fakes: &viewsFakes{}, clk: clock.NewFake(viewsNow), space: ids.New(), thread: map[ids.ID]ids.ID{}}
	threads := map[ids.ID][]ids.ID{}
	for range channels {
		c := ids.New()
		w.visible = append(w.visible, c)
		w.thread[c] = ids.New()
		threads[c] = []ids.ID{w.thread[c]}
	}
	w.az = &statsAuthorizer{outcomes: map[authorize.Action]authorize.Outcome{}, scope: authorize.Scope{ChannelIDs: w.visible}}
	src := &fakeSources{space: w.space}
	st, err := app.NewStatistics(statsConfig, app.Deps{Authorizer: w.az, Chat: viewsChat{fakeSources: src, threads: threads}, Messaging: src,
		Runs: src, Billing: src, Files: src, Apps: src, Notify: pushSources{src}, Clock: w.clk})
	if err != nil {
		t.Fatal(err)
	}
	w.views, err = app.NewViews(cfg, app.ViewsDeps{Statistics: st, Store: w.fakes, SLI: w.fakes, Runs: w.fakes, Agents: w.fakes,
		Places: w.fakes, Clock: w.clk})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

// code returns the public code of err ("" for nil).
func code(err error) errs.Code {
	if err == nil {
		return ""
	}
	if e, ok := errs.As(err); ok {
		return e.PublicCode()
	}
	return errs.Code(err.Error())
}

// TestTimeline_WithoutCostRead (VZ-T05, 25 4 rules 1 and 2): a space manager without
// cost.read.aggregate gets no cost metric in the query and no cost row, no row without a channel
// and no row or run of a channel outside the set, even when the store returned them; runs over
// stats.timeline_max_runs give runs_truncated.
func TestTimeline_WithoutCostRead(t *testing.T) {
	t.Parallel()
	w := newViewsWorld(t, viewsConfig, 2)
	w.az.outcomes[authorize.ActionCostReadAggregate] = authorize.OutcomeForbidden
	a, b, other := w.visible[0], w.visible[1], ids.New()
	agent := ids.New()
	w.fakes.rows = []stats.TimelineBucket{
		{Start: viewsFrom, Metric: "runs_finished", ChannelID: a, AgentID: agent, Dimension: "completed", Count: 2, Sum: 84000},
		{Start: viewsFrom, Metric: "cost", ChannelID: a, AgentID: agent, Dimension: "organization", Currency: "USD", Count: 1, Sum: 5},
		{Start: viewsFrom, Metric: "sli.api_availability", Count: 10, Good: 10},
		{Start: viewsFrom, Metric: "messages_created", ChannelID: other, Dimension: "human", Count: 1},
	}
	foreign := ids.New()
	w.fakes.runs = []runs.TimelineRun{
		{ID: ids.New(), ThreadID: w.thread[a], AgentID: agent, State: "completed", QueuedAt: viewsFrom},
		{ID: ids.New(), ThreadID: foreign, AgentID: agent, State: "completed", QueuedAt: viewsFrom},
		{ID: ids.New(), ThreadID: w.thread[b], AgentID: agent, State: "running", QueuedAt: viewsFrom.Add(time.Minute)},
		{ID: ids.New(), ThreadID: w.thread[a], AgentID: agent, State: "queued", QueuedAt: viewsFrom.Add(2 * time.Minute)},
	}
	tl, err := w.views.Timeline(statsCtx(), stats.TimelineQuery{From: viewsFrom, To: viewsFrom.Add(time.Hour), SpaceID: w.space})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(w.fakes.rowsQuery.Metrics, "cost") || w.fakes.rowsQuery.WithInstance || len(w.fakes.rowsQuery.Metrics) != 17 {
		t.Fatalf("store query %+v", w.fakes.rowsQuery)
	}
	if len(tl.Buckets) != 1 || tl.Buckets[0].Metric != "runs_finished" || tl.Buckets[0].ChannelID != a {
		t.Fatalf("buckets %+v", tl.Buckets)
	}
	if w.fakes.runsQuery.Limit != viewsConfig.TimelineMaxRuns+1 || len(w.fakes.runsQuery.ThreadIDs) != 2 {
		t.Fatalf("runs query %+v", w.fakes.runsQuery)
	}
	if len(tl.Runs) != 2 || !tl.RunsTruncated || tl.Runs[0].ChannelID != a || tl.Runs[1].ChannelID != b {
		t.Fatalf("runs %+v truncated %v", tl.Runs, tl.RunsTruncated)
	}
	// With the cost right the cost rows come back; the instance rows still need the instance.
	delete(w.az.outcomes, authorize.ActionCostReadAggregate)
	tl, err = w.views.Timeline(statsCtx(), stats.TimelineQuery{From: viewsFrom, To: viewsFrom.Add(time.Hour), SpaceID: w.space,
		Metrics: []string{"cost", "cost", "sli.api_availability"}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(w.fakes.rowsQuery.Metrics, []string{"cost", "sli.api_availability"}) || len(tl.Buckets) != 1 || tl.Buckets[0].Metric != "cost" {
		t.Fatalf("query %+v buckets %+v", w.fakes.rowsQuery, tl.Buckets)
	}
}

// TestTimeline_Owner: an owner on the instance gets the rows without a channel; three runs and a
// limit of two give runs_truncated with exactly two; a limit above the runs gives no truncation.
func TestTimeline_Owner(t *testing.T) {
	t.Parallel()
	w := newViewsWorld(t, viewsConfig, 1)
	w.az.scope.InstanceWide = true
	w.fakes.rows = []stats.TimelineBucket{{Start: viewsFrom, Metric: "sli.api_availability", Count: 10, Good: 9}}
	tl, err := w.views.Timeline(statsCtx(), stats.TimelineQuery{From: viewsFrom, To: viewsFrom.Add(time.Hour)})
	if err != nil || !w.fakes.rowsQuery.WithInstance || len(tl.Buckets) != 1 || tl.RunsTruncated || tl.Runs == nil {
		t.Fatalf("%+v %v query %+v", tl, err, w.fakes.rowsQuery)
	}
}

// TestTimeline_ChatNames (owner request 2026-11-01, 25 4 rule 6): a run carries the chat of its
// thread (channel name, thread title, deep link target) only when the caller may read the thread
// (Scope of thread.read); a run of a channel outside that scope comes without any name.
func TestTimeline_ChatNames(t *testing.T) {
	t.Parallel()
	w := newViewsWorld(t, app.ViewsConfig{TimelineMaxRange: 24 * time.Hour, TimelineMaxRuns: 10, SLOWarnBudgetRatio: 0.25,
		SLOMinEvents: 100, Retention: 90 * 24 * time.Hour}, 2)
	named, hidden := w.visible[0], w.visible[1]
	w.az.threadScope = &authorize.Scope{ChannelIDs: []ids.ID{named}}
	agent := ids.New()
	w.fakes.runs = []runs.TimelineRun{
		{ID: ids.New(), ThreadID: w.thread[named], AgentID: agent, State: "completed", QueuedAt: viewsFrom},
		{ID: ids.New(), ThreadID: w.thread[hidden], AgentID: agent, State: "completed", QueuedAt: viewsFrom.Add(time.Minute)},
	}
	tl, err := w.views.Timeline(statsCtx(), stats.TimelineQuery{From: viewsFrom, To: viewsFrom.Add(time.Hour), SpaceID: w.space})
	if err != nil {
		t.Fatal(err)
	}
	if len(tl.Runs) != 2 {
		t.Fatalf("runs %+v", tl.Runs)
	}
	c := tl.Runs[0].Chat
	if c == nil || c.ThreadID != w.thread[named] || c.ChannelName != "project of "+named.String() || c.ThreadTitle != "chat of "+named.String() {
		t.Fatalf("chat of the readable thread %+v", c)
	}
	if tl.Runs[1].Chat != nil {
		t.Fatalf("chat of an unreadable thread %+v", tl.Runs[1].Chat)
	}
}

// TestTimeline_Errors (25 11): authorization comes first (a space manager without a space gets
// 403 before any range check), then the range, then the metric names.
func TestTimeline_Errors(t *testing.T) {
	t.Parallel()
	w := newViewsWorld(t, viewsConfig, 1)
	w.az.outcomes[authorize.ActionStatsRead] = authorize.OutcomeForbidden
	if _, err := w.views.Timeline(statsCtx(), stats.TimelineQuery{From: viewsFrom, To: viewsFrom}); code(err) != errs.CodeForbidden {
		t.Fatalf("denied: %v", err)
	}
	delete(w.az.outcomes, authorize.ActionStatsRead)
	for _, q := range []stats.TimelineQuery{
		{From: viewsFrom, To: viewsFrom},
		{From: viewsFrom, To: viewsFrom.Add(25 * time.Hour)},
		{From: viewsFrom.Add(time.Second), To: viewsFrom.Add(time.Hour)},
		{From: viewsNow.Add(-91 * 24 * time.Hour).Truncate(time.Minute), To: viewsNow.Add(-91*24*time.Hour + time.Hour).Truncate(time.Minute)},
		{From: viewsFrom, To: viewsFrom.Add(time.Hour), Metrics: []string{"people"}},
	} {
		if _, err := w.views.Timeline(statsCtx(), q); code(err) != errs.CodeValidationFailed {
			t.Fatalf("%+v: %v", q, err)
		}
	}
}

// TestBackends_WithoutCostRead (25 6.2, D-M6-14-3): the agents are those with a run in a visible
// channel and the operators of visible channels, not the operator of a channel outside the set;
// without cost.read.aggregate the query asks no cost and the result shows none; switches and
// denials come from the rows; the rate limit from the stored heartbeat.
func TestBackends_WithoutCostRead(t *testing.T) {
	t.Parallel()
	w := newViewsWorld(t, viewsConfig, 2)
	w.az.outcomes[authorize.ActionCostReadAggregate] = authorize.OutcomeForbidden
	runner, operator, outside := ids.New(), ids.New(), ids.New()
	w.fakes.runAgents = []ids.ID{runner}
	w.fakes.places = []chat.ChannelPlace{{ChannelID: w.visible[1], SpaceID: w.space, OperatorID: operator},
		{ChannelID: ids.New(), SpaceID: ids.New(), OperatorID: outside}, {ChannelID: w.visible[0], SpaceID: w.space}}
	resets := viewsFrom.Add(30 * time.Minute)
	w.fakes.states = []agents.BackendState{
		{AgentID: runner, BillingMode: "organization", ReportedBillingMode: "organization", BillingModesSupported: []string{"organization"},
			RateLimit: &agents.RateLimitState{State: "limited", ResetsAt: resets}},
		{AgentID: operator, BillingMode: "personal"},
	}
	w.fakes.backend = []app.BackendRow{
		{AgentID: runner, Metric: "billing_mode_switches", Dimension: "organization", Count: 2},
		{AgentID: runner, Metric: "invocations_denied", Dimension: "i2_denied", Count: 3},
		{AgentID: runner, Metric: "invocations_denied", Dimension: "billing_mode_mismatch", Count: 1},
		{AgentID: runner, Metric: "cost", Dimension: "organization", Currency: "USD", Count: 1, Sum: 700},
	}
	items, err := w.views.Backends(statsCtx(), stats.BackendQuery{From: viewsFrom, To: viewsFrom.Add(time.Hour), SpaceID: w.space})
	if err != nil {
		t.Fatal(err)
	}
	if len(w.fakes.statesAsked) != 2 || slices.Contains(w.fakes.statesAsked, outside) || w.fakes.backendQ.WithCost {
		t.Fatalf("asked %v query %+v", w.fakes.statesAsked, w.fakes.backendQ)
	}
	if len(items) != 2 {
		t.Fatalf("items %+v", items)
	}
	r := items[slices.IndexFunc(items, func(b stats.BackendAgent) bool { return b.AgentID == runner })]
	if r.Switches != 2 || r.I2Denied != 3 || r.BillingModeMismatch != 1 || r.CostsShown || r.Costs != nil || r.RateLimit == nil ||
		r.RateLimit.State != "limited" || !r.RateLimit.ResetsAt.Equal(resets) {
		t.Fatalf("runner %+v", r)
	}
	o := items[slices.IndexFunc(items, func(b stats.BackendAgent) bool { return b.AgentID == operator })]
	if o.Switches != 0 || o.RateLimit != nil || o.BillingModesSupported == nil || len(o.BillingModesSupported) != 0 {
		t.Fatalf("operator %+v", o)
	}
}

// TestBackends_Costs: with cost.read.aggregate the costs per mode and currency are estimates; the
// personal amounts only with billing.show_personal_estimates.
func TestBackends_Costs(t *testing.T) {
	t.Parallel()
	for _, show := range []bool{false, true} {
		cfg := viewsConfig
		cfg.ShowPersonalEstimates = show
		w := newViewsWorld(t, cfg, 1)
		agent := ids.New()
		w.fakes.runAgents = []ids.ID{agent}
		w.fakes.states = []agents.BackendState{{AgentID: agent, BillingMode: "organization"}}
		w.fakes.backend = []app.BackendRow{
			{AgentID: agent, Metric: "cost", Dimension: "personal", Currency: "USD", Count: 1, Sum: 400},
			{AgentID: agent, Metric: "cost", Dimension: "organization", Currency: "EUR", Count: 1, Sum: 50},
			{AgentID: agent, Metric: "cost", Dimension: "organization", Currency: "USD", Count: 2, Sum: 3000},
		}
		items, err := w.views.Backends(statsCtx(), stats.BackendQuery{From: viewsFrom, To: viewsFrom.Add(time.Hour), SpaceID: w.space})
		if err != nil || len(items) != 1 || !w.fakes.backendQ.WithCost {
			t.Fatalf("%+v %v", items, err)
		}
		want := []stats.BackendCost{{BillingMode: "organization", Currency: "EUR", AmountMicros: 50, IsEstimate: true},
			{BillingMode: "organization", Currency: "USD", AmountMicros: 3000, IsEstimate: true}}
		if show {
			want = append(want, stats.BackendCost{BillingMode: "personal", Currency: "USD", AmountMicros: 400, IsEstimate: true})
		}
		if !items[0].CostsShown || !slices.Equal(items[0].Costs, want) {
			t.Fatalf("show %v: costs %+v, want %+v", show, items[0].Costs, want)
		}
	}
}

// TestBackends_Errors: authorization first, then the range (empty, before retention.stats_minutes).
func TestBackends_Errors(t *testing.T) {
	t.Parallel()
	w := newViewsWorld(t, viewsConfig, 1)
	w.az.outcomes[authorize.ActionStatsRead] = authorize.OutcomeForbidden
	if _, err := w.views.Backends(statsCtx(), stats.BackendQuery{From: viewsFrom, To: viewsFrom}); code(err) != errs.CodeForbidden {
		t.Fatalf("denied: %v", err)
	}
	delete(w.az.outcomes, authorize.ActionStatsRead)
	for _, q := range []stats.BackendQuery{{From: viewsFrom, To: viewsFrom}, {From: viewsNow.Add(-91 * 24 * time.Hour), To: viewsFrom}} {
		if _, err := w.views.Backends(statsCtx(), q); code(err) != errs.CodeValidationFailed {
			t.Fatalf("%+v: %v", q, err)
		}
	}
}

// TestSLO (VZ-T09 through the use case): one entry per SLO with its bound, the sums over the
// dimensions, the states, owner only, and the cache for stats.cache_ttl.
func TestSLO(t *testing.T) {
	t.Parallel()
	w := newViewsWorld(t, viewsConfig, 0)
	w.fakes.sli = map[string][]stats.SLITotal{
		"sli.push_acceptance": {{Dimension: "apns", Count: 600, Good: 599}, {Dimension: "fcm", Count: 400, Good: 400}},
		"sli.run_start":       {{Count: 10000, Good: 9900}},
		"sli.reconnect":       {{Count: 50, Good: 50}},
	}
	r, err := w.views.SLO(statsCtx())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Items) != 6 || !r.At.Equal(viewsNow) {
		t.Fatalf("report %+v", r)
	}
	states := map[string]string{}
	for i, e := range r.Items {
		if i > 0 && r.Items[i-1].SLO >= e.SLO {
			t.Fatalf("not ordered by SLO: %+v", r.Items)
		}
		states[e.Metric] = e.State
		if e.PeriodDays != 30 || (e.Total == 0) != (e.Ratio == nil) {
			t.Fatalf("entry %+v", e)
		}
	}
	want := map[string]string{"sli.push_acceptance": "green", "sli.run_start": "red", "sli.reconnect": "insufficient_data",
		"sli.api_availability": "insufficient_data", "sli.delivery_latency": "insufficient_data", "sli.turn_duration": "insufficient_data"}
	for m, s := range want {
		if states[m] != s {
			t.Fatalf("%s = %s, want %s (%v)", m, states[m], s, states)
		}
	}
	for _, e := range r.Items {
		if e.Metric == "sli.push_acceptance" && (e.Total != 1000 || e.Good != 999 || e.Threshold != 0) {
			t.Fatalf("push %+v", e)
		}
		if e.Metric == "sli.run_start" && e.Threshold <= 0 {
			t.Fatalf("run start without threshold %+v", e)
		}
	}
	calls := w.fakes.sliCalls
	if _, err := w.views.SLO(statsCtx()); err != nil || w.fakes.sliCalls != calls {
		t.Fatalf("second call read again: %d calls, %v", w.fakes.sliCalls, err)
	}
	w.clk.Advance(statsConfig.CacheTTL + time.Second)
	if _, err := w.views.SLO(statsCtx()); err != nil || w.fakes.sliCalls != 2*calls {
		t.Fatalf("after stats.cache_ttl: %d calls, %v", w.fakes.sliCalls, err)
	}
	w.az.outcomes[authorize.ActionStatsRead] = authorize.OutcomeForbidden
	if _, err := w.views.SLO(statsCtx()); code(err) != errs.CodeForbidden {
		t.Fatalf("space manager: %v", err)
	}
}
