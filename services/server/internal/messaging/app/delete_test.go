// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/schema_validation"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	syncmod "github.com/OWNER/bothy/services/server/internal/sync"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// del sends deleteMessage as name.
func del(w *mw.World, name, id string) *httpservertest.Response {
	return w.Do(name, http.MethodDelete, messagePath(id), "")
}

// TestDeleteMessage_OwnIs204AndIdempotent: the author deletes; the second call is 204 too, and
// there is exactly one message.deleted, with the resource version of the tombstone.
func TestDeleteMessage_OwnIs204AndIdempotent(t *testing.T) {
	t.Parallel()
	w, _, _, main, alex, msg := world(t)
	w.Want(del(w, "alex", msg.id), http.StatusNoContent, "")
	w.Want(del(w, "alex", msg.id), http.StatusNoContent, "")
	evs := w.Events("message.deleted")
	if len(evs) != 1 || evs[0].ResourceID != msg.id || evs[0].ThreadID != main.String() {
		t.Fatalf("events %+v", evs)
	}
	p := evs[0].Payload
	if p["message_id"] != msg.id || p["thread_id"] != main.String() || p["thread_seq"] != float64(1) || p["deleted_by"] != alex.String() ||
		p["deleted_at"] == nil {
		t.Fatalf("payload %v", p)
	}
	validate(t, "MessageDeletedPayload", evs[0].Raw)
	if n := w.Count(`SELECT count(*)::int FROM event_log WHERE type = 'message.deleted' AND resource_version = $1`, msg.version+1); n != 1 {
		t.Fatalf("resource_version: %d", n)
	}
	if n := w.Count(`SELECT count(*)::int FROM audit_events WHERE action = 'message.delete'`); n != 0 {
		t.Fatalf("own deletion audited: %d", n)
	}
}

// TestDeleteMessage_ForeignNeedsManageAndIsAudited: the owner deletes Alex's message with one
// allow entry without content; Sam with write but without manage gets 403, which the deny path
// records.
func TestDeleteMessage_ForeignNeedsManageAndIsAudited(t *testing.T) {
	t.Parallel()
	w, space, ch, main, alex, _ := world(t)
	sam := w.Person("sam", false)
	w.Grant("user", sam, space, "write")
	msg := post(w, "alex", main, "canary-foreign-delete")
	w.Want(del(w, "sam", msg.id), http.StatusForbidden, "forbidden")
	w.FlushAudit()
	if n := w.Count(`SELECT count(*)::int FROM audit_events WHERE action = 'message.delete' AND decision = 'deny' AND actor_principal_id = $1`,
		sam.String()); n != 1 {
		t.Fatalf("deny entries %d", n)
	}
	w.Want(del(w, "owner", msg.id), http.StatusNoContent, "")
	var details, decision, resource string
	if err := w.Owner.QueryRow(t.Context(), `SELECT details::text, decision, resource_id::text FROM audit_events
		WHERE action = 'message.delete' AND decision = 'allow'`).Scan(&details, &decision, &resource); err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	_ = json.Unmarshal([]byte(details), &d)
	if resource != msg.id || d["thread_id"] != main.String() || d["channel_id"] != ch.String() || d["author_principal_id"] != alex.String() ||
		strings.Contains(details, "canary") || strings.Contains(details, "alex\"") {
		t.Fatalf("audit %s %s %s", decision, resource, details)
	}
	if n := w.Count(`SELECT count(*)::int FROM audit_events WHERE action = 'message.delete' AND decision = 'allow'`); n != 1 {
		t.Fatalf("allow entries %d", n)
	}
}

// threadCounters returns message_count and version of the thread.
func threadCounters(t *testing.T, w *mw.World, thread ids.ID) (int, int) {
	t.Helper()
	var count, version int
	if err := w.Owner.QueryRow(t.Context(), `SELECT message_count, version::int FROM threads WHERE id = $1`, thread.String()).Scan(&count, &version); err != nil {
		t.Fatal(err)
	}
	return count, version
}

// TestDeleteMessage_TombstoneKeepsThreadSeq: the row and thread_seq stay, message_count drops by
// one and the thread version rises by one.
func TestDeleteMessage_TombstoneKeepsThreadSeq(t *testing.T) {
	t.Parallel()
	w, _, _, main, _, msg := world(t)
	post(w, "alex", main, "second")
	count, version := threadCounters(t, w, main)
	w.Want(del(w, "alex", msg.id), http.StatusNoContent, "")
	c2, v2 := threadCounters(t, w, main)
	if c2 != count-1 || v2 != version+1 {
		t.Fatalf("count %d -> %d, version %d -> %d", count, c2, version, v2)
	}
	if got := w.Seqs(main); !slices.Equal(got, []int64{1, 2}) {
		t.Fatalf("thread_seq %v", got)
	}
	if n := w.Count(`SELECT count(*)::int FROM messages WHERE id = $1 AND thread_seq = 1 AND content_md = '' AND deleted_at IS NOT NULL`, msg.id); n != 1 {
		t.Fatal("no tombstone")
	}
}

// TestDeleteMessage_ConcurrentDeletesDecrementOnce: parallel deletions of one message are all 204
// and count it down once.
func TestDeleteMessage_ConcurrentDeletesDecrementOnce(t *testing.T) {
	t.Parallel()
	w, _, _, main, _, msg := world(t)
	count, _ := threadCounters(t, w, main)
	var wg sync.WaitGroup
	gate := make(chan struct{})
	codes := make([]int, 4)
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			codes[i] = w.H.Do(w.H.NewRequest(http.MethodDelete, messagePath(msg.id), "", "Authorization", "Bearer alex")).StatusCode
		}()
	}
	close(gate)
	wg.Wait()
	for _, c := range codes {
		if c != http.StatusNoContent {
			t.Fatalf("statuses %v", codes)
		}
	}
	if c, _ := threadCounters(t, w, main); c != count-1 {
		t.Fatalf("count %d, before %d", c, count)
	}
	if n := len(w.Events("message.deleted")); n != 1 {
		t.Fatalf("%d message.deleted", n)
	}
}

// TestDeleteMessage_EditDeleteRace: an edit and a deletion in parallel: either the edit wins and
// the deletion follows, or the edit gets 410; never a revision after the tombstone.
func TestDeleteMessage_EditDeleteRace(t *testing.T) {
	t.Parallel()
	w, _, _, main, _, _ := world(t)
	for round := range 5 {
		msg := post(w, "alex", main, "race")
		var wg sync.WaitGroup
		gate := make(chan struct{})
		var edit, remove int
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-gate
			edit = w.H.Do(w.H.NewRequest(http.MethodPatch, messagePath(msg.id), content("edited"), "Authorization", "Bearer alex",
				"If-Match", `"`+v(msg.version)+`"`)).StatusCode
		}()
		go func() {
			defer wg.Done()
			<-gate
			remove = w.H.Do(w.H.NewRequest(http.MethodDelete, messagePath(msg.id), "", "Authorization", "Bearer alex")).StatusCode
		}()
		close(gate)
		wg.Wait()
		if remove != http.StatusNoContent || (edit != http.StatusOK && edit != http.StatusGone) {
			t.Fatalf("round %d: edit %d, delete %d", round, edit, remove)
		}
		if n := w.Count(`SELECT count(*)::int FROM message_revisions WHERE message_id = $1`, msg.id); n != 0 {
			t.Fatalf("round %d: %d revisions after the tombstone", round, n)
		}
		if n := w.Count(`SELECT count(*)::int FROM messages WHERE id = $1 AND content_md = '' AND deleted_at IS NOT NULL`, msg.id); n != 1 {
			t.Fatalf("round %d: no tombstone", round)
		}
	}
}

// TestDeleteMessage_PublishesMessageDeleted: the test subscriber receives MessageDeleted with the
// remembered mentions.
func TestDeleteMessage_PublishesMessageDeleted(t *testing.T) {
	t.Parallel()
	w, space, ch, main, alex, _ := world(t)
	sam := w.Person("sam", false)
	w.Grant("user", sam, space, "write")
	msg := post(w, "alex", main, "hi @sam")
	w.Want(del(w, "owner", msg.id), http.StatusNoContent, "")
	got := w.Deleted()
	if len(got) != 1 {
		t.Fatalf("events %+v", got)
	}
	ev := got[0]
	if ev.MessageID.String() != msg.id || ev.ThreadID != main || ev.ChannelID != ch || ev.ThreadSeq != 2 || ev.AuthorID != alex ||
		ev.DeletedBy != w.OwnerID || !slices.Equal(ev.MentionedPrincipalIDs, []ids.ID{sam}) || ev.Version != msg.version+1 {
		t.Fatalf("event %+v", ev)
	}
}

// eventRow is one event_log row as text per column.
type eventRow map[string]string

// messageEvents returns every event_log row of resource message id as column texts, by seq.
func messageEvents(t *testing.T, w *mw.World, id string) []eventRow {
	t.Helper()
	rows, err := w.Owner.Query(t.Context(), `SELECT to_jsonb(l) FROM event_log l WHERE resource_kind = 'message' AND resource_id = $1 ORDER BY seq`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []eventRow
	for rows.Next() {
		var raw map[string]any
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		r := eventRow{}
		for k, val := range raw {
			b, _ := json.Marshal(val)
			r[k] = string(b)
		}
		out = append(out, r)
	}
	return out
}

// TestDPT01_DeleteMessageRemovesContent (20 DP-T01, part messages; R-DM-18): a message with a
// canary, a mention and two edits is deleted; afterwards no content, revision, mention or payload
// holds the canary, every earlier event is redacted with its envelope unchanged, and the
// delivered envelope says redacted. The search vector follows with M3-13 (messages has no
// search_vector on this base).
func TestDPT01_DeleteMessageRemovesContent(t *testing.T) {
	t.Parallel()
	w, space, _, main, _, _ := world(t)
	sam := w.Person("sam", false)
	w.Grant("user", sam, space, "write")
	const canary = "canary-7f3a9c"
	msg := post(w, "alex", main, "@sam "+canary+" one")
	r := w.Want(patch(w, "alex", msg.id, v(msg.version), content("@sam "+canary+" two")), http.StatusOK, "")
	w.Want(patch(w, "alex", msg.id, v(int64(r["version"].(float64))), content("@sam "+canary+" three")), http.StatusOK, "")
	before := messageEvents(t, w, msg.id)
	if len(before) != 3 {
		t.Fatalf("%d events before", len(before))
	}
	w.Want(del(w, "alex", msg.id), http.StatusNoContent, "")

	if n := w.Count(`SELECT count(*)::int FROM messages WHERE id = $1 AND content_md = ''`, msg.id); n != 1 {
		t.Fatal("content left")
	}
	for _, q := range []string{
		`SELECT count(*)::int FROM message_revisions WHERE message_id = $1`,
		`SELECT count(*)::int FROM mentions WHERE message_id = $1`,
	} {
		if n := w.Count(q, msg.id); n != 0 {
			t.Fatalf("%s: %d", q, n)
		}
	}
	if n := w.Count(`SELECT count(*)::int FROM event_log WHERE payload::text LIKE '%' || $1 || '%'`, canary); n != 0 {
		t.Fatalf("%d payloads hold the canary", n)
	}
	after := messageEvents(t, w, msg.id)
	if len(after) != 4 || after[3]["type"] != `"message.deleted"` || after[3]["redacted_at"] != "null" {
		t.Fatalf("events after %v", after)
	}
	for i := range before {
		if after[i]["redacted_at"] == "null" {
			t.Errorf("event %d not redacted", i)
		}
		for k, val := range before[i] {
			if k != "payload" && k != "redacted_at" && after[i][k] != val {
				t.Errorf("event %d column %s changed: %s, before %s", i, k, after[i][k], val)
			}
		}
		validate(t, "MessagePayload", after[i]["payload"])
	}
	evs, err := syncpg.NewRepository(w.Pool, slog.New(slog.DiscardHandler)).EventsAfter(context.Background(), 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	redacted := 0
	for _, ev := range evs {
		if ev.ResourceID.String() != msg.id || ev.Type == "message.deleted" {
			continue
		}
		b, _ := json.Marshal(ev.Envelope(syncmod.DeliveryFields{}))
		if !strings.Contains(string(b), `"redacted":true`) || strings.Contains(string(b), canary) {
			t.Fatalf("envelope %s", b)
		}
		redacted++
	}
	if redacted != 3 {
		t.Fatalf("%d redacted envelopes", redacted)
	}
	if !reflect.DeepEqual(revisions(t, w, "alex", msg.id), []map[string]any(nil)) {
		t.Fatal("a deleted message lists revisions")
	}
}

// TestListMessageRevisions_OrderAndVisibility: ascending revisions for a reader of the channel,
// 404 for an invisible message, an empty list for a deleted one.
func TestListMessageRevisions_OrderAndVisibility(t *testing.T) {
	t.Parallel()
	w, space, _, _, _, msg := world(t)
	reader := w.Person("reader", false)
	w.Grant("user", reader, space, "read")
	version := msg.version
	for _, c := range []string{"two", "three", "four"} {
		r := w.Want(patch(w, "alex", msg.id, v(version), content(c)), http.StatusOK, "")
		version = int64(r["version"].(float64))
	}
	revs := revisions(t, w, "reader", msg.id)
	var got []string
	for i, r := range revs {
		if r["revision"] != float64(i+1) {
			t.Fatalf("revisions %v", revs)
		}
		got = append(got, r["content_md"].(string))
	}
	if !slices.Equal(got, []string{"first version", "two", "three"}) {
		t.Fatalf("contents %v", got)
	}
	w.Person("outsider", false)
	mw.Same404(t, w.Do("outsider", http.MethodGet, messagePath(msg.id)+"/revisions", ""),
		w.Do("outsider", http.MethodGet, messagePath(ids.New().String())+"/revisions", ""))
	w.Want(del(w, "alex", msg.id), http.StatusNoContent, "")
	m := w.Want(w.Do("reader", http.MethodGet, messagePath(msg.id)+"/revisions", ""), http.StatusOK, "")
	if items, ok := m["items"].([]any); !ok || len(items) != 0 {
		t.Fatalf("deleted message revisions %v", m)
	}
}

// validate checks p against the schema name of docs/api/openapi.yaml.
func validate(t *testing.T, name, p string) {
	t.Helper()
	doc, err := libopenapi.NewDocument(rest.Spec)
	if err != nil {
		t.Fatal(err)
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		t.Fatal(err)
	}
	proxy, ok := model.Model.Components.Schemas.Get(name)
	if !ok {
		t.Fatalf("no schema %s", name)
	}
	if valid, problems := schema_validation.NewSchemaValidator().ValidateSchemaString(proxy.Schema(), p); !valid {
		for _, pr := range problems {
			t.Errorf("schema %s: %s %s", name, pr.Message, pr.Reason)
		}
		t.Fatalf("payload does not match %s: %s", name, p)
	}
}
