// SPDX-License-Identifier: Apache-2.0

// Package filecleanup_test holds the integration tests of the cleanup and deletion of files of
// brief M3-08 (section "Tests", Integration): the real server of internal/wire in role all through
// test/harness on a fake clock, PostgreSQL 18 from pgtest, the file system driver in the temporary
// root of the harness, the matrix world of 07 sections 10 and 11.1, uploads through the real HTTP
// chain and a scripted clamd. The real handlers of files.scan, files.purge_objects and
// files.cleanup_pending are kept for the test and River runs no-ops in their place, so the test
// decides when a scan, a purge or a cleanup run happens and moves the clock between them.
package filecleanup_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
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
	filesjobs "github.com/OWNER/bothy/services/server/internal/files/adapters/jobs"
	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// The windows of the test configuration: small, and the cleanup interval below the delete delay
// (cross-check object_delete_delay_gc).
const (
	partBytes       = 16384
	unattachedTTL   = time.Hour
	failedRowTTL    = 2 * time.Hour
	sessionTTL      = 30 * time.Minute
	objectDelDelay  = 3 * time.Hour
	gcInterval      = time.Minute
	eicarReply      = "stream: Eicar-Test-Signature FOUND"
	maxServersAtOne = 3
)

// servers bounds the harness servers of this package that run at the same time: every server
// opens its own pools on the one pgtest container.
var servers = make(chan struct{}, maxServersAtOne)

// world is one test server with the matrix world on the fake clock.
type world struct {
	t     *testing.T
	s     *harness.Server
	clk   *clock.Fake
	db    *pgxpool.Pool
	clamd *clamdtest.Fake
	// logs holds the JSON log lines of the server; a failed test prints its warnings and errors.
	logs *serverLog

	mu      sync.Mutex
	scan    filesjobs.Runner
	purge   filesjobs.PurgeRunner
	cleanup filesjobs.CleanupRunner

	owner, alex, sam, robin *harness.Client
	allg                    string
	main                    string
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

// lines returns the log lines that contain one of the needles.
func (l *serverLog) lines(needles ...string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, ln := range strings.Split(l.buf.String(), "\n") {
		for _, n := range needles {
			if strings.Contains(ln, n) {
				out = append(out, ln)
				break
			}
		}
	}
	return strings.Join(out, "\n")
}

func seconds(d time.Duration) string { return strconv.FormatInt(int64(d/time.Second), 10) + "s" }

// newWorld starts the server with the windows of this package and builds the world.
func newWorld(t *testing.T) *world {
	t.Helper()
	servers <- struct{}{}
	t.Cleanup(func() { <-servers })
	fake, err := clamdtest.NewFake()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fake.Close() })
	cfg := harness.RaisedRateLimits()
	for k, v := range map[string]string{
		"auth.clock_leeway":             "48h",
		"auth.dpop_proof_max_age":       "48h",
		"file.upload_part_bytes":        strconv.Itoa(partBytes),
		"file.max_upload_bytes":         "1048576",
		"file.upload_max_open_sessions": "1000",
		"file.upload_session_ttl":       seconds(sessionTTL),
		"file.unattached_ttl":           seconds(unattachedTTL),
		"file.failed_row_ttl":           seconds(failedRowTTL),
		"file.gc_interval":              seconds(gcInterval),
		"file.gc_batch_size":            "2",
		"privacy.object_delete_delay":   seconds(objectDelDelay),
		"file.scan.clamd_address":       fake.Address(),
		"file.scan.health_interval":     "1s",
	} {
		cfg[k] = v
	}
	w := &world{t: t, clk: clock.NewFake(time.Now().UTC().Truncate(time.Millisecond)), clamd: fake, logs: &serverLog{}}
	w.s = harness.Start(t, harness.Options{Config: cfg, Clock: w.clk, EgressDenyAll: true, JobModules: w.takeJobs, LogWriter: w.logs})
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("server warnings and errors:\n%s", w.logs.lines(`"level":"WARN"`, `"level":"ERROR"`))
		}
	})
	w.db = w.s.AdminDB()
	mw := harness.BuildMatrixWorld(t, w.s)
	w.owner, w.alex, w.sam, w.robin = w.s.Owner, mw.People["Alex"], mw.People["Sam"], mw.People["Robin"]
	w.allg = mw.Channels["Support/Allgemein"]
	if err := w.db.QueryRow(t.Context(), `SELECT id::text FROM threads WHERE channel_id = $1 AND kind = 'main'`, w.allg).Scan(&w.main); err != nil {
		t.Fatal(err)
	}
	return w
}

// takeJobs keeps the real handlers of files.scan, files.purge_objects and files.cleanup_pending
// and gives River no-ops in their place.
func (w *world) takeJobs(m *wire.JobModules) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.scan, w.purge, w.cleanup = m.FilesScan, m.FilesPurge, m.FilesCleanup
	m.FilesScan, m.FilesPurge, m.FilesCleanup = noScan{}, noPurge{}, noCleanup{}
}

type (
	noScan    struct{}
	noPurge   struct{}
	noCleanup struct{}
)

func (noScan) Run(context.Context, ids.ID, int64) (filesapp.PassResult, error) {
	return filesapp.PassResult{Outcome: filesapp.OutcomeNone}, nil
}

func (noPurge) Run(context.Context, ids.ID) error { return nil }

func (noCleanup) Run(context.Context) error { return nil }

func sha(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
}

// open creates an upload of content named name as c in Support/Allgemein and sends its parts,
// declaring declared; it returns the file id and the part list for completeUpload.
func (w *world) open(c *harness.Client, name string, content, declared []byte) (string, []map[string]any) {
	w.t.Helper()
	r := c.Do(w.t.Context(), harness.Request{Method: http.MethodPost, Path: "/channels/" + w.allg + "/uploads",
		Body: map[string]any{"name": name, "size_bytes": len(declared), "sha256": sha(declared), "declared_mime_type": "text/plain"}})
	if r.Status != http.StatusCreated {
		w.t.Fatalf("createUpload: %d %s", r.Status, r.Body)
	}
	var s struct {
		FileID string `json:"file_id"`
	}
	r.JSON(w.t, &s)
	var parts []map[string]any
	for n, lo := 1, 0; lo < len(content); n, lo = n+1, lo+partBytes {
		part := content[lo:min(lo+partBytes, len(content))]
		pr := w.part(c, s.FileID, n, part)
		if pr.Status != http.StatusOK {
			w.t.Fatalf("uploadPart %d: %d %s", n, pr.Status, pr.Body)
		}
		var out struct {
			ETag string `json:"etag"`
		}
		pr.JSON(w.t, &out)
		parts = append(parts, map[string]any{"part_number": n, "etag": out.ETag})
	}
	return s.FileID, parts
}

// part sends part n of an upload as c.
func (w *world) part(c *harness.Client, fileID string, n int, part []byte) harness.Response {
	h := http.Header{}
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Digest", digest(part))
	return c.Do(w.t.Context(), harness.Request{Method: http.MethodPut, Path: "/uploads/" + fileID + "/parts/" + strconv.Itoa(n), Body: part, Header: h})
}

// upload uploads and completes content as c; the file is pending until the test scans it.
func (w *world) upload(c *harness.Client, name string, content, declared []byte) string {
	w.t.Helper()
	id, parts := w.open(c, name, content, declared)
	harness.MustDo(w.t, c, harness.Request{Method: http.MethodPost, Path: "/uploads/" + id + "/complete", Body: map[string]any{"parts": parts}},
		http.StatusOK)
	return id
}

// clean uploads a text file as c and scans it clean.
func (w *world) clean(c *harness.Client, name string) string {
	w.t.Helper()
	content := []byte("text of " + name + " for the support team")
	id := w.upload(c, name, content, content)
	w.mustScan(id)
	if up, sc := w.fileState(id); up != "complete" || sc != "clean" {
		w.t.Fatalf("file %s: %s/%s", id, up, sc)
	}
	return id
}

// mustScan runs the handler of files.scan once for the file with the next round.
func (w *world) mustScan(fileID string) {
	w.t.Helper()
	w.mu.Lock()
	pass := w.scan
	w.mu.Unlock()
	round := w.count(`SELECT coalesce(max((args->>'round')::bigint), 0) FROM jobs.river_job WHERE kind = 'files.scan' AND args->>'file_id' = $1`, fileID)
	if err := (filesjobs.Handler{Pass: pass}).Work(context.Background(), jobs.Job[filesjobs.Scan]{Attempt: 1,
		Args: filesjobs.Scan{FileID: ids.MustParse(fileID), Round: int64(round)}}); err != nil {
		w.t.Fatalf("files.scan %s: %v", fileID, err)
	}
}

// purgeFile runs the handler of files.purge_objects for the file.
func (w *world) purgeFile(fileID string) error {
	w.mu.Lock()
	p := w.purge
	w.mu.Unlock()
	if p == nil {
		return errors.New("the purge of the server is not built")
	}
	return filesjobs.PurgeHandler{Purge: p}.Work(context.Background(), jobs.Job[filesjobs.PurgeObjects]{Attempt: 1,
		Args: filesjobs.PurgeObjects{FileID: ids.MustParse(fileID)}})
}

// mustPurge is purgeFile that fails the test on an error.
func (w *world) mustPurge(fileID string) {
	w.t.Helper()
	if err := w.purgeFile(fileID); err != nil {
		w.t.Fatalf("files.purge_objects %s: %v", fileID, err)
	}
}

// purgeQueued runs the handler of every files.purge_objects job of the file in the River table.
func (w *world) purgeQueued(fileID string) {
	w.t.Helper()
	n := w.count(`SELECT count(*) FROM jobs.river_job WHERE kind = 'files.purge_objects' AND args->>'file_id' = $1`, fileID)
	if n == 0 {
		w.t.Fatalf("no files.purge_objects job for %s", fileID)
	}
	for range n {
		w.mustPurge(fileID)
	}
}

// runCleanup runs the handler of files.cleanup_pending once.
func (w *world) runCleanup() error {
	w.mu.Lock()
	c := w.cleanup
	w.mu.Unlock()
	return filesjobs.CleanupHandler{Cleanup: c}.Work(context.Background(), jobs.Job[filesjobs.CleanupPending]{Attempt: 1})
}

// mustCleanup is runCleanup that fails the test on an error.
func (w *world) mustCleanup() {
	w.t.Helper()
	if err := w.runCleanup(); err != nil {
		w.t.Fatalf("files.cleanup_pending: %v", err)
	}
}

// count runs a count query on the admin pool.
func (w *world) count(q string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.db.QueryRow(w.t.Context(), q, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", q, err)
	}
	return n
}

// exec runs a statement on the admin pool.
func (w *world) exec(q string, args ...any) {
	w.t.Helper()
	if _, err := w.db.Exec(w.t.Context(), q, args...); err != nil {
		w.t.Fatalf("%s: %v", q, err)
	}
}

// fileState reads upload_state and scan_status of a file.
func (w *world) fileState(fileID string) (upload, scan string) {
	w.t.Helper()
	if err := w.db.QueryRow(w.t.Context(), `SELECT upload_state, scan_status FROM files WHERE id = $1`, fileID).Scan(&upload, &scan); err != nil {
		w.t.Fatal(err)
	}
	return upload, scan
}

// rowExists reports a files row of the id.
func (w *world) rowExists(fileID string) bool {
	return w.count(`SELECT count(*) FROM files WHERE id = $1`, fileID) == 1
}

// deleted reports deleted_at of the row.
func (w *world) deleted(fileID string) bool {
	return w.count(`SELECT count(*) FROM files WHERE id = $1 AND deleted_at IS NOT NULL`, fileID) == 1
}

// gone reports a file that is marked deleted or whose row the purge removed already (the
// catch-up of the same cleanup run purges a file the run just marked).
func (w *world) gone(fileID string) bool { return w.deleted(fileID) || !w.rowExists(fileID) }

// objectsLeft returns the regular files left in the key space of the file: its directory under
// f/ and its part directory under uploads/.
func (w *world) objectsLeft(fileID string) []string {
	w.t.Helper()
	id := ids.MustParse(fileID)
	key, err := domain.OriginalKey(id, 1)
	if err != nil {
		w.t.Fatal(err)
	}
	part, err := domain.UploadPartKey(id, 1)
	if err != nil {
		w.t.Fatal(err)
	}
	var out []string
	for _, dir := range []string{filepath.Dir(filepath.FromSlash(key)), filepath.Dir(filepath.FromSlash(part))} {
		entries, err := os.ReadDir(filepath.Join(w.s.FilesRoot, dir))
		if err != nil {
			continue
		}
		for _, e := range entries {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

// original reports whether orig-1 of the file exists in the root.
func (w *world) original(fileID string) bool {
	key, err := domain.OriginalKey(ids.MustParse(fileID), 1)
	if err != nil {
		w.t.Fatal(err)
	}
	_, err = os.Stat(filepath.Join(w.s.FilesRoot, filepath.FromSlash(key)))
	return err == nil
}

// getFile calls getFile as c and returns the status.
func (w *world) getFile(c *harness.Client, fileID string) int {
	return c.Do(w.t.Context(), harness.Request{Method: http.MethodGet, Path: "/files/" + fileID}).Status
}

// deleteFile calls deleteFile as c.
func (w *world) deleteFile(c *harness.Client, fileID string) harness.Response {
	return c.Do(w.t.Context(), harness.Request{Method: http.MethodDelete, Path: "/files/" + fileID})
}

// send posts a message as c into the main thread of Support/Allgemein with the attachments.
func (w *world) send(c *harness.Client, text string, fileIDs ...string) harness.Response {
	body := map[string]any{"client_msg_id": ids.New().String(), "content_md": text}
	if len(fileIDs) > 0 {
		body["attachment_file_ids"] = fileIDs
	}
	return c.Do(w.t.Context(), harness.Request{Method: http.MethodPost, Path: "/threads/" + w.main + "/messages", Body: body})
}

// mustSend is send that requires 201 and returns the message id.
func (w *world) mustSend(c *harness.Client, text string, fileIDs ...string) string {
	w.t.Helper()
	r := w.send(c, text, fileIDs...)
	if r.Status != http.StatusCreated {
		w.t.Fatalf("createMessage: %d %s", r.Status, r.Body)
	}
	var m struct {
		ID string `json:"id"`
	}
	r.JSON(w.t, &m)
	return m.ID
}

// problem reads code and reason of a problem body.
func problem(t *testing.T, r harness.Response) (code, reason string) {
	t.Helper()
	var p struct {
		Code   string `json:"code"`
		Reason string `json:"reason"`
	}
	r.JSON(t, &p)
	return p.Code, p.Reason
}

// wantProblem requires status, code and reason of a response.
func wantProblem(t *testing.T, r harness.Response, status int, code, reason string) {
	t.Helper()
	if r.Status != status {
		t.Fatalf("status %d, want %d: %s", r.Status, status, r.Body)
	}
	if c, rs := problem(t, r); c != code || rs != reason {
		t.Fatalf("problem %s/%s, want %s/%s: %s", c, rs, code, reason, r.Body)
	}
}

// times describes the stored times of a file for a failure message.
func (w *world) times(fileID string) string {
	w.t.Helper()
	var out string
	if err := w.db.QueryRow(w.t.Context(), `SELECT concat_ws(' ', 'created', f.created_at, 'scanned', f.scanned_at, 'completed', s.completed_at,
		'state', f.upload_state, f.scan_status, f.purpose) FROM files f LEFT JOIN file_upload_sessions s ON s.file_id = f.id WHERE f.id = $1`, fileID).Scan(&out); err != nil {
		return err.Error()
	}
	return out
}
