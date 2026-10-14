// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestSpaceEvents: space.created, space.updated (update and move), space.archived and
// sidebar.updated have the envelope of 04 section 2.2: resource space, scope the space, no null,
// the resource version of the space for the space events and none for sidebar.updated, whose
// payload carries the version of the row and whose audience is the caller.
func TestSpaceEvents(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	support := w.create("Support", `"color":"moss"`)
	w.want(w.do("owner", http.MethodPatch, spacePath(support), `{"name":"Help"}`, append(key(), ifMatch(1)...)...), http.StatusOK, "")
	w.want(w.move(support, `{}`), http.StatusOK, "")
	w.want(w.do("owner", http.MethodPut, sidebarPath(support), `{"collapsed":true}`), http.StatusOK, "")
	w.want(w.do("owner", http.MethodPost, spacePath(support)+"/archive", `{"reason":"Done"}`, key()...), http.StatusOK, "")
	for _, c := range []struct {
		typ      string
		versions []int64
	}{{"space.created", []int64{1}}, {"space.updated", []int64{2, 3}}, {"space.archived", []int64{4}}} {
		evs := w.events(c.typ)
		if len(evs) != len(c.versions) {
			t.Fatalf("%s: %d events", c.typ, len(evs))
		}
		for i, e := range evs {
			sp, _ := e.Payload["space"].(map[string]any)
			if e.ResourceKind != "space" || e.ResourceID != support.String() || e.SpaceID != support.String() || !e.HasVersion ||
				e.Version != c.versions[i] || e.ReceiveAction != "space.read" || e.Audience != "" || sp == nil || sp["version"] != float64(c.versions[i]) {
				t.Errorf("%s %d: %+v", c.typ, i, e)
			}
		}
	}
	evs := w.events("sidebar.updated")
	if len(evs) != 1 {
		t.Fatalf("sidebar.updated: %d", len(evs))
	}
	e := evs[0]
	st, _ := e.Payload["state"].(map[string]any)
	if e.Audience != w.ownerID.String() || e.ReceiveAction != "self" || e.ResourceKind != "space" || e.ResourceID != support.String() ||
		e.HasVersion || st == nil || st["version"] != float64(1) || st["collapsed"] != true {
		t.Fatalf("sidebar.updated %+v", e)
	}
}

// TestSidebarPut: last write wins across two devices with updated_by_device_id; use_global_order
// removes the key; both fields and an invalid key are 400; an agent token is 403; an invisible
// space is 404; repeating the same PUT gives the same state.
func TestSidebarPut(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alex := w.person("alex", false)
	phone := w.session("alex-phone", alex, time.Time{})
	support, marketing := w.create("Support"), w.create("Marketing")
	w.grant(alex, support, "read")
	w.agent("lotse")
	m := w.want(w.do("alex", http.MethodPut, sidebarPath(support), `{"collapsed":true,"sort_key":"a0V"}`), http.StatusOK, "")
	if m["collapsed"] != true || m["sort_key"] != "a0V" || m["version"] != float64(1) || m["updated_by_device_id"] == nil {
		t.Fatalf("first put %v", m)
	}
	m = w.want(w.do("alex-phone", http.MethodPut, sidebarPath(support), `{"collapsed":false}`), http.StatusOK, "")
	if m["collapsed"] != false || m["sort_key"] != "a0V" || m["version"] != float64(2) || m["updated_by_device_id"] != phone.String() {
		t.Fatalf("second device %v", m)
	}
	again := w.want(w.do("alex-phone", http.MethodPut, sidebarPath(support), `{"collapsed":false}`), http.StatusOK, "")
	if again["collapsed"] != false || again["sort_key"] != "a0V" {
		t.Fatalf("repeat %v", again)
	}
	m = w.want(w.do("alex", http.MethodPut, sidebarPath(support), `{"collapsed":false,"use_global_order":true}`), http.StatusOK, "")
	if _, has := m["sort_key"]; has {
		t.Fatalf("use_global_order kept the key: %v", m)
	}
	p := w.want(w.do("alex", http.MethodPut, sidebarPath(support), `{"collapsed":false,"sort_key":"a1","use_global_order":true}`), http.StatusBadRequest, "validation_failed")
	if pointer(p) != "/use_global_order" {
		t.Fatalf("both: pointer %q", pointer(p))
	}
	p = w.want(w.do("alex", http.MethodPut, sidebarPath(support), `{"collapsed":false,"sort_key":"a00"}`), http.StatusBadRequest, "validation_failed")
	if pointer(p) != "/sort_key" {
		t.Fatalf("invalid key: pointer %q", pointer(p))
	}
	p = w.want(w.do("lotse", http.MethodPut, sidebarPath(support), `{"collapsed":true}`), http.StatusForbidden, "forbidden")
	if p["reason"] != "subject_kind_not_allowed" {
		t.Fatalf("agent: %v", p)
	}
	w.want(w.do("alex", http.MethodPut, sidebarPath(marketing), `{"collapsed":true}`), http.StatusNotFound, "not_found")
	w.want(w.do("alex", http.MethodPut, sidebarPath(ids.New()), `{"collapsed":true}`), http.StatusNotFound, "not_found")
	if n := w.count(`SELECT count(*) FROM user_space_states`); n != 1 {
		t.Fatalf("%d rows", n)
	}
	if a := w.allows("sidebar.update"); len(a) != 0 {
		t.Fatalf("sidebar.update is audited: %+v", a)
	}
}

// TestListSidebarStatesPort: the port returns only the caller's rows of visible spaces that are
// not archived.
func TestListSidebarStatesPort(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alex, sam := w.person("alex", false), w.person("sam", false)
	support, marketing, research := w.create("Support"), w.create("Marketing"), w.create("Research")
	w.grant(alex, support, "read")
	w.grant(alex, marketing, "read")
	w.grant(sam, support, "read")
	for _, c := range []struct {
		who   string
		space ids.ID
	}{{"alex", support}, {"alex", marketing}, {"sam", support}, {"owner", research}} {
		w.want(w.do(c.who, http.MethodPut, sidebarPath(c.space), `{"collapsed":true}`), http.StatusOK, "")
	}
	// Alex loses Marketing: a row of an invisible space is left out (the grant row goes away
	// without a rights version, so the cache must not hold Alex yet).
	w.sql(`DELETE FROM grants WHERE subject_principal_id = $1 AND target_space_id = $2`, alex.String(), marketing.String())
	w.want(w.do("owner", http.MethodPost, spacePath(support)+"/archive", `{"reason":"Done"}`, key()...), http.StatusOK, "")
	w.want(w.do("owner", http.MethodPost, spacePath(research)+"/archive", `{"reason":"Done"}`, key()...), http.StatusOK, "")
	alexActor := w.actor("alex")
	got, err := w.svc.ListSidebarStates(context.Background(), alexActor)
	if err != nil || len(got) != 0 {
		t.Fatalf("alex: %+v, %v", got, err)
	}
	w.grant(alex, marketing, "read")
	w.bumpVersion()
	got, err = w.svc.ListSidebarStates(context.Background(), alexActor)
	if err != nil || len(got) != 1 || got[0].SpaceID != marketing || !got[0].Collapsed {
		t.Fatalf("alex with Marketing: %+v, %v", got, err)
	}
	if got, err := w.svc.ListSidebarStates(context.Background(), w.actor("owner")); err != nil || len(got) != 0 {
		t.Fatalf("owner: %+v, %v", got, err)
	}
	if got, err := w.svc.ListSidebarStates(context.Background(), actor.Actor{ID: sam, Kind: actor.KindAgent}); err != nil || len(got) != 0 {
		t.Fatalf("agent kind: %+v, %v", got, err)
	}
}

// actor returns the principal registered under token name.
func (w *world) actor(name string) actor.Actor {
	return w.actors.Get(name)
}

// bumpVersion raises permission_version as a fixture, so the snapshots of a raw grant change are
// rebuilt.
func (w *world) bumpVersion() {
	w.sql(`UPDATE sequence_counters SET value = value + 1 WHERE name = 'permission_version'`)
}
