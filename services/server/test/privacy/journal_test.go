// SPDX-License-Identifier: Apache-2.0

package privacy_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	privacyjobs "github.com/OWNER/bothy/services/server/internal/privacy/adapters/jobs"
	privacyapp "github.com/OWNER/bothy/services/server/internal/privacy/app"
	privacydomain "github.com/OWNER/bothy/services/server/internal/privacy/domain"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// journalServers bounds the servers of the journal tests that run at the same time: each opens its
// own pools on the one pgtest container.
var journalServers = make(chan struct{}, 3)

// journalWorld is a server of the matrix world behind a switch proxy, so it stops and starts again
// on the same address (a backup, a restore and bothy restore finalize run while it is stopped).
// River never runs the privacy jobs: the test runs the deletion itself. It reads the database
// through short superuser connections, because a restore ends every session of the database.
type journalWorld struct {
	t   *testing.T
	s   *harness.Server
	clk *clock.Fake
	w   *harness.World
	// logs holds the JSON log lines of the server.
	logs *lineLog

	mu        sync.Mutex
	jobs      privacyjobs.UseCases
	retention *retention.Registry

	owner, alex *harness.Client
	channel     string
	thread      string
}

// lineLog keeps the log lines of the server; it is safe for concurrent use.
type lineLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write implements io.Writer.
func (l *lineLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

// has reports whether a log line contains needle.
func (l *lineLog) has(needle string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Contains(l.buf.String(), needle)
}

// journalProxy is a reverse proxy on 127.0.0.1 port 0 whose upstream follows the listener of the
// current server generation.
type journalProxy struct {
	ln net.Listener
	mu sync.Mutex
	up *url.URL
}

// newJournalProxy starts the proxy and stops it when the test ends.
func newJournalProxy(t *testing.T) *journalProxy {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &journalProxy{ln: ln}
	rp := &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
		p.mu.Lock()
		up := p.up
		p.mu.Unlock()
		r.SetURL(up)
		r.Out.Host = r.In.Host
	}}
	srv := &http.Server{Handler: rp, ReadHeaderTimeout: time.Minute}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return p
}

func (p *journalProxy) origin() string { return "http://" + p.ln.Addr().String() }

func (p *journalProxy) set(upstream string) {
	u, _ := url.Parse(upstream)
	p.mu.Lock()
	p.up = u
	p.mu.Unlock()
}

// newJournalWorld starts the server with the content mode mode and the extra keys cfg and builds
// the matrix world.
func newJournalWorld(t *testing.T, mode string, cfg map[string]string) *journalWorld {
	t.Helper()
	journalServers <- struct{}{}
	t.Cleanup(func() { <-journalServers })
	all := harness.RaisedRateLimits()
	all["privacy.account_deletion_content"] = mode
	// Uploads complete without a virus scanner: the attachment is only the file of a message here.
	all["file.scan.mode"] = "off"
	// The clock moves by a day at most; the clients sign their proofs with the real time.
	all["auth.clock_leeway"], all["auth.dpop_proof_max_age"] = "96h", "96h"
	for k, v := range cfg {
		all[k] = v
	}
	p := newJournalProxy(t)
	w := &journalWorld{t: t, clk: clock.NewFake(time.Now().UTC().Truncate(time.Millisecond)), logs: &lineLog{}}
	w.s = harness.Start(t, harness.Options{Config: all, Clock: w.clk, EgressDenyAll: true, JobModules: w.take, LogWriter: w.logs,
		PublicURL: p.origin(), OnListen: p.set})
	w.w = harness.BuildMatrixWorld(t, w.s)
	w.owner, w.alex = w.s.Owner, w.w.People["Alex"]
	w.channel, w.thread = w.w.Channels["Support/Allgemein"], w.w.Threads["Support/Allgemein"]
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("server log:\n%s", w.logs.buf.String())
		}
	})
	return w
}

// take keeps the use cases of the privacy jobs and the participants of the retention jobs, and
// gives River no-ops for the privacy jobs.
func (w *journalWorld) take(m *wire.JobModules) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.jobs, w.retention = m.Privacy, m.RetentionParticipants
	m.Privacy = noJobs{}
}

// conn opens a superuser connection to the database of the server for one query.
func (w *journalWorld) conn() *pgx.Conn {
	w.t.Helper()
	cfg := w.s.DB().SuperConfig()
	pass, err := os.ReadFile(filepath.Join(w.s.DB().SecretsDir(), "db_superuser_password"))
	if err != nil {
		w.t.Fatal(err)
	}
	cc, err := pgx.ParseConfig("sslmode=disable")
	if err != nil {
		w.t.Fatal(err)
	}
	cc.Host, cc.Port, cc.Database, cc.User, cc.Password = cfg.Host, uint16(cfg.Port), cfg.Name, cfg.User, strings.TrimSpace(string(pass)) // #nosec G115 -- a TCP port
	c, err := pgx.ConnectConfig(w.t.Context(), cc)
	if err != nil {
		w.t.Fatal(err)
	}
	return c
}

// text runs a query that returns one value as text.
func (w *journalWorld) text(q string, args ...any) string {
	w.t.Helper()
	c := w.conn()
	defer func() { _ = c.Close(context.Background()) }()
	var v string
	if err := c.QueryRow(w.t.Context(), q, args...).Scan(&v); err != nil {
		w.t.Fatalf("%s: %v", q, err)
	}
	return v
}

// num runs a query that returns one number.
func (w *journalWorld) num(q string, args ...any) int64 {
	w.t.Helper()
	c := w.conn()
	defer func() { _ = c.Close(context.Background()) }()
	var v int64
	if err := c.QueryRow(w.t.Context(), q, args...).Scan(&v); err != nil {
		w.t.Fatalf("%s: %v", q, err)
	}
	return v
}

// exec runs a statement as the superuser.
func (w *journalWorld) exec(q string, args ...any) {
	w.t.Helper()
	c := w.conn()
	defer func() { _ = c.Close(context.Background()) }()
	if _, err := c.Exec(w.t.Context(), q, args...); err != nil {
		w.t.Fatalf("%s: %v", q, err)
	}
}

// head is the highest seq of the log.
func (w *journalWorld) head() int64 { return w.num(`SELECT COALESCE(max(seq), 0) FROM event_log`) }

// cursor is last_seq of the consumer privacy.journal.
func (w *journalWorld) cursor() int64 {
	return w.num(`SELECT COALESCE((SELECT last_seq FROM event_consumer_cursors WHERE name = 'privacy.journal'), -1)`)
}

// caughtUp waits until the consumer privacy.journal has read every event of the log.
func (w *journalWorld) caughtUp() {
	w.t.Helper()
	testkit.Until(w.t, "the consumer privacy.journal at the head", func() bool {
		h := w.head()
		return w.cursor() >= h
	})
}

// nextNumber is deletion_journal_state.next_number.
func (w *journalWorld) nextNumber() int64 {
	return w.num(`SELECT next_number FROM deletion_journal_state`)
}

// objectPath is the path of journal object n under the root of the file system driver.
func (w *journalWorld) objectPath(n int64) string {
	w.t.Helper()
	key, err := privacydomain.JournalKey(n)
	if err != nil {
		w.t.Fatal(err)
	}
	return filepath.Join(w.s.FilesRoot, filepath.FromSlash(key))
}

// object reads journal object n; found is false when it does not exist.
func (w *journalWorld) object(n int64) ([]byte, bool) {
	w.t.Helper()
	b, err := os.ReadFile(w.objectPath(n))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false
	}
	if err != nil {
		w.t.Fatal(err)
	}
	return b, true
}

// entry reads and parses journal object n; it must exist.
func (w *journalWorld) entry(n int64) privacydomain.Entry {
	w.t.Helper()
	b, ok := w.object(n)
	if !ok {
		w.t.Fatalf("journal object %d is missing", n)
	}
	e, err := privacydomain.ParseEntry(b)
	if err != nil {
		w.t.Fatalf("journal object %d: %v", n, err)
	}
	return e
}

// post posts text as c into the thread of Support/Allgemein with the attachments files and returns
// the message id.
func (w *journalWorld) post(c *harness.Client, text string, files ...string) string {
	w.t.Helper()
	body := map[string]any{"client_msg_id": ids.New().String(), "content_md": text}
	if len(files) > 0 {
		body["attachment_file_ids"] = files
	}
	var m struct {
		ID string `json:"id"`
	}
	harness.MustDo(w.t, c, harness.Request{Method: http.MethodPost, Path: "/threads/" + w.thread + "/messages", Body: body},
		http.StatusCreated).JSON(w.t, &m)
	return m.ID
}

// upload uploads a small text file as c into Support/Allgemein and returns its id (no scan: the
// world runs with file.scan.mode off).
func (w *journalWorld) upload(c *harness.Client, name string) string {
	w.t.Helper()
	content := []byte("notes of " + name + " for the support team")
	sum := sha256.Sum256(content)
	var s struct {
		FileID string `json:"file_id"`
	}
	harness.MustDo(w.t, c, harness.Request{Method: http.MethodPost, Path: "/channels/" + w.channel + "/uploads",
		Body: map[string]any{"name": name, "size_bytes": len(content), "sha256": hex.EncodeToString(sum[:]), "declared_mime_type": "text/plain"}},
		http.StatusCreated).JSON(w.t, &s)
	h := http.Header{}
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Digest", "sha-256=:"+base64.StdEncoding.EncodeToString(sum[:])+":")
	var part struct {
		ETag string `json:"etag"`
	}
	harness.MustDo(w.t, c, harness.Request{Method: http.MethodPut, Path: "/uploads/" + s.FileID + "/parts/1", Body: content, Header: h},
		http.StatusOK).JSON(w.t, &part)
	harness.MustDo(w.t, c, harness.Request{Method: http.MethodPost, Path: "/uploads/" + s.FileID + "/complete",
		Body: map[string]any{"parts": []map[string]any{{"part_number": 1, "etag": part.ETag}}}}, http.StatusOK)
	return s.FileID
}

// deleteMessage deletes the message id as c.
func (w *journalWorld) deleteMessage(c *harness.Client, id string) {
	w.t.Helper()
	harness.MustDo(w.t, c, harness.Request{Method: http.MethodDelete, Path: "/messages/" + id}, http.StatusNoContent)
}

// deleteAccount files the deletion of the account of c, lets the owner approve it and runs the
// job privacy.erase; it returns the request id.
func (w *journalWorld) deleteAccount(c *harness.Client) string {
	w.t.Helper()
	r := create(w.t, c, map[string]any{"kind": "deletion", "reason": "Delete my account"})
	harness.MustDo(w.t, w.owner, harness.Request{Method: http.MethodPost, Path: "/gdpr-requests/" + r.ID + "/decide",
		Body: map[string]any{"decision": "approve", "reason": "No retention duty"}}, http.StatusOK)
	w.mu.Lock()
	uc := w.jobs
	w.mu.Unlock()
	if err := uc.RunErase(w.t.Context(), ids.MustParse(r.ID)); err != nil {
		w.t.Fatalf("privacy.erase: %v", err)
	}
	return r.ID
}

// stopped stops the server, runs fn while it is stopped and starts it again.
func (w *journalWorld) stopped(fn func()) {
	w.t.Helper()
	w.s.Restart(w.t, harness.RestartOptions{WhileStopped: fn})
}

// journalRetention returns the participant of the journal objects in retention.privacy_job.
func (w *journalWorld) journalRetention() retention.Participant {
	w.t.Helper()
	w.mu.Lock()
	reg := w.retention
	w.mu.Unlock()
	for _, r := range reg.Of(retention.ClassPrivacyJob) {
		if _, ok := r.Participant.(*privacyapp.JournalRetention); ok {
			return r.Participant
		}
	}
	w.t.Fatal("no participant of the journal objects in retention.privacy_job")
	return nil
}

// TestJournalFromEvents (brief M7-07, 20 4.6): the deletion of a message, of a message with an
// attachment and of an account writes one content-free entry each, numbered densely in seq order
// outside the database, and the account entry names the content mode and marks its request; a
// failing storage leaves cursor and number where they were and the consumer retries; an entry
// stored before a crash, before the cursor moved, is found again and counts as stored. The
// migration accepts the cursor row privacy.journal.
func TestJournalFromEvents(t *testing.T) {
	t.Parallel()
	w := newJournalWorld(t, "delete", nil)
	if got := w.text(`SELECT name FROM event_consumer_cursors WHERE name = 'privacy.journal'`); got != "privacy.journal" {
		t.Fatalf("cursor row %q", got)
	}
	start := w.nextNumber()
	m1 := w.post(w.alex, "first note to delete")
	file := w.upload(w.alex, "notes.txt")
	m2 := w.post(w.alex, "a note with a file", file)
	// The window between the commit of a deletion and its journal object (reported, not bound:
	// the consumer lag of the runbook).
	began := time.Now()
	w.deleteMessage(w.alex, m1)
	testkit.Until(t, "the journal object of the first deletion", func() bool { _, ok := w.object(start); return ok })
	t.Logf("window between the deletion and its journal object: %v", time.Since(began))
	w.deleteMessage(w.alex, m2)
	w.caughtUp()
	if got := w.nextNumber(); got != start+2 {
		t.Fatalf("next_number %d after two deletions from %d", got, start)
	}
	e1, e2 := w.entry(start), w.entry(start+1)
	if e1.ResourceKind != privacydomain.ResourceMessage || e1.ResourceID.String() != m1 || e1.Reason != privacydomain.ReasonDeleted ||
		e2.ResourceKind != privacydomain.ResourceMessage || e2.ResourceID.String() != m2 || e1.Seq >= e2.Seq {
		t.Fatalf("entries %+v, %+v for %s and %s", e1, e2, m1, m2)
	}
	for _, n := range []int64{start, start + 1} {
		b, _ := w.object(n)
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil || len(m) != 5 || strings.Contains(string(b), "note") {
			t.Fatalf("object %d is not a content-free entry: %s", n, b)
		}
	}

	// A failing storage: a directory in the place of the next object makes Get and Put fail.
	blocker := w.objectPath(start + 2)
	if err := os.MkdirAll(filepath.Join(blocker, "x"), 0o700); err != nil {
		t.Fatal(err)
	}
	m3 := w.post(w.alex, "third note to delete")
	before := w.cursor()
	w.deleteMessage(w.alex, m3)
	failedSeq := w.head()
	testkit.Until(t, "a failed round of the consumer", func() bool { return w.logs.has("event consumer round failed") })
	if c, n := w.cursor(), w.nextNumber(); c >= failedSeq || c < before || n != start+2 {
		t.Fatalf("after the failure: cursor %d (event %d), next_number %d", c, failedSeq, n)
	}
	if err := os.RemoveAll(blocker); err != nil {
		t.Fatal(err)
	}
	w.caughtUp()
	if e := w.entry(start + 2); e.ResourceID.String() != m3 || e.Seq != failedSeq {
		t.Fatalf("entry after the retry %+v, want %s at seq %d", e, m3, failedSeq)
	}

	// A crash after the object was stored and before the commit: cursor and number back.
	stored, _ := w.object(start + 2)
	w.stopped(func() {
		w.exec(`UPDATE event_consumer_cursors SET last_seq = $1 WHERE name = 'privacy.journal'`, failedSeq-1)
		w.exec(`UPDATE deletion_journal_state SET next_number = $1`, start+2)
	})
	w.caughtUp()
	if got, _ := w.object(start + 2); !bytes.Equal(got, stored) || w.nextNumber() != start+3 {
		t.Fatalf("after the crash: object %s (was %s), next_number %d", got, stored, w.nextNumber())
	}
	if _, ok := w.object(start + 3); ok {
		t.Fatal("the redone round wrote a second object")
	}

	// The account: one entry with the content mode, the request marked.
	req := w.deleteAccount(w.alex)
	w.caughtUp()
	last := w.nextNumber() - 1
	acc := w.entry(last)
	if acc.ResourceKind != privacydomain.ResourceAccount || acc.ResourceID.String() != w.alex.PrincipalID || acc.Reason != "delete" {
		t.Fatalf("account entry %+v", acc)
	}
	for n := start + 3; n < last; n++ {
		if e := w.entry(n); e.ResourceKind != privacydomain.ResourceMessage || e.Seq >= acc.Seq {
			t.Fatalf("entry %d %+v before the account entry %+v", n, e, acc)
		}
	}
	if got := w.text(`SELECT COALESCE(progress ->> 'journaled', 'false') FROM gdpr_requests WHERE id = $1`, req); got != "true" {
		t.Fatalf("request journaled = %s", got)
	}
}

// restoreScenario is the backup and restore of SEC-T15 and DP-T07 on a world with the content
// mode mode: Alex posts two notes (one with a file) and a third, the database is copied (the
// backup), then the two notes and the account of Alex are deleted, the consumer journals them,
// the copy is restored and bothy restore finalize runs. hook, when set, runs right before the
// restore while the server is stopped.
type restoreScenario struct {
	w                    *journalWorld
	notes                []string
	file, kept           string
	snapshotNext         int64
	journalEnd           int64
	permissionAtBackup   int64
	res                  wire.RestoreResult
	objectsBeforeRestore map[int64][]byte
}

// runRestore runs the scenario; before runs after the deletions with the server running, and
// whileStopped right before the restore.
func runRestore(t *testing.T, w *journalWorld, before func(sc *restoreScenario)) *restoreScenario {
	t.Helper()
	sc := &restoreScenario{w: w}
	sc.file = w.upload(w.alex, "report.txt")
	sc.notes = []string{w.post(w.alex, "first note"), w.post(w.alex, "note with a file", sc.file)}
	sc.kept = w.post(w.alex, "a note of the history")
	w.caughtUp()
	var restore func() error
	w.stopped(func() {
		sc.snapshotNext = w.nextNumber()
		sc.permissionAtBackup = w.num(`SELECT value FROM sequence_counters WHERE name = 'permission_version'`)
		restore = w.s.DB().Snapshot(t)
	})
	for _, m := range sc.notes {
		w.deleteMessage(w.alex, m)
	}
	w.deleteAccount(w.alex)
	w.caughtUp()
	sc.journalEnd = w.nextNumber()
	if before != nil {
		before(sc)
	}
	sc.objectsBeforeRestore = map[int64][]byte{}
	for n := sc.snapshotNext; n < sc.journalEnd; n++ {
		b, ok := w.object(n)
		if !ok {
			t.Fatalf("journal object %d missing before the restore", n)
		}
		sc.objectsBeforeRestore[n] = b
	}
	w.stopped(func() {
		if err := restore(); err != nil {
			t.Fatal(err)
		}
		// The restored database holds the notes, the file and the account again.
		if got := w.num(`SELECT count(*) FROM messages WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL`, sc.notes); got != 2 {
			t.Fatalf("the restored database holds %d of the notes", got)
		}
		res, err := w.s.FinalizeRestore(t.Context())
		if err != nil {
			t.Fatalf("bothy restore finalize: %v", err)
		}
		sc.res = res
	})
	return sc
}

// assertDeletedAgain checks that the notes, the file and the account are deleted after the
// restore, with the content of the kept note as the mode says, and the counts of the result.
func (sc *restoreScenario) assertDeletedAgain(t *testing.T, mode string) {
	t.Helper()
	w := sc.w
	if got := w.num(`SELECT count(*) FROM messages WHERE id = ANY($1::uuid[]) AND deleted_at IS NOT NULL AND content_md = ''`, sc.notes); got != 2 {
		t.Fatalf("%d of the two notes are deleted again", got)
	}
	if got := w.num(`SELECT count(*) FROM files WHERE id = $1 AND deleted_at IS NULL`, sc.file); got != 0 {
		t.Fatal("the attachment of the note is back")
	}
	if got := w.text(`SELECT status FROM principals WHERE id = $1`, w.alex.PrincipalID); got != "deleted" {
		t.Fatalf("Alex is %s after the restore", got)
	}
	if got := w.num(`SELECT count(*) FROM users WHERE principal_id = $1 AND email IS NOT NULL`, w.alex.PrincipalID); got != 0 {
		t.Fatal("the e-mail address of Alex is back")
	}
	keptDeleted := w.num(`SELECT count(*) FROM messages WHERE id = $1 AND deleted_at IS NOT NULL`, sc.kept)
	if want := map[string]int64{"delete": 1, "keep_pseudonymized": 0}[mode]; keptDeleted != want {
		t.Fatalf("mode %s: the kept note deleted = %d, want %d", mode, keptDeleted, want)
	}
	j := sc.res.Journal
	t.Logf("mode %s: journal objects %d to %d, result %+v, permission version %d -> %d", mode, sc.snapshotNext, sc.journalEnd-1, j,
		sc.permissionAtBackup, sc.res.PermissionVersion)
	if j.Entries != int(sc.journalEnd-sc.snapshotNext) || j.Lost != 0 || j.Accounts != 1 || j.Messages < 2 || j.Refused != 0 || j.Next != sc.journalEnd {
		t.Fatalf("journal result %+v, objects %d to %d", j, sc.snapshotNext, sc.journalEnd)
	}
	if sc.res.PermissionVersion <= sc.permissionAtBackup || sc.res.SyncEpoch == sc.res.PreviousSyncEpoch {
		t.Fatalf("permission version %d (backup %d), epoch %s -> %s", sc.res.PermissionVersion, sc.permissionAtBackup,
			sc.res.PreviousSyncEpoch, sc.res.SyncEpoch)
	}
	if got := w.num(`SELECT value FROM sequence_counters WHERE name = 'permission_version'`); got < sc.res.PermissionVersion {
		t.Fatalf("stored permission version %d below the result %d", got, sc.res.PermissionVersion)
	}
}

// TestSECT15_RestoreKeepsDeleted is SEC-T15 and DP-T07 for both content modes: after a restore of
// a backup taken before the deletions, bothy restore finalize deletes the notes, the attachment
// and the account again with the recorded mode, raises the permission version and starts a new
// epoch.
func TestSECT15_RestoreKeepsDeleted(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"keep_pseudonymized", "delete"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			w := newJournalWorld(t, mode, nil)
			sc := runRestore(t, w, nil)
			sc.assertDeletedAgain(t, mode)
		})
	}
}

// TestApplyIsIdempotent: a second application of the same journal objects to the finalized
// database deletes nothing, counts every entry as skipped and writes no event.
func TestApplyIsIdempotent(t *testing.T) {
	t.Parallel()
	w := newJournalWorld(t, "keep_pseudonymized", nil)
	sc := runRestore(t, w, nil)
	sc.assertDeletedAgain(t, "keep_pseudonymized")
	var second wire.RestoreResult
	var headBefore, headAfter int64
	var rows, rowsAfter string
	state := func() string {
		return w.text(`SELECT string_agg(id::text || ':' || COALESCE(deleted_at::text, '-') || ':' || version::text, ',' ORDER BY id) FROM messages`) +
			"|" + w.text(`SELECT string_agg(p.id::text || ':' || p.status || ':' || p.handle, ',' ORDER BY p.id) FROM principals p`)
	}
	w.stopped(func() {
		// The same objects once more: next_number back to the restored value.
		w.exec(`UPDATE deletion_journal_state SET next_number = $1`, sc.snapshotNext)
		headBefore, rows = w.head(), state()
		var err error
		if second, err = w.s.FinalizeRestore(t.Context()); err != nil {
			t.Fatal(err)
		}
		headAfter, rowsAfter = w.head(), state()
	})
	j := second.Journal
	if j.Entries != sc.res.Journal.Entries || j.Messages != 0 || j.Files != 0 || j.Accounts != 0 || j.Skipped != j.Entries || j.Lost != 0 || j.Next != sc.journalEnd {
		t.Fatalf("second application %+v, first %+v", j, sc.res.Journal)
	}
	if headAfter != headBefore || rowsAfter != rows {
		t.Fatalf("the second application changed the database: head %d -> %d", headBefore, headAfter)
	}
}

// TestNumberAfterRestore (R-DM-52): after bothy restore finalize the next entry gets the number
// after the last object found and no object is written again; the retention removed the objects
// below the journal position of the kept backup although they are older than
// privacy.deletion_journal_retention, kept every object from it on although older too, and the
// restore still applied all of them.
func TestNumberAfterRestore(t *testing.T) {
	t.Parallel()
	const day = 24 * time.Hour
	w := newJournalWorld(t, "keep_pseudonymized", map[string]string{
		"privacy.deletion_journal_retention": "1d", "backup.logical.retention": "1d", "backup.objects.retention": "1d",
		"backup.pg.retention_full": "1", "backup.pg.local_retention_full": "1", "backup.pg.full_schedule": "0 2 * * *",
	})
	// One entry before the backup: the retention may remove it.
	old := w.post(w.alex, "an old note")
	w.deleteMessage(w.alex, old)
	w.caughtUp()
	sc := runRestore(t, w, func(sc *restoreScenario) {
		if sc.snapshotNext < 2 {
			t.Fatalf("no object below the backup position %d", sc.snapshotNext)
		}
		// The backup is kept: a succeeded logical run whose manifest names its position.
		manifest := fmt.Sprintf(`{"deletion_journal": {"next_number": %d}}`, sc.snapshotNext)
		w.exec(`INSERT INTO backup_runs (id, kind, state, started_at, finished_at, manifest) VALUES ($1, 'logical', 'succeeded', $2, $2, $3::jsonb)`,
			ids.New(), w.clk.Now().Add(day+time.Hour), manifest)
		w.clk.Advance(day + 2*time.Hour)
		p := w.journalRetention()
		removed, _, err := p.Purge(t.Context(), w.clk.Now().Add(-day), 100)
		if err != nil {
			t.Fatal(err)
		}
		if removed != int(sc.snapshotNext-1) {
			t.Fatalf("the retention removed %d objects, want the %d below the backup position", removed, sc.snapshotNext-1)
		}
		for n := int64(1); n < sc.snapshotNext; n++ {
			if _, ok := w.object(n); ok {
				t.Fatalf("object %d below the backup position stays", n)
			}
		}
		for n := sc.snapshotNext; n < sc.journalEnd; n++ {
			if _, ok := w.object(n); !ok {
				t.Fatalf("object %d of the kept backup was removed", n)
			}
		}
		if got := w.num(`SELECT low_number FROM deletion_journal_state`); got != sc.snapshotNext {
			t.Fatalf("low_number %d, want %d", got, sc.snapshotNext)
		}
	})
	sc.assertDeletedAgain(t, "keep_pseudonymized")
	if got := w.nextNumber(); got != sc.journalEnd {
		t.Fatalf("next_number %d after the finalize, want %d", got, sc.journalEnd)
	}
	o, err := w.s.OwnerSession(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	note := w.post(o, "a note after the restore")
	w.deleteMessage(o, note)
	w.caughtUp()
	if e := w.entry(sc.journalEnd); e.ResourceID.String() != note || w.nextNumber() != sc.journalEnd+1 {
		t.Fatalf("entry after the restore %+v, next_number %d", e, w.nextNumber())
	}
	for n, b := range sc.objectsBeforeRestore {
		if got, _ := w.object(n); !bytes.Equal(got, b) {
			t.Fatalf("object %d was written again", n)
		}
	}
}

// requestRetention returns the participant of the finished privacy requests in
// retention.privacy_job.
func (w *journalWorld) requestRetention() retention.Participant {
	w.t.Helper()
	w.mu.Lock()
	reg := w.retention
	w.mu.Unlock()
	for _, r := range reg.Of(retention.ClassPrivacyJob) {
		if slices.Contains(r.Participant.Tables(), "gdpr_requests") {
			return r.Participant
		}
	}
	w.t.Fatal("no participant of gdpr_requests in retention.privacy_job")
	return nil
}

// TestRequestKeptUntilJournaled (design note 1): while the consumer privacy.journal is held (its
// next object cannot be stored), retention.privacy_job keeps the finished deletion request even
// after its period, so the account entry still finds the content mode; once the consumer wrote the
// entry and marked the request, the next run removes it.
func TestRequestKeptUntilJournaled(t *testing.T) {
	t.Parallel()
	w := newJournalWorld(t, "keep_pseudonymized", nil)
	blocker := w.objectPath(w.nextNumber())
	if err := os.MkdirAll(filepath.Join(blocker, "x"), 0o700); err != nil {
		t.Fatal(err)
	}
	req := w.deleteAccount(w.alex)
	testkit.Until(t, "a failed round of the consumer", func() bool { return w.logs.has("event consumer round failed") })
	p := w.requestRetention()
	// A cutoff after the completion: the request is past retention.privacy_job.
	cutoff := w.clk.Now().Add(time.Hour)
	if _, _, err := p.Purge(t.Context(), cutoff, 100); err != nil {
		t.Fatal(err)
	}
	if n := w.num(`SELECT count(*) FROM gdpr_requests WHERE id = $1`, req); n != 1 {
		t.Fatal("the retention removed the deletion request before its journal entry")
	}
	if err := os.RemoveAll(blocker); err != nil {
		t.Fatal(err)
	}
	w.caughtUp()
	if got := w.text(`SELECT COALESCE(progress ->> 'journaled', 'false') FROM gdpr_requests WHERE id = $1`, req); got != "true" {
		t.Fatalf("request journaled = %s", got)
	}
	if e := w.entry(w.nextNumber() - 1); e.ResourceKind != privacydomain.ResourceAccount || e.Reason != "keep_pseudonymized" {
		t.Fatalf("account entry %+v", e)
	}
	if _, _, err := p.Purge(t.Context(), cutoff, 100); err != nil {
		t.Fatal(err)
	}
	if n := w.num(`SELECT count(*) FROM gdpr_requests WHERE id = $1`, req); n != 0 {
		t.Fatal("the journaled request stays after its period")
	}
}

// TestGapAfterHostLoss (red team finding of M7-07): after a host loss the database comes back newer
// than the object backup, so the journal objects written after it are missing. bothy restore
// finalize marks every missing number with a placeholder, so the numbers stay dense: a deletion
// on the recovered instance gets the next number, a later restore to a backup from before the
// loss reads past the placeholder and deletes it again, and the consumer never meets an object of
// another entry.
func TestGapAfterHostLoss(t *testing.T) {
	t.Parallel()
	w := newJournalWorld(t, "keep_pseudonymized", nil)
	m1 := w.post(w.alex, "a note deleted before the object backup")
	m2 := w.post(w.alex, "a note deleted after the object backup")
	m3 := w.post(w.alex, "a note deleted on the recovered instance")
	w.caughtUp()
	var restoreOld func() error
	var a int64
	w.stopped(func() {
		a = w.nextNumber()
		restoreOld = w.s.DB().Snapshot(t) // a kept backup from before the host loss
	})
	w.deleteMessage(w.alex, m1) // object a, in the object backup
	w.deleteMessage(w.alex, m2) // object a+1, written after the last object backup
	w.caughtUp()
	// The host loss: the database at its latest state, the objects of the older object backup.
	w.stopped(func() {
		if err := os.Remove(w.objectPath(a + 1)); err != nil {
			t.Fatal(err)
		}
		res, err := w.s.FinalizeRestore(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if j := res.Journal; j.Lost != 1 || j.Entries != 0 || j.Next != a+2 {
			t.Fatalf("recovery finalize %+v, want one lost number and next %d", j, a+2)
		}
	})
	if _, lost, err := privacydomain.ParseObject(mustObject(t, w, a+1)); err != nil || !lost {
		t.Fatalf("object %d after the recovery: lost %v, %v", a+1, lost, err)
	}
	o, err := w.s.OwnerSession(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	w.deleteMessage(o, m3)
	w.caughtUp()
	if e := w.entry(a + 2); e.ResourceID.String() != m3 {
		t.Fatalf("object %d %+v, want the deletion of %s", a+2, e, m3)
	}
	// Later a restore to the backup from before the loss: the placeholder does not stop it.
	w.stopped(func() {
		if err := restoreOld(); err != nil {
			t.Fatal(err)
		}
		res, err := w.s.FinalizeRestore(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if j := res.Journal; j.Entries != 3 || j.Lost != 1 || j.Messages != 2 || j.Next != a+3 {
			t.Fatalf("second finalize %+v, want the objects %d to %d", j, a, a+2)
		}
	})
	if n := w.num(`SELECT count(*) FROM messages WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL`, []string{m1, m3}); n != 0 {
		t.Fatalf("%d deleted notes are back", n)
	}
	o, err = w.s.OwnerSession(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	m4 := w.post(o, "a note after the second restore")
	w.deleteMessage(o, m4)
	w.caughtUp()
	if e := w.entry(a + 3); e.ResourceID.String() != m4 {
		t.Fatalf("object %d %+v, want the deletion of %s", a+3, e, m4)
	}
}

// mustObject reads journal object n; it must exist.
func mustObject(t *testing.T, w *journalWorld, n int64) []byte {
	t.Helper()
	b, ok := w.object(n)
	if !ok {
		t.Fatalf("journal object %d is missing", n)
	}
	return b
}
