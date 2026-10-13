// SPDX-License-Identifier: Apache-2.0

// Package postgres is the PostgreSQL adapter of the identity module: the sqlc queries in
// queries/, the generated code in sqlcgen/ and Repository, which implements app.Store over the
// tables principals, users and owners (docs/spec/03-data-model.md section 4.2). Only this
// package runs SQL on those tables (03 "Principles" item 12).
//
// Example:
//
//	repo := postgres.NewRepository(pool)
//	dir := app.NewDirectory(repo)
package postgres

import (
	"bytes"
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
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Repository implements app.Store with the generated queries of sqlcgen. It is safe for
// concurrent use.
type Repository struct {
	pool *pg.Pool
}

// Compile-time check: Repository is the store of the identity application layer.
var _ app.Store = (*Repository)(nil)

// NewRepository returns the repository over pool. Reads outside a unit of work use pool; reads
// and writes inside one use its transaction (pg.Querier, pg.Tx).
func NewRepository(pool *pg.Pool) *Repository {
	return &Repository{pool: pool}
}

// read returns the queries for a read: the ambient transaction of ctx or the pool.
func (r *Repository) read(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(pg.Querier(ctx, r.pool))
}

// write returns the queries for a write: only the ambient transaction of ctx, because a write
// outside a unit of work would skip its phases (02 section 8.1).
func (r *Repository) write(ctx context.Context) (*sqlcgen.Queries, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return nil, err
	}
	return sqlcgen.New(tx), nil
}

// Principal implements app.Store.
func (r *Repository) Principal(ctx context.Context, id ids.ID) (app.PrincipalRecord, bool, error) {
	row, err := r.read(ctx).GetPrincipal(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.PrincipalRecord{}, false, nil
	}
	if err != nil {
		return app.PrincipalRecord{}, false, mapError(err, "reading a principal")
	}
	return app.PrincipalRecord{Principal: toPrincipal(row.Principal), IsOwner: row.IsOwner}, true, nil
}

// Principals implements app.Store.
func (r *Repository) Principals(ctx context.Context, list []ids.ID) ([]domain.Principal, error) {
	if len(list) == 0 {
		return nil, nil
	}
	keys := make([]pgtype.UUID, 0, len(list))
	for _, id := range slices.Compact(slices.SortedFunc(slices.Values(list), compareIDs)) {
		keys = append(keys, uuid(id))
	}
	rows, err := r.read(ctx).ListPrincipalsByIDs(ctx, keys)
	if err != nil {
		return nil, mapError(err, "reading principals")
	}
	out := make([]domain.Principal, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPrincipal(row.Principal))
	}
	return out, nil
}

// PrincipalByHandle implements app.Store.
func (r *Repository) PrincipalByHandle(ctx context.Context, handle string) (domain.Principal, bool, error) {
	row, err := r.read(ctx).GetPrincipalByHandle(ctx, handle)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Principal{}, false, nil
	}
	if err != nil {
		return domain.Principal{}, false, mapError(err, "reading a principal by handle")
	}
	return toPrincipal(row.Principal), true, nil
}

// User implements app.Store.
func (r *Repository) User(ctx context.Context, id ids.ID) (app.UserRecord, bool, error) {
	row, err := r.read(ctx).GetUser(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.UserRecord{}, false, nil
	}
	if err != nil {
		return app.UserRecord{}, false, mapError(err, "reading a person")
	}
	u := domain.User{
		Principal:    toPrincipal(row.Principal),
		Email:        row.User.Email.String,
		Locale:       row.User.Locale,
		TimeZone:     row.User.TimeZone,
		LastActiveAt: timeOf(row.User.LastActiveAt),
	}
	return app.UserRecord{User: u, IsOwner: row.IsOwner}, true, nil
}

// ActiveOwners implements app.Store.
func (r *Repository) ActiveOwners(ctx context.Context) ([]app.OwnerRecord, error) {
	rows, err := r.read(ctx).ListActiveOwners(ctx)
	if err != nil {
		return nil, mapError(err, "reading the active owners")
	}
	out := make([]app.OwnerRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.OwnerRecord{
			Owner: domain.Owner{
				ID:        idOf(row.Owner.ID),
				UserID:    idOf(row.Owner.UserID),
				GrantedBy: idOf(row.Owner.GrantedBy),
				GrantedAt: timeOf(row.Owner.GrantedAt),
				Reason:    row.Owner.Reason,
				RevokedAt: timeOf(row.Owner.RevokedAt),
				RevokedBy: idOf(row.Owner.RevokedBy),
			},
			User: toPrincipal(row.Principal),
		})
	}
	return out, nil
}

// ActiveOwnerIDs implements app.Store.
func (r *Repository) ActiveOwnerIDs(ctx context.Context) ([]ids.ID, error) {
	rows, err := r.read(ctx).ListActiveOwnerIDs(ctx)
	if err != nil {
		return nil, mapError(err, "reading the active owner ids")
	}
	out := make([]ids.ID, 0, len(rows))
	for _, u := range rows {
		out = append(out, idOf(u))
	}
	return out, nil
}

// InsertPrincipal implements app.Store.
func (r *Repository) InsertPrincipal(ctx context.Context, p *domain.Principal) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	created, err := q.InsertPrincipal(ctx, sqlcgen.InsertPrincipalParams{
		ID:           uuid(p.ID),
		Kind:         string(p.Kind),
		Status:       string(p.Status),
		Handle:       p.Handle.String(),
		DisplayName:  p.DisplayName.String(),
		AvatarFileID: optionalUUID(p.AvatarFileID),
	})
	if err != nil {
		return mapError(err, "inserting a principal")
	}
	p.CreatedAt = timeOf(created)
	return nil
}

// InsertUser implements app.Store.
func (r *Repository) InsertUser(ctx context.Context, u domain.User) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.InsertUser(ctx, sqlcgen.InsertUserParams{
		PrincipalID: uuid(u.Principal.ID),
		Email:       pgtype.Text{String: u.Email, Valid: u.Email != ""},
		Locale:      u.Locale,
		TimeZone:    u.TimeZone,
	})
	if err != nil {
		return mapError(err, "inserting a person")
	}
	return nil
}

// InsertOwner implements app.Store.
func (r *Repository) InsertOwner(ctx context.Context, o *domain.Owner) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	granted, err := q.InsertOwner(ctx, sqlcgen.InsertOwnerParams{
		ID:        uuid(o.ID),
		UserID:    uuid(o.UserID),
		GrantedBy: optionalUUID(o.GrantedBy),
		Reason:    o.Reason,
	})
	if err != nil {
		return mapError(err, "inserting an owner")
	}
	o.GrantedAt = timeOf(granted)
	return nil
}

// toPrincipal converts a row of principals. The values come from the database, whose checks
// hold the rules of the domain, so they are wrapped without a second validation.
func toPrincipal(p sqlcgen.Principal) domain.Principal {
	return domain.Principal{
		ID:           idOf(p.ID),
		Kind:         domain.Kind(p.Kind),
		Status:       domain.Status(p.Status),
		Handle:       domain.HandleFromStore(p.Handle),
		DisplayName:  domain.DisplayNameFromStore(p.DisplayName),
		AvatarFileID: idOf(p.AvatarFileID),
		CreatedAt:    timeOf(p.CreatedAt),
		DisabledAt:   timeOf(p.DisabledAt),
		DeletedAt:    timeOf(p.DeletedAt),
	}
}

// uuid converts an id for a parameter.
func uuid(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

// optionalUUID converts an optional id: the zero ID becomes NULL.
func optionalUUID(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: !id.IsZero()} }

// idOf converts a column; NULL becomes the zero ID.
func idOf(u pgtype.UUID) ids.ID {
	if !u.Valid {
		return ids.ID{}
	}
	return ids.ID(u.Bytes)
}

// timeOf converts a column; NULL becomes the zero time. Times are UTC (03 "Principles" item 2).
func timeOf(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time.UTC()
}

// compareIDs orders ids by their bytes, the creation order of UUIDv7.
func compareIDs(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) }
