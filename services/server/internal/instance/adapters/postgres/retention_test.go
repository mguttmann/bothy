// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestPurgeOnlyExpired (M7-05, DP-T05): the participant of instance deletes only retired signing
// keys whose not_after lies before the cutoff; a key at or after the cutoff, a key in another
// state and a retired key of purpose audit_checkpoint stay.
func TestPurgeOnlyExpired(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	super := db.SuperConn(t)
	cutoff := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	key := func(kid, purpose, state string, notAfter time.Time) {
		t.Helper()
		if _, err := super.Exec(ctx, `INSERT INTO server_signing_keys (kid, purpose, alg, public_jwk, private_key_ref, state, not_after)
			VALUES ($1, $2, 'EdDSA', '{}', 'file:/run/keys/'||$1, $3, $4)`, kid, purpose, state, notAfter); err != nil {
			t.Fatal(err)
		}
	}
	key("gone", "access_token", "retired", cutoff.Add(-time.Second))
	key("at", "access_token", "retired", cutoff)
	key("after", "access_token", "retired", cutoff.Add(time.Second))
	key("retiring", "invocation", "retiring", cutoff.Add(-time.Hour))
	key("audit", "audit_checkpoint", "retired", cutoff.Add(-time.Hour))
	n, more, err := postgres.NewRepository(db.Pool(t, pg.RoleApp)).Retention().Purge(ctx, cutoff, 10)
	if err != nil || n != 1 || more {
		t.Fatalf("Purge = %d, %v, %v; want 1, false, nil", n, more, err)
	}
	rows, err := super.Query(ctx, "SELECT kid FROM server_signing_keys ORDER BY kid")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var kid string
		if err := rows.Scan(&kid); err != nil {
			t.Fatal(err)
		}
		got = append(got, kid)
	}
	if want := "[after at audit retiring]"; fmt.Sprint(got) != want {
		t.Fatalf("keys left %v, want %s", got, want)
	}
}
