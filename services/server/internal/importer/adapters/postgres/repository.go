// SPDX-License-Identifier: Apache-2.0

// Package postgres is the PostgreSQL adapter of the module importer: the sqlc queries in
// queries/, the generated code in sqlcgen/ and Repository, which implements app.Store over
// import_sources and import_items (docs/spec/03-datenmodell.md section 4.11). Only this package
// runs SQL on those tables (03 Grundsatz 12).
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
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/importer"
	"github.com/OWNER/bothy/services/server/internal/importer/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/importer/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Repository implements app.Store with the generated queries of sqlcgen. It is safe for
// concurrent use.
type Repository struct {
	pool *pg.Pool
}

// Compile-time check: Repository is the store of the use cases.
var _ app.Store = (*Repository)(nil)

// NewRepository returns the repository over pool.
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

// mapError turns a database error of the operation what into a kernel/errs error; the text
// never contains SQL or values.
func mapError(err error, what string) error {
	if err == nil {
		return nil
	}
	if _, ok := errs.As(err); ok {
		return err
	}
	return errs.Wrap(errs.CodeInternalError, err, "importer: "+what+" failed")
}

// one maps a query for one row: no row is found false.
func one[R, T any](row R, err error, conv func(R) T, what string) (T, bool, error) {
	var zero T
	if errors.Is(err, pgx.ErrNoRows) {
		return zero, false, nil
	}
	if err != nil {
		return zero, false, mapError(err, what)
	}
	return conv(row), true, nil
}

func uuid(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func optionalUUID(id ids.ID) pgtype.UUID {
	if id.IsZero() {
		return pgtype.UUID{}
	}
	return uuid(id)
}

func stamp(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func optionalStamp(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return stamp(t)
}

func optionalText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

func fromUUID(u pgtype.UUID) ids.ID {
	if !u.Valid {
		return ids.ID{}
	}
	return ids.ID(u.Bytes)
}

func fromStamp(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time.UTC()
}

// toSource converts a row of import_sources.
func toSource(s sqlcgen.ImportSource) importer.Source {
	return importer.Source{
		ID: fromUUID(s.ID), Kind: s.Kind, Name: s.Name, Config: s.Config, State: s.State, Stats: s.Stats, Cursor: s.Cursor,
		CreatedBy: fromUUID(s.CreatedBy), CreatedAt: fromStamp(s.CreatedAt), StartedAt: fromStamp(s.StartedAt), FinishedAt: fromStamp(s.FinishedAt),
	}
}

// toItem converts a row of import_items.
func toItem(i sqlcgen.ImportItem) app.ItemRow {
	return app.ItemRow{
		ID: fromUUID(i.ID), SourceID: fromUUID(i.SourceID), OriginKey: i.OriginKey, ItemKind: i.ItemKind, TargetKind: i.TargetKind.String,
		TargetID: fromUUID(i.TargetID), State: i.State, Error: i.Error.String, ContentSHA256: i.ContentSha256, CreatedAt: fromStamp(i.CreatedAt),
	}
}

func items(rows []sqlcgen.ImportItem, err error, what string) ([]app.ItemRow, error) {
	if err != nil {
		return nil, mapError(err, what)
	}
	out := make([]app.ItemRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, toItem(r))
	}
	return out, nil
}

// InsertSource implements app.Store.
func (r *Repository) InsertSource(ctx context.Context, s importer.Source) (importer.Source, error) {
	q, err := r.write(ctx)
	if err != nil {
		return importer.Source{}, err
	}
	row, err := q.InsertSource(ctx, sqlcgen.InsertSourceParams{
		ID: uuid(s.ID), Kind: s.Kind, Name: s.Name, Config: s.Config, CreatedBy: uuid(s.CreatedBy), CreatedAt: stamp(s.CreatedAt),
	})
	if err != nil {
		return importer.Source{}, mapError(err, "inserting a source")
	}
	return toSource(row), nil
}

// Source implements app.Store.
func (r *Repository) Source(ctx context.Context, id ids.ID) (importer.Source, bool, error) {
	row, err := r.read(ctx).GetSource(ctx, uuid(id))
	return one(row, err, toSource, "reading a source")
}

// LockSource implements app.Store.
func (r *Repository) LockSource(ctx context.Context, id ids.ID) (importer.Source, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return importer.Source{}, false, err
	}
	row, err := q.LockSource(ctx, uuid(id))
	return one(row, err, toSource, "locking a source")
}

// SaveSource implements app.Store.
func (r *Repository) SaveSource(ctx context.Context, s importer.Source) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.SaveSource(ctx, sqlcgen.SaveSourceParams{
		ID: uuid(s.ID), State: s.State, Config: s.Config, Stats: s.Stats, Cursor: s.Cursor,
		StartedAt: optionalStamp(s.StartedAt), FinishedAt: optionalStamp(s.FinishedAt),
	}), "saving a source")
}

// Sources implements app.Store.
func (r *Repository) Sources(ctx context.Context, after *app.SourceKey, limit int) ([]importer.Source, error) {
	p := sqlcgen.ListSourcesParams{PageLimit: int32(min(limit, 1<<20)), AfterAt: stamp(time.Unix(0, 0)), AfterID: uuid(ids.ID{})} //nolint:gosec // bounded above
	if after != nil {
		p.HasAfter, p.AfterAt, p.AfterID = true, stamp(after.At), uuid(after.ID)
	}
	rows, err := r.read(ctx).ListSources(ctx, p)
	if err != nil {
		return nil, mapError(err, "listing sources")
	}
	out := make([]importer.Source, 0, len(rows))
	for _, s := range rows {
		out = append(out, toSource(s))
	}
	return out, nil
}

// InsertPendingItem implements app.Store.
func (r *Repository) InsertPendingItem(ctx context.Context, n app.NewItem) (app.ItemRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.ItemRow{}, false, err
	}
	row, err := q.InsertPendingItem(ctx, sqlcgen.InsertPendingItemParams{
		ID: uuid(n.ID), SourceID: uuid(n.SourceID), OriginKey: n.OriginKey, ItemKind: n.ItemKind, ContentSha256: n.ContentSHA256,
		CreatedAt: stamp(n.At),
	})
	return one(row, err, toItem, "inserting a pending item")
}

// LockItem implements app.Store.
func (r *Repository) LockItem(ctx context.Context, source ids.ID, originKey string) (app.ItemRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.ItemRow{}, false, err
	}
	row, err := q.LockItemByKey(ctx, sqlcgen.LockItemByKeyParams{SourceID: uuid(source), OriginKey: originKey})
	return one(row, err, toItem, "locking an item")
}

// Item implements app.Store.
func (r *Repository) Item(ctx context.Context, source ids.ID, originKey string) (app.ItemRow, bool, error) {
	row, err := r.read(ctx).GetItemByKey(ctx, sqlcgen.GetItemByKeyParams{SourceID: uuid(source), OriginKey: originKey})
	return one(row, err, toItem, "reading an item")
}

// FinishItem implements app.Store.
func (r *Repository) FinishItem(ctx context.Context, id ids.ID, state, targetKind string, targetID ids.ID, at time.Time) (app.ItemRow, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.ItemRow{}, err
	}
	row, err := q.FinishItem(ctx, sqlcgen.FinishItemParams{
		ID: uuid(id), State: state, TargetKind: optionalText(targetKind), TargetID: optionalUUID(targetID), UpdatedAt: stamp(at),
	})
	if err != nil {
		return app.ItemRow{}, mapError(err, "finishing an item")
	}
	return toItem(row), nil
}

// Items implements app.Store.
func (r *Repository) Items(ctx context.Context, source ids.ID, state string, after ids.ID, limit int) ([]app.ItemRow, error) {
	rows, err := r.read(ctx).ListItems(ctx, sqlcgen.ListItemsParams{
		SourceID: uuid(source), State: optionalText(state), AfterID: uuid(after), PageLimit: int32(min(limit, 1<<20)), //nolint:gosec // bounded above
	})
	return items(rows, err, "listing items")
}

// ItemsOfKind implements app.Store.
func (r *Repository) ItemsOfKind(ctx context.Context, source ids.ID, kind, state string) ([]app.ItemRow, error) {
	rows, err := r.read(ctx).ItemsOfKind(ctx, sqlcgen.ItemsOfKindParams{SourceID: uuid(source), ItemKind: kind, State: state})
	return items(rows, err, "listing items of a kind")
}

// ControlledChannel implements app.Store.
func (r *Repository) ControlledChannel(ctx context.Context, channelID ids.ID) (bool, error) {
	ok, err := r.read(ctx).ControlledChannel(ctx, uuid(channelID))
	return ok, mapError(err, "reading the import control of a channel")
}

// OriginOfTarget implements app.Store.
func (r *Repository) OriginOfTarget(ctx context.Context, kind string, id ids.ID) (app.ItemRow, bool, error) {
	row, err := r.read(ctx).OriginOfTarget(ctx, sqlcgen.OriginOfTargetParams{TargetKind: optionalText(kind), TargetID: uuid(id)})
	return one(row, err, toItem, "reading the origin of a target")
}
