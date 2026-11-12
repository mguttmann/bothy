// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// candidateQuery returns the statement of the sqlc query name in file, with its two named
// arguments replaced by values, so the plan test explains the query the job runs.
func candidateQuery(t *testing.T, file, name string) string {
	t.Helper()
	raw, err := os.ReadFile(file) // #nosec G304 -- a query file of this repository
	if err != nil {
		t.Fatal(err)
	}
	_, rest, found := strings.Cut(string(raw), "-- name: "+name+" :many\n")
	if !found {
		t.Fatalf("%s has no query %s", file, name)
	}
	stmt, _, _ := strings.Cut(rest, "-- name: ")
	var lines []string
	for line := range strings.SplitSeq(stmt, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			lines = append(lines, line)
		}
	}
	stmt = strings.TrimSuffix(strings.TrimSpace(strings.Join(lines, "\n")), ";")
	stmt = strings.ReplaceAll(stmt, "@cutoff", "'2026-01-01T00:00:00Z'::timestamptz")
	stmt = strings.ReplaceAll(stmt, "@row_limit", "500")
	if strings.Contains(stmt, "@") {
		t.Fatalf("%s has an argument the test does not know: %s", name, stmt)
	}
	return stmt
}

// planNodes explains stmt and returns the node types of its plan and the indexes it reads.
func planNodes(t *testing.T, conn *pgx.Conn, stmt string) (nodes, indexes []string) {
	t.Helper()
	var raw []byte
	if err := conn.QueryRow(context.Background(), "EXPLAIN (FORMAT JSON) "+stmt).Scan(&raw); err != nil {
		t.Fatalf("%s: %v", stmt, err)
	}
	var plans []struct {
		Plan map[string]any `json:"Plan"`
	}
	if err := json.Unmarshal(raw, &plans); err != nil || len(plans) != 1 {
		t.Fatalf("plan %s: %v", raw, err)
	}
	var walk func(n map[string]any)
	walk = func(n map[string]any) {
		if s, ok := n["Node Type"].(string); ok {
			nodes = append(nodes, s)
		}
		if s, ok := n["Index Name"].(string); ok {
			indexes = append(indexes, s)
		}
		children, _ := n["Plans"].([]any)
		for _, c := range children {
			if m, ok := c.(map[string]any); ok {
				walk(m)
			}
		}
	}
	walk(plans[0].Plan)
	return nodes, indexes
}

// TestRetentionIndexes (00104, audit finding data-layer-3) holds the plans the retention jobs
// rely on, against the schema of all migrations and the query files the jobs run:
//
//   - every candidate query "rows whose end lies before the cutoff, oldest first" reads its index
//     in order, with no sequential scan and no sort, so a batch costs its rows and not the table;
//   - every foreign key to devices that 00104 names has an index its check can use, also in the
//     generic plan a foreign key trigger caches, so deleting a device reads no child table whole.
//
// The planner may not fall back to a sequential or bitmap scan here: on the empty tables of the
// test it would otherwise prefer one whatever index exists.
func TestRetentionIndexes(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ctx := context.Background()
	for _, set := range []string{"SET enable_seqscan = off", "SET enable_bitmapscan = off", "SET plan_cache_mode = force_generic_plan"} {
		if _, err := conn.Exec(ctx, set); err != nil {
			t.Fatal(err)
		}
	}
	identity := "../internal/identity/adapters/postgres/queries/retention.sql"
	for _, tc := range []struct{ file, query, index string }{
		{identity, "ExpiredRefreshTokens", "refresh_tokens_expires_at_idx"},
		{identity, "ExpiredAuthSessions", "auth_sessions_end_idx"},
		{identity, "ExpiredDevices", "devices_revoked_at_idx"},
		{"../internal/terminal/adapters/postgres/queries/retention.sql", "ExpiredTerminalSessions", "terminal_sessions_closed_at_idx"},
		{"../internal/messaging/adapters/postgres/queries/retention.sql", "ExpiredMessages", "messages_content_created_idx"},
	} {
		nodes, indexes := planNodes(t, conn, candidateQuery(t, tc.file, tc.query))
		if !slices.Contains(indexes, tc.index) || slices.Contains(nodes, "Seq Scan") || slices.Contains(nodes, "Sort") {
			t.Errorf("%s: plan %v over %v, want an ordered read of %s without a sequential scan or a sort", tc.query, nodes, indexes, tc.index)
		}
	}
	for i, tc := range []struct{ table, column, index string }{
		{"read_states", "updated_by_device_id", "read_states_updated_by_device_id_idx"},
		{"drafts", "updated_by_device_id", "drafts_updated_by_device_id_idx"},
		{"user_settings", "updated_by_device_id", "user_settings_updated_by_device_id_idx"},
		{"user_space_states", "updated_by_device_id", "user_space_states_updated_by_device_id_idx"},
		{"user_channel_states", "updated_by_device_id", "user_channel_states_updated_by_device_id_idx"},
		{"push_registrations", "device_id", "push_registrations_device_id_idx"},
	} {
		name := "fk_check_" + string(rune('a'+i))
		// The shape of the check a foreign key trigger runs for a deleted device.
		if _, err := conn.Exec(ctx, "PREPARE "+name+"(uuid) AS SELECT 1 FROM ONLY "+tc.table+" WHERE "+tc.column+" = $1"); err != nil {
			t.Fatal(err)
		}
		nodes, indexes := planNodes(t, conn, "EXECUTE "+name+"('00000000-0000-7000-8000-000000000001')")
		if !slices.Contains(indexes, tc.index) || slices.Contains(nodes, "Seq Scan") {
			t.Errorf("%s.%s: plan %v over %v, want a read of %s", tc.table, tc.column, nodes, indexes, tc.index)
		}
	}
}
