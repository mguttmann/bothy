// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents/adapters/agentstest"
	agentsdomain "github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// followupWorld is the world of the brief M2-11 ("Tests"): runsWorld with Sam's trigger on
// Support/Allgemein (a second person who writes follow-ups), thread_require_mention and
// allow_mentioned_agents on Support/Eskalation and Sam's write on Research/Notizen.
func followupWorld(t *testing.T) *world {
	t.Helper()
	w := runsWorld(t)
	w.grant(`"subject_kind":"user","subject_id":"` + w.sam.String() + `","target_kind":"channel","target_id":"` + w.allgemein.String() +
		`","level":"trigger","reason":"Second support writer"`)
	w.grant(`"subject_kind":"user","subject_id":"` + w.sam.String() + `","target_kind":"channel","target_id":"` + w.notizen.String() +
		`","level":"write","reason":"Research notes"`)
	w.behavior(w.eskalation, `"allow_mentioned_agents":true,"thread_require_mention":true`)
	return w
}

// followups sets Lotse's effective followups: the adapter's report (steer, queue or "" for none)
// and, with forbid, the owner's supports_followups = false.
func (w *world) followups(agent ids.ID, reported string, forbid bool) {
	w.t.Helper()
	agentstest.ReportCapabilities(w.t, w.db, agent, agentsdomain.Capabilities{Followups: reported})
	cfg := `{}`
	if forbid {
		cfg = `{"supports_followups": false}`
	}
	w.sql(`UPDATE agents SET operator_config = $1::jsonb WHERE principal_id = $2`, cfg, agent.String())
}

// fuRow is one row of run_followups.
type fuRow struct {
	Run, State string
}

// fuOf reads the row of the follow-up message; ok is false without one.
func (w *world) fuOf(message string) (fuRow, bool) {
	w.t.Helper()
	var r fuRow
	err := w.db.QueryRow(context.Background(), `SELECT run_id::text, state FROM run_followups WHERE message_id = $1`, message).Scan(&r.Run, &r.State)
	if err != nil {
		return fuRow{}, false
	}
	return r, true
}

// wantFollowup fails unless the message is a follow-up of run in state and its trigger_state is
// trigger (reason after a colon).
func (w *world) wantFollowup(message, run, state, trigger string) {
	w.t.Helper()
	r, ok := w.fuOf(message)
	if !ok || r.Run != run || r.State != state {
		w.t.Fatalf("follow-up %s: %+v (found %v), want run %s state %s", message, r, ok, run, state)
	}
	st, reason, _ := w.triggerOf(message)
	if reason != "" {
		st += ":" + reason
	}
	if st != trigger {
		w.t.Fatalf("follow-up %s: trigger_state %s, want %s", message, st, trigger)
	}
}

// followupEvents returns the payloads of agent.followup of run in log order.
func (w *world) followupEvents(run string) []map[string]any {
	return w.eventsOf("agent.followup", run)
}

// followupPayload reads the stored payload of the agent.followup of message.
func (w *world) followupPayload(message string) []byte {
	w.t.Helper()
	return []byte(w.text(`SELECT payload::text FROM event_log WHERE type = 'agent.followup' AND payload->>'message_id' = $1 ORDER BY seq DESC LIMIT 1`, message))
}

// deliverFollowup prepares the delivery of the stored agent.followup of message to the agent of
// run (the world has no poll, so this stands in for pollAgentEvents, as deliver does for
// agent.invocation); it returns the outcome.
func (w *world) deliverFollowup(run, message string) runs.DeliveryOutcome {
	w.t.Helper()
	agent := ids.MustParse(w.text(`SELECT agent_id::text FROM runs WHERE id = $1`, run))
	out, err := w.runs.Prepare(context.Background(), deliveryInput(agent, run, "agent.followup", w.followupPayload(message)))
	must(w.t, err)
	return out
}

// runVersion reads runs.version of run.
func (w *world) runVersion(run string) int64 {
	return w.count(`SELECT version FROM runs WHERE id = $1`, run)
}

// patchFollowup sends patchRun as handle: replace /followups/<message>/state with state at the
// current version of the run.
func (w *world) patchFollowup(handle, run, message, state string) *httpservertest.Response {
	body := `{"base_version":` + strconv.FormatInt(w.runVersion(run), 10) + `,"ops":[{"op":"replace","path":"/followups/` + message +
		`/state","value":"` + state + `"}]}`
	return w.do(handle, http.MethodPatch, "/api/v1/runs/"+run, body)
}

// say posts a text message as name into thread and returns its id.
func (w *world) say(name string, thread ids.ID, text string) string {
	w.t.Helper()
	return w.post(name, thread, text)["id"].(string)
}

// getRun reads the run document through getRun as name.
func (w *world) getRun(name, run string) map[string]any {
	w.t.Helper()
	return w.wantOK(w.do(name, http.MethodGet, "/api/v1/runs/"+run, ""))
}

// docFollowups returns the follow-ups of a run document as message id to state.
func docFollowups(doc map[string]any) map[string]string {
	out := map[string]string{}
	fu, _ := doc["followups"].(map[string]any)
	for k, v := range fu {
		out[k] = v.(map[string]any)["state"].(string)
	}
	return out
}

// activeRuns returns the active runs of thread.
func (w *world) activeRuns(thread ids.ID) []runRow {
	return w.runsWhere(`thread_id = $1 AND state IN ('queued','running','waiting_input')`, thread.String())
}

// mustDecode decodes raw into a map.
func mustDecode(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	must(t, json.Unmarshal(raw, &m))
	return m
}
