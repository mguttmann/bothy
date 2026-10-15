// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	notifyapp "github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// TestMain starts the PostgreSQL containers of pgtest; the world is test/messagingworld with the
// personal data of sync and the notify module (NewNotify).
func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// vetoArchive is the test subscriber of ChannelArchived, registered after notify: while set, it
// refuses the next archive once, so the cleanup of notify rolls back with it.
type vetoArchive struct {
	mu   sync.Mutex
	once bool
}

func (v *vetoArchive) onChannelArchived(context.Context, chat.ChannelArchived) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.once {
		v.once = false
		return errs.New(errs.CodeConflict, "test veto")
	}
	return nil
}

// env is the test world of 07 sections 10 and 11.1: space Support with the text channels
// Allgemein and Eskalation; Alex with trigger on Support; Sam with read on Support/Allgemein (G-4);
// Robin without a grant; the owner. T is a thread of Alex in Allgemein, M its main thread.
type env struct {
	*mw.World
	veto                                 *vetoArchive
	support, allgemein, eskalation, t, m ids.ID
	alex, sam, robin                     ids.ID
	samGrant                             ids.ID
	defaultMax                           int
}

// config is the notify configuration of the world: fixture values, not the defaults.
func config(payloadMax int) notifyapp.Config {
	return notifyapp.Config{EventPayloadMaxBytes: payloadMax, UserDefaultLevel: "mentions"}
}

func newEnv(t *testing.T) *env { return newEnvWith(t, mw.PayloadMax) }

func newEnvWith(t *testing.T, payloadMax int) *env {
	t.Helper()
	v := &vetoArchive{}
	w := mw.NewNotify(t, func(reg *txn.Registry, _ *mw.Recorder) error { return txn.Subscribe(reg, v.onChannelArchived) }, config(payloadMax))
	e := &env{World: w, veto: v, defaultMax: payloadMax}
	e.support = w.Space("Support")
	e.allgemein = w.Channel(e.support, "text", "Allgemein")
	e.eskalation = w.Channel(e.support, "text", "Eskalation")
	e.m = w.MainThread(e.allgemein)
	e.alex, e.sam, e.robin = w.Person("alex", false), w.Person("sam", false), w.Person("robin", false)
	w.Grant("user", e.alex, e.support, "trigger")
	e.samGrant = ids.New()
	w.SQL(`INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, target_channel_id, level, capabilities, reason, granted_by)
		VALUES ($1, 'user', $2, 'channel', $3, 'read', '{}', 'fixture', $4)`, e.samGrant.String(), e.sam.String(), e.allgemein.String(), w.OwnerID.String())
	e.t = e.thread("alex", e.allgemein, e.m)
	return e
}

// thread creates a thread as name in the text channel ch from a new message in main.
func (e *env) thread(name string, ch, main ids.ID) ids.ID {
	parent := e.Send(name, main, "start", http.StatusCreated)
	m := e.Want(e.Do(name, http.MethodPost, mw.ThreadsPath(ch), `{"parent_message_id":"`+parent["id"].(string)+`"}`, mw.Key()...), http.StatusCreated, "")
	return ids.MustParse(m["thread"].(map[string]any)["id"].(string))
}

func prefPath(kind string, id ids.ID) string {
	return "/api/v1/me/notification-preferences/" + kind + "/" + id.String()
}

const (
	prefsPath = "/api/v1/me/notification-preferences"
	quietPath = "/api/v1/me/quiet-hours"
)

func rulePath(id string) string { return quietPath + "/" + id }

// put sets a level as name and returns the response.
func (e *env) put(name, kind string, id ids.ID, body string) *httpservertest.Response {
	return e.Do(name, http.MethodPut, prefPath(kind, id), body)
}

// level is a body with level only.
func level(l string) string { return `{"level":"` + l + `"}` }

// version returns the JSON number m["version"] as int.
func version(m map[string]any) int {
	f, _ := m["version"].(float64)
	return int(f)
}

// event is one durable event of this ticket with its envelope columns.
type event struct {
	seq                                          int64
	typ, actor, audience, resourceKind, resource string
	resourceVersion                              int64
	scope                                        string
	payload                                      string
}

// events returns the events of type typ in seq order; it fails on JSON null in a payload.
func (e *env) events(t *testing.T, typ string) []event {
	t.Helper()
	rs, err := e.Owner.Query(context.Background(), `SELECT seq, type, coalesce(actor_principal_id::text, ''), coalesce(audience_principal_id::text, ''),
		resource_kind, coalesce(resource_id::text, ''), coalesce(resource_version, 0),
		coalesce(space_id::text, '') || coalesce(channel_id::text, '') || coalesce(thread_id::text, ''), payload::text
		FROM event_log WHERE type = $1 ORDER BY seq`, typ)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	var out []event
	for rs.Next() {
		var r event
		if err := rs.Scan(&r.seq, &r.typ, &r.actor, &r.audience, &r.resourceKind, &r.resource, &r.resourceVersion, &r.scope, &r.payload); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(r.payload, "null") {
			t.Errorf("JSON null in %s", r.payload)
		}
		out = append(out, r)
	}
	if err := rs.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// strictDecode decodes raw into v and fails on unknown members.
func strictDecode(t *testing.T, raw string, v any) {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		t.Fatalf("payload %s: %v", raw, err)
	}
}

// prefPayload decodes a notification_pref.updated payload against NotificationPrefPayload.
func prefPayload(t *testing.T, raw string) rest.NotificationPrefPayload {
	t.Helper()
	var p rest.NotificationPrefPayload
	strictDecode(t, raw, &p)
	return p
}

// personalEnvelope checks resource user without version, empty scope and audience person.
func personalEnvelope(t *testing.T, ev event, person ids.ID) {
	t.Helper()
	if ev.audience != person.String() || ev.resourceKind != "user" || ev.resource != person.String() || ev.resourceVersion != 0 || ev.scope != "" {
		t.Fatalf("envelope %+v, want the person %s", ev, person)
	}
}

// facts returns the effective level of person in thread at the clock's now.
func (e *env) facts(t *testing.T, thread ids.ID, person ids.ID) domain.EffectiveLevel {
	t.Helper()
	f, err := e.Notify.Facts().LevelFacts(context.Background(), thread, []ids.ID{person})
	if err != nil {
		t.Fatal(err)
	}
	got, err := domain.ResolveLevel(f[person], e.Clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// wantLevel checks the effective level and its source.
func (e *env) wantLevel(t *testing.T, step string, thread, person ids.ID, l domain.Level, src domain.LevelSource) {
	t.Helper()
	if got := e.facts(t, thread, person); got.Level != l || got.Source != src {
		t.Fatalf("%s: %+v, want %s from %s", step, got, l, src)
	}
}

// channelVersion returns channels.version of ch.
func (e *env) channelVersion(ch ids.ID) int {
	return e.Count(`SELECT version::int FROM channels WHERE id = $1`, ch.String())
}

// updateChannel patches ch as the owner with body.
func (e *env) updateChannel(ch ids.ID, body string) {
	e.Want(e.Do("owner", http.MethodPatch, "/api/v1/channels/"+ch.String(), body,
		append(mw.Key(), "If-Match", `"`+strconv.Itoa(e.channelVersion(ch))+`"`)...), http.StatusOK, "")
}

// archiveChannel and archiveSpace archive as the owner (in a session with a fresh step-up).
func (e *env) archiveChannel(ch ids.ID, status int) *httpservertest.Response {
	r := e.Do("owner", http.MethodPost, "/api/v1/channels/"+ch.String()+"/archive", `{"reason":"Done"}`, mw.Key()...)
	if status != 0 && r.StatusCode != status {
		e.Want(r, status, "")
	}
	return r
}

func (e *env) archiveSpace(sp ids.ID) {
	e.Want(e.Do("owner", http.MethodPost, "/api/v1/spaces/"+sp.String()+"/archive", `{"reason":"Done"}`, mw.Key()...), http.StatusOK, "")
}

// rows counts the stored preferences on target.
func (e *env) rows(kind string, id ids.ID) int {
	return e.Count(`SELECT count(*)::int FROM notification_preferences WHERE target_kind = $1 AND target_id = $2`, kind, id.String())
}

// TestEffectiveLevelOrder (13 section 3.1, PLAN row): every step of the order against
// PostgreSQL through LevelFacts and ResolveLevel, removing one source after the other; the DM
// step with a fixture channel; one call for three people with their own sources.
func TestEffectiveLevelOrder(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.Want(e.put("alex", "thread", e.t, level("mute")), http.StatusOK, "")
	e.Want(e.put("alex", "channel", e.allgemein, level("all")), http.StatusOK, "")
	e.Want(e.put("alex", "space", e.support, level("mentions")), http.StatusOK, "")
	e.updateChannel(e.allgemein, `{"default_notification_level":"mute"}`)
	e.Want(e.Do("alex", http.MethodPut, "/api/v1/me/settings/notify.default_level", `{"value":"all"}`), http.StatusOK, "")

	e.wantLevel(t, "thread row", e.t, e.alex, domain.LevelMute, domain.SourceThread)
	e.Want(e.Do("alex", http.MethodDelete, prefPath("thread", e.t), ""), http.StatusNoContent, "")
	e.wantLevel(t, "channel row", e.t, e.alex, domain.LevelAll, domain.SourceChannel)
	e.Want(e.Do("alex", http.MethodDelete, prefPath("channel", e.allgemein), ""), http.StatusNoContent, "")
	e.wantLevel(t, "space row", e.t, e.alex, domain.LevelMentions, domain.SourceSpace)
	e.Want(e.Do("alex", http.MethodDelete, prefPath("space", e.support), ""), http.StatusNoContent, "")
	e.wantLevel(t, "channel default", e.t, e.alex, domain.LevelMute, domain.SourceChannelDefault)
	e.updateChannel(e.allgemein, `{"clear_default_notification_level":true}`)
	e.wantLevel(t, "personal default", e.t, e.alex, domain.LevelAll, domain.SourcePersonalDefault)
	e.Want(e.Do("alex", http.MethodDelete, "/api/v1/me/settings/notify.default_level", ""), http.StatusNoContent, "")
	e.wantLevel(t, "instance default", e.t, e.alex, domain.LevelMentions, domain.SourceInstanceDefault)

	// The DM step: a fixture dm channel without a space, with a thread.
	dm, dmThread := ids.New(), ids.New()
	e.SQL(`INSERT INTO channels (id, kind, name, sort_key, operator_inherit, history_backfill_limit, created_by)
		VALUES ($1, 'dm', 'dm', 'a', false, 20, $2)`, dm.String(), e.OwnerID.String())
	e.SQL(`INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`, dmThread.String(), dm.String(), e.OwnerID.String())
	e.wantLevel(t, "dm", dmThread, e.alex, domain.LevelAll, domain.SourceDirectMessage)
	e.SQL(`UPDATE channels SET default_notification_level = 'mentions' WHERE id = $1`, dm.String())
	e.wantLevel(t, "dm with a channel default", dmThread, e.alex, domain.LevelMentions, domain.SourceChannelDefault)

	// One call for three people, each with its own sources.
	e.Want(e.put("alex", "thread", e.t, level("mute")), http.StatusOK, "")
	e.Want(e.put("sam", "channel", e.allgemein, level("all")), http.StatusOK, "")
	f, err := e.Notify.Facts().LevelFacts(context.Background(), e.t, []ids.ID{e.alex, e.sam, e.robin})
	if err != nil {
		t.Fatal(err)
	}
	want := map[ids.ID]domain.LevelSource{e.alex: domain.SourceThread, e.sam: domain.SourceChannel, e.robin: domain.SourceInstanceDefault}
	for p, src := range want {
		got, err := domain.ResolveLevel(f[p], e.Clock.Now())
		if err != nil || got.Source != src {
			t.Fatalf("person %s: %+v %v, want %s", p, got, err, src)
		}
	}
}

// TestEffectiveLevel_MutedUntil: muted_until in the future mutes the chosen row until the clock
// passes it; on a row that is not chosen it has no effect.
func TestEffectiveLevel_MutedUntil(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	until := e.Clock.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	e.Want(e.put("alex", "thread", e.t, `{"level":"all","muted_until":"`+until+`"}`), http.StatusOK, "")
	if got := e.facts(t, e.t, e.alex); got.Level != domain.LevelMute || !got.Muted || got.Source != domain.SourceThread {
		t.Fatalf("muted: %+v", got)
	}
	e.Clock.Advance(time.Hour + time.Second)
	if got := e.facts(t, e.t, e.alex); got.Level != domain.LevelAll || got.Muted {
		t.Fatalf("after muted_until: %+v", got)
	}
	later := e.Clock.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	e.Want(e.put("alex", "space", e.support, `{"level":"mentions","muted_until":"`+later+`"}`), http.StatusOK, "")
	if got := e.facts(t, e.t, e.alex); got.Level != domain.LevelAll || got.Muted {
		t.Fatalf("a mute on the space row while the thread has a row: %+v", got)
	}
}

// TestPutNotificationPreference_Lifecycle: version 1 without muted_until, the event envelope and
// payload, a no-op without event, a change with version 2, microseconds stored.
func TestPutNotificationPreference_Lifecycle(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	m := e.Want(e.put("alex", "thread", e.t, level("mentions")), http.StatusOK, "")
	if version(m) != 1 || m["level"] != "mentions" || m["target_kind"] != "thread" || m["target_id"] != e.t.String() {
		t.Fatalf("first put %v", m)
	}
	if _, ok := m["muted_until"]; ok {
		t.Fatalf("muted_until without one: %v", m)
	}
	evs := e.events(t, "notification_pref.updated")
	if len(evs) != 1 {
		t.Fatalf("%d events", len(evs))
	}
	personalEnvelope(t, evs[0], e.alex)
	if p := prefPayload(t, evs[0].payload); p.Deleted != nil || p.Preference.Version != 1 || p.Preference.Level != "mentions" {
		t.Fatalf("payload %s", evs[0].payload)
	}
	if evs[0].actor != e.alex.String() {
		t.Fatalf("actor %s", evs[0].actor)
	}
	m = e.Want(e.put("alex", "thread", e.t, level("mentions")), http.StatusOK, "")
	if version(m) != 1 || len(e.events(t, "notification_pref.updated")) != 1 {
		t.Fatalf("the same body again: %v", m)
	}
	m = e.Want(e.put("alex", "thread", e.t, level("all")), http.StatusOK, "")
	if version(m) != 2 || len(e.events(t, "notification_pref.updated")) != 2 {
		t.Fatalf("another level: %v", m)
	}
	nanos := e.Clock.Now().Add(time.Hour).Add(123456789 * time.Nanosecond).UTC()
	body := `{"level":"all","muted_until":"` + nanos.Format(time.RFC3339Nano) + `"}`
	m = e.Want(e.put("alex", "thread", e.t, body), http.StatusOK, "")
	if version(m) != 3 || m["muted_until"] != nanos.Truncate(time.Millisecond).Format("2006-01-02T15:04:05.000Z") {
		t.Fatalf("muted_until answer: %v", m)
	}
	var stored time.Time
	if err := e.Owner.QueryRow(context.Background(), `SELECT muted_until FROM notification_preferences WHERE target_id = $1`, e.t.String()).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !stored.Equal(nanos.Truncate(time.Microsecond)) {
		t.Fatalf("stored %v, want microseconds of %v", stored, nanos)
	}
	m = e.Want(e.put("alex", "thread", e.t, body), http.StatusOK, "")
	if version(m) != 3 || len(e.events(t, "notification_pref.updated")) != 3 {
		t.Fatalf("an equal muted_until is a no-op: %v", m)
	}
}

// TestDeleteNotificationPreference: delete with event deleted and version last + 1, a second
// delete without event, a later put at version 1.
func TestDeleteNotificationPreference(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.Want(e.put("alex", "channel", e.allgemein, level("all")), http.StatusOK, "")
	e.Want(e.put("alex", "channel", e.allgemein, level("mute")), http.StatusOK, "")
	e.Want(e.Do("alex", http.MethodDelete, prefPath("channel", e.allgemein), ""), http.StatusNoContent, "")
	evs := e.events(t, "notification_pref.updated")
	if len(evs) != 3 {
		t.Fatalf("%d events", len(evs))
	}
	personalEnvelope(t, evs[2], e.alex)
	if p := prefPayload(t, evs[2].payload); p.Deleted == nil || !*p.Deleted || p.Preference.Version != 3 || p.Preference.Level != "mute" {
		t.Fatalf("delete payload %s", evs[2].payload)
	}
	e.Want(e.Do("alex", http.MethodDelete, prefPath("channel", e.allgemein), ""), http.StatusNoContent, "")
	if n := len(e.events(t, "notification_pref.updated")); n != 3 {
		t.Fatalf("a second delete wrote an event: %d", n)
	}
	if m := e.Want(e.put("alex", "channel", e.allgemein, level("all")), http.StatusOK, ""); version(m) != 1 {
		t.Fatalf("restart: %v", m)
	}
}

// TestNotificationPreference_Concurrent: parallel identical first puts end in one row and one
// event; parallel different puts end in version 2 with the stored level of the later event.
func TestNotificationPreference_Concurrent(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	parallel := func(bodies ...string) {
		var wg sync.WaitGroup
		res := make([]*httpservertest.Response, len(bodies))
		for i, b := range bodies {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res[i] = e.put("alex", "thread", e.t, b)
			}()
		}
		wg.Wait()
		for _, r := range res {
			if r.StatusCode != http.StatusOK {
				t.Fatalf("status %d: %s", r.StatusCode, r.Body)
			}
		}
	}
	parallel(level("all"), level("all"))
	if n, ev := e.rows("thread", e.t), len(e.events(t, "notification_pref.updated")); n != 1 || ev != 1 {
		t.Fatalf("identical first puts: %d rows, %d events", n, ev)
	}
	e.Want(e.Do("alex", http.MethodDelete, prefPath("thread", e.t), ""), http.StatusNoContent, "")
	before := len(e.events(t, "notification_pref.updated"))
	parallel(level("mute"), level("mentions"))
	evs := e.events(t, "notification_pref.updated")[before:]
	var lvl string
	var version int
	if err := e.Owner.QueryRow(context.Background(), `SELECT level, version::int FROM notification_preferences WHERE target_id = $1`, e.t.String()).Scan(&lvl, &version); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 || version != 2 || string(prefPayload(t, evs[1].payload).Preference.Level) != lvl {
		t.Fatalf("different puts: %d events, version %d, level %s, last %s", len(evs), version, lvl, evs[len(evs)-1].payload)
	}
}

// TestNotificationPreference_TargetChecks (AZ-P4): Robin's writes give the 404 of an unknown id,
// a channel id as thread too; Alex can set a level on the main thread; agents get 403 on all
// seven operations.
func TestNotificationPreference_TargetChecks(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	for _, c := range []struct {
		kind string
		id   ids.ID
	}{{"thread", e.t}, {"channel", e.allgemein}, {"space", e.support}} {
		mw.Same404(t, e.put("robin", c.kind, c.id, level("all")), e.put("robin", c.kind, ids.New(), level("all")))
		mw.Same404(t, e.Do("robin", http.MethodDelete, prefPath(c.kind, c.id), ""), e.Do("robin", http.MethodDelete, prefPath(c.kind, ids.New()), ""))
	}
	mw.Same404(t, e.put("alex", "thread", e.allgemein, level("all")), e.put("alex", "thread", ids.New(), level("all")))
	e.Want(e.put("alex", "thread", e.m, level("mute")), http.StatusOK, "")

	bot := e.Agent("bot")
	e.Grant("agent", bot, e.support, "read")
	rule := ids.New().String()
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, prefsPath, ""},
		{http.MethodPut, prefPath("thread", e.t), level("all")},
		{http.MethodDelete, prefPath("thread", e.t), ""},
		{http.MethodGet, quietPath, ""},
		{http.MethodPost, quietPath, ruleBody(31, "22:00", "07:00", "")},
		{http.MethodPut, rulePath(rule), ruleBody(31, "22:00", "07:00", "")},
		{http.MethodDelete, rulePath(rule), ""},
	} {
		headers := []string{}
		if c.method == http.MethodPost {
			headers = mw.Key()
		}
		p := e.Want(e.Do("bot", c.method, c.path, c.body, headers...), http.StatusForbidden, "forbidden")
		if p["reason"] != "subject_kind_not_allowed" {
			t.Fatalf("%s %s: %v", c.method, c.path, p)
		}
	}
}

// TestListMyNotificationPreferences_ReadableOnly (04 section 14): rows of targets the person lost
// are missing from the list and stay stored; they return with the right; ordered by kind and id.
func TestListMyNotificationPreferences_ReadableOnly(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.Want(e.put("sam", "thread", e.t, level("mute")), http.StatusOK, "")
	e.Want(e.put("sam", "channel", e.allgemein, level("all")), http.StatusOK, "")
	list := func() []any {
		m := e.Want(e.Do("sam", http.MethodGet, prefsPath, ""), http.StatusOK, "")
		items, _ := m["items"].([]any)
		return items
	}
	if items := list(); len(items) != 2 || items[0].(map[string]any)["target_kind"] != "channel" || items[1].(map[string]any)["target_kind"] != "thread" {
		t.Fatalf("list %v", items)
	}
	e.SQL(`UPDATE grants SET revoked_at = now(), revoked_by = $2, revoke_reason = 'fixture' WHERE id = $1`, e.samGrant.String(), e.OwnerID.String())
	e.SQL(`UPDATE sequence_counters SET value = value + 1 WHERE name = 'permission_version'`)
	if items := list(); len(items) != 0 {
		t.Fatalf("after the revoke: %v", items)
	}
	if n := e.Count(`SELECT count(*)::int FROM notification_preferences WHERE user_id = $1`, e.sam.String()); n != 2 {
		t.Fatalf("%d rows stored", n)
	}
	e.Grant("user", e.sam, e.support, "read")
	e.SQL(`UPDATE sequence_counters SET value = value + 1 WHERE name = 'permission_version'`)
	if items := list(); len(items) != 2 {
		t.Fatalf("after a new grant: %v", items)
	}
}

// TestArchiveCleanup_Channel (R-DM-16): archiving Allgemein removes the rows on it and on T of
// every person, one event each in key order with the owner as actor; Support and Eskalation stay;
// a vetoed archive keeps everything.
func TestArchiveCleanup_Channel(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	for _, c := range []struct {
		who, kind string
		id        ids.ID
	}{{"alex", "channel", e.allgemein}, {"alex", "thread", e.t}, {"alex", "space", e.support}, {"alex", "channel", e.eskalation},
		{"sam", "channel", e.allgemein}, {"sam", "thread", e.t}} {
		e.Want(e.put(c.who, c.kind, c.id, level("mute")), http.StatusOK, "")
	}
	before := len(e.events(t, "notification_pref.updated"))
	e.veto.mu.Lock()
	e.veto.once = true
	e.veto.mu.Unlock()
	if r := e.archiveChannel(e.allgemein, 0); r.StatusCode != http.StatusConflict {
		t.Fatalf("vetoed archive: %d %s", r.StatusCode, r.Body)
	}
	if e.rows("channel", e.allgemein) != 2 || e.rows("thread", e.t) != 2 || len(e.events(t, "notification_pref.updated")) != before {
		t.Fatal("a vetoed archive changed the preferences")
	}
	e.archiveChannel(e.allgemein, http.StatusOK)
	if e.rows("channel", e.allgemein) != 0 || e.rows("thread", e.t) != 0 || e.rows("space", e.support) != 1 || e.rows("channel", e.eskalation) != 1 {
		t.Fatal("cleanup removed the wrong rows")
	}
	evs := e.events(t, "notification_pref.updated")[before:]
	if len(evs) != 4 {
		t.Fatalf("%d cleanup events, want 4", len(evs))
	}
	var last string
	for _, ev := range evs {
		p := prefPayload(t, ev.payload)
		key := ev.audience + "/" + string(p.Preference.TargetKind) + "/" + p.Preference.TargetId.String()
		if p.Deleted == nil || !*p.Deleted || p.Preference.Version != 2 || ev.actor != e.OwnerID.String() || key <= last {
			t.Fatalf("cleanup event %+v %s after %s", ev, ev.payload, last)
		}
		personalEnvelope(t, ev, ids.MustParse(ev.audience))
		last = key
	}
}

// TestArchiveCleanup_Space: archiving Support removes the rows on it, its channels and threads;
// rows of another space stay.
func TestArchiveCleanup_Space(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	other := e.Space("Other")
	e.Grant("user", e.alex, other, "read")
	e.SQL(`UPDATE sequence_counters SET value = value + 1 WHERE name = 'permission_version'`)
	for _, c := range []struct {
		kind string
		id   ids.ID
	}{{"space", e.support}, {"channel", e.allgemein}, {"channel", e.eskalation}, {"thread", e.t}, {"thread", e.m}, {"space", other}} {
		e.Want(e.put("alex", c.kind, c.id, level("all")), http.StatusOK, "")
	}
	before := len(e.events(t, "notification_pref.updated"))
	e.archiveSpace(e.support)
	if n := e.Count(`SELECT count(*)::int FROM notification_preferences`); n != 1 || e.rows("space", other) != 1 {
		t.Fatalf("%d rows left", n)
	}
	if n := len(e.events(t, "notification_pref.updated")) - before; n != 5 {
		t.Fatalf("%d cleanup events, want 5", n)
	}
}

// TestArchivedTarget_PutConflictDeleteAllowed: an archived target refuses a new preference with
// 409 target_archived; delete answers 204; the list still works.
func TestArchivedTarget_PutConflictDeleteAllowed(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.archiveChannel(e.allgemein, http.StatusOK)
	for _, c := range []struct {
		kind string
		id   ids.ID
	}{{"channel", e.allgemein}, {"thread", e.t}} {
		p := e.Want(e.put("alex", c.kind, c.id, level("all")), http.StatusConflict, "conflict")
		if p["reason"] != notifyapp.ReasonTargetArchived {
			t.Fatalf("%s: %v", c.kind, p)
		}
		e.Want(e.Do("alex", http.MethodDelete, prefPath(c.kind, c.id), ""), http.StatusNoContent, "")
	}
	e.archiveSpace(e.support)
	if p := e.Want(e.put("alex", "space", e.support, level("all")), http.StatusConflict, "conflict"); p["reason"] != notifyapp.ReasonTargetArchived {
		t.Fatalf("space: %v", p)
	}
	e.Want(e.Do("alex", http.MethodDelete, prefPath("space", e.support), ""), http.StatusNoContent, "")
	e.Want(e.Do("alex", http.MethodGet, prefsPath, ""), http.StatusOK, "")
}

// TestArchive_RaceWithPut: a put on a thread in parallel with the archive of its channel either
// commits first (and its row goes with an event) or answers 409; no row survives.
func TestArchive_RaceWithPut(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	for round := range 20 {
		ch := e.Channel(e.support, "text", "race-"+strconv.Itoa(round))
		th := e.thread("alex", ch, e.MainThread(ch))
		var wg sync.WaitGroup
		var put, archive *httpservertest.Response
		wg.Add(2)
		go func() { defer wg.Done(); put = e.put("alex", "thread", th, level("mute")) }()
		go func() { defer wg.Done(); archive = e.archiveChannel(ch, 0) }()
		wg.Wait()
		if archive.StatusCode != http.StatusOK || (put.StatusCode != http.StatusOK && put.StatusCode != http.StatusConflict) {
			t.Fatalf("round %d: archive %d, put %d %s", round, archive.StatusCode, put.StatusCode, put.Body)
		}
		if e.rows("thread", th) != 0 || e.rows("channel", ch) != 0 {
			t.Fatalf("round %d: a row survived the archive (put %d)", round, put.StatusCode)
		}
	}
}

// ruleBody is an enabled QuietHoursRuleInput; an empty zone is absent.
func ruleBody(weekdays int, start, end, zone string) string {
	b := `{"weekdays":` + strconv.Itoa(weekdays) + `,"start_local":"` + start + `","end_local":"` + end + `","enabled":true`
	if zone != "" {
		b += `,"time_zone":"` + zone + `"`
	}
	return b + "}"
}

// quietPayload decodes a quiet_hours.updated payload against QuietHoursPayload.
func quietPayload(t *testing.T, raw string) rest.QuietHoursPayload {
	t.Helper()
	var p rest.QuietHoursPayload
	strictDecode(t, raw, &p)
	return p
}

// TestQuietHours_Lifecycle: create with replay, the event with the complete list, a no-op put, a
// put that clears the zone, delete, a second delete 404 and foreign access 404.
func TestQuietHours_Lifecycle(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	key := mw.Key()
	r := e.Do("alex", http.MethodPost, quietPath, ruleBody(31, "22:00", "07:00", "Europe/Berlin"), key...)
	m := e.Want(r, http.StatusCreated, "")
	id := m["id"].(string)
	if version(m) != 1 || m["time_zone"] != "Europe/Berlin" {
		t.Fatalf("create %v", m)
	}
	evs := e.events(t, "quiet_hours.updated")
	if len(evs) != 1 {
		t.Fatalf("%d events", len(evs))
	}
	personalEnvelope(t, evs[0], e.alex)
	if p := quietPayload(t, evs[0].payload); len(p.Rules) != 1 || p.Rules[0].Id.String() != id {
		t.Fatalf("payload %s", evs[0].payload)
	}
	again := e.Do("alex", http.MethodPost, quietPath, ruleBody(31, "22:00", "07:00", "Europe/Berlin"), key...)
	if am := e.Want(again, http.StatusCreated, ""); am["id"] != id || again.Header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("replay %v %v", again.Header, am)
	}
	second := e.Want(e.Do("alex", http.MethodPost, quietPath, ruleBody(96, "00:00", "00:00", ""), mw.Key()...), http.StatusCreated, "")
	if n := e.Count(`SELECT count(*)::int FROM quiet_hours`); n != 2 {
		t.Fatalf("%d rules", n)
	}
	if p := quietPayload(t, e.events(t, "quiet_hours.updated")[1].payload); len(p.Rules) != 2 || p.Rules[0].Id.String() > p.Rules[1].Id.String() {
		t.Fatalf("complete list ascending: %+v", p.Rules)
	}
	m = e.Want(e.Do("alex", http.MethodPut, rulePath(id), ruleBody(31, "22:00", "07:00", "Europe/Berlin")), http.StatusOK, "")
	if version(m) != 1 || len(e.events(t, "quiet_hours.updated")) != 2 {
		t.Fatalf("no-op put %v", m)
	}
	m = e.Want(e.Do("alex", http.MethodPut, rulePath(id), ruleBody(31, "22:00", "07:00", "")), http.StatusOK, "")
	if _, ok := m["time_zone"]; ok || version(m) != 2 || len(e.events(t, "quiet_hours.updated")) != 3 {
		t.Fatalf("put without zone %v", m)
	}
	mw.Same404(t, e.Do("sam", http.MethodPut, rulePath(id), ruleBody(1, "01:00", "02:00", "")),
		e.Do("sam", http.MethodPut, rulePath(ids.New().String()), ruleBody(1, "01:00", "02:00", "")))
	mw.Same404(t, e.Do("sam", http.MethodDelete, rulePath(id), ""), e.Do("sam", http.MethodDelete, rulePath(ids.New().String()), ""))
	e.Want(e.Do("alex", http.MethodDelete, rulePath(id), ""), http.StatusNoContent, "")
	evs = e.events(t, "quiet_hours.updated")
	if p := quietPayload(t, evs[len(evs)-1].payload); len(p.Rules) != 1 || p.Rules[0].Id.String() != second["id"] {
		t.Fatalf("after delete %s", evs[len(evs)-1].payload)
	}
	e.Want(e.Do("alex", http.MethodDelete, rulePath(id), ""), http.StatusNotFound, "not_found")
	if items := e.Want(e.Do("alex", http.MethodGet, quietPath, ""), http.StatusOK, "")["items"].([]any); len(items) != 1 {
		t.Fatalf("list %v", items)
	}
}

// TestQuietHours_Validation: unknown zones give unknown_time_zone on /time_zone; schema errors
// come from the validator; 00:00 to 00:00 is stored.
func TestQuietHours_Validation(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	for _, zone := range []string{"Mars/Olympus", "Local"} {
		p := e.Want(e.Do("alex", http.MethodPost, quietPath, ruleBody(31, "22:00", "07:00", zone), mw.Key()...), http.StatusBadRequest, "validation_failed")
		if ptr, code := mw.Field(p); ptr != "/time_zone" || code != "unknown_time_zone" {
			t.Fatalf("%s: %v", zone, p)
		}
	}
	p := e.Want(e.Do("alex", http.MethodPost, quietPath, `{"weekdays":31,"start_local":"22:00","end_local":"07:00","enabled":true,"time_zone":""}`, mw.Key()...),
		http.StatusBadRequest, "validation_failed")
	if ptr, code := mw.Field(p); ptr != "/time_zone" || code != "unknown_time_zone" {
		t.Fatalf("empty zone: %v", p)
	}
	e.Want(e.Do("alex", http.MethodPost, quietPath, ruleBody(0, "22:00", "07:00", ""), mw.Key()...), http.StatusBadRequest, "validation_failed")
	e.Want(e.Do("alex", http.MethodPost, quietPath, ruleBody(31, "24:00", "07:00", ""), mw.Key()...), http.StatusBadRequest, "validation_failed")
	e.Want(e.Do("alex", http.MethodPost, quietPath, ruleBody(31, "00:00", "00:00", ""), mw.Key()...), http.StatusCreated, "")
}

// TestQuietHours_ListBoundByPayloadSize: with a small sync.event_payload_max_bytes the create that
// would make the list too large is limit_reached and stores nothing; a put of the same size
// passes.
func TestQuietHours_ListBoundByPayloadSize(t *testing.T) {
	t.Parallel()
	e := newEnvWith(t, 600)
	var created []string
	for range 20 {
		r := e.Do("alex", http.MethodPost, quietPath, ruleBody(31, "22:00", "07:00", "Europe/Berlin"), mw.Key()...)
		if r.StatusCode == http.StatusBadRequest {
			p := e.Want(r, http.StatusBadRequest, "validation_failed")
			if ptr, code := mw.Field(p); ptr != "" || code != "limit_reached" {
				t.Fatalf("limit: %v", p)
			}
			break
		}
		created = append(created, e.Want(r, http.StatusCreated, "")["id"].(string))
	}
	if len(created) == 0 || len(created) == 20 || e.Count(`SELECT count(*)::int FROM quiet_hours`) != len(created) {
		t.Fatalf("%d created", len(created))
	}
	e.Want(e.Do("alex", http.MethodPut, rulePath(created[0]), ruleBody(31, "21:00", "07:00", "Europe/Berlin")), http.StatusOK, "")
}

// TestQuietHours_ConcurrentCreates: ten parallel creates give ten rows, ten events, and the last
// event lists all ten.
func TestQuietHours_ConcurrentCreates(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := e.Do("alex", http.MethodPost, quietPath, ruleBody(31, "22:00", "07:00", ""), mw.Key()...)
			if r.StatusCode != http.StatusCreated {
				t.Errorf("status %d: %s", r.StatusCode, r.Body)
			}
		}()
	}
	wg.Wait()
	evs := e.events(t, "quiet_hours.updated")
	if n := e.Count(`SELECT count(*)::int FROM quiet_hours`); n != 10 || len(evs) != 10 {
		t.Fatalf("%d rows, %d events", n, len(evs))
	}
	if p := quietPayload(t, evs[9].payload); len(p.Rules) != 10 {
		t.Fatalf("the last event lists %d rules", len(p.Rules))
	}
}

// TestQuietRules_UserTimeZone (13 section 3.4 against PostgreSQL): a rule without zone follows
// users.time_zone; a zone set on the rule wins.
func TestQuietRules_UserTimeZone(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	// updateMyProfile is not part of this world; the column is set as identity would store it.
	e.SQL(`UPDATE users SET time_zone = 'Europe/Berlin' WHERE principal_id = $1`, e.alex.String())
	id := e.Want(e.Do("alex", http.MethodPost, quietPath, ruleBody(16, "22:00", "07:00", ""), mw.Key()...), http.StatusCreated, "")["id"].(string)
	at := func(s string) bool {
		t.Helper()
		rules, err := e.Notify.Facts().QuietRules(context.Background(), []ids.ID{e.alex})
		if err != nil {
			t.Fatal(err)
		}
		now, _ := time.Parse(time.RFC3339, s)
		on, err := domain.InQuietHours(rules[e.alex], now)
		if err != nil {
			t.Fatal(err)
		}
		return on
	}
	if !at("2026-10-23T21:30:00Z") || at("2026-10-23T19:30:00Z") {
		t.Fatal("the person's zone Europe/Berlin")
	}
	e.Want(e.Do("alex", http.MethodPut, rulePath(id), ruleBody(16, "22:00", "07:00", "UTC")), http.StatusOK, "")
	if at("2026-10-23T21:30:00Z") || !at("2026-10-23T22:30:00Z") {
		t.Fatal("the rule's zone UTC")
	}
}

// TestPersonalDataNotRetrievable (DP-T10 pattern): Sam sees none of Alex's preferences or rules
// through any operation; every event names its person as audience.
func TestPersonalDataNotRetrievable(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.Want(e.put("alex", "channel", e.allgemein, level("mute")), http.StatusOK, "")
	id := e.Want(e.Do("alex", http.MethodPost, quietPath, ruleBody(31, "22:00", "07:00", ""), mw.Key()...), http.StatusCreated, "")["id"].(string)
	if items := e.Want(e.Do("sam", http.MethodGet, prefsPath, ""), http.StatusOK, "")["items"].([]any); len(items) != 0 {
		t.Fatalf("sam sees %v", items)
	}
	if items := e.Want(e.Do("sam", http.MethodGet, quietPath, ""), http.StatusOK, "")["items"].([]any); len(items) != 0 {
		t.Fatalf("sam sees %v", items)
	}
	e.Want(e.Do("sam", http.MethodPut, rulePath(id), ruleBody(31, "22:00", "07:00", "")), http.StatusNotFound, "not_found")
	e.Want(e.Do("sam", http.MethodDelete, rulePath(id), ""), http.StatusNotFound, "not_found")
	// Sam's own delete on Allgemein touches only Sam's row.
	e.Want(e.Do("sam", http.MethodDelete, prefPath("channel", e.allgemein), ""), http.StatusNoContent, "")
	if e.rows("channel", e.allgemein) != 1 {
		t.Fatal("sam removed alex's row")
	}
	for _, typ := range []string{"notification_pref.updated", "quiet_hours.updated"} {
		for _, ev := range e.events(t, typ) {
			if ev.audience != e.alex.String() {
				t.Fatalf("%s with audience %s", typ, ev.audience)
			}
		}
	}
}
