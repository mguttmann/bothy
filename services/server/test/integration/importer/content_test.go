// SPDX-License-Identifier: Apache-2.0

package importer_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strconv"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// Times of the source (22 6.2: created_at and edited_at come from the data).
const (
	tsStart  = "2025-03-01T08:00:00.000Z"
	tsReply  = "2025-03-01T08:05:00.000Z"
	tsEdit   = "2025-03-01T08:07:00.000Z"
	tsLater  = "2025-03-01T09:00:00.000Z"
	tsClosed = "2025-03-02T10:00:00.000Z"
	tsNext   = "2025-03-03T10:00:00.000Z"
)

// batchResult is the answer of applyImportBatch.
type batchResult struct {
	Items          []item `json:"items"`
	UnreadFloorSeq *int64 `json:"unread_floor_seq"`
}

// threadBatch posts a thread batch as Robin.
func (w *iw) threadBatch(id, key string, items ...map[string]any) harness.Response {
	w.t.Helper()
	return w.do(w.robin, http.MethodPost, "/imports/"+id+"/batches", map[string]any{"thread_origin_key": key, "items": items}, nil)
}

// mustThreadBatch posts a thread batch and returns a 200 answer.
func (w *iw) mustThreadBatch(id, key string, items ...map[string]any) batchResult {
	w.t.Helper()
	r := w.threadBatch(id, key, items...)
	if r.Status != http.StatusOK {
		w.t.Fatalf("thread batch %s = %d %s", key, r.Status, r.Body)
	}
	var b batchResult
	r.JSON(w.t, &b)
	return b
}

// threadObj is a thread item of the channel with the Discord ID channel.
func threadObj(thread, channel, name string, extra map[string]any) map[string]any {
	data := map[string]any{"channel_origin_key": "channel:" + channel, "name": name, "created_at": tsStart, "last_activity_at": tsReply,
		"closed": false}
	for k, v := range extra {
		data[k] = v
	}
	return obj("thread:"+thread, "thread", data)
}

// msgObj is a message item by the Discord user author.
func msgObj(message, author, content, created string, extra map[string]any) map[string]any {
	data := map[string]any{"author_origin_key": "user:" + author, "content_md": content, "created_at": created}
	for k, v := range extra {
		data[k] = v
	}
	return obj("message:"+message, "message", data)
}

// pinObj is a pin item of the message with the Discord ID message.
func pinObj(message string) map[string]any {
	return obj("pin:"+message, "pin", map[string]any{"message_origin_key": "message:" + message})
}

// row reads one text column of a query on the admin pool.
func (w *iw) text(query string, args ...any) string {
	w.t.Helper()
	var s string
	if err := w.s.AdminDB().QueryRow(w.ctx, query, args...).Scan(&s); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return s
}

// eventTypes counts the events after seq by type.
func (w *iw) eventTypes(after int64) map[string]int {
	w.t.Helper()
	out := map[string]int{}
	for _, e := range w.events(after) {
		out[e.Type]++
	}
	return out
}

// TestImportNeverTriggers (plan acceptance, 22 16 "Import never triggers", 05 5 rule 1): imported
// messages in a channel with the operator Lotse, which is not read-only, create no run, no
// agent.invocation, no follow-up and no trigger_state, also with a mention of the agent.
func TestImportNeverTriggers(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, items := w.imported()
	here := items["channel:"+chHere].TargetID
	_, _, etag := w.channel(w.robin, here)
	w.must(w.robin, http.MethodPut, "/channels/"+here+"/operator", map[string]any{"mode": "agent", "agent_id": w.lotse.PrincipalID,
		"reason": "switch-over"}, ifMatch(etag), http.StatusOK)
	seq := w.maxSeq()
	w.mustThreadBatch(id, "channel:"+chHere, msgObj("7001", userAlex, "@lotse please look", tsStart, nil),
		msgObj("7002", userSam, "@lotse and again", tsReply, nil))
	w.mustThreadBatch(id, "thread:6001", threadObj("6001", chHere, "Login fails", map[string]any{"starter_message_origin_key": "message:7001"}),
		msgObj("7003", userAlex, "@lotse the thread", tsReply, nil))
	if n := w.count(`SELECT count(*) FROM runs`); n != 0 {
		t.Errorf("%d runs", n)
	}
	if n := w.count(`SELECT count(*) FROM run_followups`); n != 0 {
		t.Errorf("%d follow-ups", n)
	}
	if n := w.count(`SELECT count(*) FROM messages WHERE origin = 'import' AND (trigger_state IS NOT NULL OR NOT silent)`); n != 0 {
		t.Errorf("%d imported messages with a trigger state or not silent", n)
	}
	if n := w.count(`SELECT count(*) FROM messages WHERE origin = 'import'`); n != 3 {
		t.Errorf("%d imported messages, want 3", n)
	}
	types := w.eventTypes(seq)
	for _, typ := range []string{"agent.invocation", "agent.followup", "run.state_changed", "message.created"} {
		if types[typ] != 0 {
			t.Errorf("%d %s events", types[typ], typ)
		}
	}
}

// TestImportNoNotification (22 16 "No notification", 04 15.8): no mentions row, no notify job, no
// message.created; exactly one thread.updated per batch; unread_floor_seq equals last_message_seq,
// so the thread is unread for nobody.
func TestImportNoNotification(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, items := w.imported()
	main := w.mainThread(items["channel:"+chText].TargetID)
	jobs := w.count(`SELECT count(*) FROM jobs.river_job WHERE kind LIKE 'notify.%'`)
	seq := w.maxSeq()
	res := w.mustThreadBatch(id, "channel:"+chText, msgObj("7101", userAlex, "@robin @alex hello", tsStart, nil),
		msgObj("7102", userSam, "second", tsReply, nil))
	types := w.eventTypes(seq)
	if types["thread.updated"] != 1 || types["import.progress"] != 1 || types["message.created"] != 0 || len(types) != 2 {
		t.Errorf("events %v", types)
	}
	if n := w.count(`SELECT count(*) FROM mentions`); n != 0 {
		t.Errorf("%d mentions rows", n)
	}
	if n := w.count(`SELECT count(*) FROM jobs.river_job WHERE kind LIKE 'notify.%'`); n != jobs {
		t.Errorf("notify jobs %d, before %d", n, jobs)
	}
	var floor, last int64
	if err := w.s.AdminDB().QueryRow(w.ctx, `SELECT unread_floor_seq, last_message_seq FROM threads WHERE id = $1`, main).Scan(&floor, &last); err != nil {
		t.Fatal(err)
	}
	if floor != 2 || last != 2 || res.UnreadFloorSeq == nil || *res.UnreadFloorSeq != 2 {
		t.Errorf("floor %d last %d answer %v", floor, last, res.UnreadFloorSeq)
	}
	if n := w.count(`SELECT count(*) FROM read_states WHERE thread_id = $1`, main); n != 0 {
		t.Errorf("%d read states written", n)
	}
	seq = w.maxSeq()
	w.mustThreadBatch(id, "channel:"+chText, msgObj("7103", userAlex, "third", tsLater, nil))
	if types := w.eventTypes(seq); types["thread.updated"] != 1 || types["import.progress"] != 1 || len(types) != 2 {
		t.Errorf("events of the second batch %v", types)
	}
}

// TestThreadItemFields (22 5.1.1, R-IM-08, R-IM-11): a placeholder name gives auto, a real name
// manual; the operator comes from planned_operator while the channel has none; times and closed_at
// come from the data; the starter message link only when it is posted; forum tags from posted tag
// items.
func TestThreadItemFields(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, items := w.imported()
	text := items["channel:"+chText].TargetID
	w.mustThreadBatch(id, "channel:"+chText, msgObj("7201", userAlex, "starter", tsStart, nil))
	starter := w.text(`SELECT target_id::text FROM import_items WHERE source_id = $1 AND origin_key = 'message:7201'`, id)
	a := w.mustThreadBatch(id, "thread:6201", threadObj("6201", chText, "Login fails on mobile", map[string]any{
		"starter_message_origin_key": "message:7201", "closed": true, "closed_at": tsClosed, "last_activity_at": tsLater}))
	b := w.mustThreadBatch(id, "thread:6202", threadObj("6202", chText, "new thread", map[string]any{"starter_message_origin_key": "message:7299"}))
	c := w.mustThreadBatch(id, "thread:6203", threadObj("6203", chForum, "Feature", map[string]any{"tag_origin_keys": []string{"forum_tag:" + chForum + ":9"}}))
	type thread struct {
		source, state, channel, operator, parent, created, activity, closed string
	}
	read := func(target string) thread {
		var th thread
		if err := w.s.AdminDB().QueryRow(w.ctx, `SELECT title_source, state, channel_id::text, coalesce(operator_agent_id::text, ''),
			coalesce(parent_message_id::text, ''), to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS'),
			to_char(last_activity_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS'),
			coalesce(to_char(closed_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS'), '') FROM threads WHERE id = $1`, target).
			Scan(&th.source, &th.state, &th.channel, &th.operator, &th.parent, &th.created, &th.activity, &th.closed); err != nil {
			t.Fatal(err)
		}
		return th
	}
	ta := read(a.Items[0].TargetID)
	if ta.source != "manual" || ta.state != "closed" || ta.channel != text || ta.operator != w.lotse.PrincipalID || ta.parent != starter ||
		ta.created != "2025-03-01T08:00:00" || ta.activity != "2025-03-01T09:00:00" || ta.closed != "2025-03-02T10:00:00" {
		t.Errorf("thread a %+v", ta)
	}
	tb := read(b.Items[0].TargetID)
	if tb.source != "auto" || tb.state != "open" || tb.parent != "" || tb.operator != w.lotse.PrincipalID {
		t.Errorf("thread b %+v", tb)
	}
	tc := read(c.Items[0].TargetID)
	if tc.operator != "" || tc.channel != items["channel:"+chForum].TargetID {
		t.Errorf("thread c %+v", tc)
	}
	if n := w.count(`SELECT count(*) FROM thread_tags WHERE thread_id = $1 AND tag_id = $2`, c.Items[0].TargetID,
		items["forum_tag:"+chForum+":9"].TargetID); n != 1 {
		t.Errorf("forum tags %d", n)
	}
	if n := w.count(`SELECT count(*) FROM threads WHERE channel_id = $1 AND kind = 'thread' AND operator_agent_id IS NULL`, text); n != 0 {
		t.Errorf("%d threads without the planned operator", n)
	}
}

// TestMirrorChangesOfPostedThread (plan acceptance, 22 12.2 step 3): a rename changes an auto title
// and leaves a manual one; archive and reopen follow the source; thread.updated only when changed.
func TestMirrorChangesOfPostedThread(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, _ := w.imported()
	auto := w.mustThreadBatch(id, "thread:6301", threadObj("6301", chForum, "new thread", nil)).Items[0].TargetID
	manual := w.mustThreadBatch(id, "thread:6302", threadObj("6302", chForum, "Billing", nil)).Items[0].TargetID
	state := func(thread string) (title, source, st string) {
		if err := w.s.AdminDB().QueryRow(w.ctx, `SELECT title, title_source, state FROM threads WHERE id = $1`, thread).Scan(&title, &source, &st); err != nil {
			t.Fatal(err)
		}
		return
	}
	seq := w.maxSeq()
	w.mustThreadBatch(id, "thread:6301", threadObj("6301", chForum, "Login fails", nil))
	w.mustThreadBatch(id, "thread:6302", threadObj("6302", chForum, "Invoices", nil))
	if title, source, _ := state(auto); title != "Login fails" || source != "auto" {
		t.Errorf("auto thread %q %q", title, source)
	}
	if title, source, _ := state(manual); title != "Billing" || source != "manual" {
		t.Errorf("manual thread %q %q", title, source)
	}
	if types := w.eventTypes(seq); types["thread.updated"] != 1 {
		t.Errorf("renames appended %v", types)
	}
	seq = w.maxSeq()
	w.mustThreadBatch(id, "thread:6302", threadObj("6302", chForum, "Invoices", map[string]any{"closed": true, "closed_at": tsClosed}))
	if _, _, st := state(manual); st != "closed" {
		t.Errorf("archive: %s", st)
	}
	w.mustThreadBatch(id, "thread:6302", threadObj("6302", chForum, "Invoices", map[string]any{"last_activity_at": tsNext}))
	if _, _, st := state(manual); st != "open" {
		t.Errorf("reopen: %s", st)
	}
	if n := w.count(`SELECT count(*) FROM threads WHERE id = $1 AND closed_at IS NULL AND closed_by IS NULL
		AND last_activity_at = '2025-03-03T10:00:00Z'`, manual); n != 1 {
		t.Error("reopen kept closed_at or missed last_activity_at")
	}
	if types := w.eventTypes(seq); types["thread.updated"] != 2 || types["thread.reopened"] != 0 {
		t.Errorf("archive and reopen appended %v", types)
	}
}

// TestRepeatBatchWritesNothing (plan acceptance, 22 7.2 item 1): the same thread batch twice gives
// the same targets and no event the second time.
func TestRepeatBatchWritesNothing(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, _ := w.imported()
	batch := []map[string]any{threadObj("6401", chForum, "Question", nil), msgObj("7401", userAlex, "first", tsStart, nil),
		msgObj("7402", userSam, "second", tsReply, map[string]any{"reply_to_origin_key": "message:7401"}), pinObj("7401")}
	first := w.mustThreadBatch(id, "thread:6401", batch...)
	seq := w.maxSeq()
	messages := w.count(`SELECT count(*) FROM messages`)
	second := w.mustThreadBatch(id, "thread:6401", batch...)
	for i := range first.Items {
		if first.Items[i].TargetID != second.Items[i].TargetID || first.Items[i].ID != second.Items[i].ID || second.Items[i].State != "posted" {
			t.Errorf("item %d: %+v then %+v", i, first.Items[i], second.Items[i])
		}
	}
	if ev := w.events(seq); len(ev) != 0 {
		t.Errorf("a repeated batch appended %+v", ev)
	}
	if n := w.count(`SELECT count(*) FROM messages`); n != messages {
		t.Errorf("%d messages, before %d", n, messages)
	}
	if *first.UnreadFloorSeq != 2 || *second.UnreadFloorSeq != 2 {
		t.Errorf("floors %d %d", *first.UnreadFloorSeq, *second.UnreadFloorSeq)
	}
}

// TestStoredThen502ThreadBatch (22 16 "Stored, then error"): the answer of a committed batch is
// lost; the retry gives the same message ids and the same thread_seq values.
func TestStoredThen502ThreadBatch(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, _ := w.imported()
	batch := []map[string]any{threadObj("6501", chForum, "Question", nil), msgObj("7501", userAlex, "first", tsStart, nil),
		msgObj("7502", userSam, "second", tsReply, nil)}
	_ = w.threadBatch(id, "thread:6501", batch...) // the answer is lost
	seqs := func() string {
		return w.text(`SELECT string_agg(m.id::text || '@' || m.thread_seq, ',' ORDER BY m.thread_seq) FROM messages m
			JOIN import_items i ON i.id = m.import_item_id WHERE i.source_id = $1`, id)
	}
	before := seqs()
	retry := w.mustThreadBatch(id, "thread:6501", batch...)
	if after := seqs(); after != before {
		t.Errorf("messages %s then %s", before, after)
	}
	want := retry.Items[1].TargetID + "@1," + retry.Items[2].TargetID + "@2"
	if before != want {
		t.Errorf("stored %s, answered %s", before, want)
	}
}

// TestMessageOrderAndSeq (22 6.2): thread_seq follows the request order, not created_at;
// created_at and edited_at come from the data, without a revision; the server sets client_msg_id
// and import_item_id.
func TestMessageOrderAndSeq(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, _ := w.imported()
	res := w.mustThreadBatch(id, "thread:6601", threadObj("6601", chForum, "Order", nil),
		msgObj("7601", userAlex, "later in time", tsLater, map[string]any{"edited_at": tsEdit}), msgObj("7602", userSam, "earlier in time", tsStart, nil))
	got := w.text(`SELECT string_agg(id::text || '@' || thread_seq, ',' ORDER BY thread_seq) FROM messages WHERE thread_id = $1`, res.Items[0].TargetID)
	if want := res.Items[1].TargetID + "@1," + res.Items[2].TargetID + "@2"; got != want {
		t.Errorf("order %s, want %s", got, want)
	}
	if n := w.count(`SELECT count(*) FROM messages m JOIN import_items i ON i.id = m.import_item_id WHERE m.id = $1 AND m.origin = 'import'
		AND m.kind = 'text' AND m.created_at = '2025-03-01T09:00:00Z' AND m.edited_at = '2025-03-01T08:07:00Z' AND i.origin_key = 'message:7601'`,
		res.Items[1].TargetID); n != 1 {
		t.Error("the fields of the imported message")
	}
	if n := w.count(`SELECT count(*) FROM message_revisions`); n != 0 {
		t.Errorf("%d revisions", n)
	}
	if n := w.count(`SELECT count(*) FROM messages WHERE thread_id = $1 AND author_principal_id = $2`, res.Items[0].TargetID, w.alex.PrincipalID); n != 1 {
		t.Error("the author is not the mapped person")
	}
}

// TestAuthorNotImported (22 5.3): a message of a skipped or unknown author is 409 conflict with
// reason author_not_imported, and the batch writes nothing.
func TestAuthorNotImported(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, _ := w.imported()
	for _, author := range []string{userOtherB, "999"} {
		w.wantProblem(w.threadBatch(id, "thread:6701", threadObj("6701", chForum, "Q", nil), msgObj("7701", author, "hi", tsStart, nil)),
			http.StatusConflict, "conflict", "author_not_imported")
	}
	if n := w.count(`SELECT count(*) FROM import_items WHERE source_id = $1 AND item_kind IN ('thread', 'message')`, id); n != 0 {
		t.Errorf("%d items of a refused batch", n)
	}
}

// TestReplyTargetRules (22 6.2): a reply to a posted message of the same thread links it; a reply
// to a message of another thread or an unknown one is 400 at the pointer.
func TestReplyTargetRules(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, _ := w.imported()
	w.mustThreadBatch(id, "thread:6801", threadObj("6801", chForum, "A", nil), msgObj("7801", userAlex, "in A", tsStart, nil))
	res := w.mustThreadBatch(id, "thread:6802", threadObj("6802", chForum, "B", nil), msgObj("7802", userAlex, "in B", tsStart, nil),
		msgObj("7803", userSam, "reply in B", tsReply, map[string]any{"reply_to_origin_key": "message:7802"}))
	if got := w.text(`SELECT coalesce(reply_to_message_id::text, '') FROM messages WHERE id = $1`, res.Items[2].TargetID); got != res.Items[1].TargetID {
		t.Errorf("reply target %s", got)
	}
	for _, target := range []string{"message:7801", "message:7999"} {
		p := w.wantProblem(w.threadBatch(id, "thread:6802", msgObj("7804", userSam, "x", tsLater, map[string]any{"reply_to_origin_key": target})),
			http.StatusBadRequest, "validation_failed", "")
		if !p.hasError("/items/0/data/reply_to_origin_key", "not_in_thread") {
			t.Errorf("%s: %+v", target, p)
		}
	}
}

// TestPinsImported (22 6.6): a pin item pins the message with the importing owner as pinned_by and
// appends no pin.added.
func TestPinsImported(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, _ := w.imported()
	w.mustThreadBatch(id, "thread:6901", threadObj("6901", chForum, "Pins", nil), msgObj("7901", userAlex, "pin me", tsStart, nil))
	seq := w.maxSeq()
	res := w.mustThreadBatch(id, "thread:6901", pinObj("7901"))
	if res.Items[0].State != "posted" || res.Items[0].TargetKind != "message" {
		t.Errorf("pin item %+v", res.Items[0])
	}
	if n := w.count(`SELECT count(*) FROM pins WHERE message_id = $1 AND pinned_by = $2`, res.Items[0].TargetID, w.robin.PrincipalID); n != 1 {
		t.Error("no pin by the owner")
	}
	if types := w.eventTypes(seq); types["pin.added"] != 0 || types["thread.updated"] != 1 {
		t.Errorf("events %v", types)
	}
	p := w.wantProblem(w.threadBatch(id, "thread:6901", pinObj("7999")), http.StatusBadRequest, "validation_failed", "")
	if !p.hasError("/items/0/data/message_origin_key", "not_imported") {
		t.Errorf("unknown pin target %+v", p)
	}
}

// TestReadOnlyChannelAcceptsImport (05 rule 1.8, 22 12.1): the read-only forum accepts the batch
// that Robin's createThread is refused in.
func TestReadOnlyChannelAcceptsImport(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, items := w.imported()
	forum := items["channel:"+chForum].TargetID
	w.wantProblem(w.do(w.robin, http.MethodPost, "/channels/"+forum+"/threads", map[string]any{"title": "x",
		"first_message": map[string]any{"client_msg_id": ids.New().String(), "content_md": "x"}}, nil), http.StatusForbidden, "channel_read_only", "")
	res := w.mustThreadBatch(id, "thread:7001", threadObj("7001", chForum, "Mirrored", nil), msgObj("8001", userHook, "deployed", tsStart, nil))
	if res.Items[0].State != "posted" || res.Items[1].State != "posted" {
		t.Errorf("items %+v", res.Items)
	}
	w.wantProblem(w.threadBatch(id, "channel:"+chForum, msgObj("8002", userAlex, "x", tsStart, nil)), http.StatusBadRequest, "validation_failed", "")
}

// TestSearchLanguageOnImportedMessage (14 3.1, M3-13): an imported message and thread carry the
// language of the instance, and the search vector is built from it.
func TestSearchLanguageOnImportedMessage(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, _ := w.imported()
	res := w.mustThreadBatch(id, "thread:7101", threadObj("7101", chForum, "Search", nil), msgObj("8101", userAlex, "searchable words", tsStart, nil))
	instance := w.text(`SELECT search_language::text FROM threads WHERE kind = 'main' ORDER BY created_at LIMIT 1`)
	if got := w.text(`SELECT search_language::text FROM messages WHERE id = $1`, res.Items[1].TargetID); got != instance {
		t.Errorf("message language %s, instance %s", got, instance)
	}
	if got := w.text(`SELECT search_language::text FROM threads WHERE id = $1`, res.Items[0].TargetID); got != instance {
		t.Errorf("thread language %s, instance %s", got, instance)
	}
	if n := w.count(`SELECT count(*) FROM messages WHERE id = $1 AND search_vector @@ plainto_tsquery('simple', 'searchable')`, res.Items[1].TargetID); n != 1 {
		t.Error("the imported message is not searchable")
	}
}

// fileSlot is the answer of createImportFile.
type fileSlot struct {
	Item   item `json:"item"`
	Upload *struct {
		FileID    string `json:"file_id"`
		ExpiresAt string `json:"expires_at"`
		Parts     []struct {
			PartNumber int    `json:"part_number"`
			URL        string `json:"url"`
		} `json:"parts"`
	} `json:"upload"`
}

// fileBody is the body of createImportFile for b.
func fileBody(key, channel, author string, b []byte) map[string]any {
	sum := sha256.Sum256(b)
	return map[string]any{"origin_key": key, "channel_id": channel, "author_id": author, "name": "report.pdf", "size_bytes": len(b),
		"sha256": hex.EncodeToString(sum[:]), "declared_mime_type": "application/pdf"}
}

// createFile calls createImportFile.
func (w *iw) createFile(id string, body map[string]any) harness.Response {
	w.t.Helper()
	return w.do(w.robin, http.MethodPost, "/imports/"+id+"/files", body, nil)
}

// mustCreateFile calls createImportFile and wants 200.
func (w *iw) mustCreateFile(id string, body map[string]any) fileSlot {
	w.t.Helper()
	r := w.createFile(id, body)
	if r.Status != http.StatusOK {
		w.t.Fatalf("createImportFile = %d %s", r.Status, r.Body)
	}
	var s fileSlot
	r.JSON(w.t, &s)
	return s
}

// putFilePart sends b as part 1 of the import file.
func (w *iw) putFilePart(id, file string, b []byte) harness.Response {
	w.t.Helper()
	sum := sha256.Sum256(b)
	h := http.Header{}
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Digest", "sha-256=:"+base64.StdEncoding.EncodeToString(sum[:])+":")
	return w.do(w.robin, http.MethodPut, "/imports/"+id+"/files/"+file+"/parts/1", b, h)
}

// completeFile calls completeImportFile.
func (w *iw) completeFile(id, file string) harness.Response {
	w.t.Helper()
	return w.do(w.robin, http.MethodPost, "/imports/"+id+"/files/"+file+"/complete", map[string]any{"parts": []any{}}, nil)
}

// importFile uploads b as the file item key of author in channel and returns its file id.
func (w *iw) importFile(id, key, channel, author string, b []byte) string {
	w.t.Helper()
	s := w.mustCreateFile(id, fileBody(key, channel, author, b))
	if s.Upload == nil {
		w.t.Fatalf("no upload session: %+v", s)
	}
	if r := w.putFilePart(id, s.Item.TargetID, b); r.Status != http.StatusOK {
		w.t.Fatalf("uploadImportFilePart = %d %s", r.Status, r.Body)
	}
	if r := w.completeFile(id, s.Item.TargetID); r.Status != http.StatusOK {
		w.t.Fatalf("completeImportFile = %d %s", r.Status, r.Body)
	}
	return s.Item.TargetID
}

// TestCreateImportFileSameID (plan acceptance, 22 7.2 item 3): three calls with one key return one
// file id; an aborted session gets a new session for the same id; after the completion the item is
// posted and the call returns no upload; no quota is charged.
func TestCreateImportFileSameID(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, items := w.imported()
	forum := items["channel:"+chForum].TargetID
	b := []byte("imported attachment bytes")
	body := fileBody("file:9001", forum, w.alex.PrincipalID, b)
	first := w.mustCreateFile(id, body)
	if first.Item.State != "pending" || first.Item.TargetKind != "file" || first.Upload == nil || first.Upload.FileID != first.Item.TargetID {
		t.Fatalf("first %+v", first)
	}
	if len(first.Upload.Parts) != 1 || first.Upload.Parts[0].URL != "/api/v1/imports/"+id+"/files/"+first.Item.TargetID+"/parts/1" {
		t.Errorf("parts %+v", first.Upload.Parts)
	}
	for range 2 {
		if again := w.mustCreateFile(id, body); again.Item.TargetID != first.Item.TargetID || again.Upload == nil {
			t.Fatalf("repeat %+v", again)
		}
	}
	if _, err := w.s.AdminDB().Exec(w.ctx, `UPDATE file_upload_sessions SET state = 'aborted' WHERE file_id = $1`, first.Item.TargetID); err != nil {
		t.Fatal(err)
	}
	reopened := w.mustCreateFile(id, body)
	if reopened.Item.TargetID != first.Item.TargetID || reopened.Upload == nil {
		t.Fatalf("after the abort %+v", reopened)
	}
	if got := w.text(`SELECT state FROM file_upload_sessions WHERE file_id = $1`, first.Item.TargetID); got != "open" {
		t.Errorf("session %s", got)
	}
	if r := w.putFilePart(id, first.Item.TargetID, b); r.Status != http.StatusOK {
		t.Fatalf("part %d %s", r.Status, r.Body)
	}
	if r := w.completeFile(id, first.Item.TargetID); r.Status != http.StatusOK {
		t.Fatalf("complete %d %s", r.Status, r.Body)
	}
	done := w.mustCreateFile(id, body)
	if done.Item.State != "posted" || done.Upload != nil || done.Item.TargetID != first.Item.TargetID {
		t.Errorf("after the completion %+v", done)
	}
	if n := w.count(`SELECT count(*) FROM files WHERE uploaded_by = $1`, w.alex.PrincipalID); n != 1 {
		t.Errorf("%d files of the author", n)
	}
	if n := w.count(`SELECT count(*) FROM files WHERE id = $1 AND scope_channel_id = $2 AND purpose = 'attachment'`, first.Item.TargetID, forum); n != 1 {
		t.Error("the file row")
	}
}

// TestImportFileInReadOnlyChannel (05 rule 1.8 and 4.12): parts and completion through the import
// operations succeed in the read-only forum, where the owner's own createUpload is refused with
// 403 channel_read_only; the file attaches to an imported message.
func TestImportFileInReadOnlyChannel(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, items := w.imported()
	forum := items["channel:"+chForum].TargetID
	b := []byte("bytes of a mirrored attachment")
	sum := sha256.Sum256(b)
	w.wantProblem(w.do(w.robin, http.MethodPost, "/channels/"+forum+"/uploads", map[string]any{"name": "x.pdf", "size_bytes": len(b),
		"sha256": hex.EncodeToString(sum[:])}, nil), http.StatusForbidden, "channel_read_only", "")
	file := w.importFile(id, "file:9101", forum, w.alex.PrincipalID, b)
	res := w.mustThreadBatch(id, "thread:7201", threadObj("7201", chForum, "With a file", nil),
		msgObj("8201", userAlex, "", tsStart, map[string]any{"attachment_file_ids": []string{file}}))
	if n := w.count(`SELECT count(*) FROM message_attachments WHERE message_id = $1 AND file_id = $2`, res.Items[1].TargetID, file); n != 1 {
		t.Error("the file is not attached")
	}
	if n := w.count(`SELECT count(*) FROM messages WHERE id = $1 AND has_attachments`, res.Items[1].TargetID); n != 1 {
		t.Error("has_attachments is not set")
	}
}

// TestAttachmentRules (12 10, M8-11 design note 4 d): a file of another channel, a pending file and
// more files than message.max_attachments are refused, and the batch writes nothing.
func TestAttachmentRules(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, items := w.imported()
	forum, text := items["channel:"+chForum].TargetID, items["channel:"+chText].TargetID
	other := w.importFile(id, "file:9201", text, w.alex.PrincipalID, []byte("a file of the text channel"))
	pending := w.mustCreateFile(id, fileBody("file:9202", forum, w.alex.PrincipalID, []byte("never uploaded"))).Item.TargetID
	thread := threadObj("7301", chForum, "Files", nil)
	r := w.threadBatch(id, "thread:7301", thread, msgObj("8301", userAlex, "x", tsStart, map[string]any{"attachment_file_ids": []string{other}}))
	w.wantProblem(r, http.StatusConflict, "conflict", "file_not_attachable")
	p := w.wantProblem(w.threadBatch(id, "thread:7301", thread, msgObj("8301", userAlex, "x", tsStart,
		map[string]any{"attachment_file_ids": []string{pending}})), http.StatusBadRequest, "validation_failed", "")
	if !p.hasError("/items/1/data/attachment_file_ids/0", "not_imported") {
		t.Errorf("pending file %+v", p)
	}
	max, _ := strconv.Atoi(w.s.ConfigValue("message.max_attachments"))
	many := make([]string, max+1)
	for i := range many {
		many[i] = other
	}
	p = w.wantProblem(w.threadBatch(id, "thread:7301", thread, msgObj("8301", userAlex, "x", tsStart, map[string]any{"attachment_file_ids": many})),
		http.StatusBadRequest, "validation_failed", "")
	if !p.hasError("/items/1/data/attachment_file_ids", "limit_reached") {
		t.Errorf("too many files %+v", p)
	}
	if n := w.count(`SELECT count(*) FROM import_items WHERE source_id = $1 AND item_kind IN ('thread', 'message')`, id); n != 0 {
		t.Errorf("%d items of refused batches", n)
	}
}

// TestImportFileScanPriority (12 11, M8-11 design note 6 c): the scan job of an import file has the
// lowest priority and its mark; a live upload completed later gets the default priority, so the
// worker fetches it first (the order itself is held by platform/jobs
// TestInsertTx_PriorityFetchOrder).
func TestImportFileScanPriority(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, items := w.imported()
	here := items["channel:"+chHere].TargetID
	file := w.importFile(id, "file:9301", here, w.alex.PrincipalID, []byte("an imported file"))
	live := []byte("a live upload")
	sum := sha256.Sum256(live)
	var s struct {
		FileID string `json:"file_id"`
	}
	w.must(w.robin, http.MethodPost, "/channels/"+here+"/uploads", map[string]any{"name": "live.txt", "size_bytes": len(live),
		"sha256": hex.EncodeToString(sum[:])}, nil, http.StatusCreated).JSON(t, &s)
	h := http.Header{}
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Digest", "sha-256=:"+base64.StdEncoding.EncodeToString(sum[:])+":")
	w.must(w.robin, http.MethodPut, "/uploads/"+s.FileID+"/parts/1", live, h, http.StatusOK)
	w.must(w.robin, http.MethodPost, "/uploads/"+s.FileID+"/complete", map[string]any{"parts": []any{}}, nil, http.StatusOK)
	prio := func(f string) int {
		return w.count(`SELECT min(priority) FROM jobs.river_job WHERE kind = 'files.scan' AND args->>'file_id' = $1`, f)
	}
	if p := prio(file); p != 4 {
		t.Errorf("import scan priority %d", p)
	}
	if n := w.count(`SELECT count(*) FROM jobs.river_job WHERE kind = 'files.scan' AND args->>'file_id' = $1 AND (args->>'bulk')::boolean`, file); n < 1 {
		t.Error("the import scan job has no bulk mark")
	}
	if p := prio(s.FileID); p != 1 {
		t.Errorf("live scan priority %d", p)
	}
}
