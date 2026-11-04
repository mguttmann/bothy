// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// failedMessage stores a message of Alex in the thread of w that failed with reason and returns
// its id.
func (w *world) failedMessage(reason string, mentions ...ids.ID) ids.ID {
	msg := ids.New()
	w.msgFacts.mu.Lock()
	w.msgFacts.m[msg] = messaging.TriggerMessage{
		ID: msg, ThreadID: w.thread, ChannelID: w.channel, Origin: "human", Kind: "text", MentionedIDs: mentions,
		Author:       messaging.PrincipalRef{ID: w.alex, Kind: "user", Handle: "alex", DisplayName: "Alex"},
		TriggerState: "failed", TriggerStateReason: reason,
	}
	w.msgFacts.mu.Unlock()
	return msg
}

// retriggerAs runs Retrigger of msg as the person p.
func (w *world) retriggerAs(p, msg ids.ID) error {
	return w.svc.Retrigger(actor.WithActor(context.Background(), actor.Actor{ID: p, Kind: actor.KindUser}), msg)
}

// TestRetrigger (retriggerMessage, 09 3.8, F-PROJEKT-BEREIT): a message that failed because the
// project or the thread operator was not ready is decided again after the fix, by its author
// only; a thread whose old operator never worked in it takes the channel's operator.
func TestRetrigger(t *testing.T) {
	t.Parallel()
	t.Run("a message that failed with project_not_ready opens a run now", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		msg := w.failedMessage("project_not_ready", w.lotse)
		if err := w.retriggerAs(w.alex, msg); err != nil {
			t.Fatal(err)
		}
		if len(w.repo.runs) != 1 || w.repo.runs[0].AgentID != w.lotse || w.repo.runs[0].TriggeringMessageID != msg ||
			w.repo.runs[0].State != domain.StateQueued || w.repo.runs[0].TriggeredBy != w.alex {
			t.Fatalf("runs %+v", w.repo.runs)
		}
		tr := w.triggers()
		if len(tr) != 1 || tr[0].MessageID != msg || tr[0].State != "received" {
			t.Fatalf("trigger states %+v", tr)
		}
		if w.j.count("lock-thread") == 0 || w.j.count("repin") != 0 {
			t.Fatalf("journal %v", w.j.whats())
		}
	})
	t.Run("still not ready: failed again with the reason of now", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		msg := w.failedMessage("operator_cannot_serve_project", w.lotse)
		w.authz.decide = func(act authorize.Action, filter bool, _ authorize.Resource) decision {
			if act == authorize.ActionAgentInvoke && !filter {
				return decision{reason: "project_not_ready"}
			}
			return decision{allowed: true}
		}
		if err := w.retriggerAs(w.alex, msg); err != nil {
			t.Fatal(err)
		}
		failedWith(t, w, "project_not_ready")
	})
	t.Run("the thread takes the channel's operator when the old one never worked in it", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		withChannelOperator(w, w.texter)
		denyChannelWrite(w)
		msg := w.failedMessage("operator_not_in_channel")
		if err := w.retriggerAs(w.alex, msg); err != nil {
			t.Fatal(err)
		}
		if w.j.count("repin") != 1 || w.threads.infos[w.thread].OperatorAgentID != w.texter {
			t.Fatalf("repin %v, thread %+v", w.j.whats(), w.threads.infos[w.thread])
		}
		if len(w.repo.runs) != 1 || w.repo.runs[0].AgentID != w.texter {
			t.Fatalf("runs %+v", w.repo.runs)
		}
	})
	t.Run("an old operator that completed a run keeps the thread", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		withChannelOperator(w, w.texter)
		denyChannelWrite(w)
		w.repo.runs = append(w.repo.runs, app.RunRow{
			ID: ids.New(), ThreadID: w.thread, AgentID: w.lotse, TriggerKind: "message", State: domain.StateCompleted, Version: 3,
		})
		msg := w.failedMessage("operator_not_in_channel")
		if err := w.retriggerAs(w.alex, msg); err != nil {
			t.Fatal(err)
		}
		if w.j.count("repin") != 0 || w.threads.infos[w.thread].OperatorAgentID != w.lotse {
			t.Fatalf("the thread changed its operator: %v", w.j.whats())
		}
		if len(w.repo.runs) != 2 || w.repo.runs[1].AgentID != w.lotse {
			t.Fatalf("runs %+v", w.repo.runs)
		}
	})
	t.Run("only the author may trigger the message again", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		msg := w.failedMessage("project_not_ready")
		wantCode(t, w.retriggerAs(ids.New(), msg), errs.CodeForbidden, "not_self")
		if len(w.repo.runs) != 0 || len(w.triggers()) != 0 {
			t.Fatalf("a refused retrigger wrote: %v", w.j.whats())
		}
	})
	t.Run("a caller without trigger is refused before anything is written", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		msg := w.failedMessage("project_not_ready")
		w.authz.decide = func(act authorize.Action, filter bool, _ authorize.Resource) decision {
			if act == authorize.ActionAgentInvoke && filter {
				return decision{reason: "level_insufficient"}
			}
			return decision{allowed: true}
		}
		wantCode(t, w.retriggerAs(w.alex, msg), errs.CodeAgentInvocationDenied, "level_insufficient")
		if len(w.repo.runs) != 0 || len(w.triggers()) != 0 {
			t.Fatalf("a refused retrigger wrote: %v", w.j.whats())
		}
	})
	t.Run("the rights of triggering are checked with the thread's operator", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		msg := w.failedMessage("project_not_ready")
		if err := w.retriggerAs(w.alex, msg); err != nil {
			t.Fatal(err)
		}
		var res authorize.Resource
		for _, c := range w.j.list() {
			if c.what == "authz:agent.invoke:filter" {
				res = c.arg.(authorize.Resource)
				break
			}
		}
		if res.Kind != authorize.ResourceThread || res.ID != w.thread || res.Invoke == nil || res.Invoke.AgentID != w.lotse {
			t.Fatalf("the filter check %+v", res)
		}
	})
	t.Run("another state, another reason or a deleted message cannot be triggered again", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		for name, mutate := range map[string]func(m *messaging.TriggerMessage){
			"received":         func(m *messaging.TriggerMessage) { m.TriggerState, m.TriggerStateReason = "received", "" },
			"no_trigger_right": func(m *messaging.TriggerMessage) { m.TriggerStateReason = "no_trigger_right" },
			"deleted":          func(m *messaging.TriggerMessage) { m.Deleted = true },
		} {
			msg := w.failedMessage("project_not_ready")
			w.msgFacts.mu.Lock()
			m := w.msgFacts.m[msg]
			mutate(&m)
			w.msgFacts.m[msg] = m
			w.msgFacts.mu.Unlock()
			err := w.retriggerAs(w.alex, msg)
			if e, ok := errs.As(err); !ok || e.PublicCode() != errs.CodeInvalidStateTransition {
				t.Fatalf("%s: %v", name, err)
			}
		}
		if len(w.repo.runs) != 0 || len(w.triggers()) != 0 {
			t.Fatalf("a refused retrigger wrote: %v", w.j.whats())
		}
	})
}
