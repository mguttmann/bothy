// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/migrations"
)

// MigrationSteps returns the steps bothy migrate up runs around the goose migrations of fsys on
// the database behind pool (a pool of the schema owner with the session settings of migrate):
// after them the instance step (M1-15). MigrateUpSteps puts River's step (M1-14) before them for
// cmd/bothy; pgtest.WithSteps calls this function directly.
//
// The instance step runs in one unit of work on pool: it reads the goose version and, when it
// equals the newest migration of fsys, records it and migrations.MinServerVersion through
// instance.State.RecordMigration, creating the row with new instance_id and sync_epoch if it is
// missing. A database newer than fsys keeps its row unchanged (docs/operations/upgrade.md
// "Rückweg" item 1); Migrator.Up does not even run the steps then, and the step checks again
// because a newer binary may migrate between goose and the step.
//
// The error reports a set fsys that is not valid or a registry that cannot be built.
func MigrationSteps(pool *pg.Pool, fsys fs.FS) (pg.MigratorOptions, error) {
	highest, err := pg.RequiredSchemaVersion(fsys)
	if err != nil {
		return pg.MigratorOptions{}, err
	}
	reg, err := NewTxnRegistry(NewModules(pool, nil))
	if err != nil {
		return pg.MigratorOptions{}, err
	}
	// Zero deadlines keep the session values of migrate (lock_timeout = migrate.lock_timeout,
	// upgrade.md rule M6); no retry, a failed migrate is simply run again.
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{}, reg)
	step := &instanceStep{
		pool: pool, uow: uow, highest: highest,
		state: app.NewState(postgres.NewRepository(pool), uow),
	}
	return pg.MigratorOptions{After: []pg.Step{step}}, nil
}

// instanceStep records the instance row after the migrations.
type instanceStep struct {
	pool    *pg.Pool
	uow     txn.UnitOfWork
	state   instance.State
	highest int64
}

// Name names the step in the output of bothy migrate.
func (s *instanceStep) Name() string { return "instance" }

// Run records the row in a unit of work on the pool of the step; the database/sql handle of
// the migrator is not used, because the unit of work runs on the pgx pool behind it.
func (s *instanceStep) Run(ctx context.Context, _ *sql.DB) error {
	return s.uow.Do(ctx, func(ctx context.Context) error {
		have, err := pg.SchemaVersion(ctx, pg.Querier(ctx, s.pool))
		if err != nil {
			return fmt.Errorf("read the schema version: %w", err)
		}
		if have != s.highest {
			return nil
		}
		_, err = s.state.RecordMigration(ctx, have, migrations.MinServerVersion)
		return err
	})
}

// InstanceCounters returns the sequence counters over pool (instance.Counters). Only the
// constructors of the phase hooks of authz, sync and audit receive it (M1-29, M1-16, M1-24);
// TestCountersOnlyUsedByPhaseHooks fails when any other package names instance.Counters
// (docs/spec/02-architektur.md "Sperrreihenfolge").
func InstanceCounters(pool *pg.Pool) instance.Counters {
	return app.NewCounters(postgres.NewRepository(pool))
}

// CheckSchemaRequiresNewerServer is the readiness check that fails for good when the own version
// is below instance.min_server_version (02 "Health", 16 "Zwei Listener").
const CheckSchemaRequiresNewerServer = "schema_requires_newer_server"

// Log messages of the instance step of the start.
const (
	msgInstanceSetupPending   = "instance loaded, setup pending"
	msgInstanceSetupCompleted = "instance loaded, setup completed"
	msgServerTooOld           = "server version below instance.min_server_version"
)

// loadInstance is start step 5 of 02 "Start": it loads the instance row through instance.State
// as the role of the process (missing row: kind not_found with the hint to run
// "bothy migrate up", which ends the start with exit code 1), keeps the record for later
// tickets (setup state, instance_id, sync_epoch) and compares the own version with
// instance.min_server_version once. Below it the process keeps running but registers the
// failing readiness check schema_requires_newer_server, so it never becomes ready; the log line
// names both versions, /readyz only the check. This applies to every role.
func (s *server) loadInstance(ctx context.Context) error {
	s.mu.Lock()
	pool := s.pool
	s.mu.Unlock()
	reg, err := NewTxnRegistry(NewModules(pool, s.logger()))
	if err != nil {
		return err
	}
	unitCfg, err := UnitConfig(s.values)
	if err != nil {
		return err
	}
	st := app.NewState(postgres.NewRepository(pool), pg.NewUnitOfWork(pool, unitCfg, reg))
	rec, err := st.Load(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.instance = rec
	s.mu.Unlock()
	if rec.SetupCompleted() {
		s.logger().InfoContext(ctx, msgInstanceSetupCompleted)
	} else {
		s.logger().InfoContext(ctx, msgInstanceSetupPending)
	}
	if verr := domain.CheckServerVersion(s.deps.Version, rec.MinServerVersion); verr != nil {
		s.logger().ErrorContext(ctx, msgServerTooOld,
			slog.String(attr.VersionOwn, s.deps.Version), slog.String(attr.VersionRequired, rec.MinServerVersion))
		s.reg.AddReadiness(CheckSchemaRequiresNewerServer, func(context.Context) error { return verr })
		s.mu.Lock()
		s.blocked = true
		s.mu.Unlock()
	}
	return nil
}
