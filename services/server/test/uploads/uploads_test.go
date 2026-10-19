// SPDX-License-Identifier: Apache-2.0

// Package uploads_test holds the integration tests of the upload sessions of brief M3-03: the
// real server of internal/wire through test/harness with the matrix world (07 10.1), PostgreSQL
// 18 from pgtest, the file system driver rooted in a temporary directory, and every response
// validated against the contract when built with the tag contract_validate (make test).
package uploads_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestUploads runs the cases of the shared server: one harness server with the matrix world,
// parts of four bytes.
func TestUploads(t *testing.T) {
	s := harness.Start(t, harness.Options{Config: baseConfig(nil)})
	w := harness.BuildMatrixWorld(t, s)
	alex, sam, robin, owner := w.People["Alex"], w.People["Sam"], w.People["Robin"], w.Owner
	ch := w.Channels["Support/Allgemein"]
	db := s.AdminDB()
	ctx := context.Background()

	t.Run("CreateUpload_Session", func(t *testing.T) {
		before := time.Now().UTC()
		r := create(t, alex, ch, "report.txt", content, map[string]any{"declared_mime_type": "Text/Plain; charset=utf-8"})
		if r.Status != http.StatusCreated {
			t.Fatalf("%d %s", r.Status, r.Body)
		}
		var sess session
		r.JSON(t, &sess)
		if sess.Mode != "direct" || sess.PartSizeBytes != partBytes || len(sess.Parts) != 3 {
			t.Fatalf("session %+v", sess)
		}
		for i, p := range sess.Parts {
			if p.PartNumber != i+1 || p.URL != "/api/v1/uploads/"+sess.FileID+"/parts/"+itoa(i+1) ||
				len(p.Headers) != 1 || p.Headers[0].Name != "Content-Type" || p.Headers[0].Value != "application/octet-stream" {
				t.Errorf("part %d: %+v", i+1, p)
			}
		}
		exp, err := time.Parse(time.RFC3339Nano, sess.ExpiresAt)
		if err != nil || exp.Before(before.Add(24*time.Hour-time.Minute)) || exp.After(time.Now().Add(24*time.Hour+time.Minute)) {
			t.Errorf("expires_at %s, want about 24h (file.upload_session_ttl) from now", sess.ExpiresAt)
		}
		var key, mime, declared, purpose, upState, scan, name, uploader, scope, mode, state string
		var size, partSize int64
		var count int
		if err := db.QueryRow(ctx, `SELECT f.object_key, f.mime_type, f.declared_mime_type, f.purpose, f.upload_state, f.scan_status,
			f.original_name, f.uploaded_by::text, f.scope_channel_id::text, f.size_bytes, s.mode, s.state, s.part_size_bytes, s.part_count
			FROM files f JOIN file_upload_sessions s ON s.file_id = f.id WHERE f.id = $1`, sess.FileID).
			Scan(&key, &mime, &declared, &purpose, &upState, &scan, &name, &uploader, &scope, &size, &mode, &state, &partSize, &count); err != nil {
			t.Fatal(err)
		}
		if key != "f/"+sess.FileID[32:34]+"/"+sess.FileID[34:36]+"/"+sess.FileID+"/orig-1" || strings.Contains(key, "report") ||
			strings.Contains(key, ch) || strings.Contains(key, w.Spaces["Support"]) {
			t.Errorf("object_key %s", key)
		}
		if mime != "application/octet-stream" || declared != "text/plain" || purpose != "attachment" || upState != "pending" || scan != "pending" ||
			name != "report.txt" || uploader != alex.PrincipalID || scope != ch || size != 10 || mode != "direct" || state != "open" ||
			partSize != partBytes || count != 3 {
			t.Errorf("rows: %s %s %s %s %s %s %s %s %d %s %s %d %d", mime, declared, purpose, upState, scan, name, uploader, scope, size, mode, state, partSize, count)
		}
		var events int
		if err := db.QueryRow(ctx, `SELECT count(*) FROM event_log WHERE resource_id = $1`, sess.FileID).Scan(&events); err != nil || events != 0 {
			t.Errorf("%d events after create (%v), want none", events, err)
		}
	})

	t.Run("CreateUpload_Rights", func(t *testing.T) {
		wantProblem(t, create(t, robin, ch, "a.txt", content, nil), http.StatusNotFound, "not_found", "")
		// Sam reads Allgemein through Leser; a grant with write but without upload on Support.
		harness.MustDo(t, owner, harness.Request{Method: http.MethodPost, Path: "/grants", Body: map[string]any{
			"subject_kind": "user", "subject_id": sam.PrincipalID, "target_kind": "space", "target_id": w.Spaces["Support"],
			"level": "write", "reason": "write without upload"}}, http.StatusCreated)
		wantProblem(t, create(t, sam, ch, "a.txt", content, nil), http.StatusForbidden, "forbidden", "capability_missing")
		var archived struct {
			ID string `json:"id"`
		}
		harness.MustDo(t, owner, harness.Request{Method: http.MethodPost, Path: "/spaces/" + w.Spaces["Support"] + "/channels",
			Body: map[string]any{"kind": "text", "name": "Archiv"}}, http.StatusCreated).JSON(t, &archived)
		harness.MustDo(t, owner, harness.Request{Method: http.MethodPost, Path: "/channels/" + archived.ID + "/archive",
			Body: map[string]any{"reason": "test"}}, http.StatusOK)
		wantProblem(t, create(t, alex, archived.ID, "a.txt", content, nil), http.StatusForbidden, "channel_read_only", "channel_archived")
	})

	t.Run("CreateUpload_IdempotencyKey", func(t *testing.T) {
		h := http.Header{}
		h.Set("Idempotency-Key", ids.New().String())
		body := map[string]any{"name": "same.txt", "size_bytes": len(content), "sha256": sha(content)}
		first := alex.Do(t.Context(), harness.Request{Method: http.MethodPost, Path: "/channels/" + ch + "/uploads", Body: body, Header: h})
		again := alex.Do(t.Context(), harness.Request{Method: http.MethodPost, Path: "/channels/" + ch + "/uploads", Body: body, Header: h})
		var a, b session
		first.JSON(t, &a)
		again.JSON(t, &b)
		if first.Status != http.StatusCreated || again.Status != http.StatusCreated || again.Header.Get("Idempotent-Replay") != "true" ||
			a.FileID != b.FileID || a.ExpiresAt != b.ExpiresAt || len(a.Parts) != len(b.Parts) {
			t.Fatalf("first %d, repeat %d %q %s", first.Status, again.Status, again.Header.Get("Idempotent-Replay"), again.Body)
		}
		var n int
		if err := db.QueryRow(ctx, `SELECT count(*) FROM files WHERE original_name = 'same.txt'`).Scan(&n); err != nil || n != 1 {
			t.Fatalf("%d files (%v), want 1", n, err)
		}
	})

	t.Run("CreateUpload_NameCleaned", func(t *testing.T) {
		sess := mustCreate(t, alex, ch, "../../in\u202Evoice\x01\u200B.txt", content)
		var name string
		if err := db.QueryRow(ctx, `SELECT original_name FROM files WHERE id = $1`, sess.FileID).Scan(&name); err != nil || name != "invoice.txt" {
			t.Fatalf("original_name %q (%v)", name, err)
		}
		var f struct {
			OriginalName string `json:"original_name"`
		}
		harness.MustDo(t, alex, harness.Request{Method: http.MethodGet, Path: "/files/" + sess.FileID}, http.StatusOK).JSON(t, &f)
		if f.OriginalName != "invoice.txt" {
			t.Fatalf("getFile original_name %q", f.OriginalName)
		}
	})

	t.Run("UploadPart_SameDigest200", func(t *testing.T) {
		sess := mustCreate(t, alex, ch, "a.txt", content)
		first := mustPut(t, alex, sess.FileID, 1, content)
		path := filepath.Join(s.FilesRoot, "uploads", sess.FileID, "1")
		st, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if again := mustPut(t, alex, sess.FileID, 1, content); again != first {
			t.Fatalf("etag %s then %s", first, again)
		}
		if st2, err := os.Stat(path); err != nil || !st2.ModTime().Equal(st.ModTime()) || !os.SameFile(st, st2) {
			t.Fatal("the confirmed part was written again")
		}
		var n int
		if err := db.QueryRow(ctx, `SELECT count(*) FROM file_upload_parts WHERE file_id = $1`, sess.FileID).Scan(&n); err != nil || n != 1 {
			t.Fatalf("%d part rows (%v)", n, err)
		}
	})

	t.Run("UploadPart_OtherDigest409", func(t *testing.T) {
		sess := mustCreate(t, alex, ch, "a.txt", content)
		mustPut(t, alex, sess.FileID, 1, content)
		wantProblem(t, put(t, alex, sess.FileID, 1, []byte("abcd"), digest([]byte("abcd"))), http.StatusConflict, "conflict", "part_conflict")
		got, err := os.ReadFile(filepath.Join(s.FilesRoot, "uploads", sess.FileID, "1"))
		if err != nil || !bytes.Equal(got, part(content, 1)) {
			t.Fatalf("stored part %q (%v)", got, err)
		}
	})

	t.Run("UploadPart_WrongLengthRejected", func(t *testing.T) {
		sess := mustCreate(t, alex, ch, "a.txt", content)
		for name, c := range map[string]struct {
			n    int
			body []byte
		}{
			"middle part short": {1, []byte("012")}, "middle part long": {2, []byte("45678")},
			"last part short": {3, []byte("8")}, "last part long": {3, []byte("890")},
		} {
			r := put(t, alex, sess.FileID, c.n, c.body, digest(c.body))
			if r.Status != http.StatusBadRequest {
				t.Fatalf("%s: %d %s", name, r.Status, r.Body)
			}
			wantField(t, r, "", "part_digest_mismatch")
		}
		assertNoParts(t, s, db, sess.FileID)
	})

	t.Run("UploadPart_WrongDigestRejected", func(t *testing.T) {
		sess := mustCreate(t, alex, ch, "a.txt", content)
		wantField(t, put(t, alex, sess.FileID, 1, part(content, 1), digest([]byte("wxyz"))), "#header/Content-Digest", "part_digest_mismatch")
		assertNoParts(t, s, db, sess.FileID)
	})

	t.Run("UploadPart_HeaderAndNumber", func(t *testing.T) {
		sess := mustCreate(t, alex, ch, "a.txt", content)
		wantField(t, put(t, alex, sess.FileID, 1, part(content, 1), ""), "#header/Content-Digest", "required")
		wantField(t, put(t, alex, sess.FileID, 1, part(content, 1), "sha-256=:AAAA:"), "#header/Content-Digest", "invalid")
		wantField(t, put(t, alex, sess.FileID, 1, part(content, 1), "md5=:AAAA:"), "#header/Content-Digest", "invalid")
		wantField(t, put(t, alex, sess.FileID, 4, []byte("x"), digest([]byte("x"))), "#path/part_number", "out_of_range")
	})

	t.Run("UploadPart_ForeignAndUnknown404", func(t *testing.T) {
		sess := mustCreate(t, alex, ch, "a.txt", content)
		foreign := put(t, sam, sess.FileID, 1, part(content, 1), digest(part(content, 1)))
		unknown := put(t, sam, ids.New().String(), 1, part(content, 1), digest(part(content, 1)))
		sameNotFound(t, foreign, unknown)
	})

	t.Run("UploadPart_ParallelAnyOrder", func(t *testing.T) {
		big := []byte(strings.Repeat("abcdefgh", 8))
		sess := mustCreate(t, alex, ch, "big.bin", big)
		var wg sync.WaitGroup
		errs := make(chan string, len(sess.Parts))
		for n := len(sess.Parts); n >= 1; n-- {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				if r := put(t, alex, sess.FileID, n, part(big, n), digest(part(big, n))); r.Status != http.StatusOK {
					errs <- string(r.Body)
				}
			}(n)
		}
		wg.Wait()
		close(errs)
		for e := range errs {
			t.Error(e)
		}
		st := state(t, alex, sess.FileID)
		if st.UploadState != "uploading" || len(st.CompletedParts) != len(sess.Parts) {
			t.Fatalf("state %+v", st)
		}
	})

	t.Run("GetUpload_Resume", func(t *testing.T) {
		sess := mustCreate(t, alex, ch, "a.txt", content)
		if st := state(t, alex, sess.FileID); st.UploadState != "pending" || len(st.CompletedParts) != 0 || st.PartSizeBytes != partBytes {
			t.Fatalf("fresh %+v", st)
		}
		e3 := mustPut(t, alex, sess.FileID, 3, content)
		e1 := mustPut(t, alex, sess.FileID, 1, content)
		st := state(t, alex, sess.FileID)
		if st.UploadState != "uploading" || len(st.CompletedParts) != 2 || st.CompletedParts[0].PartNumber != 1 ||
			st.CompletedParts[0].ETag != e1 || st.CompletedParts[1].PartNumber != 3 || st.CompletedParts[1].ETag != e3 {
			t.Fatalf("resume %+v", st)
		}
		e2 := mustPut(t, alex, sess.FileID, 2, content)
		harness.MustDo(t, alex, harness.Request{Method: http.MethodPost, Path: "/uploads/" + sess.FileID + "/complete",
			Body: map[string]any{"parts": []map[string]any{{"part_number": 2, "etag": e2}}}}, http.StatusOK)
		if st := state(t, alex, sess.FileID); st.UploadState != "complete" || len(st.CompletedParts) != 3 {
			t.Fatalf("complete %+v", st)
		}
		sameNotFound(t, get(t, sam, "/uploads/"+sess.FileID), get(t, sam, "/uploads/"+ids.New().String()))
		other := mustCreate(t, alex, ch, "b.txt", content)
		if r := abort(t, alex, other.FileID); r.Status != http.StatusNoContent {
			t.Fatalf("abort %d %s", r.Status, r.Body)
		}
		wantProblem(t, get(t, alex, "/uploads/"+other.FileID), http.StatusGone, "gone", "")
	})

	t.Run("CompleteUpload_MissingParts409", func(t *testing.T) {
		big := []byte(strings.Repeat("x", 36))
		sess := mustCreate(t, alex, ch, "nine.bin", big)
		for _, n := range []int{1, 2, 3, 7, 8} {
			mustPut(t, alex, sess.FileID, n, big)
		}
		p := wantProblem(t, complete(t, alex, sess.FileID, nil), http.StatusConflict, "conflict", "upload_incomplete")
		if p.Detail != "4-6,9" {
			t.Fatalf("detail %q, want 4-6,9", p.Detail)
		}
		if st := state(t, alex, sess.FileID); st.UploadState != "uploading" {
			t.Fatalf("state %+v, want the session open", st)
		}
	})

	t.Run("CompleteUpload_Completes", func(t *testing.T) {
		sess, parts := uploadAll(t, alex, ch, "done.txt", content)
		r := complete(t, alex, sess.FileID, parts)
		var f map[string]any
		if r.Status != http.StatusOK || json.Unmarshal(r.Body, &f) != nil || f["upload_state"] != "pending" || f["scan_status"] != "pending" {
			t.Fatalf("%d %s", r.Status, r.Body)
		}
		var state string
		var completed *time.Time
		if err := db.QueryRow(ctx, `SELECT state, completed_at FROM file_upload_sessions WHERE file_id = $1`, sess.FileID).Scan(&state, &completed); err != nil ||
			state != "completed" || completed == nil {
			t.Fatalf("session %s %v (%v)", state, completed, err)
		}
		again := complete(t, alex, sess.FileID, nil)
		if again.Status != http.StatusOK {
			t.Fatalf("second complete %d %s", again.Status, again.Body)
		}
		var events, jobs int
		if err := db.QueryRow(ctx, `SELECT count(*) FROM event_log WHERE resource_id = $1`, sess.FileID).Scan(&events); err != nil || events != 0 {
			t.Errorf("%d events (%v), want none", events, err)
		}
		if err := db.QueryRow(ctx, `SELECT count(*) FROM jobs.river_job WHERE kind LIKE 'files.%'`).Scan(&jobs); err != nil || jobs != 0 {
			t.Errorf("%d file jobs (%v), want none", jobs, err)
		}
	})

	t.Run("CompleteUpload_PartsCrossCheck", func(t *testing.T) {
		sess, parts := uploadAll(t, alex, ch, "x.txt", content)
		wantField(t, complete(t, alex, sess.FileID, []map[string]any{parts[0], parts[0]}), "/parts", "duplicate")
		wantField(t, complete(t, alex, sess.FileID, []map[string]any{{"part_number": 4, "etag": `"x"`}}), "/parts", "out_of_range")
		wantProblem(t, complete(t, alex, sess.FileID, []map[string]any{{"part_number": 1, "etag": `"wrong"`}}), http.StatusConflict, "conflict", "part_conflict")
	})

	t.Run("AbortUpload_DeletesParts", func(t *testing.T) {
		before := maxSeq(t, db)
		sess := mustCreate(t, alex, ch, "gone.txt", content)
		mustPut(t, alex, sess.FileID, 1, content)
		mustPut(t, alex, sess.FileID, 3, content)
		if r := abort(t, alex, sess.FileID); r.Status != http.StatusNoContent {
			t.Fatalf("%d %s", r.Status, r.Body)
		}
		assertNoParts(t, s, db, sess.FileID)
		var state, upState string
		if err := db.QueryRow(ctx, `SELECT s.state, f.upload_state FROM file_upload_sessions s JOIN files f ON f.id = s.file_id WHERE f.id = $1`,
			sess.FileID).Scan(&state, &upState); err != nil || state != "aborted" || upState != "failed" {
			t.Fatalf("%s %s (%v)", state, upState, err)
		}
		var n int
		var audience, action, space, channel, kind string
		if err := db.QueryRow(ctx, `SELECT count(*) OVER (), audience_principal_id::text, receive_action, space_id::text, channel_id::text, resource_kind
			FROM event_log WHERE resource_id = $1 AND type = 'file.updated'`, sess.FileID).Scan(&n, &audience, &action, &space, &channel, &kind); err != nil ||
			n != 1 || audience != alex.PrincipalID || action != "file.read" || space != w.Spaces["Support"] || channel != ch || kind != "file" {
			t.Fatalf("event %d %s %s %s %s %s (%v)", n, audience, action, space, channel, kind, err)
		}
		// The payload validates against FileUpdatedEvent: listEvents answers it to Alex and the
		// response check of contract_validate holds the schema; Sam does not receive it.
		if !hasFileEvent(t, alex, before, sess.FileID) {
			t.Fatal("Alex does not receive file.updated")
		}
		if hasFileEvent(t, sam, before, sess.FileID) {
			t.Fatal("Sam receives the file.updated of Alex's unattached file")
		}
	})

	t.Run("AbortUpload_States", func(t *testing.T) {
		sess := mustCreate(t, alex, ch, "twice.txt", content)
		for range 2 {
			if r := abort(t, alex, sess.FileID); r.Status != http.StatusNoContent {
				t.Fatalf("%d %s", r.Status, r.Body)
			}
		}
		var n int
		if err := db.QueryRow(ctx, `SELECT count(*) FROM event_log WHERE resource_id = $1`, sess.FileID).Scan(&n); err != nil || n != 1 {
			t.Fatalf("%d events (%v), want 1", n, err)
		}
		done, parts := uploadAll(t, alex, ch, "done.txt", content)
		harness.MustDo(t, alex, harness.Request{Method: http.MethodPost, Path: "/uploads/" + done.FileID + "/complete",
			Body: map[string]any{"parts": parts}}, http.StatusOK)
		wantProblem(t, abort(t, alex, done.FileID), http.StatusConflict, "invalid_state_transition", "")
		sameNotFound(t, abort(t, sam, done.FileID), abort(t, sam, ids.New().String()))
		wantProblem(t, abort(t, robin, done.FileID), http.StatusNotFound, "not_found", "")
	})

	t.Run("GetFile_UploaderOnly", func(t *testing.T) {
		sess := mustCreate(t, alex, ch, "mine.txt", content)
		r := get(t, alex, "/files/"+sess.FileID)
		var f map[string]any
		if r.Status != http.StatusOK || json.Unmarshal(r.Body, &f) != nil {
			t.Fatalf("%d %s", r.Status, r.Body)
		}
		if f["scope_channel_id"] != ch || f["thumbnails"] == nil || len(f["thumbnails"].([]any)) != 0 || f["sha256"] != nil ||
			bytes.Contains(r.Body, []byte("null")) {
			t.Fatalf("file %s", r.Body)
		}
		for name, c := range map[string]*harness.Client{"Sam": sam, "Robin": robin, "owner": owner} {
			t.Run(name, func(t *testing.T) {
				sameNotFound(t, get(t, c, "/files/"+sess.FileID), get(t, c, "/files/"+ids.New().String()))
			})
		}
		if _, err := db.Exec(ctx, `UPDATE files SET deleted_at = now() WHERE id = $1`, sess.FileID); err != nil {
			t.Fatal(err)
		}
		wantProblem(t, get(t, alex, "/files/"+sess.FileID), http.StatusNotFound, "not_found", "")
	})
}

// uploadState is the answer of getUpload.
type uploadState struct {
	UploadState    string `json:"upload_state"`
	PartSizeBytes  int64  `json:"part_size_bytes"`
	CompletedParts []struct {
		PartNumber int    `json:"part_number"`
		ETag       string `json:"etag"`
	} `json:"completed_parts"`
}

// state reads getUpload with 200.
func state(t *testing.T, c *harness.Client, fileID string) uploadState {
	t.Helper()
	var st uploadState
	harness.MustDo(t, c, harness.Request{Method: http.MethodGet, Path: "/uploads/" + fileID}, http.StatusOK).JSON(t, &st)
	return st
}

// sameNotFound holds that a foreign and an unknown id answer the same 404: status, code and body
// apart from the correlation id and the instance.
func sameNotFound(t *testing.T, foreign, unknown harness.Response) {
	t.Helper()
	if foreign.Status != http.StatusNotFound || unknown.Status != http.StatusNotFound {
		t.Fatalf("foreign %d %s, unknown %d %s", foreign.Status, foreign.Body, unknown.Status, unknown.Body)
	}
	strip := func(b []byte) map[string]any {
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		delete(m, "correlation_id")
		delete(m, "instance")
		return m
	}
	fb, ub := strip(foreign.Body), strip(unknown.Body)
	if len(fb) == 0 || len(fb) != len(ub) {
		t.Fatalf("bodies differ: %s and %s", foreign.Body, unknown.Body)
	}
	for k, v := range fb {
		if ub[k] != v {
			t.Fatalf("member %s differs: %v and %v", k, v, ub[k])
		}
	}
	for _, h := range []string{"Content-Type", "Cache-Control", "Retry-After"} {
		if foreign.Header.Get(h) != unknown.Header.Get(h) {
			t.Fatalf("header %s differs", h)
		}
	}
}

// assertNoParts holds that an upload has no part row and no regular file under uploads/<id> of
// the root of the file system driver.
func assertNoParts(t *testing.T, s *harness.Server, db *pgxpool.Pool, fileID string) {
	t.Helper()
	var n int
	if err := db.QueryRow(context.Background(), `SELECT count(*) FROM file_upload_parts WHERE file_id = $1`, fileID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("%d part rows (%v), want none", n, err)
	}
	dir := filepath.Join(s.FilesRoot, "uploads", fileID)
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Type().IsRegular() {
			t.Fatalf("part file %s remains", e.Name())
		}
	}
}

// maxSeq is the head of the event log.
func maxSeq(t *testing.T, db *pgxpool.Pool) int64 {
	t.Helper()
	var n int64
	if err := db.QueryRow(context.Background(), `SELECT coalesce(max(seq), 0) FROM event_log`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// hasFileEvent reports whether c receives file.updated of fileID through listEvents after seq.
func hasFileEvent(t *testing.T, c *harness.Client, after int64, fileID string) bool {
	t.Helper()
	r := c.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/events?limit=500&after=" + itoa(int(after))})
	if r.Status != http.StatusOK {
		t.Fatalf("listEvents %d %s", r.Status, r.Body)
	}
	var page struct {
		Events []struct {
			Type     string `json:"type"`
			Resource struct {
				ID string `json:"id"`
			} `json:"resource"`
		} `json:"events"`
	}
	r.JSON(t, &page)
	for _, e := range page.Events {
		if e.Type == "file.updated" && e.Resource.ID == fileID {
			return true
		}
	}
	return false
}
