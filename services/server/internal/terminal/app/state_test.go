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

// TestSetHostState_Transitions (M2-39, PLAN row, 24 section 3.1): from each state reachable here
// to each of active, disabled and retired: the allowed ones 200 with the new state, the same state
// 200 with no_change, every other 409 invalid_state_transition with nothing written, enabling
// after duplicate_connection included.
func TestSetHostState_Transitions(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	type from struct{ state, reason string }
	const ok, same, refused = "ok", "same", "refused"
	cases := map[from]map[string]string{
		{"pairing", ""}:                      {"active": refused, "disabled": refused, "retired": refused},
		{"confirming", ""}:                   {"active": refused, "disabled": refused, "retired": refused},
		{"active", ""}:                       {"active": same, "disabled": ok, "retired": ok},
		{"disabled", "owner"}:                {"active": ok, "disabled": same, "retired": ok},
		{"disabled", "duplicate_connection"}: {"active": refused, "disabled": same, "retired": ok},
		{"retired", ""}:                      {"active": refused, "disabled": refused, "retired": same},
	}
	n := 0
	for f, targets := range cases {
		for to, want := range targets {
			n++
			h := w.Host(terminaltest.Host{Name: "Host " + ids.New().String()[:13], State: f.state, DisabledReason: f.reason})
			v, events, pv := w.HostVersion(h), len(w.Events("host.updated")), w.PermissionVersion()
			r := w.Do("robin", http.MethodPost, terminalworld.HostPath(h)+"/state", `{"state":"`+to+`","reason":"Test"}`, w.Keyed(h)...)
			switch want {
			case ok:
				m := w.Want(r, http.StatusOK, "")
				if m["state"] != to || m["version"] != float64(v+1) || w.PermissionVersion() != pv+1 || len(w.Events("host.updated")) != events+1 {
					t.Errorf("%v -> %s: %v", f, to, m)
				}
				if to == "disabled" && m["disabled_reason"] != "owner" {
					t.Errorf("%v -> %s: disabled_reason %v", f, to, m["disabled_reason"])
				}
				if to != "disabled" && m["disabled_reason"] != nil {
					t.Errorf("%v -> %s keeps disabled_reason", f, to)
				}
			case same:
				m := w.Want(r, http.StatusOK, "")
				if m["version"] != float64(v) || w.PermissionVersion() != pv || len(w.Events("host.updated")) != events {
					t.Errorf("%v -> %s changed something", f, to)
				}
				al := w.Allows("host.manage")
				if d := al[len(al)-1].Details; d["no_change"] != true || d["operation"] != "state" {
					t.Errorf("%v -> %s: audit %v", f, to, d)
				}
			default:
				w.Want(r, http.StatusConflict, "invalid_state_transition")
				if w.HostVersion(h) != v || w.PermissionVersion() != pv || len(w.Events("host.updated")) != events {
					t.Errorf("%v -> %s: a refused transition wrote", f, to)
				}
			}
		}
	}
	if n != 18 {
		t.Fatalf("%d cases", n)
	}
	// disabled_at describes the current disabled state: set by disable, cleared by enable.
	h := w.Host(terminaltest.Host{Name: "Toggle", State: "active"})
	w.Want(w.Do("robin", http.MethodPost, terminalworld.HostPath(h)+"/state", `{"state":"disabled","reason":"Off"}`, w.Keyed(h)...), http.StatusOK, "")
	if w.Count(`SELECT count(*) FROM hosts WHERE id = $1 AND disabled_at IS NOT NULL`, h.String()) != 1 {
		t.Fatal("disabled_at not set")
	}
	w.Want(w.Do("robin", http.MethodPost, terminalworld.HostPath(h)+"/state", `{"state":"active","reason":"On"}`, w.Keyed(h)...), http.StatusOK, "")
	if w.Count(`SELECT count(*) FROM hosts WHERE id = $1 AND disabled_at IS NULL AND disabled_reason IS NULL`, h.String()) != 1 {
		t.Fatal("disabled_at not cleared")
	}
}

// TestSetHostState_LeavingActiveEndsSessions (M2-39, PLAN row, T-TM-11): disabling R1 closes its
// sessions in opening, active and detached with host_disabled (detached_at NULL after), raises
// permission_version and writes host.updated and the audit entries in one transaction (an
// injected failure after the session update leaves the host active, the sessions open and the
// counter unchanged); retiring R2 closes its sessions with host_retired; other hosts are
// untouched.
func TestSetHostState_LeavingActiveEndsSessions(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	alexSU := w.SessionUser(w.R1, w.Alex, "bothy-s-alex")
	samR2 := w.SessionUser(w.R2, w.Sam, "bothy-s-sam")
	r1Sessions := []ids.ID{w.TerminalSession(w.R1, w.Alex, alexSU, "opening"), w.TerminalSession(w.R1, w.Alex, alexSU, "active"),
		w.TerminalSession(w.R1, w.Alex, alexSU, "detached")}
	r2Session := w.TerminalSession(w.R2, w.Sam, samR2, "active")
	pv := w.PermissionVersion()
	w.Audit.Set(func(e auditing.Entry) bool { return e.Attrs["operation"].Data() == "state" })
	w.Want(w.Do("robin", http.MethodPost, terminalworld.HostPath(w.R1)+"/state", `{"state":"disabled","reason":"Maintenance"}`, w.Keyed(w.R1)...),
		http.StatusInternalServerError, "internal_error")
	w.Audit.Set(nil)
	if w.Text(`SELECT state FROM hosts WHERE id = $1`, w.R1.String()) != "active" || w.PermissionVersion() != pv ||
		w.Count(`SELECT count(*) FROM terminal_sessions WHERE host_id = $1 AND state <> 'closed'`, w.R1.String()) != 3 ||
		len(w.Allows("terminal.terminate")) != 0 {
		t.Fatal("the injected failure left a partial change")
	}
	w.Want(w.Do("robin", http.MethodPost, terminalworld.HostPath(w.R1)+"/state", `{"state":"disabled","reason":"Maintenance"}`, w.Keyed(w.R1)...), http.StatusOK, "")
	for _, id := range r1Sessions {
		if got := w.Text(`SELECT state || '/' || close_reason || '/' || (detached_at IS NULL)::text FROM terminal_sessions WHERE id = $1`, id.String()); got != "closed/host_disabled/true" {
			t.Errorf("session %s: %s", id, got)
		}
	}
	if w.PermissionVersion() != pv+1 || len(w.Allows("terminal.terminate")) != 3 {
		t.Fatalf("pv %d, terminate entries %d", w.PermissionVersion()-pv, len(w.Allows("terminal.terminate")))
	}
	if w.Text(`SELECT state FROM terminal_sessions WHERE id = $1`, r2Session.String()) != "active" {
		t.Fatal("a session of another host ended")
	}
	w.Want(w.Do("robin", http.MethodPost, terminalworld.HostPath(w.R2)+"/state", `{"state":"retired","reason":"End of life"}`, w.Keyed(w.R2)...), http.StatusOK, "")
	if got := w.Text(`SELECT close_reason FROM terminal_sessions WHERE id = $1`, r2Session.String()); got != "host_retired" {
		t.Fatalf("R2 session: %s", got)
	}
}
