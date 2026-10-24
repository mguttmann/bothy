// SPDX-License-Identifier: Apache-2.0

package stats

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The granularities of a statistics query (schema StatsGranularity).
const (
	GranularityHour  = "hour"
	GranularityDay   = "day"
	GranularityWeek  = "week"
	GranularityMonth = "month"
)

// Query is the input of the statistics operations getUsageStats, getRunStats, getToolCallStats
// and getErrorStats (M6-05).
type Query struct {
	// From and To are the requested half-open range; the use case aligns them to Granularity.
	From, To time.Time
	// Granularity is one of the Granularity constants; "" means GranularityDay.
	Granularity string
	// SpaceID restricts to one space; zero means the instance (owners only).
	SpaceID ids.ID
	// AgentID restricts runs and tool calls to one agent; zero means every agent.
	AgentID ids.ID
}

// CostQuery is the input of getCostStats.
type CostQuery struct {
	Query
	// GroupBy is "space", "agent" or "none"; "" means "none".
	GroupBy string
	// Currency selects the ISO 4217 code; "" means the one currency of the range.
	Currency string
}

// Bucket is one time bucket of one group (schema StatsBucket).
type Bucket struct {
	// Start is the start of the bucket as an instant (local midnight of stats.time_zone for day,
	// week and month).
	Start time.Time
	// GroupKey is the space ID, agent ID or tool name; "" without grouping. Never a person.
	GroupKey string
	// Metrics maps the metric name to its value; a metric without a value is absent.
	Metrics map[string]float64
	// BelowMinGroup names the metrics that count people and were withheld below
	// privacy.stats_min_group_size; nil when none was.
	BelowMinGroup []string
}

// Series is the result of the statistics operations (schema StatsSeries): the buckets ordered by
// start, then group key.
type Series struct {
	Granularity string
	Buckets     []Bucket
}

// CostBucket is one valid cost sum of one bucket, group and billing mode (schema CostBucket).
type CostBucket struct {
	Start       time.Time
	GroupKey    string
	BillingMode string
	// AmountMicros is the sum in the currency of the series.
	AmountMicros int64
	// IsEstimate is true when any summed entry is an estimate.
	IsEstimate bool
}

// CostSeries is the result of getCostStats (schema CostSeries): one currency, the buckets ordered
// by start, group key and billing mode.
type CostSeries struct {
	Granularity string
	Currency    string
	Buckets     []CostBucket
}

// Statistics is the incoming port of the five statistics operations (M6-05,
// docs/observability/admin-statistics.md). The caller is the principal in ctx (kernel/actor).
// Every operation authorizes stats.read (Costs: cost.read.aggregate) on the instance without a
// space and on the space with one, counts only the channels the caller may read, never groups by
// person and caches its result per caller scope for stats.cache_ttl.
//
// Errors of every method: the decision of authorize (403 forbidden, 404 not_found for an
// unreadable space, 401), validation_failed for a range that is empty, inverted, longer than
// stats.max_range or with more than api.stats_max_buckets buckets, and a source error.
type Statistics interface {
	// Usage answers getUsageStats: messages, threads, runs, active people and storage per space.
	//
	// Example:
	//
	//	s, err := st.Usage(ctx, stats.Query{From: from, To: to, Granularity: stats.GranularityWeek, SpaceID: support})
	Usage(ctx context.Context, q Query) (Series, error)
	// Runs answers getRunStats: runs by end state, failure reasons and duration percentiles per
	// agent.
	Runs(ctx context.Context, q Query) (Series, error)
	// ToolCalls answers getToolCallStats: tool calls, failures and median duration per tool name.
	ToolCalls(ctx context.Context, q Query) (Series, error)
	// Errors answers getErrorStats: run failure rate and invocation denials, and for the instance
	// the webhook and push failure rates and the scan results.
	Errors(ctx context.Context, q Query) (Series, error)
	// Costs answers getCostStats: valid cost sums per billing mode in one currency. Errors
	// additionally: validation_failed with errors[].code currency_required when the range has
	// several currencies and q.Currency is empty.
	Costs(ctx context.Context, q CostQuery) (CostSeries, error)
}
