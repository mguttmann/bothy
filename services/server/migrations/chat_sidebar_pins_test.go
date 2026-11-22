// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// sidebarPinsVersion is the migration of FEAT-PIN-CHANNELS (user_channel_states.pinned_at and
// pinned_threads).
const sidebarPinsVersion = 111

// TestChatSidebarPinsMigration_Rows holds the personal pin columns as bothy_app: a new row is not
// pinned (pinned_at NULL, pinned_threads empty) until bothy_app sets them, and setting and clearing
// them works (01 6.24).
func TestChatSidebarPinsMigration_Rows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	const person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5501"
	const space = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5511"
	const channel = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5521"
	const post = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5531"
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'sam', 'Sam')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, app, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Dev', 'a0', $2)`, space, person)
	mustExec(t, app, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'forum', 'Projects', 'a0', 20, $3)`, channel, space, person)
	mustExec(t, app, `INSERT INTO user_channel_states (user_id, channel_id) VALUES ($1, $2)`, person, channel)
	pins := func() (bool, int) {
		t.Helper()
		var set bool
		var n int
		if err := app.QueryRow(ctx, `SELECT pinned_at IS NOT NULL, cardinality(pinned_threads) FROM user_channel_states WHERE user_id = $1`,
			person).Scan(&set, &n); err != nil {
			t.Fatal(err)
		}
		return set, n
	}
	if set, n := pins(); set || n != 0 {
		t.Errorf("a new row has pins (%v, %d); want none", set, n)
	}
	mustExec(t, app, `UPDATE user_channel_states SET pinned_at = now(), pinned_threads = ARRAY[$2::uuid] WHERE user_id = $1`, person, post)
	if set, n := pins(); !set || n != 1 {
		t.Errorf("pins not set: %v, %d", set, n)
	}
	mustExec(t, app, `UPDATE user_channel_states SET pinned_at = NULL, pinned_threads = '{}' WHERE user_id = $1`, person)
	if set, n := pins(); set || n != 0 {
		t.Errorf("pins not cleared: %v, %d", set, n)
	}
}

// TestChatSidebarPinsMigration_Down holds the Down section: down to the version before removes
// both columns and keeps the rows, up adds them again.
func TestChatSidebarPinsMigration_Down(t *testing.T) {
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
	columns := func() int {
		t.Helper()
		var n int
		if err := sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.columns
			WHERE table_name = 'user_channel_states' AND column_name IN ('pinned_at', 'pinned_threads')`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if _, err := provider.DownTo(ctx, sidebarPinsVersion-1); err != nil {
		t.Fatalf("down to %d: %v", sidebarPinsVersion-1, err)
	}
	if n := columns(); n != 0 {
		t.Errorf("%d pin columns survive the down migration", n)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	if n := columns(); n != 2 {
		t.Errorf("%d pin columns after up, want 2", n)
	}
}
