// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/status"
	"github.com/OWNER/bothy/services/server/internal/status/app"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

const day = 24 * time.Hour

// syncBuffer is a log sink that run goroutines may write concurrently.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// env is a service over fakes.
type env struct {
	svc   *app.Service
	cfg   app.Config
	exec  *fakeExec
	src   *fakeSource
	runs  *fakeRuns
	m     *fakeMetrics
	clk   *clock.Fake
	log   *syncBuffer
	start time.Time
}

// sunday is 2026-10-18 01:00 UTC, a Sunday.
var sunday = time.Date(2026, 10, 18, 1, 0, 0, 0, time.UTC)

// schedule parses expr or fails the test.
func schedule(t *testing.T, expr string) domain.Schedule {
	t.Helper()
	s, err := domain.ParseSchedule(expr)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// newEnv builds the service with the default schedules of backup.* and no off-site target;
// mutate changes the configuration first.
func newEnv(t *testing.T, mutate func(*app.Config)) *env {
	t.Helper()
	clk := clock.NewFake(sunday)
	e := &env{exec: newFakeExec(), src: newFakeSource(), m: newFakeMetrics(), clk: clk, log: &syncBuffer{}, start: sunday}
	e.exec.clk, e.exec.step = clk, time.Second
	e.runs = newFakeRuns(clk)
	e.cfg = app.Config{
		Schedules: map[domain.Kind]domain.Schedule{
			domain.KindPGFull:  schedule(t, "0 2 * * 0"),
			domain.KindPGDiff:  schedule(t, "0 2 * * 1-6"),
			domain.KindLogical: schedule(t, "0 3 * * *"),
			domain.KindObjects: schedule(t, "30 3 * * *"),
		},
		ObjectsDriver:      "fs",
		FilesRoot:          "/var/lib/bothy/files",
		DumpDir:            t.TempDir(),
		LocalRetentionFull: 1,
		RetentionFull:      4,
		LogicalRetention:   30 * day,
		ObjectsRetention:   90 * day,
		ServerVersion:      "1.4.0-rc.1",
		Volumes:            map[string]string{"pgdata": "/var/lib/postgresql/data", "files": "/var/lib/bothy/files"},
	}
	if mutate != nil {
		mutate(&e.cfg)
	}
	svc, err := app.New(e.cfg, app.Deps{
		Runs: e.runs, Exec: e.exec, Source: e.src, Metrics: e.m, Clock: clk,
		Log: slog.New(slog.NewTextHandler(e.log, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	e.svc = svc
	return e
}

// TestRunWritesRowAndManifest holds one run per kind (16 8.1, brief M7-02 rules 12 and 13): the
// row goes from running to succeeded with the size, the manifest is valid with the reference of
// its kind, and the same manifest is stored through restic stdin with its tags. Without an
// off-site target the local restic snapshots of logical, objects and the manifests are thinned.
func TestRunWritesRowAndManifest(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name     string
		kind     domain.Kind
		driver   string
		size     int64
		calls    []string
		label    string
		snapshot bool
	}{
		{"pg_full", domain.KindPGFull, "fs", 1000, []string{"PgBackRestBackup", "PgBackRestExpireLocal", "ResticBackupStdin"}, "20261018-020000F", false},
		{"pg_diff", domain.KindPGDiff, "fs", 1000, []string{"PgBackRestBackup", "PgBackRestExpireLocal", "ResticBackupStdin"}, "20261018-020000F_20261019-020000D", false},
		{"logical", domain.KindLogical, "fs", 2048, []string{"PgDump", "ResticBackup", "ResticBackupStdin", "ResticForget", "ResticForget"}, "", true},
		{"objects fs", domain.KindObjects, "fs", 2048, []string{"ResticBackup", "ResticBackupStdin", "ResticForget", "ResticForget"}, "", true},
		{"objects s3", domain.KindObjects, "s3", 4096, []string{"S3Mirror", "ResticBackupStdin", "ResticForget", "ResticForget"}, "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t, func(cfg *app.Config) { cfg.ObjectsDriver = c.driver })
			if err := e.svc.Run(context.Background(), c.kind); err != nil {
				t.Fatalf("Run: %v", err)
			}
			rows := e.runs.snapshot()
			if len(rows) != 1 {
				t.Fatalf("%d rows, want 1", len(rows))
			}
			row := rows[0]
			if row.kind != status.BackupKind(c.kind) || !slices.Equal(e.runs.states[row.id], []string{"running", "succeeded"}) ||
				row.size != c.size || row.code != "" {
				t.Errorf("row %+v, states %v; want kind %s, running then succeeded, size %d", row, e.runs.states[row.id], c.kind, c.size)
			}
			if got := e.exec.callList(); !slices.Equal(got, c.calls) {
				t.Errorf("calls %v, want %v", got, c.calls)
			}

			m, err := domain.ParseManifest(row.manifest)
			if err != nil {
				t.Fatalf("stored manifest: %v\n%s", err, row.manifest)
			}
			facts := e.src.facts
			if m.BackupID != row.id.String() || m.Kind != c.kind || !m.StartedAt.Equal(row.started) ||
				!m.FinishedAt.After(m.StartedAt) || m.ServerVersion != e.cfg.ServerVersion ||
				m.SchemaVersion != facts.SchemaVersion || m.InstanceID != facts.InstanceID || m.SyncEpoch != facts.SyncEpoch ||
				m.EventLog != (domain.EventLogBounds{HeadSeq: 120, MinRetainedSeq: 1}) ||
				m.Audit != (domain.AuditHead{LastSeq: 42, LastHash: hex.EncodeToString(facts.AuditLastHash)}) ||
				m.RowCounts["messages"] != 100 || m.Files != (domain.FileTotals{Count: 3, TotalBytes: 4096}) {
				t.Errorf("manifest %+v does not describe the run %+v and the facts %+v", m, row, facts)
			}
			if c.label != "" && (m.PgBackRest == nil || m.PgBackRest.Label != c.label) {
				t.Errorf("pgbackrest %+v, want label %s", m.PgBackRest, c.label)
			}
			if c.snapshot != (m.Restic != nil) || c.snapshot && m.Restic.SnapshotID != strings.Repeat("0f", 32) {
				t.Errorf("restic %+v, want a reference: %v", m.Restic, c.snapshot)
			}

			stdin := e.exec.restic[len(e.exec.restic)-1]
			if !bytes.Equal(stdin.Stdin, row.manifest) || stdin.StdinName != "manifest-"+row.id.String()+".json" ||
				!slices.Equal(stdin.Tags, []string{"manifest", string(c.kind), row.id.String()}) || len(stdin.Paths) != 0 {
				t.Errorf("manifest snapshot %+v, want the row manifest as manifest-%s.json with tags manifest, %s, the id", stdin, row.id, c.kind)
			}

			switch c.kind {
			case domain.KindPGFull, domain.KindPGDiff:
				want := c.name[3:] // "full" or "diff"
				if !slices.Equal(e.exec.backups, []app.PgBackRestBackup{{Type: want, Repo: 1}}) || !slices.Equal(e.exec.expires, []int{1}) {
					t.Errorf("pgbackrest %+v, expire %v; want %s into repo 1 only and expire keeping 1", e.exec.backups, e.exec.expires, want)
				}
			case domain.KindLogical:
				dump := filepath.Join(e.cfg.DumpDir, "bothy-"+row.id.String()+".dump")
				if !slices.Equal(e.exec.dumps, []string{dump}) || !slices.Equal(e.exec.restic[0].Paths, []string{dump}) ||
					!slices.Equal(e.exec.restic[0].Tags, []string{"logical"}) {
					t.Errorf("dumps %v, restic %+v; want %s stored with tag logical", e.exec.dumps, e.exec.restic[0], dump)
				}
				if _, err := os.Stat(dump); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("the dump file is still there: %v", err)
				}
				assertForgets(t, e.exec.forgets, "logical", 30*day)
			case domain.KindObjects:
				if c.driver == "fs" && (!slices.Equal(e.exec.restic[0].Paths, []string{e.cfg.FilesRoot}) || !slices.Equal(e.exec.restic[0].Tags, []string{"objects"})) {
					t.Errorf("restic %+v, want the files root with tag objects", e.exec.restic[0])
				}
				assertForgets(t, e.exec.forgets, "objects", 90*day)
			}

			if len(e.m.finished) != 1 {
				t.Fatalf("RunFinished %d times, want 1", len(e.m.finished))
			}
			f := e.m.finished[0]
			if f.kind != c.kind || !f.succeeded || f.size != c.size || f.duration != row.finished.Sub(row.started) ||
				f.duration <= 0 || !f.finishedAt.Equal(row.finished) {
				t.Errorf("RunFinished %+v, want kind %s succeeded size %d duration %v at %v", f, c.kind, c.size, row.finished.Sub(row.started), row.finished)
			}
			if !strings.Contains(e.log.String(), "state=succeeded") || !strings.Contains(e.log.String(), row.id.String()) {
				t.Errorf("log %q lacks the run line", e.log.String())
			}
		})
	}
}

// assertForgets checks the local thinning: the kind's tag with its retention and the manifests
// with the larger of logical and objects, both on the local repository.
func assertForgets(t *testing.T, got []app.ResticForget, tag string, keep time.Duration) {
	t.Helper()
	want := []app.ResticForget{{Tag: tag, KeepWithin: keep}, {Tag: "manifest", KeepWithin: 90 * day}}
	if !slices.Equal(got, want) {
		t.Errorf("forgets %+v, want %+v", got, want)
	}
}

// TestRunFailureRecordsCode holds a failed run (brief M7-02 rule 12): the row is failed with a
// code and no output text, no manifest is written or stored, the dump file is removed, and a
// canceled run is recorded as canceled with a context that still works.
func TestRunFailureRecordsCode(t *testing.T) {
	t.Parallel()
	leak := "disk full on /data/pg_wal"
	execErr := func(code string) error { return fmt.Errorf("%s: %w", leak, &app.ExecError{Code: code}) }
	for _, c := range []struct {
		name   string
		kind   domain.Kind
		method string
		err    error
		srcErr error
		code   string
	}{
		{"pgbackrest exit", domain.KindPGFull, "PgBackRestBackup", execErr("pgbackrest_exit_82"), nil, "pgbackrest_exit_82"},
		{"expire exit", domain.KindPGDiff, "PgBackRestExpireLocal", execErr("pgbackrest_exit_50"), nil, "pgbackrest_exit_50"},
		{"pg_dump exit", domain.KindLogical, "PgDump", execErr("pg_dump_exit_1"), nil, "pg_dump_exit_1"},
		{"restic exit", domain.KindLogical, "ResticBackup", execErr("restic_exit_3"), nil, "restic_exit_3"},
		{"restic not found", domain.KindObjects, "ResticBackup", execErr("restic_not_found"), nil, "restic_not_found"},
		{"pgbackrest plain error", domain.KindPGFull, "PgBackRestBackup", errors.New(leak), nil, "pgbackrest_failed"},
		{"pg_dump plain error", domain.KindLogical, "PgDump", errors.New(leak), nil, "pg_dump_failed"},
		{"invalid code", domain.KindLogical, "ResticBackup", &app.ExecError{Code: leak}, nil, "restic_failed"},
		{"snapshot", domain.KindPGFull, "", nil, errors.New(leak), "manifest_failed"},
		{"manifest store", domain.KindPGFull, "ResticBackupStdin", execErr("restic_exit_1"), nil, "manifest_store_failed"},
		{"forget", domain.KindLogical, "ResticForget", execErr("restic_exit_1"), nil, "restic_exit_1"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t, nil)
			if c.method != "" {
				e.exec.err[c.method] = c.err
			}
			e.src.err = c.srcErr
			err := e.svc.Run(context.Background(), c.kind)
			if err == nil {
				t.Fatal("Run succeeded, want the step error")
			}
			rows := e.runs.snapshot()
			if len(rows) != 1 {
				t.Fatalf("%d rows, want 1", len(rows))
			}
			row := rows[0]
			if row.state != "failed" || row.code != c.code || row.manifest != nil || row.size != -1 || row.finished.IsZero() {
				t.Errorf("row %+v, want failed with code %s, no manifest, no size", row, c.code)
			}
			if c.name != "forget" && c.method != "ResticBackupStdin" && slices.Contains(e.exec.callList(), "ResticBackupStdin") {
				t.Error("a failed run stored a manifest")
			}
			if strings.Contains(e.log.String(), leak) || strings.Contains(row.code, "disk") {
				t.Errorf("output text reached the log or the row: %q", e.log.String())
			}
			if !strings.Contains(e.log.String(), "code="+c.code) {
				t.Errorf("log %q lacks the code", e.log.String())
			}
			for _, d := range e.exec.dumps {
				if _, err := os.Stat(d); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("dump %s left behind: %v", d, err)
				}
			}
			if len(e.m.finished) != 1 || e.m.finished[0].succeeded || e.m.finished[0].size != 0 {
				t.Errorf("RunFinished %+v, want one failed run of size 0", e.m.finished)
			}
		})
	}

	t.Run("canceled", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t, nil)
		g := newGate()
		e.exec.gates["PgDump"] = g
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- e.svc.Run(ctx, domain.KindLogical) }()
		<-g.entered
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("Run = %v, want context.Canceled", err)
		}
		rows := e.runs.snapshot()
		if len(rows) != 1 || rows[0].state != "failed" || rows[0].code != "canceled" {
			t.Errorf("rows %+v, want one failed row with code canceled", rows)
		}
		if slices.ContainsFunc(e.runs.finishCtxs, func(err error) bool { return err != nil }) {
			t.Errorf("Finish saw a canceled context: %v", e.runs.finishCtxs)
		}
		if _, err := os.Stat(e.exec.dumps[0]); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("dump left behind: %v", err)
		}
	})

	t.Run("start fails", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t, nil)
		e.runs.startErr = errors.New("connection refused")
		if err := e.svc.Run(context.Background(), domain.KindPGFull); err == nil {
			t.Fatal("Run succeeded without a row")
		}
		if len(e.exec.callList()) != 0 || len(e.runs.snapshot()) != 0 {
			t.Errorf("calls %v without a row", e.exec.callList())
		}
		// The slot is free again.
		e.runs.startErr = nil
		if err := e.svc.Run(context.Background(), domain.KindPGFull); err != nil {
			t.Errorf("second Run: %v", err)
		}
	})

	t.Run("unknown kind", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t, nil)
		if err := e.svc.Run(context.Background(), domain.Kind("restore_test")); err == nil || len(e.runs.snapshot()) != 0 {
			t.Errorf("Run(restore_test) = %v, want an error and no row", err)
		}
	})
}

// TestOneRunPerKind holds "at most one run per kind" (brief M7-02 design note 1): a second run of
// a busy slot returns ErrBusy and writes no row, pg_full and pg_diff share one slot, logical and
// objects do not block each other; the scheduler starts each due kind once, a full and a
// differential at the same minute run only the full, a processed minute is not started twice and
// a time missed before the start is not caught up.
func TestOneRunPerKind(t *testing.T) {
	t.Parallel()

	t.Run("busy slots", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t, nil)
		pg, dump := newGate(), newGate()
		e.exec.gates["PgBackRestBackup"] = pg
		e.exec.gates["PgDump"] = dump
		ctx := context.Background()
		var wg sync.WaitGroup
		errs := make([]error, 2)
		wg.Go(func() { errs[0] = e.svc.Run(ctx, domain.KindPGFull) })
		wg.Go(func() { errs[1] = e.svc.Run(ctx, domain.KindLogical) })
		<-pg.entered
		<-dump.entered

		for _, k := range []domain.Kind{domain.KindPGFull, domain.KindPGDiff, domain.KindLogical} {
			if err := e.svc.Run(ctx, k); !errors.Is(err, app.ErrBusy) {
				t.Errorf("Run(%s) while busy = %v, want ErrBusy", k, err)
			}
		}
		if n := len(e.runs.snapshot()); n != 2 {
			t.Errorf("%d rows while busy, want 2", n)
		}
		// objects has its own slot.
		if err := e.svc.Run(ctx, domain.KindObjects); err != nil {
			t.Errorf("Run(objects) while logical runs: %v", err)
		}
		close(pg.release)
		close(dump.release)
		wg.Wait()
		if errs[0] != nil || errs[1] != nil {
			t.Fatalf("held runs: %v", errs)
		}
		// Free again.
		delete(e.exec.gates, "PgBackRestBackup")
		if err := e.svc.Run(ctx, domain.KindPGDiff); err != nil {
			t.Errorf("Run(pg_diff) after the full: %v", err)
		}
		if n := len(e.runs.snapshot()); n != 4 {
			t.Errorf("%d rows, want 4", n)
		}
	})

	t.Run("scheduler", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t, func(cfg *app.Config) {
			// The differential is due every day at 02:00, also on Sunday with the full.
			cfg.Schedules[domain.KindPGDiff] = schedule(t, "0 2 * * *")
		})
		ctx := context.Background()
		s := app.NewScheduler(e.svc, e.start)
		tick := func(now time.Time, want ...domain.Kind) {
			t.Helper()
			e.clk.Set(now)
			got := s.Tick(ctx, now)
			s.Wait()
			if !slices.Equal(got, want) {
				t.Errorf("Tick(%v) = %v, want %v", now, got, want)
			}
		}
		tick(sunday.Add(59 * time.Minute))
		tick(sunday.Add(time.Hour), domain.KindPGFull) // 02:00: the full only
		tick(sunday.Add(time.Hour))                    // the same minute again
		tick(sunday.Add(time.Hour + 30*time.Second))   // later in the same minute
		tick(sunday.Add(2*time.Hour+30*time.Minute), domain.KindLogical, domain.KindObjects)
		tick(sunday.Add(day+time.Hour), domain.KindPGDiff) // Monday 02:00
		var kinds []status.BackupKind
		for _, r := range e.runs.snapshot() {
			kinds = append(kinds, r.kind)
			if r.state != "succeeded" {
				t.Errorf("row %+v not succeeded", r)
			}
		}
		// logical and objects run concurrently, so their rows have no fixed order.
		slices.Sort(kinds)
		if want := []status.BackupKind{status.BackupLogical, status.BackupObjects, status.BackupPGDiff, status.BackupPGFull}; !slices.Equal(kinds, want) {
			t.Errorf("rows %v, want each of %v once", kinds, want)
		}
	})

	t.Run("no catch-up", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t, nil)
		// The process starts at 02:30 on Sunday: the full of 02:00 is missed and not run.
		s := app.NewScheduler(e.svc, sunday.Add(90*time.Minute))
		if got := s.Tick(context.Background(), sunday.Add(91*time.Minute)); len(got) != 0 {
			t.Errorf("Tick after start = %v, want nothing", got)
		}
		s.Wait()
		if n := len(e.runs.snapshot()); n != 0 {
			t.Errorf("%d rows, want 0", n)
		}
	})

	t.Run("scheduler skips a busy slot", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t, func(cfg *app.Config) {
			delete(cfg.Schedules, domain.KindLogical)
			delete(cfg.Schedules, domain.KindObjects)
		})
		g := newGate()
		e.exec.gates["PgBackRestBackup"] = g
		ctx := context.Background()
		s := app.NewScheduler(e.svc, e.start)
		if got := s.Tick(ctx, sunday.Add(time.Hour)); !slices.Equal(got, []domain.Kind{domain.KindPGFull}) {
			t.Errorf("Tick = %v, want pg_full", got)
		}
		<-g.entered
		// Monday 02:00 while the full still runs: the differential is not started.
		if got := s.Tick(ctx, sunday.Add(day+time.Hour)); len(got) != 0 {
			t.Errorf("Tick while the full runs = %v, want nothing", got)
		}
		close(g.release)
		s.Wait()
		if n := len(e.runs.snapshot()); n != 1 {
			t.Errorf("%d rows, want 1", n)
		}
	})
}

// TestOffsiteEmptyWarns holds brief M7-02 rule 15 and 14: without backup.offsite.target the
// service warns "local backup only" and backs up into repo 1 only; with a target it has no
// warning, backs up into repo 1 and repo 2 and never forgets snapshots off-site itself.
func TestOffsiteEmptyWarns(t *testing.T) {
	t.Parallel()
	local := newEnv(t, nil)
	if got := local.svc.Warnings(); !slices.Equal(got, []string{"local backup only: backup.offsite.target is empty"}) {
		t.Errorf("Warnings without off-site = %q", got)
	}
	if err := local.svc.Run(context.Background(), domain.KindPGFull); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(local.exec.backups, []app.PgBackRestBackup{{Type: "full", Repo: 1}}) {
		t.Errorf("backups without off-site %+v, want repo 1 only", local.exec.backups)
	}

	off := newEnv(t, func(cfg *app.Config) { cfg.OffsiteEnabled = true })
	if got := off.svc.Warnings(); len(got) != 0 {
		t.Errorf("Warnings with off-site = %q, want none", got)
	}
	ctx := context.Background()
	for _, k := range domain.Kinds() {
		if err := off.svc.Run(ctx, k); err != nil {
			t.Fatalf("Run(%s): %v", k, err)
		}
	}
	want := []app.PgBackRestBackup{{Type: "full", Repo: 1}, {Type: "full", Repo: 2}, {Type: "diff", Repo: 1}, {Type: "diff", Repo: 2}}
	if !slices.Equal(off.exec.backups, want) {
		t.Errorf("backups with off-site %+v, want %+v", off.exec.backups, want)
	}
	if len(off.exec.forgets) != 0 || !slices.Equal(off.exec.expires, []int{1, 1}) {
		t.Errorf("with off-site: forgets %+v, expires %v; want no forget and expire on repo 1 only", off.exec.forgets, off.exec.expires)
	}
	// The size and label of the manifest come from repo 1.
	for _, r := range off.runs.snapshot()[:2] {
		if r.size != 1000 {
			t.Errorf("%s size %d, want the repo 1 size", r.kind, r.size)
		}
	}
}

// TestPrime holds the start of bothy-backupd: the last success of each metric kind (physical
// from pg_full and pg_diff, logical, objects) and of the restore test is read from backup_runs
// into the metrics.
func TestPrime(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	ctx := context.Background()
	if err := e.svc.Run(ctx, domain.KindPGDiff); err != nil {
		t.Fatal(err)
	}
	physical := e.runs.snapshot()[0].started
	e.clk.Advance(time.Hour)
	if err := e.svc.Run(ctx, domain.KindLogical); err != nil {
		t.Fatal(err)
	}
	logical := e.runs.snapshot()[1].started
	e.runs.lastCalls = nil

	if err := e.svc.Prime(ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.runs.lastCalls; len(got) != 4 || !slices.Equal(got[0], []status.BackupKind{status.BackupPGFull, status.BackupPGDiff}) ||
		!slices.Equal(got[1], []status.BackupKind{status.BackupLogical}) || !slices.Equal(got[2], []status.BackupKind{status.BackupObjects}) ||
		!slices.Equal(got[3], []status.BackupKind{status.BackupRestoreTest}) {
		t.Errorf("LastSuccess calls %v", got)
	}
	if len(e.m.last) != 2 || !e.m.last[domain.KindPGFull].Equal(physical) || !e.m.last[domain.KindLogical].Equal(logical) {
		t.Errorf("last successes %v, want physical %v (as pg_full) and logical %v, no objects", e.m.last, physical, logical)
	}

	e.runs.lastErr = status.ErrNoBackupTable
	if err := e.svc.Prime(ctx); !errors.Is(err, status.ErrNoBackupTable) {
		t.Errorf("Prime = %v, want ErrNoBackupTable", err)
	}

	quiet, err := app.New(e.cfg, app.Deps{Runs: e.runs, Exec: e.exec, Source: e.src, Clock: e.clk})
	if err != nil {
		t.Fatal(err)
	}
	if err := quiet.Prime(ctx); err != nil {
		t.Errorf("Prime without metrics: %v", err)
	}
}

// TestSample holds the samples of the WAL archive and the volumes: each value that can be read
// is recorded, an error is logged and never stops the others.
func TestSample(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	ctx := context.Background()
	archived := sunday.Add(-5 * time.Minute)
	e.src.archived, e.src.archivedOK = archived, true
	e.exec.usage["/var/lib/bothy/files"] = app.VolumeUsage{FreeBytes: 10, SizeBytes: 100}
	e.exec.err["VolumeUsage:/var/lib/postgresql/data"] = &app.ExecError{Code: "statfs_failed"}
	e.svc.Sample(ctx)
	if !slices.Equal(e.m.wal, []time.Time{archived}) {
		t.Errorf("WAL samples %v, want %v", e.m.wal, archived)
	}
	if len(e.m.volumes) != 1 || e.m.volumes["files"] != (app.VolumeUsage{FreeBytes: 10, SizeBytes: 100}) {
		t.Errorf("volumes %v, want files only", e.m.volumes)
	}
	if !strings.Contains(e.log.String(), "pgdata") {
		t.Errorf("log %q lacks the failed volume", e.log.String())
	}

	// No archived segment yet and an error of the source: nothing recorded, no panic.
	e.src.archivedOK = false
	e.svc.Sample(ctx)
	e.src.archivedErr = errors.New("connection refused")
	e.svc.Sample(ctx)
	if len(e.m.wal) != 1 {
		t.Errorf("WAL samples %v, want still one", e.m.wal)
	}
}

// TestExpireOffsite holds the owner's off-site expiry (R-OP-09, brief M7-02 design note 4): it
// needs an off-site target and the owner's directory, expires repo2 with
// backup.pg.retention_full and forgets logical, objects and the manifests with the owner's
// credentials.
func TestExpireOffsite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	local := newEnv(t, nil)
	if err := local.svc.ExpireOffsite(ctx, "/run/owner/offsite-admin"); err == nil || len(local.exec.callList()) != 0 {
		t.Errorf("ExpireOffsite without off-site = %v, calls %v; want an error and no call", err, local.exec.callList())
	}

	off := newEnv(t, func(cfg *app.Config) { cfg.OffsiteEnabled = true })
	if err := off.svc.ExpireOffsite(ctx, ""); err == nil || len(off.exec.callList()) != 0 {
		t.Errorf("ExpireOffsite without a directory = %v, want an error and no call", err)
	}
	dir := "/run/owner/offsite-admin"
	if err := off.svc.ExpireOffsite(ctx, dir); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(off.exec.offsite, []string{dir}) || !slices.Equal(off.exec.expires, []int{4}) {
		t.Errorf("pgbackrest off-site expire %v keeping %v, want %s keeping 4", off.exec.offsite, off.exec.expires, dir)
	}
	want := []app.ResticForget{
		{Tag: "logical", KeepWithin: 30 * day, AdminDir: dir},
		{Tag: "objects", KeepWithin: 90 * day, AdminDir: dir},
		{Tag: "manifest", KeepWithin: 90 * day, AdminDir: dir},
	}
	if !slices.Equal(off.exec.forgets, want) {
		t.Errorf("forgets %+v, want %+v", off.exec.forgets, want)
	}

	failing := newEnv(t, func(cfg *app.Config) { cfg.OffsiteEnabled = true })
	failing.exec.err["PgBackRestExpireOffsite"] = &app.ExecError{Code: "pgbackrest_exit_82"}
	err := failing.svc.ExpireOffsite(ctx, dir)
	if app.ErrorCode(err, "") != "pgbackrest_exit_82" {
		t.Errorf("ExpireOffsite = %v, want the pgbackrest code", err)
	}
	if len(failing.exec.forgets) != 3 {
		t.Errorf("forgets after a failed pgbackrest expire: %+v, want all three", failing.exec.forgets)
	}
}

// TestNewRejects holds the checks of New: every dependency but Metrics and Log is required and
// the configuration names a known object driver.
func TestNewRejects(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	deps := app.Deps{Runs: e.runs, Exec: e.exec, Source: e.src, Clock: e.clk}
	for name, mut := range map[string]func(*app.Config, *app.Deps){
		"no runs":         func(_ *app.Config, d *app.Deps) { d.Runs = nil },
		"no executor":     func(_ *app.Config, d *app.Deps) { d.Exec = nil },
		"no source":       func(_ *app.Config, d *app.Deps) { d.Source = nil },
		"no clock":        func(_ *app.Config, d *app.Deps) { d.Clock = nil },
		"unknown driver":  func(c *app.Config, _ *app.Deps) { c.ObjectsDriver = "nfs" },
		"fs without root": func(c *app.Config, _ *app.Deps) { c.FilesRoot = "" },
		"no dump dir":     func(c *app.Config, _ *app.Deps) { c.DumpDir = "" },
		"retention":       func(c *app.Config, _ *app.Deps) { c.LocalRetentionFull = 0 },
	} {
		cfg, d := e.cfg, deps
		mut(&cfg, &d)
		if _, err := app.New(cfg, d); err == nil {
			t.Errorf("New with %s succeeded", name)
		}
	}
}
