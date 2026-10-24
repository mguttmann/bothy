// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/notify"
)

// StatsReader is the outgoing port of Stats: the statistics read of adapters/postgres
// (queries/stats.sql). It has the semantics of notify.StatsSource without the input checks.
type StatsReader interface {
	PushStats(ctx context.Context, starts, ends []time.Time) ([]notify.StatsValue, error)
}

// Stats implements notify.StatsSource (M6-05) over a StatsReader. It never authorizes; the stats
// use case calls it only for an owner on the instance.
type Stats struct {
	r StatsReader
}

// Compile-time check: Stats is the statistics port of notify.
var _ notify.StatsSource = (*Stats)(nil)

// NewStats returns the statistics source over r. Errors: a nil reader.
func NewStats(r StatsReader) (*Stats, error) {
	if r == nil {
		return nil, errors.New("notify: the statistics source needs its reader")
	}
	return &Stats{r: r}, nil
}

// PushStats implements notify.StatsSource.
func (s *Stats) PushStats(ctx context.Context, starts, ends []time.Time) ([]notify.StatsValue, error) {
	if len(starts) != len(ends) {
		return nil, errs.New(errs.CodeValidationFailed, "notify: the statistics bucket starts and ends differ in length")
	}
	if len(starts) == 0 {
		return nil, nil
	}
	return s.r.PushStats(ctx, starts, ends)
}
