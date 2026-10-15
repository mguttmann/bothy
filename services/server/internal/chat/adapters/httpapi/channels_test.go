// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// channelPath and spaceChannelsPath are the paths of the channel operations.
func channelPath(id ids.ID) string          { return "/api/v1/channels/" + id.String() }
func spaceChannelsPath(space ids.ID) string { return spacePath(space) + "/channels" }
func tagsPath(ch ids.ID) string             { return channelPath(ch) + "/tags" }
func tagPath(ch, tag ids.ID) string         { return tagsPath(ch) + "/" + tag.String() }

// channel creates a channel through the API as the owner and returns its id.
func (w *world) channel(space ids.ID, kind, name string, extra ...string) ids.ID {
	w.t.Helper()
	body := `{"kind":"` + kind + `","name":"` + name + `"`
	for _, e := range extra {
		body += "," + e
	}
	m := w.want(w.do("owner", http.MethodPost, spaceChannelsPath(space), body+"}", key()...), http.StatusCreated, "")
	return ids.MustParse(m["id"].(string))
}

// channelGrant inserts a grant of level with caps to the person subject on the channel ch, the
// fixture of a grant that existed before (it must precede the first request of the subject).
func (w *world) channelGrant(subject, ch ids.ID, level, caps string) {
	w.t.Helper()
	w.sql(`INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, target_channel_id, level, capabilities, reason, granted_by)
		VALUES ($1, 'user', $2, 'channel', $3, $4, $5::text[], 'fixture', $2)`, ids.New().String(), subject.String(), ch.String(), level, caps)
}

// channelNames returns the channel names of listSpaceChannels for token name.
func (w *world) channelNames(name string, space ids.ID) []string {
	w.t.Helper()
	m := w.want(w.do(name, http.MethodGet, spaceChannelsPath(space), ""), http.StatusOK, "")
	var out []string
	for _, it := range m["items"].([]any) {
		out = append(out, it.(map[string]any)["channel"].(map[string]any)["name"].(string))
	}
	return out
}

// channelVersion returns channels.version of ch.
func (w *world) channelVersion(ch ids.ID) int {
	w.t.Helper()
	return w.count(`SELECT version::int FROM channels WHERE id = $1`, ch.String())
}

// TestCreateChannel_TextHasExactlyOneMainThread (R-DM-11): a text channel has exactly one main
// thread created with it, named in main_thread_id; a second main thread violates the unique index.
func TestCreateChannel_TextHasExactlyOneMainThread(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	support := w.create("Support")
	m := w.want(w.do("owner", http.MethodPost, spaceChannelsPath(support), `{"kind":"text","name":"  tickets "}`, key()...), http.StatusCreated, "")
	ch := ids.MustParse(m["id"].(string))
	if m["name"] != "tickets" || m["topic"] != "" || m["version"] != float64(1) || m["settings_version"] != float64(1) || m["read_only"] != false {
		t.Fatalf("channel %v", m)
	}
	if op := m["operator"].(map[string]any); op["mode"] != "inherit" || len(op) != 1 {
		t.Fatalf("operator %v", op)
	}
	var mainID string
	if err := w.owner.QueryRow(context.Background(), `SELECT id::text FROM threads WHERE channel_id = $1 AND kind = 'main'`, ch.String()).Scan(&mainID); err != nil {
		t.Fatal(err)
	}
	if m["main_thread_id"] != mainID || w.count(`SELECT count(*)::int FROM threads WHERE channel_id = $1`, ch.String()) != 1 {
		t.Fatalf("main thread %v, response %v", mainID, m["main_thread_id"])
	}
	if w.count(`SELECT count(*)::int FROM threads WHERE channel_id = $1 AND title = '' AND title_source = 'auto' AND state = 'open'
		AND operator_agent_id IS NULL AND parent_message_id IS NULL AND created_by = $2`, ch.String(), w.ownerID.String()) != 1 {
		t.Fatal("main thread columns differ from R-DM-11")
	}
	if w.count(`SELECT history_backfill_limit FROM channels WHERE id = $1`, ch.String()) != chatConfig.HistoryBackfillLimitDefault {
		t.Fatal("history_backfill_limit is not channel.history_backfill_limit_default")
	}
	_, err := w.owner.Exec(context.Background(), `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`,
		ids.New().String(), ch.String(), w.ownerID.String())
	if err == nil || !strings.Contains(err.Error(), "23505") {
		t.Fatalf("second main thread: %v", err)
	}
}

// TestCreateChannel_ForumHasNoMainThread: a forum has no main thread and keeps its sort.
func TestCreateChannel_ForumHasNoMainThread(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	support := w.create("Support")
	m := w.want(w.do("owner", http.MethodPost, spaceChannelsPath(support), `{"kind":"forum","name":"bugs","forum_default_sort":"created"}`, key()...),
		http.StatusCreated, "")
	if _, ok := m["main_thread_id"]; ok || m["forum_default_sort"] != "created" || m["kind"] != "forum" {
		t.Fatalf("forum %v", m)
	}
	if w.count(`SELECT count(*)::int FROM threads`) != 0 {
		t.Fatal("a forum got a thread")
	}
	if len(w.events("thread.created")) != 0 {
		t.Fatal("thread.created for a forum")
	}
	p := w.want(w.do("owner", http.MethodPost, spaceChannelsPath(support), `{"kind":"text","name":"x","forum_default_sort":"created"}`, key()...),
		http.StatusBadRequest, "validation_failed")
	if pointer(p) != "/forum_default_sort" {
		t.Fatalf("problem %v", p)
	}
	w.want(w.do("owner", http.MethodPost, spaceChannelsPath(support), `{"kind":"dm","name":"x"}`, key()...), http.StatusBadRequest, "validation_failed")
}

// TestCreateChannel_Rights: manage on the space gives 201, write without manage 403, no read on
// the space 404 identical to an unknown space id.
func TestCreateChannel_Rights(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alex, sam := w.person("alex", false), w.person("sam", false)
	w.person("robin", false)
	support := w.create("Support")
	w.manage(alex, support)
	w.grant(sam, support, "write")
	w.want(w.do("alex", http.MethodPost, spaceChannelsPath(support), `{"kind":"text","name":"tickets"}`, key()...), http.StatusCreated, "")
	w.want(w.do("sam", http.MethodPost, spaceChannelsPath(support), `{"kind":"text","name":"other"}`, key()...), http.StatusForbidden, "forbidden")
	hidden := w.do("robin", http.MethodPost, spaceChannelsPath(support), `{"kind":"text","name":"other"}`, key()...)
	unknown := w.do("robin", http.MethodPost, spaceChannelsPath(ids.New()), `{"kind":"text","name":"other"}`, key()...)
	w.want(hidden, http.StatusNotFound, "not_found")
	same404(t, hidden, unknown)
}

// same404 compares two 404 answers after removing correlation_id and instance (AZ-P4).
func same404(t *testing.T, a, b interface {
	Problem(testing.TB) map[string]any
}) {
	t.Helper()
	pa, pb := a.Problem(t), b.Problem(t)
	for _, p := range []map[string]any{pa, pb} {
		delete(p, "correlation_id")
		delete(p, "instance")
	}
	ja, _ := json.Marshal(pa)
	jb, _ := json.Marshal(pb)
	if string(ja) != string(jb) {
		t.Fatalf("404 bodies differ:\n%s\n%s", ja, jb)
	}
}

// TestCreateChannel_DuplicateNameCaseInsensitive409: a name taken case-insensitively in the space
// is 409; it is free again once the first channel is archived; another space may use it.
func TestCreateChannel_DuplicateNameCaseInsensitive409(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	support, research := w.create("Support"), w.create("Research")
	first := w.channel(support, "text", "tickets")
	p := w.want(w.do("owner", http.MethodPost, spaceChannelsPath(support), `{"kind":"forum","name":"TICKETS"}`, key()...), http.StatusConflict, "conflict")
	if pointer(p) != "/name" {
		t.Fatalf("problem %v", p)
	}
	w.channel(research, "text", "tickets")
	w.want(w.do("owner", http.MethodPost, channelPath(first)+"/archive", `{"reason":"Moved"}`, key()...), http.StatusOK, "")
	w.channel(support, "text", "Tickets")
}

// TestCreateChannel_ArchivedSpace409: no channel in an archived space.
func TestCreateChannel_ArchivedSpace409(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	support := w.create("Support")
	w.want(w.do("owner", http.MethodPost, spacePath(support)+"/archive", `{"reason":"Done"}`, key()...), http.StatusOK, "")
	w.want(w.do("owner", http.MethodPost, spaceChannelsPath(support), `{"kind":"text","name":"tickets"}`, key()...), http.StatusConflict, "conflict")
}

// TestCreateChannel_EventsAuditAndEnvelope: channel.created then thread.created in seq order, the
// scope and resource versions, the receive actions of 05 section 6, one audit allow entry and no
// JSON null in the stored payloads (events() fails on null).
func TestCreateChannel_EventsAuditAndEnvelope(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	support := w.create("Support")
	ch := w.channel(support, "text", "tickets", `"topic":"Customer"`)
	var types []string
	rs, err := w.owner.Query(context.Background(), `SELECT type FROM event_log WHERE channel_id = $1 ORDER BY seq`, ch.String())
	if err != nil {
		t.Fatal(err)
	}
	for rs.Next() {
		var s string
		if err := rs.Scan(&s); err != nil {
			t.Fatal(err)
		}
		types = append(types, s)
	}
	rs.Close()
	if !slices.Equal(types, []string{"channel.created", "thread.created"}) {
		t.Fatalf("events %v", types)
	}
	created := w.events("channel.created")
	if len(created) != 1 {
		t.Fatalf("channel.created %+v", created)
	}
	c := created[0]
	if c.ResourceKind != "channel" || c.ResourceID != ch.String() || c.Version != 1 || c.ReceiveAction != "channel.read" || c.SpaceID != support.String() {
		t.Fatalf("channel.created %+v", c)
	}
	if body := c.Payload["channel"].(map[string]any); body["topic"] != "Customer" || body["space_id"] != support.String() {
		t.Fatalf("payload %v", body)
	}
	th := w.events("thread.created")
	if len(th) != 1 || th[0].ResourceKind != "thread" || th[0].Version != 1 || th[0].ReceiveAction != "thread.read" || th[0].SpaceID != support.String() {
		t.Fatalf("thread.created %+v", th)
	}
	thread := th[0].Payload["thread"].(map[string]any)
	if thread["kind"] != "main" || thread["channel_id"] != ch.String() || len(thread["tag_ids"].([]any)) != 0 {
		t.Fatalf("thread payload %v", thread)
	}
	if w.count(`SELECT count(*)::int FROM event_log WHERE type = 'thread.created' AND thread_id = $1`, th[0].ResourceID) != 1 {
		t.Fatal("thread.created without thread_id")
	}
	a := w.allows("channel.create")
	if len(a) != 1 || a[0].ResourceID != ch.String() || a[0].Details["space_id"] != support.String() || a[0].Details["kind"] != "text" {
		t.Fatalf("audit %+v", a)
	}
}

// TestNewChannelVisibleWithoutNewPermissionVersion (04 sections 7.2 and 8.1): Alex reads Support
// with a warm snapshot; a channel created afterwards is readable and listed without a new
// permission_version, its event row carries space_id and channel_id, and Scope for Alex lists
// Support as fully visible.
func TestNewChannelVisibleWithoutNewPermissionVersion(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alex := w.person("alex", false)
	support := w.create("Support")
	w.grant(alex, support, "read")
	w.want(w.do("alex", http.MethodGet, spacePath(support), ""), http.StatusOK, "")
	v := w.permissionVersion()
	ch := w.channel(support, "text", "tickets")
	if w.permissionVersion() != v {
		t.Fatal("channel.create raised permission_version")
	}
	m := w.want(w.do("alex", http.MethodGet, channelPath(ch), ""), http.StatusOK, "")
	if m["level"] != "read" {
		t.Fatalf("view %v", m)
	}
	if got := w.channelNames("alex", support); !slices.Equal(got, []string{"tickets"}) {
		t.Fatalf("listed %v", got)
	}
	if w.count(`SELECT count(*)::int FROM event_log WHERE type = 'channel.created' AND space_id = $1 AND channel_id = $2`, support.String(), ch.String()) != 1 {
		t.Fatal("channel.created without space_id and channel_id")
	}
	sc, err := w.az.Scope(context.Background(), w.actors.Get("alex"), authorize.ActionChannelRead)
	if err != nil || !slices.Contains(sc.FullyVisibleSpaceIDs, support) {
		t.Fatalf("scope %+v, %v", sc, err)
	}
}

// TestListSpaceChannels_FiltersInvisibleAndArchived: a grant on one channel lists only that
// channel; archived channels are absent; level and capabilities per item; the order is the
// sort order.
func TestListSpaceChannels_FiltersInvisibleAndArchived(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	sam := w.person("sam", false)
	support := w.create("Support")
	a := w.channel(support, "text", "alpha")
	w.channel(support, "forum", "beta")
	gone := w.channel(support, "text", "gamma")
	w.channelGrant(sam, a, "write", "{upload}")
	w.want(w.do("owner", http.MethodPost, channelPath(gone)+"/archive", `{"reason":"Old"}`, key()...), http.StatusOK, "")
	if got := w.channelNames("owner", support); !slices.Equal(got, []string{"alpha", "beta"}) {
		t.Fatalf("owner lists %v", got)
	}
	m := w.want(w.do("sam", http.MethodGet, spaceChannelsPath(support), ""), http.StatusOK, "")
	items := m["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("sam lists %v", items)
	}
	it := items[0].(map[string]any)
	if it["channel"].(map[string]any)["id"] != a.String() || it["level"] != "write" || len(it["capabilities"].([]any)) != 1 {
		t.Fatalf("item %v", it)
	}
	own := w.want(w.do("owner", http.MethodGet, spaceChannelsPath(support), ""), http.StatusOK, "")
	first := own["items"].([]any)[0].(map[string]any)
	if first["level"] != "trigger" || len(first["capabilities"].([]any)) == 0 {
		t.Fatalf("owner item %v", first)
	}
	w.person("robin", false)
	w.want(w.do("robin", http.MethodGet, spaceChannelsPath(support), ""), http.StatusNotFound, "not_found")
}

// TestGetChannel_ForeignAndUnknownIdentical404 (AZ-P4 style): status, headers and body equal
// after removing correlation_id and instance.
func TestGetChannel_ForeignAndUnknownIdentical404(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.person("robin", false)
	support := w.create("Support")
	ch := w.channel(support, "text", "tickets")
	a := w.do("robin", http.MethodGet, channelPath(ch), "")
	b := w.do("robin", http.MethodGet, channelPath(ids.New()), "")
	w.want(a, http.StatusNotFound, "not_found")
	w.want(b, http.StatusNotFound, "not_found")
	for _, h := range []string{"Content-Type", "Cache-Control", "ETag"} {
		if a.Header.Get(h) != b.Header.Get(h) {
			t.Fatalf("header %s: %q vs %q", h, a.Header.Get(h), b.Header.Get(h))
		}
	}
	same404(t, a, b)
	got := w.do("owner", http.MethodGet, channelPath(ch), "")
	w.want(got, http.StatusOK, "")
	if got.Header.Get("ETag") != `"1"` {
		t.Fatalf("ETag %q", got.Header.Get("ETag"))
	}
}

// TestUpdateChannel_IfMatch: 428 without If-Match, 412 with current on a stale one, 200 with
// version + 1 and channel.updated; no change gives no event; the notification default can be
// cleared; text limits and an empty body are 400.
func TestUpdateChannel_IfMatch(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	support := w.create("Support")
	ch := w.channel(support, "text", "tickets")
	path := channelPath(ch)
	w.want(w.do("owner", http.MethodPatch, path, `{"name":"desk"}`, key()...), http.StatusPreconditionRequired, "precondition_required")
	p := w.want(w.do("owner", http.MethodPatch, path, `{"name":"desk"}`, append(key(), ifMatch(7)...)...), http.StatusPreconditionFailed, "version_conflict")
	if cur, _ := p["current"].(map[string]any); cur == nil || cur["version"] != float64(1) {
		t.Fatalf("current %v", p)
	}
	m := w.want(w.do("owner", http.MethodPatch, path, `{"name":"desk","default_notification_level":"mentions"}`, append(key(), ifMatch(1)...)...),
		http.StatusOK, "")
	if m["version"] != float64(2) || m["name"] != "desk" || m["default_notification_level"] != "mentions" {
		t.Fatalf("updated %v", m)
	}
	if evs := w.events("channel.updated"); len(evs) != 1 || evs[0].Version != 2 {
		t.Fatalf("channel.updated %+v", evs)
	}
	w.want(w.do("owner", http.MethodPatch, path, `{"name":"desk"}`, append(key(), ifMatch(2)...)...), http.StatusOK, "")
	if evs := w.events("channel.updated"); len(evs) != 1 || w.channelVersion(ch) != 2 {
		t.Fatalf("no-op wrote: %d events, version %d", len(evs), w.channelVersion(ch))
	}
	m = w.want(w.do("owner", http.MethodPatch, path, `{"clear_default_notification_level":true}`, append(key(), ifMatch(2)...)...), http.StatusOK, "")
	if _, ok := m["default_notification_level"]; ok || m["version"] != float64(3) {
		t.Fatalf("cleared %v", m)
	}
	w.want(w.do("owner", http.MethodPatch, path, `{"default_notification_level":"all","clear_default_notification_level":true}`,
		append(key(), ifMatch(3)...)...), http.StatusBadRequest, "validation_failed")
	w.want(w.do("owner", http.MethodPatch, path, `{}`, append(key(), ifMatch(3)...)...), http.StatusBadRequest, "validation_failed")
	w.want(w.do("owner", http.MethodPatch, path, `{"name":"`+strings.Repeat("x", chatConfig.ChannelNameMaxChars+1)+`"}`,
		append(key(), ifMatch(3)...)...), http.StatusBadRequest, "validation_failed")
	p = w.want(w.do("owner", http.MethodPatch, path, `{"topic":"a\u202eb"}`, append(key(), ifMatch(3)...)...), http.StatusBadRequest, "validation_failed")
	if pointer(p) != "/topic" {
		t.Fatalf("problem %v", p)
	}
	if a := w.allows("channel.update"); len(a) < 3 {
		t.Fatalf("audit %+v", a)
	}
}

// TestMoveChannel_ChangesExactlyOneRow: a move writes the moved row only; the order follows;
// both neighbours and a foreign neighbour are 400; a hidden channel is 404.
func TestMoveChannel_ChangesExactlyOneRow(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	support, research := w.create("Support"), w.create("Research")
	a, b, c := w.channel(support, "text", "a"), w.channel(support, "text", "b"), w.channel(support, "text", "c")
	other := w.channel(research, "text", "x")
	before := map[ids.ID]int{a: w.channelVersion(a), b: w.channelVersion(b), other: w.channelVersion(other)}
	m := w.want(w.do("owner", http.MethodPost, channelPath(c)+"/move", `{"before_id":"`+a.String()+`"}`, key()...), http.StatusOK, "")
	if m["version"] != float64(2) {
		t.Fatalf("moved %v", m)
	}
	for id, v := range before {
		if w.channelVersion(id) != v {
			t.Fatalf("channel %v changed", id)
		}
	}
	if got := w.channelNames("owner", support); !slices.Equal(got, []string{"c", "a", "b"}) {
		t.Fatalf("order %v", got)
	}
	w.want(w.do("owner", http.MethodPost, channelPath(c)+"/move", `{}`, key()...), http.StatusOK, "")
	if got := w.channelNames("owner", support); !slices.Equal(got, []string{"c", "a", "b"}) {
		t.Fatalf("order after start %v", got)
	}
	w.want(w.do("owner", http.MethodPost, channelPath(c)+"/move", `{"after_id":"`+b.String()+`"}`, key()...), http.StatusOK, "")
	if got := w.channelNames("owner", support); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Fatalf("order after b %v", got)
	}
	w.want(w.do("owner", http.MethodPost, channelPath(c)+"/move", `{"after_id":"`+a.String()+`","before_id":"`+b.String()+`"}`, key()...),
		http.StatusBadRequest, "validation_failed")
	w.want(w.do("owner", http.MethodPost, channelPath(c)+"/move", `{"after_id":"`+other.String()+`"}`, key()...), http.StatusBadRequest, "validation_failed")
	if evs := w.events("channel.updated"); len(evs) != 3 {
		t.Fatalf("channel.updated %d", len(evs))
	}
	sam := w.person("sam", false)
	w.channelGrant(sam, a, "read", "{}")
	hidden := w.do("sam", http.MethodPost, channelPath(b)+"/move", `{}`, key()...)
	w.want(hidden, http.StatusNotFound, "not_found")
	same404(t, hidden, w.do("sam", http.MethodPost, channelPath(ids.New())+"/move", `{}`, key()...))
	w.want(w.do("sam", http.MethodPost, channelPath(a)+"/move", `{}`, key()...), http.StatusForbidden, "forbidden")
}

// TestArchiveChannel_StepUp: a stale step-up is 401 step_up_required; a fresh one 200; a blank
// reason 400.
func TestArchiveChannel_StepUp(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.session("owner-stale", w.ownerID, start.Add(-time.Hour))
	support := w.create("Support")
	ch := w.channel(support, "text", "tickets")
	w.want(w.do("owner-stale", http.MethodPost, channelPath(ch)+"/archive", `{"reason":"Old"}`, key()...), http.StatusUnauthorized, "step_up_required")
	w.want(w.do("owner", http.MethodPost, channelPath(ch)+"/archive", `{"reason":" "}`, key()...), http.StatusBadRequest, "validation_failed")
	m := w.want(w.do("owner", http.MethodPost, channelPath(ch)+"/archive", `{"reason":"Old"}`, key()...), http.StatusOK, "")
	if m["archived_at"] == nil || m["version"] != float64(2) {
		t.Fatalf("archived %v", m)
	}
}

// TestArchiveChannel_RightsChange: permission_version + 1, access.changed for Alex (trigger to
// read) and the owner, none for Sam who only had read; channel.archived, ChannelArchived and the
// audit entry with the reason.
func TestArchiveChannel_RightsChange(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alex, sam := w.person("alex", false), w.person("sam", false)
	support := w.create("Support")
	ch := w.channel(support, "text", "tickets")
	w.grant(alex, support, "trigger")
	w.grant(sam, support, "read")
	v := w.permissionVersion()
	w.want(w.do("owner", http.MethodPost, channelPath(ch)+"/archive", `{"reason":"Old"}`, key()...), http.StatusOK, "")
	if w.permissionVersion() != v+1 {
		t.Fatalf("permission_version %d, want %d", w.permissionVersion(), v+1)
	}
	changed := map[string]map[string]any{}
	for _, e := range w.events("access.changed") {
		list, _ := e.Payload["changed"].([]any)
		for _, c := range list {
			entry := c.(map[string]any)
			if entry["id"] == ch.String() {
				changed[e.Audience] = entry
			}
		}
	}
	if len(changed) != 2 || changed[alex.String()] == nil || changed[w.ownerID.String()] == nil {
		t.Fatalf("access.changed for %v", changed)
	}
	for who, entry := range changed {
		if entry["kind"] != "channel" || entry["level"] != "read" || entry["previous_level"] != "trigger" {
			t.Errorf("%s: %v", who, entry)
		}
	}
	if evs := w.events("channel.archived"); len(evs) != 1 || evs[0].Version != 2 || evs[0].SpaceID != support.String() {
		t.Fatalf("channel.archived %+v", evs)
	}
	if len(*w.channelsArchived) != 1 || (*w.channelsArchived)[0].ChannelID != ch || (*w.channelsArchived)[0].SpaceID != support {
		t.Fatalf("ChannelArchived %+v", *w.channelsArchived)
	}
	if a := w.allows("channel.archive"); len(a) != 1 || a[0].Details["reason"] != "Old" {
		t.Fatalf("audit %+v", a)
	}
}

// TestArchiveChannel_WritesDeniedForEveryone: after archiving, thread.create is forbidden with
// reason channel_archived for the owner and for Alex; channel.read stays allowed; a denial audit
// entry carries channel_archived; reading over REST shows level read and archived_at.
func TestArchiveChannel_WritesDeniedForEveryone(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alex := w.person("alex", false)
	support := w.create("Support")
	ch := w.channel(support, "text", "tickets")
	w.grant(alex, support, "trigger")
	w.want(w.do("owner", http.MethodPost, channelPath(ch)+"/archive", `{"reason":"Old"}`, key()...), http.StatusOK, "")
	ctx := context.Background()
	r := authorize.Resource{Kind: authorize.ResourceChannel, ID: ch}
	for _, who := range []string{"owner", "alex"} {
		d, err := w.az.Authorize(ctx, w.actors.Get(who), authorize.ActionThreadCreate, r)
		if err != nil || d.Outcome != authorize.OutcomeForbidden || d.Reason != "channel_archived" {
			t.Fatalf("%s thread.create %+v, %v", who, d, err)
		}
		if e, ok := errs.As(d.Err()); !ok || e.Code != errs.CodeChannelReadOnly || e.Reason != "channel_archived" {
			t.Fatalf("%s problem %+v", who, e)
		}
		if d, err := w.az.Authorize(ctx, w.actors.Get(who), authorize.ActionChannelRead, r); err != nil || !d.Allowed() {
			t.Fatalf("%s channel.read %+v, %v", who, d, err)
		}
		m := w.want(w.do(who, http.MethodGet, channelPath(ch), ""), http.StatusOK, "")
		if m["level"] != "read" || m["channel"].(map[string]any)["archived_at"] == nil {
			t.Fatalf("%s view %v", who, m)
		}
	}
	if err := w.rec.Aggregator().Flush(ctx); err != nil {
		t.Fatal(err)
	}
	if w.count(`SELECT count(*)::int FROM audit_events WHERE action = 'thread.create' AND decision = 'deny' AND deny_reason = 'channel_archived'`) == 0 {
		t.Fatal("no denial entry with channel_archived")
	}
}

// TestArchiveChannel_Idempotent: archiving again answers 200 with the channel and writes nothing.
func TestArchiveChannel_Idempotent(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	support := w.create("Support")
	ch := w.channel(support, "text", "tickets")
	w.want(w.do("owner", http.MethodPost, channelPath(ch)+"/archive", `{"reason":"Old"}`, key()...), http.StatusOK, "")
	v := w.permissionVersion()
	m := w.want(w.do("owner", http.MethodPost, channelPath(ch)+"/archive", `{"reason":"Again"}`, key()...), http.StatusOK, "")
	if m["version"] != float64(2) || w.permissionVersion() != v || len(w.events("channel.archived")) != 1 || len(*w.channelsArchived) != 1 {
		t.Fatalf("second archive wrote: %v", m)
	}
}

// TestArchivedChannel_Mutations409: update, move and tag create of an archived channel, and
// changes of a channel in an archived space, are 409; listSpaceChannels omits it, getChannel
// answers it.
func TestArchivedChannel_Mutations409(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	support := w.create("Support")
	ch := w.channel(support, "forum", "bugs")
	w.want(w.do("owner", http.MethodPost, channelPath(ch)+"/archive", `{"reason":"Old"}`, key()...), http.StatusOK, "")
	w.want(w.do("owner", http.MethodPatch, channelPath(ch), `{"name":"desk"}`, append(key(), ifMatch(2)...)...), http.StatusConflict, "conflict")
	w.want(w.do("owner", http.MethodPost, channelPath(ch)+"/move", `{}`, key()...), http.StatusConflict, "conflict")
	w.want(w.do("owner", http.MethodPost, tagsPath(ch), `{"name":"bug"}`, key()...), http.StatusConflict, "conflict")
	if got := w.channelNames("owner", support); len(got) != 0 {
		t.Fatalf("listed %v", got)
	}
	w.want(w.do("owner", http.MethodGet, channelPath(ch), ""), http.StatusOK, "")
	research := w.create("Research")
	live := w.channel(research, "forum", "ideas")
	w.want(w.do("owner", http.MethodPost, spacePath(research)+"/archive", `{"reason":"Done"}`, key()...), http.StatusOK, "")
	w.want(w.do("owner", http.MethodPatch, channelPath(live), `{"name":"desk"}`, append(key(), ifMatch(1)...)...), http.StatusConflict, "conflict")
	w.want(w.do("owner", http.MethodPost, tagsPath(live), `{"name":"bug"}`, key()...), http.StatusConflict, "conflict")
	d, err := w.az.Authorize(context.Background(), w.actors.Get("owner"), authorize.ActionThreadCreate, authorize.Resource{Kind: authorize.ResourceChannel, ID: live})
	if err != nil || d.Reason != "channel_archived" {
		t.Fatalf("thread.create in an archived space %+v, %v", d, err)
	}
}

// TestForumTags_CRUD: rights (manage needed, a read-only member 403, an outsider 404),
// restricted, palette colour accepted, hex 400, duplicate name 409, text channel 400 not_a_forum,
// delete twice 204, clear_color, moveForumTag changes the order, every change emits
// channel.updated with a higher channel version.
func TestForumTags_CRUD(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	sam := w.person("sam", false)
	w.person("robin", false)
	support := w.create("Support")
	forum := w.channel(support, "forum", "bugs")
	text := w.channel(support, "text", "tickets")
	w.grant(sam, support, "write")
	w.want(w.do("sam", http.MethodPost, tagsPath(forum), `{"name":"bug"}`, key()...), http.StatusForbidden, "forbidden")
	w.want(w.do("robin", http.MethodPost, tagsPath(forum), `{"name":"bug"}`, key()...), http.StatusNotFound, "not_found")
	p := w.want(w.do("owner", http.MethodPost, tagsPath(text), `{"name":"bug"}`, key()...), http.StatusBadRequest, "validation_failed")
	if errsList, _ := p["errors"].([]any); len(errsList) != 1 || errsList[0].(map[string]any)["code"] != "not_a_forum" {
		t.Fatalf("problem %v", p)
	}
	w.want(w.do("owner", http.MethodPost, tagsPath(forum), `{"name":"bug","color":"#ff0000"}`, key()...), http.StatusBadRequest, "validation_failed")
	w.want(w.do("owner", http.MethodPost, tagsPath(forum), `{"name":"`+strings.Repeat("t", chatConfig.TagNameMaxChars+1)+`"}`, key()...),
		http.StatusBadRequest, "validation_failed")
	v := w.channelVersion(forum)
	bug := w.want(w.do("owner", http.MethodPost, tagsPath(forum), `{"name":"bug","color":"moss","restricted":true}`, key()...), http.StatusCreated, "")
	if bug["color"] != "moss" || bug["restricted"] != true || bug["channel_id"] != forum.String() {
		t.Fatalf("tag %v", bug)
	}
	bugID := ids.MustParse(bug["id"].(string))
	idea := w.want(w.do("owner", http.MethodPost, tagsPath(forum), `{"name":"idea"}`, key()...), http.StatusCreated, "")
	ideaID := ids.MustParse(idea["id"].(string))
	if idea["restricted"] != false {
		t.Fatalf("tag %v", idea)
	}
	w.want(w.do("owner", http.MethodPost, tagsPath(forum), `{"name":"BUG"}`, key()...), http.StatusConflict, "conflict")
	if w.channelVersion(forum) != v+2 {
		t.Fatalf("channel version %d, want %d", w.channelVersion(forum), v+2)
	}
	tags := func() []string {
		m := w.want(w.do("sam", http.MethodGet, tagsPath(forum), ""), http.StatusOK, "")
		var out []string
		for _, it := range m["items"].([]any) {
			out = append(out, it.(map[string]any)["name"].(string))
		}
		return out
	}
	if got := tags(); !slices.Equal(got, []string{"bug", "idea"}) {
		t.Fatalf("tags %v", got)
	}
	w.want(w.do("owner", http.MethodPost, tagPath(forum, ideaID)+"/move", `{"before_id":"`+bugID.String()+`"}`, key()...), http.StatusOK, "")
	if got := tags(); !slices.Equal(got, []string{"idea", "bug"}) {
		t.Fatalf("tags after move %v", got)
	}
	m := w.want(w.do("owner", http.MethodPatch, tagPath(forum, bugID), `{"clear_color":true}`, key()...), http.StatusOK, "")
	if _, ok := m["color"]; ok {
		t.Fatalf("color kept %v", m)
	}
	w.want(w.do("owner", http.MethodPatch, tagPath(forum, bugID), `{"clear_color":true,"color":"moss"}`, key()...), http.StatusBadRequest, "validation_failed")
	before := w.channelVersion(forum)
	w.want(w.do("owner", http.MethodPatch, tagPath(forum, bugID), `{"restricted":true}`, key()...), http.StatusOK, "")
	if w.channelVersion(forum) != before {
		t.Fatal("a tag update without change bumped the channel version")
	}
	w.want(w.do("owner", http.MethodDelete, tagPath(forum, ideaID), ""), http.StatusNoContent, "")
	w.want(w.do("owner", http.MethodDelete, tagPath(forum, ideaID), ""), http.StatusNoContent, "")
	w.want(w.do("owner", http.MethodDelete, tagPath(forum, ids.New()), ""), http.StatusNotFound, "not_found")
	w.want(w.do("owner", http.MethodDelete, tagPath(text, bugID), ""), http.StatusBadRequest, "validation_failed")
	if got := tags(); !slices.Equal(got, []string{"bug"}) {
		t.Fatalf("tags after delete %v", got)
	}
	w.want(w.do("owner", http.MethodPost, tagsPath(forum), `{"name":"idea"}`, key()...), http.StatusCreated, "")
	evs := w.events("channel.updated")
	if len(evs) != 6 {
		t.Fatalf("channel.updated %d, want 6", len(evs))
	}
	for i := 1; i < len(evs); i++ {
		if evs[i].Version <= evs[i-1].Version || evs[i].ResourceID != forum.String() {
			t.Fatalf("channel.updated %+v", evs)
		}
	}
	if a := w.allows("forum_tag.manage"); len(a) < 6 || a[0].Details["op"] != "create" || a[0].Details["tag_id"] != bugID.String() {
		t.Fatalf("audit %+v", a)
	}
}
