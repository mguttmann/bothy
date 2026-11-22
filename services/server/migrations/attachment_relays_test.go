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

// beforeRelays is the last version before message_attachment_relays (00112).
const beforeRelays = 111

// TestMigrationMessageAttachmentRelays holds message_attachment_relays against
// docs/spec/03-data-model.md section 4.7 (FIX-TEAM-CHANNEL-ATTACHMENTS): every column and type, the
// primary key (message_id, position), UNIQUE (message_id, file_id), both foreign keys with ON
// DELETE CASCADE, the indexes (file_id) and (thread_id), no foreign key on the copies thread_id and
// channel_id. Rows: one file relayed to a copy while its original row stays the only row of
// message_attachments, the same file twice on one copy fails, deleting the copy or the file row
// removes the relay. Down drops the table; a second up changes nothing.
func TestMigrationMessageAttachmentRelays(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	wantCols := []principalColumn{
		{"message_id", "uuid", false, ""}, {"file_id", "uuid", false, ""}, {"position", "smallint", false, ""},
		{"thread_id", "uuid", false, ""}, {"channel_id", "uuid", false, ""},
	}
	if got := principalColumns(t, conn, "message_attachment_relays"); !reflect.DeepEqual(got, wantCols) {
		t.Errorf("message_attachment_relays columns:\n got %+v\nwant %+v", got, wantCols)
	}
	wantCons := []string{
		"FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE",
		"FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE",
		"PRIMARY KEY (message_id, \"position\")",
		"UNIQUE (message_id, file_id)",
	}
	cons := principalConstraints(t, conn, "message_attachment_relays")
	slices.Sort(cons)
	if !reflect.DeepEqual(cons, wantCons) {
		t.Errorf("message_attachment_relays constraints:\n got %q\nwant %q", cons, wantCons)
	}
	wantIdx := []string{
		"CREATE INDEX message_attachment_relays_file_id_idx ON public.message_attachment_relays USING btree (file_id)",
		"CREATE INDEX message_attachment_relays_thread_id_idx ON public.message_attachment_relays USING btree (thread_id)",
		"CREATE UNIQUE INDEX message_attachment_relays_message_id_file_id_key ON public.message_attachment_relays USING btree (message_id, file_id)",
		"CREATE UNIQUE INDEX message_attachment_relays_pkey ON public.message_attachment_relays USING btree (message_id, \"position\")",
	}
	idx := indexes(t, conn, "message_attachment_relays")
	slices.Sort(idx)
	if !reflect.DeepEqual(idx, wantIdx) {
		t.Errorf("message_attachment_relays indexes:\n got %q\nwant %q", idx, wantIdx)
	}

	for _, stmt := range attachmentFixture {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	const copyMessage = "0192f4a0-7b1c-7000-8a00-0000000000b5"
	if _, err := conn.Exec(ctx, `INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id,
		client_payload_sha256, origin, content_md) VALUES ($1, $2, $3, 2, $4, gen_random_uuid(), '\x00', 'human', 'copy')`,
		copyMessage, attThread, attChannel, attAlex); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO message_attachments (message_id, file_id, position, thread_id, channel_id)
		VALUES ($1, $2, 1, $3, $4)`, attMessage, attFile1, attThread, attChannel); err != nil {
		t.Fatalf("original row: %v", err)
	}
	relay := `INSERT INTO message_attachment_relays (message_id, file_id, position, thread_id, channel_id) VALUES ($1, $2, $3, $4, $5)`
	// The copies have no foreign key: ids of no thread and no channel are accepted.
	if _, err := conn.Exec(ctx, relay, copyMessage, attFile1, 1, "0192f4a0-7b1c-7000-8a00-0000000000ff", "0192f4a0-7b1c-7000-8a00-0000000000fe"); err != nil {
		t.Fatalf("relay of an attached file: %v", err)
	}
	if _, err := conn.Exec(ctx, relay, copyMessage, attFile1, 2, attThread, attChannel); err == nil ||
		!strings.Contains(err.Error(), "message_attachment_relays_message_id_file_id_key") {
		t.Errorf("the same file twice on one copy: %v, want the unique key", err)
	}
	count := func(table string) int {
		t.Helper()
		var n int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count("message_attachments"); n != 1 {
		t.Errorf("message_attachments rows with a relay: %d, want the one original", n)
	}
	if _, err := conn.Exec(ctx, `DELETE FROM messages WHERE id = $1`, copyMessage); err != nil {
		t.Fatal(err)
	}
	if n := count("message_attachment_relays"); n != 0 {
		t.Errorf("relays after the deletion of the copy: %d", n)
	}
	if _, err := conn.Exec(ctx, relay, attMessage, attFile2, 2, attThread, attChannel); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `DELETE FROM files WHERE id = $1`, attFile2); err != nil {
		t.Fatalf("deleting a relayed file row: %v", err)
	}
	if n := count("message_attachment_relays"); n != 0 {
		t.Errorf("relays after the deletion of the file row: %d", n)
	}

	provider := attachmentProvider(t, db)
	if _, err := provider.DownTo(ctx, beforeRelays); err != nil {
		t.Fatalf("down to %d: %v", beforeRelays, err)
	}
	var exists bool
	if err := conn.QueryRow(ctx, `SELECT to_regclass('message_attachment_relays') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Errorf("table after down: %v, %v", exists, err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}
	if res, err := provider.Up(ctx); err != nil || len(res) != 0 {
		t.Fatalf("second up: %d results, %v", len(res), err)
	}
}
