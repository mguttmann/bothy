// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// retry sends retryRun of run as name.
func (w *world) retry(name, run string) map[string]any {
	w.t.Helper()
	return w.want(w.do(name, http.MethodPost, "/api/v1/runs/"+run+"/retry", "", key()...), http.StatusAccepted, "")
}

// TestRetry_FailedRunStartsAgain (09 5.7, addendum of 2026-10-30): a failed run of the newest
// trigger is started again for the same message, the trigger state follows the new run, the run
// gets its anchor and agent.invocation, and a second retry answers the same run.
func TestRetry_FailedRunStartsAgain(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, msg := w.openRun()
	w.mustTransition(r.ID, domain.StateQueued, domain.StateFailed, 1, "agent_unavailable", domain.SetterServer)
	before := w.maxSeq()
	body := w.retry("alex", r.ID)
	rows := w.runsWhere(`triggering_message_id = $1`, msg)
	if len(rows) != 2 || rows[1].ID != body["id"] || rows[1].State != "queued" || rows[1].Thread != r.Thread ||
		rows[1].Agent != w.lotse.String() || rows[1].TriggeredBy != w.alex.String() {
		t.Fatalf("runs %+v, answer %v", rows, body)
	}
	if st, reason, _ := w.triggerOf(msg); st != "received" || reason != "" {
		t.Fatalf("trigger %s %s", st, reason)
	}
	got := types(w.eventsAfter(before))
	if !slices.Contains(got, "run.state_changed") || got[len(got)-1] != "agent.invocation" {
		t.Fatalf("events %v", got)
	}
	again := w.retry("alex", r.ID)
	if again["id"] != body["id"] || len(w.runsWhere(`triggering_message_id = $1`, msg)) != 2 {
		t.Fatalf("second retry %v", again)
	}
}

// TestRetry_Refusals: a completed run and a run of an older trigger are 409, a person who cannot
// see the run gets 404, and a person without trigger 403 (the rights of triggering).
func TestRetry_Refusals(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	done := w.runIn(domain.StateCompleted)
	w.want(w.do("alex", http.MethodPost, "/api/v1/runs/"+done.ID+"/retry", "", key()...), http.StatusConflict, "invalid_state_transition")

	failed := w.runIn(domain.StateFailed)
	w.person("stranger", false)
	w.want(w.do("stranger", http.MethodPost, "/api/v1/runs/"+failed.ID+"/retry", "", key()...), http.StatusNotFound, "not_found")
	if n := len(w.runsWhere(`thread_id = $1`, failed.Thread)); n != 1 {
		t.Fatalf("a refused retry opened a run: %d", n)
	}
}

// TestThreadOperatorOutsideChannel (09 3.4 step 4a, addendum of 2026-10-30): a thread whose fixed
// operator is not the channel's operator and never completed a run there (an imported thread after
// a switch-over to another operator) gets no run; the message shows operator_not_in_channel, and a
// retry of an earlier failed run of that operator is refused with the same reason.
func TestThreadOperatorOutsideChannel(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _ := w.openRun()
	w.mustTransition(r.ID, domain.StateQueued, domain.StateFailed, 1, "agent_unavailable", domain.SetterServer)
	w.sql(`UPDATE threads SET operator_agent_id = $1 WHERE id = $2`, w.texter.String(), r.Thread)
	w.sql(`UPDATE runs SET agent_id = $1 WHERE id = $2`, w.texter.String(), r.ID)

	p := w.want(w.do("alex", http.MethodPost, "/api/v1/runs/"+r.ID+"/retry", "", key()...), http.StatusForbidden, "agent_invocation_denied")
	if p["reason"] != "operator_not_in_channel" {
		t.Fatalf("problem %v", p)
	}
	m := w.post("alex", ids.MustParse(r.Thread), "Still there?")
	if m["trigger_state"] != "failed" || m["trigger_state_reason"] != "operator_not_in_channel" {
		t.Fatalf("message %v", m)
	}
	if n := len(w.runsWhere(`thread_id = $1`, r.Thread)); n != 1 {
		t.Fatalf("runs in the thread %d, want only the earlier one", n)
	}
}
