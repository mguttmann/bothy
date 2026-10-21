// SPDX-License-Identifier: Apache-2.0

package agentsessions_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestListAgentSessions (09 9.3, design notes 4 and 6): newest first, every field of AgentSession,
// no JSON null, cost in Money form; an empty thread gives items []; Robin (no read on Support)
// gets 404 with the body of an unknown thread ID.
func TestListAgentSessions(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	th := w.newThread(w.allgemein)
	empty := w.wantOK(w.listSessions("alex", th))
	if items, ok := empty["items"].([]any); !ok || len(items) != 0 {
		t.Fatalf("empty history %v", empty)
	}
	run := w.a6StartedRun(th, `"external_session_ref":"sess-list-1"`, `"model":"model-large"`)
	w.finishOK(run)
	first, _ := w.current(th)
	second := w.a6Bind("alex", th, "sess-list-2", first.ID, false)
	r := w.listSessions("alex", th)
	if strings.Contains(string(r.Body), "null") {
		t.Fatalf("JSON null in %s", r.Body)
	}
	items, _ := w.wantOK(r)["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items %v", items)
	}
	newest, older := items[0].(map[string]any), items[1].(map[string]any)
	if newest["id"] != second["id"] || older["id"] != first.ID {
		t.Fatalf("order %v, %v; want %v then %s", newest["id"], older["id"], second["id"], first.ID)
	}
	ended := w.a6RowByID(th, first.ID)
	for k, want := range map[string]any{
		"thread_id": th.String(), "agent_id": w.lotse.String(), "external_session_ref": "sess-list-1", "is_current": false,
		"started": false, "model": "model-large", "end_reason": "rebind", "version": float64(ended.Version),
	} {
		if older[k] != want {
			t.Fatalf("older %s = %v, want %v: %v", k, older[k], want, older)
		}
	}
	for _, k := range []string{"created_at", "ended_at"} {
		if s, _ := older[k].(string); s == "" {
			t.Fatalf("older has no %s: %v", k, older)
		}
	}
	for k, want := range map[string]any{
		"thread_id": th.String(), "agent_id": w.lotse.String(), "external_session_ref": "sess-list-2", "is_current": true,
		"started": true, "version": float64(1),
	} {
		if newest[k] != want {
			t.Fatalf("newest %s = %v, want %v: %v", k, newest[k], want, newest)
		}
	}
	// Absent instead of null (ADR 0053); never the cost baseline or the dev environment.
	for _, k := range []string{"model", "ended_at", "end_reason", "cost_baseline_micros", "dev_environment_id"} {
		if _, ok := newest[k]; ok {
			t.Fatalf("newest carries %s: %v", k, newest)
		}
	}
	for _, it := range items {
		m := it.(map[string]any)
		cost, ok := m["cost"].(map[string]any)
		// cost_is_estimate defaults to true until a reconciliation (03 agent_sessions).
		if !ok || cost["amount_micros"] != float64(0) || cost["is_estimate"] != true || cost["currency"] == "" || cost["currency"] == nil {
			t.Fatalf("cost %v", m["cost"])
		}
		if _, ok := m["cost_baseline_micros"]; ok {
			t.Fatalf("cost_baseline_micros in %v", m)
		}
	}
	a6SameNotFound(t, w.listSessions("robin", th), w.listSessions("robin", ids.New()))
}

// TestNewSession_FirstReportCreatesRow (adapter path without a reference at the start, 09 17.4):
// startRun without reference creates nothing; a report without If-Match creates the current row and
// sets runs.agent_session_id; exactly one agent_session.changed.
func TestNewSession_FirstReportCreatesRow(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	th := w.newThread(w.allgemein)
	run := w.a6StartedRun(th)
	if rows := w.sessionRows(th); len(rows) != 0 || w.runSession(run) != "" {
		t.Fatalf("start without reference: rows %+v, run session %q", rows, w.runSession(run))
	}
	if n := len(w.sessionEvents(th)); n != 0 {
		t.Fatalf("%d agent_session.changed before the report", n)
	}
	r := w.report("lotse", th, `{"external_session_ref":"sess-first-1","model":"model-large","started":true}`, key()...)
	body := w.wantOK(r)
	a6WantETag(t, r, version(body))
	rows := w.sessionRows(th)
	if len(rows) != 1 || !rows[0].Current || !rows[0].Started || rows[0].Ref != "sess-first-1" || rows[0].Model != "model-large" ||
		rows[0].Version != 1 || body["id"] != rows[0].ID {
		t.Fatalf("rows %+v, answer %v", rows, body)
	}
	if w.runSession(run) != rows[0].ID {
		t.Fatalf("run session %q, want %s", w.runSession(run), rows[0].ID)
	}
	ev := w.sessionEvents(th)
	if len(ev) != 1 || sessionOf(ev[0])["id"] != rows[0].ID {
		t.Fatalf("events %v", ev)
	}
}

// TestSessionCrashBeforeStarted (REUSE L14): the run fails before started; the next trigger
// delivers new with the same row; startRun with the same reference attaches it and creates no
// second row.
func TestSessionCrashBeforeStarted(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	th := w.newThread(w.allgemein)
	run := w.a6StartedRun(th, `"external_session_ref":"sess-crash-1"`)
	row, ok := w.current(th)
	if !ok || row.Started {
		t.Fatalf("row %+v", row)
	}
	w.wantOK(w.finish("lotse", run, `"state":"failed","reason":"runtime_error"`))
	next := w.trigger(th)
	p := w.a6WantMode(next, "new", row.ID)
	if s := sessionOf(p); s["started"] != false || s["external_session_ref"] != "sess-crash-1" {
		t.Fatalf("delivered row %v", s)
	}
	body := w.wantOK(w.start("lotse", next, w.deliver(next), `"external_session_ref":"sess-crash-1"`))
	if rows := w.sessionRows(th); len(rows) != 1 || rows[0].ID != row.ID || rows[0].Version != row.Version {
		t.Fatalf("rows %+v", rows)
	}
	if w.runSession(next) != row.ID || body["agent_session_id"] != row.ID {
		t.Fatalf("run session %q, answer %v, want %s", w.runSession(next), body["agent_session_id"], row.ID)
	}
}

// TestSessionSurvivesRename (REUSE L01, rename half): setThreadTitle on a thread with a started
// session; the same row stays current; the next trigger delivers resume with it. The owner renames:
// a person may set the title of a thread only as its creator (thread.title.update).
func TestSessionSurvivesRename(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	th := w.newThread(w.allgemein)
	run := w.a6StartedRun(th, `"external_session_ref":"sess-rename-1"`)
	w.wantOK(w.report("lotse", th, `{"external_session_ref":"sess-rename-1","started":true}`, append(match(1), key()...)...))
	w.finishOK(run)
	before, _ := w.current(th)
	threadVersion := w.count(`SELECT version FROM threads WHERE id = $1`, th.String())
	w.wantOK(w.do("owner", http.MethodPut, "/api/v1/threads/"+th.String()+"/title", `{"title":"Invoice export fails for March"}`,
		ifMatch(threadVersion)...))
	if w.text(`SELECT title FROM threads WHERE id = $1`, th.String()) != "Invoice export fails for March" {
		t.Fatal("the title did not change")
	}
	after, ok := w.current(th)
	if !ok || after != before {
		t.Fatalf("row after the rename %+v, before %+v", after, before)
	}
	next := w.trigger(th)
	w.a6WantMode(next, "resume", before.ID)
}

// TestResetSession (09 9.2, design note 7): Alex resets; 201 with the ended row (end_reason reset)
// and its ETag; one agent_session.changed; the next trigger delivers new without agent_session; a
// second reset gives 409 conflict with reason no_current_session; no allow audit entry.
func TestResetSession(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	th := w.newThread(w.allgemein)
	run := w.a6StartedRun(th, `"external_session_ref":"sess-reset-1"`)
	w.wantOK(w.report("lotse", th, `{"external_session_ref":"sess-reset-1","started":true}`, append(match(1), key()...)...))
	w.finishOK(run)
	row, _ := w.current(th)
	seq := w.maxSeq()
	r := w.reset("alex", th)
	body := w.want(r, http.StatusCreated, "")
	a6WantETag(t, r, version(body))
	if body["id"] != row.ID || body["end_reason"] != "reset" || body["is_current"] != false || version(body) != row.Version+1 {
		t.Fatalf("answer %v, row before %+v", body, row)
	}
	if s, _ := body["ended_at"].(string); s == "" {
		t.Fatalf("no ended_at: %v", body)
	}
	if got := w.a6RowByID(th, row.ID); got.Current || got.EndReason != "reset" {
		t.Fatalf("row %+v", got)
	}
	if ch := w.a6ChangesAfter(seq); len(ch) != 1 || ch[0].Session != row.ID || ch[0].EndReason != "reset" || ch[0].Current {
		t.Fatalf("events %+v", ch)
	}
	w.wantReason(w.reset("alex", th), http.StatusConflict, "conflict", "no_current_session")
	if a := w.a6Allows("agent_session.reset"); len(a) != 0 {
		t.Fatalf("allow audit entries of agent_session.reset: %+v", a)
	}
	next := w.trigger(th)
	w.a6WantMode(next, "new", "")
}

// TestResetDuringActiveRun (design note 7 d): with a run running on row S, a reset ends S; a report
// on S with If-Match still succeeds; runs.agent_session_id stays S; after finishRun the next
// trigger delivers new.
func TestResetDuringActiveRun(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	th := w.newThread(w.allgemein)
	run := w.a6StartedRun(th, `"external_session_ref":"sess-active-1"`)
	s, _ := w.current(th)
	ended := w.want(w.reset("alex", th), http.StatusCreated, "")
	if ended["id"] != s.ID || ended["end_reason"] != "reset" {
		t.Fatalf("reset %v", ended)
	}
	if got := w.lifecycle(run); got.State != "running" {
		t.Fatalf("the reset disturbed the run: %+v", got)
	}
	rep := w.wantOK(w.report("lotse", th, `{"external_session_ref":"sess-active-1","started":true}`, append(match(version(ended)), key()...)...))
	if rep["id"] != s.ID || rep["started"] != true || rep["is_current"] != false || version(rep) != version(ended)+1 {
		t.Fatalf("report %v", rep)
	}
	if w.runSession(run) != s.ID {
		t.Fatalf("run session %q, want %s", w.runSession(run), s.ID)
	}
	if _, ok := w.current(th); ok {
		t.Fatal("the report made a current row")
	}
	w.finishOK(run)
	next := w.trigger(th)
	w.a6WantMode(next, "new", "")
}

// TestSessionSwitchRights (05 4.6, rule 1.6; design note 5): Sam (read only) gets 403 forbidden;
// Robin gets 404; Alex with a trigger grant on Engineering/Werkzeuge (operator Runner, host, as in
// AZ-M20) gets 403 agent_invocation_denied with reason tool_class_owner_only there; a read-only
// channel gives 403 channel_read_only; each denial is in the audit log, aggregated per
// audit.deny_aggregate_window (AZ-P13).
func TestSessionSwitchRights(t *testing.T) {
	t.Parallel()
	w := sessionsWorld(t)
	th := w.newThread(w.allgemein)
	w.a6Bind("alex", th, "sess-rights-1", "", false)
	cur, _ := w.current(th)
	before := w.a6State()
	bindBody := a6BindBody("sess-rights-2", cur.ID, false)

	// Sam reads Support/Allgemein through G-4 only: level_insufficient, twice for the aggregate.
	for range 2 {
		w.want(w.reset("sam", th), http.StatusForbidden, "forbidden")
	}
	w.want(w.bind("sam", th, bindBody), http.StatusForbidden, "forbidden")
	// Robin cannot read Support: the 404 of an unknown thread.
	a6SameNotFound(t, w.reset("robin", th), w.reset("robin", ids.New()))
	a6SameNotFound(t, w.bind("robin", th, bindBody), w.bind("robin", ids.New(), bindBody))
	w.a6Unchanged(before)

	// Runner operates Engineering/Werkzeuge with the tool class host: only owners may invoke it.
	w.wantOK(w.do("owner", http.MethodPut, "/api/v1/agents/"+w.runner.String()+"/tool-class", `{"tool_class":"host","reason":"Shell work"}`,
		ifMatch(w.agentVersion(w.runner))...))
	w.grant(`"subject_kind":"user","subject_id":"` + w.alex.String() + `","target_kind":"channel","target_id":"` + w.werkzeuge.String() +
		`","level":"trigger","reason":"Tools"`)
	tools := w.mainThread(w.werkzeuge)
	w.wantReason(w.reset("alex", tools), http.StatusForbidden, "agent_invocation_denied", "tool_class_owner_only")
	w.wantReason(w.bind("alex", tools, a6BindBody("sess-rights-3", "", false)), http.StatusForbidden, "agent_invocation_denied",
		"tool_class_owner_only")

	// A read-only channel refuses every write, also the session switch (05 rule 1.8). The world
	// archives the channel: setChannelReadOnly is not enabled in this contract, and an archived
	// channel answers the same 403 channel_read_only (07 4.5), audited as channel_archived.
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/channels/"+w.allgemein.String()+"/archive", `{"reason":"Closed"}`, key()...))
	w.want(w.reset("alex", th), http.StatusForbidden, "channel_read_only")
	w.want(w.bind("alex", th, bindBody), http.StatusForbidden, "channel_read_only")
	w.a6Unchanged(before)

	sam := w.a6Denials("agent_session.reset", w.sam)
	if len(sam) != 1 || sam[0].Reason != "level_insufficient" || sam[0].Resource != th.String() || sam[0].Count != 2 {
		t.Fatalf("denials of Sam %+v, want one aggregate of 2 level_insufficient", sam)
	}
	if d := w.a6Denials("agent_session.bind", w.sam); len(d) != 1 || d[0].Reason != "level_insufficient" {
		t.Fatalf("bind denials of Sam %+v", d)
	}
	for _, action := range []string{"agent_session.reset", "agent_session.bind"} {
		robin := w.a6Denials(action, w.robin)
		if len(robin) == 0 || !a6HasDenial(robin, th.String(), "") {
			t.Fatalf("%s denials of Robin %+v", action, robin)
		}
		alex := w.a6Denials(action, w.alex)
		if !a6HasDenial(alex, tools.String(), "tool_class_owner_only") || !a6HasDenial(alex, th.String(), "channel_archived") {
			t.Fatalf("%s denials of Alex %+v", action, alex)
		}
	}
}
