// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// StatsReader is the outgoing port of Stats: the statistics reads of adapters/postgres
// (queries/stats.sql). It has the semantics of runs.StatsSource without the input checks.
type StatsReader interface {
	RunStats(ctx context.Context, q runs.StatsQuery) ([]runs.StatsValue, error)
	ToolStats(ctx context.Context, q runs.StatsQuery) ([]runs.StatsValue, error)
}

// Stats implements runs.StatsSource (M6-05) over a StatsReader: it checks the input and reads
// nothing for an empty thread set (fail closed, 03 principle 12). It never authorizes; the stats
// use cases did.
type Stats struct {
	r StatsReader
}

// Compile-time check: Stats is the statistics port of runs.
var _ runs.StatsSource = (*Stats)(nil)

// NewStats returns the statistics source over r. Errors: a nil reader.
func NewStats(r StatsReader) (*Stats, error) {
	if r == nil {
		return nil, errors.New("runs: the statistics source needs its reader")
	}
	return &Stats{r: r}, nil
}

// RunStats implements runs.StatsSource. With ByAgent the groups are not read, so GroupIDs may be
// nil; the reader then gets the thread IDs in their place.
func (s *Stats) RunStats(ctx context.Context, q runs.StatsQuery) ([]runs.StatsValue, error) {
	if q.ByAgent && q.GroupIDs == nil {
		q.GroupIDs = q.ThreadIDs
	}
	if err := checkStatsQuery(q.Granularity, q.TimeZone, len(q.ThreadIDs), len(q.GroupIDs)); err != nil {
		return nil, err
	}
	if len(q.ThreadIDs) == 0 {
		return nil, nil
	}
	return s.r.RunStats(ctx, q)
}

// ToolStats implements runs.StatsSource; GroupIDs is ignored.
func (s *Stats) ToolStats(ctx context.Context, q runs.StatsQuery) ([]runs.StatsValue, error) {
	if err := checkStatsQuery(q.Granularity, q.TimeZone, 0, 0); err != nil {
		return nil, err
	}
	if len(q.ThreadIDs) == 0 {
		return nil, nil
	}
	return s.r.ToolStats(ctx, q)
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
