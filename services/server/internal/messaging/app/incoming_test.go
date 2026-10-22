// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// wantReuse fails unless err is 422 idempotency_key_reuse.
func wantReuse(t *testing.T, err error) {
	t.Helper()
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeIdempotencyKeyReuse {
		t.Fatalf("want idempotency_key_reuse, got %v", err)
	}
}

// TestCreate_SkipMentions (R-EX-09, M2-27): with SkipMentions the handles of existing people and
// agents stay text: no mentions row, empty mention ids in the result and in MessagePosted, the
// content unchanged; the scanner does not run, so more handles than message.max_mentions pass.
func TestCreate_SkipMentions(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	space, ch, main := w.TextChannel()
	sam := w.Person("sam", false)
	w.Grant("user", sam, space, "write")
	served := w.Agent("support-agent")
	w.SetOperator(ch, served)
	content := "@sam @support-agent @owner @a @b @c"
	c, err := w.Svc.Create(t.Context(), w.Actor("owner"), messaging.CreateMessageCmd{
		ThreadID: main, ClientMsgID: ids.New(), ContentMD: &content, SkipMentions: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.Message.ContentMD != content || len(c.Message.MentionIDs) != 0 {
		t.Fatalf("created %q mentions %v", c.Message.ContentMD, c.Message.MentionIDs)
	}
	if n := w.Count(`SELECT count(*)::int FROM mentions WHERE message_id = $1`, c.Message.ID.String()); n != 0 {
		t.Fatalf("%d mentions rows", n)
	}
	posted := w.Posted()
	if last := posted[len(posted)-1]; last.MessageID != c.Message.ID || len(last.MentionedIDs) != 0 {
		t.Fatalf("MessagePosted %+v", last)
	}
	// The same content without SkipMentions resolves the mentions (the zero value keeps M1-56).
	m := w.Send("owner", main, "@sam @support-agent", http.StatusCreated)
	if got := mw.JSONList(m["mention_ids"]); got == "[]" {
		t.Fatalf("mention_ids %s", got)
	}
}

// TestCreate_HashContextOverride (M2-27 design note 6c): HashContext replaces the context part of
// the request hash. The same request with another context gives 422 idempotency_key_reuse, the
// same context a replay; the zero value keeps the M1-56 D3 hash "thread:<id>".
func TestCreate_HashContextOverride(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	owner := w.Actor("owner")
	content := "deploy finished"
	cmd := messaging.CreateMessageCmd{ThreadID: main, ClientMsgID: ids.New(), ContentMD: &content, HashContext: "webhook:one"}
	first, err := w.Svc.Create(t.Context(), owner, cmd)
	if err != nil || first.Replay {
		t.Fatalf("first %+v %v", first, err)
	}
	want := domain.RequestHash(domain.HashInput{Context: "webhook:one", Kind: string(domain.KindText), Content: content})
	if n := w.Count(`SELECT count(*)::int FROM messages WHERE id = $1 AND client_payload_sha256 = $2`, first.Message.ID.String(), want); n != 1 {
		t.Fatal("the stored hash is not over HashContext")
	}
	again, err := w.Svc.Create(t.Context(), owner, cmd)
	if err != nil || !again.Replay || again.Message.ID != first.Message.ID {
		t.Fatalf("replay %+v %v", again, err)
	}
	other := cmd
	other.HashContext = "webhook:two"
	_, err = w.Svc.Create(t.Context(), owner, other)
	wantReuse(t, err)
	plain := cmd
	plain.HashContext = ""
	_, err = w.Svc.Create(t.Context(), owner, plain)
	wantReuse(t, err)
	// The zero value hashes over "thread:<id>" as before.
	zero := messaging.CreateMessageCmd{ThreadID: main, ClientMsgID: ids.New(), ContentMD: &content}
	z, err := w.Svc.Create(t.Context(), owner, zero)
	if err != nil {
		t.Fatal(err)
	}
	m1 := domain.RequestHash(domain.HashInput{Context: "thread:" + main.String(), Kind: string(domain.KindText), Content: content})
	if n := w.Count(`SELECT count(*)::int FROM messages WHERE id = $1 AND client_payload_sha256 = $2`, z.Message.ID.String(), m1); n != 1 {
		t.Fatal("the zero value changed the M1-56 D3 hash")
	}
}

// TestCreateThread_HashContextOverride: the first message of createThread carries HashContext in
// place of the new-thread context; the same context is a replay of the thread, another one 422.
func TestCreateThread_HashContextOverride(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	forum := w.Channel(w.Space("Forum"), "forum", "alerts")
	owner := w.Actor("owner")
	title, content := "Disk full", "@owner the disk is full"
	first := &messaging.CreateMessageCmd{ClientMsgID: ids.New(), ContentMD: &content, HashContext: "webhook:one", SkipMentions: true}
	cmd := messaging.CreateThreadCmd{ChannelID: forum, Title: &title, FirstMessage: first}
	tc, err := w.Svc.CreateThread(t.Context(), owner, cmd)
	if err != nil || tc.Replay || tc.FirstMessage == nil {
		t.Fatalf("created %+v %v", tc, err)
	}
	if len(tc.FirstMessage.MentionIDs) != 0 || w.Count(`SELECT count(*)::int FROM mentions WHERE message_id = $1`, tc.FirstMessage.ID.String()) != 0 {
		t.Fatal("SkipMentions on the first message")
	}
	want := domain.RequestHash(domain.HashInput{Context: "webhook:one", Kind: string(domain.KindText), Content: content})
	if n := w.Count(`SELECT count(*)::int FROM messages WHERE id = $1 AND client_payload_sha256 = $2`, tc.FirstMessage.ID.String(), want); n != 1 {
		t.Fatal("the stored hash is not over HashContext")
	}
	// Another title with the same HashContext is the same request for the caller.
	retitled := "Disk still full"
	again := cmd
	again.Title = &retitled
	r, err := w.Svc.CreateThread(t.Context(), owner, again)
	if err != nil || !r.Replay || r.ThreadID != tc.ThreadID {
		t.Fatalf("replay %+v %v", r, err)
	}
	changed := *first
	changed.HashContext = "webhook:two"
	other := cmd
	other.FirstMessage = &changed
	_, err = w.Svc.CreateThread(t.Context(), owner, other)
	wantReuse(t, err)
	if n := w.Count(`SELECT count(*)::int FROM threads WHERE channel_id = $1`, forum.String()); n != 1 {
		t.Fatalf("%d threads", n)
	}
}
