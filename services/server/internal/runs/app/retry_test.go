// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// endedRun adds an ended run of agent in the thread of w for a new message of Alex that mentions
// mentions, queued at queued, and returns it.
func (w *world) endedRun(agent ids.ID, state domain.State, queued time.Time, mentions ...ids.ID) app.RunRow {
	msg := ids.New()
	w.msgFacts.mu.Lock()
	w.msgFacts.m[msg] = messaging.TriggerMessage{
		ID: msg, ThreadID: w.thread, ChannelID: w.channel, Origin: "human", Kind: "text", MentionedIDs: mentions,
		Author:       messaging.PrincipalRef{ID: w.alex, Kind: "user", Handle: "alex", DisplayName: "Alex"},
		TriggerState: "failed", TriggerStateReason: "adapter_error",
	}
	w.msgFacts.mu.Unlock()
	r := app.RunRow{
		ID: ids.New(), ThreadID: w.thread, AgentID: agent, TriggerKind: "message", TriggeringMessageID: msg, TriggeredBy: w.alex,
		State: state, Version: 2, BillingModeExpected: "organization", QueuedAt: queued,
	}
	if state == domain.StateFailed {
		r.StateReason = "adapter_error"
	}
	w.repo.mu.Lock()
	w.repo.runs = append(w.repo.runs, r)
	w.repo.mu.Unlock()
	return r
}

// retryAs runs Retry of run as the person p.
func (w *world) retryAs(p, run ids.ID) (runs.RunSummary, error) {
	return w.svc.Retry(actor.WithActor(context.Background(), actor.Actor{ID: p, Kind: actor.KindUser}), runs.RetryRun{RunID: run})
}

// wantCode fails unless err is a kernel error with code (and reason, when not empty).
func wantCode(t *testing.T, err error, code errs.Code, reason string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.PublicCode() != code || (reason != "" && e.Reason != reason) {
		t.Fatalf("error %v, want %s %s", err, code, reason)
	}
}

// TestRetry (09 5.7, addendum of 2026-10-30): a failed or cancelled run of the newest trigger of a
// thread can be started again for the same message, with the rights of triggering, once.
func TestRetry(t *testing.T) {
	t.Parallel()
	t.Run("a failed run starts a new run for the same message", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		old := w.endedRun(w.lotse, domain.StateFailed, t0.Add(-time.Minute), w.lotse)
		sam := ids.New()
		sum, err := w.retryAs(sam, old.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(w.repo.runs) != 2 {
			t.Fatalf("runs %+v", w.repo.runs)
		}
		r := w.repo.runs[1]
		if sum.ID != r.ID || r.ThreadID != w.thread || r.AgentID != w.lotse || r.TriggeringMessageID != old.TriggeringMessageID ||
			r.TriggeredBy != sam || r.State != domain.StateQueued || r.TriggerKind != "message" {
			t.Fatalf("new run %+v, summary %+v", r, sum)
		}
		tr := w.triggers()
		if len(tr) != 1 || tr[0].MessageID != old.TriggeringMessageID || tr[0].State != "received" {
			t.Fatalf("trigger states %+v", tr)
		}
		if w.j.count("event") < 2 || w.j.count("anchor") != 1 {
			t.Fatalf("a retry writes the anchor, run.state_changed and agent.invocation: %v", w.j.whats())
		}
		var res authorize.Resource
		for _, c := range w.j.list() {
			if c.what == "authz:agent.invoke:request" {
				res = c.arg.(authorize.Resource)
			}
		}
		if res.Kind != authorize.ResourceThread || res.ID != w.thread || res.Invoke == nil || res.Invoke.AgentID != w.lotse ||
			!res.Invoke.MentionsAgent || res.Invoke.TriggerOrigin != "human" || !res.Invoke.ActiveRunAgentID.IsZero() {
			t.Fatalf("agent.invoke descriptor %+v %+v", res, res.Invoke)
		}
	})
	t.Run("a cancelled run can be retried", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		old := w.endedRun(w.lotse, domain.StateCancelled, t0.Add(-time.Minute))
		if _, err := w.retryAs(w.alex, old.ID); err != nil || len(w.repo.runs) != 2 {
			t.Fatalf("retry: %v, runs %+v", err, w.repo.runs)
		}
	})
	t.Run("a second retry of the same run answers the first retry and writes nothing", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		old := w.endedRun(w.lotse, domain.StateFailed, t0.Add(-time.Minute))
		first, err := w.retryAs(w.alex, old.ID)
		if err != nil {
			t.Fatal(err)
		}
		before := len(w.j.list())
		again, err := w.retryAs(w.alex, old.ID)
		if err != nil || again.ID != first.ID || len(w.repo.runs) != 2 {
			t.Fatalf("again %+v, %v; runs %d", again, err, len(w.repo.runs))
		}
		for _, c := range w.j.list()[before:] {
			if c.what == "trigger" || c.what == "anchor" || c.what == "event" {
				t.Fatalf("the replay wrote %s", c.what)
			}
		}
	})
	t.Run("only failed and cancelled runs", func(t *testing.T) {
		t.Parallel()
		for _, st := range []domain.State{domain.StateCompleted, domain.StateRunning, domain.StateQueued} {
			w := newWorld(t)
			old := w.endedRun(w.lotse, st, t0.Add(-time.Minute))
			_, err := w.retryAs(w.alex, old.ID)
			wantCode(t, err, errs.CodeInvalidStateTransition, "")
			if len(w.repo.runs) != 1 {
				t.Fatalf("%s: runs %+v", st, w.repo.runs)
			}
		}
	})
	t.Run("only the newest run of the thread", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		old := w.endedRun(w.lotse, domain.StateFailed, t0.Add(-2*time.Minute))
		w.endedRun(w.lotse, domain.StateCompleted, t0.Add(-time.Minute))
		_, err := w.retryAs(w.alex, old.ID)
		wantCode(t, err, errs.CodeInvalidStateTransition, "")
		if len(w.repo.runs) != 2 {
			t.Fatalf("runs %+v", w.repo.runs)
		}
	})
	t.Run("a denial of agent.invoke answers its reason and opens nothing", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		old := w.endedRun(w.lotse, domain.StateFailed, t0.Add(-time.Minute))
		w.authz.decide = func(act authorize.Action, _ bool, _ authorize.Resource) decision {
			if act == authorize.ActionAgentInvoke {
				return decision{reason: "billing_mode_personal_not_holder"}
			}
			return decision{allowed: true}
		}
		_, err := w.retryAs(w.alex, old.ID)
		wantCode(t, err, errs.CodeAgentInvocationDenied, "billing_mode_personal_not_holder")
		if len(w.repo.runs) != 1 || len(w.triggers()) != 0 {
			t.Fatalf("a denied retry wrote: runs %+v, %v", w.repo.runs, w.j.whats())
		}
	})
	t.Run("a thread operator outside the channel is refused with the notice", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		withChannelOperator(w, ids.ID{})
		denyChannelWrite(w)
		old := w.endedRun(w.lotse, domain.StateFailed, t0.Add(-time.Minute))
		_, err := w.retryAs(w.alex, old.ID)
		wantCode(t, err, errs.CodeAgentInvocationDenied, string(domain.ReasonOperatorNotInChannel))
		if len(w.repo.runs) != 1 || w.j.count("authz:agent.invoke:request") != 0 {
			t.Fatalf("runs %+v, calls %v", w.repo.runs, w.j.whats())
		}
	})
	t.Run("an invisible or unknown run is not found and reveals no state", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		old := w.endedRun(w.lotse, domain.StateCompleted, t0.Add(-time.Minute))
		w.authz.decide = func(act authorize.Action, _ bool, _ authorize.Resource) decision {
			if act == authorize.ActionRunRead {
				return decision{reason: "no_grant"}
			}
			return decision{allowed: true}
		}
		_, err := w.retryAs(w.alex, old.ID)
		wantCode(t, err, errs.CodeNotFound, "")
		_, err = w.retryAs(w.alex, ids.New())
		wantCode(t, err, errs.CodeNotFound, "")
	})
}
