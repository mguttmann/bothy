// SPDX-License-Identifier: Apache-2.0

package attachments_test

import (
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// The JSON pointers of the attachment list in createMessage and in createThread.
const (
	listPointer  = "/attachment_file_ids"
	firstPointer = "/first_message/attachment_file_ids"
)

// rowsOf returns the rows of message_attachments of a message in position order as
// "file_id thread_id channel_id position".
func (e *env) rowsOf(t *testing.T, messageID string) []string {
	t.Helper()
	rs, err := e.db.Query(t.Context(), `SELECT file_id::text, thread_id::text, channel_id::text, position FROM message_attachments
		WHERE message_id = $1 ORDER BY position`, messageID)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	var out []string
	for rs.Next() {
		var f, th, ch string
		var pos int
		if err := rs.Scan(&f, &th, &ch, &pos); err != nil {
			t.Fatal(err)
		}
		out = append(out, f+" "+th+" "+ch+" "+strconv.Itoa(pos))
	}
	if err := rs.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// hasAttachments reads messages.has_attachments.
func (e *env) hasAttachments(t *testing.T, messageID string) bool {
	t.Helper()
	var b bool
	if err := e.db.QueryRow(t.Context(), `SELECT has_attachments FROM messages WHERE id = $1`, messageID).Scan(&b); err != nil {
		t.Fatal(err)
	}
	return b
}

// createdEvent returns the message.created envelope of messageID that c receives after seq and
// checks it against MessageCreatedEvent.
func createdEvent(t *testing.T, c *harness.Client, after int64, messageID string) message {
	t.Helper()
	evs := eventsOf(t, c, after, "message.created", messageID)
	if len(evs) != 1 {
		t.Fatalf("%d message.created of %s", len(evs), messageID)
	}
	validateSchema(t, "MessageCreatedEvent", evs[0].Raw)
	var p struct {
		Message json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(evs[0].Payload, &p); err != nil {
		t.Fatal(err)
	}
	return decodeMessage(t, p.Message)
}

// TestAttach_SendWithAttachments: Alex sends a message with two files in Support/Allgemein: 201
// with the attachments in request order, each File in its attached form; two rows with the thread
// and channel of the message; has_attachments true; message.created valid and listing both. A
// message without files has has_attachments false and attachments []. With clear_draft the draft
// that held both files is gone (12 10 point 4, M1-61).
func TestAttach_SendWithAttachments(t *testing.T) {
	t.Parallel()
	e := start(t)
	th := e.newThread(t, e.alex, e.allg)
	f1 := e.file(t, e.alex, e.allg, "plan.txt", []byte("the plan of Alex"))
	f2 := e.file(t, e.alex, e.allg, "notes.txt", []byte("the notes of Alex"))
	after := e.maxSeq(t)
	m := msgOf(t, e.send(t, e.alex, th, "Two files", []string{f1, f2}, nil), http.StatusCreated)
	if got := fileIDs(t, m); !slices.Equal(got, []string{f1, f2}) {
		t.Fatalf("attachments %v, want %v", got, []string{f1, f2})
	}
	attachedForm(t, m)
	if m.ThreadID != th || m.ChannelID != e.allg {
		t.Fatalf("message in %s/%s", m.ThreadID, m.ChannelID)
	}
	want := []string{f1 + " " + th + " " + e.allg + " 1", f2 + " " + th + " " + e.allg + " 2"}
	if got := e.rowsOf(t, m.ID); !slices.Equal(got, want) {
		t.Fatalf("rows %v, want %v", got, want)
	}
	if !e.hasAttachments(t, m.ID) {
		t.Fatal("has_attachments false after a send with files")
	}
	ev := createdEvent(t, e.alex, after, m.ID)
	if got := fileIDs(t, ev); !slices.Equal(got, []string{f1, f2}) {
		t.Fatalf("message.created lists %v", got)
	}
	attachedForm(t, ev)

	plain := msgOf(t, e.send(t, e.alex, th, "No files", nil, nil), http.StatusCreated)
	if len(plain.Attachments) != 0 || e.hasAttachments(t, plain.ID) {
		t.Fatalf("message without files: attachments %v, has_attachments %v", plain.Attachments, e.hasAttachments(t, plain.ID))
	}

	t.Run("ClearDraft", func(t *testing.T) {
		f3 := e.file(t, e.alex, e.allg, "draft-a.txt", []byte("draft file a"))
		f4 := e.file(t, e.alex, e.allg, "draft-b.txt", []byte("draft file b"))
		harness.MustDo(t, e.alex, harness.Request{Method: http.MethodPut, Path: "/threads/" + th + "/draft",
			Body: map[string]any{"content_md": "With the files", "file_ids": []string{f3, f4}}}, http.StatusOK)
		drafts := `SELECT count(*) FROM drafts WHERE user_id = $1 AND thread_id = $2`
		if n := e.count(t, drafts, e.alex.PrincipalID, th); n != 1 {
			t.Fatalf("%d drafts before the send", n)
		}
		m := msgOf(t, e.send(t, e.alex, th, "With the files", []string{f3, f4}, map[string]any{"clear_draft": true}), http.StatusCreated)
		if got := fileIDs(t, m); !slices.Equal(got, []string{f3, f4}) {
			t.Fatalf("attachments %v", got)
		}
		if n := e.count(t, drafts, e.alex.PrincipalID, th); n != 0 {
			t.Fatalf("%d drafts after the send with clear_draft", n)
		}
	})
}

// TestAttach_SecondAttach409 (12 15 row "Attaching the same file a second time"): a file attached
// to M1 refuses M2 with 409 file_not_attachable; M2 is not stored and the next message takes the
// next thread_seq without a gap.
func TestAttach_SecondAttach409(t *testing.T) {
	t.Parallel()
	e := start(t)
	th := e.newThread(t, e.alex, e.allg)
	f := e.file(t, e.alex, e.allg, "once.txt", []byte("attached once"))
	m1 := msgOf(t, e.send(t, e.alex, th, "First", []string{f}, nil), http.StatusCreated)
	seqQ := `SELECT max(thread_seq)::int FROM messages WHERE thread_id = $1`
	seq, msgs := e.count(t, seqQ, th), e.count(t, `SELECT count(*) FROM messages WHERE thread_id = $1`, th)
	wantNotAttachable(t, e.send(t, e.alex, th, "Second", []string{f}, nil), listPointer+"/0")
	if n := e.count(t, `SELECT count(*) FROM messages WHERE thread_id = $1`, th); n != msgs {
		t.Fatalf("%d messages after the refusal, want %d", n, msgs)
	}
	if got := e.rowsOf(t, m1.ID); len(got) != 1 {
		t.Fatalf("rows of M1 %v", got)
	}
	if n := e.count(t, `SELECT count(*) FROM message_attachments WHERE file_id = $1`, f); n != 1 {
		t.Fatalf("%d rows of the file", n)
	}
	m3 := msgOf(t, e.send(t, e.alex, th, "Third", nil, nil), http.StatusCreated)
	if m3.ThreadSeq != int64(seq)+1 {
		t.Fatalf("thread_seq %d after the refusal, want %d", m3.ThreadSeq, seq+1)
	}
}

// TestAttach_NotAttachable_SEC_UP_9: every file 12 10 point 1 refuses answers the same 409
// conflict with reason file_not_attachable and the same headers, detail naming the entry, and
// nothing is stored: a file of Alex for Support/Eskalation sent in Allgemein (SEC-UP-9), Lotse's
// file sent by Alex, an unknown id, an open session, an aborted upload, an upload failed by
// checksum_mismatch, a row with purpose avatar and the same id twice in one request.
func TestAttach_NotAttachable_SEC_UP_9(t *testing.T) {
	t.Parallel()
	e := start(t)
	th := e.newThread(t, e.alex, e.allg)
	good := e.file(t, e.alex, e.allg, "good.txt", []byte("a good file"))
	otherChannel := e.file(t, e.alex, e.esk, "escalation.txt", []byte("for Eskalation"))
	lotses := e.file(t, e.lotse, e.allg, "lotse.txt", []byte("a file of Lotse"))
	open := e.open(t, e.alex, e.allg, "open.txt", []byte("never completed")).FileID
	aborted := e.open(t, e.alex, e.allg, "aborted.txt", []byte("aborted upload"))
	harness.MustDo(t, e.alex, harness.Request{Method: http.MethodDelete, Path: "/uploads/" + aborted.FileID}, http.StatusNoContent)
	tampered := e.open(t, e.alex, e.allg, "tampered.txt", []byte("0123456789"))
	if err := os.WriteFile(filepath.Join(e.s.FilesRoot, "uploads", tampered.FileID, "1"), []byte("012345678X"), 0o600); err != nil {
		t.Fatal(err)
	}
	e.complete(t, e.alex, tampered)
	if st := e.waitFinal(t, tampered.FileID); st != "failed/pending" {
		t.Fatalf("tampered upload ended %s", st)
	}
	if r := e.text(t, `SELECT failure_reason FROM file_upload_sessions WHERE file_id = $1`, tampered.FileID); r != "checksum_mismatch" {
		t.Fatalf("failure_reason %q", r)
	}
	avatar := ids.New()
	key, err := domain.OriginalKey(avatar, 1)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("avatar"))
	e.exec(t, `INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose, upload_state, scan_status)
		VALUES ($1, $2, $3, 6, 'image/png', 'alex.png', $4, 'avatar', 'complete', 'clean')`, avatar.String(), key, sum[:], e.alex.PrincipalID)

	msgs := `SELECT count(*) FROM messages WHERE thread_id = $1`
	before, rows := e.count(t, msgs, th), e.count(t, `SELECT count(*) FROM message_attachments`)
	var first string
	for _, c := range []struct {
		name, pointer string
		list          []string
	}{
		{"OtherChannel", listPointer + "/0", []string{otherChannel}},
		{"ForeignUploader", listPointer + "/0", []string{lotses}},
		{"Unknown", listPointer + "/0", []string{ids.New().String()}},
		{"OpenSession", listPointer + "/0", []string{open}},
		{"Aborted", listPointer + "/0", []string{aborted.FileID}},
		{"ChecksumMismatch", listPointer + "/0", []string{tampered.FileID}},
		{"Avatar", listPointer + "/0", []string{avatar.String()}},
		{"SameIDTwice", listPointer + "/1", []string{good, good}},
		{"GoodThenForeign", listPointer + "/1", []string{good, lotses}},
	} {
		r := e.send(t, e.alex, th, "Refused "+c.name, c.list, nil)
		wantNotAttachable(t, r, c.pointer)
		got := shape(t, r)
		if first == "" {
			first = got
		} else if got != first {
			t.Errorf("%s answers\n%s\nthe first answered\n%s", c.name, got, first)
		}
	}
	if n := e.count(t, msgs, th); n != before {
		t.Fatalf("%d messages after the refusals, want %d", n, before)
	}
	if n := e.count(t, `SELECT count(*) FROM message_attachments`); n != rows {
		t.Fatalf("%d attachment rows after the refusals, want %d", n, rows)
	}
	// The good file was never bound and stays attachable.
	msgOf(t, e.send(t, e.alex, th, "Good", []string{good}, nil), http.StatusCreated)
}

// TestAttach_ConcurrentSameFile: two sends with the same file at the same time, into two threads
// so only the file row lock orders them: exactly one 201 and one 409 file_not_attachable.
func TestAttach_ConcurrentSameFile(t *testing.T) {
	t.Parallel()
	e := start(t)
	threads := []string{e.newThread(t, e.alex, e.allg), e.newThread(t, e.alex, e.allg)}
	for round := range 3 {
		f := e.file(t, e.alex, e.allg, "race-"+strconv.Itoa(round)+".txt", []byte("raced file "+strconv.Itoa(round)))
		var wg sync.WaitGroup
		results := make([]harness.Response, len(threads))
		for i, th := range threads {
			wg.Go(func() {
				r, err := harness.DoWithStepUp(t.Context(), e.alex, harness.Request{Method: http.MethodPost, Path: "/threads/" + th + "/messages",
					Body: body("Race", []string{f}, nil)})
				if err != nil {
					t.Error(err)
				}
				results[i] = r
			})
		}
		wg.Wait()
		created, refused := 0, 0
		for _, r := range results {
			switch r.Status {
			case http.StatusCreated:
				created++
			case http.StatusConflict:
				wantProblem(t, r, http.StatusConflict, "conflict", "file_not_attachable")
				refused++
			default:
				t.Fatalf("status %d %s", r.Status, r.Body)
			}
		}
		if created != 1 || refused != 1 {
			t.Fatalf("round %d: %d created, %d refused", round, created, refused)
		}
		if n := e.count(t, `SELECT count(*) FROM message_attachments WHERE file_id = $1`, f); n != 1 {
			t.Fatalf("%d rows of the raced file", n)
		}
	}
}

// TestAttach_LimitCapabilityNotEnabled: more than message.max_attachments ids give 400
// validation_failed with limit_reached; Robin with write and without upload on Support/Eskalation
// gets 403 forbidden with reason capability_missing for a message with files and 201 without; the
// M1-56 answer not_enabled for attachment_file_ids is gone.
func TestAttach_LimitCapabilityNotEnabled(t *testing.T) {
	t.Parallel()
	e := start(t)
	th := e.newThread(t, e.alex, e.allg)
	over := make([]string, maxAttachments+1)
	for i := range over {
		over[i] = ids.New().String()
	}
	wantField(t, e.send(t, e.alex, th, "Too many", over, nil), listPointer, "limit_reached")
	// At the limit the body rules pass and the files decide: no not_enabled any more.
	wantNotAttachable(t, e.send(t, e.alex, th, "At the limit", over[:maxAttachments], nil), listPointer+"/0")
	f := e.file(t, e.alex, e.allg, "allowed.txt", []byte("allowed"))
	msgOf(t, e.send(t, e.alex, th, "Allowed", []string{f}, nil), http.StatusCreated)

	harness.MustDo(t, e.owner, harness.Request{Method: http.MethodPost, Path: "/grants", Body: map[string]any{
		"subject_kind": "user", "subject_id": e.robin.PrincipalID, "target_kind": "channel", "target_id": e.esk,
		"level": "write", "reason": "write without upload",
	}}, http.StatusCreated)
	rth := e.newThread(t, e.owner, e.esk)
	wantProblem(t, e.send(t, e.robin, rth, "With a file", []string{f}, nil), http.StatusForbidden, "forbidden", "capability_missing")
	wantProblem(t, e.send(t, e.robin, rth, "With an unknown file", []string{ids.New().String()}, nil), http.StatusForbidden, "forbidden",
		"capability_missing")
	msgOf(t, e.send(t, e.robin, rth, "Without a file", nil, nil), http.StatusCreated)
}

// TestAttach_AttachmentOnlyAndPending: a message with only a file and empty or blank content_md is
// 201; a file whose scan has not run yet is attachable and shows scan_status pending; without
// files the empty content is still 400 empty.
func TestAttach_AttachmentOnlyAndPending(t *testing.T) {
	t.Parallel()
	e := start(t)
	th := e.newThread(t, e.alex, e.allg)
	e.stallScans()
	f1 := e.file(t, e.alex, e.allg, "picture-1.txt", []byte("a picture without caption"))
	f2 := e.file(t, e.alex, e.allg, "picture-2.txt", []byte("another picture"))
	for _, f := range []string{f1, f2} {
		if st := e.state(t, f); st != "pending/pending" {
			t.Fatalf("file %s is %s before the send", f, st)
		}
	}
	for _, c := range []struct {
		content string
		file    string
	}{{"", f1}, {" \n\t ", f2}} {
		m := msgOf(t, e.send(t, e.alex, th, c.content, []string{c.file}, nil), http.StatusCreated)
		if got := fileIDs(t, m); !slices.Equal(got, []string{c.file}) {
			t.Fatalf("attachments %v", got)
		}
		if s := m.Attachments[0].File["scan_status"]; s != "pending" {
			t.Fatalf("scan_status %v, want pending", s)
		}
		attachedForm(t, m)
	}
	wantField(t, e.send(t, e.alex, th, "", nil, nil), "/content_md", "empty")
	e.answerScans()
}

// TestAttach_ReplayAndHash: the same request again answers 200 with Idempotent-Replay true, the
// same attachments and no new row; the same client_msg_id with the files in another order answers
// 422 idempotency_key_reuse.
func TestAttach_ReplayAndHash(t *testing.T) {
	t.Parallel()
	e := start(t)
	th := e.newThread(t, e.alex, e.allg)
	f1 := e.file(t, e.alex, e.allg, "one.txt", []byte("file one"))
	f2 := e.file(t, e.alex, e.allg, "two.txt", []byte("file two"))
	b := body("Replayed", []string{f1, f2}, nil)
	first := msgOf(t, e.post(t, e.alex, th, b), http.StatusCreated)
	rows := e.count(t, `SELECT count(*) FROM message_attachments`)
	r := e.post(t, e.alex, th, b)
	again := msgOf(t, r, http.StatusOK)
	if r.Header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("Idempotent-Replay %q", r.Header.Get("Idempotent-Replay"))
	}
	if again.ID != first.ID || !slices.Equal(fileIDs(t, again), []string{f1, f2}) {
		t.Fatalf("replay %s with %v, original %s", again.ID, fileIDs(t, again), first.ID)
	}
	attachedForm(t, again)
	if n := e.count(t, `SELECT count(*) FROM message_attachments`); n != rows {
		t.Fatalf("%d rows after the replay, want %d", n, rows)
	}
	swapped := map[string]any{"client_msg_id": b["client_msg_id"], "content_md": b["content_md"], "attachment_file_ids": []string{f2, f1}}
	wantProblem(t, e.post(t, e.alex, th, swapped), http.StatusUnprocessableEntity, "idempotency_key_reuse", "")
	if n := e.count(t, `SELECT count(*) FROM message_attachments`); n != rows {
		t.Fatalf("%d rows after the reuse, want %d", n, rows)
	}
}

// TestAttach_CreateThreadFirstMessage: createThread in Support/Allgemein with parent_message_id and
// first_message.attachment_file_ids creates thread, message and rows in one transaction, the rows
// carrying the new thread; over message.max_attachments and a foreign file answer with the
// /first_message/attachment_file_ids pointer and create no thread.
func TestAttach_CreateThreadFirstMessage(t *testing.T) {
	t.Parallel()
	e := start(t)
	main := e.mainThread(t, e.allg)
	f1 := e.file(t, e.alex, e.allg, "first-a.txt", []byte("first a"))
	f2 := e.file(t, e.alex, e.allg, "first-b.txt", []byte("first b"))
	lotses := e.file(t, e.lotse, e.allg, "lotse.txt", []byte("a file of Lotse"))
	createThread := func(files []string) (harness.Response, string) {
		parent := msgOf(t, e.send(t, e.alex, main, "Question", nil, nil), http.StatusCreated)
		r, err := harness.DoWithStepUp(t.Context(), e.alex, harness.Request{Method: http.MethodPost, Path: "/channels/" + e.allg + "/threads",
			Body: map[string]any{"parent_message_id": parent.ID, "first_message": body("First with files", files, nil)}})
		if err != nil {
			t.Fatal(err)
		}
		return r, parent.ID
	}
	r, _ := createThread([]string{f1, f2})
	if r.Status != http.StatusCreated {
		t.Fatalf("createThread %d %s", r.Status, r.Body)
	}
	var out struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
		FirstMessage json.RawMessage `json:"first_message"`
	}
	r.JSON(t, &out)
	m := decodeMessage(t, out.FirstMessage)
	if got := fileIDs(t, m); !slices.Equal(got, []string{f1, f2}) || m.ThreadID != out.Thread.ID {
		t.Fatalf("first message in %s with %v", m.ThreadID, got)
	}
	attachedForm(t, m)
	want := []string{f1 + " " + out.Thread.ID + " " + e.allg + " 1", f2 + " " + out.Thread.ID + " " + e.allg + " 2"}
	if got := e.rowsOf(t, m.ID); !slices.Equal(got, want) {
		t.Fatalf("rows %v, want %v", got, want)
	}
	if !e.hasAttachments(t, m.ID) {
		t.Fatal("has_attachments false")
	}

	over := make([]string, maxAttachments+1)
	for i := range over {
		over[i] = ids.New().String()
	}
	threads := `SELECT count(*) FROM threads WHERE parent_message_id = $1`
	r, parent := createThread(over)
	wantField(t, r, firstPointer, "limit_reached")
	if n := e.count(t, threads, parent); n != 0 {
		t.Fatalf("%d threads after the limit", n)
	}
	r, parent = createThread([]string{lotses})
	wantNotAttachable(t, r, firstPointer+"/0")
	if n := e.count(t, threads, parent); n != 0 {
		t.Fatalf("%d threads after the refusal", n)
	}
}
