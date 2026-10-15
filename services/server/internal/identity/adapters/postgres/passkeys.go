// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the passkey store (M1-40).
var _ app.PasskeyStore = (*Repository)(nil)

// InsertPasskey implements app.PasskeyStore.
func (r *Repository) InsertPasskey(ctx context.Context, p app.StoredPasskey) (app.StoredPasskey, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.StoredPasskey{}, err
	}
	transports := p.Transports
	if transports == nil {
		transports = []string{}
	}
	row, err := q.InsertPasskey(ctx, sqlcgen.InsertPasskeyParams{
		ID: uuid(p.ID), UserID: uuid(p.UserID), CredentialID: p.CredentialID, PublicKeyCose: p.PublicKeyCOSE,
		SignCount: int64(p.SignCount), Aaguid: optionalUUID(p.AAGUID), Transports: transports, BackupEligible: p.BackupEligible,
		BackupState: p.BackupState, Name: p.Name, CreatedAt: ts(p.CreatedAt),
	})
	if err != nil {
		return app.StoredPasskey{}, mapError(err, "inserting a passkey")
	}
	return toPasskey(row), nil
}

// ActivePasskeysOfUser implements app.PasskeyStore.
func (r *Repository) ActivePasskeysOfUser(ctx context.Context, userID ids.ID) ([]app.StoredPasskey, error) {
	rows, err := r.read(ctx).ActivePasskeysOfUser(ctx, uuid(userID))
	if err != nil {
		return nil, mapError(err, "reading the passkeys of a person")
	}
	out := make([]app.StoredPasskey, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPasskey(row))
	}
	return out, nil
}

// PasskeyByCredentialID implements app.PasskeyStore.
func (r *Repository) PasskeyByCredentialID(ctx context.Context, credentialID []byte) (app.StoredPasskey, bool, error) {
	row, err := r.read(ctx).PasskeyByCredentialID(ctx, credentialID)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.StoredPasskey{}, false, nil
	}
	if err != nil {
		return app.StoredPasskey{}, false, mapError(err, "reading a passkey")
	}
	return toPasskey(row), true, nil
}

// LockPasskey implements app.PasskeyStore.
func (r *Repository) LockPasskey(ctx context.Context, id ids.ID) (app.StoredPasskey, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.StoredPasskey{}, false, err
	}
	row, err := q.LockPasskey(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.StoredPasskey{}, false, nil
	}
	if err != nil {
		return app.StoredPasskey{}, false, mapError(err, "locking a passkey")
	}
	return toPasskey(row), true, nil
}

// UpdatePasskeyUse implements app.PasskeyStore.
func (r *Repository) UpdatePasskeyUse(ctx context.Context, id ids.ID, u app.PasskeyUse) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.UpdatePasskeyUse(ctx, sqlcgen.UpdatePasskeyUseParams{
		ID: uuid(id), SignCount: int64(u.SignCount), BackupState: u.BackupState, LastUsedAt: ts(u.At),
	})
	return mapError(err, "updating a passkey after its use")
}

// RevokePasskey implements app.PasskeyStore.
func (r *Repository) RevokePasskey(ctx context.Context, id ids.ID, at time.Time) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.RevokePasskey(ctx, sqlcgen.RevokePasskeyParams{ID: uuid(id), RevokedAt: ts(at)})
	if err != nil {
		return false, mapError(err, "revoking a passkey")
	}
	return n == 1, nil
}

// CountActivePasskeys implements app.PasskeyStore.
func (r *Repository) CountActivePasskeys(ctx context.Context, userID ids.ID) (int, error) {
	n, err := r.read(ctx).CountActivePasskeys(ctx, uuid(userID))
	if err != nil {
		return 0, mapError(err, "counting the passkeys of a person")
	}
	return int(n), nil
}

// HasActivePasskey reports whether userID has a passkey with revoked_at IS NULL (the source of
// authz.PrincipalSource.HasActivePasskey, M1-40).
func (r *Repository) HasActivePasskey(ctx context.Context, userID ids.ID) (bool, error) {
	n, err := r.CountActivePasskeys(ctx, userID)
	return n > 0, err
}

// toPasskey converts a row of passkey_credentials. sign_count is a WebAuthn counter of 32 bits;
// the column is bigint, so the conversion never loses a value the server wrote.
func toPasskey(row sqlcgen.PasskeyCredential) app.StoredPasskey {
	return app.StoredPasskey{
		ID: idOf(row.ID), UserID: idOf(row.UserID), CredentialID: row.CredentialID, PublicKeyCOSE: row.PublicKeyCose,
		SignCount: uint32(min(max(row.SignCount, 0), math.MaxUint32)), AAGUID: idOf(row.Aaguid), Transports: row.Transports,
		BackupEligible: row.BackupEligible, BackupState: row.BackupState, Name: row.Name, CreatedAt: timeOf(row.CreatedAt),
		LastUsedAt: timeOf(row.LastUsedAt), RevokedAt: timeOf(row.RevokedAt),
	}
}
