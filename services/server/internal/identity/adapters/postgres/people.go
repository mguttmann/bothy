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

// ListUsers implements app.Store.
func (r *Repository) ListUsers(ctx context.Context, f app.UserFilter) ([]app.UserRecord, error) {
	p := sqlcgen.ListUsersParams{
		Status:   pgtype.Text{String: string(f.Status), Valid: f.Status != ""},
		Q:        pgtype.Text{String: f.Q, Valid: f.Q != ""},
		RowLimit: int32(min(max(f.Limit, 1), 1<<30)),
	}
	if f.AfterHandle != "" {
		p.AfterHandle, p.AfterID = pgtype.Text{String: f.AfterHandle, Valid: true}, uuid(f.AfterID)
	}
	rows, err := r.read(ctx).ListUsers(ctx, p)
	if err != nil {
		return nil, mapError(err, "listing people")
	}
	out := make([]app.UserRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, userRecord(row.Principal, row.User, row.IsOwner))
	}
	return out, nil
}

// LockUser implements app.Store.
func (r *Repository) LockUser(ctx context.Context, id ids.ID) (app.UserRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.UserRecord{}, false, err
	}
	row, err := q.LockUser(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.UserRecord{}, false, nil
	}
	if err != nil {
		return app.UserRecord{}, false, mapError(err, "locking a person")
	}
	return userRecord(row.Principal, row.User, row.IsOwner), true, nil
}

// UpdateUser implements app.Store.
func (r *Repository) UpdateUser(ctx context.Context, u domain.User) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	if err := q.UpdatePrincipalDisplayName(ctx, sqlcgen.UpdatePrincipalDisplayNameParams{
		ID: uuid(u.Principal.ID), DisplayName: u.Principal.DisplayName.String(),
	}); err != nil {
		return mapError(err, "updating a principal")
	}
	err = q.UpdateUserProfile(ctx, sqlcgen.UpdateUserProfileParams{
		PrincipalID: uuid(u.Principal.ID), Email: pgtype.Text{String: u.Email, Valid: u.Email != ""},
		Locale: u.Locale, TimeZone: u.TimeZone,
	})
	if err != nil {
		return mapError(err, "updating a person")
	}
	return nil
}

// EnableUser implements app.Store.
func (r *Repository) EnableUser(ctx context.Context, id ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	if err := q.EnablePrincipal(ctx, uuid(id)); err != nil {
		return mapError(err, "enabling a person")
	}
	return nil
}

// LockActiveOwners implements app.Store.
func (r *Repository) LockActiveOwners(ctx context.Context) ([]ids.ID, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockActiveOwners(ctx)
	if err != nil {
		return nil, mapError(err, "locking the active owners")
	}
	out := make([]ids.ID, 0, len(rows))
	for _, row := range rows {
		out = append(out, idOf(row.UserID))
	}
	return out, nil
}

// ActiveOwner implements app.Store.
func (r *Repository) ActiveOwner(ctx context.Context, userID ids.ID) (app.OwnerRecord, bool, error) {
	row, err := r.read(ctx).GetActiveOwner(ctx, uuid(userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.OwnerRecord{}, false, nil
	}
	if err != nil {
		return app.OwnerRecord{}, false, mapError(err, "reading an owner")
	}
	return app.OwnerRecord{Owner: toOwner(row.Owner), User: toPrincipal(row.Principal)}, true, nil
}

// RevokeOwner implements app.Store.
func (r *Repository) RevokeOwner(ctx context.Context, userID, revokedBy ids.ID) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.RevokeOwner(ctx, sqlcgen.RevokeOwnerParams{UserID: uuid(userID), RevokedBy: optionalUUID(revokedBy)})
	if err != nil {
		return false, mapError(err, "revoking an owner")
	}
	return n > 0, nil
}

// userRecord converts a joined row of principals and users.
func userRecord(p sqlcgen.Principal, u sqlcgen.User, isOwner bool) app.UserRecord {
	return app.UserRecord{User: domain.User{
		Principal: toPrincipal(p), Email: u.Email.String, Locale: u.Locale, TimeZone: u.TimeZone,
		LastActiveAt: timeOf(u.LastActiveAt),
	}, IsOwner: isOwner}
}

// toOwner converts a row of owners.
func toOwner(o sqlcgen.Owner) domain.Owner {
	return domain.Owner{
		ID: idOf(o.ID), UserID: idOf(o.UserID), GrantedBy: idOf(o.GrantedBy), GrantedAt: timeOf(o.GrantedAt),
		Reason: o.Reason, RevokedAt: timeOf(o.RevokedAt), RevokedBy: idOf(o.RevokedBy),
	}
}
