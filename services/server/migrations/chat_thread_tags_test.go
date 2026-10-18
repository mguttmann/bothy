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

// TestChatThreadTagsMigration_Schema holds thread_tags against docs/spec/03-data-model.md
// section 4.5 column by column, with its keys and the index of the tag filter (M1-55).
func TestChatThreadTagsMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	want := []principalColumn{
		{"thread_id", "uuid", false, ""},
		{"tag_id", "uuid", false, ""},
		{"added_by", "uuid", false, ""},
		{"added_at", "timestamp with time zone", false, "now()"},
	}
	if got := principalColumns(t, conn, "thread_tags"); !reflect.DeepEqual(got, want) {
		t.Errorf("thread_tags columns:\n got %+v\nwant %+v", got, want)
	}
	cons := principalConstraints(t, conn, "thread_tags")
	slices.Sort(cons)
	wantCons := []string{
		"FOREIGN KEY (added_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"FOREIGN KEY (tag_id) REFERENCES forum_tags(id) ON DELETE CASCADE",
		"FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE",
		"PRIMARY KEY (thread_id, tag_id)",
	}
	if !reflect.DeepEqual(cons, wantCons) {
		t.Errorf("thread_tags constraints:\n got %q\nwant %q", cons, wantCons)
	}
	idx := indexes(t, conn, "thread_tags")
	slices.Sort(idx)
	wantIdx := []string{
		"CREATE INDEX thread_tags_tag_id_idx ON public.thread_tags USING btree (tag_id)",
		"CREATE UNIQUE INDEX thread_tags_pkey ON public.thread_tags USING btree (thread_id, tag_id)",
	}
	if !reflect.DeepEqual(idx, wantIdx) {
		t.Errorf("thread_tags indexes:\n got %q\nwant %q", idx, wantIdx)
	}
}

// TestChatThreadTagsMigration_Grants: bothy_app reads, inserts and deletes rows (setThreadTags
// replaces the set, deleteForumTag drops the rows of its tag), a tag is assigned once per thread.
func TestChatThreadTagsMigration_Grants(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	const person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5401"
	const space = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5411"
	const forum = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5421"
	const thread = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5431"
	const tag = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5441"
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, app, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space, person)
	mustExec(t, app, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'forum', 'bugs', 'a0', 20, $3)`, forum, space, person)
	mustExec(t, app, `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'thread', $3)`, thread, forum, person)
	mustExec(t, app, `INSERT INTO forum_tags (id, channel_id, name, sort_key) VALUES ($1, $2, 'bug', 'a0')`, tag, forum)
	ins := `INSERT INTO thread_tags (thread_id, tag_id, added_by) VALUES ($1, $2, $3)`
	mustExec(t, app, ins, thread, tag, person)
	wantState(t, exec(app, ins, thread, tag, person), "23505", "a tag assigned twice")
	mustExec(t, app, `DELETE FROM thread_tags WHERE tag_id = $1`, tag)
	mustExec(t, app, ins, thread, tag, person)
}

// TestChatThreadTagsMigration_Down: down to version 26 removes thread_tags, up applies again.
func TestChatThreadTagsMigration_Down(t *testing.T) {
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
	if _, err := provider.DownTo(ctx, 26); err != nil {
		t.Fatalf("down to 26: %v", err)
	}
	var exists bool
	if err := sqlDB.QueryRowContext(ctx, `SELECT to_regclass('thread_tags') IS NOT NULL`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("thread_tags survives the down migration")
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
}
