// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// faultStore fails the CleanupStore method named fail with err and passes every other call on.
type faultStore struct {
	app.CleanupStore
	mu   sync.Mutex
	fail string
	err  error
}

func (f *faultStore) failing(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail == name {
		return f.err
	}
	return nil
}

func (f *faultStore) ExpiredOpenSessions(ctx context.Context, now time.Time, after app.Candidate, limit int) ([]app.Candidate, error) {
	if err := f.failing("ExpiredOpenSessions"); err != nil {
		return nil, err
	}
	return f.CleanupStore.ExpiredOpenSessions(ctx, now, after, limit)
}

func (f *faultStore) UnattachedCandidates(ctx context.Context, cutoff time.Time, after app.Candidate, limit int) ([]app.Candidate, error) {
	if err := f.failing("UnattachedCandidates"); err != nil {
		return nil, err
	}
	return f.CleanupStore.UnattachedCandidates(ctx, cutoff, after, limit)
}

func (f *faultStore) FailedRowCandidates(ctx context.Context, cutoff time.Time, after app.Candidate, limit int) ([]app.Candidate, error) {
	if err := f.failing("FailedRowCandidates"); err != nil {
		return nil, err
	}
	return f.CleanupStore.FailedRowCandidates(ctx, cutoff, after, limit)
}

func (f *faultStore) DeletedRows(ctx context.Context, after app.Candidate, limit int) ([]app.Candidate, error) {
	if err := f.failing("DeletedRows"); err != nil {
		return nil, err
	}
	return f.CleanupStore.DeletedRows(ctx, after, limit)
}

func (f *faultStore) PurgeBacklog(ctx context.Context) (int64, time.Time, error) {
	if err := f.failing("PurgeBacklog"); err != nil {
		return 0, time.Time{}, err
	}
	return f.CleanupStore.PurgeBacklog(ctx)
}

func (f *faultStore) PurgeRecord(ctx context.Context, fileID ids.ID) (app.PurgeRecord, bool, error) {
	if err := f.failing("PurgeRecord"); err != nil {
		return app.PurgeRecord{}, false, err
	}
	return f.CleanupStore.PurgeRecord(ctx, fileID)
}

func (f *faultStore) LockFile(ctx context.Context, fileID ids.ID) (bool, error) {
	if err := f.failing("LockFile"); err != nil {
		return false, err
	}
	return f.CleanupStore.LockFile(ctx, fileID)
}

func (f *faultStore) CleanupRecord(ctx context.Context, fileID ids.ID) (app.CleanupRecord, bool, error) {
	if err := f.failing("CleanupRecord"); err != nil {
		return app.CleanupRecord{}, false, err
	}
	return f.CleanupStore.CleanupRecord(ctx, fileID)
}

func (f *faultStore) ExpireSession(ctx context.Context, fileID ids.ID) error {
	if err := f.failing("ExpireSession"); err != nil {
		return err
	}
	return f.CleanupStore.ExpireSession(ctx, fileID)
}

func (f *faultStore) DeleteFileRow(ctx context.Context, fileID ids.ID) (bool, error) {
	if err := f.failing("DeleteFileRow"); err != nil {
		return false, err
	}
	return f.CleanupStore.DeleteFileRow(ctx, fileID)
}

// withStore rebuilds the purge and the cleanup of e over st.
func (e *cleanupEnv) withStore(t *testing.T, st app.CleanupStore) {
	t.Helper()
	log := slog.New(slog.NewJSONHandler(e.logs, nil))
	p, err := app.NewPurge(app.PurgeDeps{Store: st, Objects: e.objects, Parts: app.NewFSPartStore(e.objects), UoW: e.uow, Log: log})
	if err != nil {
		t.Fatal(err)
	}
	c, err := app.NewCleanup(e.cfg, app.CleanupDeps{Store: st, Repo: e.repo, Scans: e.repo, UoW: e.uow, Events: e.events,
		Channels: channels{space: e.space}, Redactor: e.redact, Purge: e.purges, Purger: p, Drafts: e.drafts, Temp: e.sweep,
		Metrics: e.backlog, Clock: e.clk, Log: log})
	if err != nil {
		t.Fatal(err)
	}
	e.purge, e.cleanup = p, c
}

// TestCleanup_StoreErrorReachesRun: with one candidate for every step, a failing read or write of
// the cleanup store ends with that error from Run, and the steps after it still run.
func TestCleanup_StoreErrorReachesRun(t *testing.T) {
	for _, method := range []string{"ExpiredOpenSessions", "UnattachedCandidates", "FailedRowCandidates", "DeletedRows",
		"PurgeBacklog", "PurgeRecord", "LockFile", "CleanupRecord", "ExpireSession", "DeleteFileRow"} {
		t.Run(method, func(t *testing.T) {
			e := newCleanupEnv(t, "fs", nil)
			boom := errors.New("cleanup store down")
			e.withStore(t, &faultStore{CleanupStore: e.repo, fail: method, err: boom})
			e.seed(t, e.alex.ID, domain.SessionOpen, domain.UploadPending, domain.ScanPending)
			e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
			e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadFailed, domain.ScanPending)
			gone := e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
			e.markDeleted(gone.ID)
			e.clk.Advance(e.cfg.FailedRowTTL + time.Second)
			if err := e.cleanup.Run(context.Background()); !errors.Is(err, boom) {
				t.Fatalf("Run = %v, want the store error", err)
			}
			if len(e.sweep.calls) != 1 {
				t.Errorf("sweep calls %d, want 1", len(e.sweep.calls))
			}
		})
	}
}

// TestCleanup_CancelledContext: a cancelled ctx ends the run with ctx.Err() before any step.
func TestCleanup_CancelledContext(t *testing.T) {
	e := newCleanupEnv(t, "fs", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := e.cleanup.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run = %v, want context.Canceled", err)
	}
	if err := e.purge.Run(ctx, ids.New()); !errors.Is(err, context.Canceled) {
		t.Fatalf("Purge.Run = %v, want context.Canceled", err)
	}
	if len(e.sweep.calls) != 0 {
		t.Errorf("sweep ran after the cancel")
	}
}

// TestCleanup_BacklogWarnAfterDelay: a mark older than privacy.object_delete_delay that the
// catch-up cannot purge gives one Warn line with the count and no file id.
func TestCleanup_BacklogWarnAfterDelay(t *testing.T) {
	base := newStore(t)
	boom := errors.New("store down")
	fs := &failingStore{store: base, fail: map[string]bool{}, err: boom}
	e := newCleanupEnv(t, "fs", fs)
	e.store = base
	f := e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	e.markDeleted(f.ID)
	fs.fail[f.ObjectKey] = true
	e.clk.Advance(e.cfg.ObjectDeleteDelay + time.Second)
	if err := e.cleanup.Run(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("Run = %v, want the store error", err)
	}
	logs := e.logs.String()
	if !strings.Contains(logs, `"bothy.step":"backlog"`) || !strings.Contains(logs, `"bothy.count":1`) {
		t.Errorf("no backlog line in %s", logs)
	}
	for _, line := range strings.Split(logs, "\n") {
		if strings.Contains(line, `"bothy.step":"backlog"`) && strings.Contains(line, f.ID.String()) {
			t.Errorf("backlog line names the file: %s", line)
		}
	}
}

// TestNewCleanup_Refuses: a missing collaborator and every non positive window or batch size are
// refused; nil Clock, Log and Metrics take their defaults.
func TestNewCleanup_Refuses(t *testing.T) {
	e := newCleanupEnv(t, "fs", nil)
	full := app.CleanupDeps{Store: e.repo, Repo: e.repo, Scans: e.repo, UoW: e.uow, Events: e.events,
		Channels: channels{space: e.space}, Redactor: e.redact, Purge: e.purges, Purger: e.purge, Drafts: e.drafts}
	if _, err := app.NewCleanup(cleanupCfg, full); err != nil {
		t.Fatalf("defaults refused: %v", err)
	}
	missing := full
	missing.Drafts = nil
	if _, err := app.NewCleanup(cleanupCfg, missing); err == nil {
		t.Error("a missing collaborator was accepted")
	}
	for name, change := range map[string]func(*app.CleanupConfig){
		"unattached": func(c *app.CleanupConfig) { c.UnattachedTTL = 0 },
		"failed":     func(c *app.CleanupConfig) { c.FailedRowTTL = -time.Second },
		"session":    func(c *app.CleanupConfig) { c.SessionTTL = 0 },
		"delay":      func(c *app.CleanupConfig) { c.ObjectDeleteDelay = 0 },
		"batch":      func(c *app.CleanupConfig) { c.BatchSize = 0 },
	} {
		cfg := cleanupCfg
		change(&cfg)
		if _, err := app.NewCleanup(cfg, full); err == nil {
			t.Errorf("%s: a non positive value was accepted", name)
		}
	}
	if _, err := app.NewPurge(app.PurgeDeps{Store: e.repo, Objects: e.store, UoW: e.uow}); err == nil {
		t.Error("NewPurge accepted a missing part store")
	}
	if _, err := app.NewPurge(app.PurgeDeps{Store: e.repo, Objects: e.store, Parts: app.NewFSPartStore(e.store), UoW: e.uow}); err != nil {
		t.Errorf("NewPurge without a logger: %v", err)
	}
}

// TestPurge_RefusesBadKeyAndAttachedRow: a stored key outside the key schema and an attachment
// row of a deleted file are internal errors; the row and the original stay.
func TestPurge_RefusesBadKeyAndAttachedRow(t *testing.T) {
	e := newCleanupEnv(t, "fs", nil)
	e.logs.Reset()
	bad := e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	e.markDeleted(bad.ID)
	e.repo.mu.Lock()
	f := e.repo.files[bad.ID]
	f.ObjectKey = "elsewhere/" + bad.ID.String()
	e.repo.files[bad.ID] = f
	e.repo.mu.Unlock()
	if err := e.purge.Run(context.Background(), bad.ID); !internal(err) {
		t.Errorf("bad key: Run = %v, want an internal error", err)
	}
	if _, ok := e.row(bad.ID); !ok {
		t.Error("bad key: row gone")
	}
	held := e.seed(t, e.alex.ID, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	e.markDeleted(held.ID)
	e.repo.mu.Lock()
	e.repo.attachments[held.ID] = app.AttachmentRecord{FileID: held.ID}
	e.repo.mu.Unlock()
	if err := e.purge.Run(context.Background(), held.ID); !internal(err) {
		t.Errorf("attached: Run = %v, want an internal error", err)
	}
	if _, ok := e.row(held.ID); !ok {
		t.Error("attached: row gone")
	}
	if !strings.Contains(e.logs.String(), held.ID.String()) || strings.Contains(e.logs.String(), "notes.txt") {
		t.Errorf("logs %s", e.logs.String())
	}
}

// internal reports an *errs.Error with the code internal_error.
func internal(err error) bool {
	e, ok := errs.As(err)
	return ok && e.Code == errs.CodeInternalError
}
