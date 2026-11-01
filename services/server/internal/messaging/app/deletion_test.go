// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// forumPost creates a forum post with title and a first message as the owner and returns the
// forum, the thread and the first message.
func forumPost(w *mw.World, title, content string) (forum, thread ids.ID, first sent) {
	forum = w.Channel(w.Space("Forum"), "forum", "bugs")
	m := w.Want(w.Do("owner", http.MethodPost, mw.ThreadsPath(forum), `{"title":"`+title+`","first_message":`+mw.Body(ids.New(), content)+`}`),
		http.StatusCreated, "")
	th, fm := m["thread"].(map[string]any), m["first_message"].(map[string]any)
	return forum, ids.MustParse(th["id"].(string)), sent{id: fm["id"].(string), version: int64(fm["version"].(float64))}
}

// purge runs the job messaging.purge_deleted once with a batch of two.
func purge(t *testing.T, w *mw.World) int {
	t.Helper()
	p, ok := w.Threads.(chat.Purges)
	if !ok {
		t.Fatal("the threads of chat are not the purge port")
	}
	n, err := w.Svc.Purge(p, 2).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// TestDeletedThread_ReadsAndWrites (07 4.5a, D-DEL-3): a deleted chat leaves the post list, its
// thread and history answer 404, a new message and an edit in it are 404; its deletion state stays
// readable; a restore within the undo window brings everything back.
func TestDeletedThread_ReadsAndWrites(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	forum, th, first := forumPost(w, "Invoice export fails", "since this morning")
	post(w, "owner", th, "second message")

	info := w.Want(w.Do("owner", http.MethodPost, "/api/v1/threads/"+th.String()+"/delete", "", mw.Key()...), http.StatusOK, "")
	contents := info["contents"].(map[string]any)
	if info["state"] != "pending" || info["target_kind"] != "thread" || contents["threads"] != float64(1) || contents["messages"] != float64(2) ||
		info["undo_until"] == nil {
		t.Fatalf("deletion %v", info)
	}
	w.Want(w.Do("owner", http.MethodGet, "/api/v1/threads/"+th.String(), ""), http.StatusNotFound, "not_found")
	w.Want(w.Do("owner", http.MethodGet, mw.MessagesPath(th), ""), http.StatusNotFound, "not_found")
	w.Want(w.Do("owner", http.MethodPost, mw.MessagesPath(th), mw.Body(ids.New(), "too late")), http.StatusNotFound, "not_found")
	w.Want(patch(w, "owner", first.id, v(first.version), content("changed")), http.StatusNotFound, "not_found")
	list := w.Want(w.Do("owner", http.MethodGet, mw.ThreadsPath(forum), ""), http.StatusOK, "")
	if items := list["items"].([]any); len(items) != 0 {
		t.Fatalf("a deleted thread is listed: %v", items)
	}
	if got := w.Want(w.Do("owner", http.MethodGet, "/api/v1/threads/"+th.String()+"/deletion", ""), http.StatusOK, ""); got["state"] != "pending" {
		t.Fatalf("deletion state %v", got)
	}

	restored := w.Want(w.Do("owner", http.MethodPost, "/api/v1/threads/"+th.String()+"/restore", "", mw.Key()...), http.StatusOK, "")
	if _, ok := restored["deleted_at"]; ok {
		t.Fatalf("restored thread %v", restored)
	}
	w.Want(w.Do("owner", http.MethodGet, mw.MessagesPath(th), ""), http.StatusOK, "")
	post(w, "owner", th, "back again")
}

// TestPurgeDeleted (D-DEL-1, 20 4.2a): before the undo window ends the job leaves a deleted chat
// alone; after it, the messages go through the deletion path (tombstone without deleted_by, the
// event payloads redacted, message.deleted without deleted_by, no audit entry), the title goes,
// purged_at is set and the deletion is final; a second run changes nothing.
func TestPurgeDeleted(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, th, first := forumPost(w, "canary-purge-title", "canary-purge-first")
	second := post(w, "owner", th, "canary-purge-second")
	_, _, main := w.TextChannel()
	kept := post(w, "owner", main, "another thread")
	w.Want(w.Do("owner", http.MethodPost, "/api/v1/threads/"+th.String()+"/delete", "", mw.Key()...), http.StatusOK, "")

	if n := purge(t, w); n != 0 {
		t.Fatalf("purged %d messages inside the undo window", n)
	}
	w.Clock.Advance(31 * time.Second)
	if n := purge(t, w); n != 2 {
		t.Fatalf("purged %d messages, want 2", n)
	}
	for _, id := range []string{first.id, second.id} {
		if n := w.Count(`SELECT count(*)::int FROM messages WHERE id = $1 AND deleted_at IS NOT NULL AND deleted_by IS NULL AND content_md = ''`, id); n != 1 {
			t.Errorf("message %s is not a tombstone without deleted_by", id)
		}
	}
	if n := w.Count(`SELECT count(*)::int FROM messages WHERE id = $1 AND deleted_at IS NULL`, kept.id); n != 1 {
		t.Error("a message of another thread was purged")
	}
	if n := w.Count(`SELECT count(*)::int FROM threads WHERE id = $1 AND title = '' AND purged_at IS NOT NULL AND message_count = 0`, th.String()); n != 1 {
		t.Error("the thread keeps its title or has no purged_at")
	}
	if n := w.Count(`SELECT count(*)::int FROM event_log WHERE type LIKE 'message.%' AND payload::text LIKE '%canary-purge%'`); n != 0 {
		t.Errorf("message events still hold the content: %d", n)
	}
	for _, e := range w.Events("message.deleted") {
		if _, ok := e.Payload["deleted_by"]; ok {
			t.Errorf("message.deleted of the purge names deleted_by: %v", e.Payload)
		}
	}
	w.FlushAudit()
	if n := w.Count(`SELECT count(*)::int FROM audit_events WHERE action = 'message.delete'`); n != 0 {
		t.Errorf("the purge was audited: %d", n)
	}
	if n := purge(t, w); n != 0 {
		t.Fatalf("a second run purged %d messages", n)
	}
	got := w.Want(w.Do("owner", http.MethodGet, "/api/v1/threads/"+th.String()+"/deletion", ""), http.StatusOK, "")
	if got["state"] != "final" || got["contents"].(map[string]any)["messages"] != float64(0) {
		t.Fatalf("deletion after the purge %v", got)
	}
	w.Want(w.Do("owner", http.MethodPost, "/api/v1/threads/"+th.String()+"/restore", "", mw.Key()...), http.StatusConflict, "conflict")
}

// TestPurgeDeleted_ChannelAndSpace: the purge of a deleted channel removes the messages of all its
// threads, clears its topic and the titles of its threads and sets purged_at on the channel; the
// purge of a deleted space does the same for every channel of the space and sets purged_at on the
// space.
func TestPurgeDeleted_ChannelAndSpace(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	space, ch, main := w.TextChannel()
	w.Want(w.Do("owner", http.MethodPatch, "/api/v1/channels/"+ch.String(), `{"topic":"canary-topic"}`, "If-Match", `"1"`), http.StatusOK, "")
	m := post(w, "owner", main, "canary-channel")
	other := w.Channel(space, "text", "other")
	o := post(w, "owner", w.MainThread(other), "canary-space")
	w.Want(w.Do("owner", http.MethodPost, "/api/v1/channels/"+ch.String()+"/delete", "", mw.Key()...), http.StatusOK, "")
	w.Want(w.Do("owner", http.MethodPost, "/api/v1/spaces/"+space.String()+"/delete", "", mw.Key()...), http.StatusOK, "")
	w.Clock.Advance(31 * time.Second)
	if n := purge(t, w); n != 2 {
		t.Fatalf("purged %d messages, want 2", n)
	}
	for _, id := range []string{m.id, o.id} {
		if n := w.Count(`SELECT count(*)::int FROM messages WHERE id = $1 AND deleted_at IS NOT NULL AND content_md = ''`, id); n != 1 {
			t.Errorf("message %s is not deleted", id)
		}
	}
	if n := w.Count(`SELECT count(*)::int FROM channels WHERE id = $1 AND topic = '' AND purged_at IS NOT NULL`, ch.String()); n != 1 {
		t.Error("the channel keeps its topic or has no purged_at")
	}
	if n := w.Count(`SELECT count(*)::int FROM spaces WHERE id = $1 AND purged_at IS NOT NULL`, space.String()); n != 1 {
		t.Error("the space has no purged_at")
	}
	if n := purge(t, w); n != 0 {
		t.Fatalf("a second run purged %d messages", n)
	}
}
