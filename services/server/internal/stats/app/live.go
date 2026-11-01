// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/stats"
	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// LiveConfig holds the configuration keys of the live stream (docs/spec/25-visualization.md 15).
type LiveConfig struct {
	// Enabled is stats.live_enabled.
	Enabled bool
	// Interval is stats.live_interval: the build period and the lifetime of a base frame for
	// getStatsLive.
	Interval time.Duration
	// MaxRuns is stats.live_max_runs, per receiver; the base frame keeps one more per channel.
	MaxRuns int
	// MaxSubscribers is stats.live_max_subscribers per node.
	MaxSubscribers int
	// SLIWindow is stats.live_sli_window.
	SLIWindow time.Duration
	// PayloadMax is sync.event_payload_max_bytes.
	PayloadMax int
	// Connections reports cluster.mode = single: only then the frame carries connections.
	Connections bool
	// MaxTasks is stats.live_max_tasks: task nodes per run; 0 sends no agent tree.
	MaxTasks int
	// TaskMaxDepth is stats.live_task_max_depth; TaskKeep is stats.live_task_keep
	// (docs/design/topology-agent-tree.md 3).
	TaskMaxDepth int
	TaskKeep     time.Duration
}

// validate names every invalid value.
func (c LiveConfig) validate() error {
	if c.Interval <= 0 || c.SLIWindow <= 0 || c.MaxRuns < 1 || c.MaxSubscribers < 1 || c.PayloadMax < 1 {
		return errors.New("stats: stats.live_interval, stats.live_sli_window, stats.live_max_runs, stats.live_max_subscribers " +
			"and sync.event_payload_max_bytes must be positive")
	}
	if c.MaxTasks < 0 || c.TaskKeep < 0 || (c.MaxTasks > 0 && c.TaskMaxDepth < 1) {
		return errors.New("stats: stats.live_max_tasks and stats.live_task_keep must not be negative, " +
			"stats.live_task_max_depth must be positive")
	}
	return nil
}

// LiveRateRow is one grouped row of stats_minutes for the rates of the live frame: the minute, the
// metric, the channel (zero for none), the dimension and the currency ("" for none), summed over
// agents.
type LiveRateRow struct {
	Minute     time.Time
	Metric     string
	ChannelID  ids.ID
	Dimension  string
	Currency   string
	Count, Sum int64
}

// LiveStore is the outgoing port of the live frame on stats_minutes (adapters/postgres).
type LiveStore interface {
	// LiveRates returns the rows of the minutes last and current without the sli.* metrics.
	LiveRates(ctx context.Context, last, current time.Time) ([]LiveRateRow, error)
	// LiveSLI returns count and good of every sli.* metric with rows in the minutes from from up
	// to and including to.
	LiveSLI(ctx context.Context, from, to time.Time) ([]domain.SLICell, error)
}

// LiveMetrics records bothy.stats.live.subscribers (signals.md 4.3b).
type LiveMetrics interface {
	// Subscribers adds delta (+1 or -1) to the subscriptions of the node.
	Subscribers(ctx context.Context, delta int64)
}

// NopLiveMetrics records nothing.
type NopLiveMetrics struct{}

// Subscribers implements LiveMetrics.
func (NopLiveMetrics) Subscribers(context.Context, int64) {}

// Ticker starts a ticker of period d and returns its channel and its stop function. Production
// uses time.NewTicker; tests pass one they drive.
type Ticker func(d time.Duration) (<-chan time.Time, func())

// SystemTicker is the Ticker over time.NewTicker.
func SystemTicker(d time.Duration) (<-chan time.Time, func()) {
	t := time.NewTicker(d)
	return t.C, t.Stop
}

// LiveDeps are the collaborators of the live stream.
type LiveDeps struct {
	// Authorizer decides stats.read and cost.read.aggregate and gives the visible channels.
	Authorizer authorize.Authorizer
	// Runs are the active runs (runs.LiveSource).
	Runs runs.LiveSource
	// Places are the spaces and operators of the channels (chat.ChannelPlaces).
	Places chat.ChannelPlaces
	// Facts give the configured billing mode of agents; Presence their presence and reported mode.
	Facts    agents.Facts
	Presence agents.Presence
	// Store reads the rates and SLI totals of stats_minutes.
	Store LiveStore
	// Clock is the build time and the age of a base frame; nil means clock.System().
	Clock clock.Clock
	// Ticker drives Run; nil means SystemTicker.
	Ticker Ticker
	// Metrics records the subscriber gauge; nil records nothing.
	Metrics LiveMetrics
	// Log receives failed builds; nil means slog.Default(). It never receives IDs.
	Log *slog.Logger
}

// Live is the live stream of one api node (M6-15, 25 5): the subscriber count with its bound, the
// builder of one base frame per stats.live_interval while a subscription exists or getStatsLive
// was polled within the last interval, the last base frame for getStatsLive and the filter per
// receiver. It implements stats.Live and is safe for concurrent use.
type Live struct {
	cfg LiveConfig
	d   LiveDeps

	mu          sync.Mutex
	subscribers int
	pollUntil   time.Time
	sink        stats.LiveSink
	conns       stats.LiveConnectionCounter
	latest      *domain.BaseFrame
	builds      int64

	// buildMu makes every build a single flight: a caller that waited for it finds the fresh
	// frame and builds nothing.
	buildMu sync.Mutex
	wake    chan struct{}
}

// Compile-time check: Live is the incoming port of the live stream.
var _ stats.Live = (*Live)(nil)

// Log message and error type of a failed build (docs/observability/signals.md "Logs").
const (
	msgBuildFailed   = "stats live frame build failed"
	errorTypeBuild   = "build"
	detailLiveOff    = "stats.live_enabled is off"
	detailLiveFull   = "stats.live_max_subscribers"
	detailPeopleOnly = "only people subscribe to stats.live"
)

// NewLive returns the live stream with cfg over d. Errors: an invalid value of cfg or a missing
// collaborator.
func NewLive(cfg LiveConfig, d LiveDeps) (*Live, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if d.Authorizer == nil || d.Runs == nil || d.Places == nil || d.Facts == nil || d.Presence == nil || d.Store == nil {
		return nil, errors.New("stats: the live stream needs the Authorizer, runs, chat, agents, the presence and the store")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Ticker == nil {
		d.Ticker = SystemTicker
	}
	if d.Metrics == nil {
		d.Metrics = NopLiveMetrics{}
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Live{cfg: cfg, d: d, wake: make(chan struct{}, 1)}, nil
}

// BindSink implements stats.Live.
func (l *Live) BindSink(s stats.LiveSink) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sink = s
}

// BindConnections makes c the source of the member connections (the gateway hub of the node);
// without it, and with cluster.mode = multi, the frame has no connections.
func (l *Live) BindConnections(c stats.LiveConnectionCounter) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.conns = c
}

// resource returns the target of q: the instance or the space.
func resource(q stats.LiveQuery) authorize.Resource {
	if q.Instance() {
		return authorize.Resource{Kind: authorize.ResourceInstance}
	}
	return authorize.Resource{Kind: authorize.ResourceSpace, ID: q.SpaceID}
}

// Check implements stats.Live (M6-15 design note 1).
func (l *Live) Check(ctx context.Context, a actor.Actor, q stats.LiveQuery) error {
	if !l.cfg.Enabled {
		return errs.New(errs.CodeFeatureDisabled, detailLiveOff)
	}
	if a.Kind != actor.KindUser {
		// 04 12 item 4: agents and apps never subscribe, whatever grant they hold and whether the
		// space exists, so they get forbidden before any decision.
		return errs.New(errs.CodeForbidden, detailPeopleOnly)
	}
	d, err := l.d.Authorizer.Authorize(ctx, a, authorize.ActionStatsRead, resource(q))
	if err != nil {
		return err
	}
	return d.Err()
}

// Acquire implements stats.Live.
func (l *Live) Acquire() error {
	l.mu.Lock()
	if l.subscribers >= l.cfg.MaxSubscribers {
		l.mu.Unlock()
		return errs.New(errs.CodeRateLimited, detailLiveFull)
	}
	l.subscribers++
	l.mu.Unlock()
	l.d.Metrics.Subscribers(context.Background(), 1)
	l.signal()
	return nil
}

// Release implements stats.Live.
func (l *Live) Release() {
	l.mu.Lock()
	if l.subscribers == 0 {
		l.mu.Unlock()
		return
	}
	l.subscribers--
	l.mu.Unlock()
	l.d.Metrics.Subscribers(context.Background(), -1)
}

// Subscribers returns the subscriptions of the node.
func (l *Live) Subscribers() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.subscribers
}

// Builds returns how many base frames the node built so far.
func (l *Live) Builds() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.builds
}

// signal wakes Run.
func (l *Live) signal() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// active reports whether the node must build at now: a subscription exists or getStatsLive was
// polled within the last stats.live_interval (M6-15 design note 6).
func (l *Live) active(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.subscribers > 0 || now.Before(l.pollUntil)
}

// Run is the builder loop of the node (M6-15 design note 2): it sleeps until a subscription or a
// poll wakes it, then starts a ticker of stats.live_interval and builds one base frame per tick
// (Tick) until a tick finds the node idle, then stops the ticker and sleeps again. It returns when
// ctx ends. internal/wire runs it in the roles api and all.
func (l *Live) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-l.wake:
		}
		if !l.active(l.d.Clock.Now()) {
			continue
		}
		if !l.ticking(ctx) {
			return
		}
	}
}

// ticking runs one ticker until the node is idle at a tick (true) or ctx ends (false).
func (l *Live) ticking(ctx context.Context) bool {
	ticks, stop := l.d.Ticker(l.cfg.Interval)
	defer stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticks:
			if !l.Tick(ctx) {
				return true
			}
		}
	}
}

// Tick builds one base frame when the node is active and hands it to the sink; false when the
// node is idle (nothing built). A failed build is logged and counts as built: the next tick tries
// again. Tests drive the builder through Tick with a fake clock.
func (l *Live) Tick(ctx context.Context) bool {
	if !l.active(l.d.Clock.Now()) {
		return false
	}
	l.buildMu.Lock()
	base, err := l.build(ctx)
	l.buildMu.Unlock()
	if err != nil {
		l.d.Log.LogAttrs(ctx, slog.LevelWarn, msgBuildFailed, slog.String(logKeyErrorType, errorTypeBuild))
		return true
	}
	l.mu.Lock()
	sink := l.sink
	l.mu.Unlock()
	if sink != nil {
		sink.LiveBuilt(ctx, l.round(base))
	}
	return true
}

// Current implements stats.Live (M6-15 design note 6).
func (l *Live) Current(ctx context.Context, q stats.LiveQuery) (stats.LiveFrame, error) {
	a, _ := actor.FromContext(ctx)
	if err := l.Check(ctx, a, q); err != nil {
		return stats.LiveFrame{}, err
	}
	now := l.d.Clock.Now()
	l.mu.Lock()
	l.pollUntil = now.Add(l.cfg.Interval)
	l.mu.Unlock()
	l.signal()
	base, err := l.fresh(ctx, now)
	if err != nil {
		return stats.LiveFrame{}, err
	}
	return l.round(base).For(ctx, a, q)
}

// fresh returns the newest base frame when it was built within the last stats.live_interval
// before now; otherwise it builds one, in a single flight with every other caller and the ticker.
func (l *Live) fresh(ctx context.Context, now time.Time) (*domain.BaseFrame, error) {
	if b := l.young(now); b != nil {
		return b, nil
	}
	l.buildMu.Lock()
	defer l.buildMu.Unlock()
	if b := l.young(now); b != nil {
		return b, nil
	}
	return l.build(ctx)
}

// young returns the newest base frame when it is younger than stats.live_interval at now.
func (l *Live) young(now time.Time) *domain.BaseFrame {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.latest != nil && now.Sub(l.latest.At) < l.cfg.Interval {
		return l.latest
	}
	return nil
}

// build reads the sources and builds the base frame of now (M6-15 design notes 2 and 3);
// buildMu is held. It never filters.
func (l *Live) build(ctx context.Context) (*domain.BaseFrame, error) {
	at := l.d.Clock.Now()
	// One run more than stats.live_max_runs per channel: a receiver who sees only that channel
	// must still learn that more runs are active than the frame holds (runs_truncated).
	active, err := l.d.Runs.LiveRuns(ctx, l.cfg.MaxRuns+1)
	if err != nil {
		return nil, err
	}
	places, err := l.d.Places.ChannelPlaces(ctx)
	if err != nil {
		return nil, err
	}
	current := domain.Minute(at)
	rows, err := l.d.Store.LiveRates(ctx, current.Add(-time.Minute), current)
	if err != nil {
		return nil, err
	}
	sli, err := l.d.Store.LiveSLI(ctx, domain.Minute(at.Add(-l.cfg.SLIWindow)), current)
	if err != nil {
		return nil, err
	}
	base := &domain.BaseFrame{At: at, Channels: make(map[ids.ID]domain.ChannelPlace, len(places)),
		ChannelRates: map[ids.ID][]domain.RateCell{}, SLI: sliCells(sli)}
	want := map[ids.ID]struct{}{}
	for _, p := range places {
		base.Channels[p.ChannelID] = domain.ChannelPlace{SpaceID: p.SpaceID, OperatorID: p.OperatorID}
		if !p.OperatorID.IsZero() {
			want[p.OperatorID] = struct{}{}
		}
	}
	for _, r := range active {
		if r.SpaceID.IsZero() {
			// A run outside every space (a dm channel) has no place in the frame (StatsLiveRun
			// requires space_id) and no space manager may see it.
			continue
		}
		base.Runs = append(base.Runs, l.liveRun(r, at))
		want[r.AgentID] = struct{}{}
	}
	base.ChannelRates, base.InstanceRates = rateCells(rows, current)
	if base.Agents, err = l.agentStates(ctx, want); err != nil {
		return nil, err
	}
	l.mu.Lock()
	if l.cfg.Connections && l.conns != nil {
		c := l.conns.LiveConnections()
		base.Connections = &domain.Connections{User: c.User, Agent: c.Agent, App: c.App}
	}
	l.latest = base
	l.builds++
	l.mu.Unlock()
	return base, nil
}

// liveRun converts a run of the source with its agent tree at the build time at.
func (l *Live) liveRun(r runs.LiveRun, at time.Time) domain.LiveRun {
	out := domain.LiveRun{RunID: r.ID, ThreadID: r.ThreadID, ChannelID: r.ChannelID, SpaceID: r.SpaceID, AgentID: r.AgentID, State: r.State,
		QueuedAt: r.QueuedAt, StartedAt: r.StartedAt, BillingMode: r.BillingMode, StepKind: r.StepKind, StepName: r.StepName,
		ApprovalPending: r.ApprovalPending, CostMicros: r.Cost.AmountMicros, Currency: r.Cost.Currency, CostIsEstimate: r.Cost.IsEstimate}
	if len(r.Steps) == 0 {
		return out
	}
	steps := make([]domain.LiveStep, len(r.Steps))
	for i, s := range r.Steps {
		steps[i] = domain.LiveStep{ID: s.ID, ParentID: s.ParentID, Kind: s.Kind, Name: s.Name, Title: s.Title, Status: s.Status,
			LastTool: s.LastTool, StartedAt: s.StartedAt, FinishedAt: s.FinishedAt}
	}
	out.Tasks, out.TasksTruncated = domain.TaskTree(steps, at,
		domain.TaskLimits{Max: l.cfg.MaxTasks, MaxDepth: l.cfg.TaskMaxDepth, Keep: l.cfg.TaskKeep})
	return out
}

// rateCells splits rows into the cells per channel and the cells without a channel, with the
// counts of the minute current as Current and of the minute before as Last.
func rateCells(rows []LiveRateRow, current time.Time) (map[ids.ID][]domain.RateCell, []domain.RateCell) {
	type key struct {
		channel                     ids.ID
		metric, dimension, currency string
	}
	cells := map[key]*domain.RateCell{}
	var order []key
	for _, r := range rows {
		k := key{r.ChannelID, r.Metric, r.Dimension, r.Currency}
		c := cells[k]
		if c == nil {
			c = &domain.RateCell{Metric: r.Metric, Dimension: r.Dimension, Currency: r.Currency}
			cells[k] = c
			order = append(order, k)
		}
		if r.Minute.Equal(current) {
			c.Current += r.Count
		} else {
			c.Last += r.Count
			c.LastSum += r.Sum
		}
	}
	perChannel := map[ids.ID][]domain.RateCell{}
	var instance []domain.RateCell
	for _, k := range order {
		if k.channel.IsZero() {
			instance = append(instance, *cells[k])
			continue
		}
		perChannel[k.channel] = append(perChannel[k.channel], *cells[k])
	}
	return perChannel, instance
}

// sliCells returns one cell per sli.* metric of 25 3.2, zero where stats_minutes has no row.
func sliCells(rows []domain.SLICell) []domain.SLICell {
	out := make([]domain.SLICell, 0, len(rows))
	for _, m := range measure.Metrics() {
		if !m.IsSLI() {
			continue
		}
		c := domain.SLICell{Metric: string(m)}
		if i := slices.IndexFunc(rows, func(r domain.SLICell) bool { return r.Metric == string(m) }); i >= 0 {
			c = rows[i]
		}
		out = append(out, c)
	}
	return out
}

// agentStates returns the state of every agent of want: the configured mode from the facts, the
// presence and the reported mode from the tracker (unknown without one). People, apps and
// unknown ids drop out.
func (l *Live) agentStates(ctx context.Context, want map[ids.ID]struct{}) (map[ids.ID]domain.LiveAgent, error) {
	out := make(map[ids.ID]domain.LiveAgent, len(want))
	if len(want) == 0 {
		return out, nil
	}
	list := make([]ids.ID, 0, len(want))
	for id := range want {
		list = append(list, id)
	}
	facts, err := l.d.Facts.RunFacts(ctx, list)
	if err != nil {
		return nil, err
	}
	statuses, err := l.d.Presence.Statuses(ctx, list)
	if err != nil {
		return nil, err
	}
	for id, f := range facts {
		out[id] = domain.LiveAgent{AgentID: id, Presence: agents.StateUnknown, BillingMode: f.BillingMode}
	}
	for _, s := range statuses {
		a, ok := out[s.AgentID]
		if !ok {
			continue
		}
		a.Presence, a.ReportedBillingMode = s.State, s.ReportedBillingMode
		out[s.AgentID] = a
	}
	return out, nil
}

// round wraps base as the stats.LiveRound of the sink and of getStatsLive.
func (l *Live) round(base *domain.BaseFrame) stats.LiveRound { return liveRound{l: l, base: base} }

// liveRound is one built base frame with the filter of its node.
type liveRound struct {
	l    *Live
	base *domain.BaseFrame
}

// At implements stats.LiveRound.
func (r liveRound) At() time.Time { return r.base.At }

// For implements stats.LiveRound (M6-15 design note 4): Scope of channel.read for the visible
// channels, Scope of run.read for the channels whose task labels the receiver gets (only when a
// run of the frame has a labelled task node, docs/design/topology-agent-tree.md 5) and
// cost.read.aggregate on the target, all in the mode of ctx, then the filter and the size cut of
// the domain.
func (r liveRound) For(ctx context.Context, a actor.Actor, q stats.LiveQuery) (stats.LiveFrame, error) {
	sc, err := r.l.d.Authorizer.Scope(ctx, a, authorize.ActionChannelRead)
	if err != nil {
		return stats.LiveFrame{}, err
	}
	d, err := r.l.d.Authorizer.Authorize(ctx, a, authorize.ActionCostReadAggregate, resource(q))
	if err != nil {
		return stats.LiveFrame{}, err
	}
	visible := channelSet(sc.ChannelIDs)
	var readable map[ids.ID]struct{}
	if r.labelled() {
		rs, err := r.l.d.Authorizer.Scope(ctx, a, authorize.ActionRunRead)
		if err != nil {
			return stats.LiveFrame{}, err
		}
		readable = channelSet(rs.ChannelIDs)
	}
	b, _, err := domain.FilterLive(r.base, domain.Receiver{SpaceID: q.SpaceID, Visible: visible, Readable: readable, Costs: d.Allowed()},
		domain.LiveLimits{MaxRuns: r.l.cfg.MaxRuns, PayloadMax: r.l.cfg.PayloadMax, Interval: r.l.cfg.Interval})
	if err != nil {
		return stats.LiveFrame{}, errs.Wrap(errs.CodeInternalError, err, "stats: the live frame cannot be filtered")
	}
	return stats.LiveFrame{At: r.base.At, Payload: b}, nil
}

// labelled reports whether a run of the round has a task node with a label.
func (r liveRound) labelled() bool {
	for _, run := range r.base.Runs {
		for _, x := range run.Tasks {
			if x.Label != "" {
				return true
			}
		}
	}
	return false
}

// channelSet returns the set of list.
func channelSet(list []ids.ID) map[ids.ID]struct{} {
	out := make(map[ids.ID]struct{}, len(list))
	for _, id := range list {
		out[id] = struct{}{}
	}
	return out
}
