// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// followupEvents returns the agent.followup events of the journal.
func followupEvents(w *world) []eventlog.Event {
	var out []eventlog.Event
	for _, c := range w.j.list() {
		if c.what == "event" && c.arg.(eventlog.Event).Type == "agent.followup" {
			out = append(out, c.arg.(eventlog.Event))
		}
	}
	return out
}

// openThreadRun opens the run of Lotse in the thread of the world with a first message of Alex.
func openThreadRun(t *testing.T, w *world) app.RunRow {
	t.Helper()
	w.mustPost(w.posted(w.thread))
	if len(w.repo.runs) != 1 {
		t.Fatalf("runs %+v", w.repo.runs)
	}
	return w.repo.runs[0]
}

// TestAccept_FollowupBranch: a decision followup authorizes run.followup.create with the
// descriptor of design note 3 c; an allow inserts the row, raises the run version, sets received
// and appends agent.followup exactly when FollowupEventNeeded; a denial sets failed with the
// mapped reason and writes nothing else.
func TestAccept_FollowupBranch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		reported string
		state    domain.State
		event    bool
	}{
		{"none while running", "", domain.StateQueued, false},
		{"queue while running", "queue", domain.StateRunning, false},
		{"steer while queued", "steer", domain.StateQueued, true},
		{"steer while running", "steer", domain.StateRunning, true},
		{"none while waiting", "", domain.StateWaitingInput, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t)
			f := w.agents[w.lotse]
			f.FollowupsReported = c.reported
			w.agents[w.lotse] = f
			run := openThreadRun(t, w)
			w.repo.runs[0].State = c.state
			before := len(w.j.list())
			ev := w.posted(w.thread)
			w.mustPost(ev)
			var res []authorize.Resource
			for _, call := range w.j.list()[before:] {
				if call.what == "authz:run.followup.create:request" {
					res = append(res, call.arg.(authorize.Resource))
				}
			}
			want := authorize.InvokeFacts{AgentID: w.lotse, TriggerOrigin: "human", ActiveRunAgentID: w.lotse}
			if len(res) != 1 || res[0].ID != w.thread || res[0].Kind != authorize.ResourceThread || *res[0].Invoke != want {
				t.Fatalf("run.followup.create checks %+v", res)
			}
			rows, _ := w.fus.RunFollowups(context.Background(), run.ID)
			if len(rows) != 1 || rows[0].MessageID != ev.MessageID || rows[0].State != domain.FollowupQueued {
				t.Fatalf("rows %+v", rows)
			}
			if w.repo.runs[0].Version != run.Version+1 || len(w.repo.runs) != 1 || w.repo.runs[0].State != c.state {
				t.Fatalf("run %+v", w.repo.runs)
			}
			tr := w.triggers()
			if last := tr[len(tr)-1]; last.MessageID != ev.MessageID || last.State != "received" {
				t.Fatalf("trigger %+v", last)
			}
			evs := followupEvents(w)
			if (len(evs) == 1) != c.event || len(evs) > 1 {
				t.Fatalf("agent.followup events %+v", evs)
			}
			if c.event {
				e := evs[0]
				p, _ := domain.FollowupPayload(run.ID, w.thread, ev.MessageID)
				if e.AudiencePrincipalID != w.lotse || e.ResourceKind != "run" || e.ResourceID != run.ID || e.ResourceVersion != 0 ||
					e.ReceiveAction != "agent.events.consume" || string(e.Payload) != string(p) {
					t.Fatalf("event %+v", e)
				}
			}
		})
	}
	t.Run("denial", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		run := openThreadRun(t, w)
		w.authz.decide = func(act authorize.Action, _ bool, _ authorize.Resource) decision {
			if act == authorize.ActionRunFollowupCreate {
				return decision{reason: "level_insufficient"}
			}
			return decision{allowed: true}
		}
		before := len(w.j.list())
		ev := w.posted(w.thread, w.lotse)
		w.mustPost(ev)
		for _, call := range w.j.list()[before:] {
			switch call.what {
			case "followup:insert", "event", "anchor":
				t.Fatalf("a denied follow-up wrote %s", call.what)
			}
		}
		tr := w.triggers()
		if last := tr[len(tr)-1]; last.MessageID != ev.MessageID || last.State != "failed" || last.Reason != "no_trigger_right" {
			t.Fatalf("trigger %+v", last)
		}
		if w.repo.runs[0].Version != run.Version {
			t.Fatal("a denied follow-up raised the run version")
		}
	})
	t.Run("silent without trigger and mention", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		openThreadRun(t, w)
		w.authz.decide = func(act authorize.Action, _ bool, _ authorize.Resource) decision {
			if act == authorize.ActionRunFollowupCreate {
				return decision{reason: "level_insufficient"}
			}
			return decision{allowed: true}
		}
		before := len(w.j.list())
		w.mustPost(w.posted(w.thread))
		for _, call := range w.j.list()[before:] {
			if call.what == "trigger" || call.what == "followup:insert" || call.what == "authz:run.followup.create:request" {
				t.Fatalf("a person without trigger got %s", call.what)
			}
		}
	})
}

// TestOnRunEnded_ChecksAuthorOfCandidate: at the end of a run the first candidate is checked as
// the trigger of the follow-on run with the re-check principal of its author (not the run's
// triggered_by) and without an active run; its denial discards it with an audit entry of stage
// run_end, and the second candidate becomes the trigger.
func TestOnRunEnded_ChecksAuthorOfCandidate(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	run := openThreadRun(t, w)
	sam, robin := ids.New(), ids.New()
	first, second := ids.New(), ids.New()
	for _, m := range []struct {
		id, author ids.ID
		seq        int64
	}{{second, robin, 9}, {first, sam, 4}} {
		w.msgFacts.m[m.id] = messaging.TriggerMessage{ID: m.id, ThreadID: w.thread, ChannelID: w.channel, Origin: "human", Kind: "text",
			Author: messaging.PrincipalRef{ID: m.author, Kind: "user", Handle: "p", DisplayName: "P"}, ThreadSeq: m.seq, TriggerState: "received"}
		if err := w.fus.InsertFollowup(context.Background(), run.ID, m.id); err != nil {
			t.Fatal(err)
		}
	}
	var calls int
	w.authz.decide = func(act authorize.Action, filter bool, _ authorize.Resource) decision {
		if act == authorize.ActionAgentInvoke && filter {
			calls++
			if calls == 1 {
				return decision{reason: "no_grant"}
			}
		}
		return decision{allowed: true}
	}
	w.clk.Advance(queuedMaxAge + 1)
	before := len(w.authz.principals)
	if err := w.svc.ExpireQueued(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("%d checks of agent.invoke", calls)
	}
	p := w.authz.principals[before]
	if p.PrincipalID() != sam || !authorize.IsRecheckPrincipal(p) {
		t.Fatalf("first check by %v", p)
	}
	var res []authorize.Resource
	for _, c := range w.j.list() {
		if c.what == "authz:agent.invoke:filter" {
			res = append(res, c.arg.(authorize.Resource))
		}
	}
	if r := res[len(res)-2]; !r.Invoke.ActiveRunAgentID.IsZero() || r.Invoke.AgentID != w.lotse {
		t.Fatalf("descriptor %+v", r.Invoke)
	}
	if len(w.repo.runs) != 2 || w.repo.runs[1].TriggeringMessageID != second || w.repo.runs[1].TriggeredBy != robin ||
		w.repo.runs[0].State != domain.StateFailed {
		t.Fatalf("runs %+v", w.repo.runs)
	}
	rows, _ := w.fus.RunFollowups(context.Background(), run.ID)
	states := map[ids.ID]domain.FollowupState{}
	for _, r := range rows {
		states[r.MessageID] = r.State
	}
	if states[first] != domain.FollowupDiscarded || states[second] != domain.FollowupDeferred {
		t.Fatalf("rows %+v", rows)
	}
	es, actors := w.audit.list()
	if len(es) != 1 || !reflect.DeepEqual(es[0].Attrs["stage"], auditing.Code("run_end")) || actors[0].ID != sam || !actors[0].IsRecheckPrincipal() {
		t.Fatalf("audit %+v %+v", es, actors)
	}
	if got := w.msgFacts.m[first]; got.TriggerState != "failed" || got.TriggerStateReason != "access_revoked" {
		t.Fatalf("first trigger %s %s", got.TriggerState, got.TriggerStateReason)
	}
}

// TestFollowupStates_ForwardOnly: SetState moves a follow-up forward only; the first move out of
// queued sets followup_delivered once; a backward move is ErrFollowupTransition.
func TestFollowupStates_ForwardOnly(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	run := openThreadRun(t, w)
	m := ids.New()
	if err := w.fus.InsertFollowup(context.Background(), run.ID, m); err != nil {
		t.Fatal(err)
	}
	fu := w.svc.Followups()
	ctx := actor.WithActor(context.Background(), actor.Actor{ID: w.lotse, Kind: actor.KindAgent})
	set := func(st string) error {
		return uow{}.Do(ctx, func(ctx context.Context) error { return fu.SetState(ctx, run.ID, m, st) })
	}
	delivered := func() int {
		n := 0
		for _, c := range w.triggers() {
			if c.MessageID == m && c.State == "followup_delivered" {
				n++
			}
		}
		return n
	}
	for _, st := range []string{"delivered", "delivered", "absorbed", "absorbed"} {
		if err := set(st); err != nil {
			t.Fatalf("%s: %v", st, err)
		}
	}
	if delivered() != 1 {
		t.Fatalf("followup_delivered %d times", delivered())
	}
	if err := set("delivered"); !errors.Is(err, domain.ErrFollowupTransition) {
		t.Fatalf("backward: %v", err)
	}
	states, err := fu.States(context.Background(), run.ID, []ids.ID{m, ids.New()})
	if err != nil || len(states) != 1 || states[m] != "absorbed" {
		t.Fatalf("states %v %v", states, err)
	}
}
