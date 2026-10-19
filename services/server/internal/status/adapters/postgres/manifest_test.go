// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"bytes"
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/status/adapters/postgres"
)

// mustExec runs stmt as superuser or fails the test.
func mustExec(t *testing.T, conn *pgx.Conn, stmt string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(context.Background(), stmt, args...); err != nil {
		t.Fatalf("%s: %v", stmt, err)
	}
}

// seed fills the instance row, three messages in one thread, three files (one deleted), three
// audit rows and five events (seq 3 to 7), with valid foreign keys. The ids are EXAMPLE values.
func seed(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	const (
		person  = "0192f4a0-7b1c-7000-8a00-000000000101"
		space   = "0192f4a0-7b1c-7000-8a00-000000000102"
		channel = "0192f4a0-7b1c-7000-8a00-000000000103"
		thread  = "0192f4a0-7b1c-7000-8a00-000000000104"
	)
	mustExec(t, conn, `INSERT INTO instance (instance_id, schema_version, sync_epoch)
		VALUES ('0192f4a0-7b1c-7000-8a00-0000000000a1', 49, '0192f4a0-7b1c-7000-8a00-0000000000a2')`)
	mustExec(t, conn, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'robin', 'Robin')`, person)
	mustExec(t, conn, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, conn, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space, person)
	mustExec(t, conn, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'text', 'general', 'a0', 20, $3)`, channel, space, person)
	mustExec(t, conn, `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`, thread, channel, person)
	for i := 1; i <= 3; i++ {
		mustExec(t, conn, `INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id,
			client_payload_sha256, origin, kind, content_md) VALUES (gen_random_uuid(), $1, $2, $3, $4, gen_random_uuid(), '\x00', 'human', 'text', 'hello')`,
			thread, channel, i, person)
	}
	for i, size := range []int64{100, 250, 4000} {
		mustExec(t, conn, `INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose, deleted_at)
			VALUES (gen_random_uuid(), $1, '\x00', $2, 'image/png', 'a.png', $3, 'avatar', CASE WHEN $4 THEN now() END)`,
			"objects/"+string(rune('a'+i)), size, person, size == 4000)
	}
	for seq := 1; seq <= 3; seq++ {
		mustExec(t, conn, `INSERT INTO audit_events (seq, id, action, decision, prev_hash, hash)
			VALUES ($1::bigint, gen_random_uuid(), 'grant.create', 'allow', '\x00'::bytea, sha256($1::text::bytea))`, seq)
	}
	mustExec(t, conn, `SELECT sync_event_log_ensure_partitions(1000, 1, 7)`)
	for seq := 3; seq <= 7; seq++ {
		mustExec(t, conn, `INSERT INTO event_log (seq, id, type, resource_kind, receive_action, payload, permission_version)
			VALUES ($1::bigint, gen_random_uuid(), 'thread.closed', 'thread', 'thread.read', '{}'::jsonb, 0)`, seq)
	}
}

// TestManifestCounts holds brief M7-02 design note 3: after seeding messages, files and audit
// rows, the snapshot of the manifest equals direct queries: count(*) per ordinary and partitioned
// table of schema public (partitions counted through their parent), the bounds of event_log, the
// last audit row, the instance row and the totals of the files that are not deleted.
func TestManifestCounts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	super := db.SuperConn(t)
	seed(t, super)

	src := postgres.NewManifestSource(db.OpenPool(t, db.SuperConfig()))
	facts, err := src.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}

	rows, err := super.Query(ctx, `SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p') AND NOT c.relispartition ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if got := slices.Sorted(maps.Keys(facts.RowCounts)); !slices.Equal(got, tables) {
		t.Errorf("tables of the manifest:\n got %q\nwant %q", got, tables)
	}
	if slices.ContainsFunc(tables, func(s string) bool { return len(s) > 15 && s[:15] == "event_log_from_" }) {
		t.Error("a partition of event_log is counted on its own")
	}
	for _, table := range tables {
		var n int64
		if err := super.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if facts.RowCounts[table] != n {
			t.Errorf("row_counts[%s] = %d, want %d", table, facts.RowCounts[table], n)
		}
	}
	for table, want := range map[string]int64{"messages": 3, "files": 3, "audit_events": 3, "event_log": 5, "instance": 1} {
		if facts.RowCounts[table] != want {
			t.Errorf("row_counts[%s] = %d, want %d seeded", table, facts.RowCounts[table], want)
		}
	}

	var hash []byte
	if err := super.QueryRow(ctx, `SELECT hash FROM audit_events WHERE seq = 3`).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if facts.HeadSeq != 7 || facts.MinRetainedSeq != 3 || facts.AuditLastSeq != 3 || !bytes.Equal(facts.AuditLastHash, hash) ||
		facts.FileCount != 2 || facts.FileBytes != 350 || facts.SchemaVersion != 49 ||
		facts.InstanceID != "0192f4a0-7b1c-7000-8a00-0000000000a1" || facts.SyncEpoch != "0192f4a0-7b1c-7000-8a00-0000000000a2" {
		t.Errorf("facts %+v", facts)
	}

	// The test cluster archives no WAL: no time yet, and no error.
	if at, ok, err := src.LastArchived(ctx); err != nil || ok || !at.IsZero() {
		t.Errorf("LastArchived = %v, %v, %v; want nothing archived", at, ok, err)
	}
}

// TestManifestEmptyLogs holds the facts of a fresh instance: empty event and audit logs give
// zeros and no hash; a database without the instance row fails.
func TestManifestEmptyLogs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	super := db.SuperConn(t)
	src := postgres.NewManifestSource(db.OpenPool(t, db.SuperConfig()))
	if _, err := src.Snapshot(ctx); err == nil {
		t.Error("Snapshot without the instance row succeeded")
	}
	mustExec(t, super, `INSERT INTO instance (instance_id, schema_version, sync_epoch)
		VALUES ('0192f4a0-7b1c-7000-8a00-0000000000a1', 49, '0192f4a0-7b1c-7000-8a00-0000000000a2')`)
	facts, err := src.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if facts.HeadSeq != 0 || facts.MinRetainedSeq != 0 || facts.AuditLastSeq != 0 || facts.AuditLastHash != nil ||
		facts.FileCount != 0 || facts.FileBytes != 0 || facts.RowCounts["instance"] != 1 {
		t.Errorf("facts %+v", facts)
	}
}
