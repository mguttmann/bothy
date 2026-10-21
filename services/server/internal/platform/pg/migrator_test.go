// SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/migrations"
)

// migrateDB opens a database/sql handle of role on db with the session settings of bothy
// migrate and the given lock timeout.
func migrateDB(t *testing.T, db *pgtest.DB, role pg.Role, lockTimeout time.Duration) *sql.DB {
	t.Helper()
	pool := db.OpenPool(t, db.Config(role).ForMigrate(lockTimeout))
	sqlDB := pool.OpenSQL()
	t.Cleanup(func() { _ = sqlDB.Close() })
	return sqlDB
}

func newMigrator(t *testing.T, sqlDB *sql.DB, fsys fs.FS, opts pg.MigratorOptions) *pg.Migrator {
	t.Helper()
	m, err := pg.NewMigrator(sqlDB, fsys, opts)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// versionRows counts the rows per version in goose_db_version.
func versionRows(t *testing.T, sqlDB *sql.DB) map[int64]int {
	t.Helper()
	rows, err := sqlDB.QueryContext(context.Background(), "SELECT version_id, count(*) FROM goose_db_version GROUP BY version_id")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	out := map[int64]int{}
	for rows.Next() {
		var v int64
		var n int
		if err := rows.Scan(&v, &n); err != nil {
			t.Fatal(err)
		}
		out[v] = n
	}
	return out
}

func TestMigrateEmptyDatabase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t, pgtest.WithoutMigrations())
	m := newMigrator(t, migrateDB(t, db, pg.RoleOwner, testkit.Patience), migrations.FS, pg.MigratorOptions{})
	want := highest(t)

	res, err := m.Up(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(res.Applied)) != want || res.Version != want || res.Newer {
		t.Fatalf("first run: %+v, want %d applied and version %d", res, want, want)
	}
	for i, a := range res.Applied {
		if a.Version != int64(i+1) || a.Name == "" {
			t.Errorf("applied[%d] = %+v", i, a)
		}
	}
	if res.Applied[0].Name != "base_privileges" {
		t.Errorf("first migration named %q, want base_privileges", res.Applied[0].Name)
	}
	res, err = m.Up(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Applied) != 0 || res.Version != want {
		t.Errorf("second run: %+v, want nothing applied and version %d", res, want)
	}
}

func TestMigrateConcurrentRunsApplyOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	base := highest(t)
	n2, c2 := nextMigration(base+1, "concurrent_a")
	n3, c3 := nextMigration(base+2, "concurrent_b")
	fsys := withMigrations(t, map[string]string{n2: c2, n3: c3})
	db := pgtest.New(t, pgtest.WithoutMigrations())

	var wg sync.WaitGroup
	results := make([]pg.Result, 2)
	errs := make([]error, 2)
	for i := range 2 {
		m := newMigrator(t, migrateDB(t, db, pg.RoleOwner, testkit.Patience), fsys, pg.MigratorOptions{})
		wg.Go(func() { results[i], errs[i] = m.Up(ctx) })
	}
	wg.Wait()
	applied := map[int64]int{}
	for i := range 2 {
		if errs[i] != nil {
			t.Fatalf("run %d: %v", i, errs[i])
		}
		for _, a := range results[i].Applied {
			applied[a.Version]++
		}
	}
	for v := int64(1); v <= base+2; v++ {
		if applied[v] != 1 {
			t.Errorf("version %d applied %d times, want once", v, applied[v])
		}
	}
	for v, n := range versionRows(t, migrateDB(t, db, pg.RoleOwner, testkit.Patience)) {
		if n != 1 {
			t.Errorf("goose_db_version has %d rows for version %d", n, v)
		}
	}
}

func TestMigrateNewerSchemaIsNoop(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	have := highest(t)
	name, content := nextMigration(have+1, "from_a_newer_binary")
	db := pgtest.New(t, pgtest.WithMigrations(withMigrations(t, map[string]string{name: content})))
	sqlDB := migrateDB(t, db, pg.RoleOwner, testkit.Patience)
	before := versionRows(t, sqlDB)

	ran := false
	step := stepFunc{name: "marker", run: func(context.Context, *sql.DB) error { ran = true; return nil }}
	res, err := newMigrator(t, sqlDB, migrations.FS, pg.MigratorOptions{Before: []pg.Step{step}, After: []pg.Step{step}}).Up(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Newer || res.Version != have+1 || len(res.Applied) != 0 {
		t.Errorf("result %+v, want Newer with version %d and nothing applied", res, have+1)
	}
	if ran {
		t.Error("a step ran against a newer schema")
	}
	after := versionRows(t, sqlDB)
	if len(after) != len(before) {
		t.Errorf("goose_db_version changed: %v -> %v", before, after)
	}
}

// stepFunc is a Step from a function.
type stepFunc struct {
	name string
	run  func(context.Context, *sql.DB) error
}

func (s stepFunc) Name() string                              { return s.name }
func (s stepFunc) Run(ctx context.Context, db *sql.DB) error { return s.run(ctx, db) }

func TestMigrateStepsOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t, pgtest.WithoutMigrations())
	sqlDB := migrateDB(t, db, pg.RoleOwner, testkit.Patience)
	var order []string
	versionAt := func(label string) pg.Step {
		return stepFunc{name: label, run: func(ctx context.Context, db *sql.DB) error {
			var exists bool
			if err := db.QueryRowContext(ctx, "SELECT to_regclass('goose_db_version') IS NOT NULL").Scan(&exists); err != nil {
				return err
			}
			order = append(order, label+map[bool]string{true: ":migrated", false: ":empty"}[exists])
			return nil
		}}
	}
	_, err := newMigrator(t, sqlDB, migrations.FS, pg.MigratorOptions{
		Before: []pg.Step{versionAt("before1"), versionAt("before2")},
		After:  []pg.Step{versionAt("after")},
	}).Up(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(order, ","); got != "before1:empty,before2:empty,after:migrated" {
		t.Errorf("steps ran as %s", got)
	}

	// A failing before-step stops the run before goose and is named in the error.
	fresh := pgtest.New(t, pgtest.WithoutMigrations())
	freshDB := migrateDB(t, fresh, pg.RoleOwner, testkit.Patience)
	boom := stepFunc{name: "river", run: func(context.Context, *sql.DB) error { return errors.New("boom") }}
	_, err = newMigrator(t, freshDB, migrations.FS, pg.MigratorOptions{Before: []pg.Step{boom}}).Up(ctx)
	if err == nil || !strings.Contains(err.Error(), "river") {
		t.Fatalf("failing step: %v, want an error naming the step", err)
	}
	var exists bool
	if err := freshDB.QueryRowContext(ctx, "SELECT to_regclass('goose_db_version') IS NOT NULL").Scan(&exists); err != nil || exists {
		t.Errorf("goose ran after a failing before-step (%v)", err)
	}
}

func TestMigrateRequiresSchemaOwner(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t, pgtest.WithoutMigrations())
	for _, role := range []pg.Role{pg.RoleApp, pg.RoleMaint} {
		_, err := newMigrator(t, migrateDB(t, db, role, testkit.Patience), migrations.FS, pg.MigratorOptions{}).Up(context.Background())
		if !errors.Is(err, pg.ErrNotSchemaOwner) {
			t.Errorf("migrate as %s: %v, want ErrNotSchemaOwner", role, err)
		}
	}
	if pg.ErrNotSchemaOwner.Error() != "migrate must run as the schema owner; set database.user" {
		t.Errorf("message %q", pg.ErrNotSchemaOwner)
	}
}

func TestMigrationStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t, pgtest.WithoutMigrations())
	sqlDB := migrateDB(t, db, pg.RoleOwner, testkit.Patience)
	m := newMigrator(t, sqlDB, migrations.FS, pg.MigratorOptions{})

	st, version, err := m.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version != 0 || int64(len(st)) != highest(t) {
		t.Fatalf("before: version %d, %d entries", version, len(st))
	}
	for _, s := range st {
		if s.Applied || !s.AppliedAt.IsZero() {
			t.Errorf("before: %+v, want pending", s)
		}
	}
	var exists bool
	if err := sqlDB.QueryRowContext(ctx, "SELECT to_regclass('goose_db_version') IS NOT NULL").Scan(&exists); err != nil || exists {
		t.Errorf("Status created the version table (%v)", err)
	}

	start := time.Now().UTC().Add(-time.Minute)
	if _, err := m.Up(ctx); err != nil {
		t.Fatal(err)
	}
	st, version, err = m.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version != highest(t) {
		t.Errorf("after: version %d", version)
	}
	for _, s := range st {
		if !s.Applied || s.AppliedAt.Before(start) || s.AppliedAt.Location() != time.UTC {
			t.Errorf("after: %+v, want applied just now in UTC", s)
		}
	}
	// bothy_app may read the status (it reads the version at start).
	if _, _, err := newMigrator(t, migrateDB(t, db, pg.RoleApp, testkit.Patience), migrations.FS, pg.MigratorOptions{}).Status(ctx); err != nil {
		t.Errorf("status as bothy_app: %v", err)
	}
}

func TestMigrateFailedMigration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	base := highest(t)
	name := nextName(base+1, "broken")
	fsys := withMigrations(t, map[string]string{name: "-- +goose Up\nCREATE TABLE half_done (id int);\nSELECT no_such_function();\n"})
	db := pgtest.New(t, pgtest.WithoutMigrations())
	sqlDB := migrateDB(t, db, pg.RoleOwner, testkit.Patience)
	res, err := newMigrator(t, sqlDB, fsys, pg.MigratorOptions{}).Up(ctx)
	if err == nil || !strings.Contains(err.Error(), "migration "+itoa(base+1)) {
		t.Fatalf("Up: %v, want an error naming migration %d", err, base+1)
	}
	if int64(len(res.Applied)) != base {
		t.Errorf("applied before the failure: %+v, want %d", res.Applied, base)
	}
	// The failed migration ran in a transaction: nothing of it is left.
	var exists bool
	if err := sqlDB.QueryRowContext(ctx, "SELECT to_regclass('half_done') IS NOT NULL").Scan(&exists); err != nil || exists {
		t.Errorf("the failed migration left its table (%v)", err)
	}
}

func nextName(n int64, slug string) string {
	name, _ := nextMigration(n, slug)
	return name
}

// TestMigrateStepHonoursLockTimeout shows rule M6 of upgrade.md: a migration that waits for a
// lock held by the application fails after migrate.lock_timeout instead of blocking it.
func TestMigrateStepHonoursLockTimeout(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	base := highest(t)
	n1, c1 := nextMigration(base+1, "locked_table")
	db := pgtest.New(t, pgtest.WithMigrations(withMigrations(t, map[string]string{n1: c1})))
	n2 := nextName(base+2, "alter_locked")
	fsys := withMigrations(t, map[string]string{n1: c1, n2: "-- +goose Up\nALTER TABLE locked_table ADD COLUMN extra int;\n"})

	holder := db.SuperConn(t)
	tx, err := holder.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "LOCK TABLE locked_table IN ACCESS SHARE MODE"); err != nil {
		t.Fatal(err)
	}
	// The pool and the migrator are built before the clock starts: under a loaded runner the
	// connection alone can take seconds. 55P03 is the proof that lock_timeout ended the wait; the
	// bound only catches a migration that waits for the holder instead.
	m := newMigrator(t, migrateDB(t, db, pg.RoleOwner, 300*time.Millisecond), fsys, pg.MigratorOptions{})
	start := time.Now()
	_, err = m.Up(ctx)
	assertSQLState(t, err, "55P03")
	if elapsed := time.Since(start); elapsed > testkit.Patience {
		t.Errorf("the migration waited %s for the lock", elapsed)
	}
}
