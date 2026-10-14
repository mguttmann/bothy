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

// TestAgentsMigration_Schema holds agents against docs/spec/03-datenmodell.md section 4.4 column
// by column, with the checks, the foreign keys and the two indexes (M1-51).
func TestAgentsMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ts := "timestamp with time zone"
	columns := []principalColumn{
		{"principal_id", "uuid", false, ""},
		{"adapter_kind", "text", false, ""},
		{"description", "text", false, "''::text"},
		{"operator_config", "jsonb", false, "'{}'::jsonb"},
		{"billing_mode", "text", false, "'organization'::text"},
		{"credential_holder_id", "uuid", true, ""},
		{"billing_mode_changed_at", ts, true, ""},
		{"billing_mode_changed_by", "uuid", true, ""},
		{"tool_class", "text", false, "'chat'::text"},
		{"reported_tool_class", "text", true, ""},
		{"reported_runtime_state", "text", false, "'unknown'::text"},
		{"reported_billing_mode", "text", true, ""},
		{"reported_runtime_version", "text", true, ""},
		{"reported_details", "jsonb", false, "'{}'::jsonb"},
		{"last_heartbeat_at", ts, true, ""},
		{"version", "bigint", false, "1"},
	}
	constraints := []string{
		"CHECK (((billing_mode = 'personal'::text) = (credential_holder_id IS NOT NULL)))",
		"CHECK ((billing_mode = ANY (ARRAY['personal'::text, 'organization'::text])))",
		"CHECK ((reported_billing_mode = ANY (ARRAY['personal'::text, 'organization'::text])))",
		"CHECK ((reported_runtime_state = ANY (ARRAY['unknown'::text, 'online'::text, 'degraded'::text, 'offline'::text])))",
		"CHECK ((reported_tool_class = ANY (ARRAY['chat'::text, 'scoped'::text, 'host'::text])))",
		"CHECK ((tool_class = ANY (ARRAY['chat'::text, 'scoped'::text, 'host'::text])))",
		"FOREIGN KEY (billing_mode_changed_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"FOREIGN KEY (credential_holder_id) REFERENCES users(principal_id) ON DELETE RESTRICT",
		"FOREIGN KEY (principal_id) REFERENCES principals(id) ON DELETE RESTRICT",
		"PRIMARY KEY (principal_id)",
	}
	idx := []string{
		"CREATE INDEX agents_credential_holder_personal_idx ON public.agents USING btree (credential_holder_id) WHERE (billing_mode = 'personal'::text)",
		"CREATE INDEX agents_last_heartbeat_at_idx ON public.agents USING btree (last_heartbeat_at)",
		"CREATE UNIQUE INDEX agents_pkey ON public.agents USING btree (principal_id)",
	}
	if got := principalColumns(t, conn, "agents"); !reflect.DeepEqual(got, columns) {
		t.Errorf("columns:\n got %+v\nwant %+v", got, columns)
	}
	slices.Sort(constraints)
	got := principalConstraints(t, conn, "agents")
	slices.Sort(got)
	if !reflect.DeepEqual(got, constraints) {
		t.Errorf("constraints:\n got %q\nwant %q", got, constraints)
	}
	slices.Sort(idx)
	got = indexes(t, conn, "agents")
	slices.Sort(got)
	if !reflect.DeepEqual(got, idx) {
		t.Errorf("indexes:\n got %q\nwant %q", got, idx)
	}
}

// TestAgentsMigration_Constraints exercises the rules of 03 as bothy_app: the principal must be
// of kind agent (trigger), personal requires a holder and organization forbids one, the holder is
// a person, and bothy_app may update but not delete.
func TestAgentsMigration_Constraints(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	const agent = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a01"
	const person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a02"
	const other = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9a03"
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'lotse', 'Lotse')`, agent)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'scout', 'Scout')`, other)
	ins := `INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, $2)`
	wantState(t, exec(app, ins, person, "hermes"), "23514", "agents row for a person")
	var mode, class, state string
	var version int64
	if err := app.QueryRow(context.Background(), ins+` RETURNING billing_mode, tool_class, reported_runtime_state, version`, agent, "hermes").
		Scan(&mode, &class, &state, &version); err != nil || mode != "organization" || class != "chat" || state != "unknown" || version != 1 {
		t.Fatalf("defaults %s %s %s %d, %v", mode, class, state, version, err)
	}
	wantState(t, exec(app, `UPDATE agents SET billing_mode = 'personal' WHERE principal_id = $1`, agent), "23514", "personal without holder")
	wantState(t, exec(app, `UPDATE agents SET credential_holder_id = $2 WHERE principal_id = $1`, agent, person), "23514", "organization with holder")
	wantState(t, exec(app, `UPDATE agents SET billing_mode = 'personal', credential_holder_id = $2 WHERE principal_id = $1`, agent, other),
		"23503", "holder that is not a person")
	mustExec(t, app, `UPDATE agents SET billing_mode = 'personal', credential_holder_id = $2, billing_mode_changed_by = $2,
		billing_mode_changed_at = now(), version = version + 1 WHERE principal_id = $1`, agent, person)
	wantState(t, exec(app, `UPDATE agents SET tool_class = 'shell' WHERE principal_id = $1`, agent), "23514", "unknown tool class")
	wantState(t, exec(app, `UPDATE agents SET principal_id = $2 WHERE principal_id = $1`, agent, person), "23514", "moved to a person")
	wantState(t, exec(app, `DELETE FROM agents WHERE principal_id = $1`, agent), "42501", "bothy_app deletes")
}

// TestAgentsMigration_Down holds the Down section: down to version 15 removes agents and its
// trigger function, up applies again.
func TestAgentsMigration_Down(t *testing.T) {
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
	var exists bool
	if err := sqlDB.QueryRowContext(ctx, "SELECT to_regclass('agents') IS NOT NULL OR to_regprocedure('agents_kind_check()') IS NOT NULL").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("agents or its trigger function survives the down migration")
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	if err := sqlDB.QueryRowContext(ctx, "SELECT to_regclass('agents') IS NOT NULL").Scan(&exists); err != nil || !exists {
		t.Fatalf("agents after up again: %v, %v", exists, err)
	}
}
