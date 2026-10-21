// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// TestCreateMessage_Replay200 (04 section 5.1): the same request twice gives 201, then 200 with
// Idempotent-Replay, the same id and thread_seq, and exactly one message.created.
func TestCreateMessage_Replay200(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	b := mw.Body(ids.New(), "hello @owner")
	first := w.Do("owner", http.MethodPost, mw.MessagesPath(main), b)
	m1 := w.Want(first, http.StatusCreated, "")
	if first.Header.Get("Idempotent-Replay") != "" {
		t.Error("Idempotent-Replay on the first answer")
	}
	second := w.Do("owner", http.MethodPost, mw.MessagesPath(main), b)
	m2 := w.Want(second, http.StatusOK, "")
	if second.Header.Get("Idempotent-Replay") != "true" {
		t.Errorf("Idempotent-Replay = %q", second.Header.Get("Idempotent-Replay"))
	}
	if m1["id"] != m2["id"] || m1["thread_seq"] != m2["thread_seq"] || m1["content_md"] != m2["content_md"] {
		t.Errorf("replay differs: %v vs %v", m1, m2)
	}
	if !slices.Equal(m1["mention_ids"].([]any), m2["mention_ids"].([]any)) || len(m1["mention_ids"].([]any)) != 1 {
		t.Errorf("mentions %v vs %v", m1["mention_ids"], m2["mention_ids"])
	}
	if n := len(w.MessageEvents(main)); n != 1 {
		t.Fatalf("%d message.created events, want 1", n)
	}
	if len(w.Posted()) != 1 {
		t.Fatalf("MessagePosted %d times", len(w.Posted()))
	}
}

// TestCreateMessage_MaxContentBytes: exactly message.max_content_bytes is accepted, one byte more
// is 413 payload_too_large naming the key.
func TestCreateMessage_MaxContentBytes(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	m := w.Send("owner", main, strings.Repeat("a", mw.MaxContent), http.StatusCreated)
	if len(m["content_md"].(string)) != mw.MaxContent {
		t.Fatal("content changed")
	}
	p := w.Want(w.Do("owner", http.MethodPost, mw.MessagesPath(main), mw.Body(ids.New(), strings.Repeat("a", mw.MaxContent+1))),
		http.StatusRequestEntityTooLarge, "payload_too_large")
	if !strings.Contains(p["detail"].(string), "message.max_content_bytes") {
		t.Errorf("detail %v", p["detail"])
	}
	for _, c := range []string{"", "  \n\t "} {
		p := w.Want(w.Do("owner", http.MethodPost, mw.MessagesPath(main), mw.Body(ids.New(), c)), http.StatusBadRequest, "validation_failed")
		if ptr, code := mw.Field(p); ptr != "/content_md" || code != "empty" {
			t.Errorf("%q: %s %s", c, ptr, code)
		}
	}
	p = w.Want(w.Do("owner", http.MethodPost, mw.MessagesPath(main), `{"client_msg_id":"`+ids.New().String()+`"}`), http.StatusBadRequest, "validation_failed")
	if ptr, code := mw.Field(p); ptr != "/content_md" || code != "empty" {
		t.Errorf("absent content: %s %s", ptr, code)
	}
}

// TestCreateMessage_NotEnabledMembers400 (brief D2, M2-15 design note 6 a): kind voice,
// attachments and run_id on a text message are refused with not_enabled; a card without
// components, components without kind card and a card of a person get their card rules; nothing
// is written.
func TestCreateMessage_NotEnabledMembers400(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	for _, c := range []struct{ extra, pointer, code string }{
		{`"kind":"voice"`, "/kind", "not_enabled"},
		{`"kind":"card"`, "/components", "required"},
		{`"attachment_file_ids":["` + ids.New().String() + `"]`, "/attachment_file_ids", "not_enabled"},
		{`"components":[{"custom_id":"ok","type":"button","definition":{}}]`, "/kind", "invalid"},
		{`"kind":"card","components":[{"custom_id":"ok","type":"button","definition":{}}]`, "/components", "invalid"},
		{`"run_id":"` + ids.New().String() + `"`, "/run_id", "not_enabled"},
	} {
		r := w.Do("owner", http.MethodPost, mw.MessagesPath(main), mw.Body(ids.New(), "x", c.extra))
		if r.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", c.extra, r.StatusCode, r.Body)
		}
		p := r.Problem(t)
		if ptr, code := mw.Field(p); ptr != c.pointer || code != c.code {
			t.Errorf("%s: %s %s", c.extra, ptr, code)
		}
	}
	w.Send("owner", main, "x", http.StatusCreated, `"attachment_file_ids":[]`, `"components":[]`)
	p := w.Want(w.Do("owner", http.MethodPost, mw.MessagesPath(main), mw.Body(ids.New(), "x", `"origin":"system"`)), http.StatusBadRequest, "validation_failed")
	if ptr, code := mw.Field(p); ptr != "/origin" || code != "unknown_member" {
		t.Errorf("origin member: %s %s", ptr, code)
	}
	p = w.Want(w.Do("owner", http.MethodPost, mw.MessagesPath(main), `{"client_msg_id":"0192f4a0-7b1c-4f4e-8959-c3574b6ad93b","content_md":"x"}`),
		http.StatusBadRequest, "validation_failed")
	if ptr, code := mw.Field(p); ptr != "/client_msg_id" || code != "not_uuid_v7" {
		t.Errorf("v4 id: %s %s", ptr, code)
	}
	if w.Count(`SELECT count(*)::int FROM messages`) != 1 {
		t.Fatal("a refused request wrote")
	}
}

// TestCreateMessage_OriginFromPrincipal (09 section 3.2): a person gives human, the operator
// Agent gives agent, an app with messages:write gives app.
func TestCreateMessage_OriginFromPrincipal(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	space, ch, main := w.TextChannel()
	op := w.Agent("operator-agent")
	w.SetOperator(ch, op)
	app := w.App("deploy-app", "messages:write", "messages:read")
	w.Grant("app", app, space, "write")
	for name, want := range map[string]string{"owner": "human", "operator-agent": "agent", "deploy-app": "app"} {
		m := w.Send(name, main, "from "+name, http.StatusCreated)
		if m["origin"] != want || m["author"].(map[string]any)["kind"] == "" {
			t.Errorf("%s: origin %v", name, m["origin"])
		}
	}
	noScope := w.App("quiet-app", "messages:read")
	w.Grant("app", noScope, space, "write")
	w.Want(w.Do("quiet-app", http.MethodPost, mw.MessagesPath(main), mw.Body(ids.New(), "x")), http.StatusForbidden, "scope_missing")
}

// TestCreateMessage_RightsAndLeaks (05 rule 2, 02 section 9.2): read-only member 403; outsider
// 404 identical to an unknown thread; archived channel 403 channel_read_only with reason
// channel_archived; a replay after losing access is 404.
func TestCreateMessage_RightsAndLeaks(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	space, ch, main := w.TextChannel()
	reader := w.Person("reader", false)
	w.Grant("user", reader, space, "read")
	w.Person("outsider", false)
	w.Want(w.Do("reader", http.MethodPost, mw.MessagesPath(main), mw.Body(ids.New(), "x")), http.StatusForbidden, "forbidden")
	a := w.Do("outsider", http.MethodPost, mw.MessagesPath(main), mw.Body(ids.New(), "x"))
	b := w.Do("outsider", http.MethodPost, mw.MessagesPath(ids.New()), mw.Body(ids.New(), "x"))
	mw.Same404(t, a, b)
	c := w.Do("owner", http.MethodPost, mw.MessagesPath(ids.New()), mw.Body(ids.New(), "x"))
	if c.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown thread for the owner: %d", c.StatusCode)
	}
	// A writer loses access; the replay of his earlier message is 404.
	writer := w.Person("writer", false)
	w.Grant("user", writer, space, "write")
	id := ids.New()
	w.Want(w.Do("writer", http.MethodPost, mw.MessagesPath(main), mw.Body(id, "mine")), http.StatusCreated, "")
	w.SQL(`UPDATE grants SET revoked_at = now(), revoked_by = $2, revoke_reason = 'fixture' WHERE subject_principal_id = $1`, writer.String(), w.OwnerID.String())
	w.SQL(`UPDATE sequence_counters SET value = value + 1 WHERE name = 'permission_version'`)
	w.Want(w.Do("writer", http.MethodPost, mw.MessagesPath(main), mw.Body(id, "mine")), http.StatusNotFound, "not_found")
	// Archived channel.
	w.Want(w.Do("owner", http.MethodPost, "/api/v1/channels/"+ch.String()+"/archive", `{"reason":"done"}`, mw.Key()...), http.StatusOK, "")
	p := w.Want(w.Do("owner", http.MethodPost, mw.MessagesPath(main), mw.Body(ids.New(), "x")), http.StatusForbidden, "channel_read_only")
	if p["reason"] != "channel_archived" {
		t.Errorf("reason %v", p["reason"])
	}
}
