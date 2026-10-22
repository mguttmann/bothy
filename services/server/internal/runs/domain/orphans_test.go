// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestOrphanVerdict covers interrupted and stale of brief M2-09 notes 10a and 10b and their
// boundaries.
func TestOrphanVerdict(t *testing.T) {
	t.Parallel()
	w := domain.OrphanWindows{PresenceTimeout: 30 * time.Second, OrphanTimeout: 5 * time.Minute}
	start := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	runID, other := ids.New(), ids.New()
	running := domain.LiveRun{ID: runID, State: domain.StateRunning, QueuedAt: start.Add(-time.Second), StartedAt: start}
	waiting := running
	waiting.State = domain.StateWaitingInput
	queued := domain.LiveRun{ID: runID, State: domain.StateQueued, QueuedAt: start}
	beat := func(at time.Time, runs ...ids.ID) domain.HeartbeatState {
		return domain.HeartbeatState{At: at, ActiveRuns: runs, ActiveRunsReported: true}
	}
	rows := []struct {
		name string
		run  domain.LiveRun
		hb   domain.HeartbeatState
		now  time.Time
		want domain.OrphanOutcome
	}{
		{"heartbeat exactly presence_timeout after start", running, beat(start.Add(w.PresenceTimeout), other), start.Add(w.PresenceTimeout), domain.OrphanNone},
		{"heartbeat later than presence_timeout", running, beat(start.Add(w.PresenceTimeout+time.Nanosecond), other), start.Add(time.Minute), domain.OrphanInterrupted},
		{"waiting_input interrupted", waiting, beat(start.Add(time.Minute)), start.Add(time.Minute), domain.OrphanInterrupted},
		{"run listed", running, beat(start.Add(time.Minute), other, runID), start.Add(time.Minute), domain.OrphanNone},
		{"active_runs not reported", running, domain.HeartbeatState{At: start.Add(time.Minute)}, start.Add(time.Minute), domain.OrphanNone},
		{"heartbeat before the start", running, beat(start.Add(-time.Minute)), start.Add(time.Minute), domain.OrphanNone},
		// stale: both the heartbeat and the start must be older than run.orphan_timeout.
		{"no heartbeat, started exactly orphan_timeout ago", running, domain.HeartbeatState{}, start.Add(w.OrphanTimeout), domain.OrphanNone},
		{"no heartbeat, started longer ago", running, domain.HeartbeatState{}, start.Add(w.OrphanTimeout + time.Nanosecond), domain.OrphanStale},
		{"no heartbeat waiting_input", waiting, domain.HeartbeatState{}, start.Add(time.Hour), domain.OrphanStale},
		{"heartbeat exactly orphan_timeout old", running, beat(start.Add(time.Minute), runID), start.Add(time.Minute + w.OrphanTimeout), domain.OrphanNone},
		{"heartbeat older than orphan_timeout", running, beat(start.Add(time.Minute), runID), start.Add(time.Minute + w.OrphanTimeout + time.Nanosecond), domain.OrphanStale},
		{"old heartbeat, recent start", running, beat(start.Add(-time.Hour)), start.Add(w.OrphanTimeout), domain.OrphanNone},
		{"stale before interrupted", running, beat(start.Add(time.Minute)), start.Add(time.Hour), domain.OrphanStale},
		{"recent heartbeat listing the run", running, beat(start.Add(time.Hour), runID), start.Add(time.Hour), domain.OrphanNone},
		// Only started runs.
		{"queued", queued, domain.HeartbeatState{}, start.Add(time.Hour), domain.OrphanNone},
		{"queued missing from a heartbeat", queued, beat(start.Add(time.Hour)), start.Add(time.Hour), domain.OrphanNone},
		{"running without started_at", domain.LiveRun{ID: runID, State: domain.StateRunning}, domain.HeartbeatState{}, start.Add(time.Hour), domain.OrphanNone},
		{"ended", domain.LiveRun{ID: runID, State: domain.StateFailed, StartedAt: start}, domain.HeartbeatState{}, start.Add(time.Hour), domain.OrphanNone},
	}
	for _, r := range rows {
		if got := domain.OrphanVerdict(r.run, r.hb, r.now, w); got != r.want {
			t.Errorf("%s: got %d, want %d", r.name, got, r.want)
		}
	}
}
