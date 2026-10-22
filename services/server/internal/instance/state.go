// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Record is the single row of the table instance (docs/spec/03-data-model.md "instance"): the
// identity of this installation, its setup state and the schema lock of bothy migrate.
type Record struct {
	// InstanceID identifies the installation (issuer of tokens); created by the first
	// bothy migrate up and never changed.
	InstanceID ids.ID
	// SetupCompletedAt is when the first owner was created (08 "Initial installation"); the zero
	// time while setup is pending. Never rendered as JSON null (ADR 0053).
	SetupCompletedAt time.Time
	// SchemaVersion is the last goose migration bothy migrate recorded; for display only.
	SchemaVersion int64
	// MinServerVersion is the smallest server version (SemVer) allowed to run against the
	// schema; empty when no binary has set it.
	MinServerVersion string
	// SyncEpoch names the timeline of the event log; new after every restore (03 6.7).
	SyncEpoch ids.ID
	// DatabaseIdentity is the stored identity of the database cluster (R-DM-50); zero while
	// empty (an instance migrated before M7-04, until the next bothy migrate up fills it).
	DatabaseIdentity DatabaseIdentity
	// CreatedAt is when the row was created.
	CreatedAt time.Time
}

// SetupCompleted reports whether the first owner exists (setup_completed_at is set). Without
// it only the setup path starts (02 "Start" step 5).
func (r Record) SetupCompleted() bool { return !r.SetupCompletedAt.IsZero() }

// State is the port of the instance row. bothy migrate up writes it through RecordMigration, the
// start reads it through Load.
type State interface {
	// Load returns the instance row. Errors: kind not_found when the row is missing (the
	// database was never migrated by a binary with this module: run bothy migrate up), the
	// database error otherwise.
	Load(ctx context.Context) (Record, error)
	// RecordMigration creates the row if it is missing, with new UUIDv7 values for instance_id
	// and sync_epoch, and sets schema_version and min_server_version. It never changes
	// instance_id, sync_epoch, setup_completed_at or created_at, and it never lowers
	// schema_version: a call with a smaller schemaVersion than recorded (an older binary racing
	// a newer one) leaves the row unchanged. While the stored database identity is empty it
	// stores the current one (R-DM-50); a stored identity changes only through Restore.Finalize.
	// It returns the row as it is afterwards.
	//
	// It runs in a unit of work of its own, or joins the ambient one when ctx is inside a unit.
	// Errors: kind internal for schemaVersion < 1 or a minServerVersion that is not SemVer;
	// the database error otherwise.
	RecordMigration(ctx context.Context, schemaVersion int64, minServerVersion string) (Record, error)
	// LockForSetup returns the instance row locked until the end of the ambient unit of work, so
	// two runs of bothy setup bootstrap-owner serialise and the second sees the first one's
	// setup_completed_at (08 "Initial installation" step 1, M1-37). Errors: kind internal outside a
	// unit of work; kind not_found when the row is missing (run bothy migrate up); the database
	// error otherwise.
	LockForSetup(ctx context.Context) (Record, error)
	// CompleteSetup sets setup_completed_at = at inside the ambient unit of work (08
	// "Initial installation" step 6). Setup never reverts: a row whose setup_completed_at is set
	// stays unchanged and the call fails with kind conflict. Errors: kind internal outside a unit
	// of work; the database error otherwise.
	CompleteSetup(ctx context.Context, at time.Time) error
	// CheckIdentity reads the stored database identity of the instance row and the current one
	// of the database and compares them, the start lock of R-DM-50 (an empty stored identity
	// never mismatches). Errors: kind not_found without the instance row, the database error
	// otherwise.
	CheckIdentity(ctx context.Context) (IdentityCheck, error)
}
