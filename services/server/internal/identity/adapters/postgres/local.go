// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the local credential store (M1-39).
var _ app.LocalCredentialStore = (*Repository)(nil)

// LockLocalCredential implements app.LocalCredentialStore.
func (r *Repository) LockLocalCredential(ctx context.Context, identityID ids.ID) (app.LocalCredentialRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.LocalCredentialRecord{}, false, err
	}
	row, err := q.LockLocalCredential(ctx, uuid(identityID))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.LocalCredentialRecord{}, false, nil
	}
	if err != nil {
		return app.LocalCredentialRecord{}, false, mapError(err, "locking local credentials")
	}
	return toLocalCredential(row), true, nil
}

// LocalCredential implements app.LocalCredentialStore.
func (r *Repository) LocalCredential(ctx context.Context, identityID ids.ID) (app.LocalCredentialRecord, bool, error) {
	row, err := r.read(ctx).GetLocalCredential(ctx, uuid(identityID))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.LocalCredentialRecord{}, false, nil
	}
	if err != nil {
		return app.LocalCredentialRecord{}, false, mapError(err, "reading local credentials")
	}
	return toLocalCredential(row), true, nil
}

// SaveLocalCredential implements app.LocalCredentialStore.
func (r *Repository) SaveLocalCredential(ctx context.Context, rec app.LocalCredentialRecord) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	p := sqlcgen.SaveLocalCredentialParams{
		IdentityID: uuid(rec.IdentityID), PasswordHash: rec.PasswordHash, PasswordChangedAt: ts(rec.PasswordChangedAt),
		TotpSecretCiphertext: rec.TOTPCiphertext, TotpKeyRef: optionalText(rec.TOTPKeyRef),
		TotpEnabledAt: optionalTS(rec.TOTPEnabledAt), FailedCount: int32(rec.Lockout.FailedCount), //nolint:gosec // G115: bounded by auth.login_fail_limit.
		FailWindowStartedAt: optionalTS(rec.Lockout.WindowStartedAt), LockedUntil: optionalTS(rec.Lockout.LockedUntil),
	}
	if rec.TOTPLastStep != nil {
		p.TotpLastStep = pgtype.Int8{Int64: *rec.TOTPLastStep, Valid: true}
	}
	return mapError(q.SaveLocalCredential(ctx, p), "saving local credentials")
}

// LocalCredentialOfUser implements app.LocalCredentialStore.
func (r *Repository) LocalCredentialOfUser(ctx context.Context, userID ids.ID) (app.LocalCredentialRecord, bool, error) {
	row, err := r.read(ctx).LocalCredentialOfUser(ctx, uuid(userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.LocalCredentialRecord{}, false, nil
	}
	if err != nil {
		return app.LocalCredentialRecord{}, false, mapError(err, "reading local credentials")
	}
	return toLocalCredential(row), true, nil
}

// HasEstablishedLocalCredential implements app.Store: a row of an active identity of userID with
// a confirmed TOTP enrollment.
func (r *Repository) HasEstablishedLocalCredential(ctx context.Context, userID ids.ID) (bool, error) {
	ok, err := r.read(ctx).HasEstablishedLocalCredential(ctx, uuid(userID))
	if err != nil {
		return false, mapError(err, "reading local credentials")
	}
	return ok, nil
}

// toLocalCredential converts a row of local_credentials.
func toLocalCredential(row sqlcgen.LocalCredential) app.LocalCredentialRecord {
	rec := app.LocalCredentialRecord{
		IdentityID: idOf(row.IdentityID), PasswordHash: row.PasswordHash, PasswordChangedAt: timeOf(row.PasswordChangedAt),
		TOTPCiphertext: row.TotpSecretCiphertext, TOTPKeyRef: row.TotpKeyRef.String, TOTPEnabledAt: timeOf(row.TotpEnabledAt),
		Lockout: domain.Lockout{
			FailedCount: int(row.FailedCount), WindowStartedAt: timeOf(row.FailWindowStartedAt), LockedUntil: timeOf(row.LockedUntil),
		},
	}
	if row.TotpLastStep.Valid {
		step := row.TotpLastStep.Int64
		rec.TOTPLastStep = &step
	}
	return rec
}
