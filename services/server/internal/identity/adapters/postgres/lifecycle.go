// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the store of the revocation use cases (M1-45).
var _ app.LifecycleStore = (*Repository)(nil)

// SetPrincipalDisabled implements app.LifecycleStore.
func (r *Repository) SetPrincipalDisabled(ctx context.Context, id ids.ID, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.DisablePrincipal(ctx, sqlcgen.DisablePrincipalParams{ID: uuid(id), DisabledAt: ts(at)}), "disabling a principal")
}

// LockLiveDevicesOfUser implements app.LifecycleStore.
func (r *Repository) LockLiveDevicesOfUser(ctx context.Context, userID ids.ID) ([]identity.Device, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockLiveDevicesOfUser(ctx, uuid(userID))
	if err != nil {
		return nil, mapError(err, "locking the devices of a person")
	}
	out := make([]identity.Device, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDevice(row.Device, row.KeyThumbprint, row.StepUpKeyThumbprint))
	}
	return out, nil
}

// LockLiveSessionsOfUser implements app.LifecycleStore.
func (r *Repository) LockLiveSessionsOfUser(ctx context.Context, userID ids.ID) ([]app.SessionRecord, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockLiveSessionsOfUser(ctx, uuid(userID))
	return sessionRows(rows, err, "locking the sessions of a person")
}

// LockLiveSessionsOfDevice implements app.LifecycleStore.
func (r *Repository) LockLiveSessionsOfDevice(ctx context.Context, deviceID ids.ID) ([]app.SessionRecord, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockLiveSessionsOfDevice(ctx, uuid(deviceID))
	return sessionRows(rows, err, "locking the sessions of a device")
}

// sessionRows converts the rows of a session query, or maps its error.
func sessionRows(rows []sqlcgen.AuthSession, err error, what string) ([]app.SessionRecord, error) {
	if err != nil {
		return nil, mapError(err, what)
	}
	out := make([]app.SessionRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSession(row))
	}
	return out, nil
}

// RevokeDevices implements app.LifecycleStore.
func (r *Repository) RevokeDevices(ctx context.Context, list []ids.ID, by ids.ID, reason domain.DeviceRevokeReason, at time.Time) ([]ids.ID, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.RevokeDevices(ctx, sqlcgen.RevokeDevicesParams{
		RevokedAt: ts(at), RevokedBy: optionalUUID(by), RevokeReason: pgtype.Text{String: string(reason), Valid: true}, Ids: uuids(list),
	})
	if err != nil {
		return nil, mapError(err, "revoking devices")
	}
	out := make([]ids.ID, 0, len(rows))
	for _, u := range rows {
		out = append(out, idOf(u))
	}
	slices.SortFunc(out, compareIDs)
	return out, nil
}

// RevokeKeysOfDevices implements app.LifecycleStore.
func (r *Repository) RevokeKeysOfDevices(ctx context.Context, list []ids.ID, at time.Time) (int, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, err
	}
	n, err := q.RevokeKeysOfDevices(ctx, sqlcgen.RevokeKeysOfDevicesParams{RevokedAt: ts(at), DeviceIds: uuids(list)})
	if err != nil {
		return 0, mapError(err, "revoking device keys")
	}
	return int(n), nil
}

// InactiveDevices implements app.LifecycleStore.
func (r *Repository) InactiveDevices(ctx context.Context, before time.Time, afterID ids.ID, limit int) ([]app.DeviceRef, error) {
	rows, err := r.read(ctx).ListInactiveDevices(ctx, sqlcgen.ListInactiveDevicesParams{
		Before: ts(before), AfterID: optionalUUID(afterID), RowLimit: int32(min(limit, 1<<30)), //nolint:gosec // G115: capped above
	})
	if err != nil {
		return nil, mapError(err, "listing inactive devices")
	}
	out := make([]app.DeviceRef, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.DeviceRef{ID: idOf(row.ID), UserID: idOf(row.UserID)})
	}
	return out, nil
}
