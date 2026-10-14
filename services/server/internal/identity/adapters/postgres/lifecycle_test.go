// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// exec runs stmt on the pool of bothy_app.
func (e env) exec(t *testing.T, stmt string, args ...any) {
	t.Helper()
	if _, err := e.pool.Exec(context.Background(), stmt, args...); err != nil {
		t.Fatalf("%s: %v", stmt, err)
	}
}

// device inserts a device of user in state with a dpop key (and a step_up key when stepUp) and
// last_seen_at seen (NULL for the zero time).
func (e env) device(t *testing.T, user ids.ID, state string, seen time.Time, stepUp bool) ids.ID {
	t.Helper()
	id := ids.New()
	var approvedBy any
	var revokedAt any
	if state == "active" {
		approvedBy = user.String()
	}
	if state == "revoked" {
		revokedAt = time.Now()
	}
	var lastSeen any
	if !seen.IsZero() {
		lastSeen = seen
	}
	e.exec(t, `INSERT INTO devices (id, user_id, name, platform, state, approved_by, approved_at, revoked_at, last_seen_at)
		VALUES ($1, $2, 'Phone', 'ios', $3, $4, CASE WHEN $4::uuid IS NULL THEN NULL ELSE now() END, $5, $6)`,
		id.String(), user.String(), state, approvedBy, revokedAt, lastSeen)
	purposes := []string{"dpop"}
	if stepUp {
		purposes = append(purposes, "step_up")
	}
	for _, p := range purposes {
		e.exec(t, `INSERT INTO device_keys (id, device_id, purpose, jkt, public_jwk, alg) VALUES ($1, $2, $3, $4, '{}', 'ES256')`,
			ids.New().String(), id.String(), p, ids.New().String())
	}
	return id
}

// session inserts a session of user on device, a child of parent when parent is not zero, revoked
// when revoked is true.
func (e env) session(t *testing.T, user, device, parent ids.ID, revoked bool) ids.ID {
	t.Helper()
	id := ids.New()
	component := "app"
	var p any
	if !parent.IsZero() {
		component, p = "share_ext", parent.String()
	}
	var at, reason any
	if revoked {
		at, reason = time.Now(), "logout"
	}
	e.exec(t, `INSERT INTO auth_sessions (id, user_id, device_id, client_component, parent_session_id, auth_method, expires_at, revoked_at, revoke_reason)
		VALUES ($1, $2, $3, $4, $5, 'passkey', now() + interval '1 day', $6, $7)`, id.String(), user.String(), device.String(), component, p, at, reason)
	return id
}

// sessionIDs returns the ids of list.
func sessionIDs(list []app.SessionRecord) []ids.ID {
	out := make([]ids.ID, 0, len(list))
	for _, s := range list {
		out = append(out, s.ID)
	}
	return out
}

// TestLifecycle_StoreRoundTrip (M1-45): the revocation store locks the live devices and sessions
// of a person or a device in ascending id, revokes devices once with reason and actor (NULL for
// the system) and their keys, disables a principal, and lists inactive active devices by
// last_seen_at, else approved_at, in pages. Locks and writes need a unit of work.
func TestLifecycle_StoreRoundTrip(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sam := e.mustAddUser(t, "sam", "sam@example.com").Principal.ID
	kim := e.mustAddUser(t, "kim", "kim@example.com").Principal.ID
	old := time.Now().Add(-200 * 24 * time.Hour).UTC().Truncate(time.Microsecond)
	d1 := e.device(t, sam, "active", old, true)
	d2 := e.device(t, sam, "pending", time.Time{}, false)
	d3 := e.device(t, sam, "revoked", time.Time{}, false)
	dk := e.device(t, kim, "active", old, false)
	recent := e.device(t, kim, "active", time.Now(), false)
	s1 := e.session(t, sam, d1, ids.ID{}, false)
	s1c := e.session(t, sam, d1, s1, false)
	s2 := e.session(t, sam, d2, ids.ID{}, false)
	e.session(t, sam, d1, ids.ID{}, true)
	e.session(t, kim, dk, ids.ID{}, false)

	if _, err := e.repo.LockLiveDevicesOfUser(ctx, sam); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("lock outside a unit of work: %v", err)
	}
	if err := e.repo.SetPrincipalDisabled(ctx, sam, time.Now()); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("write outside a unit of work: %v", err)
	}
	at := time.Now().UTC().Truncate(time.Microsecond)
	err := e.write(t, func(ctx context.Context) error {
		devices, err := e.repo.LockLiveDevicesOfUser(ctx, sam)
		if err != nil {
			return err
		}
		var got []ids.ID
		for _, d := range devices {
			got = append(got, d.ID)
		}
		if !slices.Equal(got, []ids.ID{d1, d2}) || devices[0].StepUpKeyThumbprint == "" {
			t.Errorf("live devices %v", devices)
		}
		if d, ok, err := e.repo.LockDevice(ctx, d1); err != nil || !ok || d.State != identity.DeviceStateActive {
			t.Errorf("LockDevice = %+v, %v, %v", d, ok, err)
		}
		if _, ok, err := e.repo.LockDevice(ctx, ids.New()); ok || err != nil {
			t.Errorf("unknown device: %v, %v", ok, err)
		}
		live, err := e.repo.LockLiveSessionsOfUser(ctx, sam)
		if err != nil {
			return err
		}
		if !slices.Equal(sessionIDs(live), []ids.ID{s1, s1c, s2}) {
			t.Errorf("live sessions of sam %v", sessionIDs(live))
		}
		onDevice, err := e.repo.LockLiveSessionsOfDevice(ctx, d1)
		if err != nil {
			return err
		}
		if !slices.Equal(sessionIDs(onDevice), []ids.ID{s1, s1c}) || onDevice[1].ParentID != s1 {
			t.Errorf("live sessions of d1 %+v", onDevice)
		}
		revoked, err := e.repo.RevokeDevices(ctx, []ids.ID{d3, d2, d1}, sam, domain.DeviceRevokeUserDisabled, at)
		if err != nil {
			return err
		}
		if !slices.Equal(revoked, []ids.ID{d1, d2}) {
			t.Errorf("revoked %v", revoked)
		}
		if again, err := e.repo.RevokeDevices(ctx, []ids.ID{d1}, sam, domain.DeviceRevokeUserDisabled, at); err != nil || len(again) != 0 {
			t.Errorf("second revoke %v, %v", again, err)
		}
		n, err := e.repo.RevokeKeysOfDevices(ctx, revoked, at)
		if err != nil || n != 3 {
			t.Errorf("RevokeKeysOfDevices = %d, %v", n, err)
		}
		return e.repo.SetPrincipalDisabled(ctx, sam, at)
	})
	if err != nil {
		t.Fatal(err)
	}
	d, _, err := e.repo.Device(ctx, d1)
	if err != nil || d.State != identity.DeviceStateRevoked || d.RevokeReason != "user_disabled" || !d.RevokedAt.Equal(at) || d.KeyThumbprint != "" {
		t.Fatalf("revoked device %+v, %v", d, err)
	}
	p, _, err := e.repo.Principal(ctx, sam)
	if err != nil || p.Principal.Status != domain.StatusDisabled || !p.Principal.DisabledAt.Equal(at) {
		t.Fatalf("principal %+v, %v", p, err)
	}

	list, err := e.repo.InactiveDevices(ctx, time.Now().Add(-180*24*time.Hour), ids.ID{}, 10)
	if err != nil || len(list) != 1 || list[0] != (app.DeviceRef{ID: dk, UserID: kim}) {
		t.Fatalf("inactive devices %+v, %v", list, err)
	}
	if list, err := e.repo.InactiveDevices(ctx, time.Now().Add(-180*24*time.Hour), dk, 10); err != nil || len(list) != 0 {
		t.Fatalf("inactive devices after dk %+v, %v", list, err)
	}
	if list, err := e.repo.InactiveDevices(ctx, time.Now().Add(time.Hour), ids.ID{}, 1); err != nil || len(list) != 1 || list[0].ID != dk {
		t.Fatalf("first page of one %+v, %v", list, err)
	}
	if err := e.write(t, func(ctx context.Context) error {
		_, err := e.repo.RevokeDevices(ctx, []ids.ID{recent}, ids.ID{}, domain.DeviceRevokeInactive, at)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var by *string
	if err := e.pool.QueryRow(ctx, `SELECT revoked_by::text FROM devices WHERE id = $1`, recent.String()).Scan(&by); err != nil || by != nil {
		t.Fatalf("revoked_by of a system revocation %v, %v", by, err)
	}
}
