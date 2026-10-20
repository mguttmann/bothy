// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	syncmod "github.com/OWNER/bothy/services/server/internal/sync"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// TestCreateMessage_DifferentBody422: the same client_msg_id with another content, another reply
// target or into another thread is 422 idempotency_key_reuse; an absent kind equals text.
func TestCreateMessage_DifferentBody422(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, ch, main := w.TextChannel()
	other := w.MainThread(w.Channel(w.Space("Other"), "text", "general"))
	_ = ch
	id := ids.New()
	w.Want(w.Do("owner", http.MethodPost, mw.MessagesPath(main), mw.Body(id, "hello")), http.StatusCreated, "")
	w.Want(w.Do("owner", http.MethodPost, mw.MessagesPath(main), mw.Body(id, "hello", `"kind":"text"`)), http.StatusOK, "")
	w.Want(w.Do("owner", http.MethodPost, mw.MessagesPath(main), mw.Body(id, "hello!")), http.StatusUnprocessableEntity, "idempotency_key_reuse")
	w.Want(w.Do("owner", http.MethodPost, mw.MessagesPath(other), mw.Body(id, "hello")), http.StatusUnprocessableEntity, "idempotency_key_reuse")
	if n := w.Count(`SELECT count(*)::int FROM messages`); n != 1 {
		t.Fatalf("%d messages", n)
	}
}

// TestCreateMessage_ConcurrentSameClientMsgID (04 section 5.1 point 4): two identical requests
// behind a barrier create one row; one answers 201, the other the 200 replay, and the thread
// sequence has no gap.
func TestCreateMessage_ConcurrentSameClientMsgID(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	for round := range 5 {
		b := mw.Body(ids.New(), "race "+strconv.Itoa(round))
		var wg sync.WaitGroup
		gate := make(chan struct{})
		codes := make([]int, 2)
		bodies := make([]string, 2)
		for i := range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-gate
				r := w.H.Do(w.H.NewRequest(http.MethodPost, mw.MessagesPath(main), b, "Authorization", "Bearer owner"))
				codes[i], bodies[i] = r.StatusCode, string(r.Body)
			}()
		}
		close(gate)
		wg.Wait()
		slices.Sort(codes)
		if codes[0] != http.StatusOK || codes[1] != http.StatusCreated {
			t.Fatalf("round %d: statuses %v: %v", round, codes, bodies)
		}
		var a, c map[string]any
		_ = json.Unmarshal([]byte(bodies[0]), &a)
		_ = json.Unmarshal([]byte(bodies[1]), &c)
		if a["id"] != c["id"] {
			t.Fatalf("round %d: two messages %v %v", round, a["id"], c["id"])
		}
	}
	got := w.Seqs(main)
	for i, s := range got {
		if s != int64(i+1) {
			t.Fatalf("thread_seq %v has a gap", got)
		}
	}
	if w.Count(`SELECT last_message_seq::int FROM threads WHERE id = $1`, main.String()) != len(got) {
		t.Fatal("last_message_seq differs from the messages")
	}
}

// TestCreateMessage_ThreadSeqGaplessUnderConcurrency (04 section 4): many authors in one thread;
// the committed sequences are exactly 1..n.
func TestCreateMessage_ThreadSeqGaplessUnderConcurrency(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	space, _, main := w.TextChannel()
	const authors, each = 6, 4
	names := make([]string, authors)
	for i := range names {
		names[i] = "writer" + strconv.Itoa(i)
		w.Grant("user", w.Person(names[i], false), space, "write")
	}
	var wg sync.WaitGroup
	errs := make(chan string, authors*each)
	for _, n := range names {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range each {
				r := w.H.Do(w.H.NewRequest(http.MethodPost, mw.MessagesPath(main), mw.Body(ids.New(), n+" "+strconv.Itoa(j)), "Authorization", "Bearer "+n))
				if r.StatusCode != http.StatusCreated {
					errs <- string(r.Body)
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	got := w.Seqs(main)
	if len(got) != authors*each {
		t.Fatalf("%d messages", len(got))
	}
	for i, s := range got {
		if s != int64(i+1) {
			t.Fatalf("thread_seq %v is not 1..n", got)
		}
	}
}

// TestCreateMessage_LargePayloadTruncatedInEvent (04 payload rule 2): the stored payload is below
// sync.event_payload_max_bytes with truncated and content_truncated, the content cut at a
// character boundary; the REST answer has the full content.
func TestCreateMessage_LargePayloadTruncatedInEvent(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	content := strings.Repeat("é€\"<", 300)
	m := w.Send("owner", main, content, http.StatusCreated)
	if m["content_md"] != content {
		t.Fatal("the REST answer is cut")
	}
	if _, ok := m["content_truncated"]; ok {
		t.Error("content_truncated in the REST answer")
	}
	ev := w.MessageEvents(main)
	if len(ev) != 1 || !ev[0].Truncated {
		t.Fatalf("events %+v", ev)
	}
	// jsonb prints white space after separators; without it the payload is what was appended.
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(ev[0].Raw)); err != nil || compact.Len() >= mw.PayloadMax {
		t.Errorf("payload %d bytes, %v", compact.Len(), err)
	}
	msg := ev[0].Payload["message"].(map[string]any)
	cut := msg["content_md"].(string)
	if msg["content_truncated"] != true || !utf8.ValidString(cut) || !strings.HasPrefix(content, cut) || len(cut) == 0 || len(cut) >= len(content) {
		t.Errorf("cut %d of %d bytes, flag %v", len(cut), len(content), msg["content_truncated"])
	}
	small := w.Send("owner", main, "short", http.StatusCreated)
	for _, e := range w.MessageEvents(main) {
		if e.ResourceID == small["id"] && e.Truncated {
			t.Error("a small message is truncated")
		}
	}
}

// TestCreateMessage_Mentions (03 messages "Inhaltsformat", 07 section 4.6, brief D5).
func TestCreateMessage_Mentions(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	space, ch, main := w.TextChannel()
	sam := w.Person("sam", false)
	w.Grant("user", sam, space, "write")
	stranger := w.Person("stranger", false)
	served := w.Agent("support-agent")
	w.SetOperator(ch, served)
	hidden := w.Agent("hidden-agent")
	// sam reads the channel the support agent serves, but no channel of the hidden agent.
	m := w.Send("sam", main, "@stranger @support-agent @hidden-agent @nobody `@owner` sam@example.com", http.StatusCreated)
	want := []string{stranger.String(), served.String()}
	slices.Sort(want)
	if got := mw.JSONList(m["mention_ids"]); got != "["+strings.Join(want, ",")+"]" {
		t.Fatalf("mention_ids %v, want %v", got, want)
	}
	if w.Count(`SELECT count(*)::int FROM mentions WHERE message_id = $1`, m["id"]) != 2 ||
		w.Count(`SELECT count(*)::int FROM mentions WHERE principal_id = $1`, stranger.String()) != 1 ||
		w.Count(`SELECT count(*)::int FROM mentions WHERE principal_id = $1`, hidden.String()) != 0 {
		t.Fatal("mentions rows")
	}
	posted := w.Posted()
	last := posted[len(posted)-1]
	if last.AuthorDeviceID == nil || last.AuthorKind != "user" || len(last.MentionedIDs) != 2 || last.Origin != "human" {
		t.Fatalf("MessagePosted %+v", last)
	}
	// mention_ids must equal the resolved set.
	p := w.Want(w.Do("sam", http.MethodPost, mw.MessagesPath(main), mw.Body(ids.New(), "@stranger", `"mention_ids":["`+served.String()+`"]`)),
		http.StatusBadRequest, "validation_failed")
	if ptr, code := mw.Field(p); ptr != "/mention_ids" || code != "mention_mismatch" {
		t.Fatalf("mismatch: %s %s", ptr, code)
	}
	w.Send("sam", main, "@stranger", http.StatusCreated, `"mention_ids":["`+stranger.String()+`"]`)
	// An agent author mentions a person who reads the thread and one who does not.
	// The owner reads every thread through the owner role, without a grant (07 section 2.1).
	am := w.Send("support-agent", main, "@sam @stranger @owner", http.StatusCreated)
	wantAgent := []string{sam.String(), w.OwnerID.String()}
	slices.Sort(wantAgent)
	if got := mw.JSONList(am["mention_ids"]); got != "["+strings.Join(wantAgent, ",")+"]" {
		t.Fatalf("agent author mentions %s", got)
	}
	if w.Count(`SELECT count(*)::int FROM mentions WHERE message_id = $1`, am["id"]) != 2 {
		t.Fatal("a row for the person without access")
	}
	if posted := w.Posted(); posted[len(posted)-1].AuthorDeviceID != nil || posted[len(posted)-1].Origin != "agent" {
		t.Fatalf("agent MessagePosted %+v", posted[len(posted)-1])
	}
	// More than message.max_mentions distinct handles.
	p = w.Want(w.Do("sam", http.MethodPost, mw.MessagesPath(main), mw.Body(ids.New(), "@a @b @c @d @e @a")), http.StatusBadRequest, "validation_failed")
	if ptr, code := mw.Field(p); ptr != "/content_md" || code != "too_many_mentions" {
		t.Fatalf("too many: %s %s", ptr, code)
	}
	w.Send("sam", main, "@a @b @c @d @a @A", http.StatusCreated)
}

// TestCreateMessage_ReopensClosedThread (01 section 6.1, brief D6, D7): a message into a closed
// thread reopens it; the events are thread.reopened, the system message.created and the user
// message.created; the sequences are n and n+1.
func TestCreateMessage_ReopensClosedThread(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	forum := w.Channel(w.Space("Forum"), "forum", "bugs")
	tc := w.Want(w.Do("owner", http.MethodPost, mw.ThreadsPath(forum), `{"title":"Export","first_message":`+mw.Body(ids.New(), "first")+`}`),
		http.StatusCreated, "")
	thread := ids.MustParse(tc["thread"].(map[string]any)["id"].(string))
	w.Want(w.Do("owner", http.MethodPost, "/api/v1/threads/"+thread.String()+"/close", "", mw.Key()...), http.StatusOK, "")
	before := len(w.Events())
	m := w.Send("owner", thread, "still broken", http.StatusCreated)
	all := w.Events()[before:]
	var types []string
	for _, e := range all {
		types = append(types, e.Type)
	}
	if !slices.Equal(types, []string{"thread.reopened", "message.created", "message.created"}) {
		t.Fatalf("events %v", types)
	}
	line := all[1].Payload["message"].(map[string]any)
	if line["kind"] != "system" || line["origin"] != "system" || line["system_code"] != "thread.reopened" || line["content_md"] != "" ||
		line["silent"] != true || line["author"].(map[string]any)["id"] != w.OwnerID.String() || line["thread_seq"] != float64(2) {
		t.Fatalf("system line %v", line)
	}
	if all[1].ClientMsgID != "" || all[2].ClientMsgID != m["client_msg_id"] {
		t.Errorf("client_msg_id echo %q %q", all[1].ClientMsgID, all[2].ClientMsgID)
	}
	if m["thread_seq"] != float64(3) {
		t.Fatalf("message seq %v", m["thread_seq"])
	}
	if w.Count(`SELECT count(*)::int FROM threads WHERE id = $1 AND state = 'open'`, thread.String()) != 1 {
		t.Fatal("thread not open")
	}
	posted := w.Posted()
	sys, user := posted[len(posted)-2], posted[len(posted)-1]
	if sys.Kind != "system" || sys.Origin != "system" || sys.ReopenedThread || sys.AuthorDeviceID != nil ||
		user.Kind != "text" || !user.ReopenedThread || user.ThreadSeq != 3 || sys.ThreadSeq != 2 {
		t.Fatalf("MessagePosted %+v %+v", sys, user)
	}
}

// TestCreateMessage_ReplySameThreadOnly: a reply into the thread is accepted; an id of another
// thread and an unknown id give the same 400.
func TestCreateMessage_ReplySameThreadOnly(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	other := w.MainThread(w.Channel(w.Space("Other"), "text", "general"))
	target := w.Send("owner", main, "question", http.StatusCreated)
	foreign := w.Send("owner", other, "elsewhere", http.StatusCreated)
	r := w.Send("owner", main, "answer", http.StatusCreated, `"reply_to_message_id":"`+target["id"].(string)+`"`)
	if r["reply_to_message_id"] != target["id"] {
		t.Fatalf("reply %v", r["reply_to_message_id"])
	}
	var details []string
	for _, id := range []string{foreign["id"].(string), ids.New().String()} {
		p := w.Want(w.Do("owner", http.MethodPost, mw.MessagesPath(main), mw.Body(ids.New(), "x", `"reply_to_message_id":"`+id+`"`)),
			http.StatusBadRequest, "validation_failed")
		ptr, code := mw.Field(p)
		if ptr != "/reply_to_message_id" || code != "not_in_thread" {
			t.Fatalf("%s %s", ptr, code)
		}
		details = append(details, p["detail"].(string)+"|"+mw.JSONErrors(p))
	}
	if details[0] != details[1] {
		t.Fatalf("answers differ: %v", details)
	}
}

// TestCreateMessage_MessagePostedVetoRollsBack (02 section 8.3): the subscriber sees all fields;
// when it vetoes, no message, mention, thread sequence or Event remains.
func TestCreateMessage_MessagePostedVetoRollsBack(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, ch, main := w.TextChannel()
	reply := w.Send("owner", main, "first", http.StatusCreated)
	m := w.Send("owner", main, "hello @owner", http.StatusCreated, `"reply_to_message_id":"`+reply["id"].(string)+`"`, `"silent":true`, `"clear_draft":true`)
	ev := w.Posted()[1]
	if ev.MessageID.String() != m["id"] || ev.ThreadID != main || ev.ChannelID != ch || ev.SpaceID.IsZero() || ev.ThreadKind != "main" ||
		ev.ThreadSeq != 2 || ev.AuthorID != w.OwnerID || ev.AuthorKind != "user" || ev.Origin != "human" || ev.Kind != "text" ||
		ev.ClientMsgID.String() != m["client_msg_id"] || ev.AuthorDeviceID == nil || ev.ReplyToID == nil || ev.ReplyToID.String() != reply["id"] ||
		len(ev.MentionedIDs) != 1 || ev.MentionedIDs[0] != w.OwnerID || !ev.Silent || !ev.ClearDraft || ev.CreatedThread ||
		ev.ReopenedThread || ev.CreatedAt.IsZero() {
		t.Fatalf("MessagePosted %+v", ev)
	}
	messages, mentions, events := w.Count(`SELECT count(*)::int FROM messages`), w.Count(`SELECT count(*)::int FROM mentions`), len(w.Events())
	seq := w.Count(`SELECT last_message_seq::int FROM threads WHERE id = $1`, main.String())
	w.SetVeto(true)
	r := w.Do("owner", http.MethodPost, mw.MessagesPath(main), mw.Body(ids.New(), "vetoed @owner"))
	if r.StatusCode < 500 {
		t.Fatalf("veto answered %d", r.StatusCode)
	}
	if w.Count(`SELECT count(*)::int FROM messages`) != messages || w.Count(`SELECT count(*)::int FROM mentions`) != mentions ||
		len(w.Events()) != events || w.Count(`SELECT last_message_seq::int FROM threads WHERE id = $1`, main.String()) != seq {
		t.Fatal("the veto left rows behind")
	}
}

// TestSeq_04_16_1_SendOnline (04 section 16.1 up to the log tail): with LISTEN event_log open,
// createMessage gives 201, the notification carries the new seq, and the sync tail reads
// message.created with the echoed client_msg_id.
func TestSeq_04_16_1_SendOnline(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	ctx := testkit.Context(t)
	conn, err := w.Owner.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `LISTEN event_log`); err != nil {
		t.Fatal(err)
	}
	got := make(chan syncmod.Event, 16)
	tail := syncpg.NewTail(w.Pool, syncpg.NewRepository(w.Pool, nil), syncpg.TailConfig{PollInterval: 50 * time.Millisecond, BatchSize: 100},
		[]syncmod.Sink{func(_ context.Context, batch []syncmod.Event) error {
			for _, e := range batch {
				got <- e
			}
			return nil
		}}, nil)
	tctx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- tail.Run(tctx) }()
	defer func() {
		stop()
		<-done
	}()
	<-tail.Started()
	clientID := ids.New()
	m := w.Want(w.Do("owner", http.MethodPost, mw.MessagesPath(main), mw.Body(clientID, "online")), http.StatusCreated, "")
	n, err := conn.Conn().WaitForNotification(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seq, err := strconv.ParseInt(n.Payload, 10, 64)
	if err != nil || seq != w.MessageEvents(main)[0].Seq {
		t.Fatalf("notification %q, want the seq of message.created", n.Payload)
	}
	for {
		select {
		case e := <-got:
			if e.Type == "message.created" && e.ResourceID.String() == m["id"] {
				if e.ClientMsgID != clientID || e.Seq != seq {
					t.Fatalf("tail Event %+v", e)
				}
				return
			}
		case <-ctx.Done():
			t.Fatal("the tail did not deliver message.created")
		}
	}
}
