// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// withChannelOperator sets the effective operator of the channel of w (zero: none).
func withChannelOperator(w *world, op ids.ID) {
	ch := w.channels[w.channel]
	ch.EffectiveOperatorID = op
	w.channels[w.channel] = ch
}

// denyChannelWrite makes every agent lack a write of its own on the channel (thread.create on the
// channel, 09 3.4 step 4a); everything else is allowed.
func denyChannelWrite(w *world) {
	w.authz.decide = func(act authorize.Action, _ bool, _ authorize.Resource) decision {
		if act == authorize.ActionThreadCreate {
			return decision{reason: "level_insufficient"}
		}
		return decision{allowed: true}
	}
}

// TestAccept_ThreadOperatorOutside is the trigger rule 09 3.4 step 4a (addendum of 2026-10-30): a
// thread whose fixed operator the channel no longer has (an imported thread after a switch-over
// without operator) gets no run, only the notice operator_not_in_channel; an operator that
// completed a run in the thread keeps triggering (R-DM-12).
func TestAccept_ThreadOperatorOutside(t *testing.T) {
	t.Parallel()
	t.Run("channel without operator fails with the notice and opens no run", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		withChannelOperator(w, ids.ID{})
		denyChannelWrite(w)
		w.mustPost(w.posted(w.thread))
		failedWith(t, w, string(domain.ReasonOperatorNotInChannel))
		if w.j.count("authz:agent.invoke:request") != 0 {
			t.Fatalf("a request-mode check for an operator outside the channel: %v", w.j.whats())
		}
		var res authorize.Resource
		for _, c := range w.j.list() {
			if c.what == "authz:thread.create:filter" {
				res = c.arg.(authorize.Resource)
			}
		}
		if res.Kind != authorize.ResourceChannel || res.ID != w.channel {
			t.Fatalf("the channel write of the operator was not checked on the channel: %+v", res)
		}
	})
	t.Run("another channel operator does not take the thread over", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		withChannelOperator(w, w.texter)
		denyChannelWrite(w)
		w.mustPost(w.posted(w.thread, w.lotse))
		failedWith(t, w, string(domain.ReasonOperatorNotInChannel))
	})
	t.Run("a person without trigger who did not mention it gets nothing", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		withChannelOperator(w, ids.ID{})
		w.authz.decide = func(authorize.Action, bool, authorize.Resource) decision {
			return decision{reason: "level_insufficient"}
		}
		w.mustPost(w.posted(w.thread))
		noWrites(t, w)
	})
	t.Run("an operator with a write of its own on the channel keeps triggering", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		withChannelOperator(w, ids.ID{})
		w.mustPost(w.posted(w.thread))
		if len(w.repo.runs) != 1 || w.repo.runs[0].AgentID != w.lotse {
			t.Fatalf("runs %+v", w.repo.runs)
		}
	})
	t.Run("an operator that completed a run in the thread keeps triggering", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		withChannelOperator(w, ids.ID{})
		denyChannelWrite(w)
		w.repo.runs = append(w.repo.runs, app.RunRow{
			ID: ids.New(), ThreadID: w.thread, AgentID: w.lotse, TriggerKind: "message", State: domain.StateCompleted, Version: 3,
		})
		w.mustPost(w.posted(w.thread))
		if len(w.repo.runs) != 2 || w.repo.runs[1].AgentID != w.lotse || w.repo.runs[1].State != domain.StateQueued {
			t.Fatalf("runs %+v", w.repo.runs)
		}
	})
	t.Run("a failed run of the operator does not count as work in the thread", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		withChannelOperator(w, ids.ID{})
		denyChannelWrite(w)
		w.repo.runs = append(w.repo.runs, app.RunRow{
			ID: ids.New(), ThreadID: w.thread, AgentID: w.lotse, TriggerKind: "message", State: domain.StateFailed, Version: 2,
		})
		w.mustPost(w.posted(w.thread))
		got := w.triggers()
		if len(got) != 1 || got[0].State != "failed" || got[0].Reason != string(domain.ReasonOperatorNotInChannel) || len(w.repo.runs) != 1 {
			t.Fatalf("trigger states %+v, runs %+v", got, w.repo.runs)
		}
	})
}
