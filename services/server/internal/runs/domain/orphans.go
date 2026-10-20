// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// HeartbeatState is the last persisted heartbeat of an agent as runs.stall_check sees it
// (agents.last_heartbeat_at and agents.reported_details.active_runs). The worker sees only the
// stored row, which lags the live heartbeat by up to agent.heartbeat_persist_interval.
type HeartbeatState struct {
	// At is the time of the heartbeat; zero when the agent never sent one.
	At time.Time
	// ActiveRuns are the runs the adapter reported as active.
	ActiveRuns []ids.ID
	// ActiveRunsReported is false when the heartbeat carried no active_runs: then no run can be
	// missing from it.
	ActiveRunsReported bool
}

// OrphanWindows are the windows of the stall check.
type OrphanWindows struct {
	// PresenceTimeout is agent.presence_timeout, the window of interrupted.
	PresenceTimeout time.Duration
	// OrphanTimeout is run.orphan_timeout, the window of stale.
	OrphanTimeout time.Duration
}

// OrphanOutcome is the verdict of the stall check on one run.
type OrphanOutcome uint8

// The outcomes of OrphanVerdict.
const (
	// OrphanNone: the run stays as it is.
	OrphanNone OrphanOutcome = iota
	// OrphanInterrupted: the run ends failed with interrupted.
	OrphanInterrupted
	// OrphanStale: the run ends failed with stale.
	OrphanStale
)

// OrphanVerdict decides whether the started run r is orphaned at now, given the last persisted
// heartbeat hb of its agent (09 4.8 and 5.5, brief M2-09 notes 10a and 10b). Only running and
// waiting_input runs with a started_at are checked; every other run is OrphanNone (queued runs
// that wait too long are runs.queued_expiry of M2-08).
//
//   - stale, checked first: the agent has no heartbeat or its last one is older than
//     run.orphan_timeout, and the run started longer than run.orphan_timeout ago.
//   - interrupted: the heartbeat reported active_runs without the run and came more than
//     agent.presence_timeout after started_at (a heartbeat exactly that long after the start is
//     not yet interrupted, so an adapter has one full window to list a run it just started).
//
// Neither outcome is ever retried (R-AG-17).
//
// Example:
//
//	switch domain.OrphanVerdict(run, hb, now, windows) {
//	case domain.OrphanStale:       // failed with stale
//	case domain.OrphanInterrupted: // failed with interrupted
//	}
func OrphanVerdict(run LiveRun, hb HeartbeatState, now time.Time, w OrphanWindows) OrphanOutcome {
	if (run.State != StateRunning && run.State != StateWaitingInput) || run.StartedAt.IsZero() {
		return OrphanNone
	}
	silent := hb.At.IsZero() || now.Sub(hb.At) > w.OrphanTimeout
	if silent && now.Sub(run.StartedAt) > w.OrphanTimeout {
		return OrphanStale
	}
	if !hb.At.IsZero() && hb.ActiveRunsReported && !slices.Contains(hb.ActiveRuns, run.ID) &&
		hb.At.Sub(run.StartedAt) > w.PresenceTimeout {
		return OrphanInterrupted
	}
	return OrphanNone
}
