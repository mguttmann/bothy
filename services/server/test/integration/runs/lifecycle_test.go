// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// open calls Open in a unit of work for thread.
func (w *world) open(thread ids.ID) error {
	return w.inUnit(func(ctx context.Context) error {
		_, err := w.runs.Open(ctx, app.OpenCmd{ThreadID: thread, AgentID: w.lotse, TriggeredBy: w.alex, TriggerKind: "message",
			BillingModeExpected: "organization"})
		return err
	})
}

// TestOpen_AtMostOneActivePerThread (R-DM-14): two units open a run in the same thread at the
// same time; exactly one run exists and the other unit gets ActiveRunExistsError with it; after the
// first run ended a new Open succeeds.
func TestOpen_AtMostOneActivePerThread(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	th := w.newThread(w.allgemein)
	errsOf := make([]error, 2)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range errsOf {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errsOf[i] = w.open(th)
		}()
	}
	close(start)
	wg.Wait()
	runs := w.runsWhere(`thread_id = $1`, th.String())
	if len(runs) != 1 {
		t.Fatalf("%d runs", len(runs))
	}
	var exists *app.ActiveRunExistsError
	ok, lost := 0, 0
	for _, err := range errsOf {
		switch {
		case err == nil:
			ok++
		case errors.As(err, &exists):
			lost++
			if exists.Active.ID.String() != runs[0].ID {
				t.Fatalf("ActiveRunExistsError names %v, want %s", exists.Active.ID, runs[0].ID)
			}
		default:
			t.Fatalf("Open: %v", err)
		}
	}
	if ok != 1 || lost != 1 {
		t.Fatalf("ok %d, lost %d", ok, lost)
	}
	if n := w.count(`SELECT count(*) FROM messages WHERE kind = 'run' AND thread_id = $1`, th.String()); n != 1 {
		t.Fatalf("%d anchors", n)
	}
	w.mustTransition(runs[0].ID, domain.StateQueued, domain.StateCancelled, 1, "user_cancelled", domain.SetterServer)
	if err := w.open(th); err != nil {
		t.Fatalf("Open after the end: %v", err)
	}
}

// runIn returns a run of the world brought into state s through allowed transitions.
func (w *world) runIn(s domain.State) runRow {
	w.t.Helper()
	r, _ := w.openRun()
	path := map[domain.State][]domain.State{
		domain.StateQueued:       nil,
		domain.StateRunning:      {domain.StateRunning},
		domain.StateWaitingInput: {domain.StateRunning, domain.StateWaitingInput},
		domain.StateCompleted:    {domain.StateRunning, domain.StateCompleted},
		domain.StateFailed:       {domain.StateFailed},
		domain.StateCancelled:    {domain.StateCancelled},
	}[s]
	from := domain.StateQueued
	for i, to := range path {
		reason := map[domain.State]string{domain.StateFailed: "stale", domain.StateCancelled: "user_cancelled"}[to]
		w.mustTransition(r.ID, from, to, int64(i+1), reason, domain.SetterServer)
		from = to
	}
	r.State, r.Version = string(s), int64(len(path)+1)
	return r
}

// TestTransition_EveryDisallowedPairRejected: for each persisted state and each target outside
// the allowed pairs, 409 invalid_state_transition, unchanged row and version, no event.
func TestTransition_EveryDisallowedPairRejected(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	for _, from := range domain.States() {
		r := w.runIn(from)
		for _, to := range domain.States() {
			if domain.CanTransition(from, to) {
				continue
			}
			events := w.eventCount()
			reason := map[domain.State]string{domain.StateFailed: "stale", domain.StateCancelled: "user_cancelled"}[to]
			err := w.transition(app.TransitionCmd{RunID: ids.MustParse(r.ID), From: from, Version: r.Version, To: to, Reason: reason, SetBy: domain.SetterServer})
			if e, ok := errs.As(err); !ok || e.Code != errs.CodeInvalidStateTransition {
				t.Fatalf("%s -> %s: %v", from, to, err)
			}
			got := w.runsWhere(`id = $1`, r.ID)[0]
			if got.State != string(from) || got.Version != r.Version || w.eventCount() != events {
				t.Fatalf("%s -> %s changed the run %+v or appended events", from, to, got)
			}
		}
	}
}

// TestTransition_RaceOfTwoStartsOneWins (RK-18): two goroutines move the same queued run to
// running with the same version; one succeeds, the other gets ConcurrentChangeError with state
// running; exactly one run.state_changed with running and one message.updated with working.
func TestTransition_RaceOfTwoStartsOneWins(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, msg := w.openRun()
	before := w.maxSeq()
	results := make([]error, 2)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i] = w.transition(app.TransitionCmd{RunID: ids.MustParse(r.ID), From: domain.StateQueued, Version: 1, To: domain.StateRunning,
				SetBy: domain.SetterServer})
		}()
	}
	close(start)
	wg.Wait()
	ok := 0
	for _, err := range results {
		var cc *app.ConcurrentChangeError
		switch {
		case err == nil:
			ok++
		case errors.As(err, &cc):
			if cc.Current.State != "running" {
				t.Fatalf("current %+v", cc.Current)
			}
		default:
			t.Fatal(err)
		}
	}
	if ok != 1 {
		t.Fatalf("%d starts won", ok)
	}
	got := types(w.eventsAfter(before))
	if len(got) != 2 || got[0] != "run.state_changed" || got[1] != "message.updated" {
		t.Fatalf("events %v", got)
	}
	if st, _, v := w.triggerOf(msg); st != "working" || v != 3 {
		t.Fatalf("trigger %s version %d", st, v)
	}
}

// TestTransition_TriggerStatePerEndState: completed gives done, failed with stale gives failed
// with stale, cancelled with user_cancelled gives cancelled with user_cancelled; each raises the
// message version by one with one message.updated; running -> waiting_input changes no message.
func TestTransition_TriggerStatePerEndState(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	for _, c := range []struct {
		to             domain.State
		reason, st, re string
	}{
		{domain.StateCompleted, "", "done", ""},
		{domain.StateFailed, "stale", "failed", "stale"},
		{domain.StateCancelled, "user_cancelled", "cancelled", "user_cancelled"},
	} {
		r, msg := w.openRun()
		w.mustTransition(r.ID, domain.StateQueued, domain.StateRunning, 1, "", domain.SetterServer)
		_, _, v := w.triggerOf(msg)
		before := w.maxSeq()
		w.mustTransition(r.ID, domain.StateRunning, domain.StateWaitingInput, 2, "", domain.SetterServer)
		if _, _, v2 := w.triggerOf(msg); v2 != v || len(w.eventsAfter(before)) != 1 {
			t.Fatalf("waiting_input changed the message: version %d -> %d, events %v", v, v2, types(w.eventsAfter(before)))
		}
		w.mustTransition(r.ID, domain.StateWaitingInput, domain.StateRunning, 3, "", domain.SetterServer)
		_, _, v = w.triggerOf(msg)
		before = w.maxSeq()
		w.mustTransition(r.ID, domain.StateRunning, c.to, 4, c.reason, domain.SetterServer)
		st, re, v2 := w.triggerOf(msg)
		if st != c.st || re != c.re || v2 != v+1 {
			t.Fatalf("%s: trigger %s %s version %d -> %d", c.to, st, re, v, v2)
		}
		if got := types(w.eventsAfter(before)); len(got) != 2 || got[1] != "message.updated" {
			t.Fatalf("%s: events %v", c.to, got)
		}
	}
}

// TestTransition_UnknownAdapterReasonBecomesAdapterError: an adapter failed with quota_exhausted
// stores adapter_error and one status step adapter_reason with the original; runtime_error is kept
// without a step; stale from an adapter becomes adapter_error.
func TestTransition_UnknownAdapterReasonBecomesAdapterError(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	for _, c := range []struct{ in, stored, original string }{
		{"quota_exhausted", "adapter_error", "quota_exhausted"},
		{"runtime_error", "runtime_error", ""},
		{"stale", "adapter_error", "stale"},
	} {
		r, _ := w.openRun()
		w.mustTransition(r.ID, domain.StateQueued, domain.StateFailed, 1, c.in, domain.SetterAdapter)
		if got := w.runsWhere(`id = $1`, r.ID)[0]; got.Reason != c.stored {
			t.Fatalf("%s: stored %s", c.in, got.Reason)
		}
		n := w.count(`SELECT count(*) FROM run_steps WHERE run_id = $1`, r.ID)
		if c.original == "" {
			if n != 0 {
				t.Fatalf("%s: %d steps", c.in, n)
			}
			continue
		}
		detail := w.text(`SELECT detail->>'original_reason' FROM run_steps WHERE run_id = $1 AND kind = 'status' AND name = 'adapter_reason'
			AND status = 'failed' AND ordinal = 1 AND finished_at IS NOT NULL`, r.ID)
		if n != 1 || detail != c.original {
			t.Fatalf("%s: %d steps, detail %q", c.in, n, detail)
		}
	}
}

// TestTransition_EndStateTimes: finished_at on every end state, duration_ms only for started runs,
// started_at kept on waiting_input -> running.
func TestTransition_EndStateTimes(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	times := func(id string) (started, finished time.Time, duration *int64) {
		must(t, w.db.QueryRow(t.Context(), `SELECT coalesce(started_at, 'epoch'), coalesce(finished_at, 'epoch'), duration_ms FROM runs WHERE id = $1`, id).
			Scan(&started, &finished, &duration))
		return started, finished, duration
	}
	never := w.runIn(domain.StateCancelled)
	if s, f, d := times(never.ID); s.Unix() != 0 || f.Unix() == 0 || d != nil {
		t.Fatalf("never started: %v %v %v", s, f, d)
	}
	r, _ := w.openRun()
	w.mustTransition(r.ID, domain.StateQueued, domain.StateRunning, 1, "", domain.SetterServer)
	first, _, _ := times(r.ID)
	w.clk.Advance(2 * time.Second)
	w.mustTransition(r.ID, domain.StateRunning, domain.StateWaitingInput, 2, "", domain.SetterServer)
	w.mustTransition(r.ID, domain.StateWaitingInput, domain.StateRunning, 3, "", domain.SetterServer)
	if again, _, _ := times(r.ID); !again.Equal(first) {
		t.Fatalf("started_at moved %v -> %v", first, again)
	}
	w.clk.Advance(time.Second)
	w.mustTransition(r.ID, domain.StateRunning, domain.StateCompleted, 4, "", domain.SetterServer)
	s, f, d := times(r.ID)
	if f.IsZero() || d == nil || *d != f.Sub(s).Milliseconds() || *d != 3000 {
		t.Fatalf("completed: %v %v %v", s, f, d)
	}
	if len(w.metrics.list) == 0 {
		t.Fatal("no metrics after the commits")
	}
}
