// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// TestMigrationFilesTables holds files, file_upload_sessions, file_upload_parts and file_thumbnails
// (M3-05) against
// docs/spec/03-datenmodell.md section 4.7 column by column with every key, check and index
// (M3-03), files.search_vector as the stored generated column of section 5 with its GIN index,
// and the deferred foreign key principals.avatar_file_id (M1-26), validated by the second
// migration. A second up changes nothing, and the table check of files refuses a purpose other
// than attachment with a scope channel.
func TestMigrationFilesTables(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	tz := "timestamp with time zone"
	for _, tc := range []struct {
		table string
		cols  []principalColumn
		cons  []string
		idx   []string
	}{
		{
			table: "files",
			cols: []principalColumn{
				{"id", "uuid", false, ""}, {"object_key", "text", false, ""}, {"sha256", "bytea", false, ""},
				{"size_bytes", "bigint", false, ""}, {"mime_type", "text", false, ""}, {"declared_mime_type", "text", true, ""},
				{"original_name", "text", false, ""}, {"uploaded_by", "uuid", false, ""},
				{"purpose", "text", false, "'attachment'::text"}, {"scope_channel_id", "uuid", true, ""},
				{"upload_state", "text", false, "'pending'::text"}, {"scan_status", "text", false, "'pending'::text"},
				{"scan_engine", "text", true, ""}, {"scanned_at", tz, true, ""}, {"width", "integer", true, ""},
				{"height", "integer", true, ""}, {"duration_ms", "bigint", true, ""},
				{"transcript_status", "text", false, "'none'::text"}, {"transcript_text", "text", true, ""},
				{"transcript_language", "text", true, ""}, {"transcript_source", "text", true, ""},
				{"created_at", tz, false, "now()"}, {"deleted_at", tz, true, ""}, {"search_vector", "tsvector", false, ""},
			},
			cons: []string{
				"CHECK (((purpose = 'attachment'::text) = (scope_channel_id IS NOT NULL)))",
				"CHECK ((purpose = ANY (ARRAY['attachment'::text, 'avatar'::text, 'branding'::text, 'gdpr_export'::text])))",
				"CHECK ((scan_status = ANY (ARRAY['pending'::text, 'clean'::text, 'infected'::text, 'error'::text, 'skipped'::text])))",
				"CHECK ((transcript_source = ANY (ARRAY['agent'::text, 'service'::text])))",
				"CHECK ((transcript_status = ANY (ARRAY['none'::text, 'pending'::text, 'done'::text, 'failed'::text])))",
				"CHECK ((upload_state = ANY (ARRAY['pending'::text, 'complete'::text, 'failed'::text])))",
				"FOREIGN KEY (scope_channel_id) REFERENCES channels(id) ON DELETE RESTRICT",
				"FOREIGN KEY (uploaded_by) REFERENCES principals(id) ON DELETE RESTRICT",
				"PRIMARY KEY (id)",
				"UNIQUE (object_key)",
			},
			idx: []string{
				"CREATE INDEX files_scan_status_pending_idx ON public.files USING btree (scan_status) WHERE (scan_status = 'pending'::text)",
				"CREATE INDEX files_search_vector_idx ON public.files USING gin (search_vector)",
				"CREATE INDEX files_upload_state_pending_idx ON public.files USING btree (upload_state, created_at) WHERE (upload_state = 'pending'::text)",
				"CREATE INDEX files_uploaded_by_created_at_idx ON public.files USING btree (uploaded_by, created_at DESC)",
				"CREATE UNIQUE INDEX files_object_key_key ON public.files USING btree (object_key)",
				"CREATE UNIQUE INDEX files_pkey ON public.files USING btree (id)",
			},
		},
		{
			table: "file_upload_sessions",
			cols: []principalColumn{
				{"file_id", "uuid", false, ""}, {"mode", "text", false, "'direct'::text"}, {"state", "text", false, "'open'::text"},
				{"declared_size_bytes", "bigint", false, ""}, {"declared_sha256", "bytea", false, ""},
				{"part_size_bytes", "bigint", false, ""}, {"part_count", "integer", false, ""},
				{"storage_upload_id", "text", true, ""}, {"failure_reason", "text", true, ""},
				{"created_at", tz, false, "now()"}, {"completed_at", tz, true, ""}, {"expires_at", tz, false, ""},
			},
			cons: []string{
				"CHECK ((mode = ANY (ARRAY['direct'::text, 'presigned'::text])))",
				"CHECK ((state = ANY (ARRAY['open'::text, 'completed'::text, 'aborted'::text, 'expired'::text])))",
				"FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE",
				"PRIMARY KEY (file_id)",
			},
			idx: []string{
				"CREATE INDEX file_upload_sessions_open_expires_at_idx ON public.file_upload_sessions USING btree (expires_at) WHERE (state = 'open'::text)",
				"CREATE UNIQUE INDEX file_upload_sessions_pkey ON public.file_upload_sessions USING btree (file_id)",
			},
		},
		{
			table: "file_upload_parts",
			cols: []principalColumn{
				{"file_id", "uuid", false, ""}, {"part_number", "integer", false, ""}, {"size_bytes", "bigint", false, ""},
				{"sha256", "bytea", false, ""}, {"etag", "text", true, ""}, {"received_at", tz, false, "now()"},
			},
			cons: []string{
				"FOREIGN KEY (file_id) REFERENCES file_upload_sessions(file_id) ON DELETE CASCADE",
				"PRIMARY KEY (file_id, part_number)",
			},
			idx: []string{
				"CREATE UNIQUE INDEX file_upload_parts_pkey ON public.file_upload_parts USING btree (file_id, part_number)",
			},
		},
		{
			// file_thumbnails (M3-05): the read path of downloadThumbnail; M3-06 writes it.
			table: "file_thumbnails",
			cols: []principalColumn{
				{"file_id", "uuid", false, ""}, {"variant", "text", false, ""}, {"object_key", "text", false, ""},
				{"mime_type", "text", false, ""}, {"width", "integer", false, ""}, {"height", "integer", false, ""},
				{"size_bytes", "bigint", false, ""},
			},
			cons: []string{
				"CHECK ((variant = ANY (ARRAY['small'::text, 'medium'::text, 'large'::text, 'poster'::text])))",
				"FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE",
				"PRIMARY KEY (file_id, variant)",
				"UNIQUE (object_key)",
			},
			idx: []string{
				"CREATE UNIQUE INDEX file_thumbnails_object_key_key ON public.file_thumbnails USING btree (object_key)",
				"CREATE UNIQUE INDEX file_thumbnails_pkey ON public.file_thumbnails USING btree (file_id, variant)",
			},
		},
	} {
		if got := principalColumns(t, conn, tc.table); !reflect.DeepEqual(got, tc.cols) {
			t.Errorf("%s columns:\n got %+v\nwant %+v", tc.table, got, tc.cols)
		}
		cons := principalConstraints(t, conn, tc.table)
		slices.Sort(cons)
		slices.Sort(tc.cons)
		if !reflect.DeepEqual(cons, tc.cons) {
			t.Errorf("%s constraints:\n got %q\nwant %q", tc.table, cons, tc.cons)
		}
		idx := indexes(t, conn, tc.table)
		slices.Sort(idx)
		slices.Sort(tc.idx)
		if got := idx; !reflect.DeepEqual(got, tc.idx) {
			t.Errorf("%s indexes:\n got %q\nwant %q", tc.table, got, tc.idx)
		}
	}

	// search_vector: stored, generated, not null, with the expression of 03 section 5.
	var generated, expr string
	if err := conn.QueryRow(ctx, `SELECT a.attgenerated::text, pg_get_expr(d.adbin, d.adrelid)
		FROM pg_attribute a JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		WHERE a.attrelid = 'files'::regclass AND a.attname = 'search_vector'`).Scan(&generated, &expr); err != nil {
		t.Fatal(err)
	}
	if generated != "s" || !strings.Contains(expr, "original_name") || !strings.Contains(expr, "COALESCE(transcript_text, ''::text)") ||
		!strings.Contains(expr, "'A'::\"char\"") || !strings.Contains(expr, "'B'::\"char\"") {
		t.Errorf("files.search_vector: attgenerated %q, expression %s", generated, expr)
	}

	// The deferred foreign key of principals.avatar_file_id exists and is validated.
	var def string
	var validated bool
	if err := conn.QueryRow(ctx, `SELECT pg_get_constraintdef(oid), convalidated FROM pg_constraint
		WHERE conrelid = 'principals'::regclass AND conname = 'principals_avatar_file_id_fkey'`).Scan(&def, &validated); err != nil {
		t.Fatalf("principals_avatar_file_id_fkey: %v", err)
	}
	if def != "FOREIGN KEY (avatar_file_id) REFERENCES files(id) ON DELETE SET NULL" || !validated {
		t.Errorf("principals_avatar_file_id_fkey = %q, validated %v", def, validated)
	}

	pool := db.OpenPool(t, db.Config(pg.RoleOwner).ForMigrate(5*time.Second))
	sqlDB := pool.OpenSQL()
	t.Cleanup(func() { _ = sqlDB.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if res, err := provider.Up(ctx); err != nil || len(res) != 0 {
		t.Fatalf("second up: %d results, %v", len(res), err)
	}

	// The table check: a purpose other than attachment never has a scope channel.
	for _, stmt := range []string{
		`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ('0192f4a0-7b1c-7000-8a00-000000000001', 'user', 'active', 'alex', 'Alex')`,
		`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ('0192f4a0-7b1c-7000-8a00-000000000002', 'Support', 'a', '0192f4a0-7b1c-7000-8a00-000000000001')`,
		`INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by) VALUES ('0192f4a0-7b1c-7000-8a00-000000000003', '0192f4a0-7b1c-7000-8a00-000000000002', 'text', 'Allgemein', 'a', 20, '0192f4a0-7b1c-7000-8a00-000000000001')`,
	} {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	insert := `INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose, scope_channel_id)
		VALUES (gen_random_uuid(), $1, '\x00', 1, 'application/octet-stream', 'a.txt', '0192f4a0-7b1c-7000-8a00-000000000001', $2, $3)`
	if _, err := conn.Exec(ctx, insert, "k1", "avatar", "0192f4a0-7b1c-7000-8a00-000000000003"); err == nil ||
		!strings.Contains(err.Error(), "files_check") {
		t.Errorf("avatar with a scope channel: %v, want the table check", err)
	}
	if _, err := conn.Exec(ctx, insert, "k2", "attachment", "0192f4a0-7b1c-7000-8a00-000000000003"); err != nil {
		t.Errorf("attachment with a scope channel: %v", err)
	}
}
