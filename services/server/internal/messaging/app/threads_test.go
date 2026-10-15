// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// TestCreateThread_ForumPost (brief D8): the title source rules apply, the first message has
// thread_seq 1, the events are thread.created then message.created, a replay with the same
// first_message gives 200, and a changed title with the same client_msg_id gives 422.
func TestCreateThread_ForumPost(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	forum := w.Channel(w.Space("Forum"), "forum", "bugs")
	tag := w.ForumTag(forum, "bug")
	clientID := ids.New()
	req := `{"title":"Invoice export fails","tag_ids":["` + tag.String() + `"],"first_message":` + mw.Body(clientID, "since this morning") + `}`
	before := len(w.Events())
	r := w.Do("owner", http.MethodPost, mw.ThreadsPath(forum), req)
	m := w.Want(r, http.StatusCreated, "")
	th, first := m["thread"].(map[string]any), m["first_message"].(map[string]any)
	if th["title"] != "Invoice export fails" || th["title_source"] != "manual" || th["last_message_seq"] != float64(1) ||
		th["message_count"] != float64(1) || first["thread_seq"] != float64(1) || first["thread_id"] != th["id"] {
		t.Fatalf("created %v", m)
	}
	var types []string
	for _, e := range w.Events()[before:] {
		types = append(types, e.Type)
	}
	if !slices.Equal(types, []string{"thread.created", "message.created"}) {
		t.Fatalf("events %v", types)
	}
	posted := w.Posted()
	if len(posted) != 1 || !posted[0].CreatedThread || posted[0].ThreadKind != "thread" {
		t.Fatalf("MessagePosted %+v", posted)
	}
	replay := w.Do("owner", http.MethodPost, mw.ThreadsPath(forum), req)
	rm := w.Want(replay, http.StatusOK, "")
	if replay.Header.Get("Idempotent-Replay") != "true" || rm["thread"].(map[string]any)["id"] != th["id"] ||
		rm["first_message"].(map[string]any)["id"] != first["id"] {
		t.Fatalf("replay %v", rm)
	}
	changed := `{"title":"Invoice export broken","tag_ids":["` + tag.String() + `"],"first_message":` + mw.Body(clientID, "since this morning") + `}`
	w.Want(w.Do("owner", http.MethodPost, mw.ThreadsPath(forum), changed), http.StatusUnprocessableEntity, "idempotency_key_reuse")
	if w.Count(`SELECT count(*)::int FROM threads WHERE channel_id = $1`, forum.String()) != 1 {
		t.Fatal("a second post")
	}
	// A placeholder title of a person stays auto.
	auto := w.Want(w.Do("owner", http.MethodPost, mw.ThreadsPath(forum), `{"title":"New chat","first_message":`+mw.Body(ids.New(), "x")+`}`),
		http.StatusCreated, "")
	if auto["thread"].(map[string]any)["title_source"] != "auto" {
		t.Errorf("placeholder title %v", auto["thread"])
	}
}

// TestCreateThread_Validation (brief D8): a forum post without first_message, a text channel
// without parent and a text channel with tags are 400; an outsider gets 404, a reader 403.
func TestCreateThread_Validation(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	space := w.Space("Rules")
	forum := w.Channel(space, "forum", "bugs")
	text := w.Channel(space, "text", "general")
	parent := w.Send("owner", w.MainThread(text), "parent", http.StatusCreated)
	cases := []struct {
		ch           ids.ID
		req, ptr, cd string
	}{
		{forum, `{"title":"No message"}`, "/first_message", "required_for_forum_post"},
		{text, `{"first_message":` + mw.Body(ids.New(), "x") + `}`, "/parent_message_id", "required_in_text_channel"},
		{text, `{"parent_message_id":"` + parent["id"].(string) + `","tag_ids":["` + ids.New().String() + `"]}`, "/tag_ids", "tags_only_in_forum"},
	}
	for _, c := range cases {
		p := w.Want(w.Do("owner", http.MethodPost, mw.ThreadsPath(c.ch), c.req, mw.Key()...), http.StatusBadRequest, "validation_failed")
		if ptr, code := mw.Field(p); ptr != c.ptr || code != c.cd {
			t.Errorf("%s: %s %s", c.req, ptr, code)
		}
	}
	if w.Count(`SELECT count(*)::int FROM threads WHERE kind = 'thread'`) != 0 {
		t.Fatal("a refused request created a thread")
	}
	reader := w.Person("reader", false)
	w.Grant("user", reader, space, "read")
	w.Person("outsider", false)
	w.Want(w.Do("reader", http.MethodPost, mw.ThreadsPath(forum), `{"title":"x","first_message":`+mw.Body(ids.New(), "x")+`}`), http.StatusForbidden, "forbidden")
	mw.Same404(t, w.Do("outsider", http.MethodPost, mw.ThreadsPath(forum), `{"title":"x"}`), w.Do("outsider", http.MethodPost, mw.ThreadsPath(ids.New()), `{"title":"x"}`))
}
