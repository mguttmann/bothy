// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestMessageRevisionsMigration_Schema holds message_revisions against docs/spec/03-data-model.md
// (table message_revisions) column by column, with its keys (M1-58).
func TestMessageRevisionsMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	want := []principalColumn{
		{"id", "uuid", false, ""},
		{"message_id", "uuid", false, ""},
		{"revision", "integer", false, ""},
		{"content_md", "text", false, ""},
		{"edited_by", "uuid", false, ""},
		{"created_at", "timestamp with time zone", false, "now()"},
	}
	if got := principalColumns(t, conn, "message_revisions"); !reflect.DeepEqual(got, want) {
		t.Errorf("message_revisions columns:\n got %+v\nwant %+v", got, want)
	}
	wantCons := []string{
		"FOREIGN KEY (edited_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE",
		"PRIMARY KEY (id)",
		"UNIQUE (message_id, revision)",
	}
	if got := principalConstraints(t, conn, "message_revisions"); !reflect.DeepEqual(got, wantCons) {
		t.Errorf("message_revisions constraints:\n got %q\nwant %q", got, wantCons)
	}
	wantIdx := []string{
		"CREATE UNIQUE INDEX message_revisions_message_id_revision_key ON public.message_revisions USING btree (message_id, revision)",
		"CREATE UNIQUE INDEX message_revisions_pkey ON public.message_revisions USING btree (id)",
	}
	if got := indexes(t, conn, "message_revisions"); !reflect.DeepEqual(got, wantIdx) {
		t.Errorf("message_revisions indexes:\n got %q\nwant %q", got, wantIdx)
	}
}

// TestEventLogRedactionGrant (M1-58, R-DM-18): bothy_app may update exactly payload and
// redacted_at of event_log; every envelope column stays append-only.
func TestEventLogRedactionGrant(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	rows, err := conn.Query(context.Background(), `SELECT column_name FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'event_log'
		AND has_column_privilege('bothy_app', 'public.event_log', column_name, 'UPDATE') ORDER BY column_name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		got = append(got, c)
	}
	if want := []string{"payload", "redacted_at"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("bothy_app may update %q of event_log, want %q", got, want)
	}
}
