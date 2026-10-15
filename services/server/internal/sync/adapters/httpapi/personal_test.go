// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// TestMain starts the PostgreSQL containers of pgtest; the world is test/messagingworld with the
// personal data of sync (NewPersonal).
func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// posted is the test subscriber of MessagePosted; it runs after the subscriber of sync, so its
// veto rolls back what sync did in the same unit of work.
type posted struct{ rec *mw.Recorder }

func (p posted) onPosted(ctx context.Context, ev messaging.MessagePosted) error {
	return p.rec.On(ctx, ev)
}

func subscribe(reg *txn.Registry, rec *mw.Recorder) error {
	return txn.Subscribe(reg, posted{rec}.onPosted)
}

// env is a world with a space, a text channel with its main thread and a forum, and Alex and Sam
// with write on the space.
type env struct {
	*mw.World
	space, text, main, forum ids.ID
	alex, sam                ids.ID
}

func newEnv(t *testing.T) *env {
	t.Helper()
	w := mw.NewPersonal(t, subscribe)
	space, text, main := w.TextChannel()
	e := &env{World: w, space: space, text: text, main: main, forum: w.Channel(space, "forum", "ideas")}
	e.alex, e.sam = w.Person("alex", false), w.Person("sam", false)
	w.Grant("user", e.alex, space, "write")
	w.Grant("user", e.sam, space, "write")
	return e
}

// sendN posts n messages as the owner into thread.
func (e *env) sendN(thread ids.ID, n int) {
	for i := range n {
		e.Send("owner", thread, "message "+strconv.Itoa(i), http.StatusCreated)
	}
}

func readStatePath(thread ids.ID) string   { return "/api/v1/threads/" + thread.String() + "/read-state" }
func markUnreadPath(thread ids.ID) string  { return readStatePath(thread) + "/mark-unread" }
func threadDraftPath(thread ids.ID) string { return "/api/v1/threads/" + thread.String() + "/draft" }
func channelDraftPath(ch ids.ID) string    { return "/api/v1/channels/" + ch.String() + "/draft" }
func settingPath(key string) string        { return "/api/v1/me/settings/" + key }
func readStatesPath(ch ids.ID) string      { return "/api/v1/me/read-states?channel_id=" + ch.String() }

// advance puts the read position of token name in thread and returns the answer.
func (e *env) advance(name string, thread ids.ID, seq int) map[string]any {
	return e.Want(e.Do(name, http.MethodPut, readStatePath(thread), `{"last_read_seq":`+strconv.Itoa(seq)+`}`), http.StatusOK, "")
}

// num returns the JSON number m[k] as int.
func num(m map[string]any, k string) int {
	f, _ := m[k].(float64)
	return int(f)
}

// strictDecode decodes raw into v and fails on unknown members, so a payload holds exactly the
// members of its schema.
func strictDecode(t *testing.T, raw string, v any) {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		t.Fatalf("payload %s: %v", raw, err)
	}
}

// eventRow is the envelope columns of one personal event.
type eventRow struct {
	typ, audience, space, channel, thread, resourceKind, resourceID, correlation string
	resourceVersion                                                              int64
	truncated                                                                    bool
	payload                                                                      string
}

// personalEvents returns the events of type t in order.
func (e *env) personalEvents(t *testing.T, typ string) []eventRow {
	t.Helper()
	rs, err := e.Owner.Query(context.Background(), `SELECT type, coalesce(audience_principal_id::text, ''), coalesce(space_id::text, ''),
		coalesce(channel_id::text, ''), coalesce(thread_id::text, ''), resource_kind, coalesce(resource_id::text, ''),
		coalesce(resource_version, 0), truncated, payload::text, coalesce(correlation_id::text, '')
		FROM event_log WHERE type = $1 ORDER BY seq`, typ)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	var out []eventRow
	for rs.Next() {
		var r eventRow
		if err := rs.Scan(&r.typ, &r.audience, &r.space, &r.channel, &r.thread, &r.resourceKind, &r.resourceID, &r.resourceVersion,
			&r.truncated, &r.payload, &r.correlation); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(r.payload, "null") {
			t.Errorf("JSON null in %s", r.payload)
		}
		out = append(out, r)
	}
	return out
}

// TestPutReadStateMonotonic: advance to 10, then to 5: the value stays 10, the second call writes
// no event and keeps the version; 11 is out of range and 0 on a thread without a row writes
// nothing.
func TestPutReadStateMonotonic(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.sendN(e.main, 10)
	if m := e.advance("alex", e.main, 0); num(m, "version") != 0 || num(m, "last_read_seq") != 0 {
		t.Fatalf("advance to 0 without a row: %v", m)
	}
	if n := e.Count(`SELECT count(*)::int FROM read_states`); n != 0 {
		t.Fatalf("%d rows after advancing to 0", n)
	}
	first := e.advance("alex", e.main, 10)
	if num(first, "last_read_seq") != 10 || num(first, "version") != 1 {
		t.Fatalf("first %v", first)
	}
	second := e.advance("alex", e.main, 5)
	if num(second, "last_read_seq") != 10 || num(second, "version") != 1 {
		t.Fatalf("second %v", second)
	}
	if n := len(e.personalEvents(t, "read_state.updated")); n != 1 {
		t.Fatalf("%d read_state.updated", n)
	}
	p := e.Want(e.Do("alex", http.MethodPut, readStatePath(e.main), `{"last_read_seq":11}`), http.StatusBadRequest, "validation_failed")
	if ptr, code := mw.Field(p); ptr != "/last_read_seq" || code != "out_of_range" {
		t.Fatalf("out of range %s %s", ptr, code)
	}
}

// TestReadStateEventIsPersonal: the event_log row has type read_state.updated, the audience of
// the person, the scope of the thread, a payload of exactly ReadStatePayload with the calling
// device and no JSON null; the resource is the thread without a version.
func TestReadStateEventIsPersonal(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.sendN(e.main, 3)
	e.advance("alex", e.main, 2)
	evs := e.personalEvents(t, "read_state.updated")
	if len(evs) != 1 {
		t.Fatalf("%d events", len(evs))
	}
	ev := evs[0]
	if ev.audience != e.alex.String() || ev.space != e.space.String() || ev.channel != e.text.String() || ev.thread != e.main.String() ||
		ev.resourceKind != "thread" || ev.resourceID != e.main.String() || ev.resourceVersion != 0 {
		t.Fatalf("envelope %+v", ev)
	}
	var p rest.ReadStatePayload
	strictDecode(t, ev.payload, &p)
	if p.ReadState.UpdatedByDeviceId == nil || ids.ID(*p.ReadState.UpdatedByDeviceId) != e.DeviceOf("alex") || p.ReadState.LastReadSeq != 2 ||
		p.ReadState.Version != 1 || ids.ID(p.ReadState.ThreadId) != e.main || ids.ID(p.ReadState.ChannelId) != e.text {
		t.Fatalf("payload %s", ev.payload)
	}
	var receive, kind string
	if err := e.Owner.QueryRow(context.Background(), `SELECT receive_action, actor_kind FROM event_log WHERE type = 'read_state.updated'`).Scan(&receive, &kind); err != nil {
		t.Fatal(err)
	}
	if receive != "self" || kind != "user" {
		t.Fatalf("receive %s actor %s", receive, kind)
	}
}

// TestMarkThreadUnreadPreconditions: a missing If-Match is 428, a stale one 412 with current; a
// match moves the position back, answers the ETag and writes the event; above the position is
// 400.
func TestMarkThreadUnreadPreconditions(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.sendN(e.main, 5)
	e.advance("alex", e.main, 5)
	e.Want(e.Do("alex", http.MethodPost, markUnreadPath(e.main), `{"last_read_seq":2}`), http.StatusPreconditionRequired, "precondition_required")
	p := e.Want(e.Do("alex", http.MethodPost, markUnreadPath(e.main), `{"last_read_seq":2}`, "If-Match", `"7"`), http.StatusPreconditionFailed, "version_conflict")
	cur, _ := p["current"].(map[string]any)
	if num(cur, "version") != 1 || num(cur, "last_read_seq") != 5 {
		t.Fatalf("current %v", p)
	}
	r := e.Do("alex", http.MethodPost, markUnreadPath(e.main), `{"last_read_seq":2}`, append(mw.Key(), "If-Match", `"1"`)...)
	m := e.Want(r, http.StatusOK, "")
	if r.Header.Get("ETag") != `"2"` || num(m, "last_read_seq") != 2 || num(m, "version") != 2 {
		t.Fatalf("mark unread %v %s", m, r.Header.Get("ETag"))
	}
	if n := len(e.personalEvents(t, "read_state.updated")); n != 2 {
		t.Fatalf("%d events", n)
	}
	p = e.Want(e.Do("alex", http.MethodPost, markUnreadPath(e.main), `{"last_read_seq":3}`, "If-Match", `"2"`), http.StatusBadRequest, "validation_failed")
	if ptr, code := mw.Field(p); ptr != "/last_read_seq" || code != "out_of_range" {
		t.Fatalf("forward %s %s", ptr, code)
	}
	// The same value: 200 without event; a thread without a row has version 0.
	e.Want(e.Do("alex", http.MethodPost, markUnreadPath(e.main), `{"last_read_seq":2}`, "If-Match", `"2"`), http.StatusOK, "")
	other := e.MainThread(e.Channel(e.space, "text", "random"))
	if m := e.Want(e.Do("alex", http.MethodPost, markUnreadPath(other), `{"last_read_seq":0}`, "If-Match", `"0"`), http.StatusOK, ""); num(m, "version") != 0 {
		t.Fatalf("implicit %v", m)
	}
	if n := len(e.personalEvents(t, "read_state.updated")); n != 2 {
		t.Fatalf("%d events after no-ops", n)
	}
}

// TestMarkThreadUnreadTwoDevices: two devices of Alex send the same If-Match at once: exactly one
// 200 and one 412.
func TestMarkThreadUnreadTwoDevices(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.Device("alex-phone", e.alex)
	e.sendN(e.main, 5)
	for round := range 3 {
		v := num(e.advance("alex", e.main, 5), "version")
		var wg sync.WaitGroup
		gate := make(chan struct{})
		codes := make([]int, 2)
		for i, tok := range []string{"alex", "alex-phone"} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-gate
				r := e.Do(tok, http.MethodPost, markUnreadPath(e.main), `{"last_read_seq":`+strconv.Itoa(1+i)+`}`, "If-Match", `"`+strconv.Itoa(v)+`"`)
				codes[i] = r.StatusCode
			}()
		}
		close(gate)
		wg.Wait()
		if codes[0]+codes[1] != http.StatusOK+http.StatusPreconditionFailed || (codes[0] != http.StatusOK && codes[1] != http.StatusOK) {
			t.Fatalf("round %d: %v", round, codes)
		}
	}
}

// TestMentionCount: Sam mentions Alex in two messages: listMyReadStates shows mention_count 2 and
// the mentions wrote no read_state.updated; advancing past the first says 1 in the answer and
// the event, past both 0; a deleted mentioning message lowers the count without a write.
func TestMentionCount(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.sendN(e.main, 1)
	e.advance("alex", e.main, 1)
	e.Send("sam", e.main, "hello @alex", http.StatusCreated)
	second := e.Send("sam", e.main, "again @alex", http.StatusCreated)
	list := e.Want(e.Do("alex", http.MethodGet, readStatesPath(e.text), ""), http.StatusOK, "")
	items, _ := list["items"].([]any)
	if len(items) != 1 || num(items[0].(map[string]any), "mention_count") != 2 {
		t.Fatalf("list %v", list)
	}
	if n := len(e.personalEvents(t, "read_state.updated")); n != 1 {
		t.Fatalf("%d events after the mentions", n)
	}
	if m := e.advance("alex", e.main, 2); num(m, "mention_count") != 1 {
		t.Fatalf("past the first %v", m)
	}
	evs := e.personalEvents(t, "read_state.updated")
	var p rest.ReadStatePayload
	strictDecode(t, evs[len(evs)-1].payload, &p)
	if p.ReadState.MentionCount != 1 {
		t.Fatalf("event %s", evs[len(evs)-1].payload)
	}
	if m := e.advance("alex", e.main, 3); num(m, "mention_count") != 0 {
		t.Fatalf("past both %v", m)
	}
	m := e.Want(e.Do("alex", http.MethodPost, markUnreadPath(e.main), `{"last_read_seq":1}`, "If-Match", `"3"`), http.StatusOK, "")
	if num(m, "mention_count") != 2 {
		t.Fatalf("back %v", m)
	}
	before := e.Count(`SELECT version::int FROM read_states`)
	e.SQL(`UPDATE messages SET deleted_at = now(), content_md = '' WHERE id = $1`, second["id"].(string))
	list = e.Want(e.Do("alex", http.MethodGet, readStatesPath(e.text), ""), http.StatusOK, "")
	items, _ = list["items"].([]any)
	if num(items[0].(map[string]any), "mention_count") != 1 || e.Count(`SELECT version::int FROM read_states`) != before {
		t.Fatalf("after the delete %v", list)
	}
}

// putDraft puts body as the draft of path for token name and returns the answer.
func (e *env) putDraft(name, path, body string) map[string]any {
	return e.Want(e.Do(name, http.MethodPut, path, body), http.StatusOK, "")
}

// draftOf returns the draft member of a DraftPutResult answer.
func draftOf(m map[string]any) map[string]any {
	d, _ := m["draft"].(map[string]any)
	return d
}

// TestDraftsTwoDevicesServerOrder: device A puts without base (1, false), device B with base 1
// (2, false), A again with base 1 (3, true).
func TestDraftsTwoDevicesServerOrder(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.Device("alex-phone", e.alex)
	for i, c := range []struct {
		tok, body string
		version   int
		overwrote bool
	}{
		{"alex", `{"content_md":"one"}`, 1, false},
		{"alex-phone", `{"content_md":"two","base_version":1}`, 2, false},
		{"alex", `{"content_md":"three","base_version":1}`, 3, true},
	} {
		m := e.putDraft(c.tok, threadDraftPath(e.main), c.body)
		if num(draftOf(m), "version") != c.version || m["overwrote_newer"] != c.overwrote {
			t.Fatalf("step %d: %v", i, m)
		}
	}
	evs := e.personalEvents(t, "draft.updated")
	if len(evs) != 3 || evs[2].resourceVersion != 3 || evs[2].thread != e.main.String() || evs[2].audience != e.alex.String() {
		t.Fatalf("events %+v", evs)
	}
	var p rest.DraftPayload
	strictDecode(t, evs[1].payload, &p)
	if p.Draft.UpdatedByDeviceId == nil || ids.ID(*p.Draft.UpdatedByDeviceId) != e.DeviceOf("alex-phone") || p.Draft.ContentMd != "two" {
		t.Fatalf("payload %s", evs[1].payload)
	}
}

// TestDraftsConcurrentFirstWrite: two parallel first PUTs without base both answer 200, one row
// remains with versions 1 and 2 answered and exactly one overwrote_newer.
func TestDraftsConcurrentFirstWrite(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.Device("alex-phone", e.alex)
	for round := range 4 {
		thread := e.MainThread(e.Channel(e.space, "text", "race-"+strconv.Itoa(round)))
		var wg sync.WaitGroup
		gate := make(chan struct{})
		res := make([]*httpservertest.Response, 2)
		for i, tok := range []string{"alex", "alex-phone"} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-gate
				res[i] = e.Do(tok, http.MethodPut, threadDraftPath(thread), `{"content_md":"from `+tok+`"}`)
			}()
		}
		close(gate)
		wg.Wait()
		versions, overwrote := map[int]bool{}, 0
		for _, r := range res {
			m := e.Want(r, http.StatusOK, "")
			versions[num(draftOf(m), "version")] = true
			if m["overwrote_newer"] == true {
				overwrote++
			}
		}
		if !versions[1] || !versions[2] || overwrote != 1 || e.Count(`SELECT count(*)::int FROM drafts WHERE thread_id = $1`, thread.String()) != 1 {
			t.Fatalf("round %d: versions %v, overwrote %d", round, versions, overwrote)
		}
	}
}

// TestClearDraftSameTransaction: createMessage with clear_draft removes the draft, and
// draft.deleted and message.created carry the same correlation_id; a unit of work that ran
// OnMessagePosted and then rolled back keeps the draft.
func TestClearDraftSameTransaction(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.putDraft("alex", threadDraftPath(e.main), `{"content_md":"almost"}`)
	e.SetVeto(true)
	e.Want(e.Do("alex", http.MethodPost, mw.MessagesPath(e.main), mw.Body(ids.New(), "send", `"clear_draft":true`)), http.StatusInternalServerError, "")
	if n := e.Count(`SELECT count(*)::int FROM drafts`); n != 1 {
		t.Fatalf("%d drafts after the rollback", n)
	}
	e.SetVeto(false)
	e.Send("alex", e.main, "send", http.StatusCreated, `"clear_draft":true`)
	if n := e.Count(`SELECT count(*)::int FROM drafts`); n != 0 {
		t.Fatalf("%d drafts after sending", n)
	}
	del := e.personalEvents(t, "draft.deleted")
	created := e.personalEvents(t, "message.created")
	if len(del) != 1 || len(created) != 1 || del[0].correlation == "" || del[0].correlation != created[0].correlation {
		t.Fatalf("deleted %+v created %+v", del, created)
	}
	var p rest.DraftDeletedPayload
	strictDecode(t, del[0].payload, &p)
	if p.ThreadId == nil || ids.ID(*p.ThreadId) != e.main || ids.ID(p.ChannelId) != e.text || del[0].resourceVersion != 0 {
		t.Fatalf("payload %s %+v", del[0].payload, del[0])
	}
	// Without clear_draft the draft stays.
	e.putDraft("alex", threadDraftPath(e.main), `{"content_md":"keep"}`)
	e.Send("alex", e.main, "other", http.StatusCreated)
	if n := e.Count(`SELECT count(*)::int FROM drafts`); n != 1 {
		t.Fatalf("%d drafts without clear_draft", n)
	}
}

// TestClearDraftForumPost: createThread in a forum with first_message.clear_draft deletes the
// channel draft.
func TestClearDraftForumPost(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	m := e.putDraft("alex", channelDraftPath(e.forum), `{"content_md":"idea"}`)
	if draftOf(m)["thread_id"] != nil {
		t.Fatalf("channel draft with a thread %v", m)
	}
	body := `{"title":"Idea","first_message":` + mw.Body(ids.New(), "idea", `"clear_draft":true`) + `}`
	e.Want(e.Do("alex", http.MethodPost, mw.ThreadsPath(e.forum), body, mw.Key()...), http.StatusCreated, "")
	if n := e.Count(`SELECT count(*)::int FROM drafts`); n != 0 {
		t.Fatalf("%d drafts", n)
	}
	del := e.personalEvents(t, "draft.deleted")
	if len(del) != 1 || del[0].thread != "" || del[0].channel != e.forum.String() {
		t.Fatalf("deleted %+v", del)
	}
}

// TestEmptyDraftPutDeletes: an empty draft deletes the row like DELETE, writes draft.deleted and
// answers the empty final state; without a row it answers version 1 and writes nothing; DELETE
// without a draft is 204.
func TestEmptyDraftPutDeletes(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.putDraft("alex", threadDraftPath(e.main), `{"content_md":"x"}`)
	e.putDraft("alex", threadDraftPath(e.main), `{"content_md":"y","base_version":1}`)
	m := e.putDraft("alex", threadDraftPath(e.main), `{"content_md":"  \n","base_version":2}`)
	d := draftOf(m)
	if num(d, "version") != 3 || d["content_md"] != "  \n" && d["content_md"] != "" || len(d["file_ids"].([]any)) != 0 || m["overwrote_newer"] != false {
		t.Fatalf("empty put %v", m)
	}
	if d["content_md"] != "" {
		t.Fatalf("final content %q", d["content_md"])
	}
	if e.Count(`SELECT count(*)::int FROM drafts`) != 0 || len(e.personalEvents(t, "draft.deleted")) != 1 {
		t.Fatal("the empty draft was not deleted with its event")
	}
	m = e.putDraft("alex", threadDraftPath(e.main), `{"content_md":""}`)
	if num(draftOf(m), "version") != 1 || len(e.personalEvents(t, "draft.deleted")) != 1 {
		t.Fatalf("empty without a row %v", m)
	}
	e.Want(e.Do("alex", http.MethodDelete, threadDraftPath(e.main), ""), http.StatusNoContent, "")
	e.putDraft("alex", threadDraftPath(e.main), `{"content_md":"z"}`)
	e.Want(e.Do("alex", http.MethodDelete, threadDraftPath(e.main), ""), http.StatusNoContent, "")
	if e.Count(`SELECT count(*)::int FROM drafts`) != 0 || len(e.personalEvents(t, "draft.deleted")) != 2 {
		t.Fatal("DELETE did not delete with its event")
	}
}

// TestDraftValidation: content above message.max_content_bytes (413 as for messages), duplicate
// and too many files, a reply outside the thread, a channel draft in a text channel and with a
// reply target.
func TestDraftValidation(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	other := e.MainThread(e.Channel(e.space, "text", "random"))
	foreign := e.Send("owner", other, "elsewhere", http.StatusCreated)["id"].(string)
	own := e.Send("owner", e.main, "here", http.StatusCreated)["id"].(string)
	big, _ := json.Marshal(strings.Repeat("x", mw.MaxContent+1))
	e.Want(e.Do("alex", http.MethodPut, threadDraftPath(e.main), `{"content_md":`+string(big)+`}`), http.StatusRequestEntityTooLarge, "payload_too_large")
	f := ids.New().String()
	for _, c := range []struct{ body, ptr, code string }{
		{`{"content_md":"x","file_ids":["` + f + `","` + f + `"]}`, "/file_ids/1", "duplicate"},
		{`{"content_md":"x","file_ids":["` + ids.New().String() + `","` + ids.New().String() + `","` + ids.New().String() + `","` + ids.New().String() + `"]}`, "/file_ids", "too_many_items"},
		{`{"content_md":"x","reply_to_message_id":"` + foreign + `"}`, "/reply_to_message_id", "not_in_thread"},
		{`{"content_md":"x","reply_to_message_id":"` + ids.New().String() + `"}`, "/reply_to_message_id", "not_in_thread"},
	} {
		p := e.Want(e.Do("alex", http.MethodPut, threadDraftPath(e.main), c.body), http.StatusBadRequest, "validation_failed")
		if ptr, code := mw.Field(p); ptr != c.ptr || code != c.code {
			t.Errorf("%s: %s %s", c.body, ptr, code)
		}
	}
	m := e.putDraft("alex", threadDraftPath(e.main), `{"content_md":"x","reply_to_message_id":"`+own+`","file_ids":["`+f+`"]}`)
	if draftOf(m)["reply_to_message_id"] != own {
		t.Fatalf("reply %v", m)
	}
	e.Want(e.Do("alex", http.MethodPut, channelDraftPath(e.text), `{"content_md":"x"}`), http.StatusBadRequest, "validation_failed")
	p := e.Want(e.Do("alex", http.MethodPut, channelDraftPath(e.forum), `{"content_md":"x","reply_to_message_id":"`+own+`"}`), http.StatusBadRequest, "validation_failed")
	if ptr, _ := mw.Field(p); ptr != "/reply_to_message_id" {
		t.Fatalf("reply on a channel draft %v", p)
	}
}

// TestDraftReadOnlyChannel: saving and deleting a draft succeed in a read-only and in an archived
// channel; sending from there is 403 channel_read_only.
func TestDraftReadOnlyChannel(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.SQL(`UPDATE channels SET read_only = true, version = version + 1 WHERE id = $1`, e.text.String())
	e.putDraft("alex", threadDraftPath(e.main), `{"content_md":"x"}`)
	e.Want(e.Do("alex", http.MethodPost, mw.MessagesPath(e.main), mw.Body(ids.New(), "x")), http.StatusForbidden, "channel_read_only")
	e.Want(e.Do("alex", http.MethodDelete, threadDraftPath(e.main), ""), http.StatusNoContent, "")
	e.putDraft("alex", channelDraftPath(e.forum), `{"content_md":"post"}`)
	e.Want(e.Do("owner", http.MethodPost, "/api/v1/channels/"+e.forum.String()+"/archive", `{"reason":"done"}`, mw.Key()...), http.StatusOK, "")
	e.putDraft("alex", channelDraftPath(e.forum), `{"content_md":"post 2"}`)
	body := `{"title":"Idea","first_message":` + mw.Body(ids.New(), "idea") + `}`
	e.Want(e.Do("alex", http.MethodPost, mw.ThreadsPath(e.forum), body, mw.Key()...), http.StatusForbidden, "channel_read_only")
	e.Want(e.Do("alex", http.MethodDelete, channelDraftPath(e.forum), ""), http.StatusNoContent, "")
	if n := e.Count(`SELECT count(*)::int FROM drafts`); n != 0 {
		t.Fatalf("%d drafts", n)
	}
}

// TestDraftEventTruncated: a draft near message.max_content_bytes gives a draft.updated payload
// below sync.event_payload_max_bytes with truncated; the REST answer keeps the whole text.
func TestDraftEventTruncated(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	text := strings.Repeat("ä", mw.MaxContent/2)
	b, _ := json.Marshal(text)
	m := e.putDraft("alex", threadDraftPath(e.main), `{"content_md":`+string(b)+`}`)
	if draftOf(m)["content_md"] != text {
		t.Fatal("the answer was cut")
	}
	evs := e.personalEvents(t, "draft.updated")
	if len(evs) != 1 || !evs[0].truncated {
		t.Fatalf("events %+v", evs)
	}
	var p rest.DraftPayload
	strictDecode(t, evs[0].payload, &p)
	// The appender bounds the payload as appended, the compact form; jsonb::text adds spaces.
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(evs[0].payload)); err != nil {
		t.Fatal(err)
	}
	if len(p.Draft.ContentMd) == 0 || !strings.HasPrefix(text, p.Draft.ContentMd) || compact.Len() >= mw.PayloadMax {
		t.Fatalf("payload of %d bytes, content %d bytes", compact.Len(), len(p.Draft.ContentMd))
	}
}

// TestSettingsPutValidation: an unknown key is 404, an invalid value 400 on /value; a valid value
// is stored with version 1 and its event.
func TestSettingsPutValidation(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.Want(e.Do("alex", http.MethodPut, settingPath("appearance.font"), `{"value":"x"}`), http.StatusNotFound, "not_found")
	for _, c := range []struct{ key, value string }{
		{"appearance.theme", `"blue"`}, {"composer.enter_sends", `1`}, {"layout.column_widths", `{"sidebar":{"w":1}}`},
		{"layout.column_widths", `{"sidebar":-5}`}, {"notify.default_level", `"loud"`},
	} {
		p := e.Want(e.Do("alex", http.MethodPut, settingPath(c.key), `{"value":`+c.value+`}`), http.StatusBadRequest, "validation_failed")
		if ptr, _ := mw.Field(p); ptr != "/value" {
			t.Errorf("%s=%s: %v", c.key, c.value, p)
		}
	}
	m := e.Want(e.Do("alex", http.MethodPut, settingPath("layout.column_widths"), `{"value":{"sidebar":240,"post_list":480}}`), http.StatusOK, "")
	if num(m, "version") != 1 || m["key"] != "layout.column_widths" {
		t.Fatalf("put %v", m)
	}
	evs := e.personalEvents(t, "settings.updated")
	if len(evs) != 1 || evs[0].resourceKind != "user" || evs[0].resourceID != e.alex.String() || evs[0].space != "" || evs[0].resourceVersion != 0 {
		t.Fatalf("events %+v", evs)
	}
	var p rest.SettingsPayload
	strictDecode(t, evs[0].payload, &p)
	if p.Setting.Key != "layout.column_widths" || p.Deleted != nil || p.Setting.UpdatedByDeviceId == nil {
		t.Fatalf("payload %s", evs[0].payload)
	}
	list := e.Want(e.Do("alex", http.MethodGet, "/api/v1/me/settings", ""), http.StatusOK, "")
	if items, _ := list["items"].([]any); len(items) != 1 {
		t.Fatalf("list %v", list)
	}
}

// TestSettingsPutSameValueNoEvent: the stored value answers 200 without event and version.
func TestSettingsPutSameValueNoEvent(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.Want(e.Do("alex", http.MethodPut, settingPath("appearance.theme"), `{"value":"dark"}`), http.StatusOK, "")
	m := e.Want(e.Do("alex", http.MethodPut, settingPath("appearance.theme"), `{"value":"dark"}`), http.StatusOK, "")
	if num(m, "version") != 1 || len(e.personalEvents(t, "settings.updated")) != 1 {
		t.Fatalf("same value %v", m)
	}
	m = e.Want(e.Do("alex", http.MethodPut, settingPath("appearance.theme"), `{"value":"light"}`), http.StatusOK, "")
	if num(m, "version") != 2 || len(e.personalEvents(t, "settings.updated")) != 2 {
		t.Fatalf("new value %v", m)
	}
}

// TestSettingsDeleteWritesDeletedEvent: deleting a stored key writes settings.updated with the
// last value, version last + 1 and deleted; a later PUT starts again at version 1.
func TestSettingsDeleteWritesDeletedEvent(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.Want(e.Do("alex", http.MethodPut, settingPath("composer.enter_sends"), `{"value":false}`), http.StatusOK, "")
	e.Want(e.Do("alex", http.MethodPut, settingPath("composer.enter_sends"), `{"value":true}`), http.StatusOK, "")
	e.Want(e.Do("alex", http.MethodDelete, settingPath("composer.enter_sends"), ""), http.StatusNoContent, "")
	evs := e.personalEvents(t, "settings.updated")
	var p rest.SettingsPayload
	strictDecode(t, evs[len(evs)-1].payload, &p)
	if p.Deleted == nil || !*p.Deleted || p.Setting.Version != 3 || p.Setting.Value != true {
		t.Fatalf("payload %s", evs[len(evs)-1].payload)
	}
	if m := e.Want(e.Do("alex", http.MethodPut, settingPath("composer.enter_sends"), `{"value":true}`), http.StatusOK, ""); num(m, "version") != 1 {
		t.Fatalf("restart %v", m)
	}
}

// TestSettingsDeleteUnset204: a known key that is not stored is 204 without event; an unknown key
// is 404.
func TestSettingsDeleteUnset204(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.Want(e.Do("alex", http.MethodDelete, settingPath("appearance.density"), ""), http.StatusNoContent, "")
	e.Want(e.Do("alex", http.MethodDelete, settingPath("appearance.font"), ""), http.StatusNotFound, "not_found")
	if n := len(e.personalEvents(t, "settings.updated")); n != 0 {
		t.Fatalf("%d events", n)
	}
}

// TestDPT10ForeignReadStateNotRetrievable (DP-T10): Sam sees none of Alex's read states, drafts
// or settings through any of the twelve operations, and Alex's events name Alex as audience.
func TestDPT10ForeignReadStateNotRetrievable(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.sendN(e.main, 3)
	e.advance("alex", e.main, 3)
	e.putDraft("alex", threadDraftPath(e.main), `{"content_md":"secret draft"}`)
	e.putDraft("alex", channelDraftPath(e.forum), `{"content_md":"secret post"}`)
	e.Want(e.Do("alex", http.MethodPut, settingPath("appearance.theme"), `{"value":"dark"}`), http.StatusOK, "")
	e.Want(e.Do("owner", http.MethodPut, "/api/v1/me/sidebar/spaces/"+e.space.String(), `{"collapsed":true}`), http.StatusOK, "")
	for _, path := range []string{readStatesPath(e.text), "/api/v1/me/drafts", "/api/v1/me/settings", "/api/v1/me/sidebar"} {
		r := e.Do("sam", http.MethodGet, path, "")
		m := e.Want(r, http.StatusOK, "")
		if items, _ := m["items"].([]any); len(items) != 0 || bytes.Contains(r.Body, []byte("secret")) {
			t.Errorf("%s for Sam: %s", path, r.Body)
		}
	}
	// Sam's writes touch only Sam's rows.
	if m := e.advance("sam", e.main, 1); num(m, "version") != 1 || num(m, "last_read_seq") != 1 {
		t.Fatalf("sam advance %v", m)
	}
	if m := e.Want(e.Do("sam", http.MethodPost, markUnreadPath(e.main), `{"last_read_seq":0}`, "If-Match", `"1"`), http.StatusOK, ""); num(m, "last_read_seq") != 0 {
		t.Fatalf("sam mark unread %v", m)
	}
	if m := e.putDraft("sam", threadDraftPath(e.main), `{"content_md":"mine"}`); num(draftOf(m), "version") != 1 || m["overwrote_newer"] != false {
		t.Fatalf("sam draft %v", m)
	}
	e.Want(e.Do("sam", http.MethodDelete, threadDraftPath(e.main), ""), http.StatusNoContent, "")
	e.putDraft("sam", channelDraftPath(e.forum), `{"content_md":"mine"}`)
	e.Want(e.Do("sam", http.MethodDelete, channelDraftPath(e.forum), ""), http.StatusNoContent, "")
	e.Want(e.Do("sam", http.MethodPut, settingPath("appearance.theme"), `{"value":"light"}`), http.StatusOK, "")
	e.Want(e.Do("sam", http.MethodDelete, settingPath("appearance.theme"), ""), http.StatusNoContent, "")
	if e.Count(`SELECT last_read_seq::int FROM read_states WHERE user_id = $1`, e.alex.String()) != 3 ||
		e.Count(`SELECT count(*)::int FROM drafts WHERE user_id = $1`, e.alex.String()) != 2 ||
		e.Count(`SELECT count(*)::int FROM user_settings WHERE user_id = $1`, e.alex.String()) != 1 {
		t.Fatal("Sam changed a row of Alex")
	}
	n := e.Count(`SELECT count(*)::int FROM event_log WHERE type IN ('read_state.updated','draft.updated','draft.deleted','settings.updated')
		AND audience_principal_id IS DISTINCT FROM actor_principal_id`)
	if n != 0 {
		t.Fatalf("%d personal events for another person", n)
	}
}

// sameAnswer checks that two answers have the same status, body and headers apart from the
// correlation id and the rate limit state.
func sameAnswer(t *testing.T, a, b *httpservertest.Response) {
	t.Helper()
	if a.StatusCode != b.StatusCode || !bytes.Equal(a.Body, b.Body) {
		t.Fatalf("answers differ: %d %s / %d %s", a.StatusCode, a.Body, b.StatusCode, b.Body)
	}
	for k := range a.Header {
		switch http.CanonicalHeaderKey(k) {
		case "Correlation-Id", "Ratelimit", "Ratelimit-Policy", "Date":
			continue
		}
		if a.Header.Get(k) != b.Header.Get(k) {
			t.Fatalf("header %s differs: %q %q", k, a.Header.Get(k), b.Header.Get(k))
		}
	}
	if len(a.Header) != len(b.Header) {
		t.Fatalf("headers differ: %v %v", a.Header, b.Header)
	}
}

// TestListMyReadStatesInvisibleChannel: a foreign and an unknown channel give the same status,
// body and headers.
func TestListMyReadStatesInvisibleChannel(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	hidden := e.Channel(e.Space("Hidden"), "text", "general")
	e.sendN(e.MainThread(hidden), 1)
	e.advance("owner", e.MainThread(hidden), 1)
	a := e.Do("alex", http.MethodGet, readStatesPath(hidden), "")
	b := e.Do("alex", http.MethodGet, readStatesPath(ids.New()), "")
	e.Want(a, http.StatusOK, "")
	sameAnswer(t, a, b)
	// Writes on an invisible thread are the same 404 as on an unknown one.
	mw.Same404(t, e.Do("alex", http.MethodPut, readStatePath(e.MainThread(hidden)), `{"last_read_seq":1}`),
		e.Do("alex", http.MethodPut, readStatePath(ids.New()), `{"last_read_seq":1}`))
	mw.Same404(t, e.Do("alex", http.MethodPut, threadDraftPath(e.MainThread(hidden)), `{"content_md":"x"}`),
		e.Do("alex", http.MethodPut, threadDraftPath(ids.New()), `{"content_md":"x"}`))
	mw.Same404(t, e.Do("alex", http.MethodPut, channelDraftPath(hidden), `{"content_md":"x"}`),
		e.Do("alex", http.MethodPut, channelDraftPath(ids.New()), `{"content_md":"x"}`))
}

// TestListMyDraftsHidesLostChannels: after the grant is revoked the draft is no longer listed and
// is still stored.
func TestListMyDraftsHidesLostChannels(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.putDraft("alex", threadDraftPath(e.main), `{"content_md":"x"}`)
	e.putDraft("alex", channelDraftPath(e.forum), `{"content_md":"y"}`)
	m := e.Want(e.Do("alex", http.MethodGet, "/api/v1/me/drafts", ""), http.StatusOK, "")
	if items, _ := m["items"].([]any); len(items) != 2 {
		t.Fatalf("before %v", m)
	}
	e.SQL(`UPDATE grants SET revoked_at = now(), revoked_by = $2, revoke_reason = 'fixture' WHERE subject_principal_id = $1`, e.alex.String(), e.OwnerID.String())
	e.SQL(`UPDATE sequence_counters SET value = value + 1 WHERE name = 'permission_version'`)
	m = e.Want(e.Do("alex", http.MethodGet, "/api/v1/me/drafts", ""), http.StatusOK, "")
	if items, _ := m["items"].([]any); len(items) != 0 {
		t.Fatalf("after %v", m)
	}
	if n := e.Count(`SELECT count(*)::int FROM drafts`); n != 2 {
		t.Fatalf("%d drafts stored", n)
	}
}

// TestGetMySidebarVisibleOnly: the sidebar lists the own rows of readable spaces that are not
// archived.
func TestGetMySidebarVisibleOnly(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	other := e.Space("Other")
	e.Want(e.Do("alex", http.MethodPut, "/api/v1/me/sidebar/spaces/"+e.space.String(), `{"collapsed":true}`), http.StatusOK, "")
	e.Want(e.Do("owner", http.MethodPut, "/api/v1/me/sidebar/spaces/"+other.String(), `{"collapsed":true}`), http.StatusOK, "")
	// A row of Alex for a space Alex cannot read (written as a fixture).
	e.SQL(`INSERT INTO user_space_states (user_id, space_id, collapsed) VALUES ($1, $2, true)`, e.alex.String(), other.String())
	m := e.Want(e.Do("alex", http.MethodGet, "/api/v1/me/sidebar", ""), http.StatusOK, "")
	items, _ := m["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["space_id"] != e.space.String() {
		t.Fatalf("sidebar %v", m)
	}
}

// TestPersonalOperationsRejectAgentToken: an agent with write on the space gets 403
// subject_kind_not_allowed on each of the twelve operations, an app too.
func TestPersonalOperationsRejectAgentToken(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	agent := e.Agent("helper")
	e.Grant("agent", agent, e.space, "write")
	e.Grant("app", e.App("tool", "messages:read", "messages:write"), e.space, "write")
	e.sendN(e.main, 1)
	calls := []struct{ method, path, body string }{
		{http.MethodGet, readStatesPath(e.text), ""},
		{http.MethodPut, readStatePath(e.main), `{"last_read_seq":1}`},
		{http.MethodPost, markUnreadPath(e.main), `{"last_read_seq":0}`},
		{http.MethodPut, threadDraftPath(e.main), `{"content_md":"x"}`},
		{http.MethodDelete, threadDraftPath(e.main), ""},
		{http.MethodPut, channelDraftPath(e.forum), `{"content_md":"x"}`},
		{http.MethodDelete, channelDraftPath(e.forum), ""},
		{http.MethodGet, "/api/v1/me/drafts", ""},
		{http.MethodGet, "/api/v1/me/settings", ""},
		{http.MethodPut, settingPath("appearance.theme"), `{"value":"dark"}`},
		{http.MethodDelete, settingPath("appearance.theme"), ""},
		{http.MethodGet, "/api/v1/me/sidebar", ""},
	}
	for _, tok := range []string{"helper", "tool"} {
		for _, c := range calls {
			p := e.Want(e.Do(tok, c.method, c.path, c.body, "If-Match", `"0"`), http.StatusForbidden, "forbidden")
			if p["reason"] != "subject_kind_not_allowed" {
				t.Errorf("%s %s %s: %v", tok, c.method, c.path, p)
			}
		}
	}
}
