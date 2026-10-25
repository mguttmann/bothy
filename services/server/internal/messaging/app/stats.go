// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/kernel/statskit"
	"github.com/OWNER/bothy/services/server/internal/messaging"
)

// StatsReader is the outgoing port of Stats: the statistics reads of adapters/postgres
// (queries/stats.sql). It has the semantics of messaging.StatsSource without the input checks.
type StatsReader interface {
	MessageStats(ctx context.Context, q messaging.StatsQuery) ([]messaging.StatsValue, error)
	InvocationDenials(ctx context.Context, q messaging.StatsQuery) ([]messaging.StatsValue, error)
}

// Stats implements messaging.StatsSource (M6-05) over a StatsReader: it checks the input and reads
// nothing for an empty channel set (fail closed, 03 principle 12). It never authorizes; the stats
// use cases did.
type Stats struct {
	r StatsReader
}

// Compile-time check: Stats is the statistics port of messaging.
var _ messaging.StatsSource = (*Stats)(nil)

// NewStats returns the statistics source over r. Errors: a nil reader.
func NewStats(r StatsReader) (*Stats, error) {
	if r == nil {
		return nil, errors.New("messaging: the statistics source needs its reader")
	}
	return &Stats{r: r}, nil
}

// MessageStats implements messaging.StatsSource.
func (s *Stats) MessageStats(ctx context.Context, q messaging.StatsQuery) ([]messaging.StatsValue, error) {
	if err := statskit.CheckQuery(q.Granularity, q.TimeZone, len(q.ChannelIDs), len(q.GroupIDs)); err != nil {
		return nil, err
	}
	if len(q.ChannelIDs) == 0 {
		return nil, nil
	}
	return s.r.MessageStats(ctx, q)
}

// InvocationDenials implements messaging.StatsSource.
func (s *Stats) InvocationDenials(ctx context.Context, q messaging.StatsQuery) ([]messaging.StatsValue, error) {
	if err := statskit.CheckQuery(q.Granularity, q.TimeZone, len(q.ChannelIDs), len(q.GroupIDs)); err != nil {
		return nil, err
	}
	if len(q.ChannelIDs) == 0 {
		return nil, nil
	}
	return s.r.InvocationDenials(ctx, q)
}
