// SPDX-License-Identifier: Apache-2.0

package filecleanup_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/clamd/clamdtest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestPurge_MessageDeleteRemovesObjects (12 12 row "Message deleted"): deleting a message marks
// its two files deleted, redacts their file.updated events and queues one purge per file in the
// same transaction; the purge removes every object and the rows; getFile answers 404.
func TestPurge_MessageDeleteRemovesObjects(t *testing.T) {
	w := newWorld(t)
	f1, f2 := w.clean(w.alex, "report.txt"), w.clean(w.alex, "notes.txt")
	msg := w.mustSend(w.alex, "two files", f1, f2)
	deletedAt := w.clk.Now()
	harness.MustDo(t, w.alex, harness.Request{Method: http.MethodDelete, Path: "/messages/" + msg}, http.StatusNoContent)
	for _, f := range []string{f1, f2} {
		if !w.deleted(f) {
			t.Fatalf("file %s not marked deleted", f)
		}
		if n := w.count(`SELECT count(*) FROM jobs.river_job WHERE kind = 'files.purge_objects' AND args->>'file_id' = $1`, f); n != 1 {
			t.Errorf("file %s: %d purge jobs", f, n)
		}
		if n := w.count(`SELECT count(*) FROM event_log WHERE resource_kind = 'file' AND resource_id = $1 AND redacted_at IS NULL`, f); n != 0 {
			t.Errorf("file %s: %d unredacted events", f, n)
		}
		if n := w.count(`SELECT count(*) FROM event_log WHERE resource_kind = 'file' AND resource_id = $1 AND redacted_at IS NOT NULL`, f); n == 0 {
			t.Errorf("file %s: no redacted event", f)
		}
	}
	w.clk.Advance(time.Minute)
	for _, f := range []string{f1, f2} {
		w.purgeQueued(f)
		if left := w.objectsLeft(f); len(left) != 0 {
			t.Errorf("file %s: objects left %v", f, left)
		}
		if w.rowExists(f) || w.count(`SELECT count(*) FROM file_upload_sessions WHERE file_id = $1`, f) != 0 ||
			w.count(`SELECT count(*) FROM file_upload_parts WHERE file_id = $1`, f) != 0 {
			t.Errorf("file %s: rows left", f)
		}
		for _, c := range []*harness.Client{w.alex, w.sam} {
			if st := w.getFile(c, f); st != http.StatusNotFound {
				t.Errorf("getFile %s: %d", f, st)
			}
		}
	}
	if w.clk.Now().Sub(deletedAt) >= objectDelDelay {
		t.Error("the purge ran outside privacy.object_delete_delay")
	}
}

// TestPurge_CatchUpAfterLostJob (12 12 "catches up on missed deletions"): with the purge job gone
// from the River table, the next cleanup run removes objects and row.
func TestPurge_CatchUpAfterLostJob(t *testing.T) {
	w := newWorld(t)
	f := w.clean(w.alex, "lost.txt")
	msg := w.mustSend(w.alex, "one file", f)
	harness.MustDo(t, w.alex, harness.Request{Method: http.MethodDelete, Path: "/messages/" + msg}, http.StatusNoContent)
	w.exec(`DELETE FROM jobs.river_job WHERE kind = 'files.purge_objects'`)
	w.mustCleanup()
	if w.rowExists(f) || len(w.objectsLeft(f)) != 0 {
		t.Fatalf("row %v, objects %v after the catch-up", w.rowExists(f), w.objectsLeft(f))
	}
}

// TestCleanup_UnattachedAfterTTL (12 15 row "Cleanup"): a never attached file stays one second
// before file.unattached_ttl after its completion and goes after it; a file whose session stayed
// open for a while keeps its full window from the completion.
func TestCleanup_UnattachedAfterTTL(t *testing.T) {
	w := newWorld(t)
	f := w.clean(w.alex, "early.txt")
	content := []byte("a slow upload of the support team")
	slow, parts := w.open(w.alex, "slow.txt", content, content)
	w.clk.Advance(sessionTTL - time.Minute)
	harness.MustDo(t, w.alex, harness.Request{Method: http.MethodPost, Path: "/uploads/" + slow + "/complete", Body: map[string]any{"parts": parts}},
		http.StatusOK)
	w.mustScan(slow)
	w.clk.Advance(unattachedTTL - (sessionTTL - time.Minute) - time.Second)
	w.mustCleanup()
	if w.gone(f) || !w.original(f) {
		t.Fatal("deleted one second before the TTL")
	}
	w.clk.Advance(time.Second)
	w.mustCleanup()
	if !w.gone(f) {
		t.Fatalf("not deleted after the TTL: %s, clock %s", w.times(f), w.clk.Now().UTC())
	}
	if w.gone(slow) {
		t.Fatal("the slow upload lost its window")
	}
	w.purgeQueued(f)
	if w.rowExists(f) || w.original(f) {
		t.Fatal("row or object stays after the purge")
	}
	if st := w.getFile(w.alex, f); st != http.StatusNotFound {
		t.Fatalf("getFile %d", st)
	}
	w.clk.Advance(sessionTTL)
	w.mustCleanup()
	if !w.gone(slow) {
		t.Fatal("the slow upload stays after its window")
	}
}

// TestCleanup_UploaderDraftKeepsFile and a foreign draft: Alex's thread draft holds F after the
// TTL; Sam's draft with the ID of Alex's G protects nothing; after deleteThreadDraft F goes too.
func TestCleanup_UploaderDraftKeepsFile(t *testing.T) {
	w := newWorld(t)
	f := w.clean(w.alex, "draft.txt")
	g := w.clean(w.alex, "foreign.txt")
	harness.MustDo(t, w.alex, harness.Request{Method: http.MethodPut, Path: "/threads/" + w.main + "/draft",
		Body: map[string]any{"content_md": "later", "file_ids": []string{f}}}, http.StatusOK)
	harness.MustDo(t, w.sam, harness.Request{Method: http.MethodPut, Path: "/threads/" + w.main + "/draft",
		Body: map[string]any{"content_md": "keep", "file_ids": []string{g}}}, http.StatusOK)
	w.clk.Advance(unattachedTTL)
	w.mustCleanup()
	if w.gone(f) || !w.gone(g) {
		t.Fatalf("held gone %v, foreign gone %v", w.gone(f), w.gone(g))
	}
	w.purgeQueued(g)
	if n := w.count(`SELECT count(*) FROM drafts WHERE $1::uuid = ANY(file_ids)`, g); n != 1 {
		t.Errorf("Sam's draft lost the ID: %d", n)
	}
	r := w.send(w.alex, "too late", g)
	wantProblem(t, r, http.StatusConflict, "conflict", "file_not_attachable")
	harness.MustDo(t, w.alex, harness.Request{Method: http.MethodDelete, Path: "/threads/" + w.main + "/draft"}, http.StatusNoContent)
	w.mustCleanup()
	if !w.gone(f) {
		t.Fatal("F stays after its draft went")
	}
}

// TestCleanup_ExpiredSession (12 12 row "Session open and expired", 12 3.6): an open session past
// expires_at becomes expired, its file failed with one file.updated for Alex without actor, the
// part rows go, the purge removes the part objects, and the next uploadPart answers 410.
func TestCleanup_ExpiredSession(t *testing.T) {
	w := newWorld(t)
	content := []byte("an upload nobody completes")
	f, _ := w.open(w.alex, "open.txt", content, content)
	w.clk.Advance(sessionTTL)
	w.mustCleanup()
	var state string
	if err := w.db.QueryRow(t.Context(), `SELECT state FROM file_upload_sessions WHERE file_id = $1`, f).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if up, _ := w.fileState(f); state != "expired" || up != "failed" {
		t.Fatalf("session %s, file %s", state, up)
	}
	if n := w.count(`SELECT count(*) FROM file_upload_parts WHERE file_id = $1`, f); n != 0 {
		t.Errorf("%d part rows", n)
	}
	if n := w.count(`SELECT count(*) FROM event_log WHERE resource_kind = 'file' AND resource_id = $1 AND type = 'file.updated'
		AND audience_principal_id = $2 AND actor_kind IS NULL`, f, w.alex.PrincipalID); n != 1 {
		t.Errorf("%d file.updated for Alex without actor", n)
	}
	w.purgeQueued(f)
	if left := w.objectsLeft(f); len(left) != 0 {
		t.Errorf("objects left %v", left)
	}
	if r := w.part(w.alex, f, 1, content); r.Status != http.StatusGone {
		t.Errorf("uploadPart %d %s", r.Status, r.Body)
	}
}

// TestCleanup_FailedAndBlockedRows (12 12 row "Failed or blocked"): a checksum failure and an
// EICAR file lose their objects through the purge queued with the transition; the rows stay for
// Alex until file.failed_row_ttl and then go; an infected file attached to a message keeps its
// row.
func TestCleanup_FailedAndBlockedRows(t *testing.T) {
	w := newWorld(t)
	content := []byte("the declared bytes of a broken upload")
	declared := []byte("the DECLARED bytes of a broken upload")
	failed := w.upload(w.alex, "broken.txt", content, declared)
	w.mustScan(failed)
	w.clamd.SetScript(clamdtest.Script{Reply: eicarReply})
	infected := w.clean2(w.alex, "eicar.txt")
	attached := w.clean2(w.alex, "eicar2.txt")
	w.clamd.SetScript(clamdtest.Script{})
	w.mustSend(w.alex, "blocked attachment", attached)
	for _, f := range []string{failed, infected} {
		w.purgeQueued(f)
		if w.original(f) || !w.rowExists(f) {
			t.Fatalf("file %s: object %v, row %v", f, w.original(f), w.rowExists(f))
		}
		if st := w.getFile(w.alex, f); st != http.StatusOK {
			t.Errorf("getFile %s: %d", f, st)
		}
	}
	w.clk.Advance(failedRowTTL)
	w.mustCleanup()
	for _, f := range []string{failed, infected} {
		if w.rowExists(f) {
			t.Errorf("row %s stays after file.failed_row_ttl", f)
		}
	}
	if !w.rowExists(attached) || w.deleted(attached) {
		t.Error("the attached infected file lost its row")
	}
}

// clean2 uploads a text file as c and scans it with the current script of clamd.
func (w *world) clean2(c *harness.Client, name string) string {
	w.t.Helper()
	content := []byte("text of " + name + " for the support team")
	id := w.upload(c, name, content, content)
	w.mustScan(id)
	return id
}

// TestDeleteFile (05 4.8, 06 7, AZ-P4): Alex deletes his unattached file with 204, again with 204
// before the purge and 404 after it; Sam, Robin and the Owner get 404; an attached file answers
// 409 file_attached; a running upload answers 204, its session is aborted and the next part 404.
func TestDeleteFile(t *testing.T) {
	w := newWorld(t)
	f := w.clean(w.alex, "mine.txt")
	for _, c := range []*harness.Client{w.sam, w.robin, w.owner} {
		wantProblem(t, w.deleteFile(c, f), http.StatusNotFound, "not_found", "")
	}
	for range 2 {
		if r := w.deleteFile(w.alex, f); r.Status != http.StatusNoContent {
			t.Fatalf("deleteFile %d %s", r.Status, r.Body)
		}
	}
	for _, c := range []*harness.Client{w.sam, w.robin, w.owner} {
		wantProblem(t, w.deleteFile(c, f), http.StatusNotFound, "not_found", "")
	}
	w.purgeQueued(f)
	if w.rowExists(f) || len(w.objectsLeft(f)) != 0 {
		t.Fatal("row or objects stay")
	}
	wantProblem(t, w.deleteFile(w.alex, f), http.StatusNotFound, "not_found", "")
	a := w.clean(w.alex, "attached.txt")
	w.mustSend(w.alex, "attached", a)
	wantProblem(t, w.deleteFile(w.alex, a), http.StatusConflict, "conflict", "file_attached")
	if w.deleted(a) {
		t.Fatal("the attached file was deleted")
	}
	content := []byte("an upload that is still running")
	run, _ := w.open(w.alex, "running.txt", content, content)
	if r := w.deleteFile(w.alex, run); r.Status != http.StatusNoContent {
		t.Fatalf("deleteFile of a running upload %d %s", r.Status, r.Body)
	}
	var state string
	if err := w.db.QueryRow(t.Context(), `SELECT state FROM file_upload_sessions WHERE file_id = $1`, run).Scan(&state); err != nil || state != "aborted" {
		t.Fatalf("session %s %v", state, err)
	}
	if r := w.part(w.alex, run, 1, content); r.Status != http.StatusNotFound {
		t.Errorf("uploadPart after deleteFile: %d", r.Status)
	}
	w.purgeQueued(run)
	if left := w.objectsLeft(run); len(left) != 0 {
		t.Errorf("parts left %v", left)
	}
}

// TestDeleteFile_ConcurrentWithAttach: a send with F and deleteFile of F at the same time end
// with F attached or deleted, never both.
func TestDeleteFile_ConcurrentWithAttach(t *testing.T) {
	w := newWorld(t)
	for range 3 {
		f := w.clean(w.alex, "race.txt")
		var send, del harness.Response
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); send = w.send(w.alex, "race", f) }()
		go func() { defer wg.Done(); del = w.deleteFile(w.alex, f) }()
		wg.Wait()
		attached := w.count(`SELECT count(*) FROM message_attachments WHERE file_id = $1`, f) == 1
		switch {
		case send.Status == http.StatusCreated && del.Status == http.StatusConflict:
			if !attached || w.gone(f) {
				t.Fatalf("send won but attached %v gone %v", attached, w.gone(f))
			}
		case send.Status == http.StatusConflict && del.Status == http.StatusNoContent:
			if attached || !w.gone(f) {
				t.Fatalf("delete won but attached %v gone %v", attached, w.gone(f))
			}
		default:
			t.Fatalf("send %d %s, delete %d %s", send.Status, send.Body, del.Status, del.Body)
		}
	}
}

// TestCleanup_ConcurrentWithAttach: a send with F while the cleanup handles F after its TTL ends
// with F attached or deleted, never both.
func TestCleanup_ConcurrentWithAttach(t *testing.T) {
	w := newWorld(t)
	f := w.clean(w.alex, "race.txt")
	w.clk.Advance(unattachedTTL)
	var send harness.Response
	var cerr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); send = w.send(w.alex, "race", f) }()
	go func() { defer wg.Done(); cerr = w.runCleanup() }()
	wg.Wait()
	if cerr != nil {
		t.Fatal(cerr)
	}
	attached := w.count(`SELECT count(*) FROM message_attachments WHERE file_id = $1`, f) == 1
	if attached == w.gone(f) || (send.Status == http.StatusCreated) != attached {
		t.Fatalf("send %d, attached %v, gone %v", send.Status, attached, w.gone(f))
	}
}

// TestDPT01_FilePart (DP-T01, files): the file name and the message text carry a canary; after
// deleteMessage and the purge no event payload holds it, the file.updated events are listed as
// redacted with a valid payload, and no row or object of the file is left.
func TestDPT01_FilePart(t *testing.T) {
	w := newWorld(t)
	canary := "canary" + strings.ReplaceAll(w.alex.PrincipalID, "-", "")[:12]
	f := w.clean(w.alex, canary+".txt")
	msg := w.mustSend(w.alex, "text "+canary, f)
	harness.MustDo(t, w.alex, harness.Request{Method: http.MethodDelete, Path: "/messages/" + msg}, http.StatusNoContent)
	w.purgeQueued(f)
	if n := w.count(`SELECT count(*) FROM event_log WHERE payload::text LIKE '%' || $1 || '%'`, canary); n != 0 {
		t.Fatalf("%d event payloads hold the canary", n)
	}
	var redacted, total int
	if err := w.db.QueryRow(t.Context(), `SELECT count(*) FILTER (WHERE redacted_at IS NOT NULL), count(*) FROM event_log
		WHERE resource_kind = 'file' AND resource_id = $1 AND type = 'file.updated'`, f).Scan(&redacted, &total); err != nil {
		t.Fatal(err)
	}
	if total == 0 || redacted != total {
		t.Fatalf("file.updated %d of %d redacted", redacted, total)
	}
	var name string
	if err := w.db.QueryRow(t.Context(), `SELECT payload->'file'->>'original_name' FROM event_log
		WHERE resource_kind = 'file' AND resource_id = $1 AND type = 'file.updated' LIMIT 1`, f).Scan(&name); err != nil || name != "" {
		t.Fatalf("redacted name %q %v", name, err)
	}
	if w.rowExists(f) || len(w.objectsLeft(f)) != 0 {
		t.Fatal("row or object left")
	}
}

// TestNoDeduplication (R-DM-13, 20 4.3): Alex and the Owner upload identical bytes; deleting
// Alex's file leaves the Owner's row and object unchanged.
func TestNoDeduplication(t *testing.T) {
	w := newWorld(t)
	a := w.clean(w.alex, "same.txt")
	o := w.clean(w.owner, "same.txt")
	harness.MustDo(t, w.alex, harness.Request{Method: http.MethodDelete, Path: "/files/" + a}, http.StatusNoContent)
	w.purgeQueued(a)
	if !w.rowExists(o) || w.deleted(o) || !w.original(o) {
		t.Fatal("the Owner's file changed")
	}
}

// TestCleanup_TempSweep (12 4.2): a temporary file in <root>/.tmp older than
// file.upload_session_ttl is removed by the run, a fresh one stays.
func TestCleanup_TempSweep(t *testing.T) {
	w := newWorld(t)
	stale := filepath.Join(w.s.FilesRoot, ".tmp", "stale-test")
	fresh := filepath.Join(w.s.FilesRoot, ".tmp", "fresh-test")
	for _, p := range []string{stale, fresh} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	w.clk.Advance(sessionTTL + time.Hour)
	future := w.clk.Now().Add(time.Hour)
	if err := os.Chtimes(fresh, future, future); err != nil {
		t.Fatal(err)
	}
	w.mustCleanup()
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale temporary file stays: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("fresh temporary file went: %v", err)
	}
}

// TestPurge_Idempotent: two purge jobs of one file and a catch-up run in parallel delete the row
// once, end without error and leave no object.
func TestPurge_Idempotent(t *testing.T) {
	w := newWorld(t)
	f := w.clean(w.alex, "twice.txt")
	harness.MustDo(t, w.alex, harness.Request{Method: http.MethodDelete, Path: "/files/" + f}, http.StatusNoContent)
	errs := make(chan error, 3)
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() { errs <- w.purgeFile(f) })
	}
	wg.Go(func() { errs <- w.runCleanup() })
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if w.rowExists(f) || len(w.objectsLeft(f)) != 0 {
		t.Fatalf("row %v, objects %v", w.rowExists(f), w.objectsLeft(f))
	}
}

// TestPurge_WhileScanRuns: deleteFile while the scan pass of the file waits for clamd; after both
// no object and no row exist, and the scan job ends with nil.
func TestPurge_WhileScanRuns(t *testing.T) {
	w := newWorld(t)
	content := []byte("scanned while it is deleted")
	f := w.upload(w.alex, "scan.txt", content, content)
	started := make(chan struct{})
	w.clamd.SetScript(clamdtest.Script{Delay: 200 * time.Millisecond})
	scanErr := make(chan error, 1)
	go func() {
		close(started)
		w.mu.Lock()
		pass := w.scan
		w.mu.Unlock()
		_, err := pass.Run(context.Background(), ids.MustParse(f), 0)
		scanErr <- err
	}()
	<-started
	if r := w.deleteFile(w.alex, f); r.Status != http.StatusNoContent {
		t.Fatalf("deleteFile %d %s", r.Status, r.Body)
	}
	w.mustPurge(f)
	if err := <-scanErr; err != nil {
		t.Fatalf("scan pass: %v", err)
	}
	w.mustPurge(f)
	if w.rowExists(f) || len(w.objectsLeft(f)) != 0 {
		t.Fatalf("row %v, objects %v", w.rowExists(f), w.objectsLeft(f))
	}
}

// TestAnonymousFileByID_Unauthenticated (07 4.2 step 1, AZ-P4): a request without credentials on
// getFile and deleteFile answers 401 unauthenticated before any file is read, alike for a live, a
// deleted, a purged and an unknown id, so it learns nothing about the id.
func TestAnonymousFileByID_Unauthenticated(t *testing.T) {
	w := newWorld(t)
	live := w.clean(w.alex, "live.txt")
	deleted := w.clean(w.alex, "deleted.txt")
	purged := w.clean(w.alex, "purged.txt")
	for _, f := range []string{deleted, purged} {
		if r := w.deleteFile(w.alex, f); r.Status != http.StatusNoContent {
			t.Fatalf("deleteFile %d %s", r.Status, r.Body)
		}
	}
	w.purgeQueued(purged)
	if w.rowExists(purged) {
		t.Fatal("the purged row stays")
	}
	for _, f := range []string{live, deleted, purged, ids.New().String()} {
		for _, method := range []string{http.MethodGet, http.MethodDelete} {
			r := w.alex.DoAnonymous(t.Context(), harness.Request{Method: method, Path: "/files/" + f})
			wantProblem(t, r, http.StatusUnauthorized, "unauthenticated", "")
		}
	}
	if !w.rowExists(live) || w.deleted(live) {
		t.Fatal("an anonymous deleteFile changed the live file")
	}
}
