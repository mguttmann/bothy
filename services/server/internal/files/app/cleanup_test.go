// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// failingStore fails Delete of the keys in fail (once each when once is set) and records the
// order of the calls of the wrapped store.
type failingStore struct {
	*store
	mu   sync.Mutex
	fail map[string]bool
	once bool
	err  error
}

func (f *failingStore) Delete(ctx context.Context, key string) error {
	f.mu.Lock()
	failing := f.fail[key]
	if failing && f.once {
		delete(f.fail, key)
	}
	f.mu.Unlock()
	if failing {
		return f.err
	}
	return f.store.Delete(ctx, key)
}

// cleanupEnv is the purge and the cleanup over the fakes of env.
type cleanupEnv struct {
	*env
	purge   *app.Purge
	cleanup *app.Cleanup
	drafts  *drafts
	sweep   *sweeper
	backlog *backlogMetric
	logs    *bytes.Buffer
	objects app.ObjectStore
	cfg     app.CleanupConfig
}

// cleanupCfg is the configuration of the cleanup tests: small windows and batches of two.
var cleanupCfg = app.CleanupConfig{UnattachedTTL: time.Hour, FailedRowTTL: 2 * time.Hour, SessionTTL: 30 * time.Minute,
	ObjectDeleteDelay: 3 * time.Hour, BatchSize: 2}

// newCleanupEnv builds the purge and the cleanup with the part strategy of driver over objects
// (nil: the store of env).
func newCleanupEnv(t *testing.T, driver string, objects app.ObjectStore) *cleanupEnv {
	t.Helper()
	e := &cleanupEnv{env: newEnv(t, driver, testCfg), drafts: &drafts{held: map[ids.ID]ids.ID{}}, sweep: &sweeper{},
		backlog: &backlogMetric{}, logs: &bytes.Buffer{}, cfg: cleanupCfg}
	if objects == nil {
		objects = e.store
	}
	e.objects = objects
	parts := app.NewFSPartStore(objects)
	if driver == "s3" {
		parts = app.NewS3PartStore(objects, e.repo, e.uow)
	}
	log := slog.New(slog.NewJSONHandler(e.logs, nil))
	p, err := app.NewPurge(app.PurgeDeps{Store: e.repo, Objects: objects, Parts: parts, UoW: e.uow, Log: log})
	if err != nil {
		t.Fatal(err)
	}
	c, err := app.NewCleanup(e.cfg, app.CleanupDeps{Store: e.repo, Repo: e.repo, Scans: e.repo, UoW: e.uow, Events: e.events,
		Channels: channels{space: e.space}, Redactor: e.redact, Purge: e.purges, Purger: p, Drafts: e.drafts, Temp: e.sweep,
		Metrics: e.backlog, Clock: e.clk, Log: log})
	if err != nil {
		t.Fatal(err)
	}
	e.purge, e.cleanup = p, c
	return e
}

// seed stores a file of uploader with a valid original key, a session in state with two parts
// and the upload and scan states, created at the clock time, and puts its original.
func (e *cleanupEnv) seed(t *testing.T, uploader ids.ID, state domain.SessionState, upload, scan string) app.FileRecord {
	t.Helper()
	id := ids.New()
	key, err := domain.OriginalKey(id, 1)
	if err != nil {
		t.Fatal(err)
	}
	f := app.FileRecord{ID: id, ObjectKey: key, SizeBytes: 8, MIMEType: "text/plain", OriginalName: "notes.txt", UploadedBy: uploader,
		Purpose: string(domain.PurposeAttachment), ScopeChannelID: ids.New(), UploadState: upload, ScanStatus: scan,
		TranscriptStatus: "none", CreatedAt: e.clk.Now().UTC()}
	s := app.SessionRecord{FileID: id, Mode: "direct", State: state, PartCount: 2, CreatedAt: f.CreatedAt,
		ExpiresAt: f.CreatedAt.Add(e.cfg.SessionTTL), File: f}
	if state == domain.SessionCompleted {
		s.CompletedAt = f.CreatedAt
	}
	e.repo.mu.Lock()
	e.repo.files[id] = f
	e.repo.sessions[id] = s
	e.repo.mu.Unlock()
	if err := e.store.Put(context.Background(), key, strings.NewReader("original"), 8); err != nil {
		t.Fatal(err)
	}
	return f
}

// markDeleted marks f deleted at the clock time, as markDeleted does.
func (e *cleanupEnv) markDeleted(id ids.ID) {
	e.repo.mu.Lock()
	defer e.repo.mu.Unlock()
	f := e.repo.files[id]
	f.Deleted = true
	e.repo.files[id] = f
	e.repo.deletedAt[id] = e.clk.Now().UTC()
}

func (e *cleanupEnv) row(id ids.ID) (app.FileRecord, bool) {
	e.repo.mu.Lock()
	defer e.repo.mu.Unlock()
	f, ok := e.repo.files[id]
	return f, ok
}

func (e *cleanupEnv) hasObject(key string) bool {
	e.store.mu.Lock()
	defer e.store.mu.Unlock()
	_, ok := e.store.objects[key]
	return ok
}

// keysOf are the keys the purge deletes for f.
func keysOf(t *testing.T, f app.FileRecord) []string {
	t.Helper()
	keys, err := domain.ObjectKeysOf(f.ID, f.ObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

// TestPurge_NoStoreCallInUnitOfWork holds 02 8.5 for both strategies: the fake store fails every
// call made inside a unit of work, and the purge of a deleted file deletes every key of the key
// schema, discards the parts (fs) or aborts the multipart upload (s3), and then removes the row.
func TestPurge_NoStoreCallInUnitOfWork(t *testing.T) {
	for _, driver := range []string{"fs", "s3"} {
		t.Run(driver, func(t *testing.T) {
			e := newCleanupEnv(t, driver, nil)
			f := e.seed(t, e.alex.ID, domain.SessionOpen, domain.UploadPending, domain.ScanPending)
			if driver == "s3" {
				e.repo.mu.Lock()
				s := e.repo.sessions[f.ID]
				s.StorageUploadID = "upload-77"
				e.repo.sessions[f.ID] = s
				e.repo.mu.Unlock()
			}
			e.markDeleted(f.ID)
			if err := e.purge.Run(context.Background(), f.ID); err != nil {
				t.Fatal(err)
			}
			wantDeletes := len(keysOf(t, f))
			if driver == "fs" {
				wantDeletes += 2 // the two part keys
			}
			if got := e.store.count("Delete"); got != wantDeletes {
				t.Errorf("Delete calls %d, want %d", got, wantDeletes)
			}
			if driver == "s3" && e.store.count("AbortMultipart") != 1 {
				t.Errorf("AbortMultipart calls %d", e.store.count("AbortMultipart"))
			}
			if _, ok := e.row(f.ID); ok || e.hasObject(f.ObjectKey) {
				t.Error("row or original stays")
			}
		})
	}
}

// TestPurge_LiveFileUntouched: a stray job never touches a live file (deliverable, scan pending,
// running upload).
func TestPurge_LiveFileUntouched(t *testing.T) {
	e := newCleanupEnv(t, "fs", nil)
	before := e.store.total()
	for _, f := range []app.FileRecord{
		e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean),
		e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanPending),
		e.seed(t, e.alex.ID, domain.SessionOpen, domain.UploadPending, domain.ScanPending),
	} {
		if err := e.purge.Run(context.Background(), f.ID); err != nil {
			t.Fatal(err)
		}
		if _, ok := e.row(f.ID); !ok || !e.hasObject(f.ObjectKey) {
			t.Errorf("file %s lost row or object", f.ID)
		}
	}
	if got := e.store.total() - before - 3; got != 0 { // the three Puts of seed
		t.Errorf("%d store calls for live files", got)
	}
	if err := e.purge.Run(context.Background(), ids.New()); err != nil {
		t.Errorf("unknown file: %v", err)
	}
}

// TestPurge_FailedKeepsRow: a failed or blocked file loses its objects and parts and keeps its row.
func TestPurge_FailedKeepsRow(t *testing.T) {
	e := newCleanupEnv(t, "fs", nil)
	for _, f := range []app.FileRecord{
		e.seed(t, e.alex.ID, domain.SessionAborted, domain.UploadFailed, domain.ScanPending),
		e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanInfected),
		e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanError),
	} {
		part, _ := domain.UploadPartKey(f.ID, 1)
		if err := e.store.Put(context.Background(), part, strings.NewReader("p"), 1); err != nil {
			t.Fatal(err)
		}
		if err := e.purge.Run(context.Background(), f.ID); err != nil {
			t.Fatal(err)
		}
		if _, ok := e.row(f.ID); !ok || e.hasObject(f.ObjectKey) || e.hasObject(part) {
			t.Errorf("file %s: row %v, original or part stays", f.ID, ok)
		}
	}
}

// TestPurge_StoreErrorReturned: the first failing Delete returns its error and the row stays; the
// next run deletes everything, a missing object counting as deleted.
func TestPurge_StoreErrorReturned(t *testing.T) {
	base := newStore(t)
	boom := errors.New("store down")
	fs := &failingStore{store: base, fail: map[string]bool{}, once: true, err: boom}
	e := newCleanupEnv(t, "fs", fs)
	e.store = base
	f := e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	keys := keysOf(t, f)
	fs.fail[keys[2]] = true
	e.markDeleted(f.ID)
	if err := e.purge.Run(context.Background(), f.ID); !errors.Is(err, boom) {
		t.Fatalf("Run = %v, want the store error", err)
	}
	if _, ok := e.row(f.ID); !ok {
		t.Fatal("the row went before its objects")
	}
	if err := e.purge.Run(context.Background(), f.ID); err != nil {
		t.Fatalf("second Run = %v", err)
	}
	if _, ok := e.row(f.ID); ok || e.hasObject(f.ObjectKey) {
		t.Error("row or original stays")
	}
}

// recordingStore records the order of the reads of the cleanup steps.
type recordingStore struct {
	app.CleanupStore
	mu    sync.Mutex
	calls []string
}

func (r *recordingStore) add(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, name)
}

func (r *recordingStore) ExpiredOpenSessions(ctx context.Context, now time.Time, after app.Candidate, limit int) ([]app.Candidate, error) {
	r.add("expired")
	return r.CleanupStore.ExpiredOpenSessions(ctx, now, after, limit)
}

func (r *recordingStore) UnattachedCandidates(ctx context.Context, cutoff time.Time, after app.Candidate, limit int) ([]app.Candidate, error) {
	r.add("unattached")
	return r.CleanupStore.UnattachedCandidates(ctx, cutoff, after, limit)
}

func (r *recordingStore) FailedRowCandidates(ctx context.Context, cutoff time.Time, after app.Candidate, limit int) ([]app.Candidate, error) {
	r.add("failed")
	return r.CleanupStore.FailedRowCandidates(ctx, cutoff, after, limit)
}

func (r *recordingStore) DeletedRows(ctx context.Context, after app.Candidate, limit int) ([]app.Candidate, error) {
	r.add("deleted")
	return r.CleanupStore.DeletedRows(ctx, after, limit)
}

func (r *recordingStore) PurgeBacklog(ctx context.Context) (int64, time.Time, error) {
	r.add("backlog")
	return r.CleanupStore.PurgeBacklog(ctx)
}

// TestCleanup_StepOrderAndBatches: the steps run in the order a to f; with file.gc_batch_size 2
// five never attached files are read in three batches; one failing candidate leaves the others
// done and the run returns its error after the last step.
func TestCleanup_StepOrderAndBatches(t *testing.T) {
	e := newCleanupEnv(t, "fs", nil)
	var five []app.FileRecord
	for range 5 {
		five = append(five, e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean))
		e.clk.Advance(time.Second)
	}
	rec := &recordingStore{CleanupStore: e.repo}
	e.drafts.fail = five[2].ID
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	p, err := app.NewPurge(app.PurgeDeps{Store: e.repo, Objects: e.store, Parts: app.NewFSPartStore(e.store), UoW: e.uow, Log: log})
	if err != nil {
		t.Fatal(err)
	}
	c, err := app.NewCleanup(e.cfg, app.CleanupDeps{Store: rec, Repo: e.repo, Scans: e.repo, UoW: e.uow, Events: e.events,
		Channels: channels{space: e.space}, Redactor: e.redact, Purge: e.purges,
		Purger: p, Drafts: e.drafts, Temp: e.sweep, Metrics: e.backlog, Clock: e.clk, Log: log})
	if err != nil {
		t.Fatal(err)
	}
	e.clk.Advance(e.cfg.UnattachedTTL)
	if err := c.Run(context.Background()); err == nil {
		t.Fatal("Run hid the failing candidate")
	}
	var order []string
	for _, name := range rec.calls {
		if len(order) == 0 || order[len(order)-1] != name {
			order = append(order, name)
		}
	}
	if want := []string{"expired", "unattached", "failed", "deleted", "backlog"}; !slices.Equal(order, want) {
		t.Errorf("step order %v, want %v", order, want)
	}
	if n := countOf(rec.calls, "unattached"); n != 3 {
		t.Errorf("unattached batches %d, want 3", n)
	}
	for i, f := range five {
		got, ok := e.row(f.ID)
		if i == 2 {
			if !ok || got.Deleted {
				t.Errorf("the failing candidate changed")
			}
			continue
		}
		if ok {
			t.Errorf("file %d: row stays (deleted %v)", i, got.Deleted)
		}
	}
	if len(e.sweep.calls) != 1 || !e.sweep.calls[0].Equal(e.clk.Now().UTC().Truncate(time.Microsecond).Add(-e.cfg.SessionTTL)) {
		t.Errorf("sweep %v", e.sweep.calls)
	}
}

func countOf(list []string, s string) int {
	n := 0
	for _, v := range list {
		if v == s {
			n++
		}
	}
	return n
}

// TestCleanup_CatchUpStopsOnStoreError: the first storage error ends step d for the run, steps e
// and f still run, and the next run purges the rest.
func TestCleanup_CatchUpStopsOnStoreError(t *testing.T) {
	base := newStore(t)
	boom := errors.New("store down")
	fs := &failingStore{store: base, fail: map[string]bool{}, err: boom}
	e := newCleanupEnv(t, "fs", fs)
	e.store = base
	var rows []app.FileRecord
	for range 3 {
		f := e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
		e.markDeleted(f.ID)
		rows = append(rows, f)
		e.clk.Advance(time.Second)
	}
	fs.fail[rows[0].ObjectKey] = true
	if err := e.cleanup.Run(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("Run = %v, want the store error", err)
	}
	for _, f := range rows {
		if _, ok := e.row(f.ID); !ok {
			t.Errorf("file %s purged after the error stopped the step", f.ID)
		}
	}
	if len(e.sweep.calls) != 1 || len(e.backlog.vals) != 1 || e.backlog.vals[0] != 3 {
		t.Errorf("sweep %v backlog %v", e.sweep.calls, e.backlog.vals)
	}
	if !strings.Contains(e.logs.String(), `"bothy.step":"catch_up"`) || strings.Contains(e.logs.String(), "notes.txt") {
		t.Errorf("logs %s", e.logs.String())
	}
	fs.mu.Lock()
	fs.fail = map[string]bool{}
	fs.mu.Unlock()
	if err := e.cleanup.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, f := range rows {
		if _, ok := e.row(f.ID); ok {
			t.Errorf("file %s stays after the second run", f.ID)
		}
	}
	if e.backlog.vals[len(e.backlog.vals)-1] != 0 {
		t.Errorf("backlog %v", e.backlog.vals)
	}
}

// TestCleanup_UnattachedDraftsAndAges: a never attached file stays one nanosecond before
// file.unattached_ttl after its last change and while a draft of its uploader holds it; a draft of
// another principal protects nothing; the deletion redacts and queues the purge.
func TestCleanup_UnattachedDraftsAndAges(t *testing.T) {
	e := newCleanupEnv(t, "fs", nil)
	held := e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	foreign := e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	plain := e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	e.drafts.held[held.ID] = e.alex.ID
	e.drafts.held[foreign.ID] = ids.New()
	e.clk.Advance(e.cfg.UnattachedTTL - time.Microsecond)
	if err := e.cleanup.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(e.purges.list()) != 0 {
		t.Fatalf("deleted before the TTL: %v", e.purges.list())
	}
	e.clk.Advance(time.Microsecond)
	if err := e.cleanup.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := e.purges.list(); len(got) != 2 || !slices.Contains(got, foreign.ID) || !slices.Contains(got, plain.ID) {
		t.Fatalf("purges %v", got)
	}
	if got := e.redact.list(); len(got) != 2 {
		t.Errorf("redactions %v", got)
	}
	if f, ok := e.row(held.ID); !ok || f.Deleted {
		t.Error("the file held by the uploader's draft went")
	}
}

// TestCleanup_ExpiredSession: an open session past expires_at becomes expired, its file failed with
// one file.updated without actor, its part rows go and the purge is queued; an open session before
// it stays.
func TestCleanup_ExpiredSession(t *testing.T) {
	e := newCleanupEnv(t, "fs", nil)
	f := e.seed(t, e.alex.ID, domain.SessionOpen, domain.UploadPending, domain.ScanPending)
	e.repo.mu.Lock()
	e.repo.parts[f.ID] = map[int]app.PartRecord{1: {FileID: f.ID, PartNumber: 1}}
	e.repo.mu.Unlock()
	e.clk.Advance(e.cfg.SessionTTL - time.Microsecond)
	if err := e.cleanup.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s := e.repo.sessions[f.ID]; s.State != domain.SessionOpen {
		t.Fatalf("expired early: %s", s.State)
	}
	e.clk.Advance(time.Microsecond)
	if err := e.cleanup.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := e.row(f.ID)
	if s := e.repo.sessions[f.ID]; s.State != domain.SessionExpired || got.UploadState != domain.UploadFailed || len(e.repo.parts[f.ID]) != 0 {
		t.Fatalf("session %s, file %s, parts %d", s.State, got.UploadState, len(e.repo.parts[f.ID]))
	}
	if len(e.events.list) != 1 || e.events.list[0].Type != "file.updated" || e.events.list[0].ActorKind != "" || !e.events.list[0].ActorPrincipalID.IsZero() {
		t.Errorf("events %+v", e.events.list)
	}
	if q := e.purges.list(); len(q) != 1 || q[0] != f.ID || e.finalCount(f.ID) != 1 {
		t.Errorf("purges %v finalized %d", q, e.finalCount(f.ID))
	}
}

// TestCleanup_FailedRows: the row of an unattached failed or blocked attachment goes after
// file.failed_row_ttl; an attached one and an avatar stay.
func TestCleanup_FailedRows(t *testing.T) {
	e := newCleanupEnv(t, "fs", nil)
	failed := e.seed(t, e.alex.ID, domain.SessionAborted, domain.UploadFailed, domain.ScanPending)
	infected := e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanInfected)
	attached := e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanInfected)
	avatar := e.seed(t, e.alex.ID, domain.SessionAborted, domain.UploadFailed, domain.ScanPending)
	e.repo.mu.Lock()
	a := e.repo.files[avatar.ID]
	a.Purpose = string(domain.PurposeAvatar)
	e.repo.files[avatar.ID] = a
	e.repo.attachments[attached.ID] = app.AttachmentRecord{MessageID: ids.New(), FileID: attached.ID}
	e.repo.mu.Unlock()
	e.clk.Advance(e.cfg.FailedRowTTL)
	if err := e.cleanup.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, f := range []app.FileRecord{failed, infected} {
		if _, ok := e.row(f.ID); ok {
			t.Errorf("row %s stays", f.ID)
		}
	}
	for _, f := range []app.FileRecord{attached, avatar} {
		if got, ok := e.row(f.ID); !ok || got.Deleted {
			t.Errorf("row %s went", f.ID)
		}
	}
}

// TestDelete_Answers holds the table of design note 9.
func TestDelete_Answers(t *testing.T) {
	e := newCleanupEnv(t, "fs", nil)
	sam := actor.Actor{ID: ids.New(), Kind: actor.KindUser}
	samCtx := actor.WithActor(context.Background(), sam)
	own := e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	deletedOwn := e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	deletedOther := e.seed(t, sam.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	attached := e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	avatar := e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	running := e.seed(t, e.alex.ID, domain.SessionOpen, domain.UploadPending, domain.ScanPending)
	e.markDeleted(deletedOwn.ID)
	e.markDeleted(deletedOther.ID)
	e.repo.mu.Lock()
	a := e.repo.files[avatar.ID]
	a.Purpose = string(domain.PurposeAvatar)
	e.repo.files[avatar.ID] = a
	e.repo.attachments[attached.ID] = app.AttachmentRecord{MessageID: ids.New(), FileID: attached.ID, ChannelID: ids.New()}
	e.repo.mu.Unlock()
	problem := func(err error) string {
		if err == nil {
			return "204"
		}
		ee, ok := errs.As(err)
		if !ok {
			return err.Error()
		}
		if ee.Reason != "" {
			return string(ee.Code) + "/" + ee.Reason
		}
		return string(ee.Code)
	}
	for _, c := range []struct {
		name string
		ctx  context.Context
		id   ids.ID
		want string
	}{
		{"missing", e.ctx, ids.New(), "not_found"},
		{"deleted by the caller", e.ctx, deletedOwn.ID, "204"},
		{"deleted by another uploader", e.ctx, deletedOther.ID, "not_found"},
		{"attached", e.ctx, attached.ID, "conflict/file_attached"},
		{"avatar", e.ctx, avatar.ID, "conflict/purpose_not_deletable"},
		{"own unattached", e.ctx, own.ID, "204"},
		{"own unattached again", e.ctx, own.ID, "204"},
		{"running upload", e.ctx, running.ID, "204"},
	} {
		if got := problem(e.svc.Delete(c.ctx, c.id)); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
	if s := e.repo.sessions[running.ID]; s.State != domain.SessionAborted {
		t.Errorf("running session %s, want aborted", s.State)
	}
	if q := e.purges.list(); len(q) != 2 || q[0] != own.ID || q[1] != running.ID {
		t.Errorf("purges %v", q)
	}
	if r := e.redact.list(); len(r) != 2 {
		t.Errorf("redactions %v", r)
	}
	if len(e.events.list) != 0 {
		t.Errorf("events %+v", e.events.list)
	}
	// Not visible: authorize answers not_found for Sam; a denial with read answers forbidden.
	other := e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	e.az.rules[authorize.ActionFileDelete] = authorize.Decision{Outcome: authorize.OutcomeNotFound}
	if got := problem(e.svc.Delete(samCtx, other.ID)); got != "not_found" {
		t.Errorf("not visible: %s", got)
	}
	e.az.rules[authorize.ActionFileDelete] = authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "capability_missing"}
	if got := problem(e.svc.Delete(samCtx, other.ID)); got != "forbidden/capability_missing" {
		t.Errorf("denied: %s", got)
	}
}

// TestMarkDeleted_SecondCallNoEffect: markDeleted through DeleteForMessage takes its steps only for
// the files it marked.
func TestMarkDeleted_SecondCallNoEffect(t *testing.T) {
	e := newCleanupEnv(t, "fs", nil)
	f := e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	for range 2 {
		if err := e.svc.Delete(e.ctx, f.ID); err != nil {
			t.Fatal(err)
		}
	}
	if len(e.purges.list()) != 1 || len(e.redact.list()) != 1 {
		t.Errorf("purges %v redactions %v", e.purges.list(), e.redact.list())
	}
}

// TestFailedTransitionQueuesPurge (design note 5): the abort of an upload makes the file final and
// not deliverable, so the purge is queued in the same unit of work.
func TestFailedTransitionQueuesPurge(t *testing.T) {
	e := newCleanupEnv(t, "fs", nil)
	f := e.seed(t, e.alex.ID, domain.SessionOpen, domain.UploadPending, domain.ScanPending)
	if err := e.svc.Abort(e.ctx, f.ID); err != nil {
		t.Fatal(err)
	}
	if q := e.purges.list(); len(q) != 1 || q[0] != f.ID {
		t.Errorf("purges %v", q)
	}
}

// deletedAtWrite matches a statement that writes files.deleted_at.
var deletedAtWrite = regexp.MustCompile(`(?is)\bSET\b[^;]*\bdeleted_at\s*=`)

// TestOneDeletedAtWriter: exactly one sqlc statement of the module writes deleted_at (brief M3-08
// design note 2).
func TestOneDeletedAtWriter(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "adapters", "postgres", "queries", "*.sql"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("queries %v %v", paths, err)
	}
	var writers []string
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, stmt := range strings.Split(string(b), "-- name: ")[1:] {
			if deletedAtWrite.MatchString(stmt) {
				writers = append(writers, strings.Fields(stmt)[0])
			}
		}
	}
	if !slices.Equal(writers, []string{"MarkFilesDeleted"}) {
		t.Fatalf("deleted_at writers %v, want only MarkFilesDeleted", writers)
	}
}
