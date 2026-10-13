// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the API token store of the identity application layer.
var _ app.APITokenStore = (*Repository)(nil)

// InsertAPIToken implements app.APITokenStore. A token_hash that exists already (a collision of
// 256 random bits, practically impossible) stays internal, like a reused primary key: only a
// broken random source produces it, and the error carries neither the value nor the constraint.
func (r *Repository) InsertAPIToken(ctx context.Context, t app.APITokenRecord) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	scopes := t.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	err = q.InsertAPIToken(ctx, sqlcgen.InsertAPITokenParams{
		ID: uuid(t.ID), PrincipalID: uuid(t.PrincipalID), Name: t.Name, TokenPrefix: t.Prefix, TokenHash: t.Hash,
		Scopes: scopes, DpopJkt: pgtype.Text{String: t.DPoPJKT, Valid: t.DPoPJKT != ""}, ExpiresAt: ts(t.ExpiresAt),
		CreatedBy: uuid(t.CreatedBy), CreatedAt: ts(t.CreatedAt),
	})
	return mapError(err, "inserting an API token")
}

// APIToken implements app.APITokenStore.
func (r *Repository) APIToken(ctx context.Context, id ids.ID) (app.APITokenRecord, bool, error) {
	row, err := r.read(ctx).GetAPIToken(ctx, uuid(id))
	return apiTokenResult(row, err, "reading an API token")
}

// LockAPIToken implements app.APITokenStore.
func (r *Repository) LockAPIToken(ctx context.Context, id ids.ID) (app.APITokenRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.APITokenRecord{}, false, err
	}
	row, err := q.LockAPIToken(ctx, uuid(id))
	return apiTokenResult(row, err, "locking an API token")
}

// APITokenByHash implements app.APITokenStore.
func (r *Repository) APITokenByHash(ctx context.Context, hash []byte) (app.APITokenAuthRecord, bool, error) {
	row, err := r.read(ctx).GetAPITokenByHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.APITokenAuthRecord{}, false, nil
	}
	if err != nil {
		return app.APITokenAuthRecord{}, false, mapError(err, "reading an API token by hash")
	}
	return app.APITokenAuthRecord{APITokenRecord: toAPIToken(row.ApiToken), PrincipalKind: domain.Kind(row.PrincipalKind)}, true, nil
}

// ActiveAPITokens implements app.APITokenStore.
func (r *Repository) ActiveAPITokens(ctx context.Context, principalID ids.ID, afterCreated time.Time, afterID ids.ID, limit int) ([]app.APITokenRecord, error) {
	rows, err := r.read(ctx).ListActiveAPITokens(ctx, sqlcgen.ListActiveAPITokensParams{
		PrincipalID: uuid(principalID), AfterID: optionalUUID(afterID), AfterCreated: ts(afterCreated), RowLimit: int32(min(limit, 1<<30)), //nolint:gosec // G115: capped above
	})
	if err != nil {
		return nil, mapError(err, "listing API tokens")
	}
	out := make([]app.APITokenRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAPIToken(row))
	}
	return out, nil
}

// RevokeAPIToken implements app.APITokenStore.
func (r *Repository) RevokeAPIToken(ctx context.Context, id, by ids.ID, at time.Time) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.RevokeAPIToken(ctx, sqlcgen.RevokeAPITokenParams{ID: uuid(id), RevokedBy: optionalUUID(by), RevokedAt: ts(at)})
	if err != nil {
		return false, mapError(err, "revoking an API token")
	}
	return n > 0, nil
}

// RevokeAPITokensOfPrincipal implements app.APITokenStore.
func (r *Repository) RevokeAPITokensOfPrincipal(ctx context.Context, principalID, by ids.ID, at time.Time) (int, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, err
	}
	n, err := q.RevokeAPITokensOfPrincipal(ctx, sqlcgen.RevokeAPITokensOfPrincipalParams{
		PrincipalID: uuid(principalID), RevokedBy: optionalUUID(by), RevokedAt: ts(at),
	})
	if err != nil {
		return 0, mapError(err, "revoking the API tokens of a principal")
	}
	return int(n), nil
}

// TouchAPIToken implements app.APITokenStore. It runs on the pool, never in the request's unit of
// work, so a failure cannot roll back the request.
func (r *Repository) TouchAPIToken(ctx context.Context, id ids.ID, at, threshold time.Time) error {
	err := sqlcgen.New(r.pool).TouchAPIToken(ctx, sqlcgen.TouchAPITokenParams{ID: uuid(id), UsedAt: ts(at), Threshold: ts(threshold)})
	return mapError(err, "recording API token use")
}

// apiTokenResult converts the result of a single-row token query.
func apiTokenResult(row sqlcgen.ApiToken, err error, what string) (app.APITokenRecord, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.APITokenRecord{}, false, nil
	}
	if err != nil {
		return app.APITokenRecord{}, false, mapError(err, what)
	}
	return toAPIToken(row), true, nil
}

// toAPIToken converts a row.
func toAPIToken(t sqlcgen.ApiToken) app.APITokenRecord {
	return app.APITokenRecord{
		ID: idOf(t.ID), PrincipalID: idOf(t.PrincipalID), Name: t.Name, Prefix: t.TokenPrefix, Hash: t.TokenHash,
		Scopes: slices.Clone(t.Scopes), DPoPJKT: t.DpopJkt.String, ExpiresAt: timeOf(t.ExpiresAt), CreatedBy: idOf(t.CreatedBy),
		CreatedAt: timeOf(t.CreatedAt), LastUsedAt: timeOf(t.LastUsedAt), RevokedAt: timeOf(t.RevokedAt), RevokedBy: idOf(t.RevokedBy),
	}
}
