// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/postgres"
)

// TestPurgeOnlyExpired (M7-05, DP-T05): the participant of terminal deletes terminal sessions
// closed before the cutoff, and the session users and the host of a host retired before it; a
// session closed at the cutoff, an open session, a host retired at the cutoff and an active host
// stay, and so does the session user an unexpired session still holds.
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
	robin, dev, sess := ids.New(), ids.New(), ids.New()
	exec(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', 'robin', 'Robin')`, robin)
	exec(`INSERT INTO users (principal_id) VALUES ($1)`, robin)
	exec(`INSERT INTO devices (id, user_id, name, platform, state) VALUES ($1, $2, 'Laptop', 'macos', 'active')`, dev, robin)
	exec(`INSERT INTO auth_sessions (id, user_id, device_id, auth_method, expires_at) VALUES ($1, $2, $3, 'passkey', $4)`, sess, robin, dev, cutoff.Add(time.Hour))
	host := func(name string, retiredAt *time.Time) ids.ID {
		id := ids.New()
		state := "pairing"
		code := []byte(name)
		if retiredAt != nil {
			state, code = "retired", nil
		}
		exec(`INSERT INTO hosts (id, name, state, llm_credentials, pairing_code_hash, created_by, retired_at) VALUES ($1, $2, $3, 'none', $4, $5, $6)`,
			id, name, state, code, robin, retiredAt)
		return id
	}
	hsu := func(h ids.ID) ids.ID {
		id := ids.New()
		exec(`INSERT INTO host_session_users (id, host_id, user_id, os_user, created_by) VALUES ($1, $2, $3, 'robin', $3)`, id, h, robin)
		return id
	}
	tsess := func(h, u ids.ID, closedAt *time.Time) ids.ID {
		id := ids.New()
		state, reason := "active", (*string)(nil)
		if closedAt != nil {
			r := "client_closed"
			state, reason = "closed", &r
		}
		exec(`INSERT INTO terminal_sessions (id, host_id, user_id, host_session_user_id, opened_by_device_id, device_id, auth_session_id,
			profile, state, token_jti, key_generation, opened_at, closed_at, close_reason)
			VALUES ($1, $2, $3, $4, $5, $5, $6, 'shell', $7, $8, 1, $9, $10, $11)`,
			id, h, robin, u, dev, sess, state, ids.New(), cutoff.Add(-48*time.Hour), closedAt, reason)
		return id
	}
	ptr := func(t time.Time) *time.Time { return &t }

	gone := host("old", ptr(before))
	goneUser := hsu(gone)
	goneSess := tsess(gone, goneUser, ptr(before))
	held := host("held", ptr(before))
	heldUser := hsu(held)
	heldSess := tsess(held, heldUser, ptr(cutoff))
	edge := host("edge", ptr(cutoff))
	edgeUser := hsu(edge)
	live := host("live", nil)
	liveUser := hsu(live)
	openSess := tsess(live, liveUser, nil)

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
		{"terminal_sessions", goneSess, false}, {"host_session_users", goneUser, false}, {"hosts", gone, false},
		{"terminal_sessions", heldSess, true}, {"host_session_users", heldUser, true}, {"hosts", held, true},
		{"host_session_users", edgeUser, true}, {"hosts", edge, true},
		{"terminal_sessions", openSess, true}, {"host_session_users", liveUser, true}, {"hosts", live, true},
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
