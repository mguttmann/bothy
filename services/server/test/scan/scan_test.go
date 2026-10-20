// SPDX-License-Identifier: Apache-2.0

// Package scan_test holds the integration tests of the scan pass of brief M3-04: the real server of
// internal/wire in role all through test/harness (River runs files.scan from the queue files),
// PostgreSQL 18 from pgtest, the file system driver in a temporary root, uploads through the real
// HTTP chain as Alex, and a throwaway clamd of the pinned image (clamdtest) behind a Unix socket.
package scan_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// safeBuffer collects the log lines of a server.
type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// nestedZip wraps payload in depth levels of ZIP archives.
func nestedZip(t *testing.T, payload []byte, depth int) []byte {
	t.Helper()
	b := payload
	for i := range depth {
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		f, err := w.Create(fmt.Sprintf("level-%d.bin", i))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(b); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		b = buf.Bytes()
	}
	return b
}

// gone reports that path does not exist, waiting up to waitScan: the pass deletes objects and
// parts after the commit of the result (design note 6f), so the row can be final a moment earlier.
func gone(t *testing.T, path string) bool {
	t.Helper()
	deadline := time.Now().Add(waitScan)
	for {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestScan runs the cases of the shared server with the shared clamd.
func TestScan(t *testing.T) {
	clamd := sharedClamd(t)
	logs := &safeBuffer{}
	s := harness.Start(t, harness.Options{Config: config(clamd.Address(), map[string]string{"file.denied_mime_types": "image/gif"}), LogWriter: logs})
	w := harness.BuildMatrixWorld(t, s)
	alex := w.People["Alex"]
	ch := w.Channels["Support/Allgemein"]
	db := s.AdminDB()
	ctx := context.Background()

	objectGone := func(t *testing.T, id string, r fileRow) {
		t.Helper()
		if !gone(t, filepath.Join(s.FilesRoot, r.ObjectKey)) || !gone(t, filepath.Join(s.FilesRoot, "uploads", id)) {
			t.Errorf("objects of %s remain", id)
		}
	}
	updates := func(id string) int {
		return count(t, db, `SELECT count(*) FROM event_log WHERE type = 'file.updated' AND resource_id = $1`, id)
	}
	audits := func(id string) int {
		return count(t, db, `SELECT count(*) FROM audit_events WHERE resource_id = $1 AND action = 'file.upload'`, id)
	}

	t.Run("EICARInfected", func(t *testing.T) {
		u := send(t, alex, ch, "canary-eicar-name.txt", "text/plain", clamdtest.EICAR(), clamdtest.EICAR())
		u.complete(t, alex, "")
		r := waitFinal(t, db, u.FileID)
		if r.UploadState != "complete" || r.ScanStatus != "infected" || strOf(r.FailureReason) != "malware_detected" || strOf(r.ScanEngine) != "clamd" {
			t.Fatalf("row %+v", r)
		}
		objectGone(t, u.FileID, r)
		var f struct {
			ScanStatus string `json:"scan_status"`
		}
		harness.MustDo(t, alex, harness.Request{Method: http.MethodGet, Path: "/files/" + u.FileID}, http.StatusOK).JSON(t, &f)
		if f.ScanStatus != "infected" {
			t.Errorf("getFile scan_status %q", f.ScanStatus)
		}
		var decision, reason, kind string
		var actor *string
		var details []byte
		if err := db.QueryRow(ctx, `SELECT decision, deny_reason, resource_kind, actor_principal_id::text, details FROM audit_events
			WHERE resource_id = $1 AND action = 'file.upload'`, u.FileID).Scan(&decision, &reason, &kind, &actor, &details); err != nil {
			t.Fatal(err)
		}
		var d map[string]any
		_ = json.Unmarshal(details, &d)
		sig, _ := d["signature"].(string)
		if decision != "deny" || reason != "malware_detected" || kind != "file" || actor != nil || len(d) != 3 ||
			!strings.HasPrefix(sig, "local.test.eicar") || d["sha256"] != sha(clamdtest.EICAR()) || d["size_bytes"] != float64(len(clamdtest.EICAR())) {
			t.Errorf("audit %s %s %s %v %s", decision, reason, kind, actor, details)
		}
		if n := audits(u.FileID); n != 1 {
			t.Errorf("%d audit rows", n)
		}
		if n := updates(u.FileID); n != 1 {
			t.Errorf("%d file.updated", n)
		}
	})

	t.Run("CleanFile", func(t *testing.T) {
		content := bytes.Repeat([]byte("a harmless report of Sam\n"), 2000)
		u := send(t, alex, ch, "report.txt", "", content, content)
		u.complete(t, alex, "")
		r := waitFinal(t, db, u.FileID)
		if r.UploadState != "complete" || r.ScanStatus != "clean" || r.MIMEType != "text/plain" || strOf(r.ScanEngine) != "clamd" ||
			r.ScannedAt == nil || r.FailureReason != nil {
			t.Fatalf("row %+v", r)
		}
		stored, err := os.ReadFile(filepath.Join(s.FilesRoot, r.ObjectKey)) // #nosec G304 -- a key of the test server
		if err != nil || sha(stored) != sha(content) || fmt.Sprintf("%x", r.SHA256) != sha(content) {
			t.Fatalf("stored object: %v", err)
		}
		if !gone(t, filepath.Join(s.FilesRoot, "uploads", u.FileID)) {
			t.Error("parts remain")
		}
		var payload []byte
		var actorKind *string
		if err := db.QueryRow(ctx, `SELECT payload, actor_kind FROM event_log WHERE type = 'file.updated' AND resource_id = $1`, u.FileID).
			Scan(&payload, &actorKind); err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(payload, []byte("null")) || actorKind != nil || !bytes.Contains(payload, []byte(`"scan_status": "clean"`)) && !bytes.Contains(payload, []byte(`"scan_status":"clean"`)) {
			t.Errorf("event %s actor %v", payload, actorKind)
		}
		if n := audits(u.FileID); n != 0 {
			t.Errorf("%d audit rows for a clean file", n)
		}
	})

	limits := func(t *testing.T, name string, content []byte) {
		t.Helper()
		u := send(t, alex, ch, name, "", content, content)
		u.complete(t, alex, "")
		r := waitFinal(t, db, u.FileID)
		if r.UploadState != "complete" || r.ScanStatus != "error" || strOf(r.FailureReason) != "scan_limits_exceeded" {
			t.Fatalf("row %+v", r)
		}
		objectGone(t, u.FileID, r)
		if n := audits(u.FileID); n != 0 {
			t.Errorf("%d audit rows", n)
		}
	}
	t.Run("ArchiveOverMaxRecursion", func(t *testing.T) {
		limits(t, "nested.zip", nestedZip(t, []byte("plain text"), testMaxRecursion+2))
	})
	t.Run("ArchiveOverMaxScanSize", func(t *testing.T) {
		limits(t, "big.zip", nestedZip(t, bytes.Repeat([]byte{0}, testMaxScanSize*4), 1))
	})

	t.Run("StreamLimitExceeded", func(t *testing.T) {
		content := bytes.Repeat([]byte("z"), testStreamMax+partBytes)
		u := send(t, alex, ch, "large.bin", "", content, content)
		u.complete(t, alex, "")
		r := waitFinal(t, db, u.FileID)
		if r.UploadState != "complete" || r.ScanStatus != "error" || strOf(r.FailureReason) != "scan_stream_limit_exceeded" {
			t.Fatalf("row %+v", r)
		}
		objectGone(t, u.FileID, r)
	})

	tamper := func(t *testing.T, content, stored []byte) (upload, fileRow) {
		t.Helper()
		u := send(t, alex, ch, "tampered.txt", "", content, content)
		if err := os.WriteFile(filepath.Join(s.FilesRoot, "uploads", u.FileID, "1"), stored, 0o600); err != nil {
			t.Fatal(err)
		}
		u.complete(t, alex, "")
		return u, waitFinal(t, db, u.FileID)
	}
	t.Run("ChecksumMismatch", func(t *testing.T) {
		content := []byte("0123456789")
		u, r := tamper(t, content, []byte("012345678X"))
		if r.UploadState != "failed" || r.ScanStatus != "pending" || strOf(r.FailureReason) != "checksum_mismatch" || r.ScanEngine != nil {
			t.Fatalf("row %+v", r)
		}
		objectGone(t, u.FileID, r)
		if n := updates(u.FileID); n != 1 {
			t.Errorf("%d file.updated", n)
		}
		// The same with EICAR bytes behind a wrong checksum: the detection is audited as well.
		eicar := clamdtest.EICAR()
		u, r = tamper(t, bytes.Repeat([]byte("e"), len(eicar)), eicar)
		if r.UploadState != "failed" || strOf(r.FailureReason) != "checksum_mismatch" || audits(u.FileID) != 1 {
			t.Fatalf("row %+v, audits %d", r, audits(u.FileID))
		}
	})

	t.Run("DetectedTypeDenied", func(t *testing.T) {
		u := send(t, alex, ch, "picture.txt", "text/plain", gifBytes, gifBytes)
		u.complete(t, alex, "")
		r := waitFinal(t, db, u.FileID)
		if r.UploadState != "failed" || strOf(r.FailureReason) != "file_type_denied" || r.MIMEType != "image/gif" {
			t.Fatalf("row %+v", r)
		}
		objectGone(t, u.FileID, r)
	})

	t.Run("DeclaredTypeIgnored", func(t *testing.T) {
		u := send(t, alex, ch, "picture.txt", "text/plain", pngBytes, pngBytes)
		u.complete(t, alex, "")
		r := waitFinal(t, db, u.FileID)
		// An image stays pending until files.thumbnail stored its cleaned original (M3-06 design
		// note 1c); the job then makes it complete, possibly before this poll saw the row.
		if r.UploadState == "failed" || r.ScanStatus != "clean" || r.MIMEType != "image/png" || thumbnailJobs(t, db, u.FileID) != 1 {
			t.Fatalf("row %+v", r)
		}
		if r := waitComplete(t, db, u.FileID); r.UploadState != "complete" || r.MIMEType != "image/png" {
			t.Fatalf("after files.thumbnail: %+v", r)
		}
	})

	t.Run("CompleteUploadInsertsScanJob", func(t *testing.T) {
		content := []byte("one job per completion")
		u := send(t, alex, ch, "job.txt", "", content, content)
		key := ids.New().String()
		u.complete(t, alex, key)
		again := u.complete(t, alex, key)
		if again.Header.Get("Idempotent-Replay") != "true" {
			t.Errorf("no replay: %v", again.Header)
		}
		waitFinal(t, db, u.FileID)
		// Round 0 comes from the completion only; a retry under load would add round 1 and later.
		if n := count(t, db, `SELECT count(*) FROM jobs.river_job WHERE kind = 'files.scan' AND args->>'file_id' = $1
			AND (args->>'round')::bigint = 0`, u.FileID); n != 1 {
			t.Fatalf("%d files.scan jobs of round 0, want 1", n)
		}
		if n := count(t, db, `SELECT count(*) FROM jobs.river_job WHERE kind = 'files.scan' AND args->>'file_id' = $1 AND queue <> 'files'`,
			u.FileID); n != 0 {
			t.Fatalf("%d files.scan jobs outside the queue files", n)
		}
	})

	t.Run("NoNamesOrSignaturesOutsideAudit", func(t *testing.T) {
		out := logs.String()
		for _, canary := range []string{"canary-eicar-name", "Local.Test.EICAR", "local.test.eicar", "FOUND"} {
			if strings.Contains(out, canary) {
				t.Errorf("the logs carry %q", canary)
			}
		}
	})
}

// TestScanClamdStoppedStaysPending (12 15 "clamd stopped", ADR 0033): while clamd is stopped the
// file stays pending without event and the job keeps inserting rounds with growing delay up to
// file.scan.retry_backoff_cap; after the start the due round sets clean.
func TestScanClamdStoppedStaysPending(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Minute)
	defer cancel()
	clamd, err := clamdtest.Start(ctx, clamdtest.Options{Restartable: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = clamd.Terminate(context.Background()) })
	s := harness.Start(t, harness.Options{Config: config(clamd.Address(), nil)})
	w := harness.BuildMatrixWorld(t, s)
	alex := w.People["Alex"]
	db := s.AdminDB()
	if err := clamd.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	content := []byte("waiting for the scanner")
	u := send(t, alex, w.Channels["Support/Allgemein"], "wait.txt", "", content, content)
	u.complete(t, alex, "")
	deadline := time.Now().Add(waitScan)
	for count(t, db, `SELECT count(*) FROM jobs.river_job WHERE kind = 'files.scan' AND args->>'file_id' = $1`, u.FileID) < 5 {
		if time.Now().After(deadline) {
			t.Fatal("fewer than five rounds while clamd is stopped")
		}
		time.Sleep(200 * time.Millisecond)
	}
	rows, err := db.Query(ctx, `SELECT (args->>'round')::bigint, scheduled_at, created_at FROM jobs.river_job
		WHERE kind = 'files.scan' AND args->>'file_id' = $1 ORDER BY (args->>'round')::bigint`, u.FileID)
	if err != nil {
		t.Fatal(err)
	}
	type job struct {
		round              int64
		scheduled, created time.Time
	}
	var jobs []job
	for rows.Next() {
		var j job
		if err := rows.Scan(&j.round, &j.scheduled, &j.created); err != nil {
			t.Fatal(err)
		}
		jobs = append(jobs, j)
	}
	rows.Close()
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 4 * time.Second}
	for i, d := range want {
		j := jobs[i+1]
		if j.round != int64(i+1) {
			t.Fatalf("rounds %+v", jobs)
		}
		if got := j.scheduled.Sub(j.created); got < d-200*time.Millisecond || got > d+time.Second {
			t.Errorf("round %d scheduled %v after its insert, want %v", j.round, got, d)
		}
	}
	if r := readFile(t, db, u.FileID); r.UploadState != "pending" || r.ScanStatus != "pending" {
		t.Fatalf("row %+v", r)
	}
	if n := count(t, db, `SELECT count(*) FROM event_log WHERE resource_id = $1`, u.FileID); n != 0 {
		t.Fatalf("%d events while pending", n)
	}
	if err := clamd.StartAgain(ctx); err != nil {
		t.Fatal(err)
	}
	if r := waitFinal(t, db, u.FileID); r.ScanStatus != "clean" || r.UploadState != "complete" {
		t.Fatalf("after the start: %+v", r)
	}
}

// TestScanModeOff: with file.scan.mode = off no clamd is needed and files end skipped.
func TestScanModeOff(t *testing.T) {
	logs := &safeBuffer{}
	s := harness.Start(t, harness.Options{Config: config("unix:///nonexistent/clamd.sock", map[string]string{"file.scan.mode": "off"}), LogWriter: logs})
	w := harness.BuildMatrixWorld(t, s)
	alex := w.People["Alex"]
	db := s.AdminDB()
	u := send(t, alex, w.Channels["Support/Allgemein"], "skip.png", "", pngBytes, pngBytes)
	u.complete(t, alex, "")
	r := waitFinal(t, db, u.FileID)
	if r.UploadState == "failed" || r.ScanStatus != "skipped" || r.MIMEType != "image/png" || r.ScanEngine != nil || r.ScannedAt != nil ||
		thumbnailJobs(t, db, u.FileID) != 1 {
		t.Fatalf("row %+v", r)
	}
	if r := waitComplete(t, db, u.FileID); r.UploadState != "complete" {
		t.Fatalf("after files.thumbnail: %+v", r)
	}
	if !strings.Contains(logs.String(), "virus scan is off") {
		t.Error("no warning that the virus scan is off")
	}
}
