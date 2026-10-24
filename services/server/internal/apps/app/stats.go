// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/apps"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
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
	if err := checkStatsQuery(granularity, tz, 0, 0); err != nil {
		return nil, err
	}
	return s.r.WebhookStats(ctx, from, to, granularity, tz)
}

// statsGranularities are the bucket sizes date_trunc of the statistics reads accepts.
var statsGranularities = map[string]bool{"hour": true, "day": true, "week": true, "month": true}

// checkStatsQuery refuses what the statistics reads cannot run: an unknown granularity or time zone
// and parallel lists of different lengths (validation_failed, a programming error of the caller).
func checkStatsQuery(granularity, tz string, keys, groups int) error {
	if !statsGranularities[granularity] {
		return errs.New(errs.CodeValidationFailed, ": unknown statistics granularity")
	}
	if _, err := time.LoadLocation(tz); err != nil || tz == "" || tz == "Local" {
		return errs.New(errs.CodeValidationFailed, ": unknown statistics time zone")
	}
	if keys != groups {
		return errs.New(errs.CodeValidationFailed, ": the statistics keys and groups differ in length")
	}
	return nil
}
