// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// MessagePath is the path of updateMessage, deleteMessage and getMessage.
func messagePath(id string) string { return "/api/v1/messages/" + id }

// sent is a message the test sent: its id and version.
type sent struct {
	id      string
	version int64
}

// post sends content as name into thread and returns the new message.
func post(w *mw.World, name string, thread ids.ID, content string) sent {
	m := w.Send(name, thread, content, http.StatusCreated)
	return sent{id: m["id"].(string), version: int64(m["version"].(float64))}
}

// patch sends updateMessage as name with If-Match version ("" for none) and body.
func patch(w *mw.World, name, id, version, body string, headers ...string) *httpservertest.Response {
	if version != "" {
		headers = append(headers, "If-Match", `"`+version+`"`)
	}
	return w.Do(name, http.MethodPatch, messagePath(id), body, headers...)
}

// v formats a version for If-Match.
func v(n int64) string { return strconv.FormatInt(n, 10) }

// content returns a body that sets content_md.
func content(s string) string {
	b, _ := json.Marshal(map[string]string{"content_md": s})
	return string(b)
}

// world returns a world with the text channel of Support, the person alex with write on it and
// alex's first message in the main thread.
func world(t *testing.T) (w *mw.World, space, ch, main ids.ID, alex ids.ID, msg sent) {
	t.Helper()
	w = mw.New(t, subscribe)
	space, ch, main = w.TextChannel()
	alex = w.Person("alex", false)
	w.Grant("user", alex, space, "write")
	msg = post(w, "alex", main, "first version")
	return w, space, ch, main, alex, msg
}

// TestUpdateMessage_MissingIfMatch428: no If-Match is 428 before any lookup, the same for an
// unknown message.
func TestUpdateMessage_MissingIfMatch428(t *testing.T) {
	t.Parallel()
	w, _, _, _, _, msg := world(t)
	w.Want(patch(w, "alex", msg.id, "", content("x")), http.StatusPreconditionRequired, "precondition_required")
	w.Want(patch(w, "alex", ids.New().String(), "", content("x")), http.StatusPreconditionRequired, "precondition_required")
}

// TestUpdateMessage_StaleIfMatch412WithCurrent: a stale or malformed If-Match is 412 with the
// current representation, whose version is the stored one.
func TestUpdateMessage_StaleIfMatch412WithCurrent(t *testing.T) {
	t.Parallel()
	w, _, _, _, _, msg := world(t)
	w.Want(patch(w, "alex", msg.id, v(msg.version), content("second")), http.StatusOK, "")
	for _, im := range []string{v(msg.version), "x"} {
		p := w.Want(patch(w, "alex", msg.id, im, content("third")), http.StatusPreconditionFailed, "version_conflict")
		cur, _ := p["current"].(map[string]any)
		stored := w.Count(`SELECT version::int FROM messages WHERE id = $1`, msg.id)
		if cur == nil || cur["version"] != float64(stored) || cur["content_md"] != "second" || cur["id"] != msg.id {
			t.Fatalf("current %v, stored version %d", p["current"], stored)
		}
	}
}

// TestUpdateMessage_Deleted410: editing a deleted message is 410 message_deleted.
func TestUpdateMessage_Deleted410(t *testing.T) {
	t.Parallel()
	w, _, _, _, _, msg := world(t)
	w.Want(w.Do("alex", http.MethodDelete, messagePath(msg.id), ""), http.StatusNoContent, "")
	w.Want(patch(w, "alex", msg.id, v(msg.version+1), content("again")), http.StatusGone, "message_deleted")
}

// TestUpdateMessage_NotAuthor403: the owner may read Alex's message but is not its author.
func TestUpdateMessage_NotAuthor403(t *testing.T) {
	t.Parallel()
	w, _, _, _, _, msg := world(t)
	w.Want(patch(w, "owner", msg.id, v(msg.version), content("mine now")), http.StatusForbidden, "forbidden")
}

// TestUpdateMessage_InvisibleIs404: Alex on a message in Marketing, which Alex cannot read, gets
// the same 404 as for a random UUIDv7.
func TestUpdateMessage_InvisibleIs404(t *testing.T) {
	t.Parallel()
	w, _, _, _, _, _ := world(t)
	marketing := w.MainThread(w.Channel(w.Space("Marketing"), "text", "general"))
	foreign := post(w, "owner", marketing, "marketing plan")
	a := patch(w, "alex", foreign.id, v(foreign.version), content("x"))
	b := patch(w, "alex", ids.New().String(), "1", content("x"))
	mw.Same404(t, a, b)
	mw.Same404(t, w.Do("alex", http.MethodDelete, messagePath(foreign.id), ""), w.Do("alex", http.MethodDelete, messagePath(ids.New().String()), ""))
	mw.Same404(t, w.Do("alex", http.MethodGet, messagePath(foreign.id)+"/revisions", ""),
		w.Do("alex", http.MethodGet, messagePath(ids.New().String())+"/revisions", ""))
}

// TestUpdateMessage_ReadOnlyChannel403: in a read-only channel the author gets 403
// channel_read_only.
func TestUpdateMessage_ReadOnlyChannel403(t *testing.T) {
	t.Parallel()
	w, _, ch, _, _, msg := world(t)
	w.SQL(`UPDATE channels SET read_only = true, version = version + 1 WHERE id = $1`, ch.String())
	w.SQL(`UPDATE sequence_counters SET value = value + 1 WHERE name = 'permission_version'`)
	w.Want(patch(w, "alex", msg.id, v(msg.version), content("x")), http.StatusForbidden, "channel_read_only")
}

// revisions returns the revisions of id as name.
func revisions(t *testing.T, w *mw.World, name, id string) []map[string]any {
	t.Helper()
	m := w.Want(w.Do(name, http.MethodGet, messagePath(id)+"/revisions", ""), http.StatusOK, "")
	var out []map[string]any
	for _, x := range m["items"].([]any) {
		out = append(out, x.(map[string]any))
	}
	return out
}

// TestUpdateMessage_StoresRevisions: two edits give revision 1 with the original and 2 with the
// first edit; edited_at is set and the version rose by two.
func TestUpdateMessage_StoresRevisions(t *testing.T) {
	t.Parallel()
	w, _, _, _, alex, msg := world(t)
	r := w.Want(patch(w, "alex", msg.id, v(msg.version), content("second version")), http.StatusOK, "")
	r = w.Want(patch(w, "alex", msg.id, v(int64(r["version"].(float64))), content("third version")), http.StatusOK, "")
	if r["version"] != float64(msg.version+2) || r["edited_at"] == nil || r["content_md"] != "third version" {
		t.Fatalf("after two edits %v", r)
	}
	revs := revisions(t, w, "alex", msg.id)
	if len(revs) != 2 || revs[0]["revision"] != float64(1) || revs[0]["content_md"] != "first version" ||
		revs[1]["revision"] != float64(2) || revs[1]["content_md"] != "second version" || revs[0]["edited_by"] != alex.String() {
		t.Fatalf("revisions %v", revs)
	}
}

// TestUpdateMessage_NoChangeIsNoop: the same content, the same content with its mentions and an
// empty body answer the current message without a revision, an event or a new version.
func TestUpdateMessage_NoChangeIsNoop(t *testing.T) {
	t.Parallel()
	w, _, _, _, _, msg := world(t)
	before := len(w.Events())
	for _, body := range []string{content("first version"), `{}`, `{"content_md":"first version","mention_ids":[]}`} {
		r := patch(w, "alex", msg.id, v(msg.version), body)
		m := w.Want(r, http.StatusOK, "")
		if m["version"] != float64(msg.version) || r.Header.Get("ETag") != `"`+v(msg.version)+`"` || m["edited_at"] != nil {
			t.Fatalf("%s: %v %s", body, m, r.Header.Get("ETag"))
		}
	}
	if n := w.Count(`SELECT count(*)::int FROM message_revisions`); n != 0 || len(w.Events()) != before {
		t.Fatalf("%d revisions, %d events, before %d", n, len(w.Events()), before)
	}
}

// mentionsOf returns the mentioned principals of message id from the table.
func mentionsOf(t *testing.T, w *mw.World, id string) []string {
	t.Helper()
	rows, err := w.Owner.Query(t.Context(), `SELECT principal_id::text FROM mentions WHERE message_id = $1 ORDER BY principal_id`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

// TestUpdateMessage_ReplacesMentions: new content without mention_ids resolves its tokens; a
// token that is gone removes its row.
func TestUpdateMessage_ReplacesMentions(t *testing.T) {
	t.Parallel()
	w, space, _, main, _, _ := world(t)
	sam, robin := w.Person("sam", false), w.Person("robin", false)
	w.Grant("user", sam, space, "write")
	w.Grant("user", robin, space, "write")
	msg := post(w, "alex", main, "hi @sam")
	if got := mentionsOf(t, w, msg.id); !slices.Equal(got, []string{sam.String()}) {
		t.Fatalf("mentions %v", got)
	}
	r := w.Want(patch(w, "alex", msg.id, v(msg.version), content("hi @robin")), http.StatusOK, "")
	if got := mentionsOf(t, w, msg.id); !slices.Equal(got, []string{robin.String()}) || mw.JSONList(r["mention_ids"]) != "["+robin.String()+"]" {
		t.Fatalf("mentions %v, answer %v", got, r["mention_ids"])
	}
	thread := w.Count(`SELECT count(*)::int FROM mentions WHERE message_id = $1 AND thread_id = $2`, msg.id, main.String())
	if thread != 1 {
		t.Fatalf("thread copy %d", thread)
	}
}

// TestUpdateMessage_MentionMismatch400: mention_ids that differ from the resolved set are 400
// mention_mismatch, and nothing changes.
func TestUpdateMessage_MentionMismatch400(t *testing.T) {
	t.Parallel()
	w, space, _, _, _, msg := world(t)
	sam := w.Person("sam", false)
	w.Grant("user", sam, space, "write")
	p := w.Want(patch(w, "alex", msg.id, v(msg.version), `{"content_md":"hi @sam","mention_ids":[]}`), http.StatusBadRequest, "validation_failed")
	if ptr, code := mw.Field(p); ptr != "/mention_ids" || code != "mention_mismatch" {
		t.Fatalf("field %s %s", ptr, code)
	}
	if n := w.Count(`SELECT version::int FROM messages WHERE id = $1`, msg.id); n != int(msg.version) {
		t.Fatalf("version %d", n)
	}
}

// TestUpdateMessage_TooLarge413: content above message.max_content_bytes is 413 naming the key;
// an empty content is 400.
func TestUpdateMessage_TooLarge413(t *testing.T) {
	t.Parallel()
	w, _, _, _, _, msg := world(t)
	p := w.Want(patch(w, "alex", msg.id, v(msg.version), content(strings.Repeat("a", mw.MaxContent+1))), http.StatusRequestEntityTooLarge, "payload_too_large")
	if !strings.Contains(p["detail"].(string), "message.max_content_bytes") {
		t.Fatalf("detail %v", p["detail"])
	}
	p = w.Want(patch(w, "alex", msg.id, v(msg.version), content("  ")), http.StatusBadRequest, "validation_failed")
	if ptr, code := mw.Field(p); ptr != "/content_md" || code != "empty" {
		t.Fatalf("field %s %s", ptr, code)
	}
}

// TestUpdateMessage_AppendsMessageUpdated: one message.updated with the new resource version and
// the edited message, without client_msg_id.
func TestUpdateMessage_AppendsMessageUpdated(t *testing.T) {
	t.Parallel()
	w, _, _, main, _, msg := world(t)
	w.Want(patch(w, "alex", msg.id, v(msg.version), content("second version")), http.StatusOK, "")
	evs := w.Events("message.updated")
	if len(evs) != 1 || evs[0].ResourceID != msg.id || evs[0].ThreadID != main.String() || evs[0].ClientMsgID != "" {
		t.Fatalf("events %+v", evs)
	}
	var version int64
	if err := w.Owner.QueryRow(t.Context(), `SELECT resource_version FROM event_log WHERE type = 'message.updated'`).Scan(&version); err != nil || version != msg.version+1 {
		t.Fatalf("resource_version %d, %v", version, err)
	}
	m := evs[0].Payload["message"].(map[string]any)
	if m["content_md"] != "second version" || m["version"] != float64(msg.version+1) || m["edited_at"] == nil {
		t.Fatalf("payload %v", m)
	}
}

// TestUpdateMessage_NeverTriggers: an edit publishes no MessagePosted and leaves the trigger_state
// columns as they are.
func TestUpdateMessage_NeverTriggers(t *testing.T) {
	t.Parallel()
	w, _, _, _, _, msg := world(t)
	w.SQL(`UPDATE messages SET trigger_state = 'failed', trigger_state_reason = 'agent_disabled', trigger_state_at = '2026-10-14T09:00:00Z'
		WHERE id = $1`, msg.id)
	posted := len(w.Posted())
	w.Want(patch(w, "alex", msg.id, v(msg.version), content("@owner second version")), http.StatusOK, "")
	if len(w.Posted()) != posted {
		t.Fatalf("an edit published MessagePosted: %d, before %d", len(w.Posted()), posted)
	}
	n := w.Count(`SELECT count(*)::int FROM messages WHERE id = $1 AND trigger_state = 'failed' AND trigger_state_reason = 'agent_disabled'
		AND trigger_state_at = '2026-10-14T09:00:00Z'`, msg.id)
	if n != 1 {
		t.Fatal("an edit changed trigger_state")
	}
}

// TestUpdateMessage_IdempotencyKeyReplay: the same Idempotency-Key twice gives the stored response
// and one revision.
func TestUpdateMessage_IdempotencyKeyReplay(t *testing.T) {
	t.Parallel()
	w, _, _, _, _, msg := world(t)
	key := mw.Key()
	a := patch(w, "alex", msg.id, v(msg.version), content("second version"), key...)
	b := patch(w, "alex", msg.id, v(msg.version), content("second version"), key...)
	var ma, mb map[string]any
	_ = json.Unmarshal(a.Body, &ma)
	_ = json.Unmarshal(b.Body, &mb)
	if a.StatusCode != http.StatusOK || b.StatusCode != http.StatusOK || !reflect.DeepEqual(ma, mb) || ma["version"] != float64(msg.version+1) {
		t.Fatalf("%d %s / %d %s", a.StatusCode, a.Body, b.StatusCode, b.Body)
	}
	if n := w.Count(`SELECT count(*)::int FROM message_revisions`); n != 1 {
		t.Fatalf("%d revisions", n)
	}
}

// TestUpdateMessage_ConcurrentSameIfMatch: two parallel edits with the same If-Match give one 200
// and one 412.
func TestUpdateMessage_ConcurrentSameIfMatch(t *testing.T) {
	t.Parallel()
	w, _, _, _, _, msg := world(t)
	var wg sync.WaitGroup
	gate := make(chan struct{})
	codes := make([]int, 2)
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			r := w.H.Do(w.H.NewRequest(http.MethodPatch, messagePath(msg.id), content("edit "+strconv.Itoa(i)),
				"Authorization", "Bearer alex", "If-Match", `"`+v(msg.version)+`"`))
			codes[i] = r.StatusCode
		}()
	}
	close(gate)
	wg.Wait()
	slices.Sort(codes)
	if codes[0] != http.StatusOK || codes[1] != http.StatusPreconditionFailed {
		t.Fatalf("statuses %v", codes)
	}
	if n := w.Count(`SELECT count(*)::int FROM message_revisions`); n != 1 {
		t.Fatalf("%d revisions", n)
	}
}

// TestUpdateMessage_SystemLineForbidden: a system line names a person as author but is the
// server's; editing it is 403 (docs/DECISIONS.md M1-58).
func TestUpdateMessage_SystemLineForbidden(t *testing.T) {
	t.Parallel()
	w, _, ch, _, _, _ := world(t)
	_ = ch
	th := w.Want(w.Do("alex", http.MethodPost, mw.ThreadsPath(ch), `{"parent_message_id":"`+firstMessage(t, w, ch)+`","first_message":`+
		mw.Body(ids.New(), "reply")+`}`), http.StatusCreated, "")
	thread := ids.MustParse(th["thread"].(map[string]any)["id"].(string))
	w.Want(w.Do("alex", http.MethodPost, "/api/v1/threads/"+thread.String()+"/close", "", mw.Key()...), http.StatusOK, "")
	post(w, "alex", thread, "reopen")
	var line string
	if err := w.Owner.QueryRow(t.Context(), `SELECT id::text FROM messages WHERE thread_id = $1 AND kind = 'system'`, thread.String()).Scan(&line); err != nil {
		t.Fatal(err)
	}
	w.Want(patch(w, "alex", line, "1", content("x")), http.StatusForbidden, "forbidden")
}

// firstMessage returns the id of the first message of the main thread of ch.
func firstMessage(t *testing.T, w *mw.World, ch ids.ID) string {
	t.Helper()
	var id string
	if err := w.Owner.QueryRow(t.Context(), `SELECT m.id::text FROM messages m JOIN threads th ON th.id = m.thread_id
		WHERE th.channel_id = $1 AND th.kind = 'main' ORDER BY m.thread_seq LIMIT 1`, ch.String()).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
