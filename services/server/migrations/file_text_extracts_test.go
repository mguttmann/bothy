// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestMigrationFileTextExtracts holds file_text_extracts against docs/spec/03-data-model.md
// section 4.7 column by column with every key, check and index (M3-15), search_vector as the
// stored generated column of section 5, the state check, and the cascade from files.
func TestMigrationFileTextExtracts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	cols := []principalColumn{
		{"file_id", "uuid", false, ""}, {"state", "text", false, "'pending'::text"}, {"extractor", "text", true, ""},
		{"text_content", "text", true, ""}, {"truncated", "boolean", false, "false"},
		{"search_language", "regconfig", false, "'simple'::regconfig"}, {"extracted_at", "timestamp with time zone", true, ""},
		{"error", "text", true, ""}, {"search_vector", "tsvector", false, ""},
	}
	if got := principalColumns(t, conn, "file_text_extracts"); !reflect.DeepEqual(got, cols) {
		t.Errorf("file_text_extracts columns:\n got %+v\nwant %+v", got, cols)
	}
	wantCons := []string{
		"CHECK ((state = ANY (ARRAY['pending'::text, 'done'::text, 'failed'::text, 'unsupported'::text])))",
		"FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE",
		"PRIMARY KEY (file_id)",
	}
	cons := principalConstraints(t, conn, "file_text_extracts")
	slices.Sort(cons)
	if !reflect.DeepEqual(cons, wantCons) {
		t.Errorf("file_text_extracts constraints:\n got %q\nwant %q", cons, wantCons)
	}
	wantIdx := []string{
		"CREATE INDEX file_text_extracts_pending_idx ON public.file_text_extracts USING btree (state) WHERE (state = 'pending'::text)",
		"CREATE INDEX file_text_extracts_search_vector_idx ON public.file_text_extracts USING gin (search_vector)",
		"CREATE UNIQUE INDEX file_text_extracts_pkey ON public.file_text_extracts USING btree (file_id)",
	}
	idx := indexes(t, conn, "file_text_extracts")
	slices.Sort(idx)
	if !reflect.DeepEqual(idx, wantIdx) {
		t.Errorf("file_text_extracts indexes:\n got %q\nwant %q", idx, wantIdx)
	}

	var generated, expr string
	if err := conn.QueryRow(ctx, `SELECT a.attgenerated::text, pg_get_expr(d.adbin, d.adrelid)
		FROM pg_attribute a JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		WHERE a.attrelid = 'file_text_extracts'::regclass AND a.attname = 'search_vector'`).Scan(&generated, &expr); err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"to_tsvector(search_language, COALESCE(text_content, ''::text))", "'C'::\"char\"",
		"to_tsvector('simple'::regconfig, COALESCE(text_content, ''::text))", "'D'::\"char\""} {
		if generated != "s" || !strings.Contains(expr, part) {
			t.Errorf("file_text_extracts.search_vector: attgenerated %q, expression %s lacks %s", generated, expr, part)
		}
	}

	for _, stmt := range []string{
		`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ('0192f4a0-7b1c-7000-8a00-000000000001', 'user', 'active', 'alex', 'Alex')`,
		`INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose)
			VALUES ('0192f4a0-7b1c-7000-8a00-000000000009', 'k', '\x00', 1, 'text/plain', 'a.txt', '0192f4a0-7b1c-7000-8a00-000000000001', 'avatar')`,
	} {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if _, err := conn.Exec(ctx, `INSERT INTO file_text_extracts (file_id, state) VALUES ('0192f4a0-7b1c-7000-8a00-000000000009', 'running')`); err == nil ||
		!strings.Contains(err.Error(), "file_text_extracts_state_check") {
		t.Errorf("state running: %v, want the state check", err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO file_text_extracts (file_id, state, text_content, search_language)
		VALUES ('0192f4a0-7b1c-7000-8a00-000000000009', 'done', 'Rechnungen', 'german')`); err != nil {
		t.Fatal(err)
	}
	var match bool
	if err := conn.QueryRow(ctx, `SELECT search_vector @@ to_tsquery('german', 'Rechnung') FROM file_text_extracts`).Scan(&match); err != nil || !match {
		t.Errorf("stem match: %v, %v", match, err)
	}
	if _, err := conn.Exec(ctx, `DELETE FROM files WHERE id = '0192f4a0-7b1c-7000-8a00-000000000009'`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM file_text_extracts`).Scan(&n); err != nil || n != 0 {
		t.Errorf("rows after the deletion of the file: %d, %v", n, err)
	}
}
