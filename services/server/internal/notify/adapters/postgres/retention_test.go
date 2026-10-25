// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestPurgeOnlyExpired (M7-05, DP-T05): the participant of notify deletes only registrations
// disabled before the cutoff; one disabled at or after it and an enabled one stay.
func TestPurgeOnlyExpired(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	super := db.SuperConn(t)
	cutoff := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := super.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	sam, dev, sess := ids.New(), ids.New(), ids.New()
	exec(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', 'sam', 'Sam')`, sam)
	exec(`INSERT INTO users (principal_id) VALUES ($1)`, sam)
	exec(`INSERT INTO devices (id, user_id, name, platform, state) VALUES ($1, $2, 'Phone', 'ios', 'active')`, dev, sam)
	exec(`INSERT INTO auth_sessions (id, user_id, device_id, auth_method, expires_at) VALUES ($1, $2, $3, 'passkey', $4)`, sess, sam, dev, cutoff.Add(time.Hour))
	reg := func(token string, disabledAt *time.Time) ids.ID {
		id := ids.New()
		exec(`INSERT INTO push_registrations (id, user_id, device_id, auth_session_id, provider, token, app_id, disabled_at, disabled_reason)
			VALUES ($1, $2, $3, $4, 'apns', $5, 'com.example.app', $6, CASE WHEN $6::timestamptz IS NULL THEN NULL ELSE 'logout' END)`,
			id, sam, dev, sess, token, disabledAt)
		return id
	}
	ptr := func(t time.Time) *time.Time { return &t }
	gone := reg("a", ptr(cutoff.Add(-time.Second)))
	keep := []ids.ID{reg("b", ptr(cutoff)), reg("c", ptr(cutoff.Add(time.Second))), reg("d", nil)}
	n, more, err := postgres.NewRegistrations(db.Pool(t, pg.RoleApp)).Purge(ctx, cutoff, 10)
	if err != nil || n != 1 || more {
		t.Fatalf("Purge = %d, %v, %v; want 1, false, nil", n, more, err)
	}
	for id, want := range map[ids.ID]bool{gone: false, keep[0]: true, keep[1]: true, keep[2]: true} {
		var ok bool
		if err := super.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM push_registrations WHERE id = $1)", id).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		if ok != want {
			t.Errorf("registration present = %v, want %v", ok, want)
		}
	}
}
