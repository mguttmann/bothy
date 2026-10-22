// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// BackupKind is backup_runs.kind (docs/spec/03-data-model.md 4.12): the kinds of
// docs/spec/16-operations.md 8.1 and the restore test of 8.3.
type BackupKind string

// The values of backup_runs.kind.
const (
	// BackupPGFull is a full pgBackRest backup (backup.pg.full_schedule).
	BackupPGFull BackupKind = "pg_full"
	// BackupPGDiff is a differential pgBackRest backup (backup.pg.diff_schedule).
	BackupPGDiff BackupKind = "pg_diff"
	// BackupLogical is the logical dump pg_dump -Fc (backup.logical.schedule).
	BackupLogical BackupKind = "logical"
	// BackupObjects is the object backup (backup.objects.schedule).
	BackupObjects BackupKind = "objects"
	// BackupRestoreTest is the automatic restore test (M7-03).
	BackupRestoreTest BackupKind = "restore_test"
)

// ErrNoBackupTable reports a schema that has no table backup_runs yet: it was migrated by a
// version before M7-02, so no backup run can have been recorded (bothy migrate treats it as
// exempt from migrate.require_recent_backup, docs/decisions.d/M7-02.md D-M7-02-8).
var ErrNoBackupTable = errors.New("status: the table backup_runs does not exist yet")

// BackupRuns is the inbound port of the table backup_runs (02 4.5): bothy-backupd records its
// runs through it with the role bothy_backup, which may write only this table, and bothy migrate
// reads the last successful run (16 10 item 1). Implementations are safe for concurrent use.
type BackupRuns interface {
	// Start inserts a row in state running with started_at = now and returns its id.
	//
	// Example:
	//
	//	id, err := runs.Start(ctx, status.BackupPGFull)
	Start(ctx context.Context, kind BackupKind) (ids.ID, error)
	// Finish moves the running row run.ID to succeeded or failed with finished_at = now, the
	// size, the manifest and the error code. A row that is not running is an error.
	Finish(ctx context.Context, run FinishedRun) error
	// LastSuccess returns the started_at of the newest succeeded run of any of kinds and true,
	// or false when there is none. It returns ErrNoBackupTable when the table does not exist.
	LastSuccess(ctx context.Context, kinds ...BackupKind) (time.Time, bool, error)
	// LatestSucceeded returns the newest succeeded run of any of kinds by started_at with its
	// manifest and true, or false when there is none: the restore test reads the backup it
	// restores this way (M7-03).
	//
	// Example:
	//
	//	run, ok, err := runs.LatestSucceeded(ctx, status.BackupPGFull, status.BackupPGDiff)
	LatestSucceeded(ctx context.Context, kinds ...BackupKind) (BackupRun, bool, error)
}

// BackupRun is a recorded run as LatestSucceeded returns it.
type BackupRun struct {
	// ID is backup_runs.id.
	ID ids.ID
	// Kind is backup_runs.kind.
	Kind BackupKind
	// StartedAt is backup_runs.started_at.
	StartedAt time.Time
	// Manifest is backup_runs.manifest, the JSON of backup_manifest.v1.
	Manifest []byte
}

// FinishedRun is the outcome of one run as Finish records it.
type FinishedRun struct {
	// ID is the id Start returned.
	ID ids.ID
	// Succeeded selects the state succeeded, else failed.
	Succeeded bool
	// SizeBytes is backup_runs.size_bytes; a negative value records NULL (unknown).
	SizeBytes int64
	// Manifest is the JSON of backup_manifest.v1; nil records the default '{}' (a failed run).
	Manifest []byte
	// Checks is the JSON of restore_test_checks.v1 of a restore test; nil records the default
	// '{}' (every backup kind).
	Checks []byte
	// Error is the error code of a failed run, a code without content or secrets
	// (domain.ValidErrorCode); "" for a succeeded run.
	Error string
}
