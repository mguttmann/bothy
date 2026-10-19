// SPDX-License-Identifier: Apache-2.0

package uploads_test

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestCreateUpload_Limits holds the limits of createUpload (design notes 5, 7 and 8) on a server
// with a small file.max_upload_bytes, a quota, three open sessions and a denied type.
func TestCreateUpload_Limits(t *testing.T) {
	s := harness.Start(t, harness.Options{Config: baseConfig(map[string]string{
		"file.max_upload_bytes": "64", "file.quota_bytes_per_principal": "40", "file.upload_max_open_sessions": "3",
		"file.denied_mime_types": "image/png",
	})})
	w := harness.BuildMatrixWorld(t, s)
	alex, lotse := w.People["Alex"], w.Agents["Lotse"]
	ch := w.Channels["Support/Allgemein"]

	t.Run("size", func(t *testing.T) {
		p := wantProblem(t, create(t, alex, ch, "big.bin", make([]byte, 65), nil), http.StatusRequestEntityTooLarge, "payload_too_large", "")
		if !strings.Contains(p.Detail, "file.max_upload_bytes") {
			t.Fatalf("detail %q does not name the key", p.Detail)
		}
	})
	t.Run("denied type", func(t *testing.T) {
		wantProblem(t, create(t, alex, ch, "a.txt", content, map[string]any{"declared_mime_type": "Image/PNG"}),
			http.StatusUnsupportedMediaType, "unsupported_media_type", "file_type_denied")
		wantProblem(t, create(t, alex, ch, "picture.PNG", content, nil), http.StatusUnsupportedMediaType, "unsupported_media_type", "file_type_denied")
		wantField(t, create(t, alex, ch, "a.txt", content, map[string]any{"declared_mime_type": "not a type"}), "/declared_mime_type", "invalid")
	})
	t.Run("quota", func(t *testing.T) {
		thirty := []byte(strings.Repeat("q", 30))
		first := mustCreate(t, lotse, ch, "a.bin", thirty)
		wantProblem(t, create(t, lotse, ch, "b.bin", make([]byte, 11), nil), http.StatusConflict, "conflict", "quota_exceeded")
		if r := abort(t, lotse, first.FileID); r.Status != http.StatusNoContent {
			t.Fatalf("abort %d %s", r.Status, r.Body)
		}
		// The aborted upload frees its quota (D-M3-03-6).
		mustCreate(t, lotse, ch, "c.bin", thirty)
	})
	t.Run("open sessions", func(t *testing.T) {
		for i := range 3 {
			mustCreate(t, alex, ch, "s"+strconv.Itoa(i)+".txt", content)
		}
		r := create(t, alex, ch, "more.txt", content, nil)
		p := wantProblem(t, r, http.StatusTooManyRequests, "rate_limited", "")
		retry, err := strconv.Atoi(r.Header.Get("Retry-After"))
		if err != nil || retry < 1 || retry > int((24*time.Hour).Seconds()) || !strings.Contains(p.Detail, "file.upload_max_open_sessions") {
			t.Fatalf("Retry-After %q, detail %q", r.Header.Get("Retry-After"), p.Detail)
		}
	})
}

// TestCreateUpload_ParallelAtLimit: parallel creates one below the open-session limit and one
// below the quota: exactly one succeeds each time (the advisory lock, D-M3-03-7).
func TestCreateUpload_ParallelAtLimit(t *testing.T) {
	s := harness.Start(t, harness.Options{Config: baseConfig(map[string]string{
		"file.upload_max_open_sessions": "2", "file.quota_bytes_per_principal": "1000",
	})})
	w := harness.BuildMatrixWorld(t, s)
	alex, lotse := w.People["Alex"], w.Agents["Lotse"]
	ch := w.Channels["Support/Allgemein"]

	mustCreate(t, alex, ch, "first.txt", content)
	if n := parallelCreates(t, alex, ch, 8, content); n != 1 {
		t.Fatalf("%d parallel creates at the session limit succeeded, want 1", n)
	}
	mustCreate(t, lotse, ch, "first.bin", make([]byte, 990))
	if n := parallelCreates(t, lotse, ch, 8, content); n != 1 {
		t.Fatalf("%d parallel creates at the quota succeeded, want 1", n)
	}
}

// parallelCreates sends n creates of b at once and counts the 201 answers.
func parallelCreates(t *testing.T, c *harness.Client, ch string, n int, b []byte) int {
	t.Helper()
	var mu sync.Mutex
	var wg sync.WaitGroup
	ok := 0
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := create(t, c, ch, "p"+strconv.Itoa(i)+".txt", b, nil)
			mu.Lock()
			defer mu.Unlock()
			switch r.Status {
			case http.StatusCreated:
				ok++
			case http.StatusTooManyRequests, http.StatusConflict:
			default:
				t.Errorf("create: %d %s", r.Status, r.Body)
			}
		}(i)
	}
	wg.Wait()
	return ok
}

// TestCreateUpload_StorageFull: with a reserve file.fs.min_free_bytes above the free space of the
// root, createUpload answers 503 unavailable storage_full with Retry-After.
func TestCreateUpload_StorageFull(t *testing.T) {
	s := harness.Start(t, harness.Options{Config: baseConfig(map[string]string{"file.fs.min_free_bytes": "9000000000000000000"})})
	w := harness.BuildMatrixWorld(t, s)
	r := create(t, w.People["Alex"], w.Channels["Support/Allgemein"], "a.txt", content, nil)
	wantProblem(t, r, http.StatusServiceUnavailable, "unavailable", "storage_full")
	if r.Header.Get("Retry-After") == "" {
		t.Fatal("no Retry-After")
	}
}

// TestUploadExpiryAndLostRead runs on a server with a short file.upload_session_ttl: an expired
// and an aborted session answer 410 to a part, and a lost read right aborts the session at the
// next part (12 3.6).
func TestUploadExpiryAndLostRead(t *testing.T) {
	s := harness.Start(t, harness.Options{Config: baseConfig(map[string]string{"file.upload_session_ttl": "2s"})})
	w := harness.BuildMatrixWorld(t, s)
	alex := w.People["Alex"]
	ch := w.Channels["Support/Allgemein"]
	db := s.AdminDB()

	t.Run("UploadPart_Expired410", func(t *testing.T) {
		sess := mustCreate(t, alex, ch, "late.txt", content)
		time.Sleep(2500 * time.Millisecond)
		wantProblem(t, put(t, alex, sess.FileID, 1, part(content, 1), digest(part(content, 1))), http.StatusGone, "gone", "")
		wantProblem(t, get(t, alex, "/uploads/"+sess.FileID), http.StatusGone, "gone", "")
	})
	t.Run("UploadPart_Aborted410", func(t *testing.T) {
		sess := mustCreate(t, alex, ch, "stop.txt", content)
		if r := abort(t, alex, sess.FileID); r.Status != http.StatusNoContent {
			t.Fatalf("abort %d %s", r.Status, r.Body)
		}
		wantProblem(t, put(t, alex, sess.FileID, 1, part(content, 1), digest(part(content, 1))), http.StatusGone, "gone", "")
	})
	t.Run("LostReadAbortsSession", func(t *testing.T) {
		sess := mustCreate(t, alex, ch, "lost.txt", content)
		mustPut(t, alex, sess.FileID, 1, content)
		harness.MustDo(t, w.Owner, harness.Request{Method: http.MethodPost, Path: "/grants/" + w.Grants["G-1"] + "/revoke",
			Body: map[string]any{"reason": "lost read"}}, http.StatusOK)
		wantProblem(t, put(t, alex, sess.FileID, 2, part(content, 2), digest(part(content, 2))), http.StatusNotFound, "not_found", "")
		var state, upState string
		if err := db.QueryRow(t.Context(), `SELECT s.state, f.upload_state FROM file_upload_sessions s JOIN files f ON f.id = s.file_id
			WHERE f.id = $1`, sess.FileID).Scan(&state, &upState); err != nil || state != "aborted" || upState != "failed" {
			t.Fatalf("session %s, file %s (%v)", state, upState, err)
		}
		assertNoParts(t, s, db, sess.FileID)
	})
}
