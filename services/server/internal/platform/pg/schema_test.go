// SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"testing"
	"testing/fstest"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// withMigrations returns the server's migrations plus extra files (name to content).
func withMigrations(t *testing.T, extra map[string]string) fstest.MapFS {
	t.Helper()
	out := fstest.MapFS{}
	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		b, err := fs.ReadFile(migrations.FS, n)
		if err != nil {
			t.Fatal(err)
		}
		out[n] = &fstest.MapFile{Data: b}
	}
	for n, content := range extra {
		out[n] = &fstest.MapFile{Data: []byte(content)}
	}
	return out
}

// highest is the version the server's migrations require.
func highest(t *testing.T) int64 {
	t.Helper()
	v, err := pg.RequiredSchemaVersion(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// nextMigration returns a migration file named for version n that creates a marker table.
func nextMigration(n int64, table string) (string, string) {
	name := fmt.Sprintf("%05d_%s.sql", n, table)
	return name, "-- +goose Up\nCREATE TABLE " + table + " (id int);\n\n-- +goose Down\nDROP TABLE " + table + ";\n"
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestCheckSchemaTooOld(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	have := highest(t)
	name, content := nextMigration(have+1, "newer_marker")
	newer := withMigrations(t, map[string]string{name: content})

	pool := pgtest.New(t).Pool(t, pg.RoleApp)
	err := pg.CheckSchema(ctx, pool, have+1)
	var tooOld *pg.SchemaTooOldError
	if !errors.As(err, &tooOld) || tooOld.Have != have || tooOld.Need != have+1 {
		t.Fatalf("CheckSchema(required %d) = %v, want *SchemaTooOldError{%d, %d}", have+1, err, have, have+1)
	}
	want := "database schema version " + itoa(have) + " is older than required " + itoa(have+1) + `; run "bothy migrate up"`
	if err.Error() != want {
		t.Errorf("message %q, want %q", err, want)
	}
	if err := pg.CheckSchema(ctx, pool, have); err != nil {
		t.Errorf("equal version: %v", err)
	}
	// A newer schema (a newer binary migrated it) passes.
	if err := pg.CheckSchema(ctx, pgtest.New(t, pgtest.WithMigrations(newer)).Pool(t, pg.RoleApp), have); err != nil {
		t.Errorf("newer schema: %v", err)
	}
	// A database that was never migrated has version 0.
	err = pg.CheckSchema(ctx, pgtest.New(t, pgtest.WithoutMigrations()).Pool(t, pg.RoleApp), have)
	if !errors.As(err, &tooOld) || tooOld.Have != 0 {
		t.Errorf("unmigrated database: %v, want *SchemaTooOldError with Have 0", err)
	}
}
