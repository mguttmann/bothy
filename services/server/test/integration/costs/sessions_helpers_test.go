// SPDX-License-Identifier: Apache-2.0

package costs_test

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

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
