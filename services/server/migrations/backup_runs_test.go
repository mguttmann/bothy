// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// TestMigrationBackupRuns holds backup_runs against docs/spec/03-data-model.md section 4.12
// column by column with its checks, key and index (M7-02), the rights of 16 7.1 rule 3 (bothy_app
// reads it for the console and the status page, bothy_backup may read, insert and update it and
// holds no right on any other table), the check that a row is running exactly while finished_at
// is NULL, and the Down section, which drops the table.
func TestMigrationBackupRuns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	tz := "timestamp with time zone"

	wantCols := []principalColumn{
		{"id", "uuid", false, ""}, {"kind", "text", false, ""}, {"state", "text", false, "'running'::text"},
		{"started_at", tz, false, "now()"}, {"finished_at", tz, true, ""}, {"size_bytes", "bigint", true, ""},
		{"manifest", "jsonb", false, "'{}'::jsonb"}, {"checks", "jsonb", false, "'{}'::jsonb"}, {"error", "text", true, ""},
	}
	if got := principalColumns(t, conn, "backup_runs"); !reflect.DeepEqual(got, wantCols) {
		t.Errorf("backup_runs columns:\n got %+v\nwant %+v", got, wantCols)
	}
	wantCons := []string{
		"CHECK (((state = 'running'::text) = (finished_at IS NULL)))",
		"CHECK ((kind = ANY (ARRAY['pg_full'::text, 'pg_diff'::text, 'logical'::text, 'objects'::text, 'restore_test'::text])))",
		"CHECK ((state = ANY (ARRAY['running'::text, 'succeeded'::text, 'failed'::text])))",
		"PRIMARY KEY (id)",
	}
	cons := principalConstraints(t, conn, "backup_runs")
	slices.Sort(cons)
	slices.Sort(wantCons)
	if !reflect.DeepEqual(cons, wantCons) {
		t.Errorf("backup_runs constraints:\n got %q\nwant %q", cons, wantCons)
	}
	wantIdx := []string{
		"CREATE INDEX backup_runs_kind_started_at_idx ON public.backup_runs USING btree (kind, started_at DESC)",
		"CREATE UNIQUE INDEX backup_runs_pkey ON public.backup_runs USING btree (id)",
	}
	if got := indexes(t, conn, "backup_runs"); !reflect.DeepEqual(got, wantIdx) {
		t.Errorf("backup_runs indexes:\n got %q\nwant %q", got, wantIdx)
	}

	// Rights: bothy_backup holds SELECT, INSERT and UPDATE on backup_runs and nothing anywhere
	// else; bothy_app reads backup_runs (appPrivileges of privileges_test.go holds the rest).
	rows, err := conn.Query(ctx, `SELECT c.relname, p.priv,
			has_table_privilege('bothy_backup', c.oid, p.priv), has_table_privilege('bothy_app', c.oid, p.priv)
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace,
			unnest(ARRAY['SELECT','INSERT','UPDATE','DELETE','TRUNCATE','REFERENCES','TRIGGER','MAINTAIN']) AS p(priv)
		WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p', 'v', 'm', 'f') ORDER BY 1, 2`)
	if err != nil {
		t.Fatal(err)
	}
	var backupRights []string
	for rows.Next() {
		var table, priv string
		var backup, app bool
		if err := rows.Scan(&table, &priv, &backup, &app); err != nil {
			t.Fatal(err)
		}
		if backup {
			backupRights = append(backupRights, table+":"+priv)
		}
		if table == "backup_runs" && app != (priv == "SELECT") {
			t.Errorf("bothy_app on backup_runs: %s is %v", priv, app)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"backup_runs:INSERT", "backup_runs:SELECT", "backup_runs:UPDATE"}; !slices.Equal(backupRights, want) {
		t.Errorf("rights of bothy_backup %q, want %q", backupRights, want)
	}

	// The state check: running exactly while finished_at is NULL; kind and state are closed.
	insert := `INSERT INTO backup_runs (id, kind, state, finished_at) VALUES (gen_random_uuid(), $1, $2, $3)`
	now := time.Date(2026, 10, 18, 2, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		kind, state string
		finished    *time.Time
		check       string
	}{
		{"pg_full", "running", &now, "backup_runs_check"},
		{"pg_full", "succeeded", nil, "backup_runs_check"},
		{"pg_full", "done", &now, "backup_runs_state_check"},
		{"snapshot", "running", nil, "backup_runs_kind_check"},
	} {
		if _, err := conn.Exec(ctx, insert, c.kind, c.state, c.finished); err == nil || !strings.Contains(err.Error(), c.check) {
			t.Errorf("insert %s %s finished %v: %v, want %s", c.kind, c.state, c.finished, err, c.check)
		}
	}
	for _, c := range []struct {
		kind, state string
		finished    *time.Time
	}{{"restore_test", "running", nil}, {"logical", "failed", &now}, {"objects", "succeeded", &now}} {
		if _, err := conn.Exec(ctx, insert, c.kind, c.state, c.finished); err != nil {
			t.Errorf("insert %s %s: %v", c.kind, c.state, err)
		}
	}

	// Down drops the table; Up creates it again.
	pool := db.OpenPool(t, db.Config(pg.RoleOwner).ForMigrate(5*time.Second))
	sqlDB := pool.OpenSQL()
	t.Cleanup(func() { _ = sqlDB.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 49); err != nil {
		t.Fatalf("down to 49: %v", err)
	}
	var exists bool
	if err := conn.QueryRow(ctx, `SELECT to_regclass('backup_runs') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Errorf("backup_runs after down: exists %v, %v", exists, err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
}
