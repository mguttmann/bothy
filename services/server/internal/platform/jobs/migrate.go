// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// migrateLockID serialises concurrent runs of Migrate on one database with a session advisory
// lock; its value is arbitrary but fixed ("bothyjob" as eight ASCII bytes).
const migrateLockID int64 = 0x626f7468796a6f62

// setSchemaSQL passes the schema name to the DO block as a setting of the session, so the name
// never becomes part of a statement text (Semgrep rule bothy-go-sql-string-building); the
// connection is held for the advisory lock anyway, and clearSchemaSQL empties the setting before
// the connection goes back to the pool. The DO block is one statement and so runs atomically
// without an explicit transaction, which only platform/pg opens
// (bothy-go-tx-only-in-platform-pg).
const (
	setSchemaSQL   = "SELECT set_config('bothy.jobs_schema', $1, false)"
	clearSchemaSQL = "SELECT set_config('bothy.jobs_schema', '', false)"
)

// schemaSQL creates the schema of River's tables and the rights bothy_app needs on what
// bothy_owner creates there: DML on tables, USAGE and SELECT on sequences, EXECUTE on functions
// (docs/spec/16-operations.md "Roles": bothy_owner migrates, bothy_app runs). The default
// privileges cover the tables River's migrations create next; the grants on ALL cover tables
// an earlier run created before. format with %I quotes the name.
const schemaSQL = `DO $schema$
DECLARE
	s text := current_setting('bothy.jobs_schema');
BEGIN
	EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', s);
	EXECUTE format('REVOKE ALL ON SCHEMA %I FROM PUBLIC', s);
	EXECUTE format('GRANT USAGE ON SCHEMA %I TO bothy_app', s);
	EXECUTE format('ALTER DEFAULT PRIVILEGES FOR ROLE bothy_owner IN SCHEMA %I GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO bothy_app', s);
	EXECUTE format('ALTER DEFAULT PRIVILEGES FOR ROLE bothy_owner IN SCHEMA %I GRANT USAGE, SELECT ON SEQUENCES TO bothy_app', s);
	EXECUTE format('ALTER DEFAULT PRIVILEGES FOR ROLE bothy_owner IN SCHEMA %I GRANT EXECUTE ON FUNCTIONS TO bothy_app', s);
	EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA %I TO bothy_app', s);
	EXECUTE format('GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA %I TO bothy_app', s);
	EXECUTE format('GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA %I TO bothy_app', s);
END
$schema$`

// MigrateResult is what Migrate did.
type MigrateResult struct {
	// Applied lists the River migration versions this run applied, in order.
	Applied []int
	// Version is the highest River migration version in the schema after the run.
	Version int
}

// Migrate prepares schema for River and applies River's pending migrations to it (bothy migrate
// up, before goose, docs/operations/upgrade.md rule M11). pool connects as bothy_owner with the
// session settings of migrate. In one transaction it creates the schema if needed, takes the
// schema away from PUBLIC and grants bothy_app USAGE on it and DML, sequence and function rights
// on everything bothy_owner creates there (one DO block, so all or nothing); then rivermigrate
// migrates up with Config.Schema = schema. A session advisory lock serialises concurrent runs; a second run changes nothing.
//
// The error reports an invalid schema name (ValidateSchema), a failed schema statement or a
// failed River migration.
func Migrate(ctx context.Context, pool *pg.Pool, schema string) (MigrateResult, error) {
	if err := ValidateSchema(schema); err != nil {
		return MigrateResult{}, fmt.Errorf("jobs: %w", err)
	}
	if err := needConns(pool); err != nil {
		return MigrateResult{}, err
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return MigrateResult{}, fmt.Errorf("jobs: migrate: %w", err)
	}
	defer conn.Release()
	if err := lockForMigrate(ctx, conn); err != nil {
		return MigrateResult{}, err
	}
	defer func() {
		// Cleaning up on a fresh context: the lock and the setting must go also when ctx ended;
		// a failed statement ends with the connection, which pgx closes after an error.
		cctx := context.WithoutCancel(ctx)
		_, _ = conn.Exec(cctx, clearSchemaSQL)
		_, _ = conn.Exec(cctx, "SELECT pg_advisory_unlock($1)", migrateLockID)
	}()
	if _, err = conn.Exec(ctx, setSchemaSQL, schema); err == nil {
		_, err = conn.Exec(ctx, schemaSQL)
	}
	if err != nil {
		return MigrateResult{}, fmt.Errorf("jobs: migrate: create schema %s: %w", schema, err)
	}
	m, err := migrator(pool, schema)
	if err != nil {
		return MigrateResult{}, err
	}
	res, err := m.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return MigrateResult{}, fmt.Errorf("jobs: migrate River in schema %s: %w", schema, err)
	}
	out := MigrateResult{}
	for _, v := range res.Versions {
		out.Applied = append(out.Applied, v.Version)
	}
	st, err := migrationStatus(ctx, m)
	if err != nil {
		return out, err
	}
	out.Version = st.Version
	return out, nil
}

// lockForMigrate takes the advisory lock of Migrate on conn. It polls pg_try_advisory_lock with
// the cadence of goose's lock (pg.MigrateLockPoll, pg.MigrateLockAttempts): pg_advisory_lock
// would end after migrate.lock_timeout, the session setting of every migrate connection, while
// a concurrent bothy migrate up is still running. The error reports a failed statement, an
// ended ctx, or a lock still held after the last attempt.
func lockForMigrate(ctx context.Context, conn *pgxpool.Conn) error {
	for range pg.MigrateLockAttempts {
		var ok bool
		if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", migrateLockID).Scan(&ok); err != nil {
			return fmt.Errorf("jobs: migrate: lock: %w", err)
		}
		if ok {
			return nil
		}
		t := time.NewTimer(pg.MigrateLockPoll)
		select {
		case <-ctx.Done():
			t.Stop()
			return fmt.Errorf("jobs: migrate: lock: %w", ctx.Err())
		case <-t.C:
		}
	}
	return errors.New("jobs: migrate: another bothy migrate up still holds the lock of River's migrations")
}

// minConns is the smallest pool River can work with: Migrate holds one connection for its lock
// while River migrates on another, and a working River client holds one for LISTEN while its
// producers fetch on others. River waits for a connection without a deadline, so a smaller pool
// would hang instead of failing.
const minConns = 2

// needConns refuses a pool smaller than minConns, naming the key to raise.
func needConns(pool *pg.Pool) error {
	if pool.Stat().MaxConns() < minConns {
		return fmt.Errorf("jobs: River needs database.max_conns of at least %d", minConns)
	}
	return nil
}

// Status is the state of River's migrations in one schema.
type Status struct {
	// Version is the highest applied River migration version; 0 when none is applied.
	Version int
	// Latest is the highest River migration version this binary carries.
	Latest int
	// Pending is the number of River migrations this binary carries that are not applied.
	Pending int
}

// MigrationStatus reads the River migration state of schema (bothy migrate status). It only
// reads: a schema without River's tables has Version 0 and every migration pending. The error
// reports an invalid schema name or a failed query.
func MigrationStatus(ctx context.Context, pool *pg.Pool, schema string) (Status, error) {
	if err := ValidateSchema(schema); err != nil {
		return Status{}, fmt.Errorf("jobs: %w", err)
	}
	m, err := migrator(pool, schema)
	if err != nil {
		return Status{}, err
	}
	return migrationStatus(ctx, m)
}

// migrationStatus reads the applied versions through m.
func migrationStatus(ctx context.Context, m *rivermigrate.Migrator[pgx.Tx]) (Status, error) {
	existing, err := m.ExistingVersions(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("jobs: read the River migrations: %w", err)
	}
	applied := map[int]bool{}
	st := Status{}
	for _, v := range existing {
		applied[v.Version] = true
		st.Version = max(st.Version, v.Version)
	}
	for _, v := range m.AllVersions() {
		st.Latest = max(st.Latest, v.Version)
		if !applied[v.Version] {
			st.Pending++
		}
	}
	return st, nil
}

// ValidateMigrations checks that every River migration this binary carries is applied in schema;
// the start of bothy serve calls it for every role, as it checks the goose version, so a process
// never runs against missing River tables (R-AR-16: serve never migrates). The error names the
// schema and the missing versions and points to bothy migrate up.
func ValidateMigrations(ctx context.Context, pool *pg.Pool, schema string) error {
	if err := ValidateSchema(schema); err != nil {
		return fmt.Errorf("jobs: %w", err)
	}
	m, err := migrator(pool, schema)
	if err != nil {
		return err
	}
	res, err := m.Validate(ctx, nil)
	if err != nil {
		return fmt.Errorf("jobs: validate the River migrations in schema %s: %w", schema, err)
	}
	if !res.OK {
		return fmt.Errorf("jobs: the River migrations in schema %s are not complete (%s); run \"bothy migrate up\"", schema, strings.Join(res.Messages, "; "))
	}
	return nil
}

// CheckEmbeddedMigrations checks River's migrations embedded in this binary without a database
// (bothy migrate validate): they load and are numbered 1 to n without gaps. It returns n.
func CheckEmbeddedMigrations() (int, error) {
	m, err := rivermigrate.New(riverpgxv5.New(nil), &rivermigrate.Config{Logger: quiet})
	if err != nil {
		return 0, fmt.Errorf("jobs: River migrations: %w", err)
	}
	var versions []int
	for _, v := range m.AllVersions() {
		if strings.TrimSpace(v.SQLUp) == "" {
			return 0, fmt.Errorf("jobs: River migration %d has no up step", v.Version)
		}
		versions = append(versions, v.Version)
	}
	slices.Sort(versions)
	if len(versions) == 0 {
		return 0, errors.New("jobs: the binary carries no River migration")
	}
	for i, v := range versions {
		if v != i+1 {
			return 0, fmt.Errorf("jobs: River migrations are not numbered 1 to %d without gaps (found %d at position %d)", len(versions), v, i+1)
		}
	}
	return len(versions), nil
}

// quiet discards the log of River's migrator: its default writes warnings to stdout, which `bothy
// migrate` keeps for the lines scripts read; the results come back as values and errors.
var quiet = slog.New(slog.DiscardHandler)

// migrator returns River's migrator for schema over pool.
func migrator(pool *pg.Pool, schema string) (*rivermigrate.Migrator[pgx.Tx], error) {
	m, err := rivermigrate.New(riverpgxv5.New(pool.PGX()), &rivermigrate.Config{Schema: schema, Logger: quiet})
	if err != nil {
		return nil, fmt.Errorf("jobs: River migrator: %w", err)
	}
	return m, nil
}
