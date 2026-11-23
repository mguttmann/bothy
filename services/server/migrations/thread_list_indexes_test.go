// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestThreadListIndexes holds the plans of listChannelThreads (00120 and 00121, audit finding
// data-layer-11): a page sorted by created or by activity, without a state filter, reads the
// threads of its channel in the keyset order of an index and sorts nothing, so a page does not
// grow with the forum. The seed spreads the threads over channels (foreign keys off); sequential
// and bitmap scans are off, as in TestHotPathIndexes.
func TestThreadListIndexes(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ctx := context.Background()
	for _, stmt := range []string{
		"SET session_replication_role = replica",
		`INSERT INTO threads (id, channel_id, kind, created_by, created_at, last_activity_at, search_language)
		SELECT gen_random_uuid(), ('00000000-0000-7000-8000-' || lpad((n % 50)::text, 12, '0'))::uuid, 'thread', gen_random_uuid(),
			'2026-01-01Z'::timestamptz + n * interval '1 minute', '2026-01-01Z'::timestamptz + (n % 997) * interval '1 minute', 'simple'
		FROM generate_series(1, 20000) n`,
		"SET session_replication_role = DEFAULT",
		"ANALYZE threads",
		"SET enable_seqscan = off",
		"SET enable_bitmapscan = off",
	} {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	threads := "../internal/chat/adapters/postgres/queries/threads.sql"
	args := map[string]string{
		"@channel_id": "'00000000-0000-7000-8000-000000000007'::uuid", "sqlc.narg(state)": "NULL", "sqlc.narg(tag_id)": "NULL",
		"@has_after": "false", "@after_at": "'2026-01-01Z'", "@after_id": "'00000000-0000-7000-8000-000000000001'", "@max_rows": "50",
	}
	for _, tc := range []struct{ query, index string }{
		{"ListThreadsByCreated", "threads_channel_created_idx"},
		{"ListThreadsByActivity", "threads_channel_activity_idx"},
	} {
		nodes, indexes := planNodes(t, conn, namedQuery(t, threads, tc.query, ":many", args))
		if !slices.Contains(indexes, tc.index) || slices.Contains(nodes, "Seq Scan") || slices.Contains(nodes, "Sort") {
			t.Errorf("%s: plan %v over %v, want a read of %s without a scan or a sort", tc.query, nodes, indexes, tc.index)
		}
	}
}
