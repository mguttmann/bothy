// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// messageUpdates counts message.updated of message after seq.
func (w *world) messageUpdates(message string, seq int64) int64 {
	return w.count(`SELECT count(*) FROM event_log WHERE seq > $1 AND type = 'message.updated' AND resource_id = $2`, seq, message)
}

// TestAG13_Steer: with steer a follow-up of Sam to Alex's running run is one queued row with
// received, raises the run version, and writes one agent.followup to Lotse that names the message
// only by id; its delivery carries a fresh token for Sam and leaves invocation_jti; the adapter's
// delivered (after a rebase) sets followup_delivered with one message.updated, absorbed changes no
// trigger_state, and finishRun completed leaves it absorbed without a follow-on run (AG-13).
func TestAG13_Steer(t *testing.T) {
	t.Parallel()
	w := followupWorld(t)
	w.followups(w.lotse, "steer", false)
	r, trig, _ := w.started()
	th := ids.MustParse(r.Thread)
	v0, jti0 := w.runVersion(r.ID), w.lifecycle(r.ID).Invocation
	m := w.say("sam", th, "Also check the logs")
	w.wantFollowup(m, r.ID, "queued", "received")
	if v := w.runVersion(r.ID); v != v0+1 {
		t.Fatalf("version %d, want %d", v, v0+1)
	}
	evs := w.followupEvents(r.ID)
	if len(evs) != 1 || len(evs[0]) != 3 || evs[0]["run_id"] != r.ID || evs[0]["thread_id"] != r.Thread || evs[0]["message_id"] != m {
		t.Fatalf("agent.followup %v", evs)
	}
	if aud := w.text(`SELECT audience_principal_id::text || '/' || coalesce(resource_version::text, 'none') || '/' || resource_kind
		FROM event_log WHERE type = 'agent.followup' AND resource_id = $1`, r.ID); aud != w.lotse.String()+"/none/run" {
		t.Fatalf("event row %s", aud)
	}
	if strings.Contains(w.text(`SELECT payload::text FROM event_log WHERE type = 'agent.followup'`), "check the logs") {
		t.Fatal("content in the payload")
	}
	out := w.deliverFollowup(r.ID, m)
	c := claimsOf(t, out.InvocationToken)
	if !out.Send || c["trigger_kind"] != "followup" || c["sub"] != w.sam.String() || c["message_id"] != m || c["run_id"] != r.ID {
		t.Fatalf("delivery %v claims %v", out.Send, c)
	}
	if got := w.lifecycle(r.ID).Invocation; got != jti0 {
		t.Fatal("the delivery replaced invocation_jti of a running run")
	}
	stale := `{"base_version":` + "1" + `,"ops":[{"op":"replace","path":"/followups/` + m + `/state","value":"delivered"}]}`
	conflict := w.want(w.do("lotse", http.MethodPatch, "/api/v1/runs/"+r.ID, stale), http.StatusConflict, "base_version_conflict")
	if docFollowups(conflict["current"].(map[string]any))[m] != "queued" {
		t.Fatalf("current %v", conflict["current"])
	}
	seq := w.maxSeq()
	w.wantOK(w.patchFollowup("lotse", r.ID, m, "delivered"))
	w.wantFollowup(m, r.ID, "delivered", "followup_delivered")
	if n := w.messageUpdates(m, seq); n != 1 {
		t.Fatalf("%d message.updated", n)
	}
	seq = w.maxSeq()
	w.wantOK(w.patchFollowup("lotse", r.ID, m, "absorbed"))
	w.wantFollowup(m, r.ID, "absorbed", "followup_delivered")
	if n := w.messageUpdates(m, seq); n != 0 {
		t.Fatalf("%d message.updated after absorbed", n)
	}
	w.wantOK(w.finish("lotse", r.ID, `"state":"completed"`))
	w.wantFollowup(m, r.ID, "absorbed", "followup_delivered")
	if st, _, _ := w.triggerOf(trig); st != "done" {
		t.Fatalf("trigger %s", st)
	}
	if n := w.count(`SELECT count(*) FROM runs WHERE thread_id = $1`, r.Thread); n != 1 {
		t.Fatalf("%d runs", n)
	}
	w.validateFollowupPayloads()
	w.noTokenAnywhere(out.InvocationToken)
}

// TestAG13_Queue: with queue two follow-ups (Sam, then Alex) wait as queued rows without an
// event; finishRun completed defers both, the earlier one by thread_seq triggers a new queued run
// for its author with an anchor, received, agent.invocation and the expiry, and stays deferred on
// the ended run; the later one becomes a queued follow-up of the new run (AG-13).
func TestAG13_Queue(t *testing.T) {
	t.Parallel()
	w := followupWorld(t)
	w.followups(w.lotse, "queue", false)
	r, _, _ := w.started()
	th := ids.MustParse(r.Thread)
	a := w.say("sam", th, "First addition")
	b := w.say("alex", th, "Second addition")
	w.wantFollowup(a, r.ID, "queued", "received")
	w.wantFollowup(b, r.ID, "queued", "received")
	if n := len(w.followupEvents(r.ID)); n != 0 {
		t.Fatalf("%d agent.followup with queue", n)
	}
	anchors := w.count(`SELECT count(*) FROM messages WHERE kind = 'run' AND thread_id = $1`, r.Thread)
	w.wantOK(w.finish("lotse", r.ID, `"state":"completed"`))
	next, ok := w.runOf(a)
	if !ok || next.State != "queued" || next.TriggeredBy != w.sam.String() || next.Agent != w.lotse.String() || next.Thread != r.Thread {
		t.Fatalf("follow-on run %+v %v", next, ok)
	}
	w.wantFollowup(a, r.ID, "deferred", "received")
	w.wantFollowup(b, next.ID, "queued", "received")
	if n := w.count(`SELECT count(*) FROM messages WHERE kind = 'run' AND thread_id = $1`, r.Thread); n != anchors+1 {
		t.Fatalf("anchors %d -> %d", anchors, n)
	}
	if inv := w.eventsOf("agent.invocation", next.ID); len(inv) != 1 {
		t.Fatalf("agent.invocation %v", inv)
	}
	if got := docFollowups(w.getRun("alex", r.ID)); len(got) != 1 || got[a] != "deferred" {
		t.Fatalf("ended run followups %v", got)
	}
	if got := docFollowups(w.getRun("alex", next.ID)); len(got) != 1 || got[b] != "queued" {
		t.Fatalf("new run followups %v", got)
	}
}

// TestAG13_QueueWithoutCapability: supports_followups = false with a steer report behaves as
// queue: no event, and the follow-up waits for the end of the run.
func TestAG13_QueueWithoutCapability(t *testing.T) {
	t.Parallel()
	w := followupWorld(t)
	w.followups(w.lotse, "steer", true)
	r, _, _ := w.started()
	m := w.say("sam", ids.MustParse(r.Thread), "Addition")
	w.wantFollowup(m, r.ID, "queued", "received")
	if n := len(w.followupEvents(r.ID)); n != 0 {
		t.Fatalf("%d agent.followup without the capability", n)
	}
	w.wantOK(w.finish("lotse", r.ID, `"state":"completed"`))
	if _, ok := w.runOf(m); !ok {
		t.Fatal("no follow-on run")
	}
}

// TestAG13_DeliveredNotAbsorbedIsDeferred: a follow-up acknowledged delivered becomes deferred
// when the run fails with runtime_error and triggers the follow-on run; an absorbed one stays.
func TestAG13_DeliveredNotAbsorbedIsDeferred(t *testing.T) {
	t.Parallel()
	w := followupWorld(t)
	w.followups(w.lotse, "steer", false)
	r, _, _ := w.started()
	th := ids.MustParse(r.Thread)
	a := w.say("sam", th, "Absorbed one")
	b := w.say("sam", th, "Delivered one")
	w.wantOK(w.patchFollowup("lotse", r.ID, a, "absorbed"))
	w.wantOK(w.patchFollowup("lotse", r.ID, b, "delivered"))
	w.wantOK(w.finish("lotse", r.ID, `"state":"failed","reason":"runtime_error"`))
	w.wantFollowup(a, r.ID, "absorbed", "followup_delivered")
	w.wantFollowup(b, r.ID, "deferred", "received")
	if next, ok := w.runOf(b); !ok || next.State != "queued" {
		t.Fatalf("follow-on run %+v %v", next, ok)
	}
}

// TestAG13_FollowOnRunMovesRestWithSteer: three follow-ups with steer, the run completes before
// any acknowledgement: the first triggers the new run; the other two move as queued with
// received (also the one that showed followup_delivered), and their two agent.followup for the new
// run follow its agent.invocation in thread_seq order.
func TestAG13_FollowOnRunMovesRestWithSteer(t *testing.T) {
	t.Parallel()
	w := followupWorld(t)
	w.followups(w.lotse, "steer", false)
	r, _, _ := w.started()
	th := ids.MustParse(r.Thread)
	a, b, c := w.say("sam", th, "One"), w.say("alex", th, "Two"), w.say("sam", th, "Three")
	w.wantOK(w.patchFollowup("lotse", r.ID, b, "delivered"))
	seq := w.maxSeq()
	w.wantOK(w.finish("lotse", r.ID, `"state":"completed"`))
	next, ok := w.runOf(a)
	if !ok {
		t.Fatal("no follow-on run")
	}
	w.wantFollowup(a, r.ID, "deferred", "received")
	w.wantFollowup(b, next.ID, "queued", "received")
	w.wantFollowup(c, next.ID, "queued", "received")
	var order []string
	for _, e := range w.eventsAfter(seq) {
		if e.Resource == next.ID && (e.Type == "agent.invocation" || e.Type == "agent.followup") {
			order = append(order, e.Type)
		}
	}
	if !slices.Equal(order, []string{"agent.invocation", "agent.followup", "agent.followup"}) {
		t.Fatalf("event order %v", order)
	}
	evs := w.followupEvents(next.ID)
	if len(evs) != 2 || evs[0]["message_id"] != b || evs[1]["message_id"] != c {
		t.Fatalf("agent.followup of the new run %v", evs)
	}
}

// TestAG13_StopDiscardsOpenFollowups: a stop by a person discards the open follow-ups with
// trigger_state cancelled / user_cancelled and starts nothing; the same for a queued run that
// cancelRun ends at once.
func TestAG13_StopDiscardsOpenFollowups(t *testing.T) {
	t.Parallel()
	w := followupWorld(t)
	w.followups(w.lotse, "queue", false)
	r, _, _ := w.started()
	th := ids.MustParse(r.Thread)
	a, b := w.say("sam", th, "One"), w.say("alex", th, "Two")
	invocations := w.count(`SELECT count(*) FROM event_log WHERE type = 'agent.invocation'`)
	w.want(w.cancel("alex", r.ID), http.StatusAccepted, "")
	w.wantOK(w.finish("lotse", r.ID, `"state":"cancelled","reason":"user_cancelled"`))
	for _, m := range []string{a, b} {
		w.wantFollowup(m, r.ID, "discarded", "cancelled:user_cancelled")
	}
	if n := w.count(`SELECT count(*) FROM runs WHERE thread_id = $1`, r.Thread); n != 1 {
		t.Fatalf("%d runs", n)
	}
	if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'agent.invocation'`); n != invocations {
		t.Fatal("a new invocation")
	}

	q, _ := w.openRun()
	m := w.say("sam", ids.MustParse(q.Thread), "Queued addition")
	w.wantFollowup(m, q.ID, "queued", "received")
	w.want(w.cancel("alex", q.ID), http.StatusAccepted, "")
	w.wantFollowup(m, q.ID, "discarded", "cancelled:user_cancelled")
	if n := w.count(`SELECT count(*) FROM runs WHERE thread_id = $1`, q.Thread); n != 1 {
		t.Fatalf("%d runs after stopping the queued run", n)
	}
}

// TestAG13_CancelForOtherReasonDefers: Alex loses trigger on Support while Lotse runs her run and
// Sam has a queued follow-up; the adapter's cancelled / access_revoked makes Sam's follow-up the
// trigger of a new run of Sam; the same after runs.cancel_timeout (literal 09 7).
func TestAG13_CancelForOtherReasonDefers(t *testing.T) {
	t.Parallel()
	for _, viaJob := range []bool{false, true} {
		w := followupWorld(t)
		w.followups(w.lotse, "queue", false)
		r, _, _ := w.started()
		m := w.say("sam", ids.MustParse(r.Thread), "Addition")
		w.wantOK(w.do("owner", http.MethodPost, "/api/v1/grants/"+w.g1.String()+"/revoke", `{"reason":"Moved team"}`, key()...))
		if got := w.lifecycle(r.ID); got.State != "running" {
			t.Fatalf("run %+v", got)
		}
		if viaJob {
			w.clk.Advance(31 * time.Second)
			must(t, w.runs.CancelTimeout(context.Background(), ids.MustParse(r.ID)))
		} else {
			w.wantOK(w.finish("lotse", r.ID, `"state":"cancelled","reason":"access_revoked"`))
		}
		next, ok := w.runOf(m)
		if !ok || next.TriggeredBy != w.sam.String() || next.State != "queued" {
			t.Fatalf("job %v: follow-on run %+v %v", viaJob, next, ok)
		}
		w.wantFollowup(m, r.ID, "deferred", "received")
	}
}

// TestAG13_AnswerToWaitingInput: a follow-up to a waiting run gets agent.followup in steer and in
// queue; its delivery records the token's jti, which resumes the run, while the start token is
// superseded; with queue a follow-up written while the run was running gets no event and is
// deferred at the end.
func TestAG13_AnswerToWaitingInput(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"steer", "queue"} {
		w := followupWorld(t)
		w.followups(w.lotse, mode, false)
		r, _, s := w.started()
		th := ids.MustParse(r.Thread)
		early := w.say("sam", th, "Before the question")
		w.wantOK(w.await("lotse", r.ID, "Which file?"))
		answer := w.say("alex", th, "The export file")
		evs := w.followupEvents(r.ID)
		if last := evs[len(evs)-1]; last["message_id"] != answer {
			t.Fatalf("%s: events %v", mode, evs)
		}
		if mode == "queue" && len(evs) != 1 {
			t.Fatalf("queue: %d events, the running follow-up got one", len(evs))
		}
		out := w.deliverFollowup(r.ID, answer)
		if !out.Send || w.lifecycle(r.ID).Invocation != jtiOf(t, out.InvocationToken) {
			t.Fatalf("%s: delivery did not record its jti", mode)
		}
		w.wantReason(w.resume("lotse", r.ID, s), http.StatusForbidden, "invocation_token_invalid", "superseded")
		if w.wantOK(w.resume("lotse", r.ID, out.InvocationToken))["state"] != "running" {
			t.Fatalf("%s: resume did not run", mode)
		}
		if mode == "queue" {
			w.wantOK(w.finish("lotse", r.ID, `"state":"completed"`))
			w.wantFollowup(early, r.ID, "deferred", "received")
		}
	}
}
