// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/statskit"
)

// StatsReader is the outgoing port of Stats: the statistics reads of adapters/postgres
// (queries/stats.sql). It has the semantics of chat.StatsSource without the input checks.
type StatsReader interface {
	StatsChannels(ctx context.Context, channelIDs []ids.ID) ([]chat.StatsChannel, error)
	StatsThreads(ctx context.Context, channelIDs []ids.ID, before time.Time) ([]chat.StatsThread, error)
	ThreadStats(ctx context.Context, q chat.StatsQuery) ([]chat.StatsValue, error)
}

// Stats implements chat.StatsSource (M6-05) over a StatsReader: it checks the input and reads
// nothing for an empty channel set (fail closed, 03 principle 12). It never authorizes; the stats
// use cases did.
type Stats struct {
	r StatsReader
}

// Compile-time check: Stats is the statistics port of chat.
var _ chat.StatsSource = (*Stats)(nil)

// NewStats returns the statistics source over r. Errors: a nil reader.
func NewStats(r StatsReader) (*Stats, error) {
	if r == nil {
		return nil, errors.New("chat: the statistics source needs its reader")
	}
	return &Stats{r: r}, nil
}

// StatsChannels implements chat.StatsSource.
func (s *Stats) StatsChannels(ctx context.Context, channelIDs []ids.ID) ([]chat.StatsChannel, error) {
	if len(channelIDs) == 0 {
		return nil, nil
	}
	return s.r.StatsChannels(ctx, channelIDs)
}

// StatsThreads implements chat.StatsSource.
func (s *Stats) StatsThreads(ctx context.Context, channelIDs []ids.ID, before time.Time) ([]chat.StatsThread, error) {
	if len(channelIDs) == 0 {
		return nil, nil
	}
	return s.r.StatsThreads(ctx, channelIDs, before)
}

// ThreadStats implements chat.StatsSource.
func (s *Stats) ThreadStats(ctx context.Context, q chat.StatsQuery) ([]chat.StatsValue, error) {
	if err := statskit.CheckQuery(q.Granularity, q.TimeZone, len(q.ChannelIDs), len(q.GroupIDs)); err != nil {
		return nil, err
	}
	if len(q.ChannelIDs) == 0 {
		return nil, nil
	}
	return s.r.ThreadStats(ctx, q)
}
