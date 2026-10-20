// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// stampOf is the API form of the fake clock's time (06 section 2).
func stampOf(w *mw.World) string { return w.Clock.Now().UTC().Format("2006-01-02T15:04:05.000Z") }

// sameInstant reports whether the REST timestamp v is the time of the fake clock.
func sameInstant(v any, w *mw.World) bool {
	s, _ := v.(string)
	at, err := time.Parse(time.RFC3339Nano, s)
	return err == nil && at.Equal(w.Clock.Now().Truncate(time.Millisecond))
}

// internal reports whether err is the internal error of kernel/errs.
func internal(err error) bool {
	e, ok := errs.As(err)
	return ok && e.Code == errs.CodeInternalError
}

// inUnit runs f in one unit of work of the world and returns its error.
func inUnit(w *mw.World, f func(ctx context.Context) error) error {
	return w.UoW.Do(context.Background(), f)
}

// setTrigger calls SetTriggerState inside a unit of work and fails the test on an error.
func setTrigger(t *testing.T, w *mw.World, id ids.ID, state, reason string) {
	t.Helper()
	if err := inUnit(w, func(ctx context.Context) error {
		return w.Svc.SetTriggerState(ctx, messaging.SetTriggerStateCmd{MessageID: id, State: state, Reason: reason})
	}); err != nil {
		t.Fatalf("SetTriggerState(%s, %q): %v", state, reason, err)
	}
}

// updatedOf returns the message member of the message.updated events of the message id.
func updatedOf(w *mw.World, id ids.ID) []map[string]any {
	var out []map[string]any
	for _, e := range w.Events("message.updated") {
		if e.ResourceID == id.String() {
			out = append(out, e.Payload["message"].(map[string]any))
		}
	}
	return out
}

// seedRun writes a committed queued run of agent in thread when the table runs exists, so the
// deferred foreign key messages.run_id finds it, and returns its id (M2-05 migration 00053).
func seedRun(w *mw.World, thread, agent, by ids.ID) ids.ID {
	id := ids.New()
	if w.Count(`SELECT count(*)::int FROM pg_class WHERE relname = 'runs' AND relkind IN ('r', 'p')`) == 0 {
		return id
	}
	w.SQL(`INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggered_by, billing_mode_expected, invocation_jti)
		VALUES ($1, $2, $3, 'message', $4, 'organization', $5)`, id.String(), thread.String(), agent.String(), by.String(), ids.New().String())
	return id
}

// TestSetTriggerState_RaisesVersionAndAppendsUpdated (09 section 6, brief note 7): each change
// sets the three columns, raises the version by one and appends one message.updated with the
// stored representation; getMessage shows the same fields.
func TestSetTriggerState_RaisesVersionAndAppendsUpdated(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	m := w.Send("owner", main, "hello", http.StatusCreated)
	if _, ok := m["trigger_state"]; ok {
		t.Fatalf("a new message without a run carries trigger_state: %v", m)
	}
	id := ids.MustParse(m["id"].(string))
	setTrigger(t, w, id, "received", "")
	ups := updatedOf(w, id)
	if len(ups) != 1 || ups[0]["trigger_state"] != "received" || ups[0]["trigger_state_at"] != stampOf(w) ||
		ups[0]["version"] != float64(2) || ups[0]["content_md"] != "hello" {
		t.Fatalf("message.updated %v", ups)
	}
	if _, ok := ups[0]["trigger_state_reason"]; ok {
		t.Fatalf("trigger_state_reason without a reason: %v", ups[0])
	}
	setTrigger(t, w, id, "failed", "agent_disabled")
	ups = updatedOf(w, id)
	if len(ups) != 2 || ups[1]["trigger_state"] != "failed" || ups[1]["trigger_state_reason"] != "agent_disabled" || ups[1]["version"] != float64(3) {
		t.Fatalf("message.updated %v", ups)
	}
	got := w.Want(w.Do("owner", http.MethodGet, mw.MessagePath(id), ""), http.StatusOK, "")
	if got["trigger_state"] != "failed" || got["trigger_state_reason"] != "agent_disabled" || !sameInstant(got["trigger_state_at"], w) ||
		got["version"] != float64(3) {
		t.Fatalf("getMessage %v", got)
	}
	if n := w.Count(`SELECT count(*)::int FROM messages WHERE id = $1 AND trigger_state = 'failed' AND trigger_state_reason = 'agent_disabled'
		AND trigger_state_at IS NOT NULL AND version = 3`, id.String()); n != 1 {
		t.Fatal("the row does not carry the trigger state")
	}
}

// TestSetTriggerState_SameStateIsNoop (D-M2-05-17): the stored state and reason again write
// nothing and append no event; another reason is a change.
func TestSetTriggerState_SameStateIsNoop(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	id := ids.MustParse(w.Send("owner", main, "hello", http.StatusCreated)["id"].(string))
	setTrigger(t, w, id, "received", "")
	setTrigger(t, w, id, "received", "")
	if n := len(updatedOf(w, id)); n != 1 {
		t.Fatalf("%d message.updated after the same state twice", n)
	}
	setTrigger(t, w, id, "cancelled", "user_cancelled")
	setTrigger(t, w, id, "cancelled", "user_cancelled")
	setTrigger(t, w, id, "cancelled", "access_revoked")
	ups := updatedOf(w, id)
	if len(ups) != 3 || ups[2]["trigger_state_reason"] != "access_revoked" || ups[2]["version"] != float64(4) {
		t.Fatalf("message.updated %v", ups)
	}
	if n := w.Count(`SELECT version::int FROM messages WHERE id = $1`, id.String()); n != 4 {
		t.Fatalf("version %d", n)
	}
}

// TestSetTriggerState_InternalErrors: outside a unit of work, an unknown message, a message whose
// origin is not human, an unknown state and a reason that does not fit the state are internal
// errors that write nothing.
func TestSetTriggerState_InternalErrors(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, ch, main := w.TextChannel()
	human := ids.MustParse(w.Send("owner", main, "hello", http.StatusCreated)["id"].(string))
	op := w.Agent("lotse")
	w.SetOperator(ch, op)
	agentMsg := ids.MustParse(w.Send("lotse", main, "an agent answer", http.StatusCreated)["id"].(string))
	err := w.Svc.SetTriggerState(context.Background(), messaging.SetTriggerStateCmd{MessageID: human, State: "received"})
	if !internal(err) {
		t.Fatalf("outside a unit of work: %v", err)
	}
	for _, c := range []struct {
		name string
		cmd  messaging.SetTriggerStateCmd
	}{
		{"unknown message", messaging.SetTriggerStateCmd{MessageID: ids.New(), State: "received"}},
		{"origin agent", messaging.SetTriggerStateCmd{MessageID: agentMsg, State: "received"}},
		{"unknown state", messaging.SetTriggerStateCmd{MessageID: human, State: "started"}},
		{"empty state", messaging.SetTriggerStateCmd{MessageID: human}},
		{"failed without reason", messaging.SetTriggerStateCmd{MessageID: human, State: "failed"}},
		{"cancelled without reason", messaging.SetTriggerStateCmd{MessageID: human, State: "cancelled"}},
		{"received with reason", messaging.SetTriggerStateCmd{MessageID: human, State: "received", Reason: "agent_disabled"}},
		{"done with reason", messaging.SetTriggerStateCmd{MessageID: human, State: "done", Reason: "x"}},
	} {
		err := inUnit(w, func(ctx context.Context) error { return w.Svc.SetTriggerState(ctx, c.cmd) })
		if !internal(err) {
			t.Errorf("%s: %v, want internal", c.name, err)
		}
	}
	if n := len(w.Events("message.updated")); n != 0 {
		t.Fatalf("%d message.updated", n)
	}
	if n := w.Count(`SELECT count(*)::int FROM messages WHERE trigger_state IS NOT NULL OR version <> 1`); n != 0 {
		t.Fatalf("%d messages changed", n)
	}
}

// TestSetTriggerState_TombstoneKeepsItsForm: a deleted message gets the trigger state, and its
// message.updated is the tombstone (no content, no mentions) with the trigger fields, which are
// not content (03 messages).
func TestSetTriggerState_TombstoneKeepsItsForm(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	id := ids.MustParse(w.Send("owner", main, "secret @owner", http.StatusCreated)["id"].(string))
	w.Want(w.Do("owner", http.MethodDelete, mw.MessagePath(id), ""), http.StatusNoContent, "")
	setTrigger(t, w, id, "cancelled", "user_cancelled")
	ups := updatedOf(w, id)
	if len(ups) != 1 || ups[0]["content_md"] != "" || ups[0]["deleted_at"] == nil || len(ups[0]["mention_ids"].([]any)) != 0 ||
		ups[0]["trigger_state"] != "cancelled" || ups[0]["version"] != float64(3) {
		t.Fatalf("message.updated %v", ups)
	}
}

// TestCreateRunAnchor writes the live box anchor (09 section 5.4): the next thread_seq, kind run,
// origin agent, the agent as author, empty content, silent, run_id, a server client_msg_id and
// no mentions; message.created without the client_msg_id echo, then MessagePosted with Kind run.
// Outside a unit of work it is an internal error.
func TestCreateRunAnchor(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	lotse := w.Agent("lotse")
	w.Send("owner", main, "hello", http.StatusCreated)
	run := seedRun(w, main, lotse, w.OwnerID)
	var ref messaging.AnchorRef
	if err := inUnit(w, func(ctx context.Context) error {
		var err error
		ref, err = w.Svc.CreateRunAnchor(ctx, messaging.RunAnchorCmd{ThreadID: main, RunID: run, AgentID: lotse})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if ref.ThreadSeq != 2 || ref.MessageID.IsZero() {
		t.Fatalf("AnchorRef %+v", ref)
	}
	if n := w.Count(`SELECT count(*)::int FROM messages WHERE id = $1 AND kind = 'run' AND origin = 'agent' AND author_principal_id = $2
		AND content_md = '' AND silent AND run_id = $3 AND thread_seq = 2 AND version = 1 AND trigger_state IS NULL
		AND client_msg_id IS NOT NULL AND length(client_payload_sha256) = 32`, ref.MessageID.String(), lotse.String(), run.String()); n != 1 {
		t.Fatal("the anchor row differs")
	}
	if n := w.Count(`SELECT count(*)::int FROM mentions WHERE message_id = $1`, ref.MessageID.String()); n != 0 {
		t.Fatalf("%d mentions", n)
	}
	evs := w.MessageEvents(main)
	last := evs[len(evs)-1]
	body := last.Payload["message"].(map[string]any)
	if last.ResourceID != ref.MessageID.String() || last.ClientMsgID != "" || body["kind"] != "run" || body["origin"] != "agent" ||
		body["run_id"] != run.String() || body["silent"] != true || body["content_md"] != "" ||
		body["author"].(map[string]any)["id"] != lotse.String() || body["thread_seq"] != float64(2) {
		t.Fatalf("message.created %+v", last)
	}
	posted := w.Posted()
	ev := posted[len(posted)-1]
	if ev.MessageID != ref.MessageID || ev.Kind != "run" || ev.Origin != "agent" || ev.AuthorKind != "agent" || ev.AuthorID != lotse ||
		ev.AuthorDeviceID != nil || !ev.Silent || ev.ThreadSeq != 2 || len(ev.MentionedIDs) != 0 {
		t.Fatalf("MessagePosted %+v", ev)
	}
	got := w.Want(w.Do("owner", http.MethodGet, mw.MessagePath(ref.MessageID), ""), http.StatusOK, "")
	if got["run_id"] != run.String() || got["kind"] != "run" {
		t.Fatalf("getMessage %v", got)
	}
	// The history carries the same representation: the anchor with run_id, the message of the
	// person with its trigger state.
	setTrigger(t, w, ids.MustParse(evs[0].ResourceID), "received", "")
	items := w.Want(w.Do("owner", http.MethodGet, mw.MessagesPath(main), ""), http.StatusOK, "")["items"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["trigger_state"] != "received" || items[1].(map[string]any)["run_id"] != run.String() {
		t.Fatalf("listThreadMessages %v", items)
	}
	err := func() error {
		_, err := w.Svc.CreateRunAnchor(context.Background(), messaging.RunAnchorCmd{ThreadID: main, RunID: ids.New(), AgentID: lotse})
		return err
	}()
	if !internal(err) {
		t.Fatalf("outside a unit of work: %v", err)
	}
}

// TestCreateMessage_CreatedBeforePosted (ruling D-M2-05-3, 04 section 10): createMessage appends
// message.created of the unit before it publishes MessagePosted, so an anchor a subscriber writes
// into the same thread follows the message in the log, and the reopen line comes first.
func TestCreateMessage_CreatedBeforePosted(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	forum := w.Channel(w.Space("Forum"), "forum", "bugs")
	tc := w.Want(w.Do("owner", http.MethodPost, mw.ThreadsPath(forum), `{"title":"Export","first_message":`+mw.Body(ids.New(), "first")+`}`),
		http.StatusCreated, "")
	thread := ids.MustParse(tc["thread"].(map[string]any)["id"].(string))
	w.Want(w.Do("owner", http.MethodPost, "/api/v1/threads/"+thread.String()+"/close", "", mw.Key()...), http.StatusOK, "")
	lotse := w.Agent("lotse")
	run := seedRun(w, thread, lotse, w.OwnerID)
	w.AfterPosted(func(ctx context.Context, ev messaging.MessagePosted) error {
		if ev.Kind != "text" || ev.Origin != "human" {
			return nil
		}
		_, err := w.Svc.CreateRunAnchor(ctx, messaging.RunAnchorCmd{ThreadID: ev.ThreadID, RunID: run, AgentID: lotse})
		return err
	})
	m := w.Send("owner", thread, "still broken", http.StatusCreated)
	evs := w.MessageEvents(thread)
	var kinds []string
	var seqs []float64
	for _, e := range evs {
		body := e.Payload["message"].(map[string]any)
		kinds = append(kinds, body["kind"].(string))
		seqs = append(seqs, body["thread_seq"].(float64))
	}
	if !slices.Equal(kinds, []string{"text", "system", "text", "run"}) || !slices.Equal(seqs, []float64{1, 2, 3, 4}) {
		t.Fatalf("message.created kinds %v seqs %v", kinds, seqs)
	}
	if evs[2].ResourceID != m["id"] {
		t.Fatalf("the third message.created is %s, want the message %v", evs[2].ResourceID, m["id"])
	}
	posted := w.Posted()
	var order []string
	for _, p := range posted[len(posted)-3:] {
		order = append(order, p.Kind)
	}
	if !slices.Equal(order, []string{"system", "text", "run"}) {
		t.Fatalf("MessagePosted order %v", order)
	}
}

// TestCreateMessage_BodyIsStoredRowAfterUnit (04 section 16.5, brief note 7): a subscriber of
// MessagePosted that sets trigger_state in the same unit is visible in the 201 body (received,
// trigger_state_at, version 2), in the replay and in the first message of createThread.
func TestCreateMessage_BodyIsStoredRowAfterUnit(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	forum := w.Channel(w.Space("Forum"), "forum", "bugs")
	w.AfterPosted(func(ctx context.Context, ev messaging.MessagePosted) error {
		if ev.Kind != "text" || ev.Origin != "human" {
			return nil
		}
		return w.Svc.SetTriggerState(ctx, messaging.SetTriggerStateCmd{MessageID: ev.MessageID, State: "received"})
	})
	cm := ids.New()
	m := w.Want(w.Do("owner", http.MethodPost, mw.MessagesPath(main), mw.Body(cm, "hello")), http.StatusCreated, "")
	if m["trigger_state"] != "received" || !sameInstant(m["trigger_state_at"], w) || m["version"] != float64(2) {
		t.Fatalf("201 body %v", m)
	}
	replay := w.Want(w.Do("owner", http.MethodPost, mw.MessagesPath(main), mw.Body(cm, "hello")), http.StatusOK, "")
	if replay["trigger_state"] != "received" || replay["version"] != float64(2) {
		t.Fatalf("replay body %v", replay)
	}
	types := []string{}
	for _, e := range w.Events("message.created", "message.updated") {
		if e.ResourceID == m["id"] {
			types = append(types, e.Type)
		}
	}
	if !slices.Equal(types, []string{"message.created", "message.updated"}) {
		t.Fatalf("events %v", types)
	}
	tc := w.Want(w.Do("owner", http.MethodPost, mw.ThreadsPath(forum), `{"title":"Idea","first_message":`+mw.Body(ids.New(), "in a thread")+`}`, mw.Key()...),
		http.StatusCreated, "")
	first := tc["first_message"].(map[string]any)
	if first["trigger_state"] != "received" || first["version"] != float64(2) {
		t.Fatalf("first message %v", first)
	}
}
