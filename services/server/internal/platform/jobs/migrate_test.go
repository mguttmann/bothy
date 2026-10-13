// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestMigrationFunctionsRefuseAnInvalidSchema: every entry point checks the schema name before
// it touches the pool.
func TestMigrationFunctionsRefuseAnInvalidSchema(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	if _, err := jobs.MigrationStatus(ctx, nil, "Not-Valid"); err == nil {
		t.Error("MigrationStatus accepted an invalid schema name")
	}
	if err := jobs.ValidateMigrations(ctx, nil, "public"); err == nil {
		t.Error("ValidateMigrations accepted the schema public")
	}
	if _, err := jobs.Migrate(ctx, nil, "pg_jobs"); err == nil {
		t.Error("Migrate accepted a schema starting with pg_")
	}
}

// TestMigrationStatusBeforeAndAfterMigrate: a schema without River's tables has every migration
// pending and fails ValidateMigrations with a pointer to bothy migrate up; after Migrate nothing
// is pending.
func TestMigrationStatusBeforeAndAfterMigrate(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	pool := db.OpenPool(t, db.Config(pg.RoleOwner).ForMigrate(time.Second))
	ctx, cancel := context.WithTimeout(t.Context(), waitLong)
	defer cancel()
	const schema = "jobs_fresh"
	st, err := jobs.MigrationStatus(ctx, pool, schema)
	if err != nil {
		t.Fatalf("MigrationStatus of a fresh schema: %v", err)
	}
	if st.Version != 0 || st.Latest == 0 || st.Pending != st.Latest {
		t.Fatalf("status of a fresh schema = %+v, want version 0 and every migration pending", st)
	}
	if err := jobs.ValidateMigrations(ctx, pool, schema); err == nil || !strings.Contains(err.Error(), "bothy migrate up") {
		t.Fatalf("ValidateMigrations of a fresh schema = %v, want an error naming bothy migrate up", err)
	}
	if _, err := jobs.Migrate(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	st, err = jobs.MigrationStatus(ctx, pool, schema)
	if err != nil || st.Pending != 0 || st.Version != st.Latest {
		t.Fatalf("status after Migrate = %+v, %v; want nothing pending", st, err)
	}
	if err := jobs.ValidateMigrations(ctx, pool, schema); err != nil {
		t.Fatalf("ValidateMigrations after Migrate: %v", err)
	}
}

// TestMigrationStatusFailsOnAnEndedContext: a failed read is an error, not a status.
func TestMigrationStatusFailsOnAnEndedContext(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	pool := db.OpenPool(t, db.Config(pg.RoleOwner).ForMigrate(time.Second))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := jobs.MigrationStatus(ctx, pool, "jobs_ended"); err == nil {
		t.Fatal("MigrationStatus on an ended context returned a status")
	}
	if err := jobs.ValidateMigrations(ctx, pool, "jobs_ended"); err == nil {
		t.Fatal("ValidateMigrations on an ended context passed")
	}
}

// TestMigrateStopsWaitingForTheLockWhenTheContextEnds: a run that waits for another run's lock
// gives up with the context and says so.
func TestMigrateStopsWaitingForTheLockWhenTheContextEnds(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	pool := db.OpenPool(t, db.Config(pg.RoleOwner).ForMigrate(time.Second))
	hctx, hcancel := context.WithTimeout(t.Context(), waitLong)
	defer hcancel()
	holder, err := pool.PGX().Acquire(hctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Release()
	if _, err := holder.Exec(hctx, "SELECT pg_advisory_lock($1)", jobs.MigrateLockIDForTest); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = holder.Exec(context.WithoutCancel(hctx), "SELECT pg_advisory_unlock($1)", jobs.MigrateLockIDForTest)
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	_, err = jobs.Migrate(ctx, pool, testSchema)
	if err == nil || !strings.Contains(err.Error(), "lock") || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Migrate while another run holds the lock = %v, want a lock error with the deadline", err)
	}
}
