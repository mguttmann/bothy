// SPDX-License-Identifier: Apache-2.0

package download_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/clamd/clamdtest"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// partBytes is the part size of the test servers.
const partBytes = 16384

// env is one test server with its world and its scripted clamd.
type env struct {
	s     *harness.Server
	w     *harness.World
	clamd *clamdtest.Fake
	db    *pgxpool.Pool
	logs  *safeBuffer
	ch    string
	alex  *harness.Client
	sam   *harness.Client
	robin *harness.Client
	plain *http.Client
}

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

// start runs a server in role all with the fake clamd, the matrix world and extra configuration.
func start(t *testing.T, opts harness.Options, extra map[string]string) *env {
	t.Helper()
	fake, err := clamdtest.NewFake()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fake.Close() })
	cfg := harness.RaisedRateLimits()
	cfg["file.upload_part_bytes"] = strconv.Itoa(partBytes)
	cfg["file.max_upload_bytes"] = "1048576"
	cfg["file.upload_max_open_sessions"] = "1000"
	cfg["file.scan.clamd_address"] = fake.Address()
	cfg["file.scan.health_interval"] = "1s"
	cfg["file.scan.timeout"] = "1s"
	cfg["file.scan.retry_backoff_cap"] = "2s"
	cfg["file.not_ready_retry_after"] = "7s"
	cfg["client.cache_max_age"] = "720h"
	cfg["audit.deny_aggregate_window"] = "50ms"
	for k, v := range extra {
		cfg[k] = v
	}
	opts.Config = cfg
	logs := &safeBuffer{}
	if opts.LogWriter == nil {
		opts.LogWriter = logs
	}
	s := harness.Start(t, opts)
	w := harness.BuildMatrixWorld(t, s)
	return &env{s: s, w: w, clamd: fake, db: s.AdminDB(), logs: logs, ch: w.Channels["Support/Allgemein"],
		alex: w.People["Alex"], sam: w.People["Sam"], robin: w.People["Robin"], plain: &http.Client{Timeout: testkit.Patience}}
}

// duration reads a duration key of the server ("720h", "7s", also "30d").
func (e *env) duration(t *testing.T, key string) time.Duration {
	t.Helper()
	v := e.s.ConfigValue(key)
	if d, ok := strings.CutSuffix(v, "d"); ok {
		n, err := strconv.Atoi(d)
		if err != nil {
			t.Fatalf("%s = %q", key, v)
		}
		return time.Duration(n) * 24 * time.Hour
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		t.Fatalf("%s = %q: %v", key, v, err)
	}
	return d
}

func sha(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
}

// upload sends content as a file named name through the real upload flow of M3-03 as c and
// completes it; declared is the content the client claims (its SHA-256 and length).
func (e *env) upload(t *testing.T, c *harness.Client, name string, content, declared []byte) string {
	t.Helper()
	r := c.Do(t.Context(), harness.Request{Method: http.MethodPost, Path: "/channels/" + e.ch + "/uploads",
		Body: map[string]any{"name": name, "size_bytes": len(declared), "sha256": sha(declared)}})
	if r.Status != http.StatusCreated {
		t.Fatalf("createUpload: %d %s", r.Status, r.Body)
	}
	var s struct {
		FileID string `json:"file_id"`
	}
	r.JSON(t, &s)
	var parts []map[string]any
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
		parts = append(parts, map[string]any{"part_number": n, "etag": out.ETag})
	}
	harness.MustDo(t, c, harness.Request{Method: http.MethodPost, Path: "/uploads/" + s.FileID + "/complete", Body: map[string]any{"parts": parts}}, http.StatusOK)
	return s.FileID
}

// ready uploads content as Alex and waits until the scan pass made it final.
func (e *env) ready(t *testing.T, name string, content []byte) string {
	t.Helper()
	id := e.upload(t, e.alex, name, content, content)
	if st := e.waitFinal(t, id); st != "complete/clean" {
		t.Fatalf("file %s ended %s", name, st)
	}
	return id
}

// state is upload_state/scan_status of a file.
func (e *env) state(t *testing.T, id string) string {
	t.Helper()
	var up, scan string
	if err := e.db.QueryRow(t.Context(), `SELECT upload_state, scan_status FROM files WHERE id = $1`, id).Scan(&up, &scan); err != nil {
		t.Fatal(err)
	}
	return up + "/" + scan
}

// waitFinal waits until the file left upload_state and scan_status pending and returns its state;
// testkit.Patience bounds the wait.
func (e *env) waitFinal(t *testing.T, id string) string {
	t.Helper()
	var st string
	testkit.Until(t, "file "+id+" leaves pending/pending", func() bool {
		st = e.state(t, id)
		return st != "pending/pending"
	})
	return st
}

// exec runs a statement of the test setup.
func (e *env) exec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := e.db.Exec(t.Context(), q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// writeObject stores body under key in the file system driver of the server.
func (e *env) writeObject(t *testing.T, key string, body []byte) {
	t.Helper()
	p := filepath.Join(e.s.FilesRoot, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

// seedThumbnail stores body as the thumbnail variant of file id with mimeType.
func (e *env) seedThumbnail(t *testing.T, id, variant, mimeType string, body []byte) {
	t.Helper()
	key, err := domain.VariantKey(ids.MustParse(id), domain.Variant(variant))
	if err != nil {
		t.Fatal(err)
	}
	e.writeObject(t, key, body)
	e.exec(t, `INSERT INTO file_thumbnails (file_id, variant, object_key, mime_type, width, height, size_bytes) VALUES ($1, $2, $3, $4, 8, 8, $5)`,
		id, variant, key, mimeType, len(body))
}

// seedFile stores a deliverable file of purpose without a scope channel, uploaded by uploader.
func (e *env) seedFile(t *testing.T, uploader, purpose, name, mimeType string, body []byte) string {
	t.Helper()
	id := ids.New()
	key, err := domain.OriginalKey(id, 1)
	if err != nil {
		t.Fatal(err)
	}
	e.writeObject(t, key, body)
	sum := sha256.Sum256(body)
	e.exec(t, `INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose, upload_state, scan_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'complete', 'clean')`, id.String(), key, sum[:], len(body), mimeType, name, uploader, purpose)
	return id.String()
}

// fetch sends an anonymous GET of a relative reference (/api/v1/...) without any credential, as a
// media player does, with extra headers.
func (e *env) fetch(t *testing.T, ref string, h http.Header) harness.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, e.s.PublicURL+ref, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range h {
		req.Header[k] = v
	}
	resp, err := e.plain.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return harness.Response{Status: resp.StatusCode, Header: resp.Header, Body: b}
}

// ticket is the answer of createFileTicket.
type ticket struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// newTicket creates a ticket of variant for file id as c and fails unless the answer is 201.
func newTicket(t *testing.T, c *harness.Client, id, variant string) ticket {
	t.Helper()
	var tk ticket
	harness.MustDo(t, c, harness.Request{Method: http.MethodPost, Path: "/files/" + id + "/tickets", Body: map[string]any{"variant": variant}},
		http.StatusCreated).JSON(t, &tk)
	return tk
}

// get downloads a path below /api/v1 as c with extra headers.
func get(t *testing.T, c *harness.Client, path string, h http.Header) harness.Response {
	t.Helper()
	return c.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: path, Header: h})
}

// header returns a header with one value.
func header(k, v string) http.Header {
	h := http.Header{}
	h.Set(k, v)
	return h
}

// problemOf is the problem body without its per-request members.
func problemOf(t *testing.T, r harness.Response) string {
	t.Helper()
	var p map[string]any
	r.JSON(t, &p)
	delete(p, "correlation_id")
	delete(p, "instance")
	b := new(strings.Builder)
	for _, k := range []string{"type", "title", "status", "code", "detail", "reason"} {
		if v, ok := p[k]; ok {
			b.WriteString(k + "=" + toString(v) + ";")
			delete(p, k)
		}
	}
	if len(p) > 0 {
		t.Errorf("problem has unexpected members %v", p)
	}
	return b.String()
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return "?"
}

// headerSet is the response headers without the per-request ones.
func headerSet(r harness.Response) map[string]string {
	out := map[string]string{}
	for k, v := range r.Header {
		switch k {
		case "Correlation-Id", "Date", "Ratelimit", "Ratelimit-Policy", "Content-Length",
			// The authenticator adds DPoP-Nonce to any response once the client's nonce is stale
			// (06 section 3.1); that depends on when the nonce rotates, not on the response.
			"Dpop-Nonce":
			continue
		}
		out[k] = strings.Join(v, ",")
	}
	return out
}

// pngBytes is the start of a PNG file.
var pngBytes = append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), bytes.Repeat([]byte{7}, 300)...)
