// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// ErrNotSchemaOwner reports that bothy migrate runs as a role that does not own schema public
// (docs/spec/16-operations.md "Roles" rule 4): the migrations would create tables the owner role
// does not own, or fail half way. The text is the message the command prints.
var ErrNotSchemaOwner = errors.New("migrate must run as the schema owner; set database.user")

// The advisory lock poll of goose's session locker: every second, up to five minutes. A second
// bothy migrate up waits that long for the first one and then finds nothing left to apply.
const (
	lockPollSeconds  = 1
	lockPollAttempts = 300
)

// The same poll for the advisory lock of a Step that serialises itself (platform/jobs), so a
// concurrent bothy migrate up waits as long in every part of the run; lock_timeout would end
// pg_advisory_lock after migrate.lock_timeout instead.
const (
	// MigrateLockPoll is the pause between two attempts to take the lock.
	MigrateLockPoll = lockPollSeconds * time.Second
	// MigrateLockAttempts is the number of attempts before the step gives up.
	MigrateLockAttempts = lockPollAttempts
)

// Step is work that bothy migrate runs before or after the goose migrations, in the same
// process and against the same database: River's own migrations before (M1-14,
// docs/operations/upgrade.md rule M11), the instance row with schema_version and
// min_server_version after (M1-15). A step must be idempotent, because every run of
// migrate up runs it, also when no migration is pending, and must tolerate a second migrate up
// running it at the same time: steps run outside goose's migration lock.
type Step interface {
	// Name names the step in errors and in the output of bothy migrate.
	Name() string
	// Run does the work; db connects as the schema owner with the session settings of migrate.
	Run(ctx context.Context, db *sql.DB) error
}

// MigratorOptions are the steps around the goose migrations, each list run in order.
type MigratorOptions struct {
	// Before runs before goose, after the owner and version checks.
	Before []Step
	// After runs after goose has applied every pending migration.
	After []Step
}

// Result is the outcome of Migrator.Up.
type Result struct {
	// Applied lists the migrations this run applied, in order; empty when none was pending.
	Applied []AppliedMigration
	// Version is the schema version of the database after the run.
	Version int64
	// Newer reports a database whose version is above the highest embedded migration: it
	// belongs to a newer binary, so nothing ran (upgrade.md "Rollback").
	Newer bool
}

// AppliedMigration is one migration applied by Up.
type AppliedMigration struct {
	// Version is the number of the migration, for example 1 for 00001_base_privileges.sql.
	Version int64
	// Name is the part of the file name after the version, for example "base_privileges".
	Name string
	// Duration is how long the migration took.
	Duration time.Duration
}

// MigrationStatus is one embedded migration and whether the database has it.
type MigrationStatus struct {
	// Version is the number of the migration.
	Version int64
	// Name is the part of the file name after the version.
	Name string
	// Applied reports whether the database has the migration.
	Applied bool
	// AppliedAt is when it was applied, in UTC; the zero time when it is pending.
	AppliedAt time.Time
}

// Migrator runs the embedded goose migrations of one file system against one database
// (ADR 0007). Two migrators on the same database, also in two processes, apply each migration
// once: goose holds a PostgreSQL advisory lock while it migrates (session locker). The steps run
// outside that lock, so each Step must tolerate a concurrent run of itself (River's migrator
// and an upsert of the instance row do). Each migration runs in its own transaction unless the
// file is marked -- +goose NO TRANSACTION.
type Migrator struct {
	db       *sql.DB
	opts     MigratorOptions
	files    []migrationFile
	provider *goose.Provider
}

// NewMigrator returns a migrator for the migrations fsys on db. db should connect as the schema
// owner (bothy_owner) with the session settings of bothy migrate: lock_timeout =
// migrate.lock_timeout for every step (upgrade.md rule M6), no statement or idle deadline.
//
// It validates fsys first (Validate), so a broken set never reaches the database. The error
// reports a nil db, an invalid set, or a goose setup error.
func NewMigrator(db *sql.DB, fsys fs.FS, opts MigratorOptions) (*Migrator, error) {
	if db == nil {
		return nil, errors.New("pg: NewMigrator needs a database handle")
	}
	if err := Validate(fsys); err != nil {
		return nil, err
	}
	files, err := listMigrations(fsys)
	if err != nil {
		return nil, err
	}
	// A locker per migrator: goose keeps the retry state of its polls in the locker.
	locker, err := lock.NewPostgresSessionLocker(lock.WithLockTimeout(lockPollSeconds, lockPollAttempts))
	if err != nil {
		return nil, fmt.Errorf("pg: migration lock: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys,
		goose.WithDisableGlobalRegistry(true), goose.WithSessionLocker(locker))
	if err != nil {
		return nil, fmt.Errorf("pg: migrations: %w", err)
	}
	return &Migrator{db: db, opts: opts, files: files, provider: provider}, nil
}

// Highest returns the highest embedded version, the version the binary requires.
func (m *Migrator) Highest() int64 { return m.files[len(m.files)-1].version }

// Up applies every pending migration. It first checks that the session user owns schema
// public (ErrNotSchemaOwner otherwise) and reads the database version: above the highest
// embedded version it returns Result.Newer without running anything. Otherwise it runs the
// before-steps, goose, and the after-steps.
//
// On a failed migration the error names it and wraps the database error, and Result.Applied
// lists the migrations of this run that were applied before it; a failed step is named as well.
func (m *Migrator) Up(ctx context.Context) (Result, error) {
	if err := m.checkOwner(ctx); err != nil {
		return Result{}, err
	}
	have, err := sqlSchemaVersion(ctx, m.db)
	if err != nil {
		return Result{}, fmt.Errorf("pg: read the schema version: %w", err)
	}
	if have > m.Highest() {
		return Result{Version: have, Newer: true}, nil
	}
	if err := runSteps(ctx, m.db, "before", m.opts.Before); err != nil {
		return Result{}, err
	}
	results, err := m.provider.Up(ctx)
	if err != nil {
		var partial *goose.PartialError
		if errors.As(err, &partial) {
			return Result{Applied: m.applied(partial.Applied)},
				fmt.Errorf("pg: migration %d failed: %w", partial.Failed.Source.Version, partial.Err)
		}
		return Result{}, fmt.Errorf("pg: migrations: %w", err)
	}
	applied := m.applied(results)
	if err := runSteps(ctx, m.db, "after", m.opts.After); err != nil {
		return Result{Applied: applied}, err
	}
	version, err := sqlSchemaVersion(ctx, m.db)
	if err != nil {
		return Result{Applied: applied}, fmt.Errorf("pg: read the schema version: %w", err)
	}
	return Result{Applied: applied, Version: version}, nil
}

// Status returns one entry per embedded migration in version order and the version of the
// database. It only reads: a database that was never migrated has no version table and
// version 0, and nothing is created or locked. The role needs SELECT on goose_db_version.
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, int64, error) {
	appliedAt, version, err := m.appliedVersions(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]MigrationStatus, 0, len(m.files))
	for _, f := range m.files {
		at, ok := appliedAt[f.version]
		out = append(out, MigrationStatus{Version: f.version, Name: f.name, Applied: ok, AppliedAt: at})
	}
	return out, version, nil
}

// appliedVersions reads the applied versions with their time and the database version.
func (m *Migrator) appliedVersions(ctx context.Context) (map[int64]time.Time, int64, error) {
	var exists bool
	if err := m.db.QueryRowContext(ctx, versionTableQuery).Scan(&exists); err != nil {
		return nil, 0, fmt.Errorf("pg: read the schema version: %w", err)
	}
	applied := map[int64]time.Time{}
	if !exists {
		return applied, 0, nil
	}
	rows, err := m.db.QueryContext(ctx,
		"SELECT version_id, max(tstamp) FROM goose_db_version WHERE is_applied AND version_id > 0 GROUP BY version_id")
	if err != nil {
		return nil, 0, fmt.Errorf("pg: read the applied migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var version int64
	for rows.Next() {
		var v int64
		var at time.Time
		if err := rows.Scan(&v, &at); err != nil {
			return nil, 0, fmt.Errorf("pg: read the applied migrations: %w", err)
		}
		// goose writes tstamp with now() in the session time zone, UTC for every pool of pg.
		applied[v] = time.Date(at.Year(), at.Month(), at.Day(), at.Hour(), at.Minute(), at.Second(), at.Nanosecond(), time.UTC)
		version = max(version, v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("pg: read the applied migrations: %w", err)
	}
	return applied, version, nil
}

// checkOwner returns ErrNotSchemaOwner unless the session user owns schema public.
func (m *Migrator) checkOwner(ctx context.Context) error {
	var owner bool
	err := m.db.QueryRowContext(ctx,
		"SELECT coalesce((SELECT pg_get_userbyid(nspowner) = current_user FROM pg_namespace WHERE nspname = 'public'), false)").
		Scan(&owner)
	if err != nil {
		return fmt.Errorf("pg: check the schema owner: %w", err)
	}
	if !owner {
		return ErrNotSchemaOwner
	}
	return nil
}

// applied converts goose results to AppliedMigration, naming each by its file.
func (m *Migrator) applied(results []*goose.MigrationResult) []AppliedMigration {
	names := make(map[int64]string, len(m.files))
	for _, f := range m.files {
		names[f.version] = f.name
	}
	out := make([]AppliedMigration, 0, len(results))
	for _, r := range results {
		if r == nil || r.Source == nil {
			continue
		}
		out = append(out, AppliedMigration{Version: r.Source.Version, Name: names[r.Source.Version], Duration: r.Duration})
	}
	return out
}

// runSteps runs steps in order and names the first that fails.
func runSteps(ctx context.Context, db *sql.DB, phase string, steps []Step) error {
	for _, s := range steps {
		if err := s.Run(ctx, db); err != nil {
			return fmt.Errorf("pg: %s-step %s: %w", phase, s.Name(), err)
		}
	}
	return nil
}
