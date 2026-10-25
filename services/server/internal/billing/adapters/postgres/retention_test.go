// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/billing/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestPurgeOnlyExpired (M7-05, DP-T05): the participant of billing deletes cost entries that
// occurred before the cutoff and price rows closed before it; an entry at or after the cutoff, an
// open price row and one closed at the cutoff stay, and an expired entry whose kept correction
// names it stays with it while an expired correction chain goes completely.
func TestPurgeOnlyExpired(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	super := db.SuperConn(t)
	cutoff := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	before := cutoff.Add(-time.Second)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := super.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	// The seed skips the foreign keys to runs, agents, threads and channels, which this
	// participant never touches; the self reference supersedes_id is checked at the delete.
	exec(`SET session_replication_role = replica`)
	entry := func(at time.Time, supersedes *ids.ID) ids.ID {
		id := ids.New()
		exec(`INSERT INTO cost_entries (id, run_id, agent_id, thread_id, channel_id, billing_mode, provider, model, amount_micros,
			currency, is_estimate, source, occurred_at, supersedes_id)
			VALUES ($1, $2, $2, $2, $2, 'organization', 'example', 'model-a', 10, 'EUR', true, 'computed', $3, $4)`, id, ids.New(), at, supersedes)
		return id
	}
	price := func(from time.Time, to *time.Time) ids.ID {
		id := ids.New()
		exec(`INSERT INTO price_table (id, provider, model, unit, price_micros, currency, valid_from, valid_to, source_url, retrieved_at, created_by)
			VALUES ($1, 'example', 'model-a', 'input_mtok', 1, 'EUR', $2, $3, 'https://prices.example.com', $2, $1)`, id, from, to)
		return id
	}
	ptr := func(t time.Time) *time.Time { return &t }
	gone := entry(before, nil)
	chainA := entry(before.Add(-time.Hour), nil)
	chainB := entry(before, &chainA)
	heldA := entry(before.Add(-time.Hour), nil)
	heldB := entry(cutoff, &heldA)
	after := entry(cutoff.Add(time.Second), nil)
	oldPrice := price(cutoff.Add(-48*time.Hour), ptr(before))
	edgePrice := price(before, ptr(cutoff))
	openPrice := price(cutoff, nil)
	exec(`SET session_replication_role = origin`)

	p := postgres.NewRetention(db.Pool(t, pg.RoleApp))
	for {
		_, more, err := p.Purge(ctx, cutoff, 1)
		if err != nil {
			t.Fatal(err)
		}
		if !more {
			break
		}
	}
	for _, c := range []struct {
		table string
		id    ids.ID
		want  bool
	}{
		{"cost_entries", gone, false}, {"cost_entries", chainA, false}, {"cost_entries", chainB, false},
		{"cost_entries", heldA, true}, {"cost_entries", heldB, true}, {"cost_entries", after, true},
		{"price_table", oldPrice, false}, {"price_table", edgePrice, true}, {"price_table", openPrice, true},
	} {
		var ok bool
		if err := super.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM "+c.table+" WHERE id = $1)", c.id).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		if ok != c.want {
			t.Errorf("%s row present = %v, want %v", c.table, ok, c.want)
		}
	}
}
