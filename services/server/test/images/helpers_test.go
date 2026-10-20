// SPDX-License-Identifier: Apache-2.0

package images_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestMain migrates the pgtest templates like bothy migrate up. Without a Docker-compatible
// runtime the tests fail; they never skip (plan P-09).
func TestMain(m *testing.M) {
	os.Exit(harness.Main(m))
}

// The limits of the test server and of the waits.
const (
	partBytes = 16384
	// The variant edges of the test server: small enough that the test images reach them.
	smallPx  = 16
	mediumPx = 32
	largePx  = 64
	// maxPixels is file.image.max_pixels of the test server: far above every normal test image
	// and far below the declared size of every bomb.
	maxPixels = 1_000_000
	// waitJob bounds every wait for the scan and files.thumbnail, including one River retry with
	// its backoff.
	waitJob = 2 * time.Minute
	// pollEvery is the poll interval of the waits.
	pollEvery = 100 * time.Millisecond
	// stillFor is how long a done file is watched for late changes.
	stillFor = 3 * time.Second
)

// The values of the rows and payloads the tests compare.
const (
	stateComplete = "complete"
	statePending  = "pending"
	stateFailed   = "failed"
	scanSkipped   = "skipped"
	mimeJPEG      = "image/jpeg"
	mimePNG       = "image/png"
	mimeGIF       = "image/gif"
	mimeWebP      = "image/webp"
	jobKind       = "files.thumbnail"
	// classTooManyPixels is the error class the job logs when the pixel guard refused a decode.
	classTooManyPixels = "too_many_pixels"
)

// config returns the keys of the test server: no clamd (file.scan.mode off), small variant edges
// and pixel limit, and one image run at a time (file.image.max_concurrent = 1), which
// TestImageJobConcurrencyLimit checks and which keeps a held run from racing another test's run.
func config() map[string]string {
	cfg := harness.RaisedRateLimits()
	for k, v := range map[string]string{
		"file.upload_part_bytes":        strconv.Itoa(partBytes),
		"file.max_upload_bytes":         "1048576",
		"file.upload_max_open_sessions": "1000",
		"file.scan.mode":                "off",
		"file.scan.clamd_address":       "unix:///nonexistent/clamd.sock",
		"file.thumbnail.small_px":       strconv.Itoa(smallPx),
		"file.thumbnail.medium_px":      strconv.Itoa(mediumPx),
		"file.thumbnail.large_px":       strconv.Itoa(largePx),
		"file.image.max_pixels":         strconv.Itoa(maxPixels),
		"file.image.max_concurrent":     "1",
	} {
		cfg[k] = v
	}
	return cfg
}

// safeBuffer collects the log lines of a server.
type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

// Write implements io.Writer.
func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

// String returns everything written so far.
func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// hooks routes the test seams of files.thumbnail to behaviour registered per file ID, so one
// server serves every case: a file without registration runs unchanged. It also counts the runs
// that hold a slot at the same time over the whole life of the server.
type hooks struct {
	beforeRun    sync.Map // file ID -> func(context.Context) error
	afterPut     sync.Map // file ID -> func(context.Context)
	beforeCommit sync.Map // file ID -> func(context.Context) error
	active       atomic.Int64
	maxActive    atomic.Int64
	runs         atomic.Int64
}

// seams returns the ImageHooks of h for harness.Options.
func (h *hooks) seams() *filesapp.ImageHooks {
	return &filesapp.ImageHooks{
		BeforeRun: func(ctx context.Context, id ids.ID) error {
			h.runs.Add(1)
			n := h.active.Add(1)
			for {
				m := h.maxActive.Load()
				if n <= m || h.maxActive.CompareAndSwap(m, n) {
					break
				}
			}
			if f, ok := h.beforeRun.Load(id.String()); ok {
				return f.(func(context.Context) error)(ctx)
			}
			return nil
		},
		AfterRun: func(ids.ID) { h.active.Add(-1) },
		AfterPut: func(ctx context.Context, id ids.ID) {
			if f, ok := h.afterPut.Load(id.String()); ok {
				f.(func(context.Context))(ctx)
			}
		},
		BeforeCommit: func(ctx context.Context, id ids.ID) error {
			if f, ok := h.beforeCommit.Load(id.String()); ok {
				return f.(func(context.Context) error)(ctx)
			}
			return nil
		},
	}
}

// env is the shared test server with its world, the uploader Alex, the reader Sam, the channel
// Support/Allgemein, the admin pool, the captured logs and the hooks.
type env struct {
	s       *harness.Server
	alex    *harness.Client
	sam     *harness.Client
	channel string
	db      *pgxpool.Pool
	logs    *safeBuffer
	hooks   *hooks
}

// startEnv starts the server of the package with hooks and captured logs and builds the matrix
// world on it.
func startEnv(t *testing.T) *env {
	t.Helper()
	h := &hooks{}
	logs := &safeBuffer{}
	s := harness.Start(t, harness.Options{Config: config(), LogWriter: logs, FilesImageHooks: h.seams()})
	w := harness.BuildMatrixWorld(t, s)
	return &env{s: s, alex: w.People["Alex"], sam: w.People["Sam"], channel: w.Channels["Support/Allgemein"], db: s.AdminDB(), logs: logs, hooks: h}
}

// sha returns the hex SHA-256 of b.
func sha(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// digest returns the Content-Digest value of b.
func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
}

// upload is one created upload with its parts sent.
type upload struct {
	FileID string
	parts  []map[string]any
}

// create calls createUpload as Alex for content named name with the declared type mime (empty:
// none) and sends the parts; the job starts only after complete, so a test registers its hooks
// for FileID in between.
func (e *env) create(t *testing.T, name, mime string, content []byte) upload {
	t.Helper()
	body := map[string]any{"name": canaryNamePrefix + name, "size_bytes": len(content), "sha256": sha(content)}
	if mime != "" {
		body["declared_mime_type"] = mime
	}
	var s struct {
		FileID string `json:"file_id"`
	}
	harness.MustDo(t, e.alex, harness.Request{Method: http.MethodPost, Path: "/channels/" + e.channel + "/uploads", Body: body}, http.StatusCreated).JSON(t, &s)
	u := upload{FileID: s.FileID}
	for n, lo := 1, 0; lo < len(content); n, lo = n+1, lo+partBytes {
		part := content[lo:min(lo+partBytes, len(content))]
		h := http.Header{}
		h.Set("Content-Type", "application/octet-stream")
		h.Set("Content-Digest", digest(part))
		var out struct {
			ETag string `json:"etag"`
		}
		harness.MustDo(t, e.alex, harness.Request{Method: http.MethodPut, Path: "/uploads/" + s.FileID + "/parts/" + strconv.Itoa(n), Body: part, Header: h},
			http.StatusOK).JSON(t, &out)
		u.parts = append(u.parts, map[string]any{"part_number": n, "etag": out.ETag})
	}
	return u
}

// complete calls completeUpload as Alex.
func (e *env) complete(t *testing.T, u upload) {
	t.Helper()
	harness.MustDo(t, e.alex, harness.Request{Method: http.MethodPost, Path: "/uploads/" + u.FileID + "/complete", Body: map[string]any{"parts": u.parts}}, http.StatusOK)
}

// send creates and completes an upload and returns its file ID.
func (e *env) send(t *testing.T, name, mime string, content []byte) string {
	t.Helper()
	u := e.create(t, name, mime, content)
	e.complete(t, u)
	return u.FileID
}

// fileRow is the state of a file and its session.
type fileRow struct {
	UploadState, ScanStatus, MIMEType, ObjectKey, OriginalName string
	FailureReason                                              *string
	Width, Height                                              *int32
	DurationMS                                                 *int64
	SizeBytes                                                  int64
	SHA256, DeclaredSHA256                                     []byte
	Deleted                                                    bool
}

// revision returns the revision of the object key, 0 when it is no original key.
func (r fileRow) revision() int {
	_, rev, err := domain.ParseOriginalKey(r.ObjectKey)
	if err != nil {
		return 0
	}
	return rev
}

// readFile reads the row of file id with its session.
func (e *env) readFile(t *testing.T, id string) fileRow {
	t.Helper()
	var r fileRow
	if err := e.db.QueryRow(t.Context(), `SELECT f.upload_state, f.scan_status, f.mime_type, f.object_key, f.original_name, s.failure_reason,
		f.width, f.height, f.duration_ms, f.size_bytes, f.sha256, s.declared_sha256, f.deleted_at IS NOT NULL
		FROM files f JOIN file_upload_sessions s ON s.file_id = f.id WHERE f.id = $1`, id).
		Scan(&r.UploadState, &r.ScanStatus, &r.MIMEType, &r.ObjectKey, &r.OriginalName, &r.FailureReason, &r.Width, &r.Height, &r.DurationMS,
			&r.SizeBytes, &r.SHA256, &r.DeclaredSHA256, &r.Deleted); err != nil {
		t.Fatal(err)
	}
	return r
}

// count runs a count query with one argument.
func (e *env) count(t *testing.T, q string, arg any) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(t.Context(), q, arg).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// jobs counts the files.thumbnail jobs of file id.
func (e *env) jobs(t *testing.T, id string) int {
	t.Helper()
	return e.count(t, `SELECT count(*) FROM jobs.river_job WHERE kind = 'files.thumbnail' AND args->>'file_id' = $1`, id)
}

// thumbnails counts the file_thumbnails rows of file id.
func (e *env) thumbnails(t *testing.T, id string) int {
	t.Helper()
	return e.count(t, `SELECT count(*) FROM file_thumbnails WHERE file_id = $1`, id)
}

// audits counts the audit entries file.upload of file id.
func (e *env) audits(t *testing.T, id string) int {
	t.Helper()
	return e.count(t, `SELECT count(*) FROM audit_events WHERE resource_id = $1 AND action = 'file.upload'`, id)
}

// poll calls done every pollEvery until it reports true or waitJob passed; it fails the test
// with what on timeout.
func poll(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitJob)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("%s: not reached after %v", what, waitJob)
		}
		time.Sleep(pollEvery)
	}
}

// waitScanned waits until the scan left scan_status pending and returns the row.
func (e *env) waitScanned(t *testing.T, id string) fileRow {
	t.Helper()
	var r fileRow
	poll(t, "scan of "+id, func() bool {
		r = e.readFile(t, id)
		return r.ScanStatus != statePending
	})
	return r
}

// waitJobDone waits until every files.thumbnail job of file id reached a final River state and
// returns the row.
func (e *env) waitJobDone(t *testing.T, id string) fileRow {
	t.Helper()
	poll(t, "files.thumbnail of "+id, func() bool {
		return e.jobs(t, id) > 0 && e.count(t, `SELECT count(*) FROM jobs.river_job WHERE kind = 'files.thumbnail' AND args->>'file_id' = $1
			AND state NOT IN ('completed', 'discarded', 'cancelled')`, id) == 0
	})
	return e.readFile(t, id)
}

// fileEvent is one file.updated row with the parts of its payload the tests check.
type fileEvent struct {
	Seq     int64
	Raw     []byte
	Payload struct {
		File struct {
			ID           string `json:"id"`
			UploadState  string `json:"upload_state"`
			ScanStatus   string `json:"scan_status"`
			MIMEType     string `json:"mime_type"`
			OriginalName string `json:"original_name"`
			SHA256       string `json:"sha256"`
			SizeBytes    int64  `json:"size_bytes"`
			Width        int    `json:"width"`
			Height       int    `json:"height"`
			DurationMS   int64  `json:"duration_ms"`
			Thumbnails   []struct {
				Variant  string `json:"variant"`
				MIMEType string `json:"mime_type"`
				Width    int    `json:"width"`
				Height   int    `json:"height"`
			} `json:"thumbnails"`
		} `json:"file"`
	}
}

// events returns the file.updated rows of file id in seq order.
func (e *env) events(t *testing.T, id string) []fileEvent {
	t.Helper()
	rows, err := e.db.Query(t.Context(), `SELECT seq, payload FROM event_log WHERE type = 'file.updated' AND resource_id = $1 ORDER BY seq`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []fileEvent
	for rows.Next() {
		var ev fileEvent
		if err := rows.Scan(&ev.Seq, &ev.Raw); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(ev.Raw, &ev.Payload); err != nil {
			t.Fatalf("payload %s: %v", ev.Raw, err)
		}
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// fileDir returns the directory of the objects of file id under the store root.
func (e *env) fileDir(t *testing.T, id string) string {
	t.Helper()
	k, err := domain.OriginalKey(ids.MustParse(id), 1)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(e.s.FilesRoot, filepath.Dir(k))
}

// objects returns the names of the objects of file id (orig-1, orig-2, small, ...), sorted.
func (e *env) objects(t *testing.T, id string) []string {
	t.Helper()
	entries, err := os.ReadDir(e.fileDir(t, id))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, d := range entries {
		out = append(out, d.Name())
	}
	return out
}

// pathsOf walks the whole store root and returns every path that names file id (its objects and
// its upload parts).
func (e *env) pathsOf(t *testing.T, id string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(e.s.FilesRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(p, id) && !d.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// object reads the object key under the store root.
func (e *env) object(t *testing.T, key string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(e.s.FilesRoot, key)) // #nosec G304 -- a key of the test server
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// thumbRow is one file_thumbnails row.
type thumbRow struct {
	Variant, ObjectKey, MIMEType string
	Width, Height                int
	SizeBytes                    int64
}

// thumbRows returns the file_thumbnails rows of file id ordered small, medium, large.
func (e *env) thumbRows(t *testing.T, id string) []thumbRow {
	t.Helper()
	rows, err := e.db.Query(t.Context(), `SELECT variant, object_key, mime_type, width, height, size_bytes FROM file_thumbnails WHERE file_id = $1
		ORDER BY array_position(ARRAY['small','medium','large','poster'], variant)`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []thumbRow
	for rows.Next() {
		var r thumbRow
		if err := rows.Scan(&r.Variant, &r.ObjectKey, &r.MIMEType, &r.Width, &r.Height, &r.SizeBytes); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// apiFile is the part of the schema File the tests read from getFile.
type apiFile struct {
	UploadState string `json:"upload_state"`
	ScanStatus  string `json:"scan_status"`
	MIMEType    string `json:"mime_type"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Thumbnails  []struct {
		Variant  string `json:"variant"`
		MIMEType string `json:"mime_type"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
	} `json:"thumbnails"`
}

// getFile calls getFile as c and returns the status and the decoded body on 200.
func getFile(t *testing.T, c *harness.Client, id string) (int, apiFile) {
	t.Helper()
	r := c.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/files/" + id})
	var f apiFile
	if r.Status == http.StatusOK {
		r.JSON(t, &f)
	}
	return r.Status, f
}

// noMetadata fails the test when b carries any canary of the test metadata.
func noMetadata(t *testing.T, what string, b []byte) {
	t.Helper()
	for _, c := range []string{canaryGPS, canaryXMP, canaryComment, canaryIPTC, "xmpmeta", "Photoshop"} {
		if bytes.Contains(b, []byte(c)) {
			t.Errorf("%s carries %q", what, c)
		}
	}
}

// logLine reports whether a captured log line names file id with the error class class.
func (e *env) logLine(id, class string) bool {
	for _, l := range strings.Split(e.logs.String(), "\n") {
		if strings.Contains(l, id) && strings.Contains(l, class) {
			return true
		}
	}
	return false
}
