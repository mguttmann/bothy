// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// Executor runs the external steps of a backup, one method per step (brief M7-02 design note 2).
// adapters/executor runs the binaries with arguments as a list, never through a shell, and maps
// exit codes to error codes without output text; adapters/s3mirror implements S3Mirror. Unit
// tests use a fake. Every error a method returns is an *ExecError or wraps one.
type Executor interface {
	// PgBackRestBackup runs "pgbackrest backup" of req.Type into repository req.Repo (1 local,
	// 2 off-site) and returns the label and the size of the new backup set.
	PgBackRestBackup(ctx context.Context, req PgBackRestBackup) (PgBackRestResult, error)
	// PgBackRestExpireLocal runs "pgbackrest expire" on repo1 only, keeping keepFull full
	// backups (backup.pg.local_retention_full).
	PgBackRestExpireLocal(ctx context.Context, keepFull int) error
	// PgBackRestExpireOffsite runs "pgbackrest expire" on repo2 with the owner's credentials in
	// adminDir, keeping keepFull full backups (backup.pg.retention_full). Only the owner's
	// bothy-backupd expire --offsite calls it (R-OP-09).
	PgBackRestExpireOffsite(ctx context.Context, adminDir string, keepFull int) error
	// PgDump writes "pg_dump -Fc" of the database to path, connecting as
	// migrate.bootstrap_user, and returns the size of the file.
	PgDump(ctx context.Context, path string) (int64, error)
	// ResticBackup stores req in the restic repository of the instance (off-site when
	// backup.offsite.target is set, else local), initialising the repository on first use, and
	// returns the snapshot id and the bytes it added.
	ResticBackup(ctx context.Context, req ResticBackup) (ResticResult, error)
	// ResticForget forgets and prunes the snapshots of req.Tag older than req.KeepWithin. With
	// req.AdminDir it acts on the off-site repository with the owner's credentials; without, on
	// the local repository, which exists only when no off-site target is set.
	ResticForget(ctx context.Context, req ResticForget) error
	// S3Mirror copies every object of the file bucket that the off-site bucket lacks and never
	// deletes (driver s3, brief M7-02 design note 5).
	S3Mirror(ctx context.Context) (MirrorResult, error)
	// VolumeUsage returns the free and the total bytes of the file system at path.
	VolumeUsage(ctx context.Context, path string) (VolumeUsage, error)
}

// PgBackRestBackup is one pgBackRest backup.
type PgBackRestBackup struct {
	// Type is "full" or "diff".
	Type string
	// Repo is the repository index: 1 local, 2 off-site.
	Repo int
}

// PgBackRestResult is the new backup set.
type PgBackRestResult struct {
	// Label is the backup label, for example 20261019-020000F.
	Label string
	// SizeBytes is the size the set added to the repository (info "repository delta").
	SizeBytes int64
}

// ResticBackup is one restic snapshot: either Paths or Stdin.
type ResticBackup struct {
	// Paths are the files or directories to store.
	Paths []string
	// Stdin, when not nil, is stored as one file named StdinName.
	Stdin     []byte
	StdinName string
	// Tags are the snapshot tags, for example "logical" or "manifest".
	Tags []string
}

// ResticResult is the stored snapshot.
type ResticResult struct {
	// SnapshotID is the full snapshot id (64 lowercase hex characters).
	SnapshotID string
	// SizeBytes is the data the snapshot added to the repository.
	SizeBytes int64
}

// ResticForget selects the snapshots to forget.
type ResticForget struct {
	// Tag is the tag of the snapshots, for example "logical".
	Tag string
	// KeepWithin keeps every snapshot younger than this.
	KeepWithin time.Duration
	// AdminDir is the owner's off-site credentials directory; "" for the local repository.
	AdminDir string
}

// MirrorResult is the outcome of one mirror pass.
type MirrorResult struct {
	// Copied is the number of objects copied in this pass.
	Copied int64
	// SizeBytes is the sum of their sizes.
	SizeBytes int64
}

// VolumeUsage is the space of one file system.
type VolumeUsage struct {
	FreeBytes uint64
	SizeBytes uint64
}

// ExecError is the error of an external step: a code without output text (brief M7-02 rule 12),
// for example "pgbackrest_exit_82", "pg_dump_exit_1", "restic_exit_10" or "s3_mirror_failed".
type ExecError struct {
	// Code is the error code; domain.ValidErrorCode holds it.
	Code string
}

// Error returns the code.
func (e *ExecError) Error() string { return "status: " + e.Code }

// ErrorCode returns the code of the *ExecError in err's chain, or fallback when there is none
// or its code is not a valid error code.
func ErrorCode(err error, fallback string) string {
	var e *ExecError
	if errors.As(err, &e) && domain.ValidErrorCode(e.Code) {
		return e.Code
	}
	return fallback
}

// ManifestSource reads the facts of the manifest from the database as migrate.bootstrap_user
// (brief M7-02 design note 3); adapters/postgres implements it.
type ManifestSource interface {
	// Snapshot reads every fact in one READ ONLY transaction with REPEATABLE READ, so the counts
	// belong to one snapshot: count(*) per table of the domain schema (partitions are counted
	// through their parent), head and smallest seq of event_log, the last audit_events row,
	// instance and the totals of files with deleted_at IS NULL.
	Snapshot(ctx context.Context) (ManifestFacts, error)
	// LastArchived returns pg_stat_archiver.last_archived_time and true, or false when no WAL
	// segment was archived yet (bothy.backup.wal.last_archived.timestamp).
	LastArchived(ctx context.Context) (time.Time, bool, error)
}

// ManifestFacts are the database facts of a manifest.
type ManifestFacts struct {
	SchemaVersion  int64
	InstanceID     string
	SyncEpoch      string
	HeadSeq        int64
	MinRetainedSeq int64
	AuditLastSeq   int64
	// AuditLastHash is audit_events.hash of the last row; nil when the log is empty.
	AuditLastHash []byte
	RowCounts     map[string]int64
	FileCount     int64
	FileBytes     int64
}

// Metrics records the instruments bothy.backup.* and bothy.storage.volume.* of
// docs/observability/signals.md 4.6; adapters/otelmetrics implements it.
type Metrics interface {
	// RunFinished records one finished run: duration and size of the last run of its kind and,
	// when it succeeded, its finish time as the last success. For domain.KindRestoreTest it
	// records the duration and the last success of the restore test and no size.
	RunFinished(ctx context.Context, kind domain.Kind, succeeded bool, duration time.Duration, sizeBytes int64, finishedAt time.Time)
	// LastSuccess records the last success of kind read from backup_runs at start.
	LastSuccess(ctx context.Context, kind domain.Kind, at time.Time)
	// WALArchived records the time the last WAL segment was archived.
	WALArchived(ctx context.Context, at time.Time)
	// Volume records the free and total bytes of a volume: pgdata, files, pgbackrest-repo or
	// backup-scratch.
	Volume(ctx context.Context, volume string, usage VolumeUsage)
	// RestoreChecksFailed adds one to bothy.backup.restore_test.failures per failed check of a
	// restore test, with bothy.reason set to the check name (alerting.md). RunFinished and
	// LastSuccess with domain.KindRestoreTest record its duration and its last success.
	RestoreChecksFailed(ctx context.Context, checks []domain.CheckName)
}
