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

// TestMigrations_ImporterTables holds import_sources and import_items against
// docs/spec/03-datenmodell.md section 4.11 column by column, with the checks, the keys, the
// indexes and the validated foreign key messages.import_item_id (M8-03).
func TestMigrations_ImporterTables(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ts := "timestamp with time zone"
	sources := []principalColumn{
		{"id", "uuid", false, ""},
		{"kind", "text", false, ""},
		{"name", "text", false, ""},
		{"config", "jsonb", false, ""},
		{"state", "text", false, "'draft'::text"},
		{"cursor", "jsonb", false, "'{}'::jsonb"},
		{"stats", "jsonb", false, "'{}'::jsonb"},
		{"created_by", "uuid", false, ""},
		{"created_at", ts, false, "now()"},
		{"started_at", ts, true, ""},
		{"finished_at", ts, true, ""},
	}
	items := []principalColumn{
		{"id", "uuid", false, ""},
		{"source_id", "uuid", false, ""},
		{"origin_key", "text", false, ""},
		{"item_kind", "text", false, ""},
		{"target_kind", "text", true, ""},
		{"target_id", "uuid", true, ""},
		{"state", "text", false, "'pending'::text"},
		{"content_sha256", "bytea", true, ""},
		{"error", "text", true, ""},
		{"created_at", ts, false, "now()"},
		{"updated_at", ts, false, "now()"},
	}
	if got := principalColumns(t, conn, "import_sources"); !reflect.DeepEqual(got, sources) {
		t.Errorf("import_sources columns:\n got %+v\nwant %+v", got, sources)
	}
	if got := principalColumns(t, conn, "import_items"); !reflect.DeepEqual(got, items) {
		t.Errorf("import_items columns:\n got %+v\nwant %+v", got, items)
	}
	for table, want := range map[string][]string{
		"import_sources": {
			"CHECK ((kind = ANY (ARRAY['discord'::text, 'archive'::text])))",
			"CHECK ((state = ANY (ARRAY['draft'::text, 'running'::text, 'paused'::text, 'completed'::text, 'failed'::text])))",
			"FOREIGN KEY (created_by) REFERENCES principals(id) ON DELETE RESTRICT",
			"PRIMARY KEY (id)",
		},
		"import_items": {
			"CHECK ((item_kind = ANY (ARRAY['space'::text, 'channel'::text, 'forum_tag'::text, 'thread'::text, 'message'::text, 'file'::text, 'user'::text, 'reaction'::text, 'pin'::text])))",
			"CHECK ((state = ANY (ARRAY['pending'::text, 'posted'::text, 'skipped'::text, 'failed'::text])))",
			"FOREIGN KEY (source_id) REFERENCES import_sources(id) ON DELETE CASCADE",
			"PRIMARY KEY (id)",
			"UNIQUE (source_id, origin_key)",
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
		"import_sources": {
			"CREATE INDEX import_sources_state_idx ON public.import_sources USING btree (state)",
			"CREATE UNIQUE INDEX import_sources_pkey ON public.import_sources USING btree (id)",
		},
		"import_items": {
			"CREATE INDEX import_items_source_state_idx ON public.import_items USING btree (source_id, state)",
			"CREATE INDEX import_items_target_idx ON public.import_items USING btree (target_kind, target_id)",
			"CREATE UNIQUE INDEX import_items_pkey ON public.import_items USING btree (id)",
			"CREATE UNIQUE INDEX import_items_source_id_origin_key_key ON public.import_items USING btree (source_id, origin_key)",
		},
	} {
		if got := indexes(t, conn, table); !reflect.DeepEqual(got, want) {
			t.Errorf("%s indexes:\n got %q\nwant %q", table, got, want)
		}
	}
	var def string
	var validated bool
	if err := conn.QueryRow(context.Background(), `SELECT pg_get_constraintdef(oid), convalidated FROM pg_constraint
		WHERE conname = 'messages_import_item_id_fkey'`).Scan(&def, &validated); err != nil {
		t.Fatalf("messages_import_item_id_fkey: %v", err)
	}
	if def != "FOREIGN KEY (import_item_id) REFERENCES import_items(id) ON DELETE SET NULL DEFERRABLE INITIALLY DEFERRED" || !validated {
		t.Errorf("messages import foreign key %q, validated %v", def, validated)
	}
}

// TestMigration_ImporterDown removes both tables and the foreign key of messages and applies them
// again.
func TestMigration_ImporterDown(t *testing.T) {
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
	if _, err := provider.DownTo(ctx, 59); err != nil {
		t.Fatalf("down to 59: %v", err)
	}
	var left int
	if err := sqlDB.QueryRowContext(ctx, `SELECT (to_regclass('import_sources') IS NOT NULL)::int + (to_regclass('import_items') IS NOT NULL)::int
		+ (SELECT count(*)::int FROM pg_constraint WHERE conname = 'messages_import_item_id_fkey')`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d of import_sources, import_items and messages_import_item_id_fkey left after down", left)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
}
