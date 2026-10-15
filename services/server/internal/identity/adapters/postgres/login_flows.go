// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/oidcflow"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the login flow store (M1-41).
var _ app.LoginFlowStore = (*Repository)(nil)

// InsertLoginFlow implements app.LoginFlowStore.
func (r *Repository) InsertLoginFlow(ctx context.Context, rec app.LoginFlowRecord) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.InsertLoginFlow(ctx, sqlcgen.InsertLoginFlowParams{
		ID: uuid(rec.ID), ProviderID: uuid(rec.ProviderID), Mode: string(rec.Mode), UserID: optionalUUID(rec.UserID),
		OidcStateHash: rec.StateHash, OidcNonceHash: rec.NonceHash, ClientChallenge: rec.ClientChallenge,
		AppRedirectUri: rec.AppRedirectURI, EnrollmentCodeID: optionalUUID(rec.EnrollmentCodeID), CreatedAt: ts(rec.CreatedAt),
		ExpiresAt: ts(rec.ExpiresAt),
	})
	return mapError(err, "inserting a login flow")
}

// LoginFlowByStateHash implements app.LoginFlowStore.
func (r *Repository) LoginFlowByStateHash(ctx context.Context, hash []byte) (app.LoginFlowRecord, bool, error) {
	row, err := r.read(ctx).LoginFlowByStateHash(ctx, hash)
	return flowResult(row, err, "reading a login flow")
}

// LockLoginFlowByCodeHash implements app.LoginFlowStore.
func (r *Repository) LockLoginFlowByCodeHash(ctx context.Context, hash []byte) (app.LoginFlowRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.LoginFlowRecord{}, false, err
	}
	row, err := q.LockLoginFlowByCodeHash(ctx, hash)
	return flowResult(row, err, "locking a login flow")
}

// flowResult converts the result of a single-row flow query.
func flowResult(row sqlcgen.AuthLoginFlow, err error, what string) (app.LoginFlowRecord, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.LoginFlowRecord{}, false, nil
	}
	if err != nil {
		return app.LoginFlowRecord{}, false, mapError(err, what)
	}
	rec, err := toLoginFlow(row)
	return rec, err == nil, err
}

// IssueLoginCode implements app.LoginFlowStore.
func (r *Repository) IssueLoginCode(ctx context.Context, id ids.ID, ext oidcflow.ExternalIdentity, codeHash []byte, codeExpiresAt time.Time) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	doc, err := ext.MarshalJSON()
	if err != nil {
		return false, errs.Wrap(errs.CodeInternalError, err, "identity: encoding the external identity")
	}
	n, err := q.IssueLoginCode(ctx, sqlcgen.IssueLoginCodeParams{
		ID: uuid(id), ExternalIdentity: doc, LoginCodeHash: codeHash, LoginCodeExpiresAt: ts(codeExpiresAt),
	})
	if err != nil {
		return false, mapError(err, "issuing a login code")
	}
	return n == 1, nil
}

// FailLoginFlow implements app.LoginFlowStore.
func (r *Repository) FailLoginFlow(ctx context.Context, id ids.ID, code string) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.FailLoginFlow(ctx, sqlcgen.FailLoginFlowParams{ID: uuid(id), Error: optionalText(code)})
	if err != nil {
		return false, mapError(err, "failing a login flow")
	}
	return n == 1, nil
}

// CompleteLoginFlow implements app.LoginFlowStore.
func (r *Repository) CompleteLoginFlow(ctx context.Context, id ids.ID) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.CompleteLoginFlow(ctx, uuid(id))
	if err != nil {
		return false, mapError(err, "completing a login flow")
	}
	return n == 1, nil
}

// DeleteExpiredLoginFlows implements app.LoginFlowStore.
func (r *Repository) DeleteExpiredLoginFlows(ctx context.Context, now time.Time) (int64, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, err
	}
	n, err := q.DeleteExpiredLoginFlows(ctx, ts(now))
	return n, mapError(err, "deleting expired login flows")
}

// toLoginFlow converts a row of auth_login_flows; a stored external identity that breaks its
// schema is an internal error.
func toLoginFlow(row sqlcgen.AuthLoginFlow) (app.LoginFlowRecord, error) {
	rec := app.LoginFlowRecord{
		ID: ids.ID(row.ID.Bytes), ProviderID: ids.ID(row.ProviderID.Bytes), Mode: oidcflow.Mode(row.Mode),
		StateHash: row.OidcStateHash, NonceHash: row.OidcNonceHash, ClientChallenge: row.ClientChallenge,
		AppRedirectURI: row.AppRedirectUri, LoginCodeHash: row.LoginCodeHash, Status: oidcflow.Status(row.Status),
		Error: row.Error.String, CreatedAt: row.CreatedAt.Time.UTC(), ExpiresAt: row.ExpiresAt.Time.UTC(),
	}
	if row.UserID.Valid {
		rec.UserID = ids.ID(row.UserID.Bytes)
	}
	if row.EnrollmentCodeID.Valid {
		rec.EnrollmentCodeID = ids.ID(row.EnrollmentCodeID.Bytes)
	}
	if row.LoginCodeExpiresAt.Valid {
		rec.LoginCodeExpiresAt = row.LoginCodeExpiresAt.Time.UTC()
	}
	if row.ExternalIdentity != nil {
		ext, err := oidcflow.ParseExternalIdentity(row.ExternalIdentity)
		if err != nil {
			return app.LoginFlowRecord{}, errs.Wrap(errs.CodeInternalError, err, "identity: reading a login flow")
		}
		rec.External = &ext
	}
	return rec, nil
}
