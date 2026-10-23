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
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestSessionActor_ActiveRevokedExpiredForeignUnknown (M3-05 design note 6): the actor of a
// download ticket carries the flags of the session read per call; it never fails on them, only
// authorize does. A family on another key than the device's dpop key is no active key, a pending
// device past auth.device_pending_ttl counts as revoked, a foreign or unknown session is inactive
// with an unknown device, and there is never a step-up time. KeyThumbprint is the dpop_jkt of the
// session's active family, "" without one (M3-22 design note 8 d).
func TestSessionActor_ActiveRevokedExpiredForeignUnknown(t *testing.T) {
	t.Parallel()
	w := newPeopleWorld(t)
	cfg := peopleCfg
	cfg.RefreshTokenTTL, cfg.SessionMaxAge, cfg.DevicePendingTTL = time.Hour, 24*time.Hour, 24*time.Hour
	cfg.DeviceInactiveRevokeAfter = 180 * 24 * time.Hour
	sessions, err := app.NewSessions(cfg, tokenPepper, app.SessionsDeps{
		Store: w.repo, Devices: w.repo, People: w.repo, Tokens: &app.AccessTokens{}, DPoP: noDPoP{}, UoW: w.uow,
		Authorizer: w.az, Audit: w.rec, Events: w.appender, Cursors: httpapi.Cursors{}, Clock: w.clk,
	})
	if err != nil {
		t.Fatal(err)
	}
	alex := w.person("alex", "active", false)
	sam := w.person("sam", "active", false)
	device := func(state, jkt string, created time.Time) ids.ID {
		id := ids.New()
		var approvedBy, approvedAt any
		if state == "active" {
			approvedBy, approvedAt = alex.ID.String(), created
		}
		w.sql(`INSERT INTO devices (id, user_id, name, platform, state, approved_by, approved_at, last_seen_at, created_at)
			VALUES ($1, $2, 'Phone', 'ios', $3, $4, $5, $5, $6)`, id.String(), alex.ID.String(), state, approvedBy, approvedAt, created)
		w.sql(`INSERT INTO device_keys (id, device_id, purpose, jkt, public_jwk, alg) VALUES ($1, $2, 'dpop', $3, '{}', 'ES256')`,
			ids.New().String(), id.String(), jkt)
		return id
	}
	session := func(device ids.ID, familyJKT string) ids.ID {
		id := ids.New()
		w.sql(`INSERT INTO auth_sessions (id, user_id, device_id, client_component, auth_method, expires_at, step_up_at, step_up_method)
			VALUES ($1, $2, $3, 'app', 'passkey', $4, $5, 'passkey')`, id.String(), alex.ID.String(), device.String(),
			peopleStart.Add(24*time.Hour), peopleStart)
		w.sql(`INSERT INTO refresh_token_families (id, session_id, dpop_jkt) VALUES ($1, $2, $3)`, ids.New().String(), id.String(), familyJKT)
		return id
	}
	phone := device("active", "alex-phone-key", peopleStart)
	bound := session(phone, "alex-phone-key")
	otherKey := session(phone, "another-key")
	pending := device("pending", "alex-tablet-key", peopleStart.Add(-25*time.Hour))
	stale := session(pending, "alex-tablet-key")
	ctx := context.Background()
	get := func(user, sid ids.ID) actor.Actor {
		t.Helper()
		a, err := sessions.SessionActor(ctx, user, sid)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	want := func(name string, got, want actor.Actor) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: got %+v, want %+v", name, got, want)
		}
	}
	base := actor.Actor{Kind: actor.KindUser, ID: alex.ID, AuthMethod: "passkey", ClientComponent: "app"}

	active := base
	active.SessionID, active.DeviceID, active.SessionActive, active.DeviceState, active.DeviceKeyActive =
		bound, phone, true, actor.DeviceActive, true
	active.KeyThumbprint = "alex-phone-key"
	want("active", get(alex.ID, bound), active)

	mismatch := active
	mismatch.SessionID, mismatch.DeviceKeyActive, mismatch.CredentialInactive, mismatch.KeyThumbprint = otherKey, false, true, "another-key"
	want("key of another family", get(alex.ID, otherKey), mismatch)

	expiredDevice := base
	expiredDevice.SessionID, expiredDevice.DeviceID, expiredDevice.SessionActive, expiredDevice.DeviceState =
		stale, pending, true, actor.DeviceRevoked
	expiredDevice.DeviceKeyActive, expiredDevice.CredentialInactive, expiredDevice.KeyThumbprint = true, true, "alex-tablet-key"
	want("pending past auth.device_pending_ttl", get(alex.ID, stale), expiredDevice)

	foreign := actor.Actor{Kind: actor.KindUser, ID: sam.ID, SessionID: bound, DeviceState: actor.DeviceUnknown, CredentialInactive: true}
	want("foreign user", get(sam.ID, bound), foreign)

	unknownID := ids.New()
	unknown := actor.Actor{Kind: actor.KindUser, ID: alex.ID, SessionID: unknownID, DeviceState: actor.DeviceUnknown, CredentialInactive: true}
	want("unknown session", get(alex.ID, unknownID), unknown)

	if err := sessions.RevokeSession(ctx, bound, identity.RevokeLogout, nil); err != nil {
		t.Fatal(err)
	}
	revoked := active
	revoked.SessionActive, revoked.DeviceKeyActive, revoked.CredentialInactive, revoked.KeyThumbprint = false, false, true, ""
	want("revoked", get(alex.ID, bound), revoked)

	w.clk.Advance(25 * time.Hour)
	expired := mismatch
	expired.SessionActive = false
	want("expired", get(alex.ID, otherKey), expired)
}
