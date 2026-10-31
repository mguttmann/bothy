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

// sidebarOrderVersion is the migration of M4-51 (user_space_states.channel_order and
// user_channel_states).
const sidebarOrderVersion = 85

// TestChatSidebarOrderMigration_Schema holds user_space_states.channel_order and
// user_channel_states against docs/spec/03-data-model.md section 4.5 column by column, with the
// foreign keys and the primary key (M4-51).
func TestChatSidebarOrderMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ts := "timestamp with time zone"
	space := principalColumns(t, conn, "user_space_states")
	if !slices.Contains(space, principalColumn{"channel_order", "ARRAY", true, ""}) {
		t.Errorf("user_space_states.channel_order missing or wrong: %+v", space)
	}
	want := []principalColumn{
		{"user_id", "uuid", false, ""},
		{"channel_id", "uuid", false, ""},
		{"thread_order", "ARRAY", false, "'{}'::uuid[]"},
		{"version", "bigint", false, "1"},
		{"updated_by_device_id", "uuid", true, ""},
		{"updated_at", ts, false, "now()"},
	}
	if got := principalColumns(t, conn, "user_channel_states"); !reflect.DeepEqual(got, want) {
		t.Errorf("user_channel_states columns:\n got %+v\nwant %+v", got, want)
	}
	constraints := []string{
		"FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE",
		"FOREIGN KEY (updated_by_device_id) REFERENCES devices(id) ON DELETE SET NULL",
		"FOREIGN KEY (user_id) REFERENCES users(principal_id) ON DELETE CASCADE",
		"PRIMARY KEY (user_id, channel_id)",
	}
	slices.Sort(constraints)
	if got := principalConstraints(t, conn, "user_channel_states"); !reflect.DeepEqual(got, constraints) {
		t.Errorf("user_channel_states constraints:\n got %q\nwant %q", got, constraints)
	}
	idx := []string{"CREATE UNIQUE INDEX user_channel_states_pkey ON public.user_channel_states USING btree (user_id, channel_id)"}
	if got := indexes(t, conn, "user_channel_states"); !reflect.DeepEqual(got, idx) {
		t.Errorf("user_channel_states indexes:\n got %q\nwant %q", got, idx)
	}
	for _, c := range []struct{ table, column string }{{"user_space_states", "channel_order"}, {"user_channel_states", "thread_order"}} {
		var elem string
		if err := conn.QueryRow(context.Background(), `SELECT udt_name FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2`, c.table, c.column).Scan(&elem); err != nil || elem != "_uuid" {
			t.Errorf("%s.%s element type %q, %v; want _uuid", c.table, c.column, elem, err)
		}
	}
}

// TestChatSidebarOrderMigration_Constraints exercises the rules as bothy_app: a sidebar row keeps
// a NULL channel order by default, a channel row starts with an empty list and version 1, it
// cascades with its person and bothy_app may delete it (erasure, 20 section 4.4).
func TestChatSidebarOrderMigration_Constraints(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	const person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5301"
	const space = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5311"
	const channel = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5321"
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, app, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Dev', 'a0', $2)`, space, person)
	mustExec(t, app, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'forum', 'Projects', 'a0', 20, $3)`, channel, space, person)
	mustExec(t, app, `INSERT INTO user_space_states (user_id, space_id) VALUES ($1, $2)`, person, space)
	var isNull bool
	if err := app.QueryRow(ctx, `SELECT channel_order IS NULL FROM user_space_states WHERE user_id = $1`, person).Scan(&isNull); err != nil || !isNull {
		t.Errorf("channel_order default NULL: %v, %v", isNull, err)
	}
	mustExec(t, app, `UPDATE user_space_states SET channel_order = ARRAY[$2::uuid] WHERE user_id = $1`, person, channel)
	mustExec(t, app, `INSERT INTO user_channel_states (user_id, channel_id) VALUES ($1, $2)`, person, channel)
	var n int
	var version int64
	if err := app.QueryRow(ctx, `SELECT cardinality(thread_order), version FROM user_channel_states WHERE user_id = $1`, person).Scan(&n, &version); err != nil ||
		n != 0 || version != 1 {
		t.Errorf("new row: %d items, version %d, %v", n, version, err)
	}
	wantState(t, exec(app, `INSERT INTO user_channel_states (user_id, channel_id) VALUES ($1, $2)`, person,
		"0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f53ff"), "23503", "an unknown channel")
	mustExec(t, app, `DELETE FROM user_channel_states WHERE user_id = $1`, person)
}

// TestChatSidebarOrderMigration_Down holds the Down section: down to the version before removes
// the table and the column, up applies again.
func TestChatSidebarOrderMigration_Down(t *testing.T) {
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
	if _, err := provider.DownTo(ctx, sidebarOrderVersion-1); err != nil {
		t.Fatalf("down to %d: %v", sidebarOrderVersion-1, err)
	}
	var exists bool
	if err := sqlDB.QueryRowContext(ctx, `SELECT to_regclass('user_channel_states') IS NOT NULL OR EXISTS (
		SELECT 1 FROM information_schema.columns WHERE table_name = 'user_space_states' AND column_name = 'channel_order')`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("the table or the column survives the down migration")
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
}
