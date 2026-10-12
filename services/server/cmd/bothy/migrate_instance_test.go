// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// instanceState is the instance row as the superuser reads it.
type instanceState struct {
	instanceID, syncEpoch, minServer string
	schemaVersion                    int64
}

func readInstance(t *testing.T, db *pgtest.DB) instanceState {
	t.Helper()
	var s instanceState
	err := db.SuperConn(t).QueryRow(context.Background(), `SELECT instance_id::text, sync_epoch::text,
		coalesce(min_server_version, ''), schema_version FROM instance`).Scan(&s.instanceID, &s.syncEpoch, &s.minServer, &s.schemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestMigrateUp_RecordsSchemaAndMinServerVersion: bothy migrate up creates the instance row with
// the goose version and migrations.MinServerVersion and keeps its identity on the next run;
// against a database newer than the binary it changes nothing and exits 0.
func TestMigrateUp_RecordsSchemaAndMinServerVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	need, err := pg.RequiredSchemaVersion(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	db := pgtest.New(t, pgtest.WithoutMigrations())
	env := migrateEnvFor(db, pg.RoleOwner)
	if code, _, stderr := runBothy(t, env, "migrate", "up"); code != 0 {
		t.Fatalf("first up: exit %d, stderr %q", code, stderr)
	}
	first := readInstance(t, db)
	if first.schemaVersion != need || first.minServer != migrations.MinServerVersion ||
		first.instanceID[14] != '7' || first.syncEpoch[14] != '7' || first.instanceID == first.syncEpoch {
		t.Fatalf("row after the first run: %+v (want schema %d, min %s, two UUIDv7)", first, need, migrations.MinServerVersion)
	}
	if code, _, stderr := runBothy(t, env, "migrate", "up"); code != 0 {
		t.Fatalf("second up: exit %d, stderr %q", code, stderr)
	}
	if again := readInstance(t, db); again != first {
		t.Fatalf("second run changed the row: %+v, was %+v", again, first)
	}

	// A newer binary applies one more migration and records its own values.
	newer := fstest.MapFS{}
	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		b, err := fs.ReadFile(migrations.FS, n)
		if err != nil {
			t.Fatal(err)
		}
		newer[n] = &fstest.MapFile{Data: b}
	}
	newer[fmt.Sprintf("%05d_from_a_newer_binary.sql", need+1)] = &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE newer_marker (id int);\n")}
	owner := db.Pool(t, pg.RoleOwner)
	sqlDB := owner.OpenSQL()
	defer func() { _ = sqlDB.Close() }()
	m, err := pg.NewMigrator(sqlDB, newer, pg.MigratorOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SuperConn(t).Exec(ctx, "UPDATE instance SET schema_version = $1, min_server_version = '0.0.0-newer'", need+1); err != nil {
		t.Fatal(err)
	}
	before := readInstance(t, db)
	code, stdout, stderr := runBothy(t, env, "migrate", "up")
	if code != 0 || stdout != newerSchemaNotice {
		t.Fatalf("up against a newer schema: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if after := readInstance(t, db); after != before {
		t.Fatalf("up against a newer schema changed the row: %+v, was %+v", after, before)
	}
	if strings.Contains(stderr, "instance") {
		t.Errorf("stderr mentions the instance step: %q", stderr)
	}
}
