// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/terminaltest"
	"github.com/OWNER/bothy/services/server/test/terminalworld"
)

// TestUpdateHost_NameAndHolder (M2-39): a rename and a new holder give 200, version + 1, one
// host.updated, one audit entry update with name_changed and to_assigned_user_id, a raised
// permission_version and no access.changed; the new holder reads the host; clear_assigned_user
// removes the holder; both together are 400 mutually_exclusive.
func TestUpdateHost_NameAndHolder(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	r4 := w.Host(terminaltest.Host{Name: "Bench", State: "active"})
	w.Want(w.Do("sam", http.MethodGet, terminalworld.HostPath(r4), ""), http.StatusNotFound, "not_found")
	pv, seq, events := w.PermissionVersion(), w.LastSeq(), len(w.Events("host.updated"))
	m := w.Want(w.Do("robin", http.MethodPatch, terminalworld.HostPath(r4), `{"name":"Bench two","assigned_user_id":"`+w.Sam.String()+`","reason":"Sam takes it"}`,
		w.Keyed(r4)...), http.StatusOK, "")
	if m["name"] != "Bench two" || m["assigned_user_id"] != w.Sam.String() || m["version"] != float64(2) {
		t.Fatalf("answer %v", m)
	}
	if len(w.Events("host.updated")) != events+1 || w.PermissionVersion() != pv+1 || len(w.AccessChanges(seq)) != 0 {
		t.Fatalf("events %d, pv %d, access.changed %v", len(w.Events("host.updated"))-events, w.PermissionVersion()-pv, w.AccessChanges(seq))
	}
	al := w.Allows("host.manage")
	last := al[len(al)-1].Details
	if last["operation"] != "update" || last["name_changed"] != true || last["to_assigned_user_id"] != w.Sam.String() || last["from_assigned_user_id"] != nil {
		t.Fatalf("audit %v", last)
	}
	w.Want(w.Do("sam", http.MethodGet, terminalworld.HostPath(r4), ""), http.StatusOK, "")
	m = w.Want(w.Do("robin", http.MethodPatch, terminalworld.HostPath(r4), `{"clear_assigned_user":true,"reason":"Back"}`, w.Keyed(r4)...), http.StatusOK, "")
	if _, ok := m["assigned_user_id"]; ok {
		t.Fatalf("holder still set: %v", m)
	}
	al = w.Allows("host.manage")
	if d := al[len(al)-1].Details; d["from_assigned_user_id"] != w.Sam.String() || d["to_assigned_user_id"] != nil || d["name_changed"] != false {
		t.Fatalf("audit of the clear %v", d)
	}
	w.Want(w.Do("sam", http.MethodGet, terminalworld.HostPath(r4), ""), http.StatusNotFound, "not_found")
	p := w.Want(w.Do("robin", http.MethodPatch, terminalworld.HostPath(r4), `{"assigned_user_id":"`+w.Sam.String()+`","clear_assigned_user":true,"reason":"x"}`,
		w.Keyed(r4)...), http.StatusBadRequest, "validation_failed")
	if code, ptr := terminalworld.Field(p); code != "mutually_exclusive" || ptr != "/clear_assigned_user" {
		t.Fatalf("both fields: %s at %s", code, ptr)
	}
	if code, ptr := terminalworld.Field(w.Want(w.Do("robin", http.MethodPatch, terminalworld.HostPath(r4), `{"reason":"Nothing"}`, w.Keyed(r4)...),
		http.StatusBadRequest, "validation_failed")); code != "required" || ptr != "" {
		t.Fatalf("empty change: %s at %q", code, ptr)
	}
}

// TestUpdateHost_SessionUsers (M2-39, ruling T12): Robin assigns bothy-s-sam to Sam and
// bothy-s-robin to Robin on R2; session_users lists both; one host.updated; two audit entries
// session_user with change, user_id and os_user; permission_version raised.
func TestUpdateHost_SessionUsers(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	pv, events := w.PermissionVersion(), len(w.Events("host.updated"))
	m := w.Want(w.Do("robin", http.MethodPatch, terminalworld.HostPath(w.R2), `{"add_session_users":[{"user_id":"`+w.Sam.String()+
		`","os_user":"bothy-s-sam"},{"user_id":"`+w.Robin.String()+`","os_user":"bothy-s-robin"}],"reason":"Accounts for the lab"}`,
		w.Keyed(w.R2)...), http.StatusOK, "")
	users, _ := m["session_users"].([]any)
	if len(users) != 2 || users[0].(map[string]any)["os_user"] != "bothy-s-sam" || users[1].(map[string]any)["user_id"] != w.Robin.String() {
		t.Fatalf("session_users %v", m["session_users"])
	}
	if len(w.Events("host.updated")) != events+1 || w.PermissionVersion() != pv+1 {
		t.Fatal("one host.updated and one rights version")
	}
	var su []map[string]any
	for _, r := range w.Allows("host.manage") {
		if r.Details["operation"] == "session_user" {
			su = append(su, r.Details)
		}
	}
	if len(su) != 2 || su[0]["change"] != "assigned" || su[0]["user_id"] != w.Sam.String() || su[0]["os_user"] != "bothy-s-sam" ||
		su[1]["os_user"] != "bothy-s-robin" {
		t.Fatalf("session_user entries %v", su)
	}
}

// TestUpdateHost_NameNotReported_TMT28e (TM-T28 (e), server part of updateHost): a name the host
// does not report, also a case variant of a reported one, is 400 session_user_not_on_host at
// /add_session_users/0/os_user; nothing is stored.
func TestUpdateHost_NameNotReported_TMT28e(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	for _, name := range []string{"bothy-s-typo", "BOTHY-S-SAM"} {
		pv, v, events, rows, allows := w.PermissionVersion(), w.HostVersion(w.R1), len(w.Events("host.updated")),
			w.Count(`SELECT count(*) FROM host_session_users`), len(w.Allows("host.manage"))
		p := w.Want(w.Do("robin", http.MethodPatch, terminalworld.HostPath(w.R1), `{"add_session_users":[{"user_id":"`+w.Sam.String()+`","os_user":"`+name+
			`"}],"reason":"Account"}`, w.Keyed(w.R1)...), http.StatusBadRequest, "validation_failed")
		if code, ptr := terminalworld.Field(p); code != "session_user_not_on_host" || ptr != "/add_session_users/0/os_user" {
			t.Fatalf("%s: %s at %s", name, code, ptr)
		}
		if w.PermissionVersion() != pv || w.HostVersion(w.R1) != v || len(w.Events("host.updated")) != events ||
			w.Count(`SELECT count(*) FROM host_session_users`) != rows || len(w.Allows("host.manage")) != allows {
			t.Fatalf("%s: something was written", name)
		}
	}
}

// TestUpdateHost_SessionUserTaken_TMT28b (TM-T28 (b), server part of updateHost): a name that
// belongs or belonged to another person, also in another case, and a second valid assignment are
// 409 conflict with reason session_user_taken and change nothing; a request that revokes Sam's
// assignment and adds a new name for Sam succeeds.
func TestUpdateHost_SessionUserTaken_TMT28b(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	w.SessionUser(w.R1, w.Alex, "bothy-s-alex")
	patch := func(body string, status int, code string) map[string]any {
		t.Helper()
		return w.Want(w.Do("robin", http.MethodPatch, terminalworld.HostPath(w.R1), body, w.Keyed(w.R1)...), status, code)
	}
	add := func(user ids.ID, name string) string {
		return `{"user_id":"` + user.String() + `","os_user":"` + name + `"}`
	}
	patch(`{"revoke_session_users":["`+w.Alex.String()+`"],"reason":"Alex leaves"}`, http.StatusOK, "")
	taken := func(body string) {
		t.Helper()
		v, rows := w.HostVersion(w.R1), w.Count(`SELECT count(*) FROM host_session_users`)
		if p := patch(body, http.StatusConflict, "conflict"); p["reason"] != "session_user_taken" {
			t.Fatalf("%s: %v", body, p)
		}
		if w.HostVersion(w.R1) != v || w.Count(`SELECT count(*) FROM host_session_users`) != rows {
			t.Fatalf("%s changed something", body)
		}
	}
	taken(`{"add_session_users":[` + add(w.Sam, "bothy-s-alex") + `],"reason":"Reuse"}`)
	patch(`{"add_session_users":[`+add(w.Robin, "bothy-s-shared")+`],"reason":"Robin"}`, http.StatusOK, "")
	taken(`{"add_session_users":[` + add(w.Sam, "BOTHY-S-SHARED") + `],"reason":"Other case"}`)
	taken(`{"add_session_users":[` + add(w.Robin, "bothy-s-robin") + `],"reason":"Second"}`)
	taken(`{"add_session_users":[` + add(w.Sam, "bothy-s-sam") + `,` + add(w.Alex, "bothy-s-alex") + `],"reason":"Mixed"}`)
	if n := w.Count(`SELECT count(*) FROM host_session_users WHERE user_id = $1`, w.Sam.String()); n != 0 {
		t.Fatalf("the mixed request stored %d rows for Sam", n)
	}
	patch(`{"add_session_users":[`+add(w.Sam, "bothy-s-sam")+`],"reason":"Sam"}`, http.StatusOK, "")
	w.SQL(`UPDATE hosts SET reported_session_users = reported_session_users || '[{"os_user":"bothy-s-sam2","workdirs":[]}]' WHERE id = $1`, w.R1.String())
	m := patch(`{"revoke_session_users":["`+w.Sam.String()+`"],"add_session_users":[`+add(w.Sam, "bothy-s-sam2")+`],"reason":"Move"}`, http.StatusOK, "")
	found := false
	for _, u := range m["session_users"].([]any) {
		if u.(map[string]any)["user_id"] == w.Sam.String() {
			found = u.(map[string]any)["os_user"] == "bothy-s-sam2"
		}
	}
	if !found {
		t.Fatalf("Sam moved: %v", m["session_users"])
	}
	if n := w.Count(`SELECT count(*) FROM host_session_users WHERE host_id = $1 AND revoked_at IS NOT NULL`, w.R1.String()); n != 2 {
		t.Fatalf("%d ended rows stay, want 2 (Alex and Sam)", n)
	}
}

// TestUpdateHost_RevokeEndsSessions (M2-39, PLAN row, SR12, T-TM-11): revoking Alex on R1 closes
// exactly Alex's two open sessions with access_revoked in the transaction that ends the
// assignment (an injected failure after the session update rolls back both), the formerly
// detached session has detached_at NULL, Sam's session and the closed row stay, two
// terminal.terminate entries without an actor, and the ended row stays with its name.
func TestUpdateHost_RevokeEndsSessions(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	alexSU := w.SessionUser(w.R1, w.Alex, "bothy-s-alex")
	samSU := w.SessionUser(w.R1, w.Sam, "bothy-s-sam")
	a1 := w.TerminalSession(w.R1, w.Alex, alexSU, "active")
	a2 := w.TerminalSession(w.R1, w.Alex, alexSU, "detached")
	a3 := w.TerminalSession(w.R1, w.Alex, alexSU, "closed")
	s1 := w.TerminalSession(w.R1, w.Sam, samSU, "active")
	body := `{"revoke_session_users":["` + w.Alex.String() + `"],"reason":"Alex leaves"}`
	w.Audit.Set(func(e auditing.Entry) bool { return e.Attrs["change"].Data() == "revoked" })
	w.Want(w.Do("robin", http.MethodPatch, terminalworld.HostPath(w.R1), body, w.Keyed(w.R1)...), http.StatusInternalServerError, "internal_error")
	w.Audit.Set(nil)
	if n := w.Count(`SELECT count(*) FROM terminal_sessions WHERE state <> 'closed' AND user_id = $1`, w.Alex.String()); n != 2 {
		t.Fatalf("after the injected failure %d sessions of Alex are open, want 2", n)
	}
	if n := w.Count(`SELECT count(*) FROM host_session_users WHERE id = $1 AND revoked_at IS NULL`, alexSU.String()); n != 1 {
		t.Fatal("after the injected failure the assignment ended")
	}
	w.Want(w.Do("robin", http.MethodPatch, terminalworld.HostPath(w.R1), body, w.Keyed(w.R1)...), http.StatusOK, "")
	for _, c := range []struct {
		id            ids.ID
		state, reason string
	}{{a1, "closed", "access_revoked"}, {a2, "closed", "access_revoked"}, {a3, "closed", "client_closed"}, {s1, "active", ""}} {
		got := w.Text(`SELECT state || '/' || coalesce(close_reason, '') || '/' || (detached_at IS NULL)::text FROM terminal_sessions WHERE id = $1`, c.id.String())
		if want := c.state + "/" + c.reason + "/true"; got != want {
			t.Errorf("session %s: %s, want %s", c.id, got, want)
		}
	}
	terms := w.Allows("terminal.terminate")
	if len(terms) != 2 {
		t.Fatalf("%d terminal.terminate entries, want 2", len(terms))
	}
	for _, r := range terms {
		if r.Actor != "" || r.Details["close_reason"] != "access_revoked" || r.Details["bytes_out"] != float64(4096) || r.Details["duration_ms"] == nil {
			t.Errorf("terminal.terminate %+v", r)
		}
	}
	if w.Text(`SELECT os_user FROM host_session_users WHERE id = $1 AND revoked_at IS NOT NULL`, alexSU.String()) != "bothy-s-alex" {
		t.Fatal("the ended row is gone")
	}
}

// TestUpdateHost_NoChange_IfMatch_Idempotency (M2-39, 06 sections 2, 7 and 8): a request without
// effect answers 200 without a new version, terminalworld.event or rights version and audits no_change; a
// missing If-Match is 428, a stale one 412 with current; a replay with the same Idempotency-Key
// returns the stored answer and writes nothing; a retired host is 409 conflict.
func TestUpdateHost_NoChange_IfMatch_Idempotency(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	pv, events := w.PermissionVersion(), len(w.Events("host.updated"))
	m := w.Want(w.Do("robin", http.MethodPatch, terminalworld.HostPath(w.R1), `{"name":"Build server","assigned_user_id":"`+w.Alex.String()+
		`","revoke_session_users":["`+w.Sam.String()+`"],"reason":"Check"}`, w.Keyed(w.R1)...), http.StatusOK, "")
	if m["version"] != float64(1) || w.PermissionVersion() != pv || len(w.Events("host.updated")) != events {
		t.Fatalf("a call without effect changed something: %v", m)
	}
	al := w.Allows("host.manage")
	if d := al[len(al)-1].Details; d["no_change"] != true || d["operation"] != "update" {
		t.Fatalf("audit %v", d)
	}
	w.Want(w.Do("robin", http.MethodPatch, terminalworld.HostPath(w.R1), `{"name":"New","reason":"x"}`, terminalworld.Key()...), http.StatusPreconditionRequired, "precondition_required")
	p := w.Want(w.Do("robin", http.MethodPatch, terminalworld.HostPath(w.R1), `{"name":"New","reason":"x"}`, append(terminalworld.IfMatch(7), terminalworld.Key()...)...),
		http.StatusPreconditionFailed, "version_conflict")
	if cur, _ := p["current"].(map[string]any); cur["version"] != float64(1) {
		t.Fatalf("current %v", p["current"])
	}
	k := terminalworld.Key()
	first := w.Want(w.Do("robin", http.MethodPatch, terminalworld.HostPath(w.R1), `{"name":"Renamed","reason":"x"}`, append(terminalworld.IfMatch(1), k...)...), http.StatusOK, "")
	events = len(w.Events("host.updated"))
	replay := w.Do("robin", http.MethodPatch, terminalworld.HostPath(w.R1), `{"name":"Renamed","reason":"x"}`, append(terminalworld.IfMatch(1), k...)...)
	if replay.StatusCode != http.StatusOK || replay.Header.Get("Idempotent-Replay") != "true" || w.Want(replay, http.StatusOK, "")["version"] != first["version"] {
		t.Fatalf("replay %d %s", replay.StatusCode, replay.Body)
	}
	if len(w.Events("host.updated")) != events {
		t.Fatal("the replay wrote an terminalworld.Event")
	}
	retired := w.Host(terminaltest.Host{Name: "Gone", State: "retired"})
	w.Want(w.Do("robin", http.MethodPatch, terminalworld.HostPath(retired), `{"name":"Back","reason":"x"}`, w.Keyed(retired)...), http.StatusConflict, "conflict")
}
