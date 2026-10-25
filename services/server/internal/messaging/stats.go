// SPDX-License-Identifier: Apache-2.0

package messaging

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The metric names of the statistics source of messaging (docs/observability/admin-statistics.md
// section 2, M6-05).
const (
	// MetricMessagesHuman, MetricMessagesAgent, MetricMessagesApp and MetricMessagesImport count
	// the messages not deleted by origin; messages of origin system are not counted.
	MetricMessagesHuman  = "messages_human"
	MetricMessagesAgent  = "messages_agent"
	MetricMessagesApp    = "messages_app"
	MetricMessagesImport = "messages_import"
	// MetricActivePeople is the number of distinct people with at least one message of origin
	// human in the bucket and group.
	MetricActivePeople = "active_people"
	// MetricInvocationDenials counts the messages whose trigger was refused with
	// billing_mode_personal_not_holder.
	MetricInvocationDenials = "invocation_denials"
)

// StatsQuery selects the aggregates of a statistics source (M6-05). From and To are a half-open
// range already aligned to Granularity (hour, day, week or month); TimeZone is the IANA name the
// day, week and month buckets start in (UTC for hour). ChannelIDs are the only channels the
// source reads, and GroupIDs is parallel to it: channel i counts toward the group GroupIDs[i]
// (its space). The source never widens the set; an empty set reads nothing.
type StatsQuery struct {
	From, To    time.Time
	Granularity string
	TimeZone    string
	ChannelIDs  []ids.ID
	GroupIDs    []ids.ID
	// CountPeople asks MessageStats for MetricActivePeople, counted inside messaging for exactly
	// the requested bucket, because a monthly distinct count cannot be built from weekly ones.
	CountPeople bool
}

// StatsValue is one aggregate: the value of Metric in the bucket starting at Bucket for the group
// Group (a space ID as text). It never names a person or carries content.
type StatsValue struct {
	Bucket time.Time
	Group  string
	Metric string
	Value  float64
}

// StatsSource is the statistics port of messaging (M6-05). It never authorizes: the stats use
// cases authorize first and pass only the channels the caller may read. No person ID leaves it:
// active people are counted with COUNT(DISTINCT author_principal_id) inside the query.
type StatsSource interface {
	// MessageStats returns the message counts by origin per bucket and group and, when
	// q.CountPeople is set, MetricActivePeople. Errors: validation_failed for an unknown
	// granularity or time zone, a database error.
	//
	// Example:
	//
	//	vs, err := src.MessageStats(ctx, messaging.StatsQuery{From: from, To: to, Granularity: "week",
	//		TimeZone: "UTC", ChannelIDs: chans, GroupIDs: spaces, CountPeople: true})
	MessageStats(ctx context.Context, q StatsQuery) ([]StatsValue, error)
	// InvocationDenials returns MetricInvocationDenials per bucket of the message's creation and
	// group. Errors as MessageStats.
	InvocationDenials(ctx context.Context, q StatsQuery) ([]StatsValue, error)
}
