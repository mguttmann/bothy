// SPDX-License-Identifier: Apache-2.0

package textextract_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/clamd/clamdtest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// partBytes is file.upload_part_bytes of the test servers.
const partBytes = 262144

// servers bounds the harness servers of this package that run at the same time: every server
// opens its own pools on the one pgtest container, whose connection slots run out with more than
// a few parallel servers (SQLSTATE 53300).
var servers = make(chan struct{}, 3)

// env is one test server with the matrix world.
type env struct {
	s     *harness.Server
	db    *pgxpool.Pool
	logs  *serverLog
	clamd *clamdtest.Fake
	owner *harness.Client
	alex  *harness.Client
	robin *harness.Client
	// allg is the channel Support/Allgemein, where Alex uploads (G-1 gives upload) and Robin reads
	// nothing.
	allg string
}

// options are the settings of one test server.
type options struct {
	// cfg overrides keys of the configuration.
	cfg map[string]string
	// clamd scans with the scripted clamd of M3-04; without it file.scan.mode is off.
	clamd bool
	// telemetry are the in-process telemetry sinks; such a test never runs in parallel.
	telemetry *harness.TelemetrySinks
	// locale is locale.default; "" keeps the default.
	locale string
}

// start runs a server in role all with the matrix world.
func start(t *testing.T, o options) *env {
	t.Helper()
	servers <- struct{}{}
	t.Cleanup(func() { <-servers })
	cfg := harness.RaisedRateLimits()
	cfg["file.upload_part_bytes"] = strconv.Itoa(partBytes)
	cfg["file.max_upload_bytes"] = "4194304"
	cfg["file.upload_max_open_sessions"] = "1000"
	e := &env{logs: &serverLog{}}
	if o.clamd {
		fake, err := clamdtest.NewFake()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = fake.Close() })
		e.clamd = fake
		cfg["file.scan.clamd_address"] = fake.Address()
		cfg["file.scan.health_interval"] = "1s"
		cfg["file.scan.timeout"] = "5s"
		cfg["file.scan.retry_backoff_cap"] = "2s"
	} else {
		cfg["file.scan.mode"] = "off"
	}
	for k, v := range o.cfg {
		cfg[k] = v
	}
	e.s = harness.Start(t, harness.Options{Config: cfg, LogWriter: e.logs, Telemetry: o.telemetry})
	w := harness.BuildMatrixWorld(t, e.s)
	e.db, e.owner, e.alex, e.robin, e.allg = e.s.AdminDB(), e.s.Owner, w.People["Alex"], w.People["Robin"], w.Channels["Support/Allgemein"]
	if o.locale != "" {
		r, err := harness.DoWithStepUp(t.Context(), e.owner, harness.Request{Method: http.MethodPut, Path: "/instance/settings/locale.default",
			Body: map[string]any{"value": o.locale}})
		if err != nil || r.Status != http.StatusOK {
			t.Fatalf("locale.default: %v %d %s", err, r.Status, r.Body)
		}
	}
	return e
}

// serverLog keeps the log lines of a test server; it is safe for concurrent use.
type serverLog struct {
	mu  sync.Mutex
	buf strings.Builder
}

// Write implements io.Writer.
func (l *serverLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

// text returns every line written so far.
func (l *serverLog) text() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// lines returns the log lines that contain needle.
func (l *serverLog) lines(needle string) []string {
	var out []string
	for _, ln := range strings.Split(l.text(), "\n") {
		if strings.Contains(ln, needle) {
			out = append(out, ln)
		}
	}
	return out
}

func sha(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
}

// upload creates an upload of content named name in Allgemein as Alex, declaring declared (the
// content's digest when nil), sends its parts and completes it; the scan pass runs afterwards in
// the worker. It returns the file id.
func (e *env) upload(t *testing.T, name string, content, declared []byte) string {
	t.Helper()
	if declared == nil {
		declared = content
	}
	r := e.alex.Do(t.Context(), harness.Request{Method: http.MethodPost, Path: "/channels/" + e.allg + "/uploads",
		Body: map[string]any{"name": name, "size_bytes": len(content), "sha256": sha(declared)}})
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
		pr := e.alex.Do(t.Context(), harness.Request{Method: http.MethodPut, Path: "/uploads/" + s.FileID + "/parts/" + strconv.Itoa(n), Body: part, Header: h})
		if pr.Status != http.StatusOK {
			t.Fatalf("uploadPart %d: %d %s", n, pr.Status, pr.Body)
		}
		var out struct {
			ETag string `json:"etag"`
		}
		pr.JSON(t, &out)
		parts = append(parts, map[string]any{"part_number": n, "etag": out.ETag})
	}
	harness.MustDo(t, e.alex, harness.Request{Method: http.MethodPost, Path: "/uploads/" + s.FileID + "/complete", Body: map[string]any{"parts": parts}},
		http.StatusOK)
	return s.FileID
}

// state is upload_state/scan_status/mime_type of a file.
func (e *env) state(t *testing.T, id string) string {
	t.Helper()
	var up, scan, mime string
	if err := e.db.QueryRow(t.Context(), `SELECT upload_state, scan_status, mime_type FROM files WHERE id = $1`, id).Scan(&up, &scan, &mime); err != nil {
		t.Fatal(err)
	}
	return up + "/" + scan + "/" + mime
}

// final waits until the scan pass decided the file and returns its state.
func (e *env) final(t *testing.T, id string) string {
	t.Helper()
	var st string
	testkit.Until(t, "the scan of file "+id, func() bool {
		st = e.state(t, id)
		return !strings.HasPrefix(st, "pending/")
	})
	return st
}

// attached uploads content as a deliverable attachment named name on a new message of Alex in
// Allgemein; the detected type must be mime. It returns the file id and the message id.
func (e *env) attached(t *testing.T, name, mime string, content []byte) (string, string) {
	t.Helper()
	id := e.upload(t, name, content, nil)
	if st := e.final(t, id); st != "complete/skipped/"+mime && st != "complete/clean/"+mime {
		t.Fatalf("file %s ended %s, want deliverable %s", name, st, mime)
	}
	return id, e.attach(t, id)
}

// attach posts a message of Alex in the main thread of Allgemein with the file and returns its id.
func (e *env) attach(t *testing.T, fileID string) string {
	t.Helper()
	var thread string
	if err := e.db.QueryRow(t.Context(), `SELECT id::text FROM threads WHERE channel_id = $1 AND kind = 'main'`, e.allg).Scan(&thread); err != nil {
		t.Fatal(err)
	}
	r, err := harness.DoWithStepUp(t.Context(), e.alex, harness.Request{Method: http.MethodPost, Path: "/threads/" + thread + "/messages",
		Body: map[string]any{"client_msg_id": ids.New().String(), "content_md": "see the file", "attachment_file_ids": []string{fileID}}})
	if err != nil || r.Status != http.StatusCreated {
		t.Fatalf("createMessage: %v %d %s", err, r.Status, r.Body)
	}
	var m struct {
		ID string `json:"id"`
	}
	r.JSON(t, &m)
	return m.ID
}

// serverRow is a file_text_extracts row as the server tests read it.
type serverRow struct {
	State, Extractor, Text, Error, Language string
	Truncated, HasText, Found               bool
}

// row reads the extract row of a file.
func (e *env) row(t *testing.T, id string) serverRow {
	t.Helper()
	var r serverRow
	var extractor, text, code *string
	err := e.db.QueryRow(t.Context(), `SELECT state, extractor, text_content, error, search_language::text, truncated
		FROM file_text_extracts WHERE file_id = $1`, id).Scan(&r.State, &extractor, &text, &code, &r.Language, &r.Truncated)
	if errors.Is(err, pgx.ErrNoRows) {
		return r
	}
	if err != nil {
		t.Fatal(err)
	}
	r.Found, r.HasText = true, text != nil
	if extractor != nil {
		r.Extractor = *extractor
	}
	if text != nil {
		r.Text = *text
	}
	if code != nil {
		r.Error = *code
	}
	return r
}

// extracted waits until the extract row of a file left pending and returns it.
func (e *env) extracted(t *testing.T, id string) serverRow {
	t.Helper()
	var r serverRow
	testkit.Until(t, "the extraction of file "+id, func() bool {
		r = e.row(t, id)
		return r.Found && r.State != "pending"
	})
	return r
}

// count runs a count query.
func (e *env) count(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(t.Context(), q, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

// exec runs a statement of the test setup.
func (e *env) exec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := e.db.Exec(t.Context(), q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// fileHits sends GET /search?type=files as c for q and returns the file ids of the page with the
// joined segment texts of each.
func fileHits(t *testing.T, c *harness.Client, q string) map[string]string {
	t.Helper()
	r := c.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/search?" + url.Values{"q": {q}, "type": {"files"}}.Encode()})
	if r.Status != http.StatusOK {
		t.Fatalf("search %q: %d %s", q, r.Status, r.Body)
	}
	var p struct {
		Items []struct {
			File     map[string]any `json:"file"`
			Segments []struct {
				Text  string `json:"text"`
				Match bool   `json:"match"`
			} `json:"segments"`
		} `json:"items"`
	}
	r.JSON(t, &p)
	out := map[string]string{}
	for _, it := range p.Items {
		var b strings.Builder
		for _, s := range it.Segments {
			if s.Match {
				b.WriteString("[" + s.Text + "]")
			} else {
				b.WriteString(s.Text)
			}
		}
		out[it.File["id"].(string)] = b.String()
	}
	return out
}
