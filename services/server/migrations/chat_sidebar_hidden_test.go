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

// sidebarHiddenVersion is the migration of PROJ-ARCHIVE (user_channel_states.hidden_at).
const sidebarHiddenVersion = 102

// TestChatSidebarHiddenMigration_Rows holds the personal archive column as bothy_app: a row stays
// shown (hidden_at NULL) until bothy_app sets it, and setting and clearing it works (01 6.22).
func TestChatSidebarHiddenMigration_Rows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	const person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5401"
	const space = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5411"
	const channel = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5421"
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'sam', 'Sam')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, app, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Dev', 'a0', $2)`, space, person)
	mustExec(t, app, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'forum', 'Projects', 'a0', 20, $3)`, channel, space, person)
	mustExec(t, app, `INSERT INTO user_channel_states (user_id, channel_id) VALUES ($1, $2)`, person, channel)
	hidden := func() bool {
		t.Helper()
		var set bool
		if err := app.QueryRow(ctx, `SELECT hidden_at IS NOT NULL FROM user_channel_states WHERE user_id = $1`, person).Scan(&set); err != nil {
			t.Fatal(err)
		}
		return set
	}
	if hidden() {
		t.Error("a new row is archived; want shown (hidden_at NULL)")
	}
	mustExec(t, app, `UPDATE user_channel_states SET hidden_at = now() WHERE user_id = $1`, person)
	if !hidden() {
		t.Error("hidden_at not set")
	}
	mustExec(t, app, `UPDATE user_channel_states SET hidden_at = NULL WHERE user_id = $1`, person)
	if hidden() {
		t.Error("hidden_at not cleared")
	}
}

// TestChatSidebarHiddenMigration_Down holds the Down section: down to the version before removes
// the column and keeps the rows, up adds it again.
func TestChatSidebarHiddenMigration_Down(t *testing.T) {
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
	column := func() bool {
		t.Helper()
		var exists bool
		if err := sqlDB.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns
			WHERE table_name = 'user_channel_states' AND column_name = 'hidden_at')`).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		return exists
	}
	if _, err := provider.DownTo(ctx, sidebarHiddenVersion-1); err != nil {
		t.Fatalf("down to %d: %v", sidebarHiddenVersion-1, err)
	}
	if column() {
		t.Error("hidden_at survives the down migration")
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	if !column() {
		t.Error("hidden_at missing after up")
	}
}
