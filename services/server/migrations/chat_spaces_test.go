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

// TestChatSpacesMigration_Schema holds spaces and user_space_states against
// docs/spec/03-data-model.md section 4.5 column by column, with the foreign keys, the partial
// indexes, the C collation of both sort keys and the foreign key of grants.target_space_id (M1-52).
func TestChatSpacesMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ts := "timestamp with time zone"
	for _, c := range []struct {
		table       string
		columns     []principalColumn
		constraints []string
		idx         []string
	}{
		{"spaces", []principalColumn{
			{"id", "uuid", false, ""},
			{"name", "text", false, ""},
			{"icon", "text", true, ""},
			{"color", "text", true, ""},
			{"sort_key", "text", false, ""},
			{"default_operator_agent_id", "uuid", true, ""},
			{"created_by", "uuid", false, ""},
			{"created_at", ts, false, "now()"},
			{"archived_at", ts, true, ""},
			{"version", "bigint", false, "1"},
		}, []string{
			"FOREIGN KEY (created_by) REFERENCES principals(id) ON DELETE RESTRICT",
			"FOREIGN KEY (default_operator_agent_id) REFERENCES agents(principal_id) ON DELETE RESTRICT",
			"PRIMARY KEY (id)",
		}, []string{
			"CREATE INDEX spaces_default_operator_agent_id_idx ON public.spaces USING btree (default_operator_agent_id)",
			"CREATE INDEX spaces_sort_key_idx ON public.spaces USING btree (sort_key) WHERE (archived_at IS NULL)",
			"CREATE UNIQUE INDEX spaces_name_active_key ON public.spaces USING btree (lower(name)) WHERE (archived_at IS NULL)",
			"CREATE UNIQUE INDEX spaces_pkey ON public.spaces USING btree (id)",
		}},
		{"user_space_states", []principalColumn{
			{"user_id", "uuid", false, ""},
			{"space_id", "uuid", false, ""},
			{"collapsed", "boolean", false, "false"},
			{"sort_key", "text", true, ""},
			{"version", "bigint", false, "1"},
			{"updated_by_device_id", "uuid", true, ""},
			{"updated_at", ts, false, "now()"},
			// M4-51 (migration 00085): the personal channel order.
			{"channel_order", "ARRAY", true, ""},
		}, []string{
			"FOREIGN KEY (space_id) REFERENCES spaces(id) ON DELETE CASCADE",
			"FOREIGN KEY (updated_by_device_id) REFERENCES devices(id) ON DELETE SET NULL",
			"FOREIGN KEY (user_id) REFERENCES users(principal_id) ON DELETE CASCADE",
			"PRIMARY KEY (user_id, space_id)",
		}, []string{
			"CREATE UNIQUE INDEX user_space_states_pkey ON public.user_space_states USING btree (user_id, space_id)",
		}},
	} {
		if got := principalColumns(t, conn, c.table); !reflect.DeepEqual(got, c.columns) {
			t.Errorf("%s columns:\n got %+v\nwant %+v", c.table, got, c.columns)
		}
		slices.Sort(c.constraints)
		if got := principalConstraints(t, conn, c.table); !reflect.DeepEqual(got, c.constraints) {
			t.Errorf("%s constraints:\n got %q\nwant %q", c.table, got, c.constraints)
		}
		slices.Sort(c.idx)
		got := indexes(t, conn, c.table)
		slices.Sort(got)
		if !reflect.DeepEqual(got, c.idx) {
			t.Errorf("%s indexes:\n got %q\nwant %q", c.table, got, c.idx)
		}
		var collation string
		if err := conn.QueryRow(context.Background(), `SELECT coalesce(collation_name, '') FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1 AND column_name = 'sort_key'`, c.table).Scan(&collation); err != nil || collation != "C" {
			t.Errorf("%s.sort_key collation %q, %v", c.table, collation, err)
		}
	}
	var def string
	var valid bool
	if err := conn.QueryRow(context.Background(), `SELECT pg_get_constraintdef(oid), convalidated FROM pg_constraint
		WHERE conrelid = 'grants'::regclass AND conname = 'grants_target_space_id_fkey'`).Scan(&def, &valid); err != nil ||
		def != "FOREIGN KEY (target_space_id) REFERENCES spaces(id) ON DELETE RESTRICT" || !valid {
		t.Errorf("grants_target_space_id_fkey %q validated %v, %v", def, valid, err)
	}
}

// TestChatSpacesMigration_Constraints exercises the rules as bothy_app: a name is unique
// case-insensitively among spaces that are not archived, an archived space frees its name, the
// sidebar row cascades with its space and bothy_app may not delete a space.
func TestChatSpacesMigration_Constraints(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	const person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5201"
	const s1 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5211"
	const s2 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5212"
	const s3 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5213"
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	ins := `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, $2, $3, $4)`
	mustExec(t, app, ins, s1, "Support", "a0", person)
	wantState(t, exec(app, ins, s2, "support", "a1", person), "23505", "a name taken case-insensitively")
	mustExec(t, app, `UPDATE spaces SET archived_at = now(), version = version + 1 WHERE id = $1`, s1)
	mustExec(t, app, ins, s2, "support", "a1", person)
	wantState(t, exec(app, ins, s3, "Marketing", "a2", "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f52ff"), "23503", "an unknown creator")
	mustExec(t, app, `INSERT INTO user_space_states (user_id, space_id, collapsed, sort_key) VALUES ($1, $2, true, 'a0V')`, person, s2)
	wantState(t, exec(app, `DELETE FROM spaces WHERE id = $1`, s2), "42501", "bothy_app deletes a space")
	mustExec(t, app, `DELETE FROM user_space_states WHERE user_id = $1`, person)
	var sorted []string
	rows, err := app.Query(context.Background(), `SELECT k FROM unnest(ARRAY['a0', 'Zz', 'a0V', 'a0a']::text[]) AS k ORDER BY k COLLATE "C"`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		sorted = append(sorted, k)
	}
	if !slices.Equal(sorted, []string{"Zz", "a0", "a0V", "a0a"}) {
		t.Fatalf("byte order %v", sorted)
	}
}

// TestChatSpacesMigration_Down holds the Down sections: down to version 22 removes both tables
// and the foreign key of grants, up applies again.
func TestChatSpacesMigration_Down(t *testing.T) {
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
	if _, err := provider.DownTo(ctx, 22); err != nil {
		t.Fatalf("down to 22: %v", err)
	}
	var exists bool
	if err := sqlDB.QueryRowContext(ctx, `SELECT to_regclass('spaces') IS NOT NULL OR to_regclass('user_space_states') IS NOT NULL
		OR EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'grants_target_space_id_fkey')`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("a table or the foreign key of grants survives the down migration")
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
}
