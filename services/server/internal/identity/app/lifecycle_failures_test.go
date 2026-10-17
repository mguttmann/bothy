// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// errStore is the injected store failure.
var errStore = errors.New("injected store failure")

// failingLifecycleStore is the PostgreSQL store with one method failing.
type failingLifecycleStore struct {
	app.LifecycleStore
	fail string
}

func (s failingLifecycleStore) SetPrincipalDisabled(ctx context.Context, id ids.ID, at time.Time) error {
	if s.fail == "SetPrincipalDisabled" {
		return errStore
	}
	return s.LifecycleStore.SetPrincipalDisabled(ctx, id, at)
}

func (s failingLifecycleStore) LockDevice(ctx context.Context, id ids.ID) (identity.Device, bool, error) {
	if s.fail == "LockDevice" {
		return identity.Device{}, false, errStore
	}
	return s.LifecycleStore.LockDevice(ctx, id)
}

func (s failingLifecycleStore) LockLiveDevicesOfUser(ctx context.Context, userID ids.ID) ([]identity.Device, error) {
	if s.fail == "LockLiveDevicesOfUser" {
		return nil, errStore
	}
	return s.LifecycleStore.LockLiveDevicesOfUser(ctx, userID)
}

func (s failingLifecycleStore) LockLiveSessionsOfUser(ctx context.Context, userID ids.ID) ([]app.SessionRecord, error) {
	if s.fail == "LockLiveSessionsOfUser" {
		return nil, errStore
	}
	return s.LifecycleStore.LockLiveSessionsOfUser(ctx, userID)
}

func (s failingLifecycleStore) LockLiveSessionsOfDevice(ctx context.Context, deviceID ids.ID) ([]app.SessionRecord, error) {
	if s.fail == "LockLiveSessionsOfDevice" {
		return nil, errStore
	}
	return s.LifecycleStore.LockLiveSessionsOfDevice(ctx, deviceID)
}

func (s failingLifecycleStore) RevokeDevices(ctx context.Context, list []ids.ID, by ids.ID, reason domain.DeviceRevokeReason, at time.Time) ([]ids.ID, error) {
	if s.fail == "RevokeDevices" {
		return nil, errStore
	}
	return s.LifecycleStore.RevokeDevices(ctx, list, by, reason, at)
}

func (s failingLifecycleStore) RevokeKeysOfDevices(ctx context.Context, list []ids.ID, at time.Time) (int, error) {
	if s.fail == "RevokeKeysOfDevices" {
		return 0, errStore
	}
	return s.LifecycleStore.RevokeKeysOfDevices(ctx, list, at)
}

func (s failingLifecycleStore) InactiveDevices(ctx context.Context, before time.Time, afterID ids.ID, limit int) ([]app.DeviceRef, error) {
	if s.fail == "InactiveDevices" {
		return nil, errStore
	}
	return s.LifecycleStore.InactiveDevices(ctx, before, afterID, limit)
}

// TestLifecycle_StoreFailures: a failing step of the store fails the use case with its error and
// leaves nothing of the unit behind (the person stays active, the device and the session live).
func TestLifecycle_StoreFailures(t *testing.T) {
	t.Parallel()
	w := newLifeWorld(t)
	o := w.person("owner", "active", true)
	alex := w.person("alex", "active", false)
	d := w.device(alex.ID, "active", peopleStart.Add(-200*24*time.Hour))
	s := w.session(alex.ID, d, ids.ID{})
	failing := func(method string) *app.Lifecycle {
		deps := w.deps
		deps.Store = failingLifecycleStore{LifecycleStore: w.deps.Store, fail: method}
		l, err := app.NewLifecycle(w.cfg, deps)
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	ctx := ctxOf(o)
	for _, m := range []string{"SetPrincipalDisabled", "LockLiveDevicesOfUser", "LockLiveSessionsOfUser", "RevokeDevices", "RevokeKeysOfDevices"} {
		if _, err := failing(m).DisableUser(ctx, identity.DisableUserCmd{Actor: o, UserID: alex.ID, Reason: "Left"}); !errors.Is(err, errStore) {
			t.Errorf("DisableUser with %s failing = %v", m, err)
		}
	}
	for _, m := range []string{"LockDevice", "LockLiveSessionsOfDevice", "RevokeDevices", "RevokeKeysOfDevices"} {
		if _, err := failing(m).RevokeDevice(ctx, identity.RevokeDeviceCmd{Actor: o, DeviceID: d, Reason: "Lost"}); !errors.Is(err, errStore) {
			t.Errorf("RevokeDevice with %s failing = %v", m, err)
		}
	}
	for _, m := range []string{"InactiveDevices", "LockDevice", "LockLiveSessionsOfDevice", "RevokeDevices"} {
		if _, err := failing(m).ExpireInactiveDevices(context.Background()); !errors.Is(err, errStore) {
			t.Errorf("ExpireInactiveDevices with %s failing = %v", m, err)
		}
	}
	if st := w.text(`SELECT status FROM principals WHERE id = $1`, alex.ID.String()); st != "active" {
		t.Errorf("person is %s after the failures", st)
	}
	if st := w.text(`SELECT state FROM devices WHERE id = $1`, d.String()); st != "active" {
		t.Errorf("device is %s after the failures", st)
	}
	if n := w.count(`SELECT count(*) FROM auth_sessions WHERE id = $1 AND revoked_at IS NULL`, s.String()); n != 1 {
		t.Errorf("session revoked after the failures")
	}
}
