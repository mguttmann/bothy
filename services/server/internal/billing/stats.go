// SPDX-License-Identifier: Apache-2.0

package billing

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The groupings of CostStatsQuery.GroupBy (getCostStats group_by).
const (
	GroupBySpace = "space"
	GroupByAgent = "agent"
	GroupByNone  = "none"
)

// CostStatsQuery selects the cost aggregates of the statistics (M6-05). From and To are a
// half-open range on occurred_at already aligned to Granularity (hour, day, week or month);
// TimeZone is the IANA name the day, week and month buckets start in (UTC for hour). ChannelIDs
// are the only channels whose entries count; the source never widens the set and an empty set
// reads nothing.
type CostStatsQuery struct {
	From, To    time.Time
	Granularity string
	TimeZone    string
	ChannelIDs  []ids.ID
	// GroupBy is GroupBySpace, GroupByAgent or GroupByNone.
	GroupBy string
	// AgentID restricts to one agent; zero means every agent.
	AgentID ids.ID
	// Currency restricts to one ISO 4217 code; "" means every currency.
	Currency string
}

// CostStatsValue is one valid sum of the statistics: the entries without a successor of one
// bucket, group (space or agent ID as text, "" for GroupByNone), billing mode and currency.
type CostStatsValue struct {
	Bucket                time.Time
	Group                 string
	BillingMode, Currency string
	AmountMicros          int64
	// IsEstimate is true when any summed entry is an estimate.
	IsEstimate bool
}

// StatsSource is the statistics port of billing (M6-05). It never authorizes: the stats use case
// authorizes cost.read.aggregate first and passes only the channels the caller may read. It
// applies the display rule of 10 section 12.2: entries of mode personal count only when
// billing.show_personal_estimates is on.
type StatsSource interface {
	// CostStats returns the valid sums per bucket, group, billing mode and currency, ordered by
	// bucket, group, billing mode and currency. Errors: validation_failed for an unknown
	// granularity, grouping or time zone, a database error.
	//
	// Example:
	//
	//	vs, err := src.CostStats(ctx, billing.CostStatsQuery{From: from, To: to, Granularity: "day", TimeZone: "UTC",
	//		ChannelIDs: chans, GroupBy: billing.GroupBySpace, Currency: "USD"})
	CostStats(ctx context.Context, q CostStatsQuery) ([]CostStatsValue, error)
	// CostCurrencies returns the currencies of the valid entries CostStats would sum for q with
	// its Currency ignored, ascending.
	CostCurrencies(ctx context.Context, q CostStatsQuery) ([]string, error)
}
