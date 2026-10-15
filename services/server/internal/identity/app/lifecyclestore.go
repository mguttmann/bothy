// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// DeviceRef is a device and its person, what the inactivity sweep selects.
type DeviceRef struct {
	ID, UserID ids.ID
}

// LifecycleStore is the outgoing port of the revocation use cases (M1-45) over principals,
// devices, device_keys and auth_sessions, beside the stores they share with the other use cases
// (Store, DeviceStore, SessionStore, APITokenStore; implemented by adapters/postgres). Locks and
// writes run only inside a unit of work, rows of one table in ascending id (03 section 6.3).
// Errors are kernel/errs errors without SQL or values.
type LifecycleStore interface {
	// SetPrincipalDisabled sets status disabled and disabled_at = at on the principal id.
	SetPrincipalDisabled(ctx context.Context, id ids.ID, at time.Time) error
	// LockDevice reads the device id FOR UPDATE; found is false for an unknown id.
	LockDevice(ctx context.Context, id ids.ID) (d identity.Device, found bool, err error)
	// LockLiveDevicesOfUser locks every device of userID that is not revoked (pending included)
	// and returns them in ascending id.
	LockLiveDevicesOfUser(ctx context.Context, userID ids.ID) ([]identity.Device, error)
	// LockLiveSessionsOfUser locks every session of userID with revoked_at IS NULL (children
	// included) and returns them in ascending id.
	LockLiveSessionsOfUser(ctx context.Context, userID ids.ID) ([]SessionRecord, error)
	// LockLiveSessionsOfDevice locks every session of deviceID with revoked_at IS NULL (children
	// included, they share the device) and returns them in ascending id.
	LockLiveSessionsOfDevice(ctx context.Context, deviceID ids.ID) ([]SessionRecord, error)
	// RevokeDevices revokes the devices among list that are not revoked yet: state revoked,
	// revoked_at = at, revoked_by = by (the zero id stores NULL) and revoke_reason; it returns
	// the ids it changed, ascending.
	RevokeDevices(ctx context.Context, list []ids.ID, by ids.ID, reason domain.DeviceRevokeReason, at time.Time) ([]ids.ID, error)
	// RevokeKeysOfDevices sets revoked_at = at on every key of the devices in list with
	// revoked_at IS NULL and returns their number.
	RevokeKeysOfDevices(ctx context.Context, list []ids.ID, at time.Time) (int, error)
	// InactiveDevices returns up to limit active devices whose last_seen_at (approved_at when
	// never seen) lies before before, in ascending id strictly after afterID (zero for the
	// first page). It reads without locks.
	InactiveDevices(ctx context.Context, before time.Time, afterID ids.ID, limit int) ([]DeviceRef, error)
}
