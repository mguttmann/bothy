// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Repository implements app.Repository over a pool. It is safe for concurrent use.
type Repository struct {
	pool *pg.Pool
}

// Compile-time checks: Repository is the outgoing port of app and of bothy restore finalize.
var (
	_ app.Repository        = (*Repository)(nil)
	_ app.RestoreRepository = (*Repository)(nil)
)

// NewRepository returns the repository over pool; inside a unit of work every statement uses the
// ambient transaction instead (pg.Querier).
func NewRepository(pool *pg.Pool) *Repository {
	return &Repository{pool: pool}
}

// queries returns the sqlc queries on the ambient transaction of ctx or on the pool.
func (r *Repository) queries(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(pg.Querier(ctx, r.pool))
}

// AdvanceCounter locks and raises a counter (app.Repository). It needs a unit of work, because
// a lock outside one would end with its autocommit and order nothing. A missing row (a name the
// table does not hold) is kind internal.
func (r *Repository) AdvanceCounter(ctx context.Context, name string, n int64) (int64, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return 0, err
	}
	v, err := sqlcgen.New(tx).AdvanceCounter(ctx, sqlcgen.AdvanceCounterParams{N: n, Name: name})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, errs.Wrap(errs.CodeInternalError, err, "instance: the counter row is missing")
	}
	return v, err
}

// CounterValue reads a counter without a lock (app.Repository).
func (r *Repository) CounterValue(ctx context.Context, name string) (int64, error) {
	v, err := r.queries(ctx).CounterValue(ctx, name)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, errs.Wrap(errs.CodeInternalError, err, "instance: the counter row is missing")
	}
	return v, err
}

// LoadInstance reads the instance row (app.Repository).
func (r *Repository) LoadInstance(ctx context.Context) (instance.Record, bool, error) {
	row, err := r.queries(ctx).LoadInstance(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return instance.Record{}, false, nil
	}
	if err != nil {
		return instance.Record{}, false, err
	}
	return toRecord(sqlcgen.LockInstanceRow(row)), true, nil
}

// LockInstance reads the instance row and locks it until the end of the unit of work
// (app.Repository); it needs one, because the lock would end with an autocommit.
func (r *Repository) LockInstance(ctx context.Context) (instance.Record, bool, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return instance.Record{}, false, err
	}
	row, err := sqlcgen.New(tx).LockInstance(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return instance.Record{}, false, nil
	}
	if err != nil {
		return instance.Record{}, false, err
	}
	return toRecord(row), true, nil
}

// CompleteSetup sets setup_completed_at = at unless it is set (app.Repository) and reports
// whether the row changed; it needs a unit of work.
func (r *Repository) CompleteSetup(ctx context.Context, at time.Time) (bool, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return false, err
	}
	n, err := sqlcgen.New(tx).CompleteSetup(ctx, timestamptz(at))
	return n == 1, err
}

// toRecord converts the columns of the instance row.
func toRecord(row sqlcgen.LockInstanceRow) instance.Record {
	rec := instance.Record{
		InstanceID:       ids.ID(row.InstanceID.Bytes),
		SchemaVersion:    row.SchemaVersion,
		MinServerVersion: row.MinServerVersion.String,
		SyncEpoch:        ids.ID(row.SyncEpoch.Bytes),
		CreatedAt:        row.CreatedAt.Time.UTC(),
	}
	if row.SetupCompletedAt.Valid {
		rec.SetupCompletedAt = row.SetupCompletedAt.Time.UTC()
	}
	if row.PgSystemIdentifier.Valid {
		rec.DatabaseIdentity = instance.DatabaseIdentity{SystemIdentifier: row.PgSystemIdentifier.String, TimelineID: row.PgTimelineID.Int64}
	}
	return rec
}

// EnsureInstance inserts the row unless it exists (app.Repository).
func (r *Repository) EnsureInstance(ctx context.Context, instanceID, syncEpoch ids.ID) error {
	return r.queries(ctx).EnsureInstance(ctx, sqlcgen.EnsureInstanceParams{
		InstanceID: uuid(instanceID), SyncEpoch: uuid(syncEpoch),
	})
}

// SetVersions records the versions unless a larger schema_version is recorded (app.Repository).
func (r *Repository) SetVersions(ctx context.Context, schemaVersion int64, minServerVersion string) error {
	return r.queries(ctx).SetVersions(ctx, sqlcgen.SetVersionsParams{SchemaVersion: schemaVersion, MinServerVersion: minServerVersion})
}

// ClusterIdentity reads the identity of the database cluster (app.Repository).
func (r *Repository) ClusterIdentity(ctx context.Context) (instance.DatabaseIdentity, error) {
	row, err := r.queries(ctx).ClusterIdentity(ctx)
	if err != nil {
		return instance.DatabaseIdentity{}, err
	}
	return instance.DatabaseIdentity{SystemIdentifier: row.SystemIdentifier, TimelineID: row.TimelineID}, nil
}

// FillIdentity stores id while the identity columns are empty (app.Repository).
func (r *Repository) FillIdentity(ctx context.Context, id instance.DatabaseIdentity) (bool, error) {
	n, err := r.queries(ctx).FillIdentity(ctx, sqlcgen.FillIdentityParams{SystemIdentifier: id.SystemIdentifier, TimelineID: id.TimelineID})
	return n == 1, err
}

// FinalizeInstance sets the epoch and the identity in the unit of work of ctx
// (app.RestoreRepository).
func (r *Repository) FinalizeInstance(ctx context.Context, syncEpoch ids.ID, id instance.DatabaseIdentity) error {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return err
	}
	n, err := sqlcgen.New(tx).FinalizeInstance(ctx, sqlcgen.FinalizeInstanceParams{
		SyncEpoch: uuid(syncEpoch), SystemIdentifier: id.SystemIdentifier, TimelineID: id.TimelineID,
	})
	if err != nil {
		return err
	}
	if n != 1 {
		return errs.New(errs.CodeNotFound, `the instance row is missing; run "bothy migrate up"`)
	}
	return nil
}

// OtherSessionsOfRole counts the other sessions of role on this database
// (app.RestoreRepository).
func (r *Repository) OtherSessionsOfRole(ctx context.Context, role string) (int64, error) {
	return r.queries(ctx).OtherSessionsOfRole(ctx, role)
}

// uuid converts an ID to the pgx type.
func uuid(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }
