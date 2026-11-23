// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestHotPathIndexes holds the plans of queries that run per delivery or per sweep, against the
// schema of all migrations and the query files: the head and the pending count of a webhook
// subscription read only its pending rows (00114, audit finding srv-apps-billing-8), and the
// sweep authz.expiry_sweep reads only the grants and memberships it has not yet reported (00117
// and 00118, audit finding srv-authz-11), so neither grows with the history of the table.
//
// The planner may not fall back to a sequential or bitmap scan here: on the empty tables of the
// test it would otherwise prefer one whatever index exists. The grants and memberships carry a
// history of reported expiries (foreign keys off for the seed), so an index over every active row
// costs what it costs on a long-running instance.
func TestHotPathIndexes(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ctx := context.Background()
	for _, stmt := range []string{
		"SET session_replication_role = replica",
		`INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, level, expires_at,
			expiry_reported_at, reason, granted_by)
		SELECT gen_random_uuid(), 'user', gen_random_uuid(), 'instance', 'read', '2025-01-01Z'::timestamptz,
			CASE WHEN n % 1000 = 0 THEN NULL ELSE '2025-01-02Z'::timestamptz END, 'seed', gen_random_uuid()
		FROM generate_series(1, 20000) n`,
		`INSERT INTO group_memberships (id, group_id, user_id, added_by, expires_at, expiry_reported_at)
		SELECT gen_random_uuid(), gen_random_uuid(), gen_random_uuid(), gen_random_uuid(), '2025-01-01Z'::timestamptz,
			CASE WHEN n % 1000 = 0 THEN NULL ELSE '2025-01-02Z'::timestamptz END
		FROM generate_series(1, 20000) n`,
		"SET session_replication_role = DEFAULT",
		"ANALYZE grants",
		"ANALYZE group_memberships",
		"SET enable_seqscan = off",
		"SET enable_bitmapscan = off",
	} {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	webhooks := "../internal/apps/adapters/postgres/queries/webhooks.sql"
	expiry := "../internal/authz/adapters/postgres/queries/expiry.sql"
	sub := map[string]string{"@subscription_id": "'00000000-0000-7000-8000-000000000001'::uuid", "@bound": "100"}
	due := map[string]string{"@at": "'2026-01-01T00:00:00Z'::timestamptz", "@max_rows": "100",
		"sqlc.embed(g)": "g.*", "sqlc.embed(m)": "m.*"}
	for _, tc := range []struct {
		file, query, kind string
		args              map[string]string
		index             string
	}{
		{webhooks, "HeadDelivery", ":one", sub, "webhook_deliveries_pending_seq_idx"},
		{webhooks, "CountPending", ":one", sub, "webhook_deliveries_pending_seq_idx"},
		{expiry, "ListDueExpiredGrants", ":many", due, "grants_expiry_due_idx"},
		{expiry, "ListDueExpiredMemberships", ":many", due, "group_memberships_expiry_due_idx"},
	} {
		nodes, indexes := planNodes(t, conn, namedQuery(t, tc.file, tc.query, tc.kind, tc.args))
		if !slices.Contains(indexes, tc.index) || slices.Contains(nodes, "Seq Scan") {
			t.Errorf("%s: plan %v over %v, want a read of %s without a sequential scan", tc.query, nodes, indexes, tc.index)
		}
	}
}
