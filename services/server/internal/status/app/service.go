// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/status"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// ErrBusy reports that the slot of the kind already runs (domain.Kind.Slot): at most one run per
// kind, and pg_full and pg_diff share one slot (brief M7-02 design note 1). No row is written.
var ErrBusy = errors.New("status: a backup of this kind is already running")

// localOnlyWarning is the start warning of brief M7-02 rule 15.
const localOnlyWarning = "local backup only: backup.offsite.target is empty"

// manifestTag is the restic tag of the manifest snapshots, the tag their thinning selects.
const manifestTag = "manifest"

// The keys of the log lines; their values are kinds, ids, states, codes and volume names only,
// never output text.
const (
	attrKind   = "kind"
	attrID     = "id"
	attrState  = "state"
	attrCode   = "code"
	attrVolume = "volume"
	// attrRecorded is false when the row could not be finished.
	attrRecorded = "recorded"
)

// Config is the configuration of the backup service, bound from the keys backup.* and file.* by
// cmd/bothy-backupd.
type Config struct {
	// Schedules are the parsed schedules per kind (backup.pg.full_schedule,
	// backup.pg.diff_schedule, backup.logical.schedule, backup.objects.schedule); a kind without
	// an entry is never started by the scheduler.
	Schedules map[domain.Kind]domain.Schedule
	// OffsiteEnabled reports a non-empty backup.offsite.target.
	OffsiteEnabled bool
	// ObjectsDriver is file.storage.driver: "fs" or "s3".
	ObjectsDriver string
	// FilesRoot is file.fs.root, the read-only volume files that the driver fs backs up.
	FilesRoot string
	// DumpDir is the directory of the pg_dump file, which is removed after each logical run.
	DumpDir string
	// LocalRetentionFull is backup.pg.local_retention_full, the full backups kept in repo1.
	LocalRetentionFull int
	// RetentionFull is backup.pg.retention_full, the full backups the owner's off-site expiry
	// keeps in repo2.
	RetentionFull int
	// LogicalRetention is backup.logical.retention.
	LogicalRetention time.Duration
	// ObjectsRetention is backup.objects.retention.
	ObjectsRetention time.Duration
	// ServerVersion is the version of bothy-backupd, written into each manifest.
	ServerVersion string
	// Volumes maps the metric volume name (pgdata, files, pgbackrest-repo, backup-scratch) to
	// its mount path for Sample.
	Volumes map[string]string
	// Restore is the configuration of the restore test (M7-03).
	Restore RestoreConfig
}

// Deps are the ports the service uses.
type Deps struct {
	// Runs records the runs in backup_runs.
	Runs status.BackupRuns
	// Exec runs the external steps.
	Exec Executor
	// Source reads the facts of the manifest and the WAL archive time.
	Source ManifestSource
	// Metrics records the instruments; nil records none.
	Metrics Metrics
	// Clock gives the times of the runs.
	Clock clock.Clock
	// Log receives one line per run; nil means slog.Default().
	Log *slog.Logger
	// Scratch is the scratch area of the restore test; nil leaves the restore test out.
	Scratch Scratch
	// Chain verifies the audit chain of the copy (the port of the module audit); the restore
	// test needs it with Scratch.
	Chain audit.RowsVerifier
}

// Service runs the backups of one instance. It is safe for concurrent use: each slot admits one
// run at a time.
type Service struct {
	cfg  Config
	deps Deps
	log  *slog.Logger

	mu   sync.Mutex
	busy map[string]bool
}

// New returns the service over cfg and deps. It fails when Runs, Exec, Source or Clock is nil,
// when cfg.ObjectsDriver is neither "fs" nor "s3", when the driver fs has no FilesRoot, when
// DumpDir is empty or when LocalRetentionFull is below 1; with Scratch, when Chain is nil or a
// sample size or the RTO target of cfg.Restore is not positive.
//
// Example:
//
//	svc, err := app.New(cfg, app.Deps{Runs: runs, Exec: exec, Source: src, Metrics: m, Clock: clock.System(), Log: log})
func New(cfg Config, deps Deps) (*Service, error) {
	switch {
	case deps.Runs == nil || deps.Exec == nil || deps.Source == nil || deps.Clock == nil:
		return nil, errors.New("status: the backup service needs Runs, Exec, Source and Clock")
	case cfg.ObjectsDriver != "fs" && cfg.ObjectsDriver != "s3":
		return nil, errors.New("status: file.storage.driver must be fs or s3")
	case cfg.ObjectsDriver == "fs" && cfg.FilesRoot == "":
		return nil, errors.New("status: the driver fs needs file.fs.root")
	case cfg.DumpDir == "":
		return nil, errors.New("status: the backup service needs a dump directory")
	case cfg.LocalRetentionFull < 1:
		return nil, errors.New("status: backup.pg.local_retention_full must be at least 1")
	case deps.Scratch != nil && deps.Chain == nil:
		return nil, errors.New("status: the restore test needs the audit verifier Chain")
	case deps.Scratch != nil && (cfg.Restore.SampleThreads < 1 || cfg.Restore.SampleFiles < 1 || cfg.Restore.RTOTarget <= 0):
		return nil, errors.New("status: the restore test needs backup.restore_test.sample_threads, sample_files and dr.rto_target above 0")
	}
	log := deps.Log
	if log == nil {
		log = slog.Default()
	}
	return &Service{cfg: cfg, deps: deps, log: log, busy: map[string]bool{}}, nil
}

// Warnings returns the warnings of the start log: "local backup only" without an off-site target
// (brief M7-02 rule 15), else none.
func (s *Service) Warnings() []string {
	if !s.cfg.OffsiteEnabled {
		return []string{localOnlyWarning}
	}
	return nil
}

// acquire takes the slot of kind and reports whether it was free.
func (s *Service) acquire(kind domain.Kind) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy[kind.Slot()] {
		return false
	}
	s.busy[kind.Slot()] = true
	return true
}

// release frees the slot of kind.
func (s *Service) release(kind domain.Kind) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.busy, kind.Slot())
}

// Run runs one backup of kind now: it takes the slot of the kind, records a backup_runs row,
// runs the steps of the kind, builds and stores the manifest and finishes the row as succeeded
// with the size and the manifest, or as failed with an error code and no manifest.
//
// It returns ErrBusy without a row when the slot is taken, the error of Runs.Start when no row
// could be written, and otherwise the error of the failed step (nil on success). A canceled ctx
// fails the run with the code "canceled"; the row is finished in every case, with a context that
// is not canceled, so no row stays running.
//
// Example:
//
//	err := svc.Run(ctx, domain.KindLogical)
func (s *Service) Run(ctx context.Context, kind domain.Kind) error {
	if _, ok := domain.ParseKind(string(kind)); !ok {
		return fmt.Errorf("status: %q is not a backup kind", kind)
	}
	if !s.acquire(kind) {
		return ErrBusy
	}
	defer s.release(kind)
	return s.run(ctx, kind)
}

// outcome is what the steps of a kind produce for the manifest and the row.
type outcome struct {
	size     int64
	label    string
	snapshot string
}

// run is Run with the slot already taken.
func (s *Service) run(ctx context.Context, kind domain.Kind) error {
	started := s.deps.Clock.Now()
	id, err := s.deps.Runs.Start(ctx, status.BackupKind(kind))
	if err != nil {
		s.log.LogAttrs(ctx, slog.LevelError, "backup run not recorded", slog.String(attrKind, string(kind)))
		return fmt.Errorf("status: recording the start of the %s backup: %w", kind, err)
	}
	manifest, out, code, runErr := s.steps(ctx, kind, id, started)
	if runErr != nil && ctx.Err() != nil {
		code = "canceled"
	}

	// The row is finished even when ctx was canceled: a run that stays running would block the
	// alert on the last success and the next restore test.
	fctx := context.WithoutCancel(ctx)
	run := status.FinishedRun{ID: id, Succeeded: runErr == nil, SizeBytes: out.size, Manifest: manifest, Error: code}
	metricSize := out.size
	if runErr != nil {
		run.SizeBytes, run.Manifest, metricSize = -1, nil, 0
	}
	finishErr := s.deps.Runs.Finish(fctx, run)
	finishedAt := s.deps.Clock.Now()
	state := "succeeded"
	if runErr != nil {
		state = "failed"
	}
	level := slog.LevelInfo
	if runErr != nil || finishErr != nil {
		level = slog.LevelError
	}
	s.log.LogAttrs(fctx, level, "backup run finished", slog.String(attrKind, string(kind)), slog.String(attrID, id.String()),
		slog.String(attrState, state), slog.String(attrCode, code), slog.Bool(attrRecorded, finishErr == nil))
	if s.deps.Metrics != nil {
		s.deps.Metrics.RunFinished(fctx, kind, runErr == nil, finishedAt.Sub(started), metricSize, finishedAt)
	}
	if runErr != nil {
		return runErr
	}
	if finishErr != nil {
		return fmt.Errorf("status: recording the end of the %s backup: %w", kind, finishErr)
	}
	return nil
}

// steps runs the backup of kind, then builds and stores the manifest and, without an off-site
// target, thins the local restic snapshots. It returns the manifest JSON, the outcome, and on
// failure the error code and the error.
func (s *Service) steps(ctx context.Context, kind domain.Kind, id ids.ID, started time.Time) ([]byte, outcome, string, error) {
	var (
		out      outcome
		code     string
		err      error
		retain   time.Duration
		thinning bool
	)
	switch kind {
	case domain.KindPGFull, domain.KindPGDiff:
		out, code, err = s.physical(ctx, kind)
	case domain.KindLogical:
		out, code, err = s.logical(ctx, id)
		retain, thinning = s.cfg.LogicalRetention, true
	case domain.KindObjects:
		out, code, err = s.objects(ctx)
		retain, thinning = s.cfg.ObjectsRetention, true
	}
	if err != nil {
		return nil, out, code, err
	}
	manifest, err := s.manifest(ctx, kind, id, started, out)
	if err != nil {
		return nil, out, "manifest_failed", err
	}
	if _, err := s.deps.Exec.ResticBackup(ctx, ResticBackup{
		Stdin: manifest, StdinName: "manifest-" + id.String() + ".json",
		Tags: []string{manifestTag, string(kind), id.String()},
	}); err != nil {
		// The backup itself exists; the code says that only its description is missing.
		return nil, out, "manifest_store_failed", err
	}
	if thinning && !s.cfg.OffsiteEnabled {
		// Without an off-site target the restic repository is local and nobody else expires it;
		// with one, only the owner's account may forget off-site (R-OP-09).
		for _, f := range []ResticForget{{Tag: string(kind), KeepWithin: retain}, {Tag: manifestTag, KeepWithin: s.manifestRetention()}} {
			if err := s.deps.Exec.ResticForget(ctx, f); err != nil {
				return nil, out, ErrorCode(err, "restic_failed"), err
			}
		}
	}
	return manifest, out, "", nil
}

// manifestRetention keeps a manifest as long as the longest-lived backup it may describe.
func (s *Service) manifestRetention() time.Duration {
	return max(s.cfg.LogicalRetention, s.cfg.ObjectsRetention)
}

// physical runs a pgBackRest backup into repo1, into repo2 with an off-site target, and expires
// repo1. Label and size come from repo1, the copy the restore test reads first.
func (s *Service) physical(ctx context.Context, kind domain.Kind) (outcome, string, error) {
	typ := "full"
	if kind == domain.KindPGDiff {
		typ = "diff"
	}
	res, err := s.deps.Exec.PgBackRestBackup(ctx, PgBackRestBackup{Type: typ, Repo: 1})
	if err != nil {
		return outcome{}, ErrorCode(err, "pgbackrest_failed"), err
	}
	if s.cfg.OffsiteEnabled {
		if _, err := s.deps.Exec.PgBackRestBackup(ctx, PgBackRestBackup{Type: typ, Repo: 2}); err != nil {
			return outcome{}, ErrorCode(err, "pgbackrest_failed"), err
		}
	}
	if err := s.deps.Exec.PgBackRestExpireLocal(ctx, s.cfg.LocalRetentionFull); err != nil {
		return outcome{}, ErrorCode(err, "pgbackrest_failed"), err
	}
	return outcome{size: res.SizeBytes, label: res.Label}, "", nil
}

// logical dumps the database into DumpDir and stores the file with restic. The file is removed
// in every case: it holds the whole content of the database and must not outlive the run.
func (s *Service) logical(ctx context.Context, id ids.ID) (outcome, string, error) {
	path := filepath.Join(s.cfg.DumpDir, "bothy-"+id.String()+".dump")
	defer func() {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			s.log.LogAttrs(context.WithoutCancel(ctx), slog.LevelError, "backup dump file not removed", slog.String(attrID, id.String()))
		}
	}()
	if _, err := s.deps.Exec.PgDump(ctx, path); err != nil {
		return outcome{}, ErrorCode(err, "pg_dump_failed"), err
	}
	res, err := s.deps.Exec.ResticBackup(ctx, ResticBackup{Paths: []string{path}, Tags: []string{string(domain.KindLogical)}})
	if err != nil {
		return outcome{}, ErrorCode(err, "restic_failed"), err
	}
	return outcome{size: res.SizeBytes, snapshot: res.SnapshotID}, "", nil
}

// objects backs up the objects: restic over FilesRoot for the driver fs, the mirror without
// deletion for the driver s3 (brief M7-02 design note 5), which has no restic snapshot.
func (s *Service) objects(ctx context.Context) (outcome, string, error) {
	if s.cfg.ObjectsDriver == "s3" {
		res, err := s.deps.Exec.S3Mirror(ctx)
		if err != nil {
			return outcome{}, ErrorCode(err, "s3_mirror_failed"), err
		}
		return outcome{size: res.SizeBytes}, "", nil
	}
	res, err := s.deps.Exec.ResticBackup(ctx, ResticBackup{Paths: []string{s.cfg.FilesRoot}, Tags: []string{string(domain.KindObjects)}})
	if err != nil {
		return outcome{}, ErrorCode(err, "restic_failed"), err
	}
	return outcome{size: res.SizeBytes, snapshot: res.SnapshotID}, "", nil
}

// manifest reads the facts and returns the validated JSON of backup_manifest.v1 for the run.
func (s *Service) manifest(ctx context.Context, kind domain.Kind, id ids.ID, started time.Time, out outcome) ([]byte, error) {
	facts, err := s.deps.Source.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	counts := facts.RowCounts
	if counts == nil {
		counts = map[string]int64{}
	}
	m := domain.Manifest{
		Schema:        domain.ManifestSchema,
		BackupID:      id.String(),
		Kind:          kind,
		StartedAt:     started.UTC(),
		FinishedAt:    s.deps.Clock.Now().UTC(),
		ServerVersion: s.cfg.ServerVersion,
		SchemaVersion: facts.SchemaVersion,
		InstanceID:    facts.InstanceID,
		SyncEpoch:     facts.SyncEpoch,
		EventLog:      domain.EventLogBounds{HeadSeq: facts.HeadSeq, MinRetainedSeq: facts.MinRetainedSeq},
		Audit:         domain.AuditHead{LastSeq: facts.AuditLastSeq, LastHash: hex.EncodeToString(facts.AuditLastHash)},
		RowCounts:     counts,
		Files:         domain.FileTotals{Count: facts.FileCount, TotalBytes: facts.FileBytes},
	}
	if out.label != "" {
		m.PgBackRest = &domain.PgBackRestRef{Label: out.label}
	}
	if out.snapshot != "" {
		m.Restic = &domain.ResticRef{SnapshotID: out.snapshot}
	}
	return m.Marshal()
}

// metricKinds are the kinds of the attribute bothy.backup.kind with the backup_runs kinds they
// cover; the physical one is reported as pg_full, which the metrics map to physical. The restore
// test has its own gauge (KindRestoreTest).
var metricKinds = []struct {
	kind  domain.Kind
	kinds []status.BackupKind
}{
	{domain.KindPGFull, []status.BackupKind{status.BackupPGFull, status.BackupPGDiff}},
	{domain.KindLogical, []status.BackupKind{status.BackupLogical}},
	{domain.KindObjects, []status.BackupKind{status.BackupObjects}},
	{domain.KindRestoreTest, []status.BackupKind{status.BackupRestoreTest}},
}

// Prime records the last success of each metric kind (physical, logical, objects) and of the
// restore test read from backup_runs, so bothy.backup.last_success.timestamp and
// bothy.backup.restore_test.last_success.timestamp are right from the start of the process and
// not only after the next run. A kind without a success records nothing.
//
// It returns the first error of Runs.LastSuccess, for example status.ErrNoBackupTable; without
// Metrics it does nothing.
func (s *Service) Prime(ctx context.Context) error {
	if s.deps.Metrics == nil {
		return nil
	}
	for _, mk := range metricKinds {
		at, ok, err := s.deps.Runs.LastSuccess(ctx, mk.kinds...)
		if err != nil {
			return fmt.Errorf("status: reading the last %s backup: %w", mk.kind, err)
		}
		if ok {
			s.deps.Metrics.LastSuccess(ctx, mk.kind, at)
		}
	}
	return nil
}

// Sample records the time of the last archived WAL segment and the free and total bytes of each
// volume in Config.Volumes. A value that cannot be read is logged and skipped; Sample never
// fails, because a missing sample must not stop the backups. Without Metrics it does nothing.
func (s *Service) Sample(ctx context.Context) {
	if s.deps.Metrics == nil {
		return
	}
	at, ok, err := s.deps.Source.LastArchived(ctx)
	switch {
	case err != nil:
		s.log.LogAttrs(ctx, slog.LevelWarn, "backup sample of the WAL archive failed")
	case ok:
		s.deps.Metrics.WALArchived(ctx, at)
	}
	for _, name := range slices.Sorted(maps.Keys(s.cfg.Volumes)) {
		usage, err := s.deps.Exec.VolumeUsage(ctx, s.cfg.Volumes[name])
		if err != nil {
			s.log.LogAttrs(ctx, slog.LevelWarn, "backup sample of a volume failed", slog.String(attrVolume, name),
				slog.String(attrCode, ErrorCode(err, "volume_usage_failed")))
			continue
		}
		s.deps.Metrics.Volume(ctx, name, usage)
	}
}

// ExpireOffsite is the owner's off-site expiry (R-OP-09, brief M7-02 design note 4): with the
// credentials in adminDir it expires repo2 keeping backup.pg.retention_full full backups and
// forgets the off-site restic snapshots of logical (backup.logical.retention), objects
// (backup.objects.retention) and the manifests (the larger of both). Every step runs even when an
// earlier one fails.
//
// It returns an error without any step when no off-site target is set or adminDir is empty, and
// otherwise the joined errors of the steps.
//
// Example:
//
//	err := svc.ExpireOffsite(ctx, "/run/owner/offsite-admin")
func (s *Service) ExpireOffsite(ctx context.Context, adminDir string) error {
	if !s.cfg.OffsiteEnabled {
		return errors.New("status: off-site expiry needs backup.offsite.target")
	}
	if adminDir == "" {
		return errors.New("status: off-site expiry needs the owner's credentials directory")
	}
	var errs []error
	if err := s.deps.Exec.PgBackRestExpireOffsite(ctx, adminDir, s.cfg.RetentionFull); err != nil {
		errs = append(errs, err)
	}
	for _, f := range []ResticForget{
		{Tag: string(domain.KindLogical), KeepWithin: s.cfg.LogicalRetention, AdminDir: adminDir},
		{Tag: string(domain.KindObjects), KeepWithin: s.cfg.ObjectsRetention, AdminDir: adminDir},
		{Tag: manifestTag, KeepWithin: s.manifestRetention(), AdminDir: adminDir},
	} {
		if err := s.deps.Exec.ResticForget(ctx, f); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
