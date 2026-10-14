// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// TestCreateThread_FromChannelMessage (brief D8): a thread from a main thread message captures
// the effective operator; a second thread of the same parent is 409; a parent of another channel
// and a deleted parent are 400; without first_message the Idempotency-Key replay works.
func TestCreateThread_FromChannelMessage(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, ch, main := w.TextChannel()
	op := w.Agent("operator-agent")
	w.SetOperator(ch, op)
	parent := w.Send("owner", main, "start a thread here", http.StatusCreated)
	k := mw.Key()
	req := `{"parent_message_id":"` + parent["id"].(string) + `"}`
	m := w.Want(w.Do("owner", http.MethodPost, mw.ThreadsPath(ch), req, k...), http.StatusCreated, "")
	th := m["thread"].(map[string]any)
	if th["operator_agent_id"] != op.String() || th["parent_message_id"] != parent["id"] || th["last_message_seq"] != float64(0) {
		t.Fatalf("thread %v", th)
	}
	if _, ok := m["first_message"]; ok {
		t.Fatal("a first message without one")
	}
	again := w.Do("owner", http.MethodPost, mw.ThreadsPath(ch), req, k...)
	if am := w.Want(again, http.StatusCreated, ""); am["thread"].(map[string]any)["id"] != th["id"] || again.Header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("Idempotency-Key replay %v %v", again.Header, am)
	}
	w.Want(w.Do("owner", http.MethodPost, mw.ThreadsPath(ch), req, mw.Key()...), http.StatusConflict, "conflict")
	other := w.MainThread(w.Channel(w.Space("Other"), "text", "general"))
	foreign := w.Send("owner", other, "elsewhere", http.StatusCreated)
	gone := w.Send("owner", main, "to delete", http.StatusCreated)
	w.SQL(`UPDATE messages SET deleted_at = now(), content_md = '' WHERE id = $1`, gone["id"])
	for _, id := range []string{foreign["id"].(string), gone["id"].(string), ids.New().String()} {
		p := w.Want(w.Do("owner", http.MethodPost, mw.ThreadsPath(ch), `{"parent_message_id":"`+id+`"}`, mw.Key()...), http.StatusBadRequest, "validation_failed")
		if ptr, code := mw.Field(p); ptr != "/parent_message_id" || code != "not_in_main_thread" {
			t.Fatalf("%s: %s %s", id, ptr, code)
		}
	}
	// With a first message: the thread and its message in one unit.
	p2 := w.Send("owner", main, "second parent", http.StatusCreated)
	wm := w.Want(w.Do("owner", http.MethodPost, mw.ThreadsPath(ch), `{"parent_message_id":"`+p2["id"].(string)+`","first_message":`+mw.Body(ids.New(), "in the thread")+`}`),
		http.StatusCreated, "")
	if wm["first_message"].(map[string]any)["thread_seq"] != float64(1) || wm["thread"].(map[string]any)["message_count"] != float64(1) {
		t.Fatalf("thread with first message %v", wm)
	}
}

// TestCreateThread_AppWithWriteScopeOnly (05 section 2.6): an app with messages:write and no
// messages:read creates a forum post with its first message and gets 201 with the thread; the
// answer must not depend on thread.read, or the committed post would be reported as refused
// (red team M1-56).
func TestCreateThread_AppWithWriteScopeOnly(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	space := w.Space("Forum")
	forum := w.Channel(space, "forum", "bugs")
	app := w.App("deploy-app", "messages:write")
	w.Grant("app", app, space, "write")
	req := `{"title":"Build failed","first_message":` + mw.Body(ids.New(), "details") + `}`
	m := w.Want(w.Do("deploy-app", http.MethodPost, mw.ThreadsPath(forum), req), http.StatusCreated, "")
	if m["thread"].(map[string]any)["message_count"] != float64(1) {
		t.Fatalf("thread %v", m)
	}
	again := w.Do("deploy-app", http.MethodPost, mw.ThreadsPath(forum), req)
	w.Want(again, http.StatusOK, "")
	if again.Header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("replay headers %v", again.Header)
	}
}
