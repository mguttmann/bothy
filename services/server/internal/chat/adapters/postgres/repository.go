// SPDX-License-Identifier: Apache-2.0

// Package postgres is the PostgreSQL adapter of the chat module (M1-52): the sqlc queries in
// queries/, the generated code in sqlcgen/ and Repository, which implements app.Store over the
// tables spaces and user_space_states (docs/spec/03-datenmodell.md section 4.5). Only this package
// runs SQL on them (03 Grundsatz 12).
//
// Example:
//
//	repo := postgres.NewRepository(pool)
//	svc, err := app.New(cfg, app.Deps{Store: repo, ...})
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Repository implements app.Store with the generated queries of sqlcgen. It is safe for
// concurrent use.
type Repository struct {
	pool *pg.Pool
}

// Compile-time check: Repository is the store of the chat application layer.
var _ app.Store = (*Repository)(nil)

// NewRepository returns the repository over pool. Reads outside a unit of work use pool; reads
// and writes inside one use its transaction.
func NewRepository(pool *pg.Pool) *Repository { return &Repository{pool: pool} }

// read returns the queries for a read: the ambient transaction of ctx or the pool.
func (r *Repository) read(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(pg.Querier(ctx, r.pool))
}

// write returns the queries for a write or a lock: only the ambient transaction of ctx.
func (r *Repository) write(ctx context.Context) (*sqlcgen.Queries, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return nil, err
	}
	return sqlcgen.New(tx), nil
}

// Space implements app.Store.
func (r *Repository) Space(ctx context.Context, id ids.ID) (domain.Space, bool, error) {
	row, err := r.read(ctx).GetSpace(ctx, uuid(id))
	return one(row, err, "reading a space")
}

// LockSpace implements app.Store.
func (r *Repository) LockSpace(ctx context.Context, id ids.ID) (domain.Space, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.Space{}, false, err
	}
	row, err := q.LockSpace(ctx, uuid(id))
	return one(row, err, "locking a space")
}

// orderLockKey is the transaction advisory lock that create and move take before the row locks:
// with no space that is not archived there is no row to lock, and two first creates would compute
// the same key (docs/DECISIONS.md M1-52/3). Its value is arbitrary but fixed and differs from
// every other advisory lock of the server.
const orderLockKey int64 = 0x626f746879737063

// LockActiveSpaces implements app.Store: the order lock, then every space that is not archived in
// ascending id.
func (r *Repository) LockActiveSpaces(ctx context.Context) error {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", orderLockKey); err != nil {
		return mapError(err, "locking the order of the spaces")
	}
	_, err = sqlcgen.New(tx).LockActiveSpaces(ctx)
	return mapError(err, "locking the spaces")
}

// ActiveOrder implements app.Store.
func (r *Repository) ActiveOrder(ctx context.Context) ([]app.SpaceKey, error) {
	rows, err := r.read(ctx).ActiveSpaceKeys(ctx)
	if err != nil {
		return nil, mapError(err, "reading the order of the spaces")
	}
	out := make([]app.SpaceKey, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.SpaceKey{ID: idOf(row.ID), SortKey: row.SortKey})
	}
	return out, nil
}

// VisibleSpaces implements app.Store.
func (r *Repository) VisibleSpaces(ctx context.Context, all bool, visible []ids.ID) ([]domain.Space, error) {
	keys := make([]pgtype.UUID, 0, len(visible))
	for _, id := range visible {
		keys = append(keys, uuid(id))
	}
	rows, err := r.read(ctx).ListVisibleSpaces(ctx, sqlcgen.ListVisibleSpacesParams{AllSpaces: all, Visible: keys})
	if err != nil {
		return nil, mapError(err, "listing spaces")
	}
	out := make([]domain.Space, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSpace(row))
	}
	return out, nil
}

// SpaceIDs implements app.Store.
func (r *Repository) SpaceIDs(ctx context.Context) ([]ids.ID, error) {
	rows, err := r.read(ctx).SpaceIDs(ctx)
	if err != nil {
		return nil, mapError(err, "reading the spaces")
	}
	out := make([]ids.ID, 0, len(rows))
	for _, row := range rows {
		out = append(out, idOf(row))
	}
	return out, nil
}

// InsertSpace implements app.Store.
func (r *Repository) InsertSpace(ctx context.Context, s domain.Space) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.InsertSpace(ctx, sqlcgen.InsertSpaceParams{
		ID: uuid(s.ID), Name: s.Name, Icon: optionalText(s.Icon), Color: optionalText(s.Color), SortKey: s.SortKey,
		CreatedBy: uuid(s.CreatedBy), CreatedAt: pgtype.Timestamptz{Time: s.CreatedAt, Valid: true}, Version: s.Version,
	})
	return mapError(err, "inserting a space")
}

// UpdateSpace implements app.Store.
func (r *Repository) UpdateSpace(ctx context.Context, s domain.Space) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.UpdateSpace(ctx, sqlcgen.UpdateSpaceParams{
		ID: uuid(s.ID), Name: s.Name, Icon: optionalText(s.Icon), Color: optionalText(s.Color), Version: s.Version,
	})
	return mapError(err, "updating a space")
}

// MoveSpace implements app.Store.
func (r *Repository) MoveSpace(ctx context.Context, s domain.Space) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.MoveSpace(ctx, sqlcgen.MoveSpaceParams{ID: uuid(s.ID), SortKey: s.SortKey, Version: s.Version}), "moving a space")
}

// ArchiveSpace implements app.Store.
func (r *Repository) ArchiveSpace(ctx context.Context, s domain.Space) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.ArchiveSpace(ctx, sqlcgen.ArchiveSpaceParams{
		ID: uuid(s.ID), ArchivedAt: pgtype.Timestamptz{Time: s.ArchivedAt, Valid: true}, Version: s.Version,
	})
	return mapError(err, "archiving a space")
}

// PutSidebarState implements app.Store.
func (r *Repository) PutSidebarState(ctx context.Context, st domain.SidebarState, keepKey bool) (domain.SidebarState, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.SidebarState{}, err
	}
	row, err := q.UpsertSidebarState(ctx, sqlcgen.UpsertSidebarStateParams{
		UserID: uuid(st.UserID), SpaceID: uuid(st.SpaceID), Collapsed: st.Collapsed, SortKey: optionalText(st.SortKey),
		UpdatedByDeviceID: optionalUUID(st.UpdatedByDeviceID), KeepKey: keepKey,
	})
	if err != nil {
		return domain.SidebarState{}, mapError(err, "writing a sidebar state")
	}
	return toSidebar(row), nil
}

// SidebarStates implements app.Store.
func (r *Repository) SidebarStates(ctx context.Context, user ids.ID) ([]domain.SidebarState, error) {
	rows, err := r.read(ctx).ListSidebarStates(ctx, uuid(user))
	if err != nil {
		return nil, mapError(err, "reading the sidebar states")
	}
	out := make([]domain.SidebarState, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSidebar(row))
	}
	return out, nil
}

// one converts the result of a single-row query; no rows is found false.
func one(row sqlcgen.Space, err error, what string) (domain.Space, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Space{}, false, nil
	}
	if err != nil {
		return domain.Space{}, false, mapError(err, what)
	}
	return toSpace(row), true, nil
}

// toSpace converts a row.
func toSpace(row sqlcgen.Space) domain.Space {
	return domain.Space{
		ID: idOf(row.ID), Name: row.Name, Icon: row.Icon.String, Color: row.Color.String, SortKey: row.SortKey,
		DefaultOperatorAgentID: idOf(row.DefaultOperatorAgentID), CreatedBy: idOf(row.CreatedBy), CreatedAt: timeOf(row.CreatedAt),
		ArchivedAt: timeOf(row.ArchivedAt), Version: row.Version,
	}
}

// toSidebar converts a row.
func toSidebar(row sqlcgen.UserSpaceState) domain.SidebarState {
	return domain.SidebarState{
		UserID: idOf(row.UserID), SpaceID: idOf(row.SpaceID), Collapsed: row.Collapsed, SortKey: row.SortKey.String,
		Version: row.Version, UpdatedByDeviceID: idOf(row.UpdatedByDeviceID),
	}
}

// SQLSTATE and constraint the adapter maps to a client error.
const (
	sqlstateUniqueViolation = "23505"
	nameConstraint          = "spaces_name_active_key"
)

// mapError turns a database error of the operation what into a kernel/errs error: a taken name is
// 409 conflict on /name (never SQL text, a constraint name or a value), a kernel/errs error passes,
// anything else is internal.
func mapError(err error, what string) error {
	if err == nil {
		return nil
	}
	if _, ok := errs.As(err); ok {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == sqlstateUniqueViolation && pgErr.ConstraintName == nameConstraint {
		return errs.Wrap(errs.CodeConflict, err, "a space with this name exists").
			WithFields(errs.FieldError{Pointer: "/name", Code: "taken", Detail: "already taken"})
	}
	return errs.Wrap(errs.CodeInternalError, err, "chat: "+what+" failed")
}

// uuid converts an id for a parameter.
func uuid(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

// optionalUUID converts an optional id: the zero ID becomes NULL.
func optionalUUID(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: !id.IsZero()} }

// optionalText converts an optional text: "" becomes NULL.
func optionalText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

// idOf converts a column; NULL becomes the zero ID.
func idOf(u pgtype.UUID) ids.ID {
	if !u.Valid {
		return ids.ID{}
	}
	return ids.ID(u.Bytes)
}

// timeOf converts a column; NULL becomes the zero time. Times are UTC (03 Grundsatz 2).
func timeOf(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time.UTC()
}
