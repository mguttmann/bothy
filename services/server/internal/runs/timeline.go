// SPDX-License-Identifier: Apache-2.0

package runs

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TimelineQuery selects the runs of the timeline and the backend view of stats (M6-14): the runs
// of the threads ThreadIDs whose interval [queued_at, finished_at) intersects the half-open range
// [From, To), an unfinished run reaching to now. runs has no channel_id (D-M2-05-9), so the caller
// resolves the visible channels to their threads through chat; the source never widens the set,
// and an empty set reads nothing.
type TimelineQuery struct {
	From, To  time.Time
	ThreadIDs []ids.ID
	// Limit bounds TimelineRuns; it must be positive there and is ignored by TimelineAgents.
	Limit int
}

// TimelineRun is the interval of one run for the timeline (docs/spec/25-visualization.md 6.5):
// never the triggering person, the title or any content.
type TimelineRun struct {
	ID, ThreadID, AgentID ids.ID
	// State is the state of the run now.
	State string
	// QueuedAt is never zero; StartedAt and FinishedAt are zero while the run has not started or
	// ended.
	QueuedAt, StartedAt, FinishedAt time.Time
}

// TimelineSource is the source port of the timeline and the backend view of stats (M6-14). It
// never authorizes: the stats use cases authorize first and pass only the threads of the channels
// the caller may read.
type TimelineSource interface {
	// TimelineRuns returns the runs of q ordered by queued_at, then ID, at most q.Limit. Errors:
	// validation_failed for a non-positive limit or an inverted range, a database error.
	//
	// Example:
	//
	//	rs, err := src.TimelineRuns(ctx, runs.TimelineQuery{From: from, To: to, ThreadIDs: threads, Limit: 5001})
	TimelineRuns(ctx context.Context, q TimelineQuery) ([]TimelineRun, error)
	// TimelineAgents returns the distinct agents of the runs of q, ascending by ID. Errors as
	// TimelineRuns, without the limit.
	TimelineAgents(ctx context.Context, q TimelineQuery) ([]ids.ID, error)
}
