// SPDX-License-Identifier: Apache-2.0

package stats

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// MeasuredByOwn is StatsSloReport.measured_by: the values come from the own measurement in
// stats_minutes, never from Prometheus (docs/spec/25-visualization.md 6.4).
const MeasuredByOwn = "own"

// TimelineQuery is the input of getStatsTimeline (M6-14, 25 6.5).
type TimelineQuery struct {
	// From and To are the half-open range on full minutes.
	From, To time.Time
	// SpaceID restricts to one space; zero means the instance (owners only).
	SpaceID ids.ID
	// Metrics are the requested metrics of 25 3.2; nil means every metric the caller may see.
	Metrics []string
}

// TimelineBucket is one row of stats_minutes (schema StatsTimelineBucket). ChannelID and AgentID
// are zero for a row without them, Currency "" for a row without one.
type TimelineBucket struct {
	Start              time.Time
	Metric             string
	ChannelID, AgentID ids.ID
	Dimension          string
	Currency           string
	Count, Sum, Good   int64
}

// TimelineRun is the interval of one run (schema StatsTimelineRun): never the triggering person,
// the title or any content (25 1 rules 2 and 3).
type TimelineRun struct {
	RunID, ChannelID, AgentID ids.ID
	// QueuedAt is never zero; StartedAt and FinishedAt are zero until the run starts or ends.
	QueuedAt, StartedAt, FinishedAt time.Time
	State                           string
}

// Timeline is the result of getStatsTimeline (schema StatsTimeline): the rows ordered by start,
// metric, channel, agent, dimension and currency; the runs ordered by queued_at and run ID, at
// most stats.timeline_max_runs, with RunsTruncated when more intersect the range.
type Timeline struct {
	From, To      time.Time
	Buckets       []TimelineBucket
	Runs          []TimelineRun
	RunsTruncated bool
}

// SLOEntry is the traffic light of one SLO (schema StatsSloEntry).
type SLOEntry struct {
	// SLO is service/name of the Sloth file; Metric its sli.* metric.
	SLO, Metric string
	// Objective is the target in percent; Threshold the latency threshold (zero for a ratio SLO).
	Objective float64
	Threshold time.Duration
	// PeriodDays is the SLO period in days.
	PeriodDays int
	// Total and Good are the sums of count and good over the period.
	Total, Good int64
	// Ratio and BudgetRemaining are set only when Total is positive.
	Ratio, BudgetRemaining *float64
	// State is green, amber, red or insufficient_data (schema StatsSloState).
	State string
}

// SLOReport is the result of getStatsSlo (schema StatsSloReport): one entry per SLO of the Sloth
// files, ordered by SLO name, computed at At; measured_by is always MeasuredByOwn.
type SLOReport struct {
	At    time.Time
	Items []SLOEntry
}

// BackendQuery is the input of getStatsBackends (M6-14, 25 6.2).
type BackendQuery struct {
	// From and To are the half-open range.
	From, To time.Time
	// SpaceID restricts to one space; zero means the instance (owners only).
	SpaceID ids.ID
}

// BackendCost is the sum of the cost rows of one agent, billing mode and currency (schema
// StatsBackendCost).
type BackendCost struct {
	BillingMode  string
	Currency     string
	AmountMicros int64
	IsEstimate   bool
}

// RateLimit is the rate-limit state the adapter of an agent reported in its last stored heartbeat
// (schema AgentRateLimitState): never a quota, key or subscription name.
type RateLimit struct {
	State string
	// ResetsAt is zero when not reported.
	ResetsAt time.Time
}

// BackendAgent is the backend view of one agent (schema StatsBackendAgent, ruling T8): no runtime
// name, no adapter kind, no version, no subscription, no seat and no credential (I5).
type BackendAgent struct {
	AgentID ids.ID
	// BillingMode is the configured mode; ReportedBillingMode the reported one ("" for none).
	BillingMode, ReportedBillingMode string
	// BillingModesSupported are the modes of the stored heartbeat; empty when not reported.
	BillingModesSupported []string
	// Switches are the billing mode switches, I2Denied and BillingModeMismatch the denials of the
	// range.
	Switches, I2Denied, BillingModeMismatch int64
	// CostsShown is true with cost.read.aggregate on the target; only then Costs is sent.
	CostsShown bool
	Costs      []BackendCost
	// RateLimit is nil when the adapter never reported one.
	RateLimit *RateLimit
}

// Views is the incoming port of the three query operations of the visualizations (M6-14):
// getStatsTimeline, getStatsSlo and getStatsBackends. The caller is the principal in ctx
// (kernel/actor). Every operation reads the own measurement (stats_minutes) and filters to the
// channels the caller may read (25 section 4); no response names a person or carries content.
//
// Errors of every method: the decision of authorize (401, 403 forbidden, 404 not_found for an
// unreadable space), validation_failed for an invalid range and a source error.
type Views interface {
	// Timeline answers getStatsTimeline: the minute rows and run intervals of a range of at most
	// stats.timeline_max_range inside retention.stats_minutes. Rows without a channel only for
	// stats.read on the instance, cost rows only with cost.read.aggregate.
	//
	// Example:
	//
	//	tl, err := v.Timeline(ctx, stats.TimelineQuery{From: from, To: from.Add(time.Hour), SpaceID: support})
	Timeline(ctx context.Context, q TimelineQuery) (Timeline, error)
	// SLO answers getStatsSlo: the traffic light of every SLO from stats_minutes over its period;
	// stats.read on the instance only.
	SLO(ctx context.Context) (SLOReport, error)
	// Backends answers getStatsBackends: per agent of the visible channels the modes, switches,
	// denials, costs (only with cost.read.aggregate) and the reported rate-limit state, ordered by
	// agent ID.
	Backends(ctx context.Context, q BackendQuery) ([]BackendAgent, error)
}
