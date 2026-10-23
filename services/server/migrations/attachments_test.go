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

// The fixture ids of the attachment tests: Alex, the space Support, the channel Allgemein, its
// main thread, a message and two files.
const (
	attAlex    = "0192f4a0-7b1c-7000-8a00-0000000000a1"
	attSpace   = "0192f4a0-7b1c-7000-8a00-0000000000a2"
	attChannel = "0192f4a0-7b1c-7000-8a00-0000000000a3"
	attThread  = "0192f4a0-7b1c-7000-8a00-0000000000a4"
	attMessage = "0192f4a0-7b1c-7000-8a00-0000000000a5"
	attFile1   = "0192f4a0-7b1c-7000-8a00-0000000000a6"
	attFile2   = "0192f4a0-7b1c-7000-8a00-0000000000a7"
)

// attachmentFixture is the chain of rows a message_attachments row needs: person, space,
// channel, thread, one message and two attachment files of the channel.
var attachmentFixture = []string{
	`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ('` + attAlex + `', 'user', 'active', 'alex', 'Alex')`,
	`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ('` + attSpace + `', 'Support', 'a', '` + attAlex + `')`,
	`INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ('` + attChannel + `', '` + attSpace + `', 'text', 'Allgemein', 'a', 20, '` + attAlex + `')`,
	`INSERT INTO threads (id, channel_id, kind, created_by) VALUES ('` + attThread + `', '` + attChannel + `', 'thread', '` + attAlex + `')`,
	`INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, content_md)
		VALUES ('` + attMessage + `', '` + attThread + `', '` + attChannel + `', 1, '` + attAlex + `', gen_random_uuid(), '\x00', 'human', 'hello')`,
	`INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose, scope_channel_id)
		VALUES ('` + attFile1 + `', 'k1', '\x00', 1, 'text/plain', 'a.txt', '` + attAlex + `', 'attachment', '` + attChannel + `')`,
	`INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose, scope_channel_id)
		VALUES ('` + attFile2 + `', 'k2', '\x00', 1, 'text/plain', 'b.txt', '` + attAlex + `', 'attachment', '` + attChannel + `')`,
}

// TestMigrationMessageAttachments holds message_attachments against docs/spec/03-data-model.md
// section 4.7 (M3-07, R-DM-43): every column and type, the primary key (message_id, position),
// both foreign keys with their ON DELETE, UNIQUE (file_id), the index (thread_id), no foreign key
// on the copies thread_id and channel_id, and nothing 03 does not name. Rows: a second row of
// the same file fails the unique key, the copies accept ids that exist nowhere, a file with a row
// cannot be deleted, and deleting the message removes its rows. A second up changes nothing.
func TestMigrationMessageAttachments(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	wantCols := []principalColumn{
		{"message_id", "uuid", false, ""}, {"file_id", "uuid", false, ""}, {"position", "smallint", false, ""},
		{"thread_id", "uuid", false, ""}, {"channel_id", "uuid", false, ""},
	}
	if got := principalColumns(t, conn, "message_attachments"); !reflect.DeepEqual(got, wantCols) {
		t.Errorf("message_attachments columns:\n got %+v\nwant %+v", got, wantCols)
	}
	wantCons := []string{
		"FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE RESTRICT",
		"FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE",
		"PRIMARY KEY (message_id, \"position\")",
		"UNIQUE (file_id)",
	}
	cons := principalConstraints(t, conn, "message_attachments")
	slices.Sort(cons)
	if !reflect.DeepEqual(cons, wantCons) {
		t.Errorf("message_attachments constraints:\n got %q\nwant %q", cons, wantCons)
	}
	for _, c := range cons {
		if strings.Contains(c, "(thread_id)") || strings.Contains(c, "(channel_id)") {
			t.Errorf("a constraint on a copy column (R-DM-43): %s", c)
		}
	}
	wantIdx := []string{
		"CREATE INDEX message_attachments_thread_id_idx ON public.message_attachments USING btree (thread_id)",
		"CREATE UNIQUE INDEX message_attachments_file_id_key ON public.message_attachments USING btree (file_id)",
		"CREATE UNIQUE INDEX message_attachments_pkey ON public.message_attachments USING btree (message_id, \"position\")",
	}
	idx := indexes(t, conn, "message_attachments")
	slices.Sort(idx)
	if !reflect.DeepEqual(idx, wantIdx) {
		t.Errorf("message_attachments indexes:\n got %q\nwant %q", idx, wantIdx)
	}

	for _, stmt := range attachmentFixture {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	insert := `INSERT INTO message_attachments (message_id, file_id, position, thread_id, channel_id) VALUES ($1, $2, $3, $4, $5)`
	if _, err := conn.Exec(ctx, insert, attMessage, attFile1, 1, attThread, attChannel); err != nil {
		t.Fatalf("first row: %v", err)
	}
	if _, err := conn.Exec(ctx, insert, attMessage, attFile1, 2, attThread, attChannel); err == nil ||
		!strings.Contains(err.Error(), "message_attachments_file_id_key") {
		t.Errorf("the same file a second time: %v, want the unique key", err)
	}
	// The copies have no foreign key: ids of no thread and no channel are accepted.
	if _, err := conn.Exec(ctx, insert, attMessage, attFile2, 2, "0192f4a0-7b1c-7000-8a00-0000000000ff", "0192f4a0-7b1c-7000-8a00-0000000000fe"); err != nil {
		t.Errorf("copies without a foreign key: %v", err)
	}
	if _, err := conn.Exec(ctx, `DELETE FROM files WHERE id = $1`, attFile1); err == nil ||
		!strings.Contains(err.Error(), "message_attachments_file_id_fkey") {
		t.Errorf("deleting an attached file: %v, want ON DELETE RESTRICT", err)
	}
	if _, err := conn.Exec(ctx, `DELETE FROM messages WHERE id = $1`, attMessage); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM message_attachments`).Scan(&n); err != nil || n != 0 {
		t.Errorf("rows after the deletion of the message: %d, %v", n, err)
	}

	provider := attachmentProvider(t, db)
	if res, err := provider.Up(ctx); err != nil || len(res) != 0 {
		t.Fatalf("second up: %d results, %v", len(res), err)
	}
}

// TestMigrationHasAttachments holds messages.has_attachments (03 section 4.6, M3-07): boolean,
// not null, default false; the CHECK deleted_at IS NULL OR NOT has_attachments is created NOT
// VALID by 00067 (docs/operations/upgrade.md rule M5) and validated by 00068. Existing rows get
// false; a deleted message with the flag fails the check. A second up changes nothing.
func TestMigrationHasAttachments(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t, pgtest.WithoutMigrations())
	conn := db.SuperConn(t)
	provider := attachmentProvider(t, db)
	if _, err := provider.UpTo(ctx, 66); err != nil {
		t.Fatalf("up to 66: %v", err)
	}
	// A message from before the column.
	for _, stmt := range attachmentFixture[:5] {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if _, err := provider.UpTo(ctx, 67); err != nil {
		t.Fatalf("up to 67: %v", err)
	}
	const checkName = "messages_has_attachments_deleted_check"
	validated := func() (string, bool) {
		t.Helper()
		var def string
		var ok bool
		if err := conn.QueryRow(ctx, `SELECT pg_get_constraintdef(oid), convalidated FROM pg_constraint
			WHERE conrelid = 'messages'::regclass AND conname = $1`, checkName).Scan(&def, &ok); err != nil {
			t.Fatalf("%s: %v", checkName, err)
		}
		return def, ok
	}
	if def, ok := validated(); def != "CHECK (((deleted_at IS NULL) OR (NOT has_attachments))) NOT VALID" || ok {
		t.Errorf("after 00067: %q validated %v, want NOT VALID", def, ok)
	}
	var flag bool
	if err := conn.QueryRow(ctx, `SELECT has_attachments FROM messages WHERE id = $1`, attMessage).Scan(&flag); err != nil || flag {
		t.Errorf("an existing message: has_attachments %v, %v; want false", flag, err)
	}
	if _, err := provider.UpTo(ctx, 68); err != nil {
		t.Fatalf("up to 68: %v", err)
	}
	if def, ok := validated(); def != "CHECK (((deleted_at IS NULL) OR (NOT has_attachments)))" || !ok {
		t.Errorf("after 00068: %q validated %v", def, ok)
	}
	var typ, dflt string
	var nullable bool
	if err := conn.QueryRow(ctx, `SELECT data_type, is_nullable = 'YES', coalesce(column_default, '')
		FROM information_schema.columns WHERE table_name = 'messages' AND column_name = 'has_attachments'`).Scan(&typ, &nullable, &dflt); err != nil {
		t.Fatal(err)
	}
	if typ != "boolean" || nullable || dflt != "false" {
		t.Errorf("has_attachments: %s nullable %v default %q", typ, nullable, dflt)
	}
	if _, err := conn.Exec(ctx, `UPDATE messages SET has_attachments = true WHERE id = $1`, attMessage); err != nil {
		t.Fatalf("flag on a live message: %v", err)
	}
	if _, err := conn.Exec(ctx, `UPDATE messages SET deleted_at = now(), content_md = '' WHERE id = $1`, attMessage); err == nil ||
		!strings.Contains(err.Error(), checkName) {
		t.Errorf("deleting a message with the flag: %v, want the check", err)
	}
	if _, err := conn.Exec(ctx, `UPDATE messages SET deleted_at = now(), content_md = '', has_attachments = false WHERE id = $1`, attMessage); err != nil {
		t.Errorf("deleting with the flag reset: %v", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}
	if res, err := provider.Up(ctx); err != nil || len(res) != 0 {
		t.Fatalf("second up: %d results, %v", len(res), err)
	}
}

// attachmentProvider returns a goose provider over db as the owner.
func attachmentProvider(t *testing.T, db *pgtest.DB) *goose.Provider {
	t.Helper()
	pool := db.OpenPool(t, db.Config(pg.RoleOwner).ForMigrate(5*time.Second))
	sqlDB := pool.OpenSQL()
	t.Cleanup(func() { _ = sqlDB.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	return provider
}
