// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// writes are the journal entries that change something.
var writes = []string{"trigger", "anchor", "event", "thread"}

// noWrites fails when the journal holds a write.
func noWrites(t *testing.T, w *world) {
	t.Helper()
	for _, c := range w.j.list() {
		if slices.Contains(writes, c.what) {
			t.Fatalf("unexpected write %s %+v; journal %v", c.what, c.arg, w.j.whats())
		}
	}
	if len(w.repo.runs) != 0 {
		t.Fatalf("unexpected run %+v", w.repo.runs)
	}
}

// failedWith checks exactly one trigger state failed with reason and no run.
func failedWith(t *testing.T, w *world, reason string) {
	t.Helper()
	got := w.triggers()
	if len(got) != 1 || got[0].State != "failed" || got[0].Reason != reason {
		t.Fatalf("trigger states %+v, want failed %s", got, reason)
	}
	if len(w.repo.runs) != 0 || w.j.count("anchor") != 0 || w.j.count("thread") != 0 {
		t.Fatalf("a rejection wrote a run, an anchor or a thread: %v", w.j.whats())
	}
}

// TestAccept_Outcomes: the outcomes of the acceptance with fakes (brief note 6).
func TestAccept_Outcomes(t *testing.T) {
	t.Parallel()
	t.Run("not a human text message writes nothing", func(t *testing.T) {
		t.Parallel()
		for _, origin := range []string{"agent", "app", "import", "system"} {
			w := newWorld(t)
			ev := w.posted(w.thread)
			ev.Origin = origin
			w.mustPost(ev)
			noWrites(t, w)
			if len(w.j.list()) != 0 {
				t.Fatalf("origin %s read %v", origin, w.j.whats())
			}
		}
		for _, kind := range []string{"run", "system"} {
			w := newWorld(t)
			ev := w.posted(w.thread)
			ev.Kind = kind
			w.mustPost(ev)
			noWrites(t, w)
		}
	})
	t.Run("a dm channel writes nothing", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.mustPost(w.posted(w.dmThread, w.lotse))
		noWrites(t, w)
	})
	t.Run("no candidate writes nothing", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.mustPost(w.posted(w.main)) // require_mention in the main thread
		noWrites(t, w)
		if w.j.count("authz:agent.invoke:request")+w.j.count("authz:agent.invoke:filter") != 0 {
			t.Fatalf("authorize without a candidate: %v", w.j.whats())
		}
	})
	t.Run("multiple agents mentioned fails without authorize", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.mustPost(w.posted(w.thread, w.lotse, w.texter))
		failedWith(t, w, "multiple_agents_mentioned")
		for _, c := range w.j.list() {
			if strings.HasPrefix(c.what, "authz:") {
				t.Fatalf("authorize called: %v", w.j.whats())
			}
		}
	})
	t.Run("allow opens the run in the thread", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		ev := w.posted(w.thread)
		w.mustPost(ev)
		if len(w.repo.runs) != 1 {
			t.Fatalf("runs %+v", w.repo.runs)
		}
		r := w.repo.runs[0]
		if r.ThreadID != w.thread || r.AgentID != w.lotse || r.TriggeredBy != w.alex || r.TriggeringMessageID != ev.MessageID ||
			r.State != domain.StateQueued || r.TriggerKind != "message" || r.BillingModeExpected != "organization" {
			t.Fatalf("run %+v", r)
		}
		// The last event is agent.invocation (M2-08).
		want := []string{"authz:agent.invoke:filter", "authz:agent.invoke:request", "anchor", "event", "trigger", "event"}
		got := slices.DeleteFunc(w.j.whats(), func(s string) bool { return !strings.HasPrefix(s, "authz:agent.invoke") && !slices.Contains(writes, s) })
		if !slices.Equal(got, want) {
			t.Fatalf("calls %v, want %v", got, want)
		}
		if tr := w.triggers(); len(tr) != 1 || tr[0].State != "received" || tr[0].MessageID != ev.MessageID || tr[0].Reason != "" {
			t.Fatalf("trigger %+v", tr)
		}
	})
	t.Run("a mention is checked in request mode only", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.mustPost(w.posted(w.thread, w.lotse))
		if w.j.count("authz:agent.invoke:filter") != 0 || w.j.count("authz:agent.invoke:request") != 1 {
			t.Fatalf("calls %v", w.j.whats())
		}
	})
	t.Run("level_insufficient without mention writes nothing and makes no request", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.authz.decide = func(authorize.Action, bool, authorize.Resource) decision {
			return decision{reason: "level_insufficient"}
		}
		w.mustPost(w.posted(w.thread))
		noWrites(t, w)
		if w.j.count("authz:agent.invoke:request") != 0 || w.j.count("authz:agent.invoke:filter") != 1 {
			t.Fatalf("calls %v", w.j.whats())
		}
	})
	for _, m := range []struct{ deny, want string }{
		{"level_insufficient", "no_trigger_right"},
		{"agent_not_reachable", "agent_not_allowed_here"},
		{"billing_mode_personal_not_holder", "billing_mode_personal_not_holder"},
		{"tool_class_owner_only", "tool_class_owner_only"},
		{"project_not_ready", "project_not_ready"},
		{"dev_environment_not_ready", "dev_environment_not_ready"},
		{"run_active_other_agent", "run_active_other_agent"},
		{"channel_read_only", "channel_read_only"},
	} {
		t.Run("denial "+m.deny, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t)
			w.authz.decide = func(_ authorize.Action, filter bool, _ authorize.Resource) decision {
				return decision{reason: m.deny}
			}
			w.mustPost(w.posted(w.thread, w.lotse))
			failedWith(t, w, m.want)
			logged := strings.Contains(w.log.String(), "does not map")
			if logged != (m.deny == "channel_read_only") {
				t.Fatalf("unknown reason logged %v: %s", logged, w.log.String())
			}
			if strings.Contains(w.log.String(), w.alex.String()) || strings.Contains(w.log.String(), w.lotse.String()) {
				t.Fatalf("the log carries an id: %s", w.log.String())
			}
		})
	}
	t.Run("agent_not_reachable of a disabled agent is agent_disabled", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		f := w.agents[w.lotse]
		f.Status = "disabled"
		w.agents[w.lotse] = f
		w.authz.decide = func(authorize.Action, bool, authorize.Resource) decision {
			return decision{reason: "agent_not_reachable"}
		}
		w.mustPost(w.posted(w.thread))
		failedWith(t, w, "agent_disabled")
	})
	t.Run("a disagreement fails closed", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		// Texter is no guest here (allow_mentioned_agents is off), but authorize allows.
		w.mustPost(w.posted(w.thread, w.texter))
		failedWith(t, w, "agent_not_allowed_here")
		if !strings.Contains(w.log.String(), "trigger_decision_mismatch") {
			t.Fatalf("no mismatch line: %s", w.log.String())
		}
		if strings.Contains(w.log.String(), w.texter.String()) {
			t.Fatalf("the log carries an id: %s", w.log.String())
		}
	})
	t.Run("the descriptor names the thread of M with the agent and its active run", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.mustPost(w.posted(w.thread))
		var res authorize.Resource
		for _, c := range w.j.list() {
			if c.what == "authz:agent.invoke:request" {
				res = c.arg.(authorize.Resource)
			}
		}
		if res.Kind != authorize.ResourceThread || res.ID != w.thread || res.Invoke == nil || res.Invoke.AgentID != w.lotse ||
			res.Invoke.TriggerOrigin != "human" || res.Invoke.MentionsAgent || !res.Invoke.ActiveRunAgentID.IsZero() {
			t.Fatalf("descriptor %+v %+v", res, res.Invoke)
		}
	})
}

// TestAccept_AutoThreadOnlyOnAllow: a denial in the main thread creates no thread; an allow
// creates it before Open, and run and anchor live in it while the trigger state stays on M.
func TestAccept_AutoThreadOnlyOnAllow(t *testing.T) {
	t.Parallel()
	t.Run("denial", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.authz.decide = func(authorize.Action, bool, authorize.Resource) decision {
			return decision{reason: "level_insufficient"}
		}
		w.mustPost(w.posted(w.main, w.lotse))
		failedWith(t, w, "no_trigger_right")
	})
	t.Run("allow", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		ev := w.posted(w.main, w.lotse)
		w.mustPost(ev)
		got := slices.DeleteFunc(w.j.whats(), func(s string) bool { return !slices.Contains(writes, s) })
		if want := []string{"thread", "anchor", "event", "trigger", "event"}; !slices.Equal(got, want) {
			t.Fatalf("writes %v, want %v", got, want)
		}
		var anchor messaging.RunAnchorCmd
		var ev2 eventlog.Event
		for _, c := range w.j.list() {
			switch c.what {
			case "anchor":
				anchor = c.arg.(messaging.RunAnchorCmd)
			case "event":
				ev2 = c.arg.(eventlog.Event)
			}
		}
		if anchor.ThreadID == w.main || w.repo.runs[0].ThreadID != anchor.ThreadID || ev2.ThreadID != anchor.ThreadID {
			t.Fatalf("run %+v anchor %+v event %+v", w.repo.runs[0], anchor, ev2)
		}
		if tr := w.triggers(); len(tr) != 1 || tr[0].MessageID != ev.MessageID || tr[0].State != "received" {
			t.Fatalf("trigger %+v", tr)
		}
		for _, c := range w.j.list() {
			if c.what == "authz:agent.invoke:request" && c.arg.(authorize.Resource).ID != w.main {
				t.Fatalf("the check named %v, not the thread of M", c.arg.(authorize.Resource).ID)
			}
		}
	})
}

// TestAccept_UnboundWritesNothing: an acceptor before Bind writes nothing (D-M2-05-19).
func TestAccept_UnboundWritesNothing(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.acc = app.NewAcceptor()
	w.mustPost(w.posted(w.thread))
	noWrites(t, w)
	if err := app.NewAcceptor().Bind(nil); err == nil {
		t.Fatal("Bind(nil) accepted")
	}
	_ = ids.New
}
