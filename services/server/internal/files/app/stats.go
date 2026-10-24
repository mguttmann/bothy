// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// StatsReader is the outgoing port of Stats: the statistics reads of adapters/postgres
// (queries/stats.sql). It has the semantics of files.StatsSource without the input checks.
type StatsReader interface {
	StorageBytes(ctx context.Context, channelIDs, groupIDs []ids.ID) ([]files.StatsValue, error)
	ScanStats(ctx context.Context, from, to time.Time, granularity, tz string) ([]files.StatsValue, error)
}

// Stats implements files.StatsSource (M6-05) over a StatsReader: it checks the input and reads
// nothing for an empty channel set (fail closed, 03 principle 12). It never authorizes; the stats
// use cases did.
type Stats struct {
	r StatsReader
}

// Compile-time check: Stats is the statistics port of files.
var _ files.StatsSource = (*Stats)(nil)

// NewStats returns the statistics source over r. Errors: a nil reader.
func NewStats(r StatsReader) (*Stats, error) {
	if r == nil {
		return nil, errors.New("files: the statistics source needs its reader")
	}
	return &Stats{r: r}, nil
}

// StorageBytes implements files.StatsSource.
func (s *Stats) StorageBytes(ctx context.Context, channelIDs, groupIDs []ids.ID) ([]files.StatsValue, error) {
	if len(channelIDs) != len(groupIDs) {
		return nil, errs.New(errs.CodeValidationFailed, "files: the statistics keys and groups differ in length")
	}
	if len(channelIDs) == 0 {
		return nil, nil
	}
	return s.r.StorageBytes(ctx, channelIDs, groupIDs)
}

// ScanStats implements files.StatsSource.
func (s *Stats) ScanStats(ctx context.Context, from, to time.Time, granularity, tz string) ([]files.StatsValue, error) {
	if err := checkStatsQuery(granularity, tz, 0, 0); err != nil {
		return nil, err
	}
	return s.r.ScanStats(ctx, from, to, granularity, tz)
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
