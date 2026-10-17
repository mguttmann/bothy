// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// listIDs pages through path as c (limit 3) and returns the ids of all items.
func (w *world) listIDs(c *client, path string) []string {
	w.t.Helper()
	var out []string
	cursor := ""
	for range 10 {
		p := path + sep(path) + "limit=3"
		if cursor != "" {
			p += "&cursor=" + cursor
		}
		m := w.want(c.call(http.MethodGet, p, ""), http.StatusOK, "")
		if _, ok := m["total"]; ok {
			w.t.Fatal("a list carries a total")
		}
		for _, it := range m["items"].([]any) {
			out = append(out, it.(map[string]any)["id"].(string))
		}
		next, _ := m["next_cursor"].(string)
		if next == "" {
			return out
		}
		cursor = next
	}
	w.t.Fatal("more than ten pages")
	return nil
}

// sep is the separator before the next query parameter of path.
func sep(path string) string {
	if slices.Contains([]byte(path), '?') {
		return "&"
	}
	return "?"
}

// strs converts ids to their text.
func strs(list ...ids.ID) []string {
	out := make([]string, 0, len(list))
	for _, id := range list {
		out = append(out, id.String())
	}
	return out
}

// TestGrants_ListAsOwner: the owner lists every grant with each filter; a half filter pair is 400
// with the pointer of the missing parameter; include_revoked adds revoked and expired grants;
// cursors are bound to the filters.
func TestGrants_ListAsOwner(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	a := w.boss.actor()
	inst := w.grantAs(a, "user", w.robin, "instance", ids.ID{}, "read")
	kamp := w.grantAs(a, "user", w.robin, "channel", w.kampagnen, "write")
	if got := w.listIDs(w.boss, "/api/v1/grants"); !slices.Equal(got, strs(w.g1, w.g2, w.g4, inst, kamp)) {
		t.Fatalf("all grants %v", got)
	}
	for _, c := range []struct {
		query string
		want  []string
	}{
		{"target_kind=space&target_id=" + w.support.String(), strs(w.g1)},
		{"target_kind=channel&target_id=" + w.kampagnen.String(), strs(kamp)},
		{"target_kind=instance", strs(inst)},
		{"subject_kind=group&subject_id=" + w.leser.String(), strs(w.g4)},
		{"subject_kind=user&subject_id=" + w.robin.String(), strs(inst, kamp)},
		{"subject_kind=user&subject_id=" + w.robin.String() + "&target_kind=instance", strs(inst)},
	} {
		if got := w.listIDs(w.boss, "/api/v1/grants?"+c.query); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.query, got, c.want)
		}
	}
	for _, c := range []struct{ query, pointer, code string }{
		{"target_id=" + w.support.String(), "#query/target_kind", "required"},
		{"target_kind=space", "#query/target_id", "required"},
		{"target_kind=instance&target_id=" + w.support.String(), "#query/target_id", "forbidden"},
		{"subject_kind=user", "#query/subject_id", "required"},
		{"subject_id=" + w.sam.String(), "#query/subject_kind", "required"},
	} {
		w.wantField(w.boss.call(http.MethodGet, "/api/v1/grants?"+c.query, ""), c.pointer, c.code)
	}
	// Revoked and expired grants only with include_revoked.
	w.want(w.boss.call(http.MethodPost, revokePath(kamp), `{"reason":"left"}`), http.StatusOK, "")
	soon := w.grantAsExpiring(w.robin, w.eskalation, w.clk.Now().Add(time.Minute))
	w.clk.Advance(time.Minute)
	robin := "/api/v1/grants?subject_kind=user&subject_id=" + w.robin.String()
	if got := w.listIDs(w.boss, robin); !slices.Equal(got, strs(inst)) {
		t.Fatalf("in effect %v", got)
	}
	if got := w.listIDs(w.boss, robin+"&include_revoked=true"); !slices.Equal(got, strs(inst, kamp, soon)) {
		t.Fatalf("with revoked and expired %v", got)
	}
	// A cursor of one filter does not work for another or for another caller.
	m := w.want(w.boss.call(http.MethodGet, "/api/v1/grants?limit=2", ""), http.StatusOK, "")
	cursor := m["next_cursor"].(string)
	w.want(w.boss.call(http.MethodGet, robin+"&cursor="+cursor, ""), http.StatusBadRequest, "cursor_invalid")
	w.want(w.boss.call(http.MethodGet, "/api/v1/grants?include_revoked=true&cursor="+cursor, ""), http.StatusBadRequest, "cursor_invalid")
	w.want(w.manager(w.alex, "trigger").call(http.MethodGet, "/api/v1/grants?cursor="+cursor, ""), http.StatusBadRequest, "cursor_invalid")
	w.want(w.boss.call(http.MethodGet, "/api/v1/grants?limit=2&cursor="+cursor, ""), http.StatusOK, "")
}

// grantAsExpiring creates a read grant for user on channel that expires at exp.
func (w *world) grantAsExpiring(user, channel ids.ID, exp time.Time) ids.ID {
	w.t.Helper()
	body := `{"subject_kind":"user","subject_id":"` + user.String() + `","target_kind":"channel","target_id":"` + channel.String() +
		`","level":"read","reason":"short","expires_at":"` + exp.UTC().Format(time.RFC3339) + `"}`
	return idOf(w.t, w.want(w.boss.call(http.MethodPost, "/api/v1/grants", body), http.StatusCreated, ""))
}

// TestGrants_ManageScope (07 section 2.6 item 6): Alex with manage on Support sees only the grants
// on Support and its channels; a filter narrows and never widens; a target he cannot read is 404,
// a readable target without manage 403; grants outside answer like random ids (AZ-P4).
func TestGrants_ManageScope(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	a := w.boss.actor()
	inst := w.grantAs(a, "user", w.sam, "instance", ids.ID{}, "read")
	esk := w.grantAs(a, "user", w.sam, "channel", w.eskalation, "read")
	alex := w.manager(w.alex, "trigger")
	all := w.listIDs(w.boss, "/api/v1/grants")
	mine := all[len(all)-1] // the manage grant of Alex
	if got := w.listIDs(alex, "/api/v1/grants"); !slices.Equal(got, append(strs(w.g1, w.g4, esk), mine)) {
		t.Fatalf("Alex sees %v", got)
	}
	if got := w.listIDs(alex, "/api/v1/grants?subject_kind=user&subject_id="+w.sam.String()); !slices.Equal(got, strs(esk)) {
		t.Fatalf("Alex with the filter on Sam sees %v (not Marketing, not the instance)", got)
	}
	if got := w.listIDs(alex, "/api/v1/grants?target_kind=channel&target_id="+w.allgemein.String()); !slices.Equal(got, strs(w.g4)) {
		t.Fatalf("Alex on Allgemein sees %v", got)
	}
	w.want(alex.call(http.MethodGet, "/api/v1/grants?target_kind=space&target_id="+w.marketing.String(), ""), http.StatusNotFound, "not_found")
	w.want(alex.call(http.MethodGet, "/api/v1/grants?target_kind=channel&target_id="+w.kampagnen.String(), ""), http.StatusNotFound, "not_found")
	w.wantReason(alex.call(http.MethodGet, "/api/v1/grants?target_kind=instance", ""), http.StatusForbidden, "forbidden", "owner_only")
	sam := w.login(w.sam)
	w.wantReason(sam.call(http.MethodGet, "/api/v1/grants?target_kind=channel&target_id="+w.allgemein.String(), ""), http.StatusForbidden, "forbidden", "owner_only")
	if got := w.listIDs(sam, "/api/v1/grants"); len(got) != 0 {
		t.Fatalf("Sam without manage sees %v", got)
	}
	// Reads of single grants.
	w.want(alex.call(http.MethodGet, grantPath(w.g4), ""), http.StatusOK, "")
	random := ids.New()
	for _, g := range []ids.ID{w.g2, inst} {
		w.sameNotFound(alex.call(http.MethodGet, grantPath(g), ""), alex.call(http.MethodGet, grantPath(random), ""))
		w.sameNotFound(alex.call(http.MethodPost, revokePath(g), `{"reason":"x"}`), alex.call(http.MethodPost, revokePath(random), `{"reason":"x"}`))
	}
	w.sameNotFound(sam.call(http.MethodGet, grantPath(w.g4), ""), sam.call(http.MethodGet, grantPath(random), ""))
	m := w.want(w.boss.call(http.MethodGet, grantPath(inst), ""), http.StatusOK, "")
	if _, ok := m["target_id"]; ok || m["target_kind"] != "instance" || m["level"] != "read" {
		t.Fatalf("instance grant %v", m)
	}
	if rows := w.audits("grant.read", 0); !slices.ContainsFunc(rows, func(r auditRow) bool { return r.Decision == "deny" }) {
		t.Fatalf("no deny entry of grant.read: %+v", rows)
	}
}

// TestGrants_CreateAndRevoke: create with grant.changed, access.changed for the subject or the
// members of a subject group and the audit details; revoke twice is 200 with one event; shape
// errors 400; an unknown subject is 400 only after authorize.
func TestGrants_CreateAndRevoke(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	seq, aseq := w.eventSeq(), w.auditSeq()
	exp := w.clk.Now().Add(48 * time.Hour)
	body := `{"subject_kind":"user","subject_id":"` + w.robin.String() + `","target_kind":"channel","target_id":"` + w.eskalation.String() +
		`","level":"write","capabilities":["upload"],"reason":" Vertretung ","expires_at":"` + exp.Format(time.RFC3339) + `"}`
	m := w.want(w.boss.call(http.MethodPost, "/api/v1/grants", body), http.StatusCreated, "")
	g := idOf(t, m)
	if m["reason"] != "Vertretung" || m["granted_by"] != w.ownerID.String() || m["level"] != "write" {
		t.Fatalf("grant %v", m)
	}
	evs := w.wantEvents(seq, "grant.changed:created", "access.changed@"+w.robin.String())
	for _, e := range evs {
		if e.Type == "grant.changed" && (e.Space != w.support.String() || e.Channel != w.eskalation.String() || e.Receive != "grant.read" || e.Actor != w.ownerID.String()) {
			t.Fatalf("grant.changed envelope %+v", e)
		}
	}
	d := w.wantAllow("grant.create", aseq)
	for k, v := range map[string]any{"subject_kind": "user", "subject_id": w.robin.String(), "target_kind": "channel", "target_id": w.eskalation.String(),
		"level": "write", "reason": "Vertretung"} {
		if d[k] != v {
			t.Errorf("audit %s = %v, want %v", k, d[k], v)
		}
	}
	if d["capabilities"] == nil || d["expires_at"] == nil {
		t.Errorf("audit details %v", d)
	}
	// A grant to a group affects its members in effect.
	seq = w.eventSeq()
	w.want(w.boss.call(http.MethodPost, "/api/v1/grants", grantBody("group", w.leser, "channel", w.eskalation, "read")), http.StatusCreated, "")
	w.wantEvents(seq, "grant.changed:created", "access.changed@"+w.sam.String())

	seq, aseq = w.eventSeq(), w.auditSeq()
	r := w.want(w.boss.call(http.MethodPost, revokePath(g), `{"reason":"done"}`), http.StatusOK, "")
	if r["revoke_reason"] != "done" || r["revoked_by"] != w.ownerID.String() || r["revoked_at"] == nil {
		t.Fatalf("revoked %v", r)
	}
	w.wantEvents(seq, "grant.changed:revoked", "access.changed@"+w.robin.String())
	w.wantAllow("grant.revoke", aseq)
	seq, aseq = w.eventSeq(), w.auditSeq()
	again := w.want(w.boss.call(http.MethodPost, revokePath(g), `{"reason":"again"}`), http.StatusOK, "")
	if again["revoke_reason"] != "done" {
		t.Fatalf("second revocation changed the record: %v", again)
	}
	w.wantEvents(seq)
	if d := w.wantAllow("grant.revoke", aseq); d["no_change"] != true {
		t.Fatalf("second revocation audit %v", d)
	}
	w.wantField(w.boss.call(http.MethodPost, revokePath(w.g2), `{"reason":"  "}`), "/reason", "required")
	// The reason of a revocation is checked like the reason of a grant (red team M1-48, minor 2).
	w.wantField(w.boss.call(http.MethodPost, revokePath(w.g2), `{"reason":"\u200b"}`), "/reason", "required")
	w.want(w.boss.call(http.MethodPost, revokePath(w.g2), `{"reason":"ok\u202e"}`), http.StatusBadRequest, "validation_failed")

	// Shape errors come first.
	for _, c := range []struct{ body, pointer, code string }{
		{`{"subject_kind":"user","subject_id":"` + w.robin.String() + `","target_kind":"space","target_id":"` + w.support.String() +
			`","level":"read","reason":"x","expires_at":"` + w.clk.Now().Format(time.RFC3339) + `"}`, "/expires_at", "in_past"},
		{grantBody("user", w.robin, "instance", w.support, "read"), "/target_id", "forbidden"},
		{grantBody("user", w.robin, "space", ids.ID{}, "read"), "/target_id", "required"},
		{grantBody("user", w.robin, "space", w.support, "read", "upload", "upload"), "/capabilities", "duplicate"},
		{`{"subject_kind":"user","subject_id":"` + w.robin.String() + `","target_kind":"space","target_id":"` + w.support.String() + `","level":"read","reason":" "}`,
			"/reason", "required"},
	} {
		w.wantField(w.boss.call(http.MethodPost, "/api/v1/grants", c.body), c.pointer, c.code)
	}
	w.want(w.boss.call(http.MethodPost, "/api/v1/grants", grantBody("user", w.robin, "channel", ids.New(), "read")), http.StatusNotFound, "not_found")

	// An unknown subject is 400 for the owner, but 404 or 403 for callers without the right.
	unknown := grantBody("user", ids.New(), "channel", w.allgemein, "read")
	w.wantField(w.boss.call(http.MethodPost, "/api/v1/grants", unknown), "/subject_id", "unknown_subject")
	w.wantField(w.boss.call(http.MethodPost, "/api/v1/grants", grantBody("agent", w.sam, "channel", w.allgemein, "read")), "/subject_id", "unknown_subject")
	w.wantField(w.boss.call(http.MethodPost, "/api/v1/grants", grantBody("group", ids.New(), "channel", w.allgemein, "read")), "/subject_id", "unknown_subject")
	robin := w.login(w.robin)
	robin.stepUp()
	w.want(robin.call(http.MethodPost, "/api/v1/grants", unknown), http.StatusNotFound, "not_found")
	sam := w.login(w.sam)
	sam.stepUp()
	w.wantReason(sam.call(http.MethodPost, "/api/v1/grants", unknown), http.StatusForbidden, "forbidden", "owner_only")
}
