// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// sidebarChannelPath is the path of putMySidebarChannel (M4-51).
func sidebarChannelPath(id ids.ID) string { return "/api/v1/me/sidebar/channels/" + id.String() }

// idList returns the JSON array of list.
func idList(list ...ids.ID) string {
	parts := make([]string, 0, len(list))
	for _, id := range list {
		parts = append(parts, `"`+id.String()+`"`)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// strs returns the string items of a decoded JSON array.
func strs(v any) []string {
	raw, _ := v.([]any)
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		s, _ := r.(string)
		out = append(out, s)
	}
	return out
}

// TestSidebarPutChannelOrder (M4-51): channel_order absent keeps the stored list, a list sets it,
// an empty list returns to the default order (absent in the answer); IDs that name no channel are
// stored (no oracle, D-M4-51-8); a duplicate, an item that is not a UUID and a list above
// sidebar.order_max_items are 400 validation_failed on /channel_order; sidebar.updated carries the list.
func TestSidebarPutChannelOrder(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	support := w.create("Support")
	a, b, unknown := ids.New(), ids.New(), ids.New()
	want := []string{b.String(), a.String(), unknown.String()}
	m := w.want(w.do("owner", http.MethodPut, sidebarPath(support), `{"collapsed":false,"channel_order":`+idList(b, a, unknown)+`}`), http.StatusOK, "")
	if !slices.Equal(strs(m["channel_order"]), want) || m["version"] != float64(1) {
		t.Fatalf("set %v", m)
	}
	m = w.want(w.do("owner", http.MethodPut, sidebarPath(support), `{"collapsed":true,"sort_key":"a0V"}`), http.StatusOK, "")
	if !slices.Equal(strs(m["channel_order"]), want) || m["sort_key"] != "a0V" {
		t.Fatalf("keep %v", m)
	}
	m = w.want(w.do("owner", http.MethodPut, sidebarPath(support), `{"collapsed":true,"channel_order":[]}`), http.StatusOK, "")
	if _, has := m["channel_order"]; has || m["sort_key"] != "a0V" {
		t.Fatalf("clear %v", m)
	}
	if n := w.count(`SELECT count(*) FROM user_space_states WHERE channel_order IS NULL`); n != 1 {
		t.Fatalf("an empty list must store NULL: %d rows", n)
	}
	p := w.want(w.do("owner", http.MethodPut, sidebarPath(support), `{"collapsed":true,"channel_order":`+idList(a, a)+`}`), http.StatusBadRequest, "validation_failed")
	if pointer(p) != "/channel_order" {
		t.Fatalf("duplicate: pointer %q", pointer(p))
	}
	p = w.want(w.do("owner", http.MethodPut, sidebarPath(support), `{"collapsed":true,"channel_order":`+idList(ids.New(), ids.New(), ids.New(), ids.New())+`}`),
		http.StatusBadRequest, "validation_failed")
	if pointer(p) != "/channel_order" {
		t.Fatalf("too many: pointer %q", pointer(p))
	}
	p = w.want(w.do("owner", http.MethodPut, sidebarPath(support), `{"collapsed":true,"channel_order":["not-an-id"]}`), http.StatusBadRequest, "validation_failed")
	if !strings.HasPrefix(pointer(p), "/channel_order") {
		t.Fatalf("not a UUID: pointer %q", pointer(p))
	}
	evs := w.events("sidebar.updated")
	if len(evs) != 3 {
		t.Fatalf("sidebar.updated: %d events", len(evs))
	}
	if st, _ := evs[0].Payload["state"].(map[string]any); !slices.Equal(strs(st["channel_order"]), want) {
		t.Fatalf("event payload %v", evs[0].Payload)
	}
}

// TestSidebarPutChannel (M4-51): putMySidebarChannel stores the post order with version + 1 and
// the device of the write, last write wins; an empty list is stored as []; a missing list, a
// duplicate and too many items are 400 validation_failed on /thread_order; an agent token is 403
// subject_kind_not_allowed; a channel the caller cannot read and an unknown one are 404;
// sidebar_channel.updated goes to the caller with resource channel and no resource version; there
// is no audit; the port lists the caller's rows of readable channels that are not archived.
func TestSidebarPutChannel(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alex := w.person("alex", false)
	phone := w.session("alex-phone", alex, time.Time{})
	support, marketing := w.create("Support"), w.create("Marketing")
	w.grant(alex, support, "read")
	w.agent("lotse")
	forum, hidden := w.channel(support, "forum", "bugs"), w.channel(marketing, "forum", "plans")
	p1, p2 := ids.New(), ids.New()
	m := w.want(w.do("alex", http.MethodPut, sidebarChannelPath(forum), `{"thread_order":`+idList(p2, p1)+`}`), http.StatusOK, "")
	if m["channel_id"] != forum.String() || !slices.Equal(strs(m["thread_order"]), []string{p2.String(), p1.String()}) ||
		m["version"] != float64(1) || m["updated_by_device_id"] == nil {
		t.Fatalf("first put %v", m)
	}
	m = w.want(w.do("alex-phone", http.MethodPut, sidebarChannelPath(forum), `{"thread_order":[]}`), http.StatusOK, "")
	if order, ok := m["thread_order"].([]any); !ok || len(order) != 0 || m["version"] != float64(2) || m["updated_by_device_id"] != phone.String() {
		t.Fatalf("second device %v", m)
	}
	for _, body := range []string{`{}`, `{"thread_order":null}`, `{"thread_order":` + idList(p1, p1) + `}`,
		`{"thread_order":` + idList(ids.New(), ids.New(), ids.New(), ids.New()) + `}`, `{"thread_order":["not-an-id"]}`} {
		p := w.want(w.do("alex", http.MethodPut, sidebarChannelPath(forum), body), http.StatusBadRequest, "validation_failed")
		if !strings.HasPrefix(pointer(p), "/thread_order") {
			t.Fatalf("%s: pointer %q", body, pointer(p))
		}
	}
	p := w.want(w.do("lotse", http.MethodPut, sidebarChannelPath(forum), `{"thread_order":[]}`), http.StatusForbidden, "forbidden")
	if p["reason"] != "subject_kind_not_allowed" {
		t.Fatalf("agent: %v", p)
	}
	w.want(w.do("alex", http.MethodPut, sidebarChannelPath(hidden), `{"thread_order":[]}`), http.StatusNotFound, "not_found")
	w.want(w.do("alex", http.MethodPut, sidebarChannelPath(ids.New()), `{"thread_order":[]}`), http.StatusNotFound, "not_found")
	if n := w.count(`SELECT count(*) FROM user_channel_states`); n != 1 {
		t.Fatalf("%d rows", n)
	}
	if a := w.allows("sidebar.update"); len(a) != 0 {
		t.Fatalf("sidebar.update is audited: %+v", a)
	}
	evs := w.events("sidebar_channel.updated")
	if len(evs) != 2 {
		t.Fatalf("sidebar_channel.updated: %d events", len(evs))
	}
	e := evs[0]
	st, _ := e.Payload["state"].(map[string]any)
	if e.Audience != alex.String() || e.ReceiveAction != "self" || e.ResourceKind != "channel" || e.ResourceID != forum.String() ||
		e.SpaceID != support.String() || e.HasVersion || st == nil || st["version"] != float64(1) || st["channel_id"] != forum.String() {
		t.Fatalf("sidebar_channel.updated %+v", e)
	}
	if st, _ := evs[1].Payload["state"].(map[string]any); st == nil || st["thread_order"] == nil {
		t.Fatalf("an empty order must be [] in the payload: %+v", evs[1].Payload)
	}
	// The port: Alex's row of the readable forum; none after the forum is archived.
	got, err := w.svc.ListSidebarChannelStates(context.Background(), w.actor("alex"))
	if err != nil || len(got) != 1 || got[0].ChannelID != forum || got[0].Version != 2 {
		t.Fatalf("port %+v, %v", got, err)
	}
	w.want(w.do("owner", http.MethodPut, sidebarChannelPath(hidden), `{"thread_order":[]}`), http.StatusOK, "")
	if got, err := w.svc.ListSidebarChannelStates(context.Background(), w.actor("alex")); err != nil || len(got) != 1 {
		t.Fatalf("another person's row leaked: %+v, %v", got, err)
	}
	w.sql(`UPDATE channels SET archived_at = now() WHERE id = $1`, forum.String())
	if got, err := w.svc.ListSidebarChannelStates(context.Background(), w.actor("alex")); err != nil || len(got) != 0 {
		t.Fatalf("archived channel listed: %+v, %v", got, err)
	}
}
