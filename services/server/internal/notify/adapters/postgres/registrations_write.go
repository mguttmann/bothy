// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
)

// LockByProviderToken implements app.RegistrationWriter with SELECT ... FOR UPDATE in the unit of
// work of ctx.
func (r *Registrations) LockByProviderToken(ctx context.Context, provider, token string) (ids.ID, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return ids.ID{}, false, err
	}
	id, err := q.LockRegistrationByProviderToken(ctx, sqlcgen.LockRegistrationByProviderTokenParams{Provider: provider, Token: token})
	if errors.Is(err, pgx.ErrNoRows) {
		return ids.ID{}, false, nil
	}
	if err != nil {
		return ids.ID{}, false, failed(err, "locking a push registration by its token")
	}
	return idOf(id), true, nil
}

// Delete implements app.RegistrationWriter.
func (r *Registrations) Delete(ctx context.Context, id ids.ID) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.DeleteRegistration(ctx, uuid(id))
	if err != nil {
		return false, failed(err, "deleting a push registration")
	}
	return n > 0, nil
}

// DisableOthersOfApp implements app.RegistrationWriter.
func (r *Registrations) DisableOthersOfApp(ctx context.Context, deviceID ids.ID, provider, appID, reason string, at time.Time) (int64, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, err
	}
	n, err := q.DisableOtherRegistrationsOfApp(ctx, sqlcgen.DisableOtherRegistrationsOfAppParams{
		DisabledAt: stamp(at), DisabledReason: text(reason), DeviceID: uuid(deviceID), Provider: provider, AppID: appID,
	})
	if err != nil {
		return 0, failed(err, "disabling the older push registrations of an app")
	}
	return n, nil
}

// Insert implements app.RegistrationWriter; a row of the same (provider, token) committed by a
// concurrent registration gives app.ErrRegistrationTaken without aborting the transaction.
func (r *Registrations) Insert(ctx context.Context, in app.NewRegistration) (notify.PushRegistration, error) {
	q, err := r.write(ctx)
	if err != nil {
		return notify.PushRegistration{}, err
	}
	row, err := q.InsertRegistration(ctx, sqlcgen.InsertRegistrationParams{
		ID: uuid(in.ID), UserID: uuid(in.UserID), DeviceID: uuid(in.DeviceID), AuthSessionID: uuid(in.SessionID),
		Provider: in.Provider, Token: in.Token, Environment: in.Environment, AppID: in.AppID, PreviewTokenHash: in.PreviewHash,
		CreatedAt: stamp(in.CreatedAt),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// ON CONFLICT (provider, token) DO NOTHING: a concurrent registration committed the token
		// first; the transaction stays usable for the takeover.
		return notify.PushRegistration{}, app.ErrRegistrationTaken
	}
	if err != nil {
		return notify.PushRegistration{}, failed(err, "inserting a push registration")
	}
	return notify.PushRegistration{
		ID: idOf(row.ID), UserID: idOf(row.UserID), DeviceID: idOf(row.DeviceID), Provider: row.Provider,
		Environment: row.Environment, AppID: row.AppID, CreatedAt: timeOf(row.CreatedAt),
	}, nil
}

// ByPreviewHash implements app.RegistrationWriter on the unique partial index of
// preview_token_hash; a disabled row is not found.
func (r *Registrations) ByPreviewHash(ctx context.Context, hash []byte) (app.PreviewRegistration, bool, error) {
	row, err := r.read(ctx).GetRegistrationByPreviewHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.PreviewRegistration{}, false, nil
	}
	if err != nil {
		return app.PreviewRegistration{}, false, failed(err, "reading a push registration by its preview token")
	}
	return app.PreviewRegistration{
		ID: idOf(row.ID), UserID: idOf(row.UserID), DeviceID: idOf(row.DeviceID), SessionID: idOf(row.AuthSessionID), Provider: row.Provider,
	}, true, nil
}

// OwnerOf implements app.RegistrationWriter with SELECT ... FOR UPDATE in the unit of work of ctx.
func (r *Registrations) OwnerOf(ctx context.Context, id ids.ID) (ids.ID, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return ids.ID{}, false, err
	}
	user, err := q.GetRegistrationForOwner(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ids.ID{}, false, nil
	}
	if err != nil {
		return ids.ID{}, false, failed(err, "locking a push registration")
	}
	return idOf(user), true, nil
}

// DisableOfSessions implements app.RegistrationWriter.
func (r *Registrations) DisableOfSessions(ctx context.Context, sessionIDs []ids.ID, reason string, at time.Time) error {
	if len(sessionIDs) == 0 {
		return nil
	}
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	if _, err := q.DisableRegistrationsOfSessions(ctx, sqlcgen.DisableRegistrationsOfSessionsParams{
		DisabledAt: stamp(at), DisabledReason: text(reason), SessionIds: uuids(sessionIDs),
	}); err != nil {
		return failed(err, "disabling the push registrations of ended sessions")
	}
	return nil
}

// DisableOfDevice implements app.RegistrationWriter.
func (r *Registrations) DisableOfDevice(ctx context.Context, deviceID ids.ID, reason string, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	if _, err := q.DisableRegistrationsOfDevice(ctx, sqlcgen.DisableRegistrationsOfDeviceParams{
		DisabledAt: stamp(at), DisabledReason: text(reason), DeviceID: uuid(deviceID),
	}); err != nil {
		return failed(err, "disabling the push registrations of a revoked device")
	}
	return nil
}

// OfDevices implements app.RegistrationWriter with one query; devices without a registration are
// absent, no devices give an empty map.
func (r *Registrations) OfDevices(ctx context.Context, deviceIDs []ids.ID) (map[ids.ID][]identity.PushTargetView, error) {
	out := map[ids.ID][]identity.PushTargetView{}
	if len(deviceIDs) == 0 {
		return out, nil
	}
	rows, err := r.read(ctx).ListRegistrationsOfDevices(ctx, uuids(deviceIDs))
	if err != nil {
		return nil, failed(err, "reading the push registrations of devices")
	}
	for _, row := range rows {
		d := idOf(row.DeviceID)
		v := identity.PushTargetView{
			ID: idOf(row.ID), DeviceID: d, Provider: row.Provider, Environment: row.Environment, AppID: row.AppID,
			CreatedAt: timeOf(row.CreatedAt), DisabledAt: timeOf(row.DisabledAt),
		}
		if row.DisabledReason.Valid {
			v.DisabledReason = row.DisabledReason.String
		}
		out[d] = append(out[d], v)
	}
	return out, nil
}
