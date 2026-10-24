// SPDX-License-Identifier: Apache-2.0

package attachments_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// move moves thread to target as the owner (moveThread) and returns the answer.
func (e *env) move(t *testing.T, thread, target string) harness.Response {
	t.Helper()
	r, err := harness.DoWithStepUp(t.Context(), e.owner, harness.Request{Method: http.MethodPost, Path: "/threads/" + thread + "/move",
		Body: map[string]any{"target_channel_id": target, "reason": "Belongs elsewhere"}})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// get reads path as c.
func get(t *testing.T, c *harness.Client, path string) harness.Response {
	t.Helper()
	return c.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: path})
}

// ticket calls createFileTicket for the original of file id as c.
func ticket(t *testing.T, c *harness.Client, id string) harness.Response {
	t.Helper()
	r, err := harness.DoWithStepUp(t.Context(), c, harness.Request{Method: http.MethodPost, Path: "/files/" + id + "/tickets",
		Body: map[string]any{"variant": "original"}})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// readsOf is the status of getFile, downloadFile and createFileTicket of file id as c.
func readsOf(t *testing.T, c *harness.Client, id string) [3]int {
	t.Helper()
	return [3]int{get(t, c, "/files/"+id).Status, get(t, c, "/files/"+id+"/content").Status, ticket(t, c, id).Status}
}

// notFoundLike checks that the reads of file id as c answer exactly like the reads of an unknown
// id (AZ-P4): the same 404 status, problem and headers.
func notFoundLike(t *testing.T, c *harness.Client, id string) {
	t.Helper()
	unknown := ids.New().String()
	for _, p := range []struct{ got, want harness.Response }{
		{get(t, c, "/files/"+id), get(t, c, "/files/"+unknown)},
		{get(t, c, "/files/"+id+"/content"), get(t, c, "/files/"+unknown+"/content")},
		{ticket(t, c, id), ticket(t, c, unknown)},
	} {
		if p.got.Status != http.StatusNotFound {
			t.Fatalf("status %d %s, want 404", p.got.Status, p.got.Body)
		}
		if g, w := shape(t, p.got), shape(t, p.want); g != w {
			t.Fatalf("the file answers\n%s\nan unknown id answers\n%s", g, w)
		}
	}
}

// TestAttachedFile_ReadPath: an attached file is read through the channel of its message (05 2.2).
// Sam (read on Allgemein) gets 404 before Alex attaches the file and 200 after, without
// scope_channel_id; Robin always gets the 404 of an unknown id; after the owner moves the thread to
// Support/Eskalation Sam gets 404 and the owner 200: the copies decide. downloadFile and
// createFileTicket (M3-05) follow the same rule.
func TestAttachedFile_ReadPath(t *testing.T) {
	t.Parallel()
	e := start(t)
	th := e.newThread(t, e.alex, e.allg)
	f := e.clean(t, e.alex, e.allg, "shared.txt", []byte("shared with the channel"))
	ok := [3]int{http.StatusOK, http.StatusOK, http.StatusCreated}
	if got := readsOf(t, e.alex, f); got != ok {
		t.Fatalf("Alex before attaching: %v", got)
	}
	notFoundLike(t, e.sam, f)
	notFoundLike(t, e.robin, f)

	msgOf(t, e.send(t, e.alex, th, "For everybody", []string{f}, nil), http.StatusCreated)
	if got := readsOf(t, e.sam, f); got != ok {
		t.Fatalf("Sam after attaching: %v", got)
	}
	var file map[string]any
	harness.MustDo(t, e.sam, harness.Request{Method: http.MethodGet, Path: "/files/" + f}, http.StatusOK).JSON(t, &file)
	if _, has := file["scope_channel_id"]; has {
		t.Fatalf("getFile of an attached file carries scope_channel_id: %v", file)
	}
	notFoundLike(t, e.robin, f)

	if r := e.move(t, th, e.esk); r.Status != http.StatusOK {
		t.Fatalf("moveThread %d %s", r.Status, r.Body)
	}
	if scope := e.text(t, `SELECT scope_channel_id::text FROM files WHERE id = $1`, f); scope != e.allg {
		t.Fatalf("scope_channel_id changed to %s", scope)
	}
	notFoundLike(t, e.sam, f)
	notFoundLike(t, e.robin, f)
	if got := readsOf(t, e.owner, f); got != ok {
		t.Fatalf("owner after the move: %v", got)
	}
	if got := readsOf(t, e.alex, f); got != ok {
		t.Fatalf("Alex after the move: %v", got)
	}
}

// TestAttachedFile_UpdatedEventScope: a file attached while pending gets its scan outcome as
// file.updated without audience in the scope {space_id, channel_id, thread_id} of the message, so
// Sam receives it; a file of Alex that is never attached sends its file.updated to Alex only.
func TestAttachedFile_UpdatedEventScope(t *testing.T) {
	t.Parallel()
	e := start(t)
	th := e.newThread(t, e.alex, e.allg)
	e.stallScans()
	f := e.file(t, e.alex, e.allg, "attached.txt", []byte("attached while pending"))
	g := e.file(t, e.alex, e.allg, "kept.txt", []byte("never attached"))
	msgOf(t, e.send(t, e.alex, th, "Pending file", []string{f}, nil), http.StatusCreated)
	if st := e.state(t, f); st != "pending/pending" {
		t.Fatalf("file is %s at the send", st)
	}
	after := e.maxSeq(t)
	e.answerScans()
	for _, id := range []string{f, g} {
		if st := e.waitFinal(t, id); st != "complete/clean" {
			t.Fatalf("file %s ended %s", id, st)
		}
	}
	support := e.w.Spaces["Support"]
	evs := eventsOf(t, e.sam, after, "file.updated", f)
	if len(evs) != 1 {
		t.Fatalf("Sam receives %d file.updated of the attached file", len(evs))
	}
	ev := evs[0]
	if ev.Audience != "" || ev.Scope.SpaceID != support || ev.Scope.ChannelID != e.allg || ev.Scope.ThreadID != th {
		t.Fatalf("attached file.updated: audience %q, scope %+v", ev.Audience, ev.Scope)
	}
	validateSchema(t, "FileUpdatedEvent", ev.Raw)
	var p map[string]any
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		t.Fatal(err)
	}
	if _, has := p["scope_channel_id"]; has {
		t.Errorf("file.updated of an attached file carries scope_channel_id: %s", ev.Payload)
	}
	if n := e.count(t, `SELECT count(*) FROM event_log WHERE type = 'file.updated' AND resource_id = $1 AND seq > $2
		AND (audience_principal_id IS NOT NULL OR thread_id IS DISTINCT FROM $3)`, f, after, th); n != 0 {
		t.Fatalf("%d file.updated of the attached file with an audience or without its thread", n)
	}

	if n := len(eventsOf(t, e.sam, after, "file.updated", g)); n != 0 {
		t.Fatalf("Sam receives %d file.updated of a file never attached", n)
	}
	own := eventsOf(t, e.alex, after, "file.updated", g)
	if len(own) != 1 || own[0].Audience != "self" || own[0].Scope.ChannelID != e.allg || own[0].Scope.ThreadID != "" {
		t.Fatalf("Alex receives %+v for the file never attached", own)
	}
	validateSchema(t, "FileUpdatedEvent", own[0].Raw)
}

// TestHasAttachments_Lifecycle: has_attachments is true after a send with a file, false without and
// false after deleteMessage, with the rows of message_attachments gone and files.deleted_at set in
// the same transaction: a vetoing subscriber of MessageDeleted leaves flag, rows and file
// unchanged. getFile then answers 404 for Alex and Sam; a second delete is 204 without writes.
func TestHasAttachments_Lifecycle(t *testing.T) {
	t.Parallel()
	e := start(t)
	th := e.newThread(t, e.alex, e.allg)
	f := e.clean(t, e.alex, e.allg, "doomed.txt", []byte("deleted with its message"))
	m := msgOf(t, e.send(t, e.alex, th, "With a file", []string{f}, nil), http.StatusCreated)
	plain := msgOf(t, e.send(t, e.alex, th, "Without a file", nil, nil), http.StatusCreated)
	if !e.hasAttachments(t, m.ID) || e.hasAttachments(t, plain.ID) {
		t.Fatalf("has_attachments %v and %v", e.hasAttachments(t, m.ID), e.hasAttachments(t, plain.ID))
	}
	harness.MustDo(t, e.sam, harness.Request{Method: http.MethodGet, Path: "/files/" + f}, http.StatusOK)
	state := func() string {
		t.Helper()
		return e.text(t, `SELECT m.has_attachments::text || ' ' || (m.deleted_at IS NOT NULL)::text || ' ' ||
			(SELECT count(*) FROM message_attachments a WHERE a.message_id = m.id)::text || ' ' ||
			coalesce((SELECT (f.deleted_at IS NOT NULL)::text FROM files f WHERE f.id = $2), 'purged')
			FROM messages m WHERE m.id = $1`, m.ID, f)
	}
	del := func() harness.Response {
		t.Helper()
		r, err := harness.DoWithStepUp(t.Context(), e.alex, harness.Request{Method: http.MethodDelete, Path: "/messages/" + m.ID})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}

	e.veto.ArmMessageDeleted()
	if r := del(); r.Status == http.StatusNoContent {
		t.Fatalf("deleteMessage passed the veto: %d", r.Status)
	}
	if e.veto.Hits() != 1 {
		t.Fatalf("%d vetoes", e.veto.Hits())
	}
	if got := state(); got != "true false 1 false" {
		t.Fatalf("after the vetoed delete: %s", got)
	}

	if r := del(); r.Status != http.StatusNoContent {
		t.Fatalf("deleteMessage %d %s", r.Status, r.Body)
	}
	// The purge job (M3-08) may already have removed the deleted file's row.
	if got := state(); got != "false true 0 true" && got != "false true 0 purged" {
		t.Fatalf("after the delete: %s", got)
	}
	notFoundAlex := get(t, e.alex, "/files/"+f)
	notFoundSam := get(t, e.sam, "/files/"+f)
	if notFoundAlex.Status != http.StatusNotFound || notFoundSam.Status != http.StatusNotFound {
		t.Fatalf("getFile after the delete: Alex %d, Sam %d", notFoundAlex.Status, notFoundSam.Status)
	}

	deletedAtOf := func() string {
		t.Helper()
		return e.text(t, `SELECT coalesce((SELECT deleted_at::text FROM files WHERE id = $1), 'purged')`, f)
	}
	version, deletedAt := e.text(t, `SELECT version::text FROM messages WHERE id = $1`, m.ID), deletedAtOf()
	head := e.maxSeq(t)
	if r := del(); r.Status != http.StatusNoContent {
		t.Fatalf("second deleteMessage %d %s", r.Status, r.Body)
	}
	if v := e.text(t, `SELECT version::text FROM messages WHERE id = $1`, m.ID); v != version {
		t.Fatalf("version %s after the second delete, was %s", v, version)
	}
	// The second delete writes nothing; only the purge may remove the row in between.
	if d := deletedAtOf(); d != deletedAt && d != "purged" {
		t.Fatalf("files.deleted_at %s after the second delete, was %s", d, deletedAt)
	}
	if n := e.count(t, `SELECT count(*) FROM event_log WHERE seq > $1 AND resource_id = $2`, head, m.ID); n != 0 {
		t.Fatalf("%d events after the second delete", n)
	}
}

// TestMoveThread_MovesAttachmentCopies: the owner moves a thread with attachments from Allgemein to
// Eskalation: every message_attachments.channel_id of the thread equals the target in the same
// transaction, thread_id stays, scope_channel_id of every file stays; a vetoing subscriber after
// files.OnThreadMoved rolls the move back and leaves the copies.
func TestMoveThread_MovesAttachmentCopies(t *testing.T) {
	t.Parallel()
	e := start(t)
	th := e.newThread(t, e.alex, e.allg)
	f1 := e.file(t, e.alex, e.allg, "m1.txt", []byte("first message file"))
	f2 := e.file(t, e.alex, e.allg, "m2a.txt", []byte("second message file a"))
	f3 := e.file(t, e.alex, e.allg, "m2b.txt", []byte("second message file b"))
	loose := e.file(t, e.alex, e.allg, "loose.txt", []byte("never attached"))
	msgOf(t, e.send(t, e.alex, th, "One", []string{f1}, nil), http.StatusCreated)
	msgOf(t, e.send(t, e.alex, th, "Two", []string{f2, f3}, nil), http.StatusCreated)
	copies := `SELECT count(*) FROM message_attachments WHERE thread_id = $1 AND channel_id = $2`
	if n := e.count(t, copies, th, e.allg); n != 3 {
		t.Fatalf("%d copies in Allgemein before the move", n)
	}

	e.veto.ArmThreadMoved()
	if r := e.move(t, th, e.esk); r.Status == http.StatusOK {
		t.Fatalf("moveThread passed the veto")
	}
	if e.veto.Hits() != 1 {
		t.Fatalf("%d vetoes", e.veto.Hits())
	}
	if n := e.count(t, copies, th, e.allg); n != 3 {
		t.Fatalf("%d copies in Allgemein after the vetoed move", n)
	}
	if ch := e.text(t, `SELECT channel_id::text FROM threads WHERE id = $1`, th); ch != e.allg {
		t.Fatalf("thread in %s after the vetoed move", ch)
	}

	if r := e.move(t, th, e.esk); r.Status != http.StatusOK {
		t.Fatalf("moveThread %d %s", r.Status, r.Body)
	}
	if n := e.count(t, copies, th, e.esk); n != 3 {
		t.Fatalf("%d copies in Eskalation after the move", n)
	}
	if n := e.count(t, `SELECT count(*) FROM message_attachments WHERE channel_id = $1 OR thread_id <> $2 AND file_id = ANY($3::uuid[])`,
		e.allg, th, []string{f1, f2, f3}); n != 0 {
		t.Fatalf("%d copies outside the moved thread", n)
	}
	if n := e.count(t, `SELECT count(*) FROM files WHERE id = ANY($1::uuid[]) AND scope_channel_id = $2`,
		[]string{f1, f2, f3, loose}, e.allg); n != 4 {
		t.Fatalf("%d files keep scope_channel_id Allgemein, want 4", n)
	}
}

// TestMoveThread_ConcurrentSendWithAttachment: many parallel sends with attachments into a thread
// while the owner moves it: at the end message_attachments.channel_id = messages.channel_id =
// threads.channel_id for every row. A send ordered after the move refuses a file uploaded for the
// old channel (SEC-UP-9) and accepts one uploaded for the new channel, so both kinds race.
func TestMoveThread_ConcurrentSendWithAttachment(t *testing.T) {
	t.Parallel()
	e := start(t)
	th := e.newThread(t, e.alex, e.allg)
	const senders = 8
	var list []string
	for i := range senders {
		ch := e.allg
		if i%2 == 1 {
			ch = e.esk
		}
		list = append(list, e.file(t, e.alex, ch, "race-"+strconv.Itoa(i)+".txt", []byte("file of sender "+strconv.Itoa(i))))
	}
	var wg sync.WaitGroup
	results := make([]harness.Response, senders)
	gate := make(chan struct{})
	for i, f := range list {
		wg.Go(func() {
			<-gate
			r, err := harness.DoWithStepUp(t.Context(), e.alex, harness.Request{Method: http.MethodPost, Path: "/threads/" + th + "/messages",
				Body: body("Racing "+strconv.Itoa(i), []string{f}, nil)})
			if err != nil {
				t.Error(err)
			}
			results[i] = r
		})
	}
	var moved harness.Response
	wg.Go(func() {
		<-gate
		r, err := harness.DoWithStepUp(t.Context(), e.owner, harness.Request{Method: http.MethodPost, Path: "/threads/" + th + "/move",
			Body: map[string]any{"target_channel_id": e.esk, "reason": "Belongs elsewhere"}})
		if err != nil {
			t.Error(err)
		}
		moved = r
	})
	close(gate)
	wg.Wait()
	if moved.Status != http.StatusOK {
		t.Fatalf("moveThread %d %s", moved.Status, moved.Body)
	}
	created := 0
	for i, r := range results {
		switch r.Status {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			wantProblem(t, r, http.StatusConflict, "conflict", "file_not_attachable")
		default:
			t.Fatalf("sender %d: %d %s\nserver log:\n%s", i, r.Status, r.Body, e.logs.lines(`"level":"ERROR"`))
		}
	}
	if created == 0 {
		t.Fatal("no send succeeded")
	}
	if n := e.count(t, `SELECT count(*) FROM message_attachments WHERE thread_id = $1`, th); n != created {
		t.Fatalf("%d rows for %d created messages", n, created)
	}
	if n := e.count(t, `SELECT count(*) FROM message_attachments a JOIN messages m ON m.id = a.message_id JOIN threads t ON t.id = a.thread_id
		WHERE a.thread_id = $1 AND (a.channel_id <> m.channel_id OR m.channel_id <> t.channel_id OR t.channel_id <> $2)`, th, e.esk); n != 0 {
		t.Fatalf("%d rows whose copies differ from the message or the thread", n)
	}
}

// TestHistory_ShowsAttachments: getMessage and listThreadMessages (M1-57) show the attachments of a
// message in position order in the attached form, [] for a message without files, and none for a
// deleted message, for the author and for a reader.
func TestHistory_ShowsAttachments(t *testing.T) {
	t.Parallel()
	e := start(t)
	th := e.newThread(t, e.alex, e.allg)
	f1 := e.file(t, e.alex, e.allg, "h1.txt", []byte("history one"))
	f2 := e.file(t, e.alex, e.allg, "h2.txt", []byte("history two"))
	f3 := e.file(t, e.alex, e.allg, "h3.txt", []byte("history three"))
	m1 := msgOf(t, e.send(t, e.alex, th, "Two files", []string{f1, f2}, nil), http.StatusCreated)
	m2 := msgOf(t, e.send(t, e.alex, th, "No files", nil, nil), http.StatusCreated)
	m3 := msgOf(t, e.send(t, e.alex, th, "Deleted later", []string{f3}, nil), http.StatusCreated)
	harness.MustDo(t, e.alex, harness.Request{Method: http.MethodDelete, Path: "/messages/" + m3.ID}, http.StatusNoContent)
	want := map[string][]string{m1.ID: {f1, f2}, m2.ID: {}, m3.ID: {}}
	for _, c := range []*harness.Client{e.alex, e.sam} {
		for id, files := range want {
			m := msgOf(t, get(t, c, "/messages/"+id), http.StatusOK)
			if got := fileIDs(t, m); !slices.Equal(got, files) {
				t.Fatalf("getMessage %s lists %v, want %v", id, got, files)
			}
			attachedForm(t, m)
		}
		var page struct {
			Items []json.RawMessage `json:"items"`
		}
		harness.MustDo(t, c, harness.Request{Method: http.MethodGet, Path: "/threads/" + th + "/messages"}, http.StatusOK).JSON(t, &page)
		seen := 0
		for _, raw := range page.Items {
			m := decodeMessage(t, raw)
			files, ok := want[m.ID]
			if !ok {
				continue
			}
			seen++
			if got := fileIDs(t, m); !slices.Equal(got, files) {
				t.Fatalf("listThreadMessages %s lists %v, want %v", m.ID, got, files)
			}
			attachedForm(t, m)
		}
		if seen != len(want) {
			t.Fatalf("listThreadMessages shows %d of the %d messages", seen, len(want))
		}
	}
}
