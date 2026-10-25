// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/apps/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestPurgeOnlyExpired (M7-05, DP-T05): the participant of apps deletes finished deliveries whose
// end lies before the cutoff: delivered_at for a delivered row, created_at for a dead one; a row
// ending at or after the cutoff and a pending row stay.
func TestPurgeOnlyExpired(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	super := db.SuperConn(t)
	cutoff := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	before, old := cutoff.Add(-time.Second), cutoff.Add(-72*time.Hour)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := super.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	agent, sub := ids.New(), ids.New()
	exec(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'agent', 'active', 'helper', 'Helper')`, agent)
	exec(`INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'generic')`, agent)
	exec(`INSERT INTO server_signing_keys (kid, purpose, alg, public_jwk, private_key_ref) VALUES ('wh', 'webhook_out', 'EdDSA', '{}', 'file:/run/keys/wh')`)
	exec(`INSERT INTO webhook_subscriptions (id, agent_id, url, event_types, signing_key_kid, created_by)
		VALUES ($1, $2, 'https://hooks.example.com/in', '{message.created}', 'wh', $2)`, sub, agent)
	seq := int64(0)
	delivery := func(status string, created time.Time, delivered *time.Time) ids.ID {
		id := ids.New()
		seq++
		exec(`INSERT INTO webhook_deliveries (id, subscription_id, event_seq, event_id, attempt, status, created_at, delivered_at)
			VALUES ($1, $2, $3, $4, 1, $5, $6, $7)`, id, sub, seq, ids.New(), status, created, delivered)
		return id
	}
	ptr := func(t time.Time) *time.Time { return &t }
	cases := map[ids.ID]bool{
		delivery("succeeded", old, ptr(before)): false,
		delivery("dead", before, nil):           false,
		delivery("succeeded", old, ptr(cutoff)): true,
		delivery("dead", cutoff, nil):           true,
		delivery("pending", old, nil):           true,
	}
	n, more, err := postgres.NewRetention(db.Pool(t, pg.RoleApp)).Purge(ctx, cutoff, 10)
	if err != nil || n != 2 || more {
		t.Fatalf("Purge = %d, %v, %v; want 2, false, nil", n, more, err)
	}
	for id, want := range cases {
		var ok bool
		if err := super.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM webhook_deliveries WHERE id = $1)", id).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		if ok != want {
			t.Errorf("delivery present = %v, want %v", ok, want)
		}
	}
}
