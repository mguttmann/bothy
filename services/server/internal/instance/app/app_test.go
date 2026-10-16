// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// fakeRepo is an in-memory repository; it records the SQL-level calls.
type fakeRepo struct {
	values  map[string]int64
	calls   int
	row     *instance.Record
	ensured int
	err     error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{values: map[string]int64{"event_seq": 0, "audit_seq": 0, "permission_version": 0}}
}

func (f *fakeRepo) AdvanceCounter(_ context.Context, name string, n int64) (int64, error) {
	f.calls++
	if f.err != nil {
		return 0, f.err
	}
	f.values[name] += n
	return f.values[name], nil
}

func (f *fakeRepo) CounterValue(_ context.Context, name string) (int64, error) {
	f.calls++
	return f.values[name], f.err
}

func (f *fakeRepo) LoadInstance(context.Context) (instance.Record, bool, error) {
	if f.err != nil {
		return instance.Record{}, false, f.err
	}
	if f.row == nil {
		return instance.Record{}, false, nil
	}
	return *f.row, true, nil
}

func (f *fakeRepo) LockInstance(ctx context.Context) (instance.Record, bool, error) {
	return f.LoadInstance(ctx)
}

func (f *fakeRepo) CompleteSetup(_ context.Context, at time.Time) (bool, error) {
	if f.err != nil || f.row == nil || f.row.SetupCompleted() {
		return false, f.err
	}
	f.row.SetupCompletedAt = at
	return true, nil
}

func (f *fakeRepo) EnsureInstance(_ context.Context, instanceID, syncEpoch ids.ID) error {
	f.ensured++
	if f.row == nil {
		f.row = &instance.Record{InstanceID: instanceID, SyncEpoch: syncEpoch, CreatedAt: time.Unix(1, 0).UTC()}
	}
	return f.err
}

func (f *fakeRepo) SetVersions(_ context.Context, schemaVersion int64, minServerVersion string) error {
	if f.row != nil && schemaVersion >= f.row.SchemaVersion {
		f.row.SchemaVersion, f.row.MinServerVersion = schemaVersion, minServerVersion
	}
	return f.err
}

// fakeUOW runs fn inside a txn unit in the work phase, like platform/pg without a database.
type fakeUOW struct{ runs int }

func (u *fakeUOW) Do(ctx context.Context, fn func(context.Context) error, _ ...txn.Option) error {
	if txn.InUnit(ctx) {
		return fn(ctx)
	}
	u.runs++
	ctx = inPhase(txn.PhaseWork)
	return fn(ctx)
}

// inPhase returns a context inside a unit in phase p.
func inPhase(p txn.Phase) context.Context {
	reg := txn.NewRegistry()
	reg.Seal()
	u, set := txn.NewUnit(reg, txn.Options{})
	set(p)
	return txn.WithUnit(context.Background(), u)
}

func TestCounters_PhaseGuardBeforeSQL(t *testing.T) {
	t.Parallel()
	allowed := map[instance.CounterName]txn.Phase{
		instance.CounterPermissionVersion: txn.PhaseRightsBegin,
		instance.CounterEventSeq:          txn.PhaseSync,
		instance.CounterAuditSeq:          txn.PhaseAudit,
	}
	all := []txn.Phase{txn.PhaseNone, txn.PhaseRightsBegin, txn.PhaseWork, txn.PhaseRightsCheck, txn.PhaseSync, txn.PhaseAudit}
	for name, only := range allowed {
		for _, p := range all {
			repo := newFakeRepo()
			c := app.NewCounters(repo)
			ctx := inPhase(p)
			last, err := c.Advance(ctx, name, 2)
			if p == only {
				if err != nil || last != 2 {
					t.Errorf("Advance(%s) in %s = %d, %v; want 2", name, p, last, err)
				}
				continue
			}
			if errs.KindOf(err) != errs.KindInternal || repo.calls != 0 {
				t.Errorf("Advance(%s) in %s = %v after %d repository calls; want kind internal before SQL", name, p, err, repo.calls)
			}
		}
	}
	// Outside a unit the phase is none.
	repo := newFakeRepo()
	if _, err := app.NewCounters(repo).Advance(context.Background(), instance.CounterEventSeq, 1); errs.KindOf(err) != errs.KindInternal || repo.calls != 0 {
		t.Errorf("Advance outside a unit = %v", err)
	}
}

func TestCounters_CurrentOnlyInHookPhases(t *testing.T) {
	t.Parallel()
	for _, p := range []txn.Phase{txn.PhaseNone, txn.PhaseWork} {
		repo := newFakeRepo()
		ctx := inPhase(p)
		if _, err := app.NewCounters(repo).Current(ctx, instance.CounterEventSeq); errs.KindOf(err) != errs.KindInternal || repo.calls != 0 {
			t.Errorf("Current in %s = %v", p, err)
		}
	}
	for _, p := range []txn.Phase{txn.PhaseRightsBegin, txn.PhaseRightsCheck, txn.PhaseSync, txn.PhaseAudit} {
		repo := newFakeRepo()
		repo.values["audit_seq"] = 7
		ctx := inPhase(p)
		if v, err := app.NewCounters(repo).Current(ctx, instance.CounterAuditSeq); err != nil || v != 7 {
			t.Errorf("Current in %s = %d, %v", p, v, err)
		}
	}
}

func TestCounters_RejectsUnknownNameAndNonPositiveN(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	c := app.NewCounters(repo)
	ctx := inPhase(txn.PhaseSync)
	for _, n := range []int64{0, -5} {
		if _, err := c.Advance(ctx, instance.CounterEventSeq, n); errs.KindOf(err) != errs.KindInternal {
			t.Errorf("Advance by %d = %v", n, err)
		}
	}
	if _, err := c.Advance(ctx, "thread_seq", 1); errs.KindOf(err) != errs.KindInternal {
		t.Errorf("Advance(thread_seq) = %v", err)
	}
	if _, err := c.Current(ctx, "thread_seq"); errs.KindOf(err) != errs.KindInternal {
		t.Errorf("Current(thread_seq) = %v", err)
	}
	if repo.calls != 0 {
		t.Errorf("%d repository calls for rejected input", repo.calls)
	}
}

func TestCounters_RepositoryErrorPassesThrough(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	repo.err = errors.New("boom")
	ctx := inPhase(txn.PhaseSync)
	if _, err := app.NewCounters(repo).Advance(ctx, instance.CounterEventSeq, 1); !errors.Is(err, repo.err) {
		t.Errorf("Advance = %v", err)
	}
	if _, err := app.NewCounters(repo).Current(ctx, instance.CounterEventSeq); !errors.Is(err, repo.err) {
		t.Errorf("Current = %v", err)
	}
}

func TestState_LoadMissingRowIsNotFound(t *testing.T) {
	t.Parallel()
	s := app.NewState(newFakeRepo(), &fakeUOW{})
	if _, err := s.Load(context.Background()); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("Load = %v, want kind not_found", err)
	}
	repo := newFakeRepo()
	repo.err = errors.New("down")
	if _, err := app.NewState(repo, &fakeUOW{}).Load(context.Background()); !errors.Is(err, repo.err) {
		t.Fatalf("Load = %v, want the repository error", err)
	}
}

func TestState_RecordMigrationKeepsIdentity(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uow := &fakeUOW{}
	s := app.NewState(repo, uow)
	first, err := s.RecordMigration(context.Background(), 2, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if first.InstanceID.IsZero() || first.SyncEpoch.IsZero() || first.InstanceID == first.SyncEpoch {
		t.Fatalf("first record %+v: want two distinct new IDs", first)
	}
	second, err := s.RecordMigration(context.Background(), 3, "1.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if second.InstanceID != first.InstanceID || second.SyncEpoch != first.SyncEpoch || second.SchemaVersion != 3 || second.MinServerVersion != "1.1.0" {
		t.Fatalf("second record %+v after %+v", second, first)
	}
	if uow.runs != 2 {
		t.Errorf("RecordMigration ran %d units of work, want one per call", uow.runs)
	}
	for _, bad := range []struct {
		v   int64
		min string
	}{{0, "1.0.0"}, {1, ""}, {1, "v1.0.0"}} {
		if _, err := s.RecordMigration(context.Background(), bad.v, bad.min); errs.KindOf(err) != errs.KindInternal {
			t.Errorf("RecordMigration(%d, %q) = %v, want kind internal", bad.v, bad.min, err)
		}
	}
}

// TestState_SetupLockAndComplete holds the setup part of instance.State (M1-37): both methods need
// a unit of work, a missing row is not_found, and setup_completed_at is set once.
func TestState_SetupLockAndComplete(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	s := app.NewState(repo, &fakeUOW{})
	ctx := inPhase(txn.PhaseWork)
	if _, err := s.LockForSetup(context.Background()); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("LockForSetup outside a unit = %v, want kind internal", err)
	}
	if err := s.CompleteSetup(context.Background(), time.Unix(5, 0)); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("CompleteSetup outside a unit = %v, want kind internal", err)
	}
	if _, err := s.LockForSetup(ctx); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("LockForSetup without a row = %v, want kind not_found", err)
	}
	repo.row = &instance.Record{InstanceID: ids.New()}
	rec, err := s.LockForSetup(ctx)
	if err != nil || rec.SetupCompleted() {
		t.Fatalf("LockForSetup = %+v, %v", rec, err)
	}
	at := time.Unix(7, 0).UTC()
	if err := s.CompleteSetup(ctx, at); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteSetup(ctx, at.Add(time.Hour)); errs.KindOf(err) != errs.KindConflict {
		t.Fatalf("second CompleteSetup = %v, want kind conflict", err)
	}
	if !repo.row.SetupCompletedAt.Equal(at) {
		t.Fatalf("setup_completed_at = %v, want %v", repo.row.SetupCompletedAt, at)
	}
}
