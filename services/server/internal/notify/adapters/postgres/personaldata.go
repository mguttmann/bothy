// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
)

// Compile-time check: Registrations serves the participant of notify in the privacy requests
// (M7-06, app.PersonalData).
var _ app.PersonalRegistrations = (*Registrations)(nil)

// OfUser implements app.PersonalRegistrations: every registration of user, active and disabled,
// ascending by id, without token or preview hash. Errors: those of the storage.
func (r *Registrations) OfUser(ctx context.Context, user ids.ID) ([]app.PersonalRegistration, error) {
	rows, err := r.read(ctx).RegistrationsOfUser(ctx, uuid(user))
	if err != nil {
		return nil, failed(err, "reading the push registrations of a person")
	}
	out := make([]app.PersonalRegistration, 0, len(rows))
	for _, row := range rows {
		reg := app.PersonalRegistration{
			ID: idOf(row.ID), DeviceID: idOf(row.DeviceID), Provider: row.Provider, Environment: row.Environment, AppID: row.AppID,
			CreatedAt: timeOf(row.CreatedAt), LastSuccessAt: timeOf(row.LastSuccessAt), DisabledAt: timeOf(row.DisabledAt),
		}
		if row.DisabledReason.Valid {
			reg.DisabledReason = row.DisabledReason.String
		}
		out = append(out, reg)
	}
	return out, nil
}

// DeleteOfUser implements app.PersonalRegistrations: it removes every registration of user in
// the unit of work of ctx; pending notify.push_send jobs of them end with no_registration.
// Errors: internal outside a unit of work, those of the storage.
func (r *Registrations) DeleteOfUser(ctx context.Context, user ids.ID) (int64, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, err
	}
	n, err := q.DeleteRegistrationsOfUser(ctx, uuid(user))
	if err != nil {
		return 0, failed(err, "deleting the push registrations of a person")
	}
	return n, nil
}
