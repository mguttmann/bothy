// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

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
