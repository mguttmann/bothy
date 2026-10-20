// SPDX-License-Identifier: Apache-2.0

// Package postgres is the PostgreSQL adapter of the status module (M7-02): the sqlc queries of
// backup_runs in queries/, the generated code in sqlcgen/, Runs, which implements the port
// status.BackupRuns over backup_runs (docs/spec/03-data-model.md 4.12), and ManifestSource, which
// reads the facts of the backup manifest in one read-only snapshot. Only this package runs SQL on
// backup_runs (03 principle 12).
//
// Example:
//
//	runs := postgres.NewRuns(backupPool, clock.System(), ids.NewGenerator(clock.System(), rand.Reader))
//	src := postgres.NewManifestSource(superPool)
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/status"
	"github.com/OWNER/bothy/services/server/internal/status/adapters/postgres/sqlcgen"
)

// Runs implements status.BackupRuns with the generated queries of sqlcgen. bothy-backupd opens
// its pool as bothy_backup, which may only select, insert and update backup_runs; bothy migrate
// reads with its own role. It is safe for concurrent use.
type Runs struct {
	q     *sqlcgen.Queries
	clock clock.Clock
	gen   *ids.Generator
}

// Compile-time check: Runs is the port of backup_runs.
var _ status.BackupRuns = (*Runs)(nil)

// NewRuns returns Runs over pool. started_at and finished_at come from c, not from now() of the
// database, so the times of a run match the times of its manifest; gen gives the UUIDv7 ids.
func NewRuns(pool *pg.Pool, c clock.Clock, gen *ids.Generator) *Runs {
	return &Runs{q: sqlcgen.New(pool), clock: c, gen: gen}
}

// Start implements status.BackupRuns: it inserts a running row of kind with a new id and
// started_at from the clock.
func (r *Runs) Start(ctx context.Context, kind status.BackupKind) (ids.ID, error) {
	id := r.gen.New()
	if err := r.q.InsertBackupRun(ctx, sqlcgen.InsertBackupRunParams{
		ID: uuid(id), Kind: string(kind), StartedAt: ts(r.clock.Now()),
	}); err != nil {
		return ids.ID{}, fmt.Errorf("status: inserting a backup run: %w", err)
	}
	return id, nil
}

// Finish implements status.BackupRuns: it moves the running row run.ID to succeeded or failed
// with finished_at from the clock. A negative size records NULL, a nil manifest the empty object
// of a failed run, nil checks the empty object of every backup kind, an empty error NULL.
//
// It fails when no running row has the id: the run is unknown or was finished before.
func (r *Runs) Finish(ctx context.Context, run status.FinishedRun) error {
	state := "failed"
	if run.Succeeded {
		state = "succeeded"
	}
	manifest, checks := run.Manifest, run.Checks
	if manifest == nil {
		manifest = []byte("{}")
	}
	if checks == nil {
		checks = []byte("{}")
	}
	n, err := r.q.FinishBackupRun(ctx, sqlcgen.FinishBackupRunParams{
		ID:         uuid(run.ID),
		State:      state,
		FinishedAt: ts(r.clock.Now()),
		SizeBytes:  pgtype.Int8{Int64: run.SizeBytes, Valid: run.SizeBytes >= 0},
		Manifest:   manifest,
		Checks:     checks,
		Error:      pgtype.Text{String: run.Error, Valid: run.Error != ""},
	})
	if err != nil {
		return fmt.Errorf("status: finishing a backup run: %w", err)
	}
	if n != 1 {
		return errors.New("status: the backup run is not running")
	}
	return nil
}

// LastSuccess implements status.BackupRuns: the newest started_at of the succeeded rows of
// kinds. It checks first that backup_runs exists and returns status.ErrNoBackupTable when the
// schema predates M7-02.
func (r *Runs) LastSuccess(ctx context.Context, kinds ...status.BackupKind) (time.Time, bool, error) {
	exists, err := r.q.BackupRunsExists(ctx)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("status: looking up backup_runs: %w", err)
	}
	if !exists {
		return time.Time{}, false, status.ErrNoBackupTable
	}
	names := make([]string, len(kinds))
	for i, k := range kinds {
		names[i] = string(k)
	}
	last, err := r.q.LastBackupSuccess(ctx, names)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("status: reading the last successful backup: %w", err)
	}
	if !last.Valid {
		return time.Time{}, false, nil
	}
	return last.Time.UTC(), true, nil
}

// LatestSucceeded implements status.BackupRuns: the newest succeeded row of kinds by started_at
// (ties by id) with its manifest.
func (r *Runs) LatestSucceeded(ctx context.Context, kinds ...status.BackupKind) (status.BackupRun, bool, error) {
	names := make([]string, len(kinds))
	for i, k := range kinds {
		names[i] = string(k)
	}
	row, err := r.q.LatestBackupRun(ctx, names)
	if errors.Is(err, pgx.ErrNoRows) {
		return status.BackupRun{}, false, nil
	}
	if err != nil {
		return status.BackupRun{}, false, fmt.Errorf("status: reading the latest successful backup: %w", err)
	}
	return status.BackupRun{ID: ids.ID(row.ID.Bytes), Kind: status.BackupKind(row.Kind), StartedAt: row.StartedAt.Time.UTC(), Manifest: row.Manifest}, true, nil
}

// uuid converts an id to its column value.
func uuid(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

// ts converts a time to its column value.
func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
