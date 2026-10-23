// SPDX-License-Identifier: Apache-2.0

package invocationhold_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents/adapters/agentstest"
	agentsdomain "github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/clamd/clamdtest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// followups sets Lotse's reported follow-up capability (steer or queue, M2-11) as its heartbeat
// stores it; the owner's switch supports_followups stays unset (permitted).
func (w *world) followups(mode string) {
	w.t.Helper()
	agentstest.ReportCapabilities(w.t, w.db, ids.MustParse(w.lotse.PrincipalID), agentsdomain.Capabilities{Followups: mode})
}

// startedRun opens a run of Lotse through a mention without files, delivers its agent.invocation
// through pollAgentEvents and starts it with the delivered token (startRun); it returns the run.
func (w *world) startedRun() runRow {
	w.t.Helper()
	m := w.mention()
	r := w.runOf(m.ID)
	got := delivered(w.poll(), r.ID)
	if len(got) != 1 || token(got[0]) == "" {
		w.t.Fatalf("the invocation of run %s was not delivered: %v", r.ID, got)
	}
	harness.MustDo(w.t, w.lotse, harness.Request{Method: http.MethodPost, Path: "/runs/" + r.ID + "/start", Body: map[string]any{
		"invocation_token": token(got[0]), "billing_mode_used": "organization", "tool_class_used": "chat",
	}}, http.StatusOK)
	if s := w.run(r.ID).State; s != "running" {
		w.t.Fatalf("run %s %s after startRun", r.ID, s)
	}
	return w.run(r.ID)
}

// followupRow reads run and state of the follow-up row of a message; ok is false without one.
func (w *world) followupRow(messageID string) (run, state string, ok bool) {
	w.t.Helper()
	err := w.db.QueryRow(w.t.Context(), `SELECT run_id::text, state FROM run_followups WHERE message_id = $1`, messageID).Scan(&run, &state)
	return run, state, err == nil
}

// say posts a message as Alex into thread with the attachments and requires 201.
func (w *world) say(thread, text string, fileIDs ...string) message {
	w.t.Helper()
	r := w.send(w.alex, thread, text, fileIDs...)
	if r.Status != http.StatusCreated {
		w.t.Fatalf("createMessage: %d %s", r.Status, r.Body)
	}
	var m message
	r.JSON(w.t, &m)
	return m
}

// TestHold_FollowOnRunHeld (M2-11, design note 11 d): a follow-on run whose first deferred
// message has a pending attachment is held like an acceptance and released once, when the
// attachment becomes clean; the hold job afterwards writes nothing.
func TestHold_FollowOnRunHeld(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	w.followups("queue")
	ended := w.startedRun()
	f := w.file("followup.txt")
	fu := w.say(ended.Thread, "One more file for you", f)
	if run, state, ok := w.followupRow(fu.ID); !ok || run != ended.ID || state != "queued" {
		t.Fatalf("follow-up %s %s %v", run, state, ok)
	}
	finishedAt := w.clk.Now()
	harness.MustDo(t, w.lotse, harness.Request{Method: http.MethodPost, Path: "/runs/" + ended.ID + "/finish",
		Body: map[string]any{"state": "completed"}}, http.StatusOK)
	if run, state, ok := w.followupRow(fu.ID); !ok || run != ended.ID || state != "deferred" {
		t.Fatalf("follow-up after the end %s %s %v", run, state, ok)
	}
	next := w.runOf(fu.ID)
	if next.State != "queued" || next.Thread != ended.Thread || !next.HeldUntil.Equal(finishedAt.Add(holdMax)) {
		t.Fatalf("follow-on run %+v, want queued and held until %v", next, finishedAt.Add(holdMax))
	}
	if jobs := w.holdJobs(next.ID); len(jobs) != 1 || !jobs[0].Equal(next.HeldUntil) {
		t.Fatalf("hold jobs %v", jobs)
	}
	if n := len(w.invocations(next.ID)); n != 0 {
		t.Fatalf("%d agent.invocation of the held follow-on run", n)
	}
	if got := delivered(w.poll(), next.ID); len(got) != 0 {
		t.Fatalf("delivered while held: %v", got)
	}
	w.clamd.start(clamdtest.Script{})
	w.mustScan(f)
	w.wantReleased(next.ID)
	if got := delivered(w.poll(), next.ID); len(got) != 1 || token(got[0]) == "" {
		t.Fatalf("delivered %v", got)
	}
	w.clk.Advance(holdMax)
	before := w.maxSeq()
	w.holdTimeout(next.ID)
	if w.maxSeq() != before || len(w.invocations(next.ID)) != 1 {
		t.Fatal("the hold job after the release wrote")
	}
}

// TestFollowup_PendingAttachmentNotHeld (M2-11, design note 11 e): a follow-up with a pending
// attachment to a running run gets its agent.followup at once (steer); the run is not held and no
// hold job is scheduled.
func TestFollowup_PendingAttachmentNotHeld(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	w.followups("steer")
	r := w.startedRun()
	f := w.file("steer.txt")
	fu := w.say(r.Thread, "Here is the file you asked for", f)
	if up, scan := w.fileState(f); up != "pending" || scan != "pending" {
		t.Fatalf("file %s %s", up, scan)
	}
	if run, _, ok := w.followupRow(fu.ID); !ok || run != r.ID {
		t.Fatalf("no follow-up row of run %s", r.ID)
	}
	n := w.count(`SELECT count(*) FROM event_log WHERE type = 'agent.followup' AND resource_id = $1 AND payload->>'message_id' = $2`, r.ID, fu.ID)
	if n != 1 {
		t.Fatalf("%d agent.followup of the follow-up, want 1 at once", n)
	}
	if got := w.run(r.ID); !got.HeldUntil.IsZero() || len(w.holdJobs(r.ID)) != 0 {
		t.Fatalf("run %+v held for a follow-up", got)
	}
	var seen bool
	for _, e := range w.poll() {
		if e.Type != "agent.followup" || e.Resource.ID != r.ID {
			continue
		}
		var p struct {
			MessageID string `json:"message_id"`
		}
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			t.Fatal(err)
		}
		seen = seen || (p.MessageID == fu.ID && token(e) != "")
	}
	if !seen {
		t.Fatal("Lotse did not receive the agent.followup with a token")
	}
}
