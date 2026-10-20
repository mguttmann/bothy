// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit"
	"github.com/OWNER/bothy/services/server/internal/status"
	"github.com/OWNER/bothy/services/server/internal/status/app"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// restoreEnv is a service with the restore test over fakes and one succeeded pg_full run whose
// manifest describes the copy exactly.
type restoreEnv struct {
	*env
	svc     *app.Service
	scratch *fakeScratch
	chain   *fakeVerifier
	backup  status.BackupRun
}

// newRestoreEnv builds the environment; mutate changes the configuration of the restore test.
func newRestoreEnv(t *testing.T, mutate func(*app.RestoreConfig)) *restoreEnv {
	t.Helper()
	e := newEnv(t, nil)
	r := &restoreEnv{env: e, scratch: newFakeScratch(e.clk),
		chain: &fakeVerifier{res: audit.VerifyResult{OK: true, CheckedFromSeq: 1, CheckedToSeq: 42, Warnings: []string{}}}}
	cfg := e.cfg
	cfg.Restore = app.RestoreConfig{SampleThreads: 200, SampleFiles: 200, RTOTarget: 4 * time.Hour}
	if mutate != nil {
		mutate(&cfg.Restore)
	}
	svc, err := app.New(cfg, app.Deps{
		Runs: e.runs, Exec: e.exec, Source: e.src, Metrics: e.m, Clock: e.clk, Scratch: r.scratch, Chain: r.chain,
		Log: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	r.svc = svc
	if err := svc.Run(context.Background(), domain.KindPGFull); err != nil {
		t.Fatal(err)
	}
	b, ok, err := e.runs.LatestSucceeded(context.Background(), status.BackupPGFull)
	if err != nil || !ok {
		t.Fatalf("seed backup: %v %v", ok, err)
	}
	r.backup = b
	r.scratch.db.setThreads(3)
	r.scratch.setFiles(5)
	return r
}

// test runs the restore test and returns its row, its parsed checks and its error.
func (r *restoreEnv) test(t *testing.T) (fakeRow, domain.RestoreChecks, error) {
	t.Helper()
	err := r.svc.RestoreTest(context.Background())
	rows := r.runs.snapshot()
	row := rows[len(rows)-1]
	if row.kind != status.BackupRestoreTest {
		t.Fatalf("last row is %s, want restore_test", row.kind)
	}
	checks, perr := domain.ParseRestoreChecks(row.checks)
	if perr != nil {
		t.Fatalf("checks of the row: %v\n%s", perr, row.checks)
	}
	return row, checks, err
}

// result returns the result of check name.
func result(c domain.RestoreChecks, name domain.CheckName) domain.CheckResult {
	for _, r := range c.Checks {
		if r.Name == name {
			return r
		}
	}
	return domain.CheckResult{}
}

// outcomes returns name:result[:reason] of every check.
func outcomes(c domain.RestoreChecks) []string {
	var out []string
	for _, r := range c.Checks {
		s := string(r.Name) + ":" + string(r.Result)
		if r.Reason != "" {
			s += ":" + r.Reason
		}
		out = append(out, s)
	}
	return out
}

// allPassed is outcomes of a clean copy.
var allPassed = []string{"pgbackrest:passed", "manifest:passed", "event_log:passed", "audit_chain:passed",
	"thread_sample:passed", "file_sample:passed", "duration:passed"}

// TestRestoreTestAllPass holds 16 8.3 and backup-restore.md 4 for a clean copy: the latest
// physical backup is restored by its label, verified, started, checked and cleared in that
// order; every check passes with its duration; the row is a succeeded restore_test with
// restore_test_checks.v1 and the id and label of the backup; the metrics record the duration
// and the success and no failure.
func TestRestoreTestAllPass(t *testing.T) {
	t.Parallel()
	r := newRestoreEnv(t, nil)
	row, checks, err := r.test(t)
	if err != nil {
		t.Fatalf("RestoreTest: %v", err)
	}
	if got := outcomes(checks); !slices.Equal(got, allPassed) {
		t.Errorf("checks %v\nwant   %v", got, allPassed)
	}
	if row.state != "succeeded" || row.code != "" || row.manifest != nil {
		t.Errorf("row %+v, want succeeded without error and manifest", row)
	}
	if checks.BackupID != r.backup.ID.String() || checks.Label != "20261018-020000F" {
		t.Errorf("backup %s %s, want %s 20261018-020000F", checks.BackupID, checks.Label, r.backup.ID)
	}
	if want := []string{"Restore", "Verify", "Start", "FetchObjects", "Clear"}; !slices.Equal(r.scratch.callList(), want) {
		t.Errorf("scratch calls %v, want %v", r.scratch.callList(), want)
	}
	if !slices.Equal(r.scratch.labels, []string{"20261018-020000F"}) {
		t.Errorf("restored labels %v", r.scratch.labels)
	}
	if res := result(checks, domain.CheckPgBackRest); res.DurationMS <= 0 {
		t.Errorf("pgbackrest took %d ms, want the measured time", res.DurationMS)
	}
	if r.chain.calls != 1 {
		t.Errorf("audit verifier called %d times", r.chain.calls)
	}
	last := r.m.finished[len(r.m.finished)-1]
	if last.kind != domain.KindRestoreTest || !last.succeeded || last.duration <= 0 {
		t.Errorf("RunFinished %+v, want a succeeded restore_test with its duration", last)
	}
	if len(r.m.failed) != 0 {
		t.Errorf("failed checks recorded: %v", r.m.failed)
	}
}

// TestRestoreFailureSkipsDependentChecks holds design note 1 and rule 13: when check 1 cannot
// restore or start the copy, checks 2 to 6 are skipped with restore_failed and the duration
// still runs; a check that fails or panics never stops the others; Clear runs in every case; the
// row is failed with the code checks_failed and each failed check is counted.
func TestRestoreFailureSkipsDependentChecks(t *testing.T) {
	t.Parallel()
	skipped := []string{"manifest:skipped:restore_failed", "event_log:skipped:restore_failed", "audit_chain:skipped:restore_failed",
		"thread_sample:skipped:restore_failed", "file_sample:skipped:restore_failed", "duration:passed"}
	for _, c := range []struct {
		name   string
		mutate func(r *restoreEnv)
		want   []string
		failed []domain.CheckName
		calls  []string
	}{
		{"restore fails", func(r *restoreEnv) { r.scratch.err["Restore"] = &app.ExecError{Code: "pgbackrest_exit_62"} },
			append([]string{"pgbackrest:failed:pgbackrest_exit_62"}, skipped...), []domain.CheckName{domain.CheckPgBackRest},
			[]string{"Restore", "Verify", "Clear"}},
		{"start fails", func(r *restoreEnv) { r.scratch.err["Start"] = errors.New("postgres exited") },
			append([]string{"pgbackrest:failed:scratch_start_failed"}, skipped...), []domain.CheckName{domain.CheckPgBackRest},
			[]string{"Restore", "Verify", "Start", "Clear"}},
		{"no backup", func(r *restoreEnv) { r.runs.rows = nil },
			append([]string{"pgbackrest:failed:no_backup"}, skipped...), []domain.CheckName{domain.CheckPgBackRest},
			[]string{"Clear"}},
		{"verify fails, the copy is still checked", func(r *restoreEnv) { r.scratch.err["Verify"] = &app.ExecError{Code: "pgbackrest_exit_104"} },
			append([]string{"pgbackrest:failed:pgbackrest_exit_104"}, allPassed[1:]...), []domain.CheckName{domain.CheckPgBackRest},
			[]string{"Restore", "Verify", "Start", "FetchObjects", "Clear"}},
		{"a check panics", func(r *restoreEnv) { r.scratch.db.panics = "ThreadSample" },
			[]string{"pgbackrest:passed", "manifest:passed", "event_log:passed", "audit_chain:passed",
				"thread_sample:failed:check_panicked", "file_sample:passed", "duration:passed"},
			[]domain.CheckName{domain.CheckThreadSample}, []string{"Restore", "Verify", "Start", "FetchObjects", "Clear"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := newRestoreEnv(t, nil)
			c.mutate(r)
			row, checks, err := r.test(t)
			if err == nil {
				t.Error("RestoreTest returned nil for a failed test")
			}
			if got := outcomes(checks); !slices.Equal(got, c.want) {
				t.Errorf("checks %v\nwant   %v", got, c.want)
			}
			if row.state != "failed" || row.code != "checks_failed" {
				t.Errorf("row %s %q, want failed checks_failed", row.state, row.code)
			}
			if !slices.Equal(r.scratch.callList(), c.calls) {
				t.Errorf("scratch calls %v, want %v", r.scratch.callList(), c.calls)
			}
			if len(r.m.failed) != 1 || !slices.Equal(r.m.failed[0], c.failed) {
				t.Errorf("failed checks recorded %v, want %v", r.m.failed, c.failed)
			}
			if last := r.m.finished[len(r.m.finished)-1]; last.kind != domain.KindRestoreTest || last.succeeded {
				t.Errorf("RunFinished %+v, want a failed restore_test", last)
			}
		})
	}
}

// TestRestoreChecksDetectDamage holds the checks 2 to 6 on damaged copies (ADR 0058
// "Confirmation" at the level of the use case): each damage fails exactly its check with its
// reason while the others pass.
func TestRestoreChecksDetectDamage(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		mutate func(r *restoreEnv)
		check  domain.CheckName
		reason string
	}{
		{"other schema version", func(r *restoreEnv) { r.scratch.db.facts.SchemaVersion = 48 }, domain.CheckManifest, "schema_version_mismatch"},
		{"other instance", func(r *restoreEnv) { r.scratch.db.facts.InstanceID = "0192f4a0-7b1c-7000-8a00-0000000000ff" }, domain.CheckManifest, "instance_mismatch"},
		{"a table is missing", func(r *restoreEnv) {
			r.scratch.db.facts.RowCounts = map[string]int64{"event_log": 120}
		}, domain.CheckManifest, "table_missing"},
		{"a row is missing", func(r *restoreEnv) {
			r.scratch.db.facts.RowCounts = map[string]int64{"messages": 99, "event_log": 120}
		}, domain.CheckManifest, "row_counts_differ"},
		{"the copy is newer than its manifest", func(r *restoreEnv) { r.scratch.db.facts.HeadSeq = 121 }, domain.CheckManifest, "copy_newer_than_manifest"},
		{"gap in a partition", func(r *restoreEnv) { r.scratch.db.events.Partitions[0].Count = 98 }, domain.CheckEventLog, "event_log_gap"},
		{"event_seq behind", func(r *restoreEnv) { r.scratch.db.events.Counters["event_seq"] = 119 }, domain.CheckEventLog, "sequence_counter_behind"},
		{"audit_seq behind", func(r *restoreEnv) { r.scratch.db.events.Counters["audit_seq"] = 41 }, domain.CheckEventLog, "sequence_counter_behind"},
		{"audit chain broken", func(r *restoreEnv) {
			r.chain.res = audit.VerifyResult{OK: false, FirstBrokenSeq: 7, FailureReason: "hash_mismatch", Warnings: []string{}}
		}, domain.CheckAuditChain, "hash_mismatch"},
		{"thread_seq gap", func(r *restoreEnv) { r.scratch.db.threads[1].Messages = 2 }, domain.CheckThreadSample, "thread_seq_gap"},
		{"thread counter behind", func(r *restoreEnv) { r.scratch.db.threads[2].LastMessageSeq = 2 }, domain.CheckThreadSample, "thread_seq_gap"},
		{"changed object", func(r *restoreEnv) {
			r.scratch.objects[r.scratch.db.files[3].ObjectKey] = sha256.Sum256([]byte("changed"))
		}, domain.CheckFileSample, "object_hash_mismatch"},
		{"missing object", func(r *restoreEnv) { delete(r.scratch.objects, r.scratch.db.files[0].ObjectKey) }, domain.CheckFileSample, "object_missing"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := newRestoreEnv(t, nil)
			c.mutate(r)
			_, checks, err := r.test(t)
			if err == nil {
				t.Error("RestoreTest returned nil for a damaged copy")
			}
			if got := checks.Failed(); !slices.Equal(got, []domain.CheckName{c.check}) {
				t.Errorf("failed %v, want only %s (%v)", got, c.check, outcomes(checks))
			}
			if got := result(checks, c.check).Reason; got != c.reason {
				t.Errorf("reason %q, want %q", got, c.reason)
			}
		})
	}
}

// TestRestoreManifestWindow: writes between the end of the backup and its manifest
// (docs/DECISIONS.md D-M7-02-6) leave the copy behind its manifest; the row counts are then not
// compared, while schema version, instance and tables still are, and backup_runs is never
// compared, because concurrent runs write it.
func TestRestoreManifestWindow(t *testing.T) {
	t.Parallel()
	r := newRestoreEnv(t, nil)
	r.scratch.db.facts.HeadSeq = 110
	r.scratch.db.facts.RowCounts = map[string]int64{"messages": 90, "event_log": 110}
	if _, checks, err := r.test(t); err != nil || !checks.Passed() {
		t.Errorf("copy behind its manifest: %v %v", err, outcomes(checks))
	}
	q := newRestoreEnv(t, nil)
	q.scratch.db.facts.RowCounts = map[string]int64{"messages": 100, "event_log": 120, "backup_runs": 3}
	q.src.facts.RowCounts = map[string]int64{"messages": 100, "event_log": 120, "backup_runs": 1}
	if err := q.svc.Run(context.Background(), domain.KindPGFull); err != nil {
		t.Fatal(err)
	}
	if _, checks, err := q.test(t); err != nil || !checks.Passed() {
		t.Errorf("backup_runs differs: %v %v", err, outcomes(checks))
	}
}

// TestDurationOverRTOFails holds check 7 with a fake clock: a test that takes longer than
// dr.rto_target fails the duration check only.
func TestDurationOverRTOFails(t *testing.T) {
	t.Parallel()
	r := newRestoreEnv(t, func(c *app.RestoreConfig) { c.RTOTarget = time.Hour })
	r.scratch.restoreTook = 2 * time.Hour
	_, checks, err := r.test(t)
	if err == nil {
		t.Error("RestoreTest returned nil over dr.rto_target")
	}
	want := append(slices.Clone(allPassed[:6]), "duration:failed:rto_exceeded")
	if got := outcomes(checks); !slices.Equal(got, want) {
		t.Errorf("checks %v\nwant   %v", got, want)
	}
	if d := result(checks, domain.CheckDuration).DurationMS; d < (2 * time.Hour).Milliseconds() {
		t.Errorf("duration %d ms, want the whole test of at least 2h", d)
	}
}

// TestSampleSizes holds rule 15: the samples ask for backup.restore_test.sample_threads and
// sample_files rows and check all rows when fewer exist.
func TestSampleSizes(t *testing.T) {
	t.Parallel()
	r := newRestoreEnv(t, nil)
	if _, checks, err := r.test(t); err != nil || result(checks, domain.CheckThreadSample).Items != 3 || result(checks, domain.CheckFileSample).Items != 5 {
		t.Errorf("fewer rows than the sample: %v, threads %d files %d, want 3 and 5", err,
			result(checks, domain.CheckThreadSample).Items, result(checks, domain.CheckFileSample).Items)
	}
	if got := r.scratch.db.samples; !slices.Equal(got["ThreadSample"], []int{200}) || !slices.Equal(got["FileSample"], []int{200}) {
		t.Errorf("sample sizes asked %v, want 200 each", got)
	}
	if len(r.scratch.fetched) != 1 || len(r.scratch.fetched[0]) != 5 {
		t.Errorf("fetched %v, want the 5 sampled keys", r.scratch.fetched)
	}

	big := newRestoreEnv(t, func(c *app.RestoreConfig) { c.SampleThreads, c.SampleFiles = 4, 2 })
	big.scratch.db.setThreads(10)
	big.scratch.setFiles(10)
	if _, checks, err := big.test(t); err != nil || result(checks, domain.CheckThreadSample).Items != 4 || result(checks, domain.CheckFileSample).Items != 2 {
		t.Errorf("more rows than the sample: %v, threads %d files %d, want 4 and 2", err,
			result(checks, domain.CheckThreadSample).Items, result(checks, domain.CheckFileSample).Items)
	}
}

// TestRestoreTestScheduleAndSlot: the scheduler starts the restore test on
// backup.restore_test.schedule in a slot of its own, and a second test while one runs is ErrBusy.
func TestRestoreTestScheduleAndSlot(t *testing.T) {
	t.Parallel()
	r := newRestoreEnv(t, func(c *app.RestoreConfig) {
		s, err := domain.ParseSchedule("0 5 * * 6")
		if err != nil {
			t.Fatal(err)
		}
		c.Schedule, c.Scheduled = s, true
	})
	// Saturday 2026-10-24 05:00 UTC is due; the backups of that night are not in the window.
	start := time.Date(2026, 10, 24, 4, 30, 0, 0, time.UTC)
	r.clk.Set(start)
	s := app.NewScheduler(r.svc, start)
	if got := s.Tick(context.Background(), start.Add(30*time.Minute)); !slices.Equal(got, []domain.Kind{domain.KindRestoreTest}) {
		t.Errorf("Tick = %v, want restore_test", got)
	}
	s.Wait()
	if rows := r.runs.snapshot(); rows[len(rows)-1].kind != status.BackupRestoreTest || rows[len(rows)-1].state != "succeeded" {
		t.Errorf("last row %+v, want a succeeded restore_test", rows[len(rows)-1])
	}

	g := newGate()
	held := newRestoreEnv(t, nil)
	gated := &gatedScratch{fakeScratch: held.scratch, g: g}
	cfg := held.cfg
	cfg.Restore = app.RestoreConfig{SampleThreads: 200, SampleFiles: 200, RTOTarget: 4 * time.Hour}
	svc, err := app.New(cfg, app.Deps{Runs: held.runs, Exec: held.exec, Source: held.src, Clock: held.clk, Scratch: gated, Chain: held.chain, Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- svc.RestoreTest(context.Background()) }()
	<-g.entered
	if err := svc.RestoreTest(context.Background()); !errors.Is(err, app.ErrBusy) {
		t.Errorf("second test = %v, want ErrBusy", err)
	}
	if err := svc.Run(context.Background(), domain.KindPGDiff); err != nil {
		t.Errorf("a backup during the test: %v", err)
	}
	close(g.release)
	if err := <-done; err != nil {
		t.Errorf("held test: %v", err)
	}
}

// gatedScratch holds Restore until the gate opens.
type gatedScratch struct {
	*fakeScratch
	g *gate
}

func (s *gatedScratch) Restore(ctx context.Context, label string) error {
	s.g.entered <- struct{}{}
	<-s.g.release
	return s.fakeScratch.Restore(ctx, label)
}

// TestRestoreTestNeedsScratch: a service without Scratch or Chain refuses the restore test
// without a row.
func TestRestoreTestNeedsScratch(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	if err := e.svc.RestoreTest(context.Background()); err == nil || len(e.runs.snapshot()) != 0 {
		t.Errorf("RestoreTest without Scratch: %v, %d rows", err, len(e.runs.snapshot()))
	}
}
