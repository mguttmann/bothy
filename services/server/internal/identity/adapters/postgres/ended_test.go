// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestEndedSessions_RevokedAndExpiredAscending covers app.SessionStore.EndedSessions against
// PostgreSQL 18 (M2-46 design note 8 c): of a list of sessions it returns the revoked ones and
// those whose expires_at is not after at, ascending; an active session, one expiring after at and
// an unknown id are absent. Inside a unit of work it reads that unit's uncommitted revocation.
func TestEndedSessions_RevokedAndExpiredAscending(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	alex := e.mustAddUser(t, "alex", "alex@example.com")
	phone := deviceOf(t, ids.New(), alex.Principal.ID)
	start := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	at := start.Add(time.Hour)
	var active, revoked, expiredAtAt, expiresAfter ids.ID
	if err := e.write(t, func(ctx context.Context) error {
		if _, err := e.repo.InsertDevice(ctx, phone); err != nil {
			return err
		}
		insert := func(expires time.Time) (ids.ID, error) {
			id := ids.New()
			return id, e.repo.InsertSession(ctx, app.SessionRecord{
				ID: id, UserID: alex.Principal.ID, DeviceID: phone.ID, Component: identity.ComponentApp,
				AuthMethod: identity.AuthPasskey, CreatedAt: start, ExpiresAt: expires,
			})
		}
		var err error
		if active, err = insert(at.Add(time.Minute)); err != nil {
			return err
		}
		if revoked, err = insert(at.Add(time.Minute)); err != nil {
			return err
		}
		if expiredAtAt, err = insert(at); err != nil {
			return err
		}
		if expiresAfter, err = insert(at.Add(time.Microsecond)); err != nil {
			return err
		}
		_, err = e.repo.RevokeSessions(ctx, []ids.ID{revoked}, identity.RevokeLogout, start)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	got, err := e.repo.EndedSessions(ctx, []ids.ID{ids.New(), expiresAfter, expiredAtAt, active, revoked}, at)
	if err != nil {
		t.Fatal(err)
	}
	if want := []ids.ID{revoked, expiredAtAt}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ended %v, want %v", got, want)
	}

	if err := e.write(t, func(ctx context.Context) error {
		if _, err := e.repo.RevokeSessions(ctx, []ids.ID{active}, identity.RevokeLogout, start); err != nil {
			return err
		}
		got, err := e.repo.EndedSessions(ctx, []ids.ID{active}, at)
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
