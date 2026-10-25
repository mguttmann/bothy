// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/statskit"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// StatsReader is the outgoing port of Stats: the statistics reads of adapters/postgres
// (queries/stats.sql). It has the semantics of runs.StatsSource without the input checks.
type StatsReader interface {
	RunStats(ctx context.Context, q runs.StatsQuery) ([]runs.StatsValue, error)
	ToolStats(ctx context.Context, q runs.StatsQuery) ([]runs.StatsValue, error)
	// TimelineRuns and TimelineAgents have the semantics of runs.TimelineSource (M6-14).
	TimelineRuns(ctx context.Context, q runs.TimelineQuery) ([]runs.TimelineRun, error)
	TimelineAgents(ctx context.Context, q runs.TimelineQuery) ([]ids.ID, error)
}

// Stats implements runs.StatsSource (M6-05) and runs.TimelineSource (M6-14) over a StatsReader: it checks the input and reads
// nothing for an empty thread set (fail closed, 03 principle 12). It never authorizes; the stats
// use cases did.
type Stats struct {
	r StatsReader
}

// Compile-time checks: Stats is the statistics port and the timeline source of runs.
var (
	_ runs.StatsSource    = (*Stats)(nil)
	_ runs.TimelineSource = (*Stats)(nil)
)

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
	if err := statskit.CheckQuery(q.Granularity, q.TimeZone, len(q.ThreadIDs), len(q.GroupIDs)); err != nil {
		return nil, err
	}
	if len(q.ThreadIDs) == 0 {
		return nil, nil
	}
	return s.r.RunStats(ctx, q)
}

// ToolStats implements runs.StatsSource; GroupIDs is ignored.
func (s *Stats) ToolStats(ctx context.Context, q runs.StatsQuery) ([]runs.StatsValue, error) {
	if err := statskit.CheckQuery(q.Granularity, q.TimeZone, 0, 0); err != nil {
		return nil, err
	}
	if len(q.ThreadIDs) == 0 {
		return nil, nil
	}
	return s.r.ToolStats(ctx, q)
}

// TimelineRuns implements runs.TimelineSource: it refuses a non-positive limit and an inverted
// range and reads nothing for an empty thread set (fail closed).
func (s *Stats) TimelineRuns(ctx context.Context, q runs.TimelineQuery) ([]runs.TimelineRun, error) {
	if q.Limit < 1 {
		return nil, errs.New(errs.CodeValidationFailed, "runs: the timeline needs a positive limit")
	}
	if err := checkTimeline(q); err != nil || len(q.ThreadIDs) == 0 {
		return []runs.TimelineRun{}, err
	}
	return s.r.TimelineRuns(ctx, q)
}

// TimelineAgents implements runs.TimelineSource; the limit is ignored.
func (s *Stats) TimelineAgents(ctx context.Context, q runs.TimelineQuery) ([]ids.ID, error) {
	if err := checkTimeline(q); err != nil || len(q.ThreadIDs) == 0 {
		return []ids.ID{}, err
	}
	return s.r.TimelineAgents(ctx, q)
}

// checkTimeline refuses a range whose start is not before its end.
func checkTimeline(q runs.TimelineQuery) error {
	if !q.From.Before(q.To) {
		return errs.New(errs.CodeValidationFailed, "runs: the timeline range is empty")
	}
	return nil
}
