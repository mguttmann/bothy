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

// TestMessagingMigration_Schema holds messages and mentions against docs/spec/03-data-model.md
// section 4.6 column by column, with every check, key and index, and with exactly the deviations
// the migration names: the search columns come with 00035 and 00036 (M3-13), has_attachments and its CHECK with 00065 and
// 00066 (M3-07); the foreign key to runs comes with 00051 (M2-05), the one to import_items with 00059 (M8-03).
// threads.parent_message_id references messages, deferred.
func TestMessagingMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	tz := "timestamp with time zone"
	want := []principalColumn{
		{"id", "uuid", false, ""},
		{"thread_id", "uuid", false, ""},
		{"channel_id", "uuid", false, ""},
		{"thread_seq", "bigint", false, ""},
		{"author_principal_id", "uuid", false, ""},
		{"client_msg_id", "uuid", false, ""},
		{"client_payload_sha256", "bytea", false, ""},
		{"origin", "text", false, ""},
		{"kind", "text", false, "'text'::text"},
		{"system_code", "text", true, ""},
		{"content_md", "text", false, "''::text"},
		{"content_format", "smallint", false, "1"},
		{"reply_to_message_id", "uuid", true, ""},
		{"run_id", "uuid", true, ""},
		{"trigger_state", "text", true, ""},
		{"trigger_state_reason", "text", true, ""},
		{"trigger_state_at", tz, true, ""},
		{"silent", "boolean", false, "false"},
		{"import_item_id", "uuid", true, ""},
		{"version", "bigint", false, "1"},
		{"edited_at", tz, true, ""},
		{"deleted_at", tz, true, ""},
		{"deleted_by", "uuid", true, ""},
		{"created_at", tz, false, "now()"},
		// Added by 00035 (M3-13); the vector is a stored generated column without a default.
		{"search_language", "regconfig", false, "'simple'::regconfig"},
		{"search_vector", "tsvector", false, ""},
		// Added by 00065 (M3-07).
		{"has_attachments", "boolean", false, "false"},
	}
	if got := principalColumns(t, conn, "messages"); !reflect.DeepEqual(got, want) {
		t.Errorf("messages columns:\n got %+v\nwant %+v", got, want)
	}
	cons := principalConstraints(t, conn, "messages")
	slices.Sort(cons)
	wantCons := []string{
		"CHECK (((deleted_at IS NULL) OR (content_md = ''::text)))",
		// Added by 00065 and validated by 00066 (M3-07).
		"CHECK (((deleted_at IS NULL) OR (NOT has_attachments)))",
		"CHECK (((kind <> 'system'::text) OR ((origin = 'system'::text) AND silent AND (content_md = ''::text))))",
		"CHECK (((kind = 'run'::text) = (run_id IS NOT NULL)))",
		"CHECK (((kind = 'system'::text) = (system_code IS NOT NULL)))",
		"CHECK (((trigger_state IS NULL) OR (origin = 'human'::text)))",
		"CHECK (((trigger_state_reason IS NULL) OR (trigger_state = ANY (ARRAY['failed'::text, 'cancelled'::text]))))",
		"CHECK ((kind = ANY (ARRAY['text'::text, 'voice'::text, 'run'::text, 'card'::text, 'system'::text])))",
		"CHECK ((origin = ANY (ARRAY['human'::text, 'agent'::text, 'app'::text, 'import'::text, 'system'::text])))",
		"CHECK ((system_code = ANY (ARRAY['thread.reopened'::text, 'agent_session.resume_failed'::text])))",
		"CHECK ((trigger_state = ANY (ARRAY['received'::text, 'queued'::text, 'working'::text, 'done'::text, 'failed'::text, 'cancelled'::text, 'followup_delivered'::text])))",
		"FOREIGN KEY (author_principal_id) REFERENCES principals(id) ON DELETE RESTRICT",
		"FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE RESTRICT",
		"FOREIGN KEY (deleted_by) REFERENCES principals(id) ON DELETE RESTRICT",
		// Added by 00059 and validated by 00060 (M8-03).
		"FOREIGN KEY (import_item_id) REFERENCES import_items(id) ON DELETE SET NULL DEFERRABLE INITIALLY DEFERRED",
		"FOREIGN KEY (reply_to_message_id) REFERENCES messages(id) ON DELETE SET NULL",
		// Added by 00051 and validated by 00053 (M2-05).
		"FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE SET NULL DEFERRABLE INITIALLY DEFERRED",
		"FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE RESTRICT",
		"PRIMARY KEY (id)",
	}
	slices.Sort(wantCons)
	if !reflect.DeepEqual(cons, wantCons) {
		t.Errorf("messages constraints:\n got %q\nwant %q", cons, wantCons)
	}
	idx := indexes(t, conn, "messages")
	wantIdx := []string{
		"CREATE INDEX messages_author_created_idx ON public.messages USING btree (author_principal_id, created_at DESC)",
		"CREATE INDEX messages_channel_created_idx ON public.messages USING btree (channel_id, created_at DESC)",
		// Added by 00036 (M3-13).
		"CREATE INDEX messages_search_vector_idx ON public.messages USING gin (search_vector)",
		"CREATE INDEX messages_trigger_state_idx ON public.messages USING btree (trigger_state) WHERE (trigger_state = ANY (ARRAY['received'::text, 'queued'::text, 'working'::text]))",
		"CREATE UNIQUE INDEX messages_author_client_msg_id_key ON public.messages USING btree (author_principal_id, client_msg_id)",
		"CREATE UNIQUE INDEX messages_pkey ON public.messages USING btree (id)",
		"CREATE UNIQUE INDEX messages_run_id_key ON public.messages USING btree (run_id) WHERE (run_id IS NOT NULL)",
		"CREATE UNIQUE INDEX messages_thread_seq_key ON public.messages USING btree (thread_id, thread_seq)",
	}
	if !reflect.DeepEqual(idx, wantIdx) {
		t.Errorf("messages indexes:\n got %q\nwant %q", idx, wantIdx)
	}

	wantMentions := []principalColumn{
		{"message_id", "uuid", false, ""},
		{"principal_id", "uuid", false, ""},
		{"thread_id", "uuid", false, ""},
		{"created_at", tz, false, "now()"},
	}
	if got := principalColumns(t, conn, "mentions"); !reflect.DeepEqual(got, wantMentions) {
		t.Errorf("mentions columns:\n got %+v\nwant %+v", got, wantMentions)
	}
	mcons := principalConstraints(t, conn, "mentions")
	slices.Sort(mcons)
	wantMCons := []string{
		"FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE",
		"FOREIGN KEY (principal_id) REFERENCES principals(id) ON DELETE RESTRICT",
		"FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE",
		"PRIMARY KEY (message_id, principal_id)",
	}
	if !reflect.DeepEqual(mcons, wantMCons) {
		t.Errorf("mentions constraints:\n got %q\nwant %q", mcons, wantMCons)
	}
	midx := indexes(t, conn, "mentions")
	wantMIdx := []string{
		"CREATE INDEX mentions_principal_created_idx ON public.mentions USING btree (principal_id, created_at DESC)",
		"CREATE INDEX mentions_principal_thread_idx ON public.mentions USING btree (principal_id, thread_id)",
		"CREATE UNIQUE INDEX mentions_pkey ON public.mentions USING btree (message_id, principal_id)",
	}
	if !reflect.DeepEqual(midx, wantMIdx) {
		t.Errorf("mentions indexes:\n got %q\nwant %q", midx, wantMIdx)
	}

	var def string
	var validated bool
	if err := conn.QueryRow(context.Background(), `SELECT pg_get_constraintdef(oid), convalidated FROM pg_constraint
		WHERE conname = 'threads_parent_message_id_fkey'`).Scan(&def, &validated); err != nil {
		t.Fatal(err)
	}
	if def != "FOREIGN KEY (parent_message_id) REFERENCES messages(id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED" || !validated {
		t.Errorf("threads parent foreign key %q, validated %v", def, validated)
	}
}

// TestMessagingMigration_Constraints: as bothy_app, the unique keys of 04 section 5.1 and of the
// thread sequence, the system line rule and the deferred parent key hold.
func TestMessagingMigration_Constraints(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	const person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5501"
	const space = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5511"
	const channel = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5521"
	const main = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5531"
	const msg = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5541"
	const client = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5551"
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, app, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space, person)
	mustExec(t, app, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'text', 'general', 'a0', 20, $3)`, channel, space, person)
	mustExec(t, app, `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`, main, channel, person)
	ins := `INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id,
		client_payload_sha256, origin, kind, system_code, content_md, silent) VALUES ($1, $2, $3, $4, $5, $6, '\x00', $7, $8, $9, $10, $11)`
	mustExec(t, app, ins, msg, main, channel, 1, person, client, "human", "text", nil, "hello", false)
	wantState(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5542", main, channel, 2, person, client, "human", "text", nil, "x", false),
		"23505", "the same client_msg_id of the author twice")
	wantState(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5543", main, channel, 1, person,
		"0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5552", "human", "text", nil, "x", false), "23505", "the same thread_seq twice")
	wantCheckViolation(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5544", main, channel, 2, person,
		"0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5553", "human", "system", "thread.reopened", "", true), "a system line of origin human")
	wantCheckViolation(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5545", main, channel, 2, person,
		"0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5554", "system", "system", "other.code", "", true), "an unknown system code")
	mustExec(t, app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5546", main, channel, 2, person,
		"0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5555", "system", "system", "thread.reopened", "", true)
	mustExec(t, app, `INSERT INTO mentions (message_id, principal_id, thread_id) VALUES ($1, $2, $3)`, msg, person, main)
	mustExec(t, app, `DELETE FROM mentions WHERE message_id = $1`, msg)
	// The parent key is deferred: a thread may name its parent before the commit checks it.
	wantState(t, exec(app, `INSERT INTO threads (id, channel_id, kind, parent_message_id, created_by)
		VALUES ('0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5532', $1, 'thread', '0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f55ff', $2)`, channel, person),
		"23503", "a thread with an unknown parent message")
	mustExec(t, app, `INSERT INTO threads (id, channel_id, kind, parent_message_id, created_by)
		VALUES ('0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5533', $1, 'thread', $2, $3)`, channel, msg, person)
}

// TestMessagingMigration_Down: down to version 31 removes both tables and the parent key, up
// applies again.
func TestMessagingMigration_Down(t *testing.T) {
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
	if _, err := provider.DownTo(ctx, 31); err != nil {
		t.Fatalf("down to 31: %v", err)
	}
	var left int
	if err := sqlDB.QueryRowContext(ctx, `SELECT (to_regclass('messages') IS NOT NULL)::int + (to_regclass('mentions') IS NOT NULL)::int
		+ (SELECT count(*)::int FROM pg_constraint WHERE conname = 'threads_parent_message_id_fkey')`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d objects survive the down migration", left)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
}
