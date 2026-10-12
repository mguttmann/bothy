// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// TestGetMe (test 9).
func TestGetMe(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	me := w.want(w.do("owner", http.MethodGet, "/api/v1/me", "", ""), http.StatusOK, "")
	p := me["principal"].(map[string]any)
	want := w.clk.Now().Add(5 * time.Minute)
	until, _ := me["step_up_valid_until"].(string)
	got, err := time.Parse(time.RFC3339Nano, until)
	if p["id"] != owner.String() || p["kind"] != "user" || me["is_owner"] != true || me["email"] != "owner@example.com" ||
		me["locale"] != "de" || me["time_zone"] != "UTC" || me["session_id"] == nil || me["device_id"] == nil ||
		err != nil || !got.Equal(want) || me["permission_version"] != float64(w.permissionVersion()) {
		t.Fatalf("me %v (want step-up until %s)", me, want)
	}
	w.session("old", owner, authz.DeviceActive, w.clk.Now().Add(-10*time.Minute))
	if old := w.want(w.do("old", http.MethodGet, "/api/v1/me", "", ""), http.StatusOK, ""); old["step_up_valid_until"] != nil {
		t.Fatalf("old step-up reported: %v", old)
	}
	w.agent("lotse")
	a := w.want(w.do("lotse", http.MethodGet, "/api/v1/me", "", ""), http.StatusOK, "")
	if a["email"] != nil || a["is_owner"] != false || a["principal"].(map[string]any)["kind"] != "agent" || a["locale"] != nil {
		t.Fatalf("agent me %v", a)
	}
	alex := w.person("alex", "active", false)
	w.session("pending-device", alex, authz.DevicePending, time.Time{})
	w.want(w.do("pending-device", http.MethodGet, "/api/v1/me", "", ""), http.StatusOK, "")
	w.want(w.do("pending-device", http.MethodGet, usersPath, "", ""), http.StatusForbidden, "device_pending")
	w.want(w.do("", http.MethodGet, "/api/v1/me", "", ""), http.StatusUnauthorized, "unauthenticated")
}

// TestUpdateMyProfile (test 10).
func TestUpdateMyProfile(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alex := w.person("alex", "active", false)
	u := w.want(w.do("alex", http.MethodPatch, "/api/v1/me/profile", `{"display_name":"Alex A","locale":"en-gb","time_zone":"Europe/Berlin"}`, ""), http.StatusOK, "")
	if u["display_name"] != "Alex A" || u["locale"] != "en-GB" || u["time_zone"] != "Europe/Berlin" || u["handle"] != "alex" {
		t.Fatalf("user %v", u)
	}
	ev := w.events("user.updated")
	if len(ev) != 1 || ev[0].ResourceID != alex.String() || ev[0].Actor != alex.String() {
		t.Fatalf("events %+v", ev)
	}
	if len(w.audits("self.profile.update")) != 0 {
		t.Fatal("self.profile.update was audited")
	}
	// Without change: no event.
	w.want(w.do("alex", http.MethodPatch, "/api/v1/me/profile", `{"display_name":"Alex A"}`, ""), http.StatusOK, "")
	if len(w.events("user.updated")) != 1 {
		t.Fatal("a profile update without change emitted an event")
	}
	w.agent("lotse")
	p := w.want(w.do("lotse", http.MethodPatch, "/api/v1/me/profile", `{"display_name":"Bot"}`, ""), http.StatusForbidden, "forbidden")
	if p["reason"] != "subject_kind_not_allowed" {
		t.Fatalf("agent reason %v", p["reason"])
	}
	for _, body := range []string{`{"time_zone":"Local"}`, `{"locale":"--"}`, `{"display_name":"  "}`, `{"display_name":"A\u202eB"}`} {
		w.want(w.do("alex", http.MethodPatch, "/api/v1/me/profile", body, ""), http.StatusBadRequest, "validation_failed")
	}
}

// TestUpdateUser: an owner changes another person's profile; the handle never changes (no such
// member); audit with the changed fields; without change no event.
func TestUpdateUser(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.person("owner", "active", true)
	alex := w.person("alex", "active", false)
	u := w.want(w.do("owner", http.MethodPatch, userPath(alex), `{"display_name":"Alex B","email":"ab@example.com"}`, ""), http.StatusOK, "")
	if u["display_name"] != "Alex B" || u["handle"] != "alex" {
		t.Fatalf("user %v", u)
	}
	a := w.audits("user.update")
	if len(a) != 1 || !strings.Contains(a[0].Raw, `"changed_fields": ["display_name", "email"]`) {
		t.Fatalf("audit %+v", a)
	}
	w.want(w.do("owner", http.MethodPatch, userPath(alex), `{"display_name":"Alex B"}`, ""), http.StatusOK, "")
	if len(w.events("user.updated")) != 1 {
		t.Fatal("an update without change emitted an event")
	}
	w.want(w.do("owner", http.MethodPatch, userPath(alex), `{"handle":"x"}`, ""), http.StatusBadRequest, "validation_failed")
	w.want(w.do("owner", http.MethodPatch, userPath(ids.New()), `{"display_name":"X"}`, ""), http.StatusNotFound, "not_found")
	w.want(w.do("owner", http.MethodPatch, userPath(alex), `{"email":"owner@example.com"}`, ""), http.StatusConflict, "conflict")
}

// TestListUsers (test 11).
func TestListUsers(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.person("owner", "active", true)
	for _, h := range []string{"anna", "ben", "carl", "dora"} {
		w.person(h, "active", false)
	}
	w.person("pia", "pending", false)
	w.sql(`UPDATE principals SET display_name = 'x%_y' WHERE handle = 'carl'`)
	page := func(path string) (handles []string, next string) {
		t.Helper()
		m := w.want(w.do("anna", http.MethodGet, path, "", ""), http.StatusOK, "")
		if _, ok := m["total"]; ok {
			t.Error("page with a total")
		}
		for _, it := range m["items"].([]any) {
			handles = append(handles, it.(map[string]any)["handle"].(string))
		}
		next, _ = m["next_cursor"].(string)
		return handles, next
	}
	first, next := page(usersPath)
	if strings.Join(first, ",") != "anna,ben" || next == "" {
		t.Fatalf("first page %v %q", first, next)
	}
	w.person("aaron", "active", false) // sorts before the cursor: not on the next pages
	w.person("eve", "active", false)
	second, next := page(usersPath + "?cursor=" + url.QueryEscape(next))
	if strings.Join(second, ",") != "carl,dora" {
		t.Fatalf("second page %v", second)
	}
	third, next := page(usersPath + "?cursor=" + url.QueryEscape(next))
	if strings.Join(third, ",") != "eve,owner" || next == "" {
		t.Fatalf("third page %v", third)
	}
	if last, n := page(usersPath + "?cursor=" + url.QueryEscape(next)); strings.Join(last, ",") != "pia" || n != "" {
		t.Fatalf("last page %v %q", last, n)
	}
	// A cursor with another filter is 400 cursor_invalid.
	_, c := page(usersPath + "?status=active")
	w.want(w.do("anna", http.MethodGet, usersPath+"?status=pending&cursor="+url.QueryEscape(c), "", ""), http.StatusBadRequest, "cursor_invalid")
	foreign, err := httpserver.EncodeCursor("devices", []byte("x"), map[string]string{"id": ids.New().String()})
	if err != nil {
		t.Fatal(err)
	}
	w.want(w.do("anna", http.MethodGet, usersPath+"?cursor="+url.QueryEscape(foreign), "", ""), http.StatusBadRequest, "cursor_invalid")
	// q: prefix on handle and display name, % and _ literal.
	if h, _ := page(usersPath + "?q=B"); strings.Join(h, ",") != "ben" {
		t.Fatalf("q=B %v", h)
	}
	if h, _ := page(usersPath + "?q=" + url.QueryEscape("x%_")); strings.Join(h, ",") != "carl" {
		t.Fatalf("q=x%%_ %v", h)
	}
	if h, _ := page(usersPath + "?q=" + url.QueryEscape("%")); len(h) != 0 {
		t.Fatalf("q=%% matched %v", h)
	}
	if h, _ := page(usersPath + "?q=_"); len(h) != 0 {
		t.Fatalf("q=_ matched %v", h)
	}
	if h, _ := page(usersPath + "?status=pending"); strings.Join(h, ",") != "pia" {
		t.Fatalf("status=pending %v", h)
	}
	if h, _ := page(usersPath + "?limit=1000"); len(h) != 3 {
		t.Fatalf("limit capped: %v", h)
	}
	w.agent("lotse")
	w.want(w.do("lotse", http.MethodGet, usersPath, "", ""), http.StatusForbidden, "forbidden")
	w.want(w.do("lotse", http.MethodGet, ownersPath, "", ""), http.StatusForbidden, "forbidden")
}

// TestGetUser (test 12): an unknown id and an agent id give the same 404 body.
func TestGetUser(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alex := w.person("alex", "active", false)
	bot := w.agent("lotse")
	if u := w.want(w.do("alex", http.MethodGet, userPath(alex), "", ""), http.StatusOK, ""); u["id"] != alex.String() || u["email"] != nil {
		t.Fatalf("user %v", u)
	}
	unknown := w.do("alex", http.MethodGet, userPath(ids.New()), "", "")
	agent := w.do("alex", http.MethodGet, userPath(bot), "", "")
	w.want(unknown, http.StatusNotFound, "not_found")
	w.want(agent, http.StatusNotFound, "not_found")
	if strip(unknown.Problem(t)) != strip(agent.Problem(t)) {
		t.Fatalf("bodies differ:\n%s\n%s", unknown.Body, agent.Body)
	}
	// An agent sees no person.
	w.want(w.do("lotse", http.MethodGet, userPath(alex), "", ""), http.StatusNotFound, "not_found")
}

// TestIdempotency (test 13).
func TestIdempotency(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	key := ids.New().String()
	body := `{"handle":"newbie","display_name":"Newbie"}`
	first := w.want(w.do("owner", http.MethodPost, usersPath, body, key), http.StatusCreated, "")
	r := w.do("owner", http.MethodPost, usersPath, body, key)
	again := w.want(r, http.StatusCreated, "")
	if r.Header.Get("Idempotent-Replay") != "true" || again["user"].(map[string]any)["id"] != first["user"].(map[string]any)["id"] {
		t.Fatalf("replay %v %v", r.Header, again)
	}
	if w.count(`SELECT count(*) FROM principals WHERE handle = 'newbie'`) != 1 {
		t.Fatal("the replay created a second person")
	}
	w.want(w.do("owner", http.MethodPost, usersPath, `{"handle":"other","display_name":"Other"}`, key), http.StatusUnprocessableEntity, "idempotency_key_reuse")
	// 401 step_up_required is not stored: the same key succeeds after the step-up.
	w.session("stale", owner, authz.DeviceActive, time.Time{})
	key2 := ids.New().String()
	w.want(w.do("stale", http.MethodPost, usersPath, `{"handle":"later","display_name":"Later"}`, key2), http.StatusUnauthorized, "step_up_required")
	stale := w.actors.Get("stale")
	stale.StepUpAt = w.clk.Now()
	w.actors.Set("stale", stale)
	w.want(w.do("stale", http.MethodPost, usersPath, `{"handle":"later","display_name":"Later"}`, key2), http.StatusCreated, "")
	// Rights changes behind the Idempotency-Key stage: addOwner and revokeOwner with a key.
	alex := w.person("alex", "active", false)
	w.want(w.do("owner", http.MethodPost, ownersPath, `{"user_id":"`+alex.String()+`","reason":"Deputy"}`, ids.New().String()), http.StatusCreated, "")
	w.want(w.do("owner", http.MethodPost, ownersPath+"/"+alex.String()+"/revoke", `{"reason":"Done"}`, ids.New().String()), http.StatusNoContent, "")
	k := ids.New().String()
	w.want(w.do("owner", http.MethodPost, ownersPath+"/"+owner.String()+"/revoke", `{"reason":"x"}`, k), http.StatusConflict, "last_owner")
	w.want(w.do("owner", http.MethodPost, ownersPath+"/"+owner.String()+"/revoke", `{"reason":"x"}`, k), http.StatusConflict, "last_owner")
	pend := w.person("pia", "pending", false)
	w.want(w.do("owner", http.MethodPost, userPath(pend)+"/enable", `{"reason":"ok"}`, ids.New().String()), http.StatusOK, "")
}

// TestNoPersonalDataInAuditOrLogs (test 14): audit entries of the five owner actions carry the
// details of scope 6, and no audit details and no captured log record contain a test display
// name or e-mail address.
//
// The access log of platform/otel writes to slog.Default, so the test captures the default logger
// as well (it changes a global and therefore does not run in parallel with itself).
func TestNoPersonalDataInAuditOrLogs(t *testing.T) {
	defaultLog := &httpservertest.LogCapture{}
	prev := slog.Default()
	slog.SetDefault(slog.New(defaultLog))
	t.Cleanup(func() { slog.SetDefault(prev) })
	w := newWorld(t)
	w.person("owner", "active", true)
	secretName, secretMail := "Zelda Quartz", "zelda.quartz@example.com"
	u := w.want(w.do("owner", http.MethodPost, usersPath, `{"handle":"zelda","display_name":"`+secretName+`","email":"`+secretMail+`","locale":"fr"}`, ""), http.StatusCreated, "")
	id, err := ids.Parse(u["user"].(map[string]any)["id"].(string))
	if err != nil {
		t.Fatal(err)
	}
	w.want(w.do("owner", http.MethodPatch, userPath(id), `{"display_name":"Zelda Opal","email":"z.opal@example.com"}`, ""), http.StatusOK, "")
	w.sql(`UPDATE principals SET status = 'disabled' WHERE id = $1`, id.String())
	w.want(w.do("owner", http.MethodPost, userPath(id)+"/enable", `{"reason":"Back"}`, ""), http.StatusOK, "")
	w.want(w.do("owner", http.MethodPost, ownersPath, `{"user_id":"`+id.String()+`","reason":"Deputy"}`, ""), http.StatusCreated, "")
	w.want(w.do("owner", http.MethodPost, ownersPath+"/"+id.String()+"/revoke", `{"reason":"Done"}`, ""), http.StatusNoContent, "")
	// Denials too.
	w.want(w.do("owner", http.MethodPost, usersPath, `{"handle":"zelda","display_name":"`+secretName+`"}`, ""), http.StatusConflict, "conflict")
	want := map[string]string{
		"user.create": `"ip_key_id"`, "user.update": `"changed_fields"`, "user.enable": `"from_status": "disabled"`,
		"owner.add": `"reason": "Deputy"`, "owner.remove": `"reason": "Done"`,
	}
	rows := w.audits("")
	for _, r := range rows {
		if r.Action == "user.create" && r.Decision == "allow" && len(r.Details) != 2 {
			t.Errorf("user.create details beyond the recorder's members: %s", r.Raw)
		}
	}
	for action, frag := range want {
		found := false
		for _, r := range rows {
			if r.Action == action && r.Decision == "allow" && strings.Contains(r.Raw, frag) {
				found = true
			}
		}
		if !found {
			t.Errorf("no allow entry of %s with %s in %+v", action, frag, rows)
		}
	}
	for _, r := range rows {
		for _, s := range []string{"Zelda", "zelda", "example.com", "Opal"} {
			if strings.Contains(r.Raw, s) {
				t.Errorf("audit details of %s contain %q: %s", r.Action, s, r.Raw)
			}
		}
	}
	records := append(append(w.h.Logs.Records(), w.logs.Records()...), defaultLog.Records()...)
	if !defaultLog.Has("http request") {
		t.Fatal("the access log was not captured; the log search would prove nothing")
	}
	for _, rec := range records {
		line := rec.Message + " " + strings.Join(mapValues(rec.Attrs), " ")
		for _, s := range []string{"Zelda", "zelda.quartz", "z.opal", "Opal"} {
			if strings.Contains(line, s) {
				t.Errorf("log record contains %q: %s", s, line)
			}
		}
	}
}

func mapValues(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

// TestPersonalTokenIsNoOwnerForReads: listOwners and getUser need no owner; a personal token
// reads the directory (people see it).
func TestPersonalTokenReadsDirectory(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	w.token("pat", owner, actor.KindUser)
	w.want(w.do("pat", http.MethodGet, ownersPath, "", ""), http.StatusOK, "")
	w.want(w.do("pat", http.MethodGet, userPath(owner), "", ""), http.StatusOK, "")
}
