// SPDX-License-Identifier: Apache-2.0

package agentsessions_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// a6BindBody is the JSON body AgentSessionBind with ref, the expected current session (omitted
// when empty) and take_over (omitted when false).
func a6BindBody(ref, expected string, takeOver bool) string {
	b, _ := json.Marshal(ref)
	body := `{"external_session_ref":` + string(b)
	if expected != "" {
		body += `,"expected_current_session_id":"` + expected + `"`
	}
	if takeOver {
		body += `,"take_over":true`
	}
	return body + "}"
}

// a6Bind binds ref in thread as name and expects 201; it returns the new row.
func (w *world) a6Bind(name string, thread ids.ID, ref, expected string, takeOver bool) map[string]any {
	w.t.Helper()
	r := w.bind(name, thread, a6BindBody(ref, expected, takeOver))
	body := w.want(r, http.StatusCreated, "")
	a6WantETag(w.t, r, version(body))
	return body
}

// a6WantETag checks that r carries ETag "<v>".
func a6WantETag(t *testing.T, r *httpservertest.Response, v int64) {
	t.Helper()
	if got, want := r.Header.Get("ETag"), `"`+strconv.FormatInt(v, 10)+`"`; got != want {
		t.Fatalf("ETag %q, want %q", got, want)
	}
}

// a6SameNotFound checks that two answers are the same 404 apart from correlation_id and instance
// in the body and Correlation-Id and Date in the headers (07 4.2: an invisible thread answers as an
// unknown one).
func a6SameNotFound(t *testing.T, a, b *httpservertest.Response) {
	t.Helper()
	if a.StatusCode != http.StatusNotFound || b.StatusCode != http.StatusNotFound {
		t.Fatalf("statuses %d %d: %s %s", a.StatusCode, b.StatusCode, a.Body, b.Body)
	}
	ha, hb := a.Header.Clone(), b.Header.Clone()
	for _, k := range []string{"Correlation-Id", "Date"} {
		ha.Del(k)
		hb.Del(k)
	}
	if fa, fb := fmt.Sprint(ha), fmt.Sprint(hb); fa != fb {
		t.Fatalf("404 headers differ:\n%s\n%s", fa, fb)
	}
	pa, pb := a.Problem(t), b.Problem(t)
	for _, k := range []string{"correlation_id", "instance"} {
		delete(pa, k)
		delete(pb, k)
	}
	ja, _ := json.Marshal(pa)
	jb, _ := json.Marshal(pb)
	if string(ja) != string(jb) {
		t.Fatalf("404 bodies differ:\n%s\n%s", ja, jb)
	}
}

// a6Change is one agent_session.changed event as the tests read it.
type a6Change struct {
	Thread, Session, EndReason string
	Current                    bool
	ResourceVersion            int64
}

// a6ChangesAfter returns the agent_session.changed events of every thread after seq, in log order.
func (w *world) a6ChangesAfter(seq int64) []a6Change {
	w.t.Helper()
	rs, err := w.db.Query(context.Background(), `SELECT coalesce(thread_id::text, ''), coalesce(resource_version, 0), payload::text
		FROM event_log WHERE type = 'agent_session.changed' AND seq > $1 ORDER BY seq`, seq)
	must(w.t, err)
	defer rs.Close()
	var out []a6Change
	for rs.Next() {
		var c a6Change
		var raw string
		must(w.t, rs.Scan(&c.Thread, &c.ResourceVersion, &raw))
		var p struct {
			ThreadID     string `json:"thread_id"`
			AgentSession struct {
				ID        string `json:"id"`
				IsCurrent bool   `json:"is_current"`
				EndReason string `json:"end_reason"`
			} `json:"agent_session"`
		}
		must(w.t, json.Unmarshal([]byte(raw), &p))
		if p.ThreadID != c.Thread {
			w.t.Fatalf("payload thread %s in an event of thread %s", p.ThreadID, c.Thread)
		}
		c.Session, c.Current, c.EndReason = p.AgentSession.ID, p.AgentSession.IsCurrent, p.AgentSession.EndReason
		out = append(out, c)
	}
	return out
}

// a6Snapshot is the state a refused session switch must leave unchanged: every row of
// agent_sessions, the agent_session.changed events and the allow entries of agent_session.bind
// (a denial writes its own deny entry, R-AZ-04).
type a6Snapshot struct {
	Rows   string
	Events int64
	Audits int
}

// a6State reads the a6Snapshot of the whole instance.
func (w *world) a6State() a6Snapshot {
	w.t.Helper()
	return a6Snapshot{
		Rows: w.text(`SELECT coalesce(string_agg(concat_ws('|', id, thread_id, external_session_ref, is_current, started, ended_at, end_reason,
			cost_baseline_micros, version), ',' ORDER BY id), '') FROM agent_sessions`),
		Events: w.count(`SELECT count(*) FROM event_log WHERE type = 'agent_session.changed'`),
		Audits: len(w.a6Allows("agent_session.bind")),
	}
}

// a6Unchanged fails when the state differs from before.
func (w *world) a6Unchanged(before a6Snapshot) {
	w.t.Helper()
	if after := w.a6State(); after != before {
		w.t.Fatalf("a refused switch changed the state:\nbefore %+v\nafter  %+v", before, after)
	}
}

// a6OneCurrentPerRef fails when an (agent_id, external_session_ref) is current in more than one
// thread or a thread has more than one current row (03 agent_sessions, both partial indexes).
func (w *world) a6OneCurrentPerRef() {
	w.t.Helper()
	if n := w.count(`SELECT count(*) FROM (SELECT 1 FROM agent_sessions WHERE is_current GROUP BY agent_id, external_session_ref
		HAVING count(*) > 1) d`); n != 0 {
		w.t.Fatalf("%d external sessions are current in more than one thread", n)
	}
	if n := w.count(`SELECT count(*) FROM (SELECT 1 FROM agent_sessions WHERE is_current GROUP BY thread_id HAVING count(*) > 1) d`); n != 0 {
		w.t.Fatalf("%d threads have more than one current session", n)
	}
}

// a6Allows returns the allow entries of action, oldest first.
func (w *world) a6Allows(action string) []auditRow {
	var out []auditRow
	for _, r := range w.audits(action) {
		if r.Decision == "allow" {
			out = append(out, r)
		}
	}
	return out
}

// a6Denial is one deny entry of the audit log with its actor and the count of its aggregate.
type a6Denial struct {
	Actor, Reason, Resource string
	Count                   int64
}

// a6Denials returns the deny entries of action by actor, oldest first, after flushing the
// aggregated denials (audit.deny_aggregate_window, AZ-P13).
func (w *world) a6Denials(action string, actor ids.ID) []a6Denial {
	w.t.Helper()
	must(w.t, w.rec.Aggregator().Flush(context.Background()))
	rs, err := w.db.Query(context.Background(), `SELECT coalesce(deny_reason, ''), coalesce(resource_id::text, ''),
		coalesce((details->'aggregate'->>'count')::bigint, 1) FROM audit_events
		WHERE action = $1 AND decision = 'deny' AND actor_principal_id = $2 ORDER BY seq`, action, actor.String())
	must(w.t, err)
	defer rs.Close()
	var out []a6Denial
	for rs.Next() {
		d := a6Denial{Actor: actor.String()}
		must(w.t, rs.Scan(&d.Reason, &d.Resource, &d.Count))
		out = append(out, d)
	}
	return out
}

// a6StartedRun triggers Lotse as Alex in thread, delivers the invocation and starts the run with
// the extra members of RunStart (for example the reference); it returns the run.
func (w *world) a6StartedRun(thread ids.ID, extra ...string) string {
	w.t.Helper()
	run := w.trigger("alex", thread)
	w.wantOK(w.start("lotse", run, w.deliver(run), extra...))
	return run
}

// a6RowByID returns the row id of thread.
func (w *world) a6RowByID(thread ids.ID, id string) sessionRow {
	w.t.Helper()
	rows := w.sessionRows(thread)
	i := slices.IndexFunc(rows, func(r sessionRow) bool { return r.ID == id })
	if i < 0 {
		w.t.Fatalf("no row %s in thread %s: %+v", id, thread, rows)
	}
	return rows[i]
}

// a6WantMode checks the session decision of the agent.invocation payload of run: mode and the
// row (empty: no agent_session).
func (w *world) a6WantMode(run, mode, row string) map[string]any {
	w.t.Helper()
	p := w.invocation(run)
	s := sessionOf(p)
	if p["session_mode"] != mode {
		w.t.Fatalf("session_mode %v, want %s: %v", p["session_mode"], mode, p)
	}
	switch {
	case row == "" && s != nil:
		w.t.Fatalf("agent_session %v, want none", s)
	case row != "" && (s == nil || s["id"] != row):
		w.t.Fatalf("agent_session %v, want %s", s, row)
	}
	return p
}

// a6HasDenial reports whether list has a denial on resource with reason ("" for any reason).
func a6HasDenial(list []a6Denial, resource, reason string) bool {
	for _, d := range list {
		if d.Resource == resource && (reason == "" || d.Reason == reason) {
			return true
		}
	}
	return false
}
