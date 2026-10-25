// SPDX-License-Identifier: Apache-2.0

package chat

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The metric names of ThreadStats (docs/observability/admin-statistics.md section 2, M6-05).
const (
	// MetricThreadsCreated counts the threads of kind thread created in a bucket.
	MetricThreadsCreated = "threads_created"
	// MetricThreadsDone counts the threads closed in a bucket that are still closed.
	MetricThreadsDone = "threads_done"
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
}

// StatsValue is one aggregate: the value of Metric in the bucket starting at Bucket for the group
// Group (a space ID as text). It never names a person or carries content.
type StatsValue struct {
	Bucket time.Time
	Group  string
	Metric string
	Value  float64
}

// StatsChannel is a channel with a space: the statistics count channels by their space.
type StatsChannel struct {
	ChannelID, SpaceID ids.ID
}

// StatsThread is a thread with its current channel: the statistics resolve the runs of the visible
// channels through their threads, because runs has no channel_id (docs/DECISIONS.md D-M2-05-9).
type StatsThread struct {
	ThreadID, ChannelID ids.ID
}

// StatsSource is the statistics port of chat (M6-05): the structure the statistics group by and
// the thread counts. It never authorizes: the stats use cases authorize first and pass only the
// channels the caller may read.
type StatsSource interface {
	// StatsChannels returns the channels of channelIDs that belong to a space, with the space,
	// ascending by channel; channels without a space (DMs) and unknown IDs are left out.
	//
	// Example:
	//
	//	cs, err := src.StatsChannels(ctx, scope.ChannelIDs)
	StatsChannels(ctx context.Context, channelIDs []ids.ID) ([]StatsChannel, error)
	// StatsThreads returns the threads of channelIDs created before before, with their channel,
	// ascending by thread.
	StatsThreads(ctx context.Context, channelIDs []ids.ID, before time.Time) ([]StatsThread, error)
	// ThreadStats returns MetricThreadsCreated and MetricThreadsDone per bucket and group. Errors:
	// validation_failed for an unknown granularity or time zone, a database error.
	ThreadStats(ctx context.Context, q StatsQuery) ([]StatsValue, error)
}
