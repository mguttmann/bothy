// SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// TestPurgeEach_ShortTransactions: PurgeEach deletes a large batch in several transactions of at
// most 32 candidates, so no transaction holds more savepoints that write than a backend caches
// (64), and every candidate is deleted.
func TestPurgeEach_ShortTransactions(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	ctx := context.Background()
	if _, err := e.db.SuperConn(t).Exec(ctx, "INSERT INTO items SELECT g, 'x' FROM generate_series(100, 199) g"); err != nil {
		t.Fatal(err)
	}
	candidates := make([]int, 0, 100)
	for id := 100; id < 200; id++ {
		candidates = append(candidates, id)
	}
	perTx := map[int64]int{}
	n, err := pg.PurgeEach(ctx, e.pool, candidates, func(ctx context.Context, q pg.DBTX, id int) (int64, error) {
		var xid int64
		if err := q.QueryRow(ctx, "SELECT txid_current()").Scan(&xid); err != nil {
			return 0, err
		}
		perTx[xid]++
		tag, err := q.Exec(ctx, "DELETE FROM items WHERE id = $1", id)
		return tag.RowsAffected(), err
	})
	if err != nil || n != 100 {
		t.Fatalf("PurgeEach = %d, %v, want 100 rows", n, err)
	}
	if len(perTx) < 4 {
		t.Errorf("%d transactions for 100 candidates, want at least 4", len(perTx))
	}
	for xid, c := range perTx {
		if c > 32 {
			t.Errorf("transaction %d deleted %d candidates, want at most 32", xid, c)
		}
	}
	for _, id := range []int{100, 150, 199} {
		if e.rowExists(t, id) {
			t.Errorf("row %d is still there", id)
		}
	}
}
