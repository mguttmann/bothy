// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

const (
	usersPath  = "/api/v1/users"
	ownersPath = "/api/v1/owners"
)

func userPath(id ids.ID) string { return usersPath + "/" + id.String() }

// writeCalls are the five owner writes with a valid body for target.
func writeCalls(target ids.ID) []struct{ method, path, body string } {
	return []struct{ method, path, body string }{
		{http.MethodPost, usersPath, `{"handle":"newbie","display_name":"Newbie"}`},
		{http.MethodPatch, userPath(target), `{"display_name":"Renamed"}`},
		{http.MethodPost, userPath(target) + "/enable", `{"reason":"Back"}`},
		{http.MethodPost, ownersPath, `{"user_id":"` + target.String() + `","reason":"Deputy"}`},
		{http.MethodPost, ownersPath + "/" + target.String() + "/revoke", `{"reason":"Done"}`},
	}
}

// TestNoSelfRegistration (test 1): anonymous createUser is 401, a person without owner role 403
// owner_only, and no operation of the ticket called by a non-owner inserts a person.
func TestNoSelfRegistration(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.person("robin", "active", true)
	alex := w.person("alex", "active", false)
	w.want(w.do("", http.MethodPost, usersPath, `{"handle":"eve","display_name":"Eve"}`, ""), http.StatusUnauthorized, "unauthenticated")
	p := w.want(w.do("alex", http.MethodPost, usersPath, `{"handle":"eve","display_name":"Eve"}`, ""), http.StatusForbidden, "forbidden")
	if p["reason"] != "owner_only" {
		t.Fatalf("reason %v, want owner_only", p["reason"])
	}
	before := w.count(`SELECT count(*) FROM users`)
	for _, c := range writeCalls(alex) {
		w.do("alex", c.method, c.path, c.body, ids.New().String())
	}
	for _, path := range []string{usersPath, userPath(alex), ownersPath, "/api/v1/me"} {
		w.do("alex", http.MethodGet, path, "", "")
	}
	w.do("alex", http.MethodPatch, "/api/v1/me/profile", `{"display_name":"Alex A"}`, "")
	if after := w.count(`SELECT count(*) FROM users`); after != before {
		t.Fatalf("people %d -> %d after the calls of a non-owner", before, after)
	}
}

// TestStepUp (test 2): the owner writes need a fresh step-up (AZ-M36 shape); a personal token
// of an owner is no owner (SR5: 403 owner_only, docs/DECISIONS.md M1-31); getMe and
// updateMyProfile need no step-up.
func TestStepUp(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	robin := w.person("robin", "active", true)
	alex := w.person("alex", "disabled", true) // an owner, so revokeOwner reaches authorize
	w.session("stale", robin, authz.DeviceActive, w.clk.Now().Add(-6*time.Minute))
	w.session("never", robin, authz.DeviceActive, time.Time{})
	w.token("pat", robin, actor.KindUser)
	for _, c := range writeCalls(alex) {
		for _, name := range []string{"stale", "never"} {
			w.want(w.do(name, c.method, c.path, c.body, ""), http.StatusUnauthorized, "step_up_required")
		}
		p := w.want(w.do("pat", c.method, c.path, c.body, ""), http.StatusForbidden, "forbidden")
		if p["reason"] != "owner_only" {
			t.Errorf("%s %s with a personal token: reason %v", c.method, c.path, p["reason"])
		}
	}
	w.want(w.do("never", http.MethodGet, "/api/v1/me", "", ""), http.StatusOK, "")
	w.want(w.do("never", http.MethodPatch, "/api/v1/me/profile", `{"locale":"en"}`, ""), http.StatusOK, "")
}

// TestAZ_M23_AddOwnerWithPersonalAgent (test 3): with agent Assistent personal for the owner,
// owner.add for Alex is 409 i1_violation naming agent, holder and Alex; nothing changed; one deny
// audit row.
func TestAZ_M23_AddOwnerWithPersonalAgent(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	alex := w.person("alex", "active", false)
	assistent := w.agent("assistent")
	w.personalAgent(assistent, owner)
	w.channel(assistent)
	version := w.permissionVersion()
	p := w.want(w.do("owner", http.MethodPost, ownersPath, `{"user_id":"`+alex.String()+`","reason":"Deputy"}`, ids.New().String()),
		http.StatusConflict, "i1_violation")
	conflicts, _ := p["conflicts"].([]any)
	if len(conflicts) != 1 {
		t.Fatalf("conflicts %v", p["conflicts"])
	}
	c := conflicts[0].(map[string]any)
	if c["agent_id"] != assistent.String() || c["credential_holder_id"] != owner.String() ||
		!slices.Equal(c["principal_ids"].([]any), []any{alex.String()}) {
		t.Fatalf("conflict %v", c)
	}
	if w.count(`SELECT count(*) FROM owners WHERE user_id = $1`, alex.String()) != 0 || w.permissionVersion() != version {
		t.Fatal("the rejected owner.add changed owners or the rights version")
	}
	if len(w.events("user.updated")) != 0 || len(w.events("access.changed")) != 0 {
		t.Fatal("the rejected owner.add emitted events")
	}
	var denies []auditRow
	for _, r := range w.audits("owner.add") {
		if r.Decision == "deny" {
			denies = append(denies, r)
		} else {
			t.Errorf("unexpected audit row %+v", r)
		}
	}
	if len(denies) != 1 || denies[0].Reason != "i1_violation" || denies[0].ResourceID != alex.String() {
		t.Fatalf("deny rows %+v", denies)
	}
}

// TestLastOwner (test 4): revoking the sole owner, also by that owner, is 409 last_owner.
func TestLastOwner(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	w.want(w.do("owner", http.MethodPost, ownersPath+"/"+owner.String()+"/revoke", `{"reason":"Leaving"}`, ""), http.StatusConflict, "last_owner")
	if w.count(`SELECT count(*) FROM owners WHERE revoked_at IS NULL`) != 1 {
		t.Fatal("the sole owner was revoked")
	}
}

// TestOwnerRace (test 5): two owners revoke each other at the same time: exactly one 204 and one
// 409 last_owner.
func TestOwnerRace(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	a := w.person("ann", "active", true)
	b := w.person("bea", "active", true)
	for range 3 {
		// Both are owners at the start of each round.
		w.sql(`INSERT INTO owners (id, user_id, reason) SELECT $1, p.id, 'again' FROM principals p
			WHERE p.kind = 'user' AND NOT EXISTS (SELECT 1 FROM owners o WHERE o.user_id = p.id AND o.revoked_at IS NULL)`, ids.New().String())
		var wg sync.WaitGroup
		statuses := make([]int, 2)
		codes := make([]string, 2)
		for i, c := range []struct {
			name   string
			target ids.ID
		}{{"ann", b}, {"bea", a}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				r := w.h.Do(w.h.NewRequest(http.MethodPost, ownersPath+"/"+c.target.String()+"/revoke", `{"reason":"Race"}`, "Authorization", "Bearer "+c.name))
				statuses[i] = r.StatusCode
				var p map[string]any
				_ = json.Unmarshal(r.Body, &p)
				codes[i], _ = p["code"].(string)
			}()
		}
		wg.Wait()
		slices.Sort(statuses)
		if !slices.Equal(statuses, []int{http.StatusNoContent, http.StatusConflict}) || !slices.Contains(codes, "last_owner") {
			t.Fatalf("statuses %v, codes %v", statuses, codes)
		}
		if w.count(`SELECT count(*) FROM owners WHERE revoked_at IS NULL`) != 1 {
			t.Fatal("not exactly one owner left")
		}
	}
}

// TestAddOwner (test 6): 201, user.updated with is_owner, access.changed for the new owner, audit
// owner.add with reason, listOwners contains the person; a second add is 409 conflict, a disabled
// person 409 conflict, an agent 404.
func TestAddOwner(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	alex := w.person("alex", "active", false)
	disabled := w.person("dora", "disabled", false)
	bot := w.agent("lotse")
	w.channel(ids.ID{})
	version := w.permissionVersion()
	body := `{"user_id":"` + alex.String() + `","reason":"Deputy"}`
	o := w.want(w.do("owner", http.MethodPost, ownersPath, body, ids.New().String()), http.StatusCreated, "")
	if o["user"].(map[string]any)["id"] != alex.String() || o["granted_by"] != owner.String() || o["reason"] != "Deputy" {
		t.Fatalf("owner %v", o)
	}
	if w.permissionVersion() != version+1 {
		t.Errorf("rights version %d, want %d", w.permissionVersion(), version+1)
	}
	ev := w.events("user.updated")
	if len(ev) != 1 || ev[0].ResourceID != alex.String() || ev[0].Payload["user"].(map[string]any)["is_owner"] != true ||
		ev[0].Actor != owner.String() || ev[0].ResourceKind != "user" || ev[0].ReceiveAction != "user.read" || ev[0].HasScope || ev[0].Audience != "" {
		t.Fatalf("user.updated %+v", ev)
	}
	ac := w.events("access.changed")
	if len(ac) != 1 || ac[0].Audience != alex.String() {
		t.Fatalf("access.changed %+v", ac)
	}
	a := w.audits("owner.add")
	if len(a) != 1 || a[0].Decision != "allow" || a[0].Details["reason"] != "Deputy" || a[0].ResourceID != alex.String() {
		t.Fatalf("audit %+v", a)
	}
	list := w.want(w.do("owner", http.MethodGet, ownersPath, "", ""), http.StatusOK, "")
	items := list["items"].([]any)
	if len(items) != 2 || items[1].(map[string]any)["user"].(map[string]any)["id"] != alex.String() {
		t.Fatalf("owners %v", items)
	}
	w.want(w.do("owner", http.MethodPost, ownersPath, body, ""), http.StatusConflict, "conflict")
	w.want(w.do("owner", http.MethodPost, ownersPath, `{"user_id":"`+disabled.String()+`","reason":"x"}`, ""), http.StatusConflict, "conflict")
	w.want(w.do("owner", http.MethodPost, ownersPath, `{"user_id":"`+bot.String()+`","reason":"x"}`, ""), http.StatusNotFound, "not_found")
	sam := w.person("sam", "active", false)
	w.want(w.do("owner", http.MethodPost, ownersPath, `{"user_id":"`+sam.String()+`","reason":"x\u202e"}`, ""), http.StatusBadRequest, "validation_failed")
	// Revoke: 204, user.updated with is_owner false; revoking again is 404.
	w.want(w.do("owner", http.MethodPost, ownersPath+"/"+alex.String()+"/revoke", `{"reason":"Done"}`, ""), http.StatusNoContent, "")
	ev = w.events("user.updated")
	if len(ev) != 2 || ev[1].Payload["user"].(map[string]any)["is_owner"] != false {
		t.Fatalf("user.updated after revoke %+v", ev)
	}
	w.want(w.do("owner", http.MethodPost, ownersPath+"/"+alex.String()+"/revoke", `{"reason":"Done"}`, ""), http.StatusNotFound, "not_found")
	if r := w.audits("owner.remove"); len(r) != 1 || r[0].Details["reason"] != "Done" {
		t.Fatalf("owner.remove audit %+v", r)
	}
}

// TestEnableUser (test 7).
func TestEnableUser(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	pending := w.person("pia", "pending", false)
	disabled := w.person("dora", "disabled", false)
	w.sql(`UPDATE principals SET disabled_at = now() WHERE id = $1`, disabled.String())
	deleted := w.person("del", "deleted", false)
	active := w.person("alex", "active", false)
	enable := func(id ids.ID) *httpservertest.Response {
		return w.do("owner", http.MethodPost, userPath(id)+"/enable", `{"reason":"Approved"}`, "")
	}
	u := w.want(enable(pending), http.StatusOK, "")
	if u["status"] != "active" {
		t.Fatalf("pending -> %v", u)
	}
	w.want(enable(disabled), http.StatusOK, "")
	if w.count(`SELECT count(*) FROM principals WHERE id = $1 AND status = 'active' AND disabled_at IS NULL`, disabled.String()) != 1 {
		t.Fatal("disabled_at not cleared")
	}
	w.want(enable(deleted), http.StatusConflict, "invalid_state_transition")
	events, version := len(w.events("user.updated")), w.permissionVersion()
	w.want(enable(active), http.StatusOK, "")
	if len(w.events("user.updated")) != events || w.permissionVersion() != version {
		t.Fatal("enabling an active person emitted an event or raised the rights version")
	}
	a := w.audits("user.enable")
	if len(a) != 3 || a[0].Details["from_status"] != "pending" || a[1].Details["from_status"] != "disabled" ||
		a[2].Details["no_change"] != true || a[0].Details["reason"] != "Approved" {
		t.Fatalf("audit %+v", a)
	}
	// A disabled second owner while a personal agent exists: enabling would give it trigger.
	second := w.person("sam", "disabled", true)
	agent := w.agent("assistent")
	w.personalAgent(agent, owner)
	served := w.channel(agent)
	w.want(enable(second), http.StatusConflict, "i1_violation")
	// A person with trigger on a channel the personal agent serves.
	robin := w.person("robin", "disabled", false)
	w.sql(`INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, target_channel_id, level, reason, granted_by)
		VALUES ($1, 'user', $2, 'channel', $3, 'trigger', 'setup', $4)`, ids.New().String(), robin.String(), served.String(), owner.String())
	w.want(enable(robin), http.StatusConflict, "i1_violation")
	if w.count(`SELECT count(*) FROM principals WHERE id IN ($1, $2) AND status = 'disabled'`, second.String(), robin.String()) != 2 {
		t.Fatal("a rejected enable changed the status")
	}
}

// TestValidation (test 8).
func TestValidation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.person("owner", "active", true)
	w.want(w.do("owner", http.MethodPost, usersPath, `{"handle":"alex","display_name":"Alex","email":"alex2@example.com"}`, ""), http.StatusCreated, "")
	cases := []struct {
		body, pointer string
		status        int
	}{
		{`{"handle":"Alex","display_name":"A"}`, "/handle", 400},
		{`{"handle":".alex","display_name":"A"}`, "/handle", 400},
		{`{"handle":"` + strings.Repeat("a", 13) + `","display_name":"A"}`, "/handle", 400},
		{`{"handle":"ALEX","display_name":"A"}`, "/handle", 400},
		{`{"handle":"alex","display_name":"A"}`, "/handle", 409},
		{`{"handle":"other","display_name":"A","email":"ALEX2@example.com"}`, "/email", 409},
		{`{"handle":"other","display_name":"Al\u202eex"}`, "/display_name", 400},
		{`{"handle":"other","display_name":"Al\u0007ex"}`, "/display_name", 400},
		{`{"handle":"other","display_name":"A","time_zone":"Mars/Olympus"}`, "/time_zone", 400},
		{`{"handle":"other","display_name":"A","time_zone":"Local"}`, "/time_zone", 400},
		{`{"handle":"other","display_name":"A","locale":"not a tag"}`, "/locale", 400},
	}
	for _, c := range cases {
		r := w.do("owner", http.MethodPost, usersPath, c.body, "")
		code := "validation_failed"
		if c.status == 409 {
			code = "conflict"
		}
		p := w.want(r, c.status, code)
		pointers := fmt.Sprint(p["errors"])
		if !strings.Contains(pointers, c.pointer) {
			t.Errorf("%s: errors %v, want pointer %s", c.body, p["errors"], c.pointer)
		}
	}
	// Locale and time zone are stored canonical.
	u := w.want(w.do("owner", http.MethodPost, usersPath, `{"handle":"sam","display_name":"Sam","locale":"en-us","time_zone":"Europe/Berlin"}`, ""), http.StatusCreated, "")
	if usr := u["user"].(map[string]any); usr["locale"] != "en-US" || usr["time_zone"] != "Europe/Berlin" || usr["status"] != "active" || usr["is_owner"] != false {
		t.Fatalf("user %v", usr)
	}
	d := w.want(w.do("owner", http.MethodPost, usersPath, `{"handle":"kim","display_name":"Kim"}`, ""), http.StatusCreated, "")
	if usr := d["user"].(map[string]any); usr["locale"] != "de" || usr["time_zone"] != "UTC" {
		t.Fatalf("defaults %v", usr)
	}
	if _, ok := d["enrollment"]; ok {
		t.Fatal("enrollment without local_account")
	}
	// Without an enabled local provider a local account is refused (M1-37).
	p := w.want(w.do("owner", http.MethodPost, usersPath, `{"handle":"robin","display_name":"Robin","local_account":true}`, ""), http.StatusConflict, "conflict")
	if p["reason"] != "local_provider_disabled" {
		t.Fatalf("reason %v, want local_provider_disabled", p["reason"])
	}
}
