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

// TestChatChannelsMigration_Schema holds channels, channel_skill_bindings, forum_tags and threads
// against docs/spec/03-datenmodell.md section 4.5 column by column, with the checks, foreign keys
// and indexes, minus the two documented deviations of threads (no search columns, no foreign key
// to messages), and the foreign key of grants.target_channel_id (M1-53).
func TestChatChannelsMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ts := "timestamp with time zone"
	for _, c := range []struct {
		table       string
		columns     []principalColumn
		constraints []string
		idx         []string
	}{
		{"channels", []principalColumn{
			{"id", "uuid", false, ""},
			{"space_id", "uuid", true, ""},
			{"kind", "text", false, ""},
			{"name", "text", false, ""},
			{"topic", "text", false, "''::text"},
			{"sort_key", "text", false, ""},
			{"operator_inherit", "boolean", false, "true"},
			{"operator_agent_id", "uuid", true, ""},
			{"require_mention", "boolean", false, "true"},
			{"thread_require_mention", "boolean", false, "false"},
			{"free_response", "boolean", false, "false"},
			{"auto_thread", "boolean", false, "true"},
			{"allow_mentioned_agents", "boolean", false, "false"},
			{"history_backfill_limit", "integer", false, ""},
			{"prompt", "text", false, "''::text"},
			{"forum_default_sort", "text", true, ""},
			{"default_notification_level", "text", true, ""},
			{"read_only", "boolean", false, "false"},
			{"created_by", "uuid", false, ""},
			{"created_at", ts, false, "now()"},
			{"archived_at", ts, true, ""},
			{"version", "bigint", false, "1"},
			{"settings_version", "bigint", false, "1"},
		}, []string{
			"CHECK (((kind <> 'dm'::text) OR ((NOT operator_inherit) AND (operator_agent_id IS NULL) AND (NOT read_only))))",
			"CHECK (((kind = 'dm'::text) = (space_id IS NULL)))",
			"CHECK (((NOT operator_inherit) OR (operator_agent_id IS NULL)))",
			"CHECK ((default_notification_level = ANY (ARRAY['all'::text, 'mentions'::text, 'mute'::text])))",
			"CHECK ((forum_default_sort = ANY (ARRAY['last_activity'::text, 'created'::text])))",
			"CHECK ((kind = ANY (ARRAY['text'::text, 'forum'::text, 'dm'::text])))",
			"FOREIGN KEY (created_by) REFERENCES principals(id) ON DELETE RESTRICT",
			"FOREIGN KEY (operator_agent_id) REFERENCES agents(principal_id) ON DELETE RESTRICT",
			"FOREIGN KEY (space_id) REFERENCES spaces(id) ON DELETE RESTRICT",
			"PRIMARY KEY (id)",
		}, []string{
			"CREATE INDEX channels_operator_agent_id_idx ON public.channels USING btree (operator_agent_id) WHERE (operator_agent_id IS NOT NULL)",
			"CREATE INDEX channels_space_sort_key_idx ON public.channels USING btree (space_id, sort_key) WHERE (archived_at IS NULL)",
			"CREATE UNIQUE INDEX channels_pkey ON public.channels USING btree (id)",
			"CREATE UNIQUE INDEX channels_space_name_active_key ON public.channels USING btree (space_id, lower(name)) WHERE ((archived_at IS NULL) AND (kind <> 'dm'::text))",
		}},
		{"channel_skill_bindings", []principalColumn{
			{"channel_id", "uuid", false, ""},
			{"position", "integer", false, ""},
			{"skill_ref", "text", false, ""},
		}, []string{
			"FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE",
			"PRIMARY KEY (channel_id, \"position\")",
			"UNIQUE (channel_id, skill_ref)",
		}, []string{
			"CREATE UNIQUE INDEX channel_skill_bindings_channel_id_skill_ref_key ON public.channel_skill_bindings USING btree (channel_id, skill_ref)",
			"CREATE UNIQUE INDEX channel_skill_bindings_pkey ON public.channel_skill_bindings USING btree (channel_id, \"position\")",
		}},
		{"forum_tags", []principalColumn{
			{"id", "uuid", false, ""},
			{"channel_id", "uuid", false, ""},
			{"name", "text", false, ""},
			{"color", "text", true, ""},
			{"sort_key", "text", false, ""},
			{"restricted", "boolean", false, "false"},
			{"deleted_at", ts, true, ""},
		}, []string{
			"FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE",
			"PRIMARY KEY (id)",
		}, []string{
			"CREATE UNIQUE INDEX forum_tags_channel_name_active_key ON public.forum_tags USING btree (channel_id, lower(name)) WHERE (deleted_at IS NULL)",
			"CREATE UNIQUE INDEX forum_tags_pkey ON public.forum_tags USING btree (id)",
		}},
		{"threads", []principalColumn{
			{"id", "uuid", false, ""},
			{"channel_id", "uuid", false, ""},
			{"kind", "text", false, ""},
			{"parent_message_id", "uuid", true, ""},
			{"title", "text", false, "''::text"},
			{"title_source", "text", false, "'auto'::text"},
			{"state", "text", false, "'open'::text"},
			{"closed_at", ts, true, ""},
			{"closed_by", "uuid", true, ""},
			{"operator_agent_id", "uuid", true, ""},
			{"last_message_seq", "bigint", false, "0"},
			{"unread_floor_seq", "bigint", false, "0"},
			{"last_activity_at", ts, false, "now()"},
			{"message_count", "integer", false, "0"},
			{"created_by", "uuid", false, ""},
			{"created_at", ts, false, "now()"},
			{"moved_at", ts, true, ""},
			{"version", "bigint", false, "1"},
		}, []string{
			"CHECK (((kind <> 'main'::text) OR ((parent_message_id IS NULL) AND (operator_agent_id IS NULL))))",
			"CHECK (((state = 'closed'::text) = (closed_at IS NOT NULL)))",
			"CHECK ((kind = ANY (ARRAY['main'::text, 'thread'::text])))",
			"CHECK ((state = ANY (ARRAY['open'::text, 'closed'::text])))",
			"CHECK ((title_source = ANY (ARRAY['auto'::text, 'manual'::text])))",
			"FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE RESTRICT",
			"FOREIGN KEY (closed_by) REFERENCES principals(id) ON DELETE RESTRICT",
			"FOREIGN KEY (created_by) REFERENCES principals(id) ON DELETE RESTRICT",
			"FOREIGN KEY (operator_agent_id) REFERENCES agents(principal_id) ON DELETE RESTRICT",
			// Added by 00032 once messages exists (M1-56).
			"FOREIGN KEY (parent_message_id) REFERENCES messages(id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED",
			"PRIMARY KEY (id)",
		}, []string{
			"CREATE INDEX threads_channel_state_activity_idx ON public.threads USING btree (channel_id, state, last_activity_at DESC)",
			"CREATE INDEX threads_operator_agent_channel_idx ON public.threads USING btree (operator_agent_id, channel_id) WHERE (operator_agent_id IS NOT NULL)",
			"CREATE UNIQUE INDEX threads_channel_main_key ON public.threads USING btree (channel_id) WHERE (kind = 'main'::text)",
			"CREATE UNIQUE INDEX threads_parent_message_id_key ON public.threads USING btree (parent_message_id) WHERE (parent_message_id IS NOT NULL)",
			"CREATE UNIQUE INDEX threads_pkey ON public.threads USING btree (id)",
		}},
	} {
		if got := principalColumns(t, conn, c.table); !reflect.DeepEqual(got, c.columns) {
			t.Errorf("%s columns:\n got %+v\nwant %+v", c.table, got, c.columns)
		}
		slices.Sort(c.constraints)
		cons := principalConstraints(t, conn, c.table)
		slices.Sort(cons)
		if !reflect.DeepEqual(cons, c.constraints) {
			t.Errorf("%s constraints:\n got %q\nwant %q", c.table, cons, c.constraints)
		}
		slices.Sort(c.idx)
		got := indexes(t, conn, c.table)
		slices.Sort(got)
		if !reflect.DeepEqual(got, c.idx) {
			t.Errorf("%s indexes:\n got %q\nwant %q", c.table, got, c.idx)
		}
	}
	for _, table := range []string{"channels", "forum_tags"} {
		var collation string
		if err := conn.QueryRow(context.Background(), `SELECT coalesce(collation_name, '') FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1 AND column_name = 'sort_key'`, table).Scan(&collation); err != nil || collation != "C" {
			t.Errorf("%s.sort_key collation %q, %v", table, collation, err)
		}
	}
	var def string
	var valid bool
	if err := conn.QueryRow(context.Background(), `SELECT pg_get_constraintdef(oid), convalidated FROM pg_constraint
		WHERE conrelid = 'grants'::regclass AND conname = 'grants_target_channel_id_fkey'`).Scan(&def, &valid); err != nil ||
		def != "FOREIGN KEY (target_channel_id) REFERENCES channels(id) ON DELETE RESTRICT" || !valid {
		t.Errorf("grants_target_channel_id_fkey %q validated %v, %v", def, valid, err)
	}
}

// TestChatChannelsMigration_Constraints exercises the rules as bothy_app: channel names are
// unique case-insensitively per space among channels that are not archived, a dm channel has no
// space, a text channel has at most one main thread (R-DM-11), a main thread has no operator, tag
// names are unique among tags that are not deleted, and bothy_app deletes no channel.
func TestChatChannelsMigration_Constraints(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	const person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5301"
	const space = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5311"
	const c1 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5321"
	const c2 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5322"
	const c3 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5323"
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, app, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space, person)
	ins := `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by) VALUES ($1, $2, $3, $4, $5, 20, $6)`
	mustExec(t, app, ins, c1, space, "text", "tickets", "a0", person)
	wantState(t, exec(app, ins, c2, space, "forum", "Tickets", "a1", person), "23505", "a channel name taken case-insensitively")
	wantState(t, exec(app, ins, c2, nil, "text", "lost", "a1", person), "23514", "a text channel without a space")
	wantState(t, exec(app, ins, c2, space, "dm", "", "a1", person), "23514", "a dm channel in a space")
	mustExec(t, app, `UPDATE channels SET archived_at = now(), version = version + 1 WHERE id = $1`, c1)
	mustExec(t, app, ins, c2, space, "forum", "Tickets", "a1", person)
	th := `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, $3, $4)`
	mustExec(t, app, th, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5331", c1, "main", person)
	wantState(t, exec(app, th, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5332", c1, "main", person), "23505", "a second main thread")
	mustExec(t, app, th, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5333", c1, "thread", person)
	wantState(t, exec(app, `INSERT INTO threads (id, channel_id, kind, created_by, state) VALUES ($1, $2, 'thread', $3, 'closed')`,
		"0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5334", c1, person), "23514", "a closed thread without closed_at")
	tag := `INSERT INTO forum_tags (id, channel_id, name, sort_key) VALUES ($1, $2, $3, 'a0')`
	mustExec(t, app, tag, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5341", c2, "bug")
	wantState(t, exec(app, tag, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5342", c2, "BUG"), "23505", "a tag name taken case-insensitively")
	mustExec(t, app, `UPDATE forum_tags SET deleted_at = now() WHERE id = $1`, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5341")
	mustExec(t, app, tag, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5342", c2, "BUG")
	wantState(t, exec(app, `DELETE FROM channels WHERE id = $1`, c2), "42501", "bothy_app deletes a channel")
	wantState(t, exec(app, `INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, target_channel_id, level, capabilities, reason, granted_by)
		VALUES ($1, 'user', $2, 'channel', $3, 'read', '{}', 'fixture', $2)`, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5351", person, c3), "23503", "a grant on an unknown channel")
}

// TestChatChannelsMigration_Down holds the Down sections: down to version 25 removes the four
// tables and the foreign key of grants, up applies again.
func TestChatChannelsMigration_Down(t *testing.T) {
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
	if _, err := provider.DownTo(ctx, 25); err != nil {
		t.Fatalf("down to 25: %v", err)
	}
	var exists bool
	if err := sqlDB.QueryRowContext(ctx, `SELECT to_regclass('channels') IS NOT NULL OR to_regclass('channel_skill_bindings') IS NOT NULL
		OR to_regclass('forum_tags') IS NOT NULL OR to_regclass('threads') IS NOT NULL
		OR EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'grants_target_channel_id_fkey')`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("a table or the foreign key of grants survives the down migration")
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
}
