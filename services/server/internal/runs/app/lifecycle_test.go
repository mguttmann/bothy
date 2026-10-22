// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// open opens a run of Lotse in the thread of w for Alex and returns its id.
func (w *world) open(t *testing.T) ids.ID {
	t.Helper()
	ev := w.posted(w.thread)
	w.mustPost(ev)
	return w.repo.runs[len(w.repo.runs)-1].ID
}

// transition runs Transition in a unit of work; veto makes the unit fail after it.
func (w *world) transition(cmd app.TransitionCmd, veto bool) error {
	ctx := actor.WithActor(context.Background(), actor.Actor{ID: w.alex, Kind: actor.KindUser})
	return uow{}.Do(ctx, func(ctx context.Context) error {
		if _, err := w.svc.Transition(ctx, cmd); err != nil {
			return err
		}
		if veto {
			return errVeto
		}
		return nil
	})
}

// TestTransition_MetricsAfterCommitOnly: a vetoed unit records nothing; a committed one records
// the transition once with the agent handle and the queued time.
func TestTransition_MetricsAfterCommitOnly(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	id := w.open(t)
	w.clk.Advance(3 * time.Second)
	cmd := app.TransitionCmd{RunID: id, From: domain.StateQueued, Version: 1, To: domain.StateRunning, SetBy: domain.SetterServer}
	// The fake repository has no rollback; a veto after the change must still record nothing.
	if err := w.transition(cmd, true); !errors.Is(err, errVeto) {
		t.Fatalf("veto: %v", err)
	}
	if len(w.metrics.list) != 0 {
		t.Fatalf("a vetoed unit recorded %+v", w.metrics.list)
	}
	cmd.Version = 2
	cmd.From, cmd.To, cmd.Reason = domain.StateRunning, domain.StateFailed, "stale"
	if err := w.transition(cmd, false); err != nil {
		t.Fatal(err)
	}
	if len(w.metrics.list) != 1 {
		t.Fatalf("recorded %+v", w.metrics.list)
	}
	got := w.metrics.list[0]
	if got.From != domain.StateRunning || got.To != domain.StateFailed || got.AgentHandle != "lotse" || !got.Started ||
		got.Run.StateReason != "stale" {
		t.Fatalf("transition %+v", got)
	}
}

// TestTransition_Rules: a disallowed pair writes nothing; an unknown adapter reason becomes
// adapter_error with a status step; a lost compare-and-swap names the current run; an unknown run
// is not_found; outside a unit Transition is an internal error.
func TestTransition_Rules(t *testing.T) {
	t.Parallel()
	t.Run("disallowed pair", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		id := w.open(t)
		before := len(w.j.list())
		err := w.transition(app.TransitionCmd{RunID: id, From: domain.StateQueued, Version: 1, To: domain.StateCompleted, SetBy: domain.SetterServer}, false)
		if e, ok := errs.As(err); !ok || e.Code != errs.CodeInvalidStateTransition {
			t.Fatalf("err %v", err)
		}
		if len(w.j.list()) != before || w.repo.runs[0].Version != 1 {
			t.Fatalf("a refused transition wrote: %v", w.j.whats()[before:])
		}
	})
	t.Run("adapter reason", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		id := w.open(t)
		if err := w.transition(app.TransitionCmd{RunID: id, From: domain.StateQueued, Version: 1, To: domain.StateFailed,
			Reason: "quota_exhausted", SetBy: domain.SetterAdapter}, false); err != nil {
			t.Fatal(err)
		}
		r := w.repo.runs[0]
		if r.StateReason != "adapter_error" || len(w.repo.steps) != 1 {
			t.Fatalf("run %+v steps %+v", r, w.repo.steps)
		}
		s := w.repo.steps[0]
		if s.Kind != "status" || s.Name != "adapter_reason" || s.Status != "failed" || string(s.Detail) != `{"original_reason":"quota_exhausted"}` ||
			s.Ordinal != 1 || s.FinishedAt.IsZero() {
			t.Fatalf("step %+v detail %s", s, s.Detail)
		}
		tr := w.triggers()
		if last := tr[len(tr)-1]; last.State != "failed" || last.Reason != "adapter_error" {
			t.Fatalf("trigger %+v", tr)
		}
	})
	t.Run("lost swap", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		id := w.open(t)
		cmd := app.TransitionCmd{RunID: id, From: domain.StateQueued, Version: 1, To: domain.StateRunning, SetBy: domain.SetterServer}
		if err := w.transition(cmd, false); err != nil {
			t.Fatal(err)
		}
		err := w.transition(cmd, false)
		var cc *app.ConcurrentChangeError
		if !errors.As(err, &cc) || cc.Current.State != "running" || cc.Current.Version != 2 {
			t.Fatalf("err %v", err)
		}
		if e, ok := errs.As(err); !ok || e.Code != errs.CodeInvalidStateTransition {
			t.Fatalf("default mapping %v", err)
		}
		err = w.transition(app.TransitionCmd{RunID: ids.New(), From: domain.StateQueued, Version: 1, To: domain.StateRunning, SetBy: domain.SetterServer}, false)
		if errs.KindOf(err) != errs.KindNotFound {
			t.Fatalf("unknown run %v", err)
		}
	})
	t.Run("outside a unit", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		_, err := w.svc.Transition(context.Background(), app.TransitionCmd{RunID: ids.New(), From: domain.StateQueued, Version: 1, To: domain.StateRunning})
		if e, ok := errs.As(err); !ok || e.Code != errs.CodeInternalError {
			t.Fatalf("err %v", err)
		}
		_, err = w.svc.Open(context.Background(), app.OpenCmd{})
		if e, ok := errs.As(err); !ok || e.Code != errs.CodeInternalError {
			t.Fatalf("Open err %v", err)
		}
	})
	t.Run("event and trigger per end state", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		id := w.open(t)
		steps := []app.TransitionCmd{
			{RunID: id, From: domain.StateQueued, Version: 1, To: domain.StateRunning, SetBy: domain.SetterServer},
			{RunID: id, From: domain.StateRunning, Version: 2, To: domain.StateWaitingInput, SetBy: domain.SetterServer},
			{RunID: id, From: domain.StateWaitingInput, Version: 3, To: domain.StateCancelled, Reason: "user_cancelled", SetBy: domain.SetterServer},
		}
		for _, c := range steps {
			if err := w.transition(c, false); err != nil {
				t.Fatal(err)
			}
		}
		var states []string
		for _, tr := range w.triggers() {
			states = append(states, tr.State+":"+tr.Reason)
		}
		if want := "received: working: cancelled:user_cancelled"; joinSpace(states) != want {
			t.Fatalf("trigger states %v, want %s", states, want)
		}
		var versions []int64
		for _, c := range w.j.list() {
			if c.what == "event" {
				ev := c.arg.(eventlog.Event)
				if ev.Type == "agent.invocation" {
					continue // the acceptance's last event (M2-08)
				}
				if ev.Type != "run.state_changed" || ev.ReceiveAction != "run.read" || ev.ResourceKind != "run" {
					t.Fatalf("event %+v", ev)
				}
				versions = append(versions, ev.ResourceVersion)
			}
		}
		if len(versions) != 4 || versions[3] != 4 {
			t.Fatalf("event versions %v", versions)
		}
	})
}

// joinSpace joins s with single spaces.
func joinSpace(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += " "
		}
		out += x
	}
	return out
}
