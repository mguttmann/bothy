// SPDX-License-Identifier: Apache-2.0

package scan_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/clamd/clamdtest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestMain migrates the pgtest templates like bothy migrate up and stops the shared clamd after
// the tests. Without a Docker-compatible runtime the tests fail; they never skip (plan P-09).
func TestMain(m *testing.M) {
	code := harness.Main(m)
	if shared.srv != nil {
		if err := shared.srv.Terminate(context.Background()); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "scan tests: %v\n", err)
			if code == 0 {
				code = 1
			}
		}
	}
	os.Exit(code)
}

// The limits of the test clamd and of the test servers.
const (
	partBytes        = 16384
	testStreamMax    = 256 << 10
	testMaxScanSize  = 128 << 10
	testMaxRecursion = 3
)

var shared struct {
	srv *clamdtest.Clamd
	err error
}

// sharedClamd starts the shared test clamd on first use (tests of this package run one at a time
// at the top level, so no lock is needed).
func sharedClamd(t *testing.T) *clamdtest.Clamd {
	t.Helper()
	if shared.srv == nil && shared.err == nil {
		ctx, cancel := context.WithDeadline(context.Background(), testkit.Deadline())
		defer cancel()
		shared.srv, shared.err = clamdtest.Start(ctx, clamdtest.Options{StreamMaxLength: testStreamMax, MaxScanSize: testMaxScanSize,
			MaxFileSize: testStreamMax, MaxRecursion: testMaxRecursion})
	}
	if shared.err != nil {
		t.Fatalf("clamd: %v", shared.err)
	}
	return shared.srv
}

// config are the keys of a test server with the clamd at address; extra entries win.
func config(address string, extra map[string]string) map[string]string {
	cfg := harness.RaisedRateLimits()
	cfg["file.upload_part_bytes"] = strconv.Itoa(partBytes)
	cfg["file.max_upload_bytes"] = "1048576"
	cfg["file.upload_max_open_sessions"] = "1000"
	cfg["file.scan.clamd_address"] = address
	cfg["file.scan.health_interval"] = "1s"
	cfg["file.scan.retry_backoff_cap"] = "4s"
	for k, v := range extra {
		cfg[k] = v
	}
	return cfg
}

func sha(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
}

// upload is one created upload with its parts sent.
type upload struct {
	FileID string
	parts  []map[string]any
}

// send creates an upload of declared named name with declared type mime (empty: none) and sends
// content as its parts; content and declared have the same length.
func send(t *testing.T, c *harness.Client, channel, name, mime string, content, declared []byte) upload {
	t.Helper()
	body := map[string]any{"name": name, "size_bytes": len(declared), "sha256": sha(declared)}
	if mime != "" {
		body["declared_mime_type"] = mime
	}
	r := c.Do(t.Context(), harness.Request{Method: http.MethodPost, Path: "/channels/" + channel + "/uploads", Body: body})
	if r.Status != http.StatusCreated {
		t.Fatalf("createUpload: %d %s", r.Status, r.Body)
	}
	var s struct {
		FileID string `json:"file_id"`
	}
	r.JSON(t, &s)
	u := upload{FileID: s.FileID}
	for n, lo := 1, 0; lo < len(content); n, lo = n+1, lo+partBytes {
		part := content[lo:min(lo+partBytes, len(content))]
		h := http.Header{}
		h.Set("Content-Type", "application/octet-stream")
		h.Set("Content-Digest", digest(part))
		pr := c.Do(t.Context(), harness.Request{Method: http.MethodPut, Path: "/uploads/" + s.FileID + "/parts/" + strconv.Itoa(n), Body: part, Header: h})
		if pr.Status != http.StatusOK {
			t.Fatalf("uploadPart %d: %d %s", n, pr.Status, pr.Body)
		}
		var out struct {
			ETag string `json:"etag"`
		}
		pr.JSON(t, &out)
		u.parts = append(u.parts, map[string]any{"part_number": n, "etag": out.ETag})
	}
	return u
}

// complete calls completeUpload with key as Idempotency-Key ("" lets the client choose one).
func (u upload) complete(t *testing.T, c *harness.Client, key string) harness.Response {
	t.Helper()
	h := http.Header{}
	if key != "" {
		h.Set("Idempotency-Key", key)
	}
	r := c.Do(t.Context(), harness.Request{Method: http.MethodPost, Path: "/uploads/" + u.FileID + "/complete", Body: map[string]any{"parts": u.parts}, Header: h})
	if r.Status != http.StatusOK {
		t.Fatalf("completeUpload: %d %s", r.Status, r.Body)
	}
	return r
}

// fileRow is the scan state of a file and its session.
type fileRow struct {
	UploadState, ScanStatus, MIMEType, ObjectKey string
	ScanEngine, FailureReason                    *string
	ScannedAt                                    *time.Time
	SHA256                                       []byte
}

func readFile(t *testing.T, db *pgxpool.Pool, id string) fileRow {
	t.Helper()
	var r fileRow
	if err := db.QueryRow(t.Context(), `SELECT f.upload_state, f.scan_status, f.mime_type, f.object_key, f.scan_engine, s.failure_reason,
		f.scanned_at, f.sha256 FROM files f JOIN file_upload_sessions s ON s.file_id = f.id WHERE f.id = $1`, id).
		Scan(&r.UploadState, &r.ScanStatus, &r.MIMEType, &r.ObjectKey, &r.ScanEngine, &r.FailureReason, &r.ScannedAt, &r.SHA256); err != nil {
		t.Fatal(err)
	}
	return r
}

// waitFinal waits until the file left upload_state and scan_status pending.
func waitFinal(t *testing.T, db *pgxpool.Pool, id string) fileRow {
	t.Helper()
	var r fileRow
	testkit.Until(t, "file "+id+" left pending", func() bool {
		r = readFile(t, db, id)
		return r.UploadState != "pending" || r.ScanStatus != "pending"
	})
	return r
}

// count runs a count query with one argument.
func count(t *testing.T, db *pgxpool.Pool, q string, arg any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(t.Context(), q, arg).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func strOf(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// pngBytes is the start of a PNG file.
var pngBytes = append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), bytes.Repeat([]byte{0}, 64)...)

// gifBytes is the start of a GIF file.
var gifBytes = append([]byte("GIF89a"), bytes.Repeat([]byte{1}, 64)...)
