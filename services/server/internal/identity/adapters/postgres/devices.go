// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/device"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the device store of the identity application layer.
var _ app.DeviceStore = (*Repository)(nil)

// InsertDevice implements app.DeviceStore inside the ambient unit of work: the device row, then
// its keys in the order of d.Keys, each with a new id. A conflict on the thumbprint of the
// step-up key names /step_up_key/public_jwk.
func (r *Repository) InsertDevice(ctx context.Context, d device.Device) (identity.Device, error) {
	q, err := r.write(ctx)
	if err != nil {
		return identity.Device{}, err
	}
	row, err := q.InsertDevice(ctx, sqlcgen.InsertDeviceParams{
		ID: uuid(d.ID), UserID: uuid(d.UserID), Name: d.Name, Platform: string(d.Platform), State: string(d.State),
		AppVersion: pgtype.Text{String: d.AppVersion, Valid: d.AppVersion != ""}, ApprovedBy: optionalUUID(d.ApprovedBy),
	})
	if err != nil {
		return identity.Device{}, mapError(err, "inserting a device")
	}
	out := identity.Device{
		ID: d.ID, UserID: d.UserID, Name: d.Name, Platform: identity.Platform(d.Platform), State: identity.DeviceState(d.State),
		AppVersion: d.AppVersion, CreatedAt: timeOf(row.CreatedAt), ApprovedAt: timeOf(row.ApprovedAt),
	}
	for _, k := range d.Keys {
		err := q.InsertDeviceKey(ctx, sqlcgen.InsertDeviceKeyParams{
			ID: uuid(ids.New()), DeviceID: uuid(d.ID), Purpose: string(k.Purpose), Jkt: k.JKT,
			PublicJwk: k.PublicJWK, Alg: k.Alg, Attestation: k.Attestation,
		})
		if err != nil {
			return identity.Device{}, keyError(mapError(err, "inserting a device key"), k.Purpose)
		}
		if k.Purpose == device.PurposeDPoP {
			out.KeyThumbprint = k.JKT
		} else {
			out.StepUpKeyThumbprint = k.JKT
		}
	}
	return out, nil
}

// keyError moves the field pointers of a step-up key error below /step_up_key.
func keyError(err error, p device.Purpose) error {
	e, ok := errs.As(err)
	if !ok || p != device.PurposeStepUp {
		return err
	}
	for i := range e.Fields {
		e.Fields[i].Pointer = "/step_up_key" + e.Fields[i].Pointer
	}
	return e
}

// Device implements app.DeviceStore.
func (r *Repository) Device(ctx context.Context, id ids.ID) (identity.Device, bool, error) {
	row, err := r.read(ctx).GetDevice(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.Device{}, false, nil
	}
	if err != nil {
		return identity.Device{}, false, mapError(err, "reading a device")
	}
	return toDevice(row.Device, row.KeyThumbprint, row.StepUpKeyThumbprint), true, nil
}

// DevicesOfUser implements app.DeviceStore.
func (r *Repository) DevicesOfUser(ctx context.Context, userID, before ids.ID, limit int) ([]identity.Device, error) {
	rows, err := r.read(ctx).ListDevicesOfUser(ctx, sqlcgen.ListDevicesOfUserParams{
		UserID: uuid(userID), Before: optionalUUID(before), RowLimit: int32(min(limit, 1<<30)), //nolint:gosec // G115: capped above
	})
	if err != nil {
		return nil, mapError(err, "listing devices")
	}
	out := make([]identity.Device, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDevice(row.Device, row.KeyThumbprint, row.StepUpKeyThumbprint))
	}
	return out, nil
}

// ActiveKey implements app.DeviceStore.
func (r *Repository) ActiveKey(ctx context.Context, deviceID ids.ID, purpose identity.KeyPurpose) (identity.DeviceKey, bool, error) {
	row, err := r.read(ctx).GetActiveDeviceKey(ctx, sqlcgen.GetActiveDeviceKeyParams{DeviceID: uuid(deviceID), Purpose: string(purpose)})
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.DeviceKey{}, false, nil
	}
	if err != nil {
		return identity.DeviceKey{}, false, mapError(err, "reading a device key")
	}
	return toKey(row), true, nil
}

// KeyByJKT returns the key with the thumbprint jkt, revoked or not; found is false when none has
// it. The login tickets use it to refuse a key that another device holds.
func (r *Repository) KeyByJKT(ctx context.Context, jkt string) (identity.DeviceKey, bool, error) {
	row, err := r.read(ctx).GetDeviceKeyByJKT(ctx, jkt)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.DeviceKey{}, false, nil
	}
	if err != nil {
		return identity.DeviceKey{}, false, mapError(err, "reading a device key")
	}
	return toKey(row), true, nil
}

func toDevice(d sqlcgen.Device, key, stepUp pgtype.Text) identity.Device {
	return identity.Device{
		ID: idOf(d.ID), UserID: idOf(d.UserID), Name: d.Name, Platform: identity.Platform(d.Platform),
		State: identity.DeviceState(d.State), AppVersion: d.AppVersion.String,
		KeyThumbprint: key.String, StepUpKeyThumbprint: stepUp.String, RevokeReason: d.RevokeReason.String,
		ApprovedAt: timeOf(d.ApprovedAt), RevokedAt: timeOf(d.RevokedAt), CreatedAt: timeOf(d.CreatedAt),
		LastSeenAt: timeOf(d.LastSeenAt),
	}
}

func toKey(k sqlcgen.DeviceKey) identity.DeviceKey {
	return identity.DeviceKey{
		ID: idOf(k.ID), DeviceID: idOf(k.DeviceID), Purpose: identity.KeyPurpose(k.Purpose), JKT: k.Jkt, Alg: k.Alg,
		PublicJWK: k.PublicJwk, RevokedAt: timeOf(k.RevokedAt),
	}
}
