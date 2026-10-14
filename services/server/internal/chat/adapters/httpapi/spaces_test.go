// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"fmt"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestMoveSpaceChangesExactlyOneRow: a move writes only the moved row (sort_key and version).
func TestMoveSpaceChangesExactlyOneRow(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	support := w.create("Support")
	w.create("Marketing")
	research := w.create("Research")
	before := w.rows()
	w.want(w.move(research, `{"before_id":"`+support.String()+`"}`), http.StatusOK, "")
	after := w.rows()
	changed := 0
	for i := range before {
		if before[i] == after[i] {
			continue
		}
		changed++
		b, a := before[i], after[i]
		if a.ID != research.String() || a.Version != b.Version+1 || a.SortKey == b.SortKey || a.Name != b.Name {
			t.Errorf("row %+v became %+v", b, a)
		}
	}
	if changed != 1 {
		t.Fatalf("%d rows changed", changed)
	}
	if got := w.order(); !slices.Equal(got, []string{"Research", "Support", "Marketing"}) {
		t.Fatalf("order %v", got)
	}
}

// TestMoveSpacePositions: after_id, before_id and an empty body move as expected; both fields,
// the space itself and an unknown or archived id are 400 with the pointer of the field; an
// archived space is 409.
func TestMoveSpacePositions(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	support, marketing, research := w.create("Support"), w.create("Marketing"), w.create("Research")
	engineering := w.create("Engineering")
	w.want(w.move(support, `{"after_id":"`+research.String()+`"}`), http.StatusOK, "")
	if got := w.order(); !slices.Equal(got, []string{"Marketing", "Research", "Support", "Engineering"}) {
		t.Fatalf("after_id: %v", got)
	}
	w.want(w.move(engineering, `{"before_id":"`+marketing.String()+`"}`), http.StatusOK, "")
	if got := w.order(); !slices.Equal(got, []string{"Engineering", "Marketing", "Research", "Support"}) {
		t.Fatalf("before_id: %v", got)
	}
	w.want(w.move(support, `{}`), http.StatusOK, "")
	if got := w.order(); !slices.Equal(got, []string{"Support", "Engineering", "Marketing", "Research"}) {
		t.Fatalf("empty body: %v", got)
	}
	w.want(w.move(research, `{"after_id":"`+support.String()+`"}`), http.StatusOK, "")
	if got := w.order(); !slices.Equal(got, []string{"Support", "Research", "Engineering", "Marketing"}) {
		t.Fatalf("after the first: %v", got)
	}
	w.want(w.do("owner", http.MethodPost, spacePath(marketing)+"/archive", `{"reason":"Done"}`, key()...), http.StatusOK, "")
	for _, c := range []struct{ body, pointer string }{
		{`{"after_id":"` + support.String() + `","before_id":"` + research.String() + `"}`, "/before_id"},
		{`{"after_id":"` + engineering.String() + `"}`, "/after_id"},
		{`{"before_id":"` + engineering.String() + `"}`, "/before_id"},
		{`{"after_id":"` + ids.New().String() + `"}`, "/after_id"},
		{`{"before_id":"` + marketing.String() + `"}`, "/before_id"},
	} {
		p := w.want(w.move(engineering, c.body), http.StatusBadRequest, "validation_failed")
		if pointer(p) != c.pointer {
			t.Errorf("%s: pointer %q, want %s", c.body, pointer(p), c.pointer)
		}
	}
	w.want(w.move(marketing, `{}`), http.StatusConflict, "conflict")
	w.want(w.move(ids.New(), `{}`), http.StatusNotFound, "not_found")
}

// TestConcurrentMovesKeepDistinctKeys: parallel creates and moves into the same gap end with
// distinct keys and a total order.
func TestConcurrentMovesKeepDistinctKeys(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	first := w.create("Support")
	w.create("Marketing")
	var moved []ids.ID
	for i := range 6 {
		moved = append(moved, w.create(fmt.Sprintf("Team %d", i)))
	}
	var wg sync.WaitGroup
	errs := make(chan string, 20)
	for i, id := range moved {
		wg.Go(func() {
			r := w.move(id, `{"after_id":"`+first.String()+`"}`)
			if r.StatusCode != http.StatusOK {
				errs <- fmt.Sprintf("move %d: %d %s", i, r.StatusCode, r.Body)
			}
		})
		wg.Go(func() {
			r := w.do("owner", http.MethodPost, spacesPath, fmt.Sprintf(`{"name":"New %d","after_space_id":"%s"}`, i, first), key()...)
			if r.StatusCode != http.StatusCreated {
				errs <- fmt.Sprintf("create %d: %d %s", i, r.StatusCode, r.Body)
			}
		})
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	keys := map[string]bool{}
	for _, r := range w.rows() {
		if keys[r.SortKey] {
			t.Fatalf("duplicate key %q", r.SortKey)
		}
		keys[r.SortKey] = true
	}
	order := w.order()
	if len(order) != 14 || order[0] != "Support" || order[13] != "Marketing" {
		t.Fatalf("order %v", order)
	}
}

// TestUpdateSpaceIfMatch: missing If-Match is 428, a stale one 412 with current, a fresh one 200
// with the new ETag.
func TestUpdateSpaceIfMatch(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	id := w.create("Support")
	w.want(w.do("owner", http.MethodPatch, spacePath(id), `{"name":"Help"}`, key()...), http.StatusPreconditionRequired, "precondition_required")
	p := w.want(w.do("owner", http.MethodPatch, spacePath(id), `{"name":"Help"}`, append(key(), ifMatch(7)...)...), http.StatusPreconditionFailed, "version_conflict")
	if cur, _ := p["current"].(map[string]any); cur == nil || cur["name"] != "Support" || cur["version"] != float64(1) {
		t.Fatalf("current %v", p["current"])
	}
	r := w.do("owner", http.MethodPatch, spacePath(id), `{"name":"Help","color":"moss"}`, append(key(), ifMatch(1)...)...)
	m := w.want(r, http.StatusOK, "")
	if m["name"] != "Help" || m["color"] != "moss" || m["version"] != float64(2) || r.Header.Get("ETag") != `"2"` {
		t.Fatalf("updated %v, ETag %s", m, r.Header.Get("ETag"))
	}
	// Without change: 200 with the same version.
	m = w.want(w.do("owner", http.MethodPatch, spacePath(id), `{"name":"Help"}`, append(key(), ifMatch(2)...)...), http.StatusOK, "")
	if m["version"] != float64(2) || len(w.events("space.updated")) != 1 {
		t.Fatalf("no change: %v", m)
	}
}

// TestSpaceColorAndIconArePaletteKeys: hex values, emoji and unknown keys are 400 on /color or
// /icon; moss and a key of the generated icon list succeed, on create and update.
func TestSpaceColorAndIconArePaletteKeys(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	for _, c := range []struct{ member, pointer string }{
		{`"color":"#3366ff"`, "/color"}, {`"color":"😀"`, "/color"}, {`"color":"teal"`, "/color"}, {`"color":""`, "/color"},
		{`"icon":"#3366ff"`, "/icon"}, {`"icon":"😀"`, "/icon"}, {`"icon":"icon.headset"`, "/icon"},
	} {
		p := w.want(w.do("owner", http.MethodPost, spacesPath, `{"name":"Support",`+c.member+`}`, key()...), http.StatusBadRequest, "validation_failed")
		if pointer(p) != c.pointer {
			t.Errorf("%s: pointer %q", c.member, pointer(p))
		}
	}
	id := w.create("Support", `"color":"moss"`, `"icon":"headset"`)
	p := w.want(w.do("owner", http.MethodPatch, spacePath(id), `{"icon":"😀"}`, append(key(), ifMatch(1)...)...), http.StatusBadRequest, "validation_failed")
	if pointer(p) != "/icon" {
		t.Fatalf("update pointer %q", pointer(p))
	}
	w.want(w.do("owner", http.MethodPatch, spacePath(id), `{"color":""}`, append(key(), ifMatch(1)...)...), http.StatusBadRequest, "validation_failed")
	m := w.want(w.do("owner", http.MethodPatch, spacePath(id), `{"icon":"flask","color":"heather"}`, append(key(), ifMatch(1)...)...), http.StatusOK, "")
	if m["icon"] != "flask" || m["color"] != "heather" {
		t.Fatalf("updated %v", m)
	}
}

// TestSECT07_SpaceNameRejectsControlAndBidi: names with U+202E or U+0007 are 400 on /name.
func TestSECT07_SpaceNameRejectsControlAndBidi(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	for _, n := range []string{`Sup\u202eport`, `Sup\u0007`} {
		p := w.want(w.do("owner", http.MethodPost, spacesPath, `{"name":"`+n+`"}`, key()...), http.StatusBadRequest, "validation_failed")
		if pointer(p) != "/name" {
			t.Errorf("%s: pointer %q", n, pointer(p))
		}
	}
	id := w.create("Support")
	w.want(w.do("owner", http.MethodPatch, spacePath(id), `{"name":"Help\u202e"}`, append(key(), ifMatch(1)...)...), http.StatusBadRequest, "validation_failed")
	if w.count(`SELECT count(*) FROM spaces`) != 1 {
		t.Fatal("a refused name was stored")
	}
}

// TestSpaceNameLimitAndUniqueness: a name above space.name_max_chars is 400, a trimmed name is
// stored, support after Support is 409 on create and rename, and a name is free again after the
// archive.
func TestSpaceNameLimitAndUniqueness(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.want(w.do("owner", http.MethodPost, spacesPath, `{"name":"Engineering13"}`, key()...), http.StatusBadRequest, "validation_failed")
	w.want(w.do("owner", http.MethodPost, spacesPath, `{"name":"   "}`, key()...), http.StatusBadRequest, "validation_failed")
	m := w.want(w.do("owner", http.MethodPost, spacesPath, `{"name":"  Support  "}`, key()...), http.StatusCreated, "")
	support := ids.MustParse(m["id"].(string))
	if m["name"] != "Support" {
		t.Fatalf("name %v", m["name"])
	}
	p := w.want(w.do("owner", http.MethodPost, spacesPath, `{"name":"support"}`, key()...), http.StatusConflict, "conflict")
	if pointer(p) != "/name" {
		t.Fatalf("pointer %q", pointer(p))
	}
	research := w.create("Research")
	w.want(w.do("owner", http.MethodPatch, spacePath(research), `{"name":"SUPPORT"}`, append(key(), ifMatch(1)...)...), http.StatusConflict, "conflict")
	w.want(w.do("owner", http.MethodPost, spacePath(support)+"/archive", `{"reason":"Merged"}`, key()...), http.StatusOK, "")
	w.create("support")
}

// TestSpaceReadWithReadOnOneChannelOnly: Sam has read on one channel of Support and nothing on the
// space: getSpace is 200 with level read and no capabilities, and Support is listed. The channel is
// real since M1-53.
func TestSpaceReadWithReadOnOneChannelOnly(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	sam := w.person("sam", false)
	support := w.create("Support")
	w.create("Marketing")
	ch := w.channel(support, "text", "tickets")
	w.sql(`INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, target_channel_id, level, capabilities, reason, granted_by)
		VALUES ($1, 'user', $2, 'channel', $3, 'read', '{}', 'fixture', $2)`, ids.New().String(), sam.String(), ch.String())
	m := w.want(w.do("sam", http.MethodGet, spacePath(support), ""), http.StatusOK, "")
	if m["level"] != "read" || len(m["capabilities"].([]any)) != 0 {
		t.Fatalf("view %v", m)
	}
	if got := w.listed("sam"); !slices.Equal(got, []string{"Support"}) {
		t.Fatalf("listed %v", got)
	}
}

// TestSpaceCreatedVisibleWithoutRightsVersion: Robin reads the instance and has a cached snapshot;
// a new space appears in listSpaces and getSpace while permission_version stays.
func TestSpaceCreatedVisibleWithoutRightsVersion(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	robin := w.person("robin", false)
	w.grant(robin, ids.ID{}, "read")
	w.create("Support")
	if got := w.listed("robin"); !slices.Equal(got, []string{"Support"}) {
		t.Fatalf("listed %v", got)
	}
	v := w.permissionVersion()
	research := w.create("Research")
	if got := w.listed("robin"); !slices.Equal(got, []string{"Support", "Research"}) {
		t.Fatalf("listed after create %v", got)
	}
	m := w.want(w.do("robin", http.MethodGet, spacePath(research), ""), http.StatusOK, "")
	if m["level"] != "read" {
		t.Fatalf("view %v", m)
	}
	if got := w.listed("owner"); !slices.Equal(got, []string{"Support", "Research"}) {
		t.Fatalf("owner listed %v", got)
	}
	if w.permissionVersion() != v {
		t.Fatal("create raised the rights version")
	}
}

// TestSpaceArchiveIsRightsChange: Alex has trigger on Support and Sam read; archiving with
// step-up raises permission_version once, writes access.changed for Alex and the owner with a
// changed entry at level read (none for Sam), space.archived, SpaceArchived and the audit entry
// with the reason; afterwards getSpace answers all three, listSpaces omits Support, updateSpace
// and moveSpace are 409, archiving again is 200 without a write, and without step-up it is 401.
func TestSpaceArchiveIsRightsChange(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alex, sam := w.person("alex", false), w.person("sam", false)
	w.session("owner-stale", w.ownerID, start.Add(-time.Hour))
	support := w.create("Support")
	w.create("Marketing")
	w.grant(alex, support, "trigger")
	w.grant(sam, support, "read")
	w.manage(sam, support)
	w.want(w.do("owner-stale", http.MethodPost, spacePath(support)+"/archive", `{"reason":"Team left"}`, key()...), http.StatusUnauthorized, "")
	w.want(w.do("owner", http.MethodPost, spacePath(support)+"/archive", `{"reason":" "}`, key()...), http.StatusBadRequest, "validation_failed")
	v := w.permissionVersion()
	m := w.want(w.do("owner", http.MethodPost, spacePath(support)+"/archive", `{"reason":"Team left"}`, key()...), http.StatusOK, "")
	if m["archived_at"] == nil || m["version"] != float64(2) {
		t.Fatalf("archived %v", m)
	}
	if w.permissionVersion() != v+1 {
		t.Fatalf("permission_version %d, want %d", w.permissionVersion(), v+1)
	}
	changed := map[string]map[string]any{}
	for _, e := range w.events("access.changed") {
		list, _ := e.Payload["changed"].([]any)
		for _, c := range list {
			entry := c.(map[string]any)
			if entry["id"] == support.String() {
				changed[e.Audience] = entry
			}
		}
	}
	if len(changed) != 2 || changed[alex.String()] == nil || changed[w.ownerID.String()] == nil {
		t.Fatalf("access.changed for %v", changed)
	}
	for who, entry := range changed {
		if entry["kind"] != "space" || entry["level"] != "read" || entry["previous_level"] != "trigger" {
			t.Errorf("%s: %v", who, entry)
		}
	}
	if evs := w.events("space.archived"); len(evs) != 1 || evs[0].Version != 2 || evs[0].SpaceID != support.String() {
		t.Fatalf("space.archived %+v", evs)
	}
	if len(*w.archived) != 1 || (*w.archived)[0].SpaceID != support {
		t.Fatalf("SpaceArchived %+v", *w.archived)
	}
	if a := w.allows("space.archive"); len(a) != 1 || a[0].Details["reason"] != "Team left" {
		t.Fatalf("audit %+v", a)
	}
	for _, who := range []string{"owner", "alex", "sam"} {
		got := w.want(w.do(who, http.MethodGet, spacePath(support), ""), http.StatusOK, "")
		if got["level"] != "read" || got["space"].(map[string]any)["archived_at"] == nil {
			t.Errorf("%s: %v", who, got)
		}
		if l := w.listed(who); slices.Contains(l, "Support") {
			t.Errorf("%s lists %v", who, l)
		}
	}
	w.want(w.do("owner", http.MethodPatch, spacePath(support), `{"name":"Help"}`, append(key(), ifMatch(2)...)...), http.StatusConflict, "conflict")
	w.want(w.do("sam", http.MethodPatch, spacePath(support), `{"name":"Help"}`, append(key(), ifMatch(2)...)...), http.StatusConflict, "conflict")
	w.want(w.move(support, `{}`), http.StatusConflict, "conflict")
	again := w.want(w.do("owner", http.MethodPost, spacePath(support)+"/archive", `{"reason":"Again"}`, key()...), http.StatusOK, "")
	if again["version"] != float64(2) || w.permissionVersion() != v+1 || len(w.events("space.archived")) != 1 {
		t.Fatalf("archive again %v", again)
	}
}

// TestSpaceOperationsRights: create and move are owner only (403 for a visible non-owner); update
// with manage on the space succeeds, with read only 403, without read 404; owner create needs no
// step-up; every allowed mutation has one audit entry.
func TestSpaceOperationsRights(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alex, sam := w.person("alex", false), w.person("sam", false)
	w.person("robin", false)
	w.session("owner-nostep", w.ownerID, time.Time{})
	support := w.create("Support")
	marketing := w.create("Marketing")
	w.manage(alex, support)
	w.grant(sam, support, "read")
	w.want(w.do("alex", http.MethodPost, spacesPath, `{"name":"Research"}`, key()...), http.StatusForbidden, "forbidden")
	w.want(w.do("alex", http.MethodPost, spacePath(support)+"/move", `{}`, key()...), http.StatusForbidden, "forbidden")
	w.want(w.do("alex", http.MethodPost, spacePath(support)+"/archive", `{"reason":"No"}`, key()...), http.StatusForbidden, "forbidden")
	m := w.want(w.do("alex", http.MethodPatch, spacePath(support), `{"name":"Help"}`, append(key(), ifMatch(1)...)...), http.StatusOK, "")
	if m["name"] != "Help" {
		t.Fatalf("manage update %v", m)
	}
	w.want(w.do("sam", http.MethodPatch, spacePath(support), `{"name":"Desk"}`, append(key(), ifMatch(2)...)...), http.StatusForbidden, "forbidden")
	w.want(w.do("robin", http.MethodPatch, spacePath(support), `{"name":"Desk"}`, append(key(), ifMatch(2)...)...), http.StatusNotFound, "not_found")
	w.want(w.do("robin", http.MethodGet, spacePath(support), ""), http.StatusNotFound, "not_found")
	w.want(w.do("robin", http.MethodGet, spacePath(ids.New()), ""), http.StatusNotFound, "not_found")
	w.want(w.do("", http.MethodGet, spacesPath, ""), http.StatusUnauthorized, "")
	w.want(w.do("owner-nostep", http.MethodPost, spacesPath, `{"name":"Research"}`, key()...), http.StatusCreated, "")
	w.want(w.move(marketing, `{}`), http.StatusOK, "")
	if a := w.allows("space.create"); len(a) != 3 {
		t.Errorf("space.create audit %d", len(a))
	}
	if a := w.allows("space.update"); len(a) != 1 || a[0].ResourceID != support.String() {
		t.Errorf("space.update audit %+v", a)
	}
	if a := w.allows("space.reorder"); len(a) != 1 || a[0].ResourceID != marketing.String() {
		t.Errorf("space.reorder audit %+v", a)
	}
	if got := w.listed("alex"); !slices.Equal(got, []string{"Help"}) {
		t.Fatalf("alex lists %v", got)
	}
	v := w.want(w.do("alex", http.MethodGet, spacePath(support), ""), http.StatusOK, "")
	if v["level"] != "read" || !slices.Contains(v["capabilities"].([]any), any("manage")) {
		t.Fatalf("alex view %v", v)
	}
}

// TestConcurrentFirstCreatesKeepDistinctKeys: parallel creates on an instance without spaces have
// no row to lock; the order lock keeps their keys distinct (docs/DECISIONS.md M1-52).
func TestConcurrentFirstCreatesKeepDistinctKeys(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			r := w.do("owner", http.MethodPost, spacesPath, fmt.Sprintf(`{"name":"Team %d"}`, i), key()...)
			if r.StatusCode != http.StatusCreated {
				t.Errorf("create %d: %d %s", i, r.StatusCode, r.Body)
			}
		})
	}
	wg.Wait()
	keys := map[string]bool{}
	for _, r := range w.rows() {
		if keys[r.SortKey] {
			t.Fatalf("duplicate key %q", r.SortKey)
		}
		keys[r.SortKey] = true
	}
	if len(keys) != 8 {
		t.Fatalf("%d spaces", len(keys))
	}
}
