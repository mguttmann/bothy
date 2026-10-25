// SPDX-License-Identifier: Apache-2.0

package runs

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The metric names of the statistics source of runs (docs/observability/admin-statistics.md
// section 2, M6-05).
const (
	// MetricRuns counts the runs that ended (completed, failed or cancelled) in a bucket.
	MetricRuns = "runs"
	// MetricCompleted, MetricFailed and MetricCancelled count the ended runs by end state.
	MetricCompleted = "completed"
	MetricFailed    = "failed"
	MetricCancelled = "cancelled"
	// MetricFailedReasonPrefix starts the metric of the failed runs of one state_reason code
	// ("failed_reason_billing_mode_mismatch"); codes are the closed list of 09 section 5.2.
	MetricFailedReasonPrefix = "failed_reason_"
	// MetricDurationP50 and MetricDurationP95 are percentiles of the duration from started_at to
	// finished_at in milliseconds, computed with percentile_cont in the query.
	MetricDurationP50 = "duration_ms_p50"
	MetricDurationP95 = "duration_ms_p95"
	// MetricCalls counts the tool calls started in a bucket; MetricFailed counts the failed ones.
	MetricCalls = "calls"
)

// StatsQuery selects the aggregates of the statistics source of runs (M6-05). From and To are a
// half-open range already aligned to Granularity (hour, day, week or month); TimeZone is the IANA
// name the day, week and month buckets start in (UTC for hour). runs has no channel_id (D-M2-05-9),
// so the caller resolves the visible channels to their threads through chat: ThreadIDs are the
// only threads the source reads, and GroupIDs is parallel to it (the group of each thread, its
// space). The source never widens the set; an empty set reads nothing.
type StatsQuery struct {
	From, To    time.Time
	Granularity string
	TimeZone    string
	ThreadIDs   []ids.ID
	GroupIDs    []ids.ID
	// AgentID restricts to one agent; zero means every agent.
	AgentID ids.ID
	// ByAgent groups RunStats by agent ID instead of GroupIDs.
	ByAgent bool
}

// StatsValue is one aggregate: the value of Metric in the bucket starting at Bucket for the group
// Group (a space ID, agent ID or tool name). It never names a person or carries content.
type StatsValue struct {
	Bucket time.Time
	Group  string
	Metric string
	Value  float64
}

// StatsSource is the statistics port of runs (M6-05). It never authorizes: the stats use cases
// authorize first and pass only the threads of the channels the caller may read. The triggering
// person of a run never leaves it.
type StatsSource interface {
	// RunStats returns per bucket of finished_at and group MetricRuns, the end states, one
	// MetricFailedReasonPrefix metric per state_reason code of failed runs that occurs and the
	// duration percentiles of the runs that started. Errors: validation_failed for an unknown
	// granularity or time zone, a database error.
	//
	// Example:
	//
	//	vs, err := src.RunStats(ctx, runs.StatsQuery{From: from, To: to, Granularity: "day", TimeZone: "UTC",
	//		ThreadIDs: threads, GroupIDs: spaces, ByAgent: true})
	RunStats(ctx context.Context, q StatsQuery) ([]StatsValue, error)
	// ToolStats returns per bucket of the step start and tool name (run_steps.name of kind
	// tool_call) MetricCalls, MetricFailed and MetricDurationP50 of the finished calls; GroupIDs
	// is ignored. Errors as RunStats.
	ToolStats(ctx context.Context, q StatsQuery) ([]StatsValue, error)
}
