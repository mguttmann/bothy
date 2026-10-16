// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the pairing store (M1-38).
var _ app.PairingStore = (*Repository)(nil)

// InsertPairing implements app.PairingStore.
func (r *Repository) InsertPairing(ctx context.Context, rec app.PairingRecord) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	reg, err := json.Marshal(rec.Registration)
	if err != nil {
		return false, errs.Wrap(errs.CodeInternalError, err, "identity: encoding a pairing registration")
	}
	n, err := q.InsertPairing(ctx, sqlcgen.InsertPairingParams{
		ID: uuid(rec.ID), PairingCodeHash: rec.Hash, DeviceRegistration: reg, CreatedAt: ts(rec.CreatedAt), ExpiresAt: ts(rec.ExpiresAt),
	})
	if err != nil {
		return false, mapError(err, "inserting a pairing")
	}
	return n == 1, nil
}

// LockRequestedPairingByHash implements app.PairingStore.
func (r *Repository) LockRequestedPairingByHash(ctx context.Context, hash []byte) (app.PairingRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.PairingRecord{}, false, err
	}
	return toPairing(q.LockRequestedPairingByHash(ctx, hash))
}

// LockPairing implements app.PairingStore.
func (r *Repository) LockPairing(ctx context.Context, id ids.ID) (app.PairingRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.PairingRecord{}, false, err
	}
	return toPairing(q.LockPairing(ctx, uuid(id)))
}

// UpdatePairing implements app.PairingStore.
func (r *Repository) UpdatePairing(ctx context.Context, rec app.PairingRecord) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.UpdatePairing(ctx, sqlcgen.UpdatePairingParams{
		ID: uuid(rec.ID), Status: string(rec.Status), UserID: optionalUUID(rec.UserID), ApprovedByDeviceID: optionalUUID(rec.ApprovedByDeviceID),
		DeviceID: optionalUUID(rec.DeviceID), ExpiresAt: ts(rec.ExpiresAt), CompletedAt: optionalTS(rec.CompletedAt),
	})
	return mapError(err, "updating a pairing")
}

// DuePairings implements app.PairingStore.
func (r *Repository) DuePairings(ctx context.Context, now time.Time, limit int) ([]app.PairingRecord, error) {
	rows, err := r.read(ctx).ListDuePairings(ctx, sqlcgen.ListDuePairingsParams{
		Now: ts(now), RowLimit: int32(min(limit, 1<<30)), //nolint:gosec // G115: capped above
	})
	if err != nil {
		return nil, mapError(err, "listing due pairings")
	}
	out := make([]app.PairingRecord, 0, len(rows))
	for _, row := range rows {
		rec, _, err := toPairing(row, nil)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

// optionalTS is a timestamp that is NULL for the zero time.
func optionalTS(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}

// toPairing converts a row of device_pairings; found is false for no row.
func toPairing(row sqlcgen.DevicePairing, err error) (app.PairingRecord, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.PairingRecord{}, false, nil
	}
	if err != nil {
		return app.PairingRecord{}, false, mapError(err, "reading a pairing")
	}
	rec := app.PairingRecord{
		ID: idOf(row.ID), Hash: row.PairingCodeHash, Status: domain.PairingStatus(row.Status), UserID: idOf(row.UserID),
		ApprovedByDeviceID: idOf(row.ApprovedByDeviceID), DeviceID: idOf(row.DeviceID), CreatedAt: timeOf(row.CreatedAt),
		ExpiresAt: timeOf(row.ExpiresAt), CompletedAt: timeOf(row.CompletedAt),
	}
	if err := json.Unmarshal(row.DeviceRegistration, &rec.Registration); err != nil {
		return app.PairingRecord{}, false, errs.Wrap(errs.CodeInternalError, err, "identity: decoding a pairing registration")
	}
	return rec, true, nil
}
