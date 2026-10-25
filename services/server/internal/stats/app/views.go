// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/stats"
)

// ViewsConfig are the configuration keys of the three query operations of the visualizations
// (docs/spec/25-visualization.md 15, M6-14).
type ViewsConfig struct {
	// TimelineMaxRange is stats.timeline_max_range: the longest range of getStatsTimeline.
	TimelineMaxRange time.Duration `config:"stats.timeline_max_range"`
	// TimelineMaxRuns is stats.timeline_max_runs: the most runs in one timeline.
	TimelineMaxRuns int `config:"stats.timeline_max_runs"`
	// SLOWarnBudgetRatio is stats.slo_warn_budget_ratio: below this remaining budget the traffic
	// light is amber.
	SLOWarnBudgetRatio float64 `config:"stats.slo_warn_budget_ratio"`
	// SLOMinEvents is stats.slo_min_events: below this many events in the period it is
	// insufficient_data.
	SLOMinEvents int64 `config:"stats.slo_min_events"`
	// Retention is retention.stats_minutes: no range may start before now minus it.
	Retention time.Duration `config:"retention.stats_minutes"`
	// ShowPersonalEstimates is billing.show_personal_estimates: costs of mode personal appear in
	// the backend view only when it is on (10 section 12.2, as in getCostStats).
	ShowPersonalEstimates bool `config:"billing.show_personal_estimates"`
}

// validate names every invalid value.
func (c ViewsConfig) validate() error {
	if c.TimelineMaxRange <= 0 || c.TimelineMaxRuns < 1 || c.SLOMinEvents < 1 || c.Retention <= 0 ||
		c.SLOWarnBudgetRatio < 0 || c.SLOWarnBudgetRatio > 1 {
		return errors.New("stats: stats.timeline_max_range, stats.timeline_max_runs, stats.slo_min_events and retention.stats_minutes " +
			"must be positive and stats.slo_warn_budget_ratio between 0 and 1")
	}
	return nil
}

// TimelineRowsQuery selects the rows of stats_minutes of a timeline: bucket_start in [From, To),
// the metric in Metrics, the channel in ChannelIDs or, with WithInstance, no channel.
type TimelineRowsQuery struct {
	From, To     time.Time
	Metrics      []string
	ChannelIDs   []ids.ID
	WithInstance bool
}

// BackendRowsQuery selects the sums of the backend view: bucket_start in [From, To), the agent in
// AgentIDs; billing_mode_switches of these agents, invocations_denied in ChannelIDs and, with
// WithCost, cost in ChannelIDs.
type BackendRowsQuery struct {
	From, To   time.Time
	AgentIDs   []ids.ID
	ChannelIDs []ids.ID
	WithCost   bool
}

// BackendRow is one sum of the backend view per agent, metric, dimension and currency ("" for
// none).
type BackendRow struct {
	AgentID                     ids.ID
	Metric, Dimension, Currency string
	Count, Sum                  int64
}

// ViewsStore is the outgoing port of the views on stats_minutes (adapters/postgres).
type ViewsStore interface {
	// TimelineRows returns the rows of q ordered by start, metric, channel, agent, dimension and
	// currency.
	TimelineRows(ctx context.Context, q TimelineRowsQuery) ([]stats.TimelineBucket, error)
	// BackendRows returns the sums of q ordered by agent, metric, dimension and currency.
	BackendRows(ctx context.Context, q BackendRowsQuery) ([]BackendRow, error)
}

// ViewsDeps are the collaborators of Views.
type ViewsDeps struct {
	// Statistics give the authorization, the channel set and the cache of M6-05 (design note 1).
	Statistics *Statistics
	// Store reads stats_minutes; SLI sums the sli.* metrics over a period.
	Store ViewsStore
	SLI   stats.SLIWindows
	// Runs are the run intervals and the agents with a run (runs.TimelineSource).
	Runs runs.TimelineSource
	// Agents are the modes and the stored rate-limit state of the agents (agents.BackendSource).
	Agents agents.BackendSource
	// Places are the operators of the channels (chat.ChannelPlaces).
	Places chat.ChannelPlaces
	// Clock is now for the retention bound and the SLO period; nil means clock.System().
	Clock clock.Clock
}

// Views implements stats.Views (M6-14): getStatsTimeline, getStatsSlo and getStatsBackends over
// the own measurement. It is safe for concurrent use.
type Views struct {
	cfg ViewsConfig
	d   ViewsDeps
}

// Compile-time check: Views is the port of the three view queries.
var _ stats.Views = (*Views)(nil)

// NewViews returns the view queries with cfg over d. Errors: an invalid value or a missing
// collaborator.
func NewViews(cfg ViewsConfig, d ViewsDeps) (*Views, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if d.Statistics == nil || d.Store == nil || d.SLI == nil || d.Runs == nil || d.Agents == nil || d.Places == nil {
		return nil, errors.New("stats: the views need the statistics, the store, the SLI sums, runs, agents and the channel places")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &Views{cfg: cfg, d: d}, nil
}

// costsAllowed decides cost.read.aggregate for a on the instance (zero space) or the space in
// filter mode: a denial only hides the costs (25 section 4 rule 2). Errors: those of the
// Authorizer.
func (v *Views) costsAllowed(ctx context.Context, a actor.Actor, space ids.ID) (bool, error) {
	res := authorize.Resource{Kind: authorize.ResourceInstance}
	if !space.IsZero() {
		res = authorize.Resource{Kind: authorize.ResourceSpace, ID: space}
	}
	d, err := v.d.Statistics.d.Authorizer.Authorize(ctx, a, authorize.ActionCostReadAggregate, res)
	if err != nil {
		return false, err
	}
	return d.Allowed(), nil
}

// threadsOf resolves channels to their threads created before to, with the channel of each
// thread, because runs has no channel_id (D-M2-05-9).
func (v *Views) threadsOf(ctx context.Context, channels []ids.ID, to time.Time) ([]ids.ID, map[ids.ID]ids.ID, error) {
	ts, err := v.d.Statistics.d.Chat.StatsThreads(ctx, channels, to)
	if err != nil {
		return nil, nil, err
	}
	in := make(map[ids.ID]bool, len(channels))
	for _, c := range channels {
		in[c] = true
	}
	threads, channelOf := make([]ids.ID, 0, len(ts)), make(map[ids.ID]ids.ID, len(ts))
	for _, th := range ts {
		// The structure source never widens the set; the check keeps that true by construction.
		if !in[th.ChannelID] {
			continue
		}
		threads = append(threads, th.ThreadID)
		channelOf[th.ThreadID] = th.ChannelID
	}
	return threads, channelOf, nil
}
