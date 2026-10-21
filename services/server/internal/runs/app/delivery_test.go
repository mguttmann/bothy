// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// deliveryWorld is the app world with bound tokens and one queued run of Lotse in the thread T,
// triggered by Alex with a mention.
type deliveryWorld struct {
	*world
	run     app.RunRow
	message ids.ID
}

func newDeliveryWorld(t *testing.T) *deliveryWorld {
	t.Helper()
	w := newWorld(t)
	if err := w.svc.BindTokens(newInvEnv(t).svc); err != nil {
		t.Fatal(err)
	}
	ev := w.posted(w.thread, w.lotse)
	w.mustPost(ev)
	if len(w.repo.runs) != 1 {
		t.Fatalf("runs %+v", w.repo.runs)
	}
	w.msgFacts.m[ev.MessageID] = messaging.TriggerMessage{
		ID: ev.MessageID, ThreadID: w.thread, ChannelID: w.channel, Origin: "human", Kind: "text", MentionedIDs: []ids.ID{w.lotse},
		Author: messaging.PrincipalRef{ID: w.alex, Kind: "user", Handle: "alex", DisplayName: "Alex"}, TriggerState: "received",
	}
	return &deliveryWorld{world: w, run: w.repo.runs[0], message: ev.MessageID}
}

// current returns the run row now.
func (w *deliveryWorld) current() app.RunRow {
	w.t.Helper()
	r, _, _ := w.repo.Run(context.Background(), w.run.ID)
	return r
}

// input is the delivery of typ for the run of w.
func (w *deliveryWorld) input(typ string) runs.DeliveryInput {
	return runs.DeliveryInput{AgentID: w.lotse, EventID: ids.New(), Seq: 7, Type: typ, ResourceKind: "run", ResourceID: w.run.ID}
}

// prepare runs Prepare and fails the test on an error.
func (w *deliveryWorld) prepare(in runs.DeliveryInput) runs.DeliveryOutcome {
	w.t.Helper()
	out, err := w.svc.Prepare(context.Background(), in)
	if err != nil {
		w.t.Fatal(err)
	}
	if out.Send != (out.InvocationToken != "") {
		w.t.Fatalf("outcome %+v: the token is set exactly when Send", out.Send)
	}
	return out
}

// deny makes agent.invoke refuse with reason.
func (w *deliveryWorld) deny(reason string) {
	w.authz.decide = func(authorize.Action, bool, authorize.Resource) decision { return decision{reason: reason} }
}

// claims decodes the payload of a token.
func claimsOf(t *testing.T, tok string) map[string]any {
	t.Helper()
	_, p, _ := segments(t, tok)
	return p
}

// setState moves the run to state through Transition.
func (w *deliveryWorld) setState(to domain.State) {
	w.t.Helper()
	cur := w.current()
	if err := uow.Do(uow{}, context.Background(), func(ctx context.Context) error {
		_, err := w.svc.Transition(ctx, app.TransitionCmd{RunID: cur.ID, From: cur.State, Version: cur.Version, To: to, SetBy: domain.SetterAdapter})
		return err
	}); err != nil {
		w.t.Fatal(err)
	}
}

// triggerStates returns the trigger states written for message.
func (w *deliveryWorld) triggerStates(message ids.ID) []string {
	var out []string
	for _, tr := range w.triggers() {
		if tr.MessageID == message {
			out = append(out, tr.State+":"+tr.Reason)
		}
	}
	return out
}

// TestPrepare_AllowCarriesFreshToken: an allowed delivery of agent.invocation for a queued run
// signs a token with the claims of design note 7, records its jti, moves trigger_state from
// received to queued once, and every further delivery records a new jti (R-SY-07, AG-09).
func TestPrepare_AllowCarriesFreshToken(t *testing.T) {
	t.Parallel()
	w := newDeliveryWorld(t)
	out := w.prepare(w.input(domain.EventAgentInvocation))
	if !out.Send {
		t.Fatal("not sent")
	}
	c := claimsOf(t, out.InvocationToken)
	run := w.current()
	if c["jti"] != run.InvocationJTI.String() || c["sub"] != w.alex.String() || c["aud"] != w.lotse.String() || c["run_id"] != w.run.ID.String() ||
		c["message_id"] != w.message.String() || c["thread_id"] != w.thread.String() || c["channel_id"] != w.channel.String() ||
		c["space_id"] != w.space.String() || c["level"] != "trigger" || c["billing_mode"] != "organization" || c["tool_class"] != "chat" ||
		c["trigger_kind"] != "message" || c["permission_version"] != float64(9) {
		t.Fatalf("claims %v, run %+v", c, run)
	}
	if run.Version != w.run.Version+1 || !run.RenewedFromJTI.IsZero() {
		t.Fatalf("run %+v", run)
	}
	if got := w.triggerStates(w.message); strings.Join(got, " ") != "received: queued:" {
		t.Fatalf("trigger states %v", got)
	}
	p := w.authz.principals[len(w.authz.principals)-1].(actor.Actor)
	if !reflect.DeepEqual(p, actor.Recheck(w.alex)) || w.authz.pins[len(w.authz.pins)-1] != 9 {
		t.Fatalf("principal %+v pin %v", p, w.authz.pins)
	}
	again := w.prepare(w.input(domain.EventAgentInvocation))
	c2 := claimsOf(t, again.InvocationToken)
	if c2["jti"] == c["jti"] || c2["jti"] != w.current().InvocationJTI.String() {
		t.Fatalf("second jti %v, first %v, stored %s", c2["jti"], c["jti"], w.current().InvocationJTI)
	}
	if got := w.triggerStates(w.message); len(got) != 2 {
		t.Fatalf("a second delivery changed the trigger state: %v", got)
	}
	if es, _ := w.audit.list(); len(es) != 0 {
		t.Fatalf("an allowed delivery audited %+v", es)
	}
}

// TestPrepare_DenialFailsQueuedRun (AG-07, AZ-M51 at the unit level): nothing is sent, the run
// fails with the mapped reason, the message follows, one audit entry with the attributes of design
// note 8 and the re-check principal as actor, one count.
func TestPrepare_DenialFailsQueuedRun(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ authz, want string }{
		{"no_grant", "access_revoked"}, {"principal_inactive", "user_disabled"}, {"origin_not_human", "access_revoked"},
		{"billing_mode_personal_not_holder", "billing_mode_personal_not_holder"}, {"tool_class_owner_only", "tool_class_owner_only"},
	} {
		w := newDeliveryWorld(t)
		w.deny(c.authz)
		out := w.prepare(w.input(domain.EventAgentInvocation))
		if out.Send {
			t.Fatalf("%s: sent", c.authz)
		}
		run := w.current()
		if run.State != domain.StateFailed || run.StateReason != c.want {
			t.Fatalf("%s: run %+v", c.authz, run)
		}
		if got := w.triggerStates(w.message); strings.Join(got, " ") != "received: failed:"+c.want {
			t.Fatalf("%s: trigger states %v", c.authz, got)
		}
		es, as := w.audit.list()
		if len(es) != 1 {
			t.Fatalf("%s: audit %+v", c.authz, es)
		}
		e := es[0]
		want := auditing.Attrs{
			"run_id": auditing.ID(w.run.ID), "agent_id": auditing.ID(w.lotse), "stage": auditing.Code("delivery"),
			"event_type": auditing.Code("agent.invocation"), "trigger_kind": auditing.Code("message"),
		}
		if e.Action != "agent.invoke" || e.Decision != auditing.DecisionDeny || string(e.Reason) != c.authz || e.ResourceKind != "thread" ||
			e.ResourceID != w.thread || len(e.Attrs) != len(want) || !reflect.DeepEqual(as[0], actor.Recheck(w.alex)) {
			t.Fatalf("%s: entry %+v actor %+v", c.authz, e, as[0])
		}
		for k, v := range want {
			if !reflect.DeepEqual(e.Attrs[k], v) {
				t.Fatalf("%s: attr %s = %v", c.authz, k, e.Attrs[k])
			}
		}
		if len(w.telemetry.denied) != 1 || w.telemetry.denied[0] != "lotse:"+c.want || w.telemetry.decisions[0] != "deny" {
			t.Fatalf("%s: telemetry %+v", c.authz, w.telemetry)
		}
	}
	w := newDeliveryWorld(t)
	w.agents[w.lotse] = agents.RunFacts{ID: w.lotse, Handle: "lotse", Status: "disabled", BillingMode: "organization", ToolClass: "chat"}
	w.deny("agent_not_reachable")
	w.prepare(w.input(domain.EventAgentInvocation))
	if r := w.current(); r.StateReason != "agent_disabled" {
		t.Fatalf("run %+v", r)
	}
}

// TestPrepare_StartedRunNotRecorded: agent.invocation of a running or waiting run is sent with a
// fresh token, invocation_jti stays; a denial sends nothing and leaves the run unchanged.
func TestPrepare_StartedRunNotRecorded(t *testing.T) {
	t.Parallel()
	for _, st := range []domain.State{domain.StateRunning, domain.StateWaitingInput} {
		w := newDeliveryWorld(t)
		w.setState(domain.StateRunning)
		if st == domain.StateWaitingInput {
			w.setState(domain.StateWaitingInput)
		}
		before := w.current()
		out := w.prepare(w.input(domain.EventAgentInvocation))
		if !out.Send || !reflect.DeepEqual(w.current(), before) {
			t.Fatalf("%s: send %v, run %+v -> %+v", st, out.Send, before, w.current())
		}
		w.deny("no_grant")
		if out := w.prepare(w.input(domain.EventAgentInvocation)); out.Send || !reflect.DeepEqual(w.current(), before) {
			t.Fatalf("%s: denial changed the run or sent", st)
		}
		if es, _ := w.audit.list(); len(es) != 1 {
			t.Fatalf("%s: audit %+v", st, es)
		}
	}
}

// TestPrepare_FollowupJtiRules: agent.followup of a running run keeps invocation_jti, of a waiting
// run replaces it; a denial fails the follow-up message and leaves the run.
func TestPrepare_FollowupJtiRules(t *testing.T) {
	t.Parallel()
	w := newDeliveryWorld(t)
	w.setState(domain.StateRunning)
	follow := ids.New()
	w.msgFacts.m[follow] = messaging.TriggerMessage{ID: follow, ThreadID: w.thread, Origin: "human", Kind: "text",
		Author: messaging.PrincipalRef{ID: w.alex, Kind: "user", Handle: "alex", DisplayName: "Alex"}}
	p, err := domain.FollowupPayload(w.run.ID, w.thread, follow)
	if err != nil {
		t.Fatal(err)
	}
	in := w.input(domain.EventAgentFollowup)
	in.Payload = p
	before := w.current()
	out := w.prepare(in)
	c := claimsOf(t, out.InvocationToken)
	if !out.Send || c["trigger_kind"] != "followup" || c["message_id"] != follow.String() || w.current().InvocationJTI != before.InvocationJTI {
		t.Fatalf("running: claims %v run %+v", c, w.current())
	}
	w.setState(domain.StateWaitingInput)
	out = w.prepare(in)
	if run := w.current(); !out.Send || run.InvocationJTI.String() != claimsOf(t, out.InvocationToken)["jti"] || !run.RenewedFromJTI.IsZero() {
		t.Fatalf("waiting: run %+v", run)
	}
	cur := w.current()
	w.deny("level_insufficient")
	if out := w.prepare(in); out.Send || !reflect.DeepEqual(w.current(), cur) {
		t.Fatalf("denied follow-up: sent %v or changed the run", out.Send)
	}
	if got := w.triggerStates(follow); strings.Join(got, " ") != "failed:access_revoked" {
		t.Fatalf("follow-up trigger states %v", got)
	}
	if es, _ := w.audit.list(); len(es) != 1 || !reflect.DeepEqual(es[0].Attrs["trigger_kind"], auditing.Code("followup")) {
		t.Fatalf("audit %+v", es)
	}
}

// TestPrepare_TerminalRunSkipped: an ended run is not checked and nothing is sent or audited.
func TestPrepare_TerminalRunSkipped(t *testing.T) {
	t.Parallel()
	w := newDeliveryWorld(t)
	w.setState(domain.StateRunning)
	w.setState(domain.StateCompleted)
	calls := w.j.count("authz:agent.invoke:filter")
	if out := w.prepare(w.input(domain.EventAgentInvocation)); out.Send || w.j.count("authz:agent.invoke:filter") != calls {
		t.Fatal("an ended run was checked or sent")
	}
	if es, _ := w.audit.list(); len(es) != 0 {
		t.Fatalf("audit %+v", es)
	}
}

// TestPrepare_UnregisteredResolverFailsClosed: every event type that bears an invocation has its
// resolver since M2-15 (interaction.created) and M2-16 (command.invoked takes its own path); a
// type that bears no invocation is an internal error, so are a run of another agent, an unknown
// run and a command.invoked whose resource is not its command (M2-16).
func TestPrepare_UnregisteredResolverFailsClosed(t *testing.T) {
	t.Parallel()
	w := newDeliveryWorld(t)
	for name, in := range map[string]runs.DeliveryInput{
		"type":    w.input("run.cancel_requested"),
		"foreign": {AgentID: w.texter, Type: domain.EventAgentInvocation, ResourceKind: "run", ResourceID: w.run.ID},
		"unknown": {AgentID: w.lotse, Type: domain.EventAgentInvocation, ResourceKind: "run", ResourceID: ids.New()},
		"invoked": w.input(domain.EventCommandInvoked),
	} {
		if _, err := w.svc.Prepare(context.Background(), in); errs.KindOf(err) != errs.KindInternal {
			t.Fatalf("%s: err %v", name, err)
		}
	}
}

// TestPrepare_QueuedTooOld: a run queued for run.queued_max_age fails with agent_unavailable at the
// next Prepare without a check, a token or an audit entry (AG-27 without the job).
func TestPrepare_QueuedTooOld(t *testing.T) {
	t.Parallel()
	w := newDeliveryWorld(t)
	w.clk.Advance(queuedMaxAge)
	calls := w.j.count("authz:agent.invoke:filter")
	if out := w.prepare(w.input(domain.EventAgentInvocation)); out.Send {
		t.Fatal("sent")
	}
	if r := w.current(); r.State != domain.StateFailed || r.StateReason != "agent_unavailable" || w.j.count("authz:agent.invoke:filter") != calls {
		t.Fatalf("run %+v", r)
	}
	if got := w.triggerStates(w.message); strings.Join(got, " ") != "received: failed:agent_unavailable" {
		t.Fatalf("trigger states %v", got)
	}
	if es, _ := w.audit.list(); len(es) != 0 || len(w.telemetry.denied) != 0 {
		t.Fatal("an expiry was audited or counted")
	}
}

// TestQueuedExpiry_Job: Open schedules the job at queued_at + run.queued_max_age; the job fails a
// due run, schedules itself again for a run not yet due and leaves a started run alone.
func TestQueuedExpiry_Job(t *testing.T) {
	t.Parallel()
	w := newDeliveryWorld(t)
	var jobs []expiryAt
	for _, c := range w.j.list() {
		if c.what == "expiry" {
			jobs = append(jobs, c.arg.(expiryAt))
		}
	}
	if len(jobs) != 1 || jobs[0].run != w.run.ID || !jobs[0].at.Equal(w.run.QueuedAt.Add(queuedMaxAge)) {
		t.Fatalf("jobs %+v", jobs)
	}
	if err := w.svc.ExpireQueued(context.Background(), w.run.ID); err != nil || w.current().State != domain.StateQueued || w.j.count("expiry") != 2 {
		t.Fatalf("not due: err %v run %+v jobs %d", err, w.current(), w.j.count("expiry"))
	}
	w.clk.Advance(queuedMaxAge)
	if err := w.svc.ExpireQueued(context.Background(), w.run.ID); err != nil {
		t.Fatal(err)
	}
	if r := w.current(); r.State != domain.StateFailed || r.StateReason != "agent_unavailable" {
		t.Fatalf("run %+v", r)
	}
	if err := w.svc.ExpireQueued(context.Background(), w.run.ID); err != nil || w.j.count("expiry") != 2 {
		t.Fatalf("an ended run: err %v jobs %d", err, w.j.count("expiry"))
	}
	if err := w.svc.ExpireQueued(context.Background(), ids.New()); err != nil {
		t.Fatalf("unknown run: %v", err)
	}
}

// TestPrepare_NewThreadAndOrigin: the auto thread of the trigger sets NewThread and the origin of
// the message goes to the guard (SR4 at delivery).
func TestPrepare_NewThreadAndOrigin(t *testing.T) {
	t.Parallel()
	w := newDeliveryWorld(t)
	f := w.msgFacts.m[w.message]
	f.ThreadID, f.Origin = w.main, "agent"
	w.msgFacts.m[w.message] = f
	w.prepare(w.input(domain.EventAgentInvocation))
	var r authorize.Resource
	for _, c := range w.j.list() {
		if c.what == "authz:agent.invoke:filter" {
			r = c.arg.(authorize.Resource)
		}
	}
	if r.Invoke == nil || !r.Invoke.NewThread || r.Invoke.TriggerOrigin != "agent" || !r.Invoke.MentionsAgent ||
		r.Invoke.ActiveRunAgentID != w.lotse || r.ID != w.thread {
		t.Fatalf("resource %+v invoke %+v", r, r.Invoke)
	}
	// A message that is not of a person carries no trigger state: the delivery does not set one.
	if got := w.triggerStates(w.message); strings.Join(got, " ") != "received:" {
		t.Fatalf("trigger states %v", got)
	}
}
