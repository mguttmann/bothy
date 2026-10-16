// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// stateOf returns instance.State over a pool of role with its own sealed registry.
func stateOf(t *testing.T, db *pgtest.DB, role pg.Role) instance.State {
	t.Helper()
	pool := db.Pool(t, role)
	reg := txn.NewRegistry()
	reg.Seal()
	return app.NewState(postgres.NewRepository(pool), pg.NewUnitOfWork(pool, pg.UnitConfig{}, reg))
}

// isV7 reports a UUIDv7 with the RFC variant.
func isV7(id ids.ID) bool { return id[6]>>4 == 7 && id[8]>>6 == 2 }

func TestState_RecordMigrationCreatesAndKeepsIdentity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	owner := stateOf(t, db, pg.RoleOwner)

	first, err := owner.RecordMigration(ctx, 2, "0.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !isV7(first.InstanceID) || !isV7(first.SyncEpoch) || first.InstanceID == first.SyncEpoch {
		t.Fatalf("first record %+v: want two distinct UUIDv7", first)
	}
	if first.SchemaVersion != 2 || first.MinServerVersion != "0.0.0" || first.SetupCompleted() || first.CreatedAt.IsZero() {
		t.Fatalf("first record %+v", first)
	}
	second, err := owner.RecordMigration(ctx, 3, "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if second.InstanceID != first.InstanceID || second.SyncEpoch != first.SyncEpoch || !second.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("identity changed: %+v after %+v", second, first)
	}
	if second.SchemaVersion != 3 || second.MinServerVersion != "0.1.0" {
		t.Fatalf("versions not updated: %+v", second)
	}
	// An older binary racing a newer one never lowers what the newer one recorded.
	third, err := owner.RecordMigration(ctx, 2, "9.9.9")
	if err != nil {
		t.Fatal(err)
	}
	if third != second {
		t.Fatalf("a lower schema version changed the row: %+v after %+v", third, second)
	}
	// setup_completed_at is never touched by RecordMigration.
	super := db.SuperConn(t)
	if _, err := super.Exec(ctx, "UPDATE instance SET setup_completed_at = now()"); err != nil {
		t.Fatal(err)
	}
	fourth, err := owner.RecordMigration(ctx, 3, "0.1.0")
	if err != nil || !fourth.SetupCompleted() {
		t.Fatalf("after setup: %+v, %v", fourth, err)
	}
	// bothy_app reads the row at start.
	loaded, err := stateOf(t, db, pg.RoleApp).Load(ctx)
	if err != nil || loaded != fourth {
		t.Fatalf("Load as bothy_app = %+v, %v; want %+v", loaded, err, fourth)
	}
	// bothy_app cannot record a migration.
	if _, err := stateOf(t, db, pg.RoleApp).RecordMigration(ctx, 4, "0.1.0"); err == nil {
		t.Fatal("bothy_app recorded a migration")
	}
}

func TestState_ConcurrentRecordMigrationCreatesOneRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	owner := stateOf(t, db, pg.RoleOwner)
	const n = 6
	results := make(chan instance.Record, n)
	failures := make(chan error, n)
	for range n {
		go func() {
			r, err := owner.RecordMigration(ctx, 2, "0.0.0")
			if err != nil {
				failures <- err
				return
			}
			results <- r
		}()
	}
	var first instance.Record
	for i := range n {
		select {
		case err := <-failures:
			t.Fatal(err)
		case r := <-results:
			if i == 0 {
				first = r
			} else if r.InstanceID != first.InstanceID {
				t.Fatalf("two instance IDs: %s and %s", first.InstanceID, r.InstanceID)
			}
		}
	}
}

func TestState_LoadMissingRow(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	_, err := stateOf(t, db, pg.RoleApp).Load(context.Background())
	if errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("Load = %v, want kind not_found", err)
	}
}

// TestState_SetupAsApp runs the setup part of instance.State as bothy_app, the role of bothy setup
// (16 section 7.1): the row is locked, setup_completed_at is set once and a second completion
// fails with conflict; Load sees the value afterwards (M1-37).
func TestState_SetupAsApp(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	if _, err := stateOf(t, db, pg.RoleOwner).RecordMigration(ctx, 2, "0.0.0"); err != nil {
		t.Fatal(err)
	}
	pool := db.Pool(t, pg.RoleApp)
	reg := txn.NewRegistry()
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{}, reg)
	s := app.NewState(postgres.NewRepository(pool), uow)
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	err := uow.Do(ctx, func(ctx context.Context) error {
		rec, err := s.LockForSetup(ctx)
		if err != nil {
			return err
		}
		if rec.SetupCompleted() {
			t.Fatalf("fresh row %+v reports setup complete", rec)
		}
		return s.CompleteSetup(ctx, at)
	})
	if err != nil {
		t.Fatal(err)
	}
	err = uow.Do(ctx, func(ctx context.Context) error { return s.CompleteSetup(ctx, at.Add(time.Hour)) })
	if errs.KindOf(err) != errs.KindConflict {
		t.Fatalf("second CompleteSetup = %v, want kind conflict", err)
	}
	rec, err := s.Load(ctx)
	if err != nil || !rec.SetupCompletedAt.Equal(at) {
		t.Fatalf("Load = %+v, %v; want setup_completed_at %v", rec, err, at)
	}
}
