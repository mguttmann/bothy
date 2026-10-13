// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the enrollment code store (M1-37).
var _ app.EnrollmentStore = (*Repository)(nil)

// InsertEnrollmentCode implements app.EnrollmentStore.
func (r *Repository) InsertEnrollmentCode(ctx context.Context, rec app.EnrollmentCodeRecord) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.InsertEnrollmentCode(ctx, sqlcgen.InsertEnrollmentCodeParams{
		ID: uuid(rec.ID), UserID: uuid(rec.UserID), Purpose: string(rec.Purpose), CodeHash: rec.Hash,
		ExpiresAt: ts(rec.ExpiresAt), CreatedBy: optionalUUID(rec.CreatedBy), CreatedAt: ts(rec.CreatedAt),
	})
	return mapError(err, "inserting an enrollment code")
}

// ExpireOtherEnrollmentCodes implements app.EnrollmentStore.
func (r *Repository) ExpireOtherEnrollmentCodes(ctx context.Context, userID, keep ids.ID, now time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.ExpireOtherEnrollmentCodes(ctx, sqlcgen.ExpireOtherEnrollmentCodesParams{Now: ts(now), UserID: uuid(userID), Keep: uuid(keep)})
	return mapError(err, "ending older enrollment codes")
}

// LockEnrollmentCodeByHash implements app.EnrollmentStore.
func (r *Repository) LockEnrollmentCodeByHash(ctx context.Context, hash []byte) (app.EnrollmentCodeRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.EnrollmentCodeRecord{}, false, err
	}
	row, err := q.LockEnrollmentCodeByHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.EnrollmentCodeRecord{}, false, nil
	}
	if err != nil {
		return app.EnrollmentCodeRecord{}, false, mapError(err, "locking an enrollment code")
	}
	return toEnrollmentCode(row), true, nil
}

// IncrementEnrollmentAttempts implements app.EnrollmentStore.
func (r *Repository) IncrementEnrollmentAttempts(ctx context.Context, id ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	_, err = q.IncrementEnrollmentAttempts(ctx, uuid(id))
	return mapError(err, "counting a failed enrollment attempt")
}

// MarkEnrollmentCodeUsed implements app.EnrollmentStore.
func (r *Repository) MarkEnrollmentCodeUsed(ctx context.Context, id, deviceID ids.ID, at time.Time) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.MarkEnrollmentCodeUsed(ctx, sqlcgen.MarkEnrollmentCodeUsedParams{ID: uuid(id), DeviceID: uuid(deviceID), UsedAt: ts(at)})
	if err != nil {
		return false, mapError(err, "marking an enrollment code used")
	}
	return n == 1, nil
}

// toEnrollmentCode converts a row of enrollment_codes.
func toEnrollmentCode(row sqlcgen.EnrollmentCode) app.EnrollmentCodeRecord {
	rec := app.EnrollmentCodeRecord{
		ID: ids.ID(row.ID.Bytes), UserID: ids.ID(row.UserID.Bytes), Purpose: domain.EnrollmentPurpose(row.Purpose),
		Hash: row.CodeHash, ExpiresAt: row.ExpiresAt.Time.UTC(), Attempts: int(row.Attempts), CreatedAt: row.CreatedAt.Time.UTC(),
	}
	if row.CreatedBy.Valid {
		rec.CreatedBy = ids.ID(row.CreatedBy.Bytes)
	}
	if row.UsedAt.Valid {
		rec.UsedAt = row.UsedAt.Time.UTC()
	}
	if row.UsedByDeviceID.Valid {
		rec.UsedByDeviceID = ids.ID(row.UsedByDeviceID.Bytes)
	}
	return rec
}
