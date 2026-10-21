// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// appendLine calls AppendSystemLine inside a unit of work of the world.
func appendLine(w *mw.World, l messaging.SystemLine) (messaging.Message, error) {
	var m messaging.Message
	err := inUnit(w, func(ctx context.Context) error {
		var err error
		m, err = w.Svc.AppendSystemLine(ctx, l)
		return err
	})
	return m, err
}

// TestAppendSystemLine writes the fail-open line of an agent session (M2-12 design note 12, M1-56
// D6): the next thread_seq, kind system, origin system, the given author, empty content, silent,
// a server client_msg_id, no mentions and no trigger_state; message.created without the
// client_msg_id echo, then MessagePosted with Kind system.
func TestAppendSystemLine(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	lotse := w.Agent("lotse")
	w.Send("owner", main, "hello", http.StatusCreated)
	m, err := appendLine(w, messaging.SystemLine{ThreadID: main, AuthorID: lotse, Code: domain.SystemAgentSessionResumeFailed})
	if err != nil {
		t.Fatal(err)
	}
	if m.ThreadSeq != 2 || m.Kind != "system" || m.Origin != "system" || m.SystemCode != "agent_session.resume_failed" ||
		m.Author.ID != lotse || !m.Silent || m.ContentMD != "" {
		t.Fatalf("AppendSystemLine %+v", m)
	}
	if n := w.Count(`SELECT count(*)::int FROM messages WHERE id = $1 AND kind = 'system' AND origin = 'system'
		AND system_code = 'agent_session.resume_failed' AND author_principal_id = $2 AND content_md = '' AND silent
		AND run_id IS NULL AND thread_seq = 2 AND version = 1 AND trigger_state IS NULL AND client_msg_id IS NOT NULL
		AND length(client_payload_sha256) = 32`, m.ID.String(), lotse.String()); n != 1 {
		t.Fatal("the system line row differs")
	}
	if n := w.Count(`SELECT count(*)::int FROM mentions WHERE message_id = $1`, m.ID.String()); n != 0 {
		t.Fatalf("%d mentions", n)
	}
	evs := w.MessageEvents(main)
	last := evs[len(evs)-1]
	body := last.Payload["message"].(map[string]any)
	if last.ResourceID != m.ID.String() || last.ClientMsgID != "" || body["kind"] != "system" || body["origin"] != "system" ||
		body["system_code"] != "agent_session.resume_failed" || body["silent"] != true || body["content_md"] != "" ||
		body["author"].(map[string]any)["id"] != lotse.String() || body["thread_seq"] != float64(2) {
		t.Fatalf("message.created %+v", last)
	}
	posted := w.Posted()
	ev := posted[len(posted)-1]
	if ev.MessageID != m.ID || ev.Kind != "system" || ev.Origin != "system" || ev.AuthorKind != "agent" || ev.AuthorID != lotse ||
		ev.AuthorDeviceID != nil || !ev.Silent || ev.ThreadSeq != 2 || len(ev.MentionedIDs) != 0 {
		t.Fatalf("MessagePosted %+v", ev)
	}
}

// TestAppendSystemLine_KeepsThreadClosed (R-DM-42): in a closed thread the line takes the next
// number and the thread stays closed; no thread.reopened event and no thread.reopened line.
func TestAppendSystemLine_KeepsThreadClosed(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	forum := w.Channel(w.Space("Forum"), "forum", "bugs")
	tc := w.Want(w.Do("owner", http.MethodPost, mw.ThreadsPath(forum), `{"title":"Export","first_message":`+mw.Body(ids.New(), "first")+`}`),
		http.StatusCreated, "")
	thread := ids.MustParse(tc["thread"].(map[string]any)["id"].(string))
	w.Want(w.Do("owner", http.MethodPost, "/api/v1/threads/"+thread.String()+"/close", "", mw.Key()...), http.StatusOK, "")
	lotse := w.Agent("lotse")
	m, err := appendLine(w, messaging.SystemLine{ThreadID: thread, AuthorID: lotse, Code: domain.SystemAgentSessionResumeFailed})
	if err != nil {
		t.Fatal(err)
	}
	if m.ThreadSeq != 2 {
		t.Fatalf("thread_seq %d", m.ThreadSeq)
	}
	if n := w.Count(`SELECT count(*)::int FROM threads WHERE id = $1 AND state = 'closed'`, thread.String()); n != 1 {
		t.Fatal("the system line reopened the thread")
	}
	if n := len(w.Events("thread.reopened")); n != 0 {
		t.Fatalf("%d thread.reopened", n)
	}
	if n := w.Count(`SELECT count(*)::int FROM messages WHERE thread_id = $1 AND system_code = 'thread.reopened'`, thread.String()); n != 0 {
		t.Fatalf("%d reopen lines", n)
	}
}

// TestAppendSystemLine_InternalErrors: outside a unit of work and with a code that is not a system
// code of 03 the call is internal and writes nothing.
func TestAppendSystemLine_InternalErrors(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	lotse := w.Agent("lotse")
	if messaging.SystemAgentSessionResumeFailed != domain.SystemAgentSessionResumeFailed {
		t.Fatal("the port and the domain name different codes")
	}
	ok := messaging.SystemLine{ThreadID: main, AuthorID: lotse, Code: messaging.SystemAgentSessionResumeFailed}
	if _, err := w.Svc.AppendSystemLine(context.Background(), ok); !internal(err) {
		t.Fatalf("outside a unit of work: %v", err)
	}
	for _, code := range []string{"", "agent_session.unknown", "Agent_session.resume_failed"} {
		if _, err := appendLine(w, messaging.SystemLine{ThreadID: main, AuthorID: lotse, Code: code}); !internal(err) {
			t.Errorf("code %q: %v, want internal", code, err)
		}
	}
	if n := w.Count(`SELECT count(*)::int FROM messages WHERE kind = 'system'`); n != 0 {
		t.Fatalf("%d system lines", n)
	}
}
