// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// answerOf is an agent message of Lotse in thread that names run.
func (w *world) answerOf(thread ids.ID, run *ids.ID) messaging.MessagePosted {
	ev := w.posted(thread)
	ev.AuthorID, ev.AuthorKind, ev.Origin, ev.RunID = w.lotse, "agent", "agent", run
	return ev
}

// putRun stores a run of agent in thread with state.
func (w *world) putRun(thread, agent ids.ID, state domain.State) ids.ID {
	id := ids.New()
	w.repo.mu.Lock()
	w.repo.runs = append(w.repo.runs, app.RunRow{ID: id, ThreadID: thread, AgentID: agent, State: state, Version: 1, QueuedAt: t0})
	w.repo.mu.Unlock()
	return id
}

// TestOnMessagePosted_AnswerBranch: the answer branch of the acceptance (M2-19 design notes 2 f
// and 4): an agent text message with RunID is checked once through LockRunForAnswer and its
// verdict becomes the problem of the request; run and system messages and agent messages without
// RunID never reach it; a RunID on a message of another origin is internal; no agent message
// reaches the trigger decision or SetTriggerState.
func TestOnMessagePosted_AnswerBranch(t *testing.T) {
	t.Parallel()
	t.Run("own active run passes", func(t *testing.T) {
		t.Parallel()
		for _, s := range []domain.State{domain.StateQueued, domain.StateRunning, domain.StateWaitingInput} {
			w := newWorld(t)
			run := w.putRun(w.thread, w.lotse, s)
			w.mustPost(w.answerOf(w.thread, &run))
			if w.repo.answerLocks != 1 || len(w.j.list()) != 0 {
				t.Fatalf("%s: locks %d, calls %v", s, w.repo.answerLocks, w.j.whats())
			}
			if w.repo.runs[0].State != s || w.repo.runs[0].Version != 1 {
				t.Fatalf("%s: the run changed: %+v", s, w.repo.runs[0])
			}
		}
	})
	t.Run("mismatch is validation_failed on /run_id", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		unknown := ids.New()
		elsewhere := w.putRun(w.main, w.lotse, domain.StateRunning)
		foreign := w.putRun(w.thread, w.texter, domain.StateRunning)
		var details []string
		for _, run := range []ids.ID{unknown, elsewhere, foreign} {
			err := w.post(w.answerOf(w.thread, &run))
			e, ok := errs.As(err)
			if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != "/run_id" || e.Fields[0].Code != "run_mismatch" {
				t.Fatalf("run %v: %v", run, err)
			}
			details = append(details, e.Detail+"|"+e.Fields[0].Detail)
		}
		if details[0] != details[1] || details[1] != details[2] {
			t.Fatalf("the answers differ: %q", details)
		}
		if w.repo.answerLocks != 3 || len(w.j.list()) != 0 {
			t.Fatalf("locks %d, calls %v", w.repo.answerLocks, w.j.whats())
		}
	})
	t.Run("ended own run is conflict run_not_active", func(t *testing.T) {
		t.Parallel()
		for _, s := range []domain.State{domain.StateCompleted, domain.StateFailed, domain.StateCancelled} {
			w := newWorld(t)
			run := w.putRun(w.thread, w.lotse, s)
			e, ok := errs.As(w.post(w.answerOf(w.thread, &run)))
			if !ok || e.Code != errs.CodeConflict || e.Reason != "run_not_active" {
				t.Fatalf("%s: %+v", s, e)
			}
		}
	})
	t.Run("run and system kinds never check", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		run := w.putRun(w.thread, w.lotse, domain.StateCompleted)
		for _, kind := range []string{"run", "system"} {
			ev := w.answerOf(w.thread, &run)
			ev.Kind = kind
			if kind == "system" {
				ev.Origin = "system"
			}
			if err := w.post(ev); err != nil && kind == "run" {
				t.Fatalf("%s: %v", kind, err)
			}
		}
		if w.repo.answerLocks != 0 {
			t.Fatalf("a %d run or system message reached the answer check", w.repo.answerLocks)
		}
	})
	t.Run("agent message without RunID does nothing", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.mustPost(w.answerOf(w.thread, nil))
		w.mustPost(w.answerOf(w.main, nil))
		if w.repo.answerLocks != 0 || len(w.j.list()) != 0 || len(w.repo.runs) != 0 {
			t.Fatalf("locks %d, calls %v, runs %v", w.repo.answerLocks, w.j.whats(), w.repo.runs)
		}
	})
	t.Run("RunID on a human text message is internal", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		run := w.putRun(w.thread, w.lotse, domain.StateRunning)
		ev := w.posted(w.thread)
		ev.RunID = &run
		e, ok := errs.As(w.post(ev))
		if !ok || e.Code != errs.CodeInternalError {
			t.Fatalf("got %+v", e)
		}
		if len(w.triggers()) != 0 || w.j.count("anchor") != 0 || w.repo.answerLocks != 0 {
			t.Fatalf("an internal error wrote: %v", w.j.whats())
		}
	})
}
