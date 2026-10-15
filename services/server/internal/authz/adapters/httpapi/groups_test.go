// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// provider inserts an OIDC identity provider and returns its id.
func (w *world) provider() ids.ID {
	w.t.Helper()
	id := ids.New()
	w.sql(`INSERT INTO identity_providers (id, kind, name, config) VALUES ($1, 'oidc', $2, '{}')`, id.String(), "Corp"+id.String()[:8])
	return id
}

// claimGroup creates a group of the claim value ref of provider through the API.
func (w *world) claimGroup(provider ids.ID, name, ref string) ids.ID {
	w.t.Helper()
	body, _ := json.Marshal(map[string]any{"name": name, "source": "oidc_claim", "provider_id": provider.String(), "external_ref": ref})
	m := w.want(w.boss.call(http.MethodPost, "/api/v1/groups", string(body)), http.StatusCreated, "")
	if m["source"] != "oidc_claim" || m["provider_id"] != provider.String() || m["external_ref"] != ref {
		w.t.Fatalf("claim group %v", m)
	}
	return idOf(w.t, m)
}

// wantEvents checks the event types since after (sorted; access.changed with its audience).
func (w *world) wantEvents(after int64, want ...string) []event {
	w.t.Helper()
	evs := w.events(after)
	slices.Sort(want)
	if got := types(evs); !slices.Equal(got, want) {
		w.t.Fatalf("events %v, want %v", got, want)
	}
	return evs
}

// wantAllow checks that action wrote exactly one allow entry since after and returns its details.
func (w *world) wantAllow(action string, after int64) map[string]any {
	w.t.Helper()
	rows := w.audits(action, after)
	if len(rows) != 1 || rows[0].Decision != "allow" {
		w.t.Fatalf("audit of %s: %+v", action, rows)
	}
	return rows[0].Details
}

// TestGroups_CreateUpdate: create with group.changed and audit, duplicate name 409 on /name, bidi
// name and claim group shapes 400, update with group.changed, update without change audited as
// no_change without an event.
func TestGroups_CreateUpdate(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	seq, aseq := w.eventSeq(), w.auditSeq()
	m := w.want(w.boss.call(http.MethodPost, "/api/v1/groups", `{"name":"  Team Nord ","description":"Tickets"}`), http.StatusCreated, "")
	if m["name"] != "Team Nord" || m["source"] != "local" || m["member_count"] != float64(0) || m["description"] != "Tickets" {
		t.Fatalf("group %v", m)
	}
	group := idOf(t, m)
	evs := w.wantEvents(seq, "group.changed:created")
	if evs[0].Resource != group.String() || evs[0].Actor != w.ownerID.String() || evs[0].Receive != "group.read" || evs[0].Space != "" {
		t.Fatalf("event %+v", evs[0])
	}
	w.wantAllow("group.create", aseq)

	w.wantProblemField(w.boss.call(http.MethodPost, "/api/v1/groups", `{"name":"team nord"}`), http.StatusConflict, "conflict", "/name", "taken")
	w.want(w.boss.call(http.MethodPost, "/api/v1/groups", "{\"name\":\"Te\u202eam\"}"), http.StatusBadRequest, "validation_failed")
	w.wantField(w.boss.call(http.MethodPost, "/api/v1/groups", `{"name":"   "}`), "/name", "required")
	w.wantField(w.boss.call(http.MethodPost, "/api/v1/groups", `{"name":"Viel zu langer Gruppenname"}`), "/name", "too_long")
	p := w.provider()
	w.wantField(w.boss.call(http.MethodPost, "/api/v1/groups", `{"name":"Claim","source":"oidc_claim","provider_id":"`+p.String()+`"}`), "/external_ref", "required")
	w.wantField(w.boss.call(http.MethodPost, "/api/v1/groups", `{"name":"Claim","source":"oidc_claim","external_ref":"staff"}`), "/provider_id", "required")
	w.wantField(w.boss.call(http.MethodPost, "/api/v1/groups", `{"name":"Claim","external_ref":"staff"}`), "/external_ref", "forbidden")
	w.wantField(w.boss.call(http.MethodPost, "/api/v1/groups", `{"name":"Claim","source":"oidc_claim","provider_id":"`+ids.New().String()+`","external_ref":"staff"}`),
		"/provider_id", "invalid")
	w.claimGroup(p, "Staff", "staff")
	w.wantProblemField(w.boss.call(http.MethodPost, "/api/v1/groups", `{"name":"Staff 2","source":"oidc_claim","provider_id":"`+p.String()+`","external_ref":"staff"}`),
		http.StatusConflict, "conflict", "/external_ref", "taken")

	seq, aseq = w.eventSeq(), w.auditSeq()
	m = w.want(w.boss.call(http.MethodPatch, groupPath(group), `{"name":"Team Sued"}`), http.StatusOK, "")
	if m["name"] != "Team Sued" || m["description"] != "Tickets" {
		t.Fatalf("updated %v", m)
	}
	w.wantEvents(seq, "group.changed:updated")
	w.wantAllow("group.update", aseq)
	seq, aseq = w.eventSeq(), w.auditSeq()
	w.want(w.boss.call(http.MethodPatch, groupPath(group), `{"name":"Team Sued"}`), http.StatusOK, "")
	w.wantEvents(seq)
	if d := w.wantAllow("group.update", aseq); d["no_change"] != true {
		t.Fatalf("no_change missing: %v", d)
	}
	w.wantField(w.boss.call(http.MethodPatch, groupPath(group), `{}`), "/name", "required")
	w.want(w.boss.call(http.MethodPatch, groupPath(ids.New()), `{"name":"X"}`), http.StatusNotFound, "not_found")
}

// TestGroups_DeleteRevokesGrants: deleting a group revokes its grants with reason group_deleted,
// keeps the memberships, writes grant.changed per grant, one group.changed and access.changed
// for every member who loses access; a second delete is 204 without change.
func TestGroups_DeleteRevokesGrants(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	group := idOf(t, w.want(w.boss.call(http.MethodPost, "/api/v1/groups", `{"name":"Projekt"}`), http.StatusCreated, ""))
	for _, u := range []ids.ID{w.sam, w.robin} {
		w.want(w.boss.call(http.MethodPost, membersPath(group), `{"user_id":"`+u.String()+`"}`), http.StatusCreated, "")
	}
	g := idOf(t, w.want(w.boss.call(http.MethodPost, "/api/v1/grants", grantBody("group", group, "channel", w.eskalation, "read")), http.StatusCreated, ""))
	// A grant that expired before the deletion is reported as expired, not revoked (red team
	// M1-48, minor 3).
	short := `{"subject_kind":"group","subject_id":"` + group.String() + `","target_kind":"channel","target_id":"` + w.kampagnen.String() +
		`","level":"read","reason":"short","expires_at":"` + w.clk.Now().Add(time.Minute).Format(time.RFC3339) + `"}`
	expired := idOf(t, w.want(w.boss.call(http.MethodPost, "/api/v1/grants", short), http.StatusCreated, ""))
	w.clk.Advance(time.Minute)
	w.boss.stepUp()
	seq, aseq := w.eventSeq(), w.auditSeq()
	w.want(w.boss.call(http.MethodDelete, groupPath(group), ""), http.StatusNoContent, "")
	evs := w.wantEvents(seq, "grant.changed:expired", "grant.changed:revoked", "group.changed:deleted", "access.changed@"+w.sam.String(), "access.changed@"+w.robin.String())
	var expiredReason string
	w.scalar(&expiredReason, `SELECT coalesce(revoke_reason, '') FROM grants WHERE id = $1`, expired.String())
	if expiredReason != "" {
		t.Fatalf("the expired grant was revoked: %q", expiredReason)
	}
	for _, e := range evs {
		if e.Type == "grant.changed" && change(e) == "revoked" && (e.Channel != w.eskalation.String() || e.Space != w.support.String()) {
			t.Errorf("grant.changed scope %+v", e)
		}
	}
	var reason string
	w.scalar(&reason, `SELECT revoke_reason FROM grants WHERE id = $1`, g.String())
	if reason != "group_deleted" {
		t.Fatalf("revoke_reason %q", reason)
	}
	if n := w.count(`SELECT count(*) FROM group_memberships WHERE group_id = $1 AND removed_at IS NULL`, group.String()); n != 2 {
		t.Fatalf("memberships after the deletion: %d, want 2 kept as history", n)
	}
	w.wantAllow("group.delete", aseq)
	w.want(w.boss.call(http.MethodGet, groupPath(group), ""), http.StatusNotFound, "not_found")
	seq = w.eventSeq()
	w.want(w.boss.call(http.MethodDelete, groupPath(group), ""), http.StatusNoContent, "")
	w.wantEvents(seq)
}

// TestGroups_Members: add and remove with events, conflicts and the 400 of unknown users,
// remove twice 204, never a member 404, claim groups 409 claim_managed_group.
func TestGroups_Members(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	seq, aseq := w.eventSeq(), w.auditSeq()
	exp := w.clk.Now().Add(48 * time.Hour)
	m := w.want(w.boss.call(http.MethodPost, membersPath(w.leser), `{"user_id":"`+w.robin.String()+`","expires_at":"`+exp.Format(time.RFC3339)+`"}`),
		http.StatusCreated, "")
	user := m["user"].(map[string]any)
	if user["id"] != w.robin.String() || user["kind"] != "user" || user["handle"] == "" || m["added_by"] != w.ownerID.String() || m["expires_at"] == nil {
		t.Fatalf("member %v", m)
	}
	evs := w.wantEvents(seq, "group.changed:member_added", "access.changed@"+w.robin.String())
	for _, e := range evs {
		if e.Type == "group.changed" && (e.Payload["user_id"] != w.robin.String() || e.Payload["group"].(map[string]any)["member_count"] != float64(2)) {
			t.Fatalf("group.changed %v", e.Payload)
		}
	}
	if d := w.wantAllow("group.member.add", aseq); d["user_id"] != w.robin.String() {
		t.Fatalf("audit %v", d)
	}
	r := w.boss.call(http.MethodPost, membersPath(w.leser), `{"user_id":"`+w.robin.String()+`"}`)
	if r.StatusCode != http.StatusConflict {
		t.Fatalf("second add: %d %s", r.StatusCode, r.Body)
	}
	w.wantField(w.boss.call(http.MethodPost, membersPath(w.leser), `{"user_id":"`+w.lotse.String()+`"}`), "/user_id", "unknown_user")
	w.wantField(w.boss.call(http.MethodPost, membersPath(w.leser), `{"user_id":"`+ids.New().String()+`"}`), "/user_id", "unknown_user")
	w.wantField(w.boss.call(http.MethodPost, membersPath(w.leser), `{"user_id":"`+w.alex.String()+`","expires_at":"`+w.clk.Now().Format(time.RFC3339)+`"}`),
		"/expires_at", "in_past")
	w.want(w.boss.call(http.MethodPost, membersPath(ids.New()), `{"user_id":"`+w.alex.String()+`"}`), http.StatusNotFound, "not_found")

	seq, aseq = w.eventSeq(), w.auditSeq()
	w.want(w.boss.call(http.MethodDelete, memberPath(w.leser, w.robin), ""), http.StatusNoContent, "")
	w.wantEvents(seq, "group.changed:member_removed", "access.changed@"+w.robin.String())
	w.wantAllow("group.member.remove", aseq)
	seq, aseq = w.eventSeq(), w.auditSeq()
	w.want(w.boss.call(http.MethodDelete, memberPath(w.leser, w.robin), ""), http.StatusNoContent, "")
	w.wantEvents(seq)
	if d := w.wantAllow("group.member.remove", aseq); d["no_change"] != true {
		t.Fatalf("second removal audit %v", d)
	}
	w.want(w.boss.call(http.MethodDelete, memberPath(w.leser, w.alex), ""), http.StatusNotFound, "not_found")

	claim := w.claimGroup(w.provider(), "Staff", "staff")
	w.wantReason(w.boss.call(http.MethodPost, membersPath(claim), `{"user_id":"`+w.alex.String()+`"}`), http.StatusConflict, "conflict", "claim_managed_group")
	w.wantReason(w.boss.call(http.MethodDelete, memberPath(claim, w.alex), ""), http.StatusConflict, "conflict", "claim_managed_group")
}

// TestGroups_MemberCountAtTheSecondOfExpiry: member_count and the member list show memberships in
// effect at the request; at expires_at the membership is already over (07 section 2.5).
func TestGroups_MemberCountAtTheSecondOfExpiry(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	exp := w.clk.Now().Add(10 * time.Minute)
	w.want(w.boss.call(http.MethodPost, membersPath(w.leser), `{"user_id":"`+w.robin.String()+`","expires_at":"`+exp.Format(time.RFC3339)+`"}`), http.StatusCreated, "")
	members := func() []string {
		var out []string
		m := w.want(w.boss.call(http.MethodGet, membersPath(w.leser)+"?limit=3", ""), http.StatusOK, "")
		for _, it := range m["items"].([]any) {
			out = append(out, it.(map[string]any)["user"].(map[string]any)["id"].(string))
		}
		return out
	}
	w.clk.Advance(10*time.Minute - time.Second)
	if m := w.want(w.boss.call(http.MethodGet, groupPath(w.leser), ""), http.StatusOK, ""); m["member_count"] != float64(2) || len(members()) != 2 {
		t.Fatalf("one second before the expiry: %v", m)
	}
	w.clk.Advance(time.Second)
	if m := w.want(w.boss.call(http.MethodGet, groupPath(w.leser), ""), http.StatusOK, ""); m["member_count"] != float64(1) {
		t.Fatalf("at the expiry: %v", m)
	}
	if got := members(); len(got) != 1 || got[0] != w.sam.String() {
		t.Fatalf("members at the expiry %v", got)
	}
}

// strip removes the members of a problem that differ per request.
func strip(p map[string]any) string {
	delete(p, "correlation_id")
	delete(p, "instance")
	b, _ := json.Marshal(p)
	return string(b)
}

// sameNotFound checks AZ-P4: the answers for an invisible and for a random id agree in status,
// problem body and headers (without the per-request Correlation-Id and Date).
func (w *world) sameNotFound(a, b *httpservertest.Response) {
	w.t.Helper()
	if a.StatusCode != http.StatusNotFound || b.StatusCode != http.StatusNotFound {
		w.t.Fatalf("status %d and %d, want 404: %s %s", a.StatusCode, b.StatusCode, a.Body, b.Body)
	}
	if strip(a.Problem(w.t)) != strip(b.Problem(w.t)) {
		w.t.Fatalf("bodies differ:\n%s\n%s", a.Body, b.Body)
	}
	for _, h := range []http.Header{a.Header, b.Header} {
		h.Del("Correlation-Id")
		h.Del("Date")
	}
	ja, _ := json.Marshal(a.Header)
	jb, _ := json.Marshal(b.Header)
	if string(ja) != string(jb) {
		w.t.Fatalf("headers differ:\n%s\n%s", ja, jb)
	}
}

// TestGroups_Visibility: the owner lists every live group, Sam only his own, a member filter on
// another person narrows Sam's list to none; Robin gets 404 on Leser, identical to a random id
// (AZ-P4), also for the member list and every write.
func TestGroups_Visibility(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	other := idOf(t, w.want(w.boss.call(http.MethodPost, "/api/v1/groups", `{"name":"Andere"}`), http.StatusCreated, ""))
	ids3 := func(m map[string]any) []string {
		var out []string
		for _, it := range m["items"].([]any) {
			out = append(out, it.(map[string]any)["id"].(string))
		}
		return out
	}
	if got := ids3(w.want(w.boss.call(http.MethodGet, "/api/v1/groups", ""), http.StatusOK, "")); !slices.Equal(got, []string{w.leser.String(), other.String()}) {
		t.Fatalf("owner sees %v", got)
	}
	if got := ids3(w.want(w.boss.call(http.MethodGet, "/api/v1/groups?member_id="+w.sam.String(), ""), http.StatusOK, "")); !slices.Equal(got, []string{w.leser.String()}) {
		t.Fatalf("owner with member filter sees %v", got)
	}
	sam, robin := w.login(w.sam), w.login(w.robin)
	if got := ids3(w.want(sam.call(http.MethodGet, "/api/v1/groups", ""), http.StatusOK, "")); !slices.Equal(got, []string{w.leser.String()}) {
		t.Fatalf("Sam sees %v", got)
	}
	if got := ids3(w.want(sam.call(http.MethodGet, "/api/v1/groups?member_id="+w.alex.String(), ""), http.StatusOK, "")); len(got) != 0 {
		t.Fatalf("Sam with a filter on Alex sees %v", got)
	}
	w.want(sam.call(http.MethodGet, groupPath(w.leser), ""), http.StatusOK, "")
	w.want(sam.call(http.MethodGet, membersPath(w.leser), ""), http.StatusOK, "")
	// A member may read but not write: 403 owner_only.
	w.wantReason(sam.call(http.MethodPatch, groupPath(w.leser), `{"name":"X"}`), http.StatusForbidden, "forbidden", "owner_only")
	if got := ids3(w.want(robin.call(http.MethodGet, "/api/v1/groups", ""), http.StatusOK, "")); len(got) != 0 {
		t.Fatalf("Robin sees %v", got)
	}
	random := ids.New()
	for _, c := range []struct{ method, a, b, body string }{
		{http.MethodGet, groupPath(w.leser), groupPath(random), ""},
		{http.MethodGet, membersPath(w.leser), membersPath(random), ""},
		{http.MethodPatch, groupPath(w.leser), groupPath(random), `{"name":"X"}`},
		{http.MethodDelete, groupPath(w.leser), groupPath(random), ""},
		{http.MethodPost, membersPath(w.leser), membersPath(random), `{"user_id":"` + w.robin.String() + `"}`},
		{http.MethodDelete, memberPath(w.leser, w.sam), memberPath(random, w.sam), ""},
	} {
		w.sameNotFound(robin.call(c.method, c.a, c.body), robin.call(c.method, c.b, c.body))
	}
	// Every denial is in the audit log (AZ-P13 presence).
	if rows := w.audits("group.read", 0); !slices.ContainsFunc(rows, func(r auditRow) bool { return r.Decision == "deny" }) {
		t.Fatalf("no deny entry of group.read: %+v", rows)
	}
}

// TestGroups_ListPages: cursor pages of groups bound to the caller and the filter.
func TestGroups_ListPages(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	var made []string
	made = append(made, w.leser.String())
	for _, n := range []string{"A", "B", "C"} {
		made = append(made, idOf(t, w.want(w.boss.call(http.MethodPost, "/api/v1/groups", `{"name":"`+n+`"}`), http.StatusCreated, "")).String())
	}
	var got []string
	cursor := ""
	for range 4 {
		path := "/api/v1/groups"
		if cursor != "" {
			path += "?cursor=" + cursor
		}
		m := w.want(w.boss.call(http.MethodGet, path, ""), http.StatusOK, "")
		for _, it := range m["items"].([]any) {
			got = append(got, it.(map[string]any)["id"].(string))
		}
		next, _ := m["next_cursor"].(string)
		if next == "" {
			break
		}
		cursor = next
	}
	if !slices.Equal(got, made) {
		t.Fatalf("pages %v, want %v", got, made)
	}
	if _, ok := w.want(w.boss.call(http.MethodGet, "/api/v1/groups", ""), http.StatusOK, "")["total"]; ok {
		t.Fatal("a list carries a total")
	}
	first := w.want(w.boss.call(http.MethodGet, "/api/v1/groups", ""), http.StatusOK, "")["next_cursor"].(string)
	w.want(w.boss.call(http.MethodGet, "/api/v1/groups?member_id="+w.sam.String()+"&cursor="+first, ""), http.StatusBadRequest, "cursor_invalid")
	w.want(w.login(w.sam).call(http.MethodGet, "/api/v1/groups?cursor="+first, ""), http.StatusBadRequest, "cursor_invalid")
}
