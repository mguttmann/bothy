// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/status"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// The reason codes of the checks (restore_test_checks.v1 reason). They name what failed, never a
// value of the copy.
const (
	reasonNoBackup           = "no_backup"
	reasonBackupRunsRead     = "backup_runs_unreadable"
	reasonManifestInvalid    = "manifest_invalid"
	reasonRestore            = "pgbackrest_restore_failed"
	reasonVerify             = "pgbackrest_verify_failed"
	reasonStart              = "scratch_start_failed"
	reasonQuery              = "scratch_query_failed"
	reasonPanicked           = "check_panicked"
	reasonSchemaVersion      = "schema_version_mismatch"
	reasonInstance           = "instance_mismatch"
	reasonCopyNewer          = "copy_newer_than_manifest"
	reasonAuditHead          = "audit_head_mismatch"
	reasonTableMissing       = "table_missing"
	reasonTableUnexpected    = "table_unexpected"
	reasonRowCounts          = "row_counts_differ"
	reasonEventGap           = "event_log_gap"
	reasonCounterMissing     = "sequence_counter_missing"
	reasonCounterBehind      = "sequence_counter_behind"
	reasonAuditUnknown       = "audit_chain_broken"
	reasonThreadGap          = "thread_seq_gap"
	reasonObjectsDriverS3    = "objects_driver_s3"
	reasonObjectFetch        = "object_fetch_failed"
	reasonObjectMissing      = "object_missing"
	reasonObjectHash         = "object_hash_mismatch"
	reasonRTOExceeded        = "rto_exceeded"
	codeChecksFailed         = "checks_failed"
	codeScratchCleanupFailed = "scratch_cleanup_failed"
	codeCanceled             = "canceled"
)

// The names of sequence_counters that event_log and audit_events draw from (03 4.1).
const (
	counterEventSeq = "event_seq"
	counterAuditSeq = "audit_seq"
)

// The keys of the log lines of the restore test: check names and results with reason codes only.
const (
	attrCheck  = "check"
	attrChecks = "checks"
)

// countedTables are the tables whose row counts check 2 compares with the manifest at equal
// heads: every row of them carries the number of its log, so equal heads mean equal counts in an
// undamaged copy. Other tables change without an event or audit row (sessions, tokens, cleanup
// jobs, backup_runs itself) while a backup runs, so their counts are not compared
// (docs/decisions.d/M7-03.md D-M7-03-3).
var countedTables = []string{"audit_events", "event_log"}

// errRestoreNotConfigured reports a service built without Scratch.
var errRestoreNotConfigured = errors.New("status: the restore test needs a scratch area and the audit verifier")

// RestoreTest runs the automatic restore test once now (docs/spec/16-operations.md 8.3,
// docs/operations/backup-restore.md 4, brief M7-03): it records a running restore_test row,
// restores the latest succeeded pg_full or pg_diff by the pgBackRest label of its manifest into
// the scratch area, runs the seven checks of 8.3, each with its own result and duration, clears
// the scratch area, and finishes the row with restore_test_checks.v1 in checks: succeeded when no
// check failed, else failed with the code checks_failed (scratch_cleanup_failed when the scratch
// area could not be cleared). The metrics get the duration, the last success and one failure
// per failed check.
//
// Check 1 failing to restore or start the copy skips checks 2 to 6 with restore_failed; any
// other failed check, also one that panics, leaves the others running. The scratch area is
// cleared in every case. The test reads the production database only through Runs (the role of
// bothy-backupd) and writes nothing else there.
//
// It returns ErrBusy without a row while a restore test runs, an error without a row when the
// service has no Scratch, the error of Runs.Start, and an error naming the failed checks when the
// test failed; nil when every check passed.
//
// Example:
//
//	if err := svc.RestoreTest(ctx); err != nil { /* exit 1 */ }
func (s *Service) RestoreTest(ctx context.Context) error {
	if s.deps.Scratch == nil || s.deps.Chain == nil {
		return errRestoreNotConfigured
	}
	if !s.acquire(domain.KindRestoreTest) {
		return ErrBusy
	}
	defer s.release(domain.KindRestoreTest)
	return s.restoreTest(ctx)
}

// restoreTest is RestoreTest with the slot taken.
func (s *Service) restoreTest(ctx context.Context) error {
	started := s.deps.Clock.Now()
	id, err := s.deps.Runs.Start(ctx, status.BackupRestoreTest)
	if err != nil {
		s.log.LogAttrs(ctx, slog.LevelError, "restore test not recorded")
		return fmt.Errorf("status: recording the start of the restore test: %w", err)
	}
	t := &restoreRun{s: s, ctx: ctx, results: map[domain.CheckName]domain.CheckResult{}}
	cleared := t.run()
	fctx := context.WithoutCancel(ctx)
	finished := s.deps.Clock.Now()
	t.record(domain.CheckDuration, func() domain.CheckResult {
		if total := finished.Sub(started); total > s.cfg.Restore.RTOTarget {
			return failed(reasonRTOExceeded)
		}
		return passed(0)
	}, finished.Sub(started))
	checks := t.checks()
	b, err := checks.Marshal()
	if err != nil {
		// Every result comes from this file, so this is a bug; the row still ends.
		s.log.LogAttrs(fctx, slog.LevelError, "restore test checks invalid", slog.String(attrID, id.String()))
		b = nil
	}
	ok := checks.Passed() && cleared && b != nil
	// A test stopped with the process (its context ended) is canceled like a backup run: its
	// failed checks say nothing about the backup, so they raise no alert.
	canceled := ctx.Err() != nil
	code := ""
	switch {
	case canceled:
		ok, code = false, codeCanceled
	case !cleared:
		code = codeScratchCleanupFailed
	case !ok:
		code = codeChecksFailed
	}
	finishErr := s.deps.Runs.Finish(fctx, status.FinishedRun{ID: id, Succeeded: ok, SizeBytes: -1, Checks: b, Error: code})
	s.restoreFinished(fctx, id, checks, ok, code, finishErr == nil, canceled, finished.Sub(started))
	switch {
	case !ok:
		return fmt.Errorf("status: restore test failed: %s", failedList(checks, code))
	case finishErr != nil:
		return fmt.Errorf("status: recording the end of the restore test: %w", finishErr)
	}
	return nil
}

// restoreFinished logs the outcome and records the metrics.
func (s *Service) restoreFinished(ctx context.Context, id ids.ID, checks domain.RestoreChecks, ok bool, code string, recorded, canceled bool, took time.Duration) {
	state, level := "succeeded", slog.LevelInfo
	if !ok || !recorded {
		state, level = "failed", slog.LevelError
	}
	attrs := []slog.Attr{slog.String(attrKind, string(domain.KindRestoreTest)), slog.String(attrID, id.String()),
		slog.String(attrState, state), slog.String(attrCode, code), slog.Bool(attrRecorded, recorded)}
	results := make([]string, 0, len(checks.Checks))
	for _, r := range checks.Checks {
		results = append(results, string(r.Name)+"="+string(r.Result)+reasonSuffix(r.Reason))
	}
	attrs = append(attrs, slog.String(attrChecks, strings.Join(results, ",")))
	s.log.LogAttrs(ctx, level, "restore test finished", attrs...)
	if s.deps.Metrics == nil {
		return
	}
	s.deps.Metrics.RunFinished(ctx, domain.KindRestoreTest, ok, took, 0, s.deps.Clock.Now())
	if f := checks.Failed(); len(f) > 0 && !canceled {
		s.deps.Metrics.RestoreChecksFailed(ctx, f)
	}
}

// reasonSuffix is ":"+reason, or "" for none.
func reasonSuffix(reason string) string {
	if reason == "" {
		return ""
	}
	return ":" + reason
}

// failedList names the failed checks with their reasons for the error of RestoreTest.
func failedList(c domain.RestoreChecks, code string) string {
	var parts []string
	for _, r := range c.Checks {
		if r.Result == domain.ResultFailed {
			parts = append(parts, string(r.Name)+" ("+r.Reason+")")
		}
	}
	if code == codeScratchCleanupFailed {
		parts = append(parts, codeScratchCleanupFailed)
	}
	if len(parts) == 0 {
		return code
	}
	return strings.Join(parts, ", ")
}

// restoreRun is one run of the restore test.
type restoreRun struct {
	s       *Service
	ctx     context.Context
	results map[domain.CheckName]domain.CheckResult
	backup  status.BackupRun
	label   string
}

// passed is a passed result over items.
func passed(items int64) domain.CheckResult {
	return domain.CheckResult{Result: domain.ResultPassed, Items: items}
}

// failed is a failed result with reason.
func failed(reason string) domain.CheckResult {
	return domain.CheckResult{Result: domain.ResultFailed, Reason: reason}
}

// record runs check, measures it on the clock (or takes took when it is positive) and stores
// its result under name. A panic of check is a failure with check_panicked; the remaining
// checks still run.
func (t *restoreRun) record(name domain.CheckName, check func() domain.CheckResult, took time.Duration) {
	start := t.s.deps.Clock.Now()
	res := func() (res domain.CheckResult) {
		defer func() {
			if p := recover(); p != nil {
				t.s.log.LogAttrs(context.WithoutCancel(t.ctx), slog.LevelError, "restore test check panicked", slog.String(attrCheck, string(name)))
				res = failed(reasonPanicked)
			}
		}()
		return check()
	}()
	if took <= 0 {
		took = t.s.deps.Clock.Now().Sub(start)
	}
	res.Name = name
	if res.Result != domain.ResultSkipped {
		res.DurationMS = max(took, 0).Milliseconds()
	}
	t.results[name] = res
}

// skip marks the checks of names as skipped with reason.
func (t *restoreRun) skip(reason string, names ...domain.CheckName) {
	for _, n := range names {
		t.results[n] = domain.CheckResult{Name: n, Result: domain.ResultSkipped, Reason: reason}
	}
}

// checks returns restore_test_checks.v1 of the run.
func (t *restoreRun) checks() domain.RestoreChecks {
	c := domain.RestoreChecks{Schema: domain.RestoreChecksSchema, Label: t.label}
	if t.label != "" {
		c.BackupID = t.backup.ID.String()
	}
	for _, n := range domain.RestoreCheckNames() {
		c.Checks = append(c.Checks, t.results[n])
	}
	return c
}

// run runs checks 1 to 6 and clears the scratch area; it reports whether the clearing worked.
// Clear runs in a deferred call, so a panic outside a check still leaves the scratch area empty.
func (t *restoreRun) run() (cleared bool) {
	sc := t.s.deps.Scratch
	defer func() {
		if err := sc.Clear(context.WithoutCancel(t.ctx)); err != nil {
			t.s.log.LogAttrs(context.WithoutCancel(t.ctx), slog.LevelError, "restore test scratch area not cleared",
				slog.String(attrCode, ErrorCode(err, codeScratchCleanupFailed)))
			cleared = false
		}
	}()
	cleared = true
	var db ScratchDB
	var manifest domain.Manifest
	t.record(domain.CheckPgBackRest, func() domain.CheckResult {
		var res domain.CheckResult
		db, manifest, res = t.restore()
		return res
	}, 0)
	if db == nil {
		t.skip(domain.ReasonRestoreFailed, domain.CheckManifest, domain.CheckEventLog, domain.CheckAuditChain,
			domain.CheckThreadSample, domain.CheckFileSample)
		return cleared
	}
	t.record(domain.CheckManifest, func() domain.CheckResult { return t.checkManifest(db, manifest) }, 0)
	t.record(domain.CheckEventLog, func() domain.CheckResult { return t.checkEventLog(db) }, 0)
	t.record(domain.CheckAuditChain, func() domain.CheckResult { return t.checkAudit(db) }, 0)
	t.record(domain.CheckThreadSample, func() domain.CheckResult { return t.checkThreads(db) }, 0)
	t.record(domain.CheckFileSample, func() domain.CheckResult { return t.checkFiles(db) }, 0)
	return cleared
}

// restore is check 1: find the latest physical backup, restore it, verify the repository and
// start the copy. It returns the copy and the manifest of the backup, or no copy when the
// restore or the start failed; a failed verify still returns the started copy.
func (t *restoreRun) restore() (ScratchDB, domain.Manifest, domain.CheckResult) {
	run, ok, err := t.s.deps.Runs.LatestSucceeded(t.ctx, status.BackupPGFull, status.BackupPGDiff)
	switch {
	case err != nil:
		return nil, domain.Manifest{}, failed(reasonBackupRunsRead)
	case !ok:
		return nil, domain.Manifest{}, failed(reasonNoBackup)
	}
	t.backup = run
	m, err := domain.ParseManifest(run.Manifest)
	if err != nil || m.PgBackRest == nil {
		return nil, domain.Manifest{}, failed(reasonManifestInvalid)
	}
	t.label = m.PgBackRest.Label
	sc := t.s.deps.Scratch
	restoreErr := sc.Restore(t.ctx, t.label)
	verifyErr := sc.Verify(t.ctx)
	if restoreErr != nil {
		return nil, m, failed(ErrorCode(restoreErr, reasonRestore))
	}
	db, err := sc.Start(t.ctx)
	if err != nil {
		return nil, m, failed(ErrorCode(err, reasonStart))
	}
	if verifyErr != nil {
		return db, m, failed(ErrorCode(verifyErr, reasonVerify))
	}
	return db, m, passed(0)
}

// checkManifest is check 2: the copy against the manifest of its backup. Schema version,
// instance and sync epoch must be equal and the copy holds exactly the tables of row_counts. The
// manifest was read after the backup step (docs/decisions.d/M7-02.md D-M7-02-6), so the copy may lag it
// but never lead it: its event_log head and audit head are not above those of the manifest, and
// at the same audit head the hash is the same. When both heads are equal no write reached the
// event or audit log in between, and the counts of countedTables must be equal
// (docs/decisions.d/M7-03.md D-M7-03-3).
func (t *restoreRun) checkManifest(db ScratchDB, m domain.Manifest) domain.CheckResult {
	f, err := db.Facts(t.ctx)
	if err != nil {
		return failed(reasonQuery)
	}
	items := int64(len(f.RowCounts))
	switch {
	case f.SchemaVersion != m.SchemaVersion:
		return failed(reasonSchemaVersion)
	case f.InstanceID != m.InstanceID || f.SyncEpoch != m.SyncEpoch:
		return failed(reasonInstance)
	case f.HeadSeq > m.EventLog.HeadSeq || f.AuditLastSeq > m.Audit.LastSeq:
		return failed(reasonCopyNewer)
	case f.AuditLastSeq == m.Audit.LastSeq && fmt.Sprintf("%x", f.AuditLastHash) != m.Audit.LastHash:
		return failed(reasonAuditHead)
	}
	if reason := compareTables(f.RowCounts, m.RowCounts, f.HeadSeq == m.EventLog.HeadSeq && f.AuditLastSeq == m.Audit.LastSeq); reason != "" {
		return failed(reason)
	}
	return passed(items)
}

// compareTables returns the reason when the tables of the copy and of the manifest differ, or,
// with counts, when the row count of one of countedTables differs; "" when they agree.
func compareTables(copyCounts, manifestCounts map[string]int64, counts bool) string {
	for table := range manifestCounts {
		if _, ok := copyCounts[table]; !ok {
			return reasonTableMissing
		}
	}
	for table := range copyCounts {
		if _, ok := manifestCounts[table]; !ok {
			return reasonTableUnexpected
		}
	}
	if !counts {
		return ""
	}
	for _, table := range countedTables {
		if copyCounts[table] != manifestCounts[table] {
			return reasonRowCounts
		}
	}
	return ""
}

// checkEventLog is check 3: every partition of event_log without gaps, count(*) = max(seq) -
// min(seq) + 1, and sequence_counters at least as large as every number assigned to event_log and
// audit_events.
func (t *restoreRun) checkEventLog(db ScratchDB) domain.CheckResult {
	f, err := db.EventLog(t.ctx)
	if err != nil {
		return failed(reasonQuery)
	}
	var head int64
	for _, p := range f.Partitions {
		if p.Count != p.MaxSeq-p.MinSeq+1 {
			return failed(reasonEventGap)
		}
		head = max(head, p.MaxSeq)
	}
	for name, used := range map[string]int64{counterEventSeq: head, counterAuditSeq: f.AuditHead} {
		v, ok := f.Counters[name]
		switch {
		case !ok:
			return failed(reasonCounterMissing)
		case v < used:
			return failed(reasonCounterBehind)
		}
	}
	return passed(int64(len(f.Partitions)))
}

// checkAudit is check 4: the chain of the copy through the port of the module audit, the
// verification of audit.verify from row 1 (the anchor of M7 is not built yet) to the end.
func (t *restoreRun) checkAudit(db ScratchDB) domain.CheckResult {
	in, err := db.AuditChain(t.ctx)
	if err != nil {
		return failed(reasonQuery)
	}
	res, err := t.s.deps.Chain.VerifyRows(t.ctx, in)
	if err != nil {
		return failed(reasonQuery)
	}
	if !res.OK {
		reason := res.FailureReason
		if !domain.ValidErrorCode(reason) {
			reason = reasonAuditUnknown
		}
		return failed(reason)
	}
	return passed(res.CheckedToSeq)
}

// checkThreads is check 5: in a sample of backup.restore_test.sample_threads threads the
// messages have thread_seq without gaps up to the counter of the thread.
func (t *restoreRun) checkThreads(db ScratchDB) domain.CheckResult {
	threads, err := db.ThreadSample(t.ctx, t.s.cfg.Restore.SampleThreads)
	if err != nil {
		return failed(reasonQuery)
	}
	for _, th := range threads {
		empty := th.Messages == 0 && th.LastMessageSeq == 0
		gapless := th.Messages > 0 && th.MinSeq >= 1 && th.Messages == th.MaxSeq-th.MinSeq+1 && th.MaxSeq == th.LastMessageSeq
		if !empty && !gapless {
			return failed(reasonThreadGap)
		}
	}
	return passed(int64(len(threads)))
}

// checkFiles is check 6: a sample of backup.restore_test.sample_files files is fetched from the
// object backup and its SHA-256 compared with files.sha256. With the driver s3 the objects lie
// in the mirror of the off-site bucket, which this build does not fetch: the check is skipped
// with objects_driver_s3 (docs/decisions.d/M7-03.md D-M7-03-4).
func (t *restoreRun) checkFiles(db ScratchDB) domain.CheckResult {
	if t.s.cfg.ObjectsDriver == "s3" {
		return domain.CheckResult{Result: domain.ResultSkipped, Reason: reasonObjectsDriverS3}
	}
	files, err := db.FileSample(t.ctx, t.s.cfg.Restore.SampleFiles)
	if err != nil {
		return failed(reasonQuery)
	}
	if len(files) == 0 {
		return passed(0)
	}
	keys := make([]string, len(files))
	for i, f := range files {
		keys[i] = f.ObjectKey
	}
	sums, err := t.s.deps.Scratch.FetchObjects(t.ctx, keys)
	if err != nil {
		return failed(ErrorCode(err, reasonObjectFetch))
	}
	missing := false
	for _, f := range files {
		sum, ok := sums[f.ObjectKey]
		switch {
		case !ok:
			missing = true
		case !bytes.Equal(sum[:], f.SHA256):
			return failed(reasonObjectHash)
		}
	}
	if missing {
		return failed(reasonObjectMissing)
	}
	return passed(int64(len(files)))
}
