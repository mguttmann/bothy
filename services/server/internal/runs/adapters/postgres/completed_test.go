// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestRepository_CompletedRunOfAgent (09 3.4 step 4a, addendum of 2026-10-30): only a completed
// run of the agent in the thread counts; a failed run, a run of another agent and a run in
// another thread do not.
func TestRepository_CompletedRunOfAgent(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	w := e.world(t)
	ctx := context.Background()
	t0 := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	want := func(label string, ok bool) {
		t.Helper()
		got, err := e.repo.CompletedRunOfAgent(ctx, w.thread, w.robin)
		if err != nil || got != ok {
			t.Fatalf("%s: CompletedRunOfAgent = %v, %v; want %v", label, got, err, ok)
		}
	}
	want("no run", false)

	failed := newRun(w, w.thread, w.robin, t0)
	e.insert(t, failed, true)
	e.transition(t, app.TransitionWrite{ID: failed.ID, From: domain.StateQueued, Version: 1, To: domain.StateFailed,
		Reason: "adapter_error", At: t0.Add(time.Second)}, true)
	other := newRun(w, w.thread, w.sam, t0.Add(time.Minute))
	e.insert(t, other, true)
	e.transition(t, app.TransitionWrite{ID: other.ID, From: domain.StateQueued, Version: 1, To: domain.StateRunning, At: t0.Add(time.Minute)}, true)
	e.transition(t, app.TransitionWrite{ID: other.ID, From: domain.StateRunning, Version: 2, To: domain.StateCompleted, At: t0.Add(2 * time.Minute)}, true)
	elsewhere := newRun(w, w.other, w.robin, t0)
	e.insert(t, elsewhere, true)
	e.transition(t, app.TransitionWrite{ID: elsewhere.ID, From: domain.StateQueued, Version: 1, To: domain.StateRunning, At: t0}, true)
	e.transition(t, app.TransitionWrite{ID: elsewhere.ID, From: domain.StateRunning, Version: 2, To: domain.StateCompleted, At: t0.Add(time.Minute)}, true)
	want("failed, other agent, other thread", false)

	done := newRun(w, w.thread, w.robin, t0.Add(3*time.Minute))
	e.insert(t, done, true)
	e.transition(t, app.TransitionWrite{ID: done.ID, From: domain.StateQueued, Version: 1, To: domain.StateRunning, At: t0.Add(3 * time.Minute)}, true)
	e.transition(t, app.TransitionWrite{ID: done.ID, From: domain.StateRunning, Version: 2, To: domain.StateCompleted, At: t0.Add(4 * time.Minute)}, true)
	want("completed", true)
}
