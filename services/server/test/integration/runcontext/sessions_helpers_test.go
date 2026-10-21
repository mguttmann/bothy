// SPDX-License-Identifier: Apache-2.0

package runcontext_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// sessionsWorld is the world of 07 11.1 with the instance row and the extensions of runsWorld.
func sessionsWorld(t *testing.T) *world {
	t.Helper()
	return runsWorld(t)
}

// trigger posts a message of Alex into thread (a thread of Support/Allgemein, operator Lotse)
// and returns the run it opened.
func (w *world) trigger(thread ids.ID) string {
	w.t.Helper()
	m := w.post("alex", thread, "Please look at the export")
	r, ok := w.runOf(m["id"].(string))
	if !ok {
		w.t.Fatal("the message opened no run")
	}
	return r.ID
}

// invocation returns the payload of the agent.invocation event of run.
func (w *world) invocation(run string) map[string]any {
	w.t.Helper()
	list := w.eventsOf("agent.invocation", run)
	if len(list) != 1 {
		w.t.Fatalf("run %s has %d agent.invocation events, want 1", run, len(list))
	}
	return list[0]
}

// sessionOf returns the member agent_session of an invocation payload, nil when absent.
func sessionOf(p map[string]any) map[string]any {
	s, _ := p["agent_session"].(map[string]any)
	return s
}

// sessionPath is the path of the session operations of thread.
func sessionPath(thread ids.ID, op string) string {
	p := "/api/v1/threads/" + thread.String() + "/agent-sessions"
	if op != "" {
		p += "/" + op
	}
	return p
}

// listSessions sends listAgentSessions as name.
func (w *world) listSessions(name string, thread ids.ID) *httpservertest.Response {
	return w.do(name, http.MethodGet, sessionPath(thread, ""), "")
}

// reset sends resetAgentSession as name with a fresh Idempotency-Key.
func (w *world) reset(name string, thread ids.ID) *httpservertest.Response {
	return w.do(name, http.MethodPost, sessionPath(thread, "reset"), "", key()...)
}

// bind sends bindAgentSession as name with the JSON body and a fresh Idempotency-Key.
func (w *world) bind(name string, thread ids.ID, body string) *httpservertest.Response {
	return w.do(name, http.MethodPost, sessionPath(thread, "bind"), body, key()...)
}

// report sends reportAgentSession as the agent handle with the JSON body and the headers.
func (w *world) report(handle string, thread ids.ID, body string, headers ...string) *httpservertest.Response {
	return w.do(handle, http.MethodPost, sessionPath(thread, "report"), body, headers...)
}

// match returns the If-Match header of version.
func match(version int64) []string {
	return []string{"If-Match", `"` + strconv.FormatInt(version, 10) + `"`}
}

// sessionRow is one row of agent_sessions as the tests read it.
type sessionRow struct {
	ID, Thread, Agent, Ref, Model, EndReason, Currency string
	Current, Started                                   bool
	Baseline, Cumulative, Version                      int64
}

// sessionRows returns the rows of agent_sessions of thread, oldest first.
func (w *world) sessionRows(thread ids.ID) []sessionRow {
	w.t.Helper()
	rs, err := w.db.Query(context.Background(), `SELECT id::text, thread_id::text, agent_id::text, external_session_ref,
		coalesce(model, ''), coalesce(end_reason, ''), currency, is_current, started, cost_baseline_micros, cumulative_cost_micros, version
		FROM agent_sessions WHERE thread_id = $1 ORDER BY created_at, id`, thread.String())
	must(w.t, err)
	defer rs.Close()
	var out []sessionRow
	for rs.Next() {
		var r sessionRow
		must(w.t, rs.Scan(&r.ID, &r.Thread, &r.Agent, &r.Ref, &r.Model, &r.EndReason, &r.Currency, &r.Current, &r.Started, &r.Baseline,
			&r.Cumulative, &r.Version))
		out = append(out, r)
	}
	return out
}

// current returns the current session row of thread; ok is false for none.
func (w *world) current(thread ids.ID) (sessionRow, bool) {
	for _, r := range w.sessionRows(thread) {
		if r.Current {
			return r, true
		}
	}
	return sessionRow{}, false
}

// runSession returns runs.agent_session_id of run, "" for NULL.
func (w *world) runSession(run string) string {
	return w.text(`SELECT coalesce(agent_session_id::text, '') FROM runs WHERE id = $1`, run)
}

// sessionEvents returns the payloads of the agent_session.changed events of thread, in log order.
func (w *world) sessionEvents(thread ids.ID) []map[string]any {
	w.t.Helper()
	rs, err := w.db.Query(context.Background(), `SELECT payload::text FROM event_log WHERE type = 'agent_session.changed' AND thread_id = $1
		ORDER BY seq`, thread.String())
	must(w.t, err)
	defer rs.Close()
	var out []map[string]any
	for rs.Next() {
		var raw string
		must(w.t, rs.Scan(&raw))
		var m map[string]any
		must(w.t, json.Unmarshal([]byte(raw), &m))
		out = append(out, m)
	}
	return out
}

// finishOK finishes run of Lotse as completed.
func (w *world) finishOK(run string) {
	w.t.Helper()
	w.wantOK(w.finish("lotse", run, `"state":"completed"`))
}

// version reads the member version of a decoded body.
func version(m map[string]any) int64 {
	v, _ := m["version"].(float64)
	return int64(v)
}
