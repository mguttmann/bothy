// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/device"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// DeviceStore is the outgoing port of the device use cases over devices and device_keys
// (implemented by adapters/postgres). Reads join the ambient transaction of ctx when there is
// one; writes run only inside a unit of work. Errors are kernel/errs errors without SQL or values:
// a known device id is conflict on /device_id, a known thumbprint conflict on /public_jwk (or
// /step_up_key/public_jwk), a failed foreign key validation_failed with the column as field.
type DeviceStore interface {
	// InsertDevice inserts d and its keys and returns the stored device; approved_at is the
	// database time when d is active.
	InsertDevice(ctx context.Context, d device.Device) (identity.Device, error)
	// Device returns the device id; found is false for an unknown id.
	Device(ctx context.Context, id ids.ID) (d identity.Device, found bool, err error)
	// DevicesOfUser returns up to limit devices of userID, every state, by id descending, with
	// ids below before (the zero id for the first page).
	DevicesOfUser(ctx context.Context, userID, before ids.ID, limit int) ([]identity.Device, error)
	// ActiveKey returns the key of deviceID and purpose with revoked_at IS NULL.
	ActiveKey(ctx context.Context, deviceID ids.ID, purpose identity.KeyPurpose) (k identity.DeviceKey, found bool, err error)
	// LockDevice returns the device id locked FOR UPDATE until the end of the unit of work; found
	// is false for an unknown id (M1-38).
	LockDevice(ctx context.Context, id ids.ID) (d identity.Device, found bool, err error)
	// ActivateDevice sets state active, approved_by = by and approved_at = at on the device id
	// while it is pending; activated is false otherwise.
	ActivateDevice(ctx context.Context, id, by ids.ID, at time.Time) (activated bool, err error)
	// RevokeDevice sets state revoked, revoked_at = at, revoked_by = by (zero for NULL) and
	// revoke_reason on the device id while it is not revoked, and revoked_at on its active keys;
	// it returns the person of the device, revoked false when it was revoked already.
	RevokeDevice(ctx context.Context, id ids.ID, reason identity.DeviceRevokeReason, by ids.ID, at time.Time) (userID ids.ID, revoked bool, err error)
	// ActiveDevicesOfUsers returns the devices with state active of userIDs, grouped by person,
	// each list ascending by id; a person without one is absent (identity.Devices.ActiveForUsers).
	ActiveDevicesOfUsers(ctx context.Context, userIDs []ids.ID) (map[ids.ID][]identity.DeviceRef, error)
	// PendingDevicesCreatedBefore returns up to limit pending devices with created_at <= before,
	// oldest first (the expiry of the job identity.auth_cleanup).
	PendingDevicesCreatedBefore(ctx context.Context, before time.Time, limit int) ([]identity.Device, error)
}

// Cursors is the outgoing port of the cursor codec (platform/httpserver, 06 "6. Pagination"):
// opaque cursors bound to the list, the filter and the caller. A mismatch on Decode is
// 400 cursor_invalid.
type Cursors interface {
	// Encode returns the cursor of key for list and binding.
	Encode(list string, binding []byte, key any) (string, error)
	// Decode checks s against list and binding and unmarshals its key into key.
	Decode(list string, binding []byte, s string, key any) error
}
