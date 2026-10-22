// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"io/fs"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// TestMigration_Commands holds commands against docs/spec/03-data-model.md section 4.4 column by
// column, with every check, key and index, and the validated foreign key of
// runs.triggering_command_id (00057, 00058). As bothy_app it then shows the rules: the unique
// partial index is NULLS NOT DISTINCT (two live instance commands of one owner and name fail),
// another scope or a removed command frees the name, the scope columns must match scope_kind, and
// a run keeps working with SET NULL semantics.
func TestMigration_Commands(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	tz := "timestamp with time zone"
	want := []principalColumn{
		{"id", "uuid", false, ""},
		{"owner_principal_id", "uuid", false, ""},
		{"name", "text", false, ""},
		{"description", "text", false, ""},
		{"scope_kind", "text", false, "'instance'::text"},
		{"scope_space_id", "uuid", true, ""},
		{"scope_channel_id", "uuid", true, ""},
		{"options", "jsonb", false, "'[]'::jsonb"},
		{"required_level", "text", false, "'trigger'::text"},
		{"control", "boolean", false, "false"},
		{"version", "bigint", false, "1"},
		{"created_at", tz, false, "now()"},
		{"removed_at", tz, true, ""},
	}
	if got := principalColumns(t, conn, "commands"); !reflect.DeepEqual(got, want) {
		t.Errorf("commands columns:\n got %+v\nwant %+v", got, want)
	}
	cons := principalConstraints(t, conn, "commands")
	slices.Sort(cons)
	wantCons := []string{
		"CHECK ((((scope_kind = 'instance'::text) AND (scope_space_id IS NULL) AND (scope_channel_id IS NULL)) OR ((scope_kind = 'space'::text) AND (scope_space_id IS NOT NULL) AND (scope_channel_id IS NULL)) OR ((scope_kind = 'channel'::text) AND (scope_channel_id IS NOT NULL) AND (scope_space_id IS NULL))))",
		"CHECK ((name ~ '^[a-z0-9][a-z0-9_-]*$'::text))",
		"CHECK ((required_level = ANY (ARRAY['write'::text, 'trigger'::text])))",
		"CHECK ((scope_kind = ANY (ARRAY['instance'::text, 'space'::text, 'channel'::text])))",
		"FOREIGN KEY (owner_principal_id) REFERENCES principals(id) ON DELETE RESTRICT",
		"FOREIGN KEY (scope_channel_id) REFERENCES channels(id) ON DELETE CASCADE",
		"FOREIGN KEY (scope_space_id) REFERENCES spaces(id) ON DELETE CASCADE",
		"PRIMARY KEY (id)",
	}
	slices.Sort(wantCons)
	if !reflect.DeepEqual(cons, wantCons) {
		t.Errorf("commands constraints:\n got %q\nwant %q", cons, wantCons)
	}
	live := " WHERE (removed_at IS NULL)"
	wantIdx := []string{
		"CREATE INDEX commands_name_prefix_idx ON public.commands USING btree (lower(name) text_pattern_ops)" + live,
		"CREATE INDEX commands_owner_live_idx ON public.commands USING btree (owner_principal_id)" + live,
		"CREATE UNIQUE INDEX commands_owner_name_scope_key ON public.commands USING btree (owner_principal_id, lower(name), scope_kind, COALESCE(scope_space_id, scope_channel_id)) NULLS NOT DISTINCT" + live,
		"CREATE UNIQUE INDEX commands_pkey ON public.commands USING btree (id)",
	}
	if got := indexes(t, conn, "commands"); !reflect.DeepEqual(got, wantIdx) {
		t.Errorf("commands indexes:\n got %q\nwant %q", got, wantIdx)
	}
	var def string
	var validated bool
	if err := conn.QueryRow(context.Background(), `SELECT pg_get_constraintdef(oid), convalidated FROM pg_constraint
		WHERE conname = 'runs_triggering_command_id_fkey'`).Scan(&def, &validated); err != nil {
		t.Fatalf("runs_triggering_command_id_fkey: %v", err)
	}
	if def != "FOREIGN KEY (triggering_command_id) REFERENCES commands(id) ON DELETE SET NULL" || !validated {
		t.Errorf("runs command foreign key %q, validated %v", def, validated)
	}

	app := db.Pool(t, pg.RoleApp)
	const (
		agent   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7501"
		person  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7502"
		space   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7511"
		channel = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7521"
		c1      = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7561"
		c2      = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7562"
		c3      = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7563"
		c4      = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7564"
	)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'lotse', 'Lotse')`, agent)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, app, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space, person)
	mustExec(t, app, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'text', 'general', 'a0', 20, $3)`, channel, space, person)
	ins := `INSERT INTO commands (id, owner_principal_id, name, description, scope_kind, scope_space_id, scope_channel_id)
		VALUES ($1, $2, $3, 'Escalate', $4, $5, $6)`
	mustExec(t, app, ins, c1, agent, "escalate", "instance", nil, nil)
	wantState(t, exec(app, ins, c2, agent, "escalate", "instance", nil, nil), "23505", "a second live instance command (NULLS NOT DISTINCT)")
	mustExec(t, app, ins, c2, agent, "escalate", "channel", nil, channel)
	wantCheckViolation(t, exec(app, ins, c3, agent, "x", "space", nil, channel), "a space command with a channel")
	wantCheckViolation(t, exec(app, ins, c3, agent, "x", "instance", space, nil), "an instance command with a space")
	wantCheckViolation(t, exec(app, ins, c3, agent, "Bad", "instance", nil, nil), "an upper-case name")
	mustExec(t, app, `UPDATE commands SET removed_at = now(), version = version + 1 WHERE id = $1`, c1)
	mustExec(t, app, ins, c4, agent, "escalate", "instance", nil, nil)
	wantState(t, exec(app, `DELETE FROM commands WHERE id = $1`, c4), "42501", "a delete by bothy_app")
}

// TestMigration_CommandsDown: down to the version before agents_commands removes commands and the
// foreign key of runs; up applies again. The version comes from the file name, so a renumbering
// of the migration (ADR 0007 rule 4) cannot leave the test pointing past it.
func TestMigration_CommandsDown(t *testing.T) {
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
	before := migrationVersion(t, "agents_commands") - 1
	if _, err := provider.DownTo(ctx, before); err != nil {
		t.Fatalf("down to %d: %v", before, err)
	}
	var left int
	if err := sqlDB.QueryRowContext(ctx, `SELECT (to_regclass('commands') IS NOT NULL)::int
		+ (SELECT count(*)::int FROM pg_constraint WHERE conname = 'runs_triggering_command_id_fkey')`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d of commands and runs_triggering_command_id_fkey left after down", left)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
}

// migrationVersion returns the version of the migration file NNNNN_<slug>.sql in migrations.FS.
func migrationVersion(t *testing.T, slug string) int64 {
	t.Helper()
	files, err := fs.Glob(migrations.FS, "*_"+slug+".sql")
	if err != nil || len(files) != 1 {
		t.Fatalf("migration %s: %v %v", slug, files, err)
	}
	v, err := strconv.ParseInt(strings.SplitN(files[0], "_", 2)[0], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
