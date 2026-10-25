// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestPurgeOnlyExpired (M7-05, DP-T05): the security participant of identity deletes exactly the
// rows whose end time lies before the cutoff: a row ended just before goes, one ended at the
// cutoff and one ended after stay, rows without an end time stay, the earlier of expiry and
// revocation counts, and a revoked device that an active session still holds stays (the foreign
// key decides, not a read of the other table).
func TestPurgeOnlyExpired(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	super := db.SuperConn(t)
	cutoff := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	before, after := cutoff.Add(-time.Second), cutoff.Add(time.Second)
	far := cutoff.Add(365 * 24 * time.Hour)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := super.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	alex := ids.New()
	exec(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', 'alex', 'Alex')`, alex)
	exec(`INSERT INTO users (principal_id) VALUES ($1)`, alex)
	device := func(revokedAt *time.Time) ids.ID {
		id := ids.New()
		state := "active"
		if revokedAt != nil {
			state = "revoked"
		}
		exec(`INSERT INTO devices (id, user_id, name, platform, state, revoked_at) VALUES ($1, $2, 'Phone', 'ios', $3, $4)`, id, alex, state, revokedAt)
		return id
	}
	session := func(dev ids.ID, expires time.Time, revokedAt *time.Time) ids.ID {
		id := ids.New()
		exec(`INSERT INTO auth_sessions (id, user_id, device_id, auth_method, expires_at, revoked_at) VALUES ($1, $2, $3, 'passkey', $4, $5)`,
			id, alex, dev, expires, revokedAt)
		return id
	}
	token := func(expires time.Time, revokedAt *time.Time) ids.ID {
		id := ids.New()
		exec(`INSERT INTO api_tokens (id, principal_id, name, token_prefix, token_hash, scopes, expires_at, created_by, revoked_at, revoked_by)
			VALUES ($1, $2, 'ci', 'bothy_pat_EXAMPL', $3, '{}', $4, $2, $5, CASE WHEN $5::timestamptz IS NULL THEN NULL ELSE $2::uuid END)`,
			id, alex, []byte(id.String()), expires, revokedAt)
		return id
	}
	ptr := func(t time.Time) *time.Time { return &t }

	devGone, devAt, devAfter, devActive := device(ptr(before)), device(ptr(cutoff)), device(ptr(after)), device(nil)
	devHeld := device(ptr(before))
	// A completed pairing keeps its device: ON DELETE SET NULL would break its check (23514), so
	// the device stays and the run goes on (M7-05 review).
	devPaired := device(ptr(before))
	exec(`INSERT INTO device_pairings (id, pairing_code_hash, device_registration, status, user_id, device_id, expires_at, completed_at)
		VALUES ($1, '\x01', '{}', 'completed', $2, $3, $4, $4)`, ids.New(), alex, devPaired, before)
	sessGone := session(devGone, before, nil)
	sessRevokedEarly := session(devActive, far, ptr(before))
	sessAt := session(devActive, cutoff, nil)
	sessAfter := session(devActive, after, nil)
	sessHolding := session(devHeld, far, nil)
	tokGone, tokAt, tokRevokedAfter := token(before, nil), token(cutoff, nil), token(far, ptr(after))

	p := postgres.NewRetention(db.Pool(t, pg.RoleApp))
	total := 0
	for {
		n, more, err := p.Purge(ctx, cutoff, 2)
		if err != nil {
			t.Fatalf("%+v", errors.Unwrap(err))
		}
		total += n
		if !more {
			break
		}
	}
	exists := func(table string, id ids.ID) bool {
		t.Helper()
		var ok bool
		if err := super.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM "+pgx.Identifier{table}.Sanitize()+" WHERE id = $1)", id).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		return ok
	}
	for _, c := range []struct {
		table string
		id    ids.ID
		want  bool
	}{
		{"auth_sessions", sessGone, false}, {"auth_sessions", sessRevokedEarly, false},
		{"auth_sessions", sessAt, true}, {"auth_sessions", sessAfter, true}, {"auth_sessions", sessHolding, true},
		{"devices", devGone, false}, {"devices", devAt, true}, {"devices", devAfter, true},
		{"devices", devActive, true}, {"devices", devHeld, true}, {"devices", devPaired, true},
		{"api_tokens", tokGone, false}, {"api_tokens", tokAt, true}, {"api_tokens", tokRevokedAfter, true},
	} {
		if got := exists(c.table, c.id); got != c.want {
			t.Errorf("%s row present = %v, want %v", c.table, got, c.want)
		}
	}
	if total != 4 {
		t.Errorf("Purge reported %d rows, want 4", total)
	}
}

// TestDeleteClosedPairings (M7-05, 03 device_pairings, class transient): identity.auth_cleanup
// deletes the completed, rejected and expired pairings past expires_at; an open pairing and a
// closed one not yet past expires_at stay.
func TestDeleteClosedPairings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	super := db.SuperConn(t)
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	sam, dev := ids.New(), ids.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', 'sam', 'Sam')`, []any{sam}},
		{`INSERT INTO users (principal_id) VALUES ($1)`, []any{sam}},
		{`INSERT INTO devices (id, user_id, name, platform, state) VALUES ($1, $2, 'Phone', 'ios', 'active')`, []any{dev, sam}},
	} {
		if _, err := super.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	pairing := func(status string, expires time.Time) ids.ID {
		id := ids.New()
		var user, device any
		if status == "completed" {
			user, device = sam, dev
		}
		if _, err := super.Exec(ctx, `INSERT INTO device_pairings (id, pairing_code_hash, device_registration, status, user_id, device_id, expires_at)
			VALUES ($1, $2, '{}', $3, $4, $5, $6)`, id, []byte(id.String()), status, user, device, expires); err != nil {
			t.Fatal(err)
		}
		return id
	}
	before := now.Add(-time.Second)
	gone := []ids.ID{pairing("completed", before), pairing("rejected", before), pairing("expired", before)}
	kept := []ids.ID{pairing("requested", before), pairing("completed", now), pairing("expired", now.Add(time.Second))}
	pool := db.Pool(t, pg.RoleApp)
	reg := txn.NewRegistry()
	reg.Seal()
	var n int64
	if err := pg.NewUnitOfWork(pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second,
		IdleInTxTimeout: 60 * time.Second}, reg).Do(ctx, func(ctx context.Context) error {
		var err error
		n, err = postgres.NewRepository(pool).DeleteClosedPairings(ctx, now)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("deleted %d pairings, want 3", n)
	}
	for _, list := range []struct {
		ids  []ids.ID
		want bool
	}{{gone, false}, {kept, true}} {
		for _, id := range list.ids {
			var ok bool
			if err := super.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM device_pairings WHERE id = $1)", id).Scan(&ok); err != nil {
				t.Fatal(err)
			}
			if ok != list.want {
				t.Errorf("pairing present = %v, want %v", ok, list.want)
			}
		}
	}
}
