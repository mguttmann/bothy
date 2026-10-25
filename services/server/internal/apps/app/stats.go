// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/apps"
	"github.com/OWNER/bothy/services/server/internal/kernel/statskit"
)

// StatsReader is the outgoing port of Stats: the statistics read of adapters/postgres
// (queries/stats.sql). It has the semantics of apps.StatsSource without the input checks.
type StatsReader interface {
	WebhookStats(ctx context.Context, from, to time.Time, granularity, tz string) ([]apps.StatsValue, error)
}

// Stats implements apps.StatsSource (M6-05) over a StatsReader. It never authorizes; the stats use
// case calls it only for an owner on the instance.
type Stats struct {
	r StatsReader
}

// Compile-time check: Stats is the statistics port of apps.
var _ apps.StatsSource = (*Stats)(nil)

// NewStats returns the statistics source over r. Errors: a nil reader.
func NewStats(r StatsReader) (*Stats, error) {
	if r == nil {
		return nil, errors.New("apps: the statistics source needs its reader")
	}
	return &Stats{r: r}, nil
}

// WebhookStats implements apps.StatsSource.
func (s *Stats) WebhookStats(ctx context.Context, from, to time.Time, granularity, tz string) ([]apps.StatsValue, error) {
	if err := statskit.CheckQuery(granularity, tz, 0, 0); err != nil {
		return nil, err
	}
	return s.r.WebhookStats(ctx, from, to, granularity, tz)
}
