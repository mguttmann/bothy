// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// TestMigrations_AppsTables holds apps and app_installations against docs/spec/03-datenmodell.md
// section 4.4 column by column, with the checks, the foreign keys and both unique indexes
// (M2-24).
func TestMigrations_AppsTables(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ts := "timestamp with time zone"
	apps := []principalColumn{
		{"principal_id", "uuid", false, ""},
		{"slug", "text", false, ""},
		{"manifest", "jsonb", false, ""},
		{"manifest_version", "text", false, ""},
		{"manifest_sha256", "bytea", false, ""},
		{"publisher", "text", false, "''::text"},
		{"homepage_url", "text", true, ""},
		{"created_at", ts, false, "now()"},
		{"updated_at", ts, false, "now()"},
	}
	installs := []principalColumn{
		{"id", "uuid", false, ""},
		{"app_id", "uuid", false, ""},
		{"granted_scopes", "ARRAY", false, "'{}'::text[]"},
		{"event_subscriptions", "ARRAY", false, "'{}'::text[]"},
		{"state", "text", false, "'active'::text"},
		{"installed_by", "uuid", false, ""},
		{"installed_at", ts, false, "now()"},
		{"uninstalled_at", ts, true, ""},
		{"uninstalled_by", "uuid", true, ""},
	}
	if got := principalColumns(t, conn, "apps"); !reflect.DeepEqual(got, apps) {
		t.Errorf("apps columns:\n got %+v\nwant %+v", got, apps)
	}
	if got := principalColumns(t, conn, "app_installations"); !reflect.DeepEqual(got, installs) {
		t.Errorf("app_installations columns:\n got %+v\nwant %+v", got, installs)
	}
	for table, want := range map[string][]string{
		"apps": {
			"FOREIGN KEY (principal_id) REFERENCES principals(id) ON DELETE RESTRICT",
			"PRIMARY KEY (principal_id)",
		},
		"app_installations": {
			"CHECK ((state = ANY (ARRAY['active'::text, 'suspended'::text, 'uninstalled'::text])))",
			"FOREIGN KEY (app_id) REFERENCES apps(principal_id) ON DELETE RESTRICT",
			"FOREIGN KEY (installed_by) REFERENCES principals(id) ON DELETE RESTRICT",
			"FOREIGN KEY (uninstalled_by) REFERENCES principals(id) ON DELETE RESTRICT",
			"PRIMARY KEY (id)",
		},
	} {
		slices.Sort(want)
		got := principalConstraints(t, conn, table)
		slices.Sort(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s constraints:\n got %q\nwant %q", table, got, want)
		}
	}
	for table, want := range map[string][]string{
		"apps": {
			"CREATE UNIQUE INDEX apps_pkey ON public.apps USING btree (principal_id)",
			"CREATE UNIQUE INDEX apps_slug_lower_key ON public.apps USING btree (lower(slug))",
		},
		"app_installations": {
			"CREATE UNIQUE INDEX app_installations_app_live_key ON public.app_installations USING btree (app_id) WHERE (state <> 'uninstalled'::text)",
			"CREATE UNIQUE INDEX app_installations_pkey ON public.app_installations USING btree (id)",
		},
	} {
		if got := indexes(t, conn, table); !reflect.DeepEqual(got, want) {
			t.Errorf("%s indexes:\n got %q\nwant %q", table, got, want)
		}
	}
}

// TestMigrations_AppsConstraints holds the behaviour of the two tables for bothy_app: the kind
// trigger refuses an apps row of a person, the case-insensitive slug and the one live
// installation per app are unique, the state check holds, and bothy_app never deletes (M2-24
// design note 16).
func TestMigrations_AppsConstraints(t *testing.T) {
	t.Parallel()
	app := pgtest.New(t).Pool(t, pg.RoleApp)
	const tracker = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a01"
	const person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a02"
	const other = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a03"
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'app', 'tracker', 'Tracker')`, tracker)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'robin', 'Robin')`, person)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'app', 'tracker2', 'Tracker 2')`, other)
	insApp := `INSERT INTO apps (principal_id, slug, manifest, manifest_version, manifest_sha256) VALUES ($1, $2, '{}', '1.0.0', '\x00')`
	wantCheckViolation(t, exec(app, insApp, person, "robin"), "apps row of a person")
	mustExec(t, app, insApp, tracker, "tracker")
	wantState(t, exec(app, insApp, other, "Tracker"), "23505", "slug taken in other case")
	wantCheckViolation(t, exec(app, `UPDATE apps SET principal_id = $1 WHERE principal_id = $2`, person, tracker), "apps row moved to a person")
	ins := `INSERT INTO app_installations (id, app_id, installed_by, state) VALUES ($1, $2, $3, $4)`
	mustExec(t, app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9b01", tracker, person, "uninstalled")
	mustExec(t, app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9b02", tracker, person, "active")
	wantState(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9b03", tracker, person, "suspended"), "23505", "second live installation")
	wantCheckViolation(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9b04", tracker, person, "paused"), "unknown state")
	wantState(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9b05", other, person, "active"), "23503", "installation of an unknown app")
	mustExec(t, app, `UPDATE app_installations SET state = 'suspended', granted_scopes = '{messages:write}' WHERE id = $1`, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9b02")
	mustExec(t, app, `UPDATE apps SET updated_at = now() WHERE principal_id = $1`, tracker)
	wantState(t, exec(app, `DELETE FROM app_installations WHERE app_id = $1`, tracker), "42501", "bothy_app deletes an installation")
	wantState(t, exec(app, `DELETE FROM apps WHERE principal_id = $1`, tracker), "42501", "bothy_app deletes an app")
}

// TestMigrations_AppsDown holds the Down sections: down to version 15 removes both tables and the
// trigger function, up applies again.
func TestMigrations_AppsDown(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	pool := db.OpenPool(t, db.Config(pg.RoleOwner).ForMigrate(5*time.Second))
	sqlDB := pool.OpenSQL()
	t.Cleanup(func() { _ = sqlDB.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 15); err != nil {
		t.Fatalf("down to 15: %v", err)
	}
	var left int
	if err := sqlDB.QueryRowContext(ctx, `SELECT (to_regclass('apps') IS NOT NULL)::int + (to_regclass('app_installations') IS NOT NULL)::int
		+ (SELECT count(*) FROM pg_proc WHERE proname = 'apps_principal_kind_check')::int`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d objects of the apps migrations survive the down migration", left)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	if err := sqlDB.QueryRowContext(ctx, "SELECT count(*) FROM app_installations").Scan(&left); err != nil {
		t.Fatalf("app_installations after up again: %v", err)
	}
}
