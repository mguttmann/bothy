// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestEndedSessions_RevokedAndExpiredAscending (M2-46 design note 8 c): of a list of sessions the
// read returns the revoked ones and those whose expires_at is not after at, ascending; an active
// session, one expiring after at and an unknown id are absent, and an empty list gives nil. Inside
// a unit of work it sees the revocation that unit has not committed yet.
func TestEndedSessions_RevokedAndExpiredAscending(t *testing.T) {
	t.Parallel()
	w := newPeopleWorld(t)
	cfg := peopleCfg
	cfg.RefreshTokenTTL, cfg.SessionMaxAge, cfg.DevicePendingTTL = time.Hour, 24*time.Hour, 24*time.Hour
	sessions, err := app.NewSessions(cfg, tokenPepper, app.SessionsDeps{
		Store: w.repo, Devices: w.repo, People: w.repo, Tokens: &app.AccessTokens{}, DPoP: noDPoP{}, UoW: w.uow,
		Authorizer: w.az, Audit: w.rec, Events: w.appender, Cursors: httpapi.Cursors{}, Clock: w.clk,
	})
	if err != nil {
		t.Fatal(err)
	}
	alex := w.person("alex", "active", false)
	device := ids.New()
	w.sql(`INSERT INTO devices (id, user_id, name, platform, state, approved_by, approved_at, created_at)
		VALUES ($1, $2, 'Phone', 'ios', 'active', $2, $3, $3)`, device.String(), alex.ID.String(), peopleStart)
	at := peopleStart.Add(time.Hour)
	session := func(expires time.Time, revoked bool) ids.ID {
		id := ids.New()
		var revokedAt, reason any
		if revoked {
			revokedAt, reason = peopleStart, string(identity.RevokeLogout)
		}
		w.sql(`INSERT INTO auth_sessions (id, user_id, device_id, client_component, auth_method, expires_at, revoked_at, revoke_reason)
			VALUES ($1, $2, $3, 'app', 'passkey', $4, $5, $6)`, id.String(), alex.ID.String(), device.String(), expires, revokedAt, reason)
		return id
	}
	active := session(at.Add(time.Minute), false)
	revoked := session(at.Add(time.Minute), true)
	expiredAtAt := session(at, false)
	expiresAfter := session(at.Add(time.Microsecond), false)
	unknown := ids.New()
	ctx := context.Background()

	got, err := sessions.EndedSessions(ctx, []ids.ID{unknown, expiresAfter, expiredAtAt, active, revoked}, at)
	if err != nil {
		t.Fatal(err)
	}
	if want := []ids.ID{revoked, expiredAtAt}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ended %v, want %v", got, want)
	}

	if got, err := sessions.EndedSessions(ctx, nil, at); err != nil || got != nil {
		t.Fatalf("empty list: %v, %v, want nil", got, err)
	}

	if err := w.uow.Do(ctx, func(ctx context.Context) error {
		if err := sessions.RevokeSession(ctx, active, identity.RevokeLogout, nil); err != nil {
			return err
		}
		got, err := sessions.EndedSessions(ctx, []ids.ID{active}, at)
		if err != nil {
			return err
		}
		if want := []ids.ID{active}; !reflect.DeepEqual(got, want) {
			t.Errorf("inside the unit of work: ended %v, want %v", got, want)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
