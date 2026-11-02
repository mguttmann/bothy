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

// TestOwnerAccountAssignment (ADR 0073, D-OA-6, D-OA-8): the host R4 reports its owner account
// "owner" with owner_account = true. Only an active owner assigns it, only to himself, and only
// with the explicit opt-in; the flag must equal the report; the answer and the audit entry carry
// the flag.
func TestOwnerAccountAssignment(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	second := w.Person("second-owner", true)
	r4 := w.Host(terminaltest.Host{Name: "Home server", State: "active", ReportedSessionUsers: []string{"owner"}})
	w.SQL(`UPDATE hosts SET reported_session_users = '[{"os_user":"owner","workdirs":[],"owner_account":true}]' WHERE id = $1`, r4.String())
	patch := func(who string, host ids.ID, user ids.ID, name, flag string, status int, code string) map[string]any {
		t.Helper()
		body := `{"add_session_users":[{"user_id":"` + user.String() + `","os_user":"` + name + `"` + flag + `}],"reason":"Admin shell"}`
		return w.Want(w.Do(who, http.MethodPatch, terminalworld.HostPath(host), body, w.Keyed(host)...), status, code)
	}
	rows := func() int { return w.Count(`SELECT count(*) FROM host_session_users WHERE host_id = $1`, r4.String()) }
	// Without the opt-in, and the opt-in for a separate session user: 400 at owner_account.
	if code, ptr := terminalworld.Field(patch("robin", r4, w.Robin, "owner", "", http.StatusBadRequest, "validation_failed")); code != "owner_account_opt_in_required" ||
		ptr != "/add_session_users/0/owner_account" {
		t.Fatalf("no opt-in: %s at %s", code, ptr)
	}
	if code, ptr := terminalworld.Field(patch("robin", w.R2, w.Robin, "bothy-s-robin", `,"owner_account":true`, http.StatusBadRequest, "validation_failed")); code != "owner_account_not_on_host" ||
		ptr != "/add_session_users/0/owner_account" {
		t.Fatalf("separate session user: %s at %s", code, ptr)
	}
	// A non-owner person, and another owner, never get the owner account: 403, nothing stored.
	for _, who := range []ids.ID{w.Alex, second} {
		if p := patch("robin", r4, who, "owner", `,"owner_account":true`, http.StatusForbidden, "forbidden"); p["reason"] != "owner_account_owner_only" {
			t.Fatalf("assignment to %s: %v", who, p)
		}
	}
	if rows() != 0 {
		t.Fatal("a refused assignment was stored")
	}
	// The owner binds himself.
	m := patch("robin", r4, w.Robin, "owner", `,"owner_account":true`, http.StatusOK, "")
	users, _ := m["session_users"].([]any)
	if len(users) != 1 || users[0].(map[string]any)["owner_account"] != true || users[0].(map[string]any)["user_id"] != w.Robin.String() {
		t.Fatalf("session_users %v", m["session_users"])
	}
	if w.Count(`SELECT count(*) FROM host_session_users WHERE host_id = $1 AND owner_account`, r4.String()) != 1 {
		t.Fatal("the flag is not stored")
	}
	var entry map[string]any
	for _, r := range w.Allows("host.manage") {
		if r.Details["operation"] == "session_user" && r.Details["os_user"] == "owner" {
			entry = r.Details
		}
	}
	if entry == nil || entry["owner_account"] != true || entry["user_id"] != w.Robin.String() {
		t.Fatalf("audit %v", entry)
	}
}

// TestOpenAuditsOwnerAccount (D-OA-8): every opening in the owner account carries
// owner_account = true in its allow entry, every other opening false.
func TestOpenAuditsOwnerAccount(t *testing.T) {
	t.Parallel()
	for _, ownerAccount := range []bool{true, false} {
		w := newSWorld(t, nil)
		w.store.own[0].OwnerAccount = ownerAccount
		if _, err := w.open(); err != nil {
			t.Fatal(err)
		}
		var got any
		for _, e := range w.audit.entries {
			if e.Action == "terminal.open" {
				if v, ok := e.Attrs["owner_account"]; ok && v.Kind() == auditing.KindBool {
					got = v.Data()
				}
			}
		}
		if got != ownerAccount {
			t.Errorf("owner_account %v: audit %v", ownerAccount, got)
		}
	}
}
