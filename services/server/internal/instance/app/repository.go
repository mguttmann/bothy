// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Repository is the outgoing port to the tables instance and sequence_counters. Statements run
// in the ambient transaction of ctx when there is one. adapters/postgres implements it.
type Repository interface {
	// AdvanceCounter runs UPDATE sequence_counters SET value = value + n WHERE name = name
	// RETURNING value, which holds the row lock until the commit, and returns the new value. It
	// requires a unit of work. The error is the database error, also for a missing row.
	AdvanceCounter(ctx context.Context, name string, n int64) (int64, error)
	// CounterValue reads the value of a counter without a lock.
	CounterValue(ctx context.Context, name string) (int64, error)
	// LoadInstance reads the instance row; found is false when it is missing.
	LoadInstance(ctx context.Context) (rec instance.Record, found bool, err error)
	// LockInstance reads the instance row like LoadInstance and locks it until the end of the unit
	// of work (SELECT ... FOR UPDATE). It requires a unit of work.
	LockInstance(ctx context.Context) (rec instance.Record, found bool, err error)
	// CompleteSetup sets setup_completed_at = at where it is NULL and reports whether it changed
	// the row. It requires a unit of work.
	CompleteSetup(ctx context.Context, at time.Time) (changed bool, err error)
	// EnsureInstance inserts the row with the given IDs unless it exists
	// (INSERT ... ON CONFLICT (id) DO NOTHING), so concurrent bothy migrate runs are safe.
	EnsureInstance(ctx context.Context, instanceID, syncEpoch ids.ID) error
	// SetVersions sets schema_version and min_server_version unless the row records a larger
	// schema_version (an older binary must never lower what a newer one wrote).
	SetVersions(ctx context.Context, schemaVersion int64, minServerVersion string) error
	// ClusterIdentity reads the current identity of the database cluster from
	// pg_control_system() and pg_control_checkpoint() (R-DM-50).
	ClusterIdentity(ctx context.Context) (instance.DatabaseIdentity, error)
	// FillIdentity stores id in the instance row while both identity columns are empty and
	// reports whether it changed the row (bothy migrate up, 03 4.1).
	FillIdentity(ctx context.Context, id instance.DatabaseIdentity) (bool, error)
}

// RestoreRepository is the outgoing port of bothy restore finalize to the tables instance and
// sequence_counters and to the session view of the database. adapters/postgres implements it.
type RestoreRepository interface {
	// LoadInstance and LockInstance as in Repository.
	LoadInstance(ctx context.Context) (rec instance.Record, found bool, err error)
	LockInstance(ctx context.Context) (rec instance.Record, found bool, err error)
	// CounterValue reads a counter without a lock; event_seq is head_seq.
	CounterValue(ctx context.Context, name string) (int64, error)
	// ClusterIdentity as in Repository.
	ClusterIdentity(ctx context.Context) (instance.DatabaseIdentity, error)
	// FinalizeInstance sets sync_epoch and the identity of the instance row; it requires a unit
	// of work. A missing row is kind not_found.
	FinalizeInstance(ctx context.Context, syncEpoch ids.ID, id instance.DatabaseIdentity) error
	// OtherSessionsOfRole counts the sessions of role on the current database besides the
	// caller's own (pg_stat_activity).
	OtherSessionsOfRole(ctx context.Context, role string) (int64, error)
	// AdvanceCounter as in Repository: bothy restore finalize raises permission_version once
	// after the deletion journal is applied (SEC-BAK-3, M7-07).
	AdvanceCounter(ctx context.Context, name string, n int64) (int64, error)
}
