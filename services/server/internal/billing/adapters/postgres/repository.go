// SPDX-License-Identifier: Apache-2.0

// Package postgres is the PostgreSQL adapter of the billing module (M2-22): the sqlc queries of
// price_table and cost_entries in queries/, the generated code in sqlcgen/ and Repository, which
// implements app.Repository (docs/spec/03-data-model.md section 4.9). Only this package runs SQL
// on the two tables (03 principle 12).
//
// Example:
//
//	repo := postgres.NewRepository(pool)
//	set, err := repo.LockPriceSet(ctx, "example", "model-large", now) // inside a unit of work
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/billing/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/billing/app"
	"github.com/OWNER/bothy/services/server/internal/billing/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// The SQLSTATEs of an overlapping price row: the exclusion constraint and the unique key of
// (provider, model, unit, valid_from).
const (
	sqlstateExclusionViolation = "23P01"
	sqlstateUniqueViolation    = "23505"
)

// Repository implements app.Repository with the generated queries of sqlcgen. Reads join the
// ambient transaction of ctx or use the pool; writes and locks need the ambient transaction. It is
// safe for concurrent use.
type Repository struct {
	pool *pg.Pool
}

// Compile-time check: Repository is the outgoing port of the billing tables.
var _ app.Repository = (*Repository)(nil)

// NewRepository returns the repository over pool, a pool of bothy_app.
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

// LockPriceSet implements app.Repository.
func (r *Repository) LockPriceSet(ctx context.Context, provider, model string, at time.Time) ([]domain.PriceRow, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockPriceSet(ctx, sqlcgen.LockPriceSetParams{Provider: provider, Model: model, At: ts(at)})
	return prices(rows, err, "locking a price set")
}

// InsertEntries implements app.Repository.
func (r *Repository) InsertEntries(ctx context.Context, es []app.NewEntry) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	for _, e := range es {
		if err := q.InsertCostEntry(ctx, sqlcgen.InsertCostEntryParams{
			ID: uuid(e.ID), RunID: uuid(e.RunID), AgentID: uuid(e.AgentID), ThreadID: uuid(e.ThreadID), ChannelID: uuid(e.ChannelID),
			SpaceID: optionalUUID(e.SpaceID), AgentSessionID: optionalUUID(e.AgentSessionID), BillingMode: e.BillingMode,
			Provider: e.Provider, Model: e.Model, TokensIn: e.Tokens.In, TokensOut: e.Tokens.Out, TokensCacheRead: e.Tokens.CacheRead,
			TokensCacheWrite: e.Tokens.CacheWrite, TokensCacheWrite1h: e.Tokens.CacheWrite1h, AmountMicros: e.AmountMicros,
			Currency: e.Currency, Source: e.Source, OccurredAt: ts(e.OccurredAt),
		}); err != nil {
			return mapError(err, "inserting a cost entry")
		}
	}
	return nil
}

// ThreadEntries implements app.Repository.
func (r *Repository) ThreadEntries(ctx context.Context, threadID ids.ID) ([]domain.EntryRow, error) {
	rows, err := r.read(ctx).ThreadEntries(ctx, uuid(threadID))
	if err != nil {
		return nil, mapError(err, "reading the cost entries of a thread")
	}
	out := make([]domain.EntryRow, len(rows))
	for i, e := range rows {
		out[i] = toEntry(e)
	}
	return out, nil
}

// Totals implements app.Repository.
func (r *Repository) Totals(ctx context.Context, f app.TotalsFilter) ([]domain.Total, error) {
	chans := make([]pgtype.UUID, len(f.ChannelIDs))
	for i, c := range f.ChannelIDs {
		chans[i] = uuid(c)
	}
	rows, err := r.read(ctx).Totals(ctx, sqlcgen.TotalsParams{Level: string(f.Level), ID: uuid(f.ID), FromAt: optionalTS(f.From),
		ToAt: optionalTS(f.To), AllChannels: f.AllChannels, ChannelIds: chans})
	if err != nil {
		return nil, mapError(err, "summing cost entries")
	}
	out := make([]domain.Total, len(rows))
	for i, t := range rows {
		out[i] = domain.Total{BillingMode: t.BillingMode, Currency: t.Currency, AmountMicros: t.AmountMicros, IsEstimate: t.IsEstimate,
			Tokens: domain.Tokens{In: t.TokensIn, Out: t.TokensOut, CacheRead: t.TokensCacheRead, CacheWrite: t.TokensCacheWrite,
				CacheWrite1h: t.TokensCacheWrite1h}, Entries: t.Entries}
	}
	return out, nil
}

// ListPrices implements app.Repository.
func (r *Repository) ListPrices(ctx context.Context) ([]domain.PriceRow, error) {
	rows, err := r.read(ctx).ListPrices(ctx)
	return prices(rows, err, "listing prices")
}

// CurrencyMismatch implements app.Repository.
func (r *Repository) CurrencyMismatch(ctx context.Context, provider, model, currency string, from, to time.Time) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	m, err := q.CurrencyMismatch(ctx, sqlcgen.CurrencyMismatchParams{Provider: provider, Model: model, Currency: currency,
		ValidFrom: ts(from), ValidTo: optionalTS(to)})
	return m, mapError(err, "checking the currency of a price set")
}

// InsertPrice implements app.Repository. Errors: app.ErrPriceOverlap for SQLSTATE 23P01 or 23505.
func (r *Repository) InsertPrice(ctx context.Context, p domain.PriceRow, createdBy ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.InsertPrice(ctx, sqlcgen.InsertPriceParams{ID: uuid(p.ID), Provider: p.Provider, Model: p.Model, Unit: string(p.Unit),
		PriceMicros: p.PriceMicros, Currency: p.Currency, ValidFrom: ts(p.ValidFrom), ValidTo: optionalTS(p.ValidTo),
		SourceUrl: p.SourceURL, RetrievedAt: ts(p.RetrievedAt), CreatedBy: uuid(createdBy)})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == sqlstateExclusionViolation || pgErr.Code == sqlstateUniqueViolation) {
		return app.ErrPriceOverlap
	}
	return mapError(err, "inserting a price")
}

// LockPrice implements app.Repository.
func (r *Repository) LockPrice(ctx context.Context, id ids.ID) (domain.PriceRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.PriceRow{}, false, err
	}
	row, err := q.LockPrice(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PriceRow{}, false, nil
	}
	if err != nil {
		return domain.PriceRow{}, false, mapError(err, "locking a price")
	}
	return toPrice(row), true, nil
}

// PriceInUse implements app.Repository.
func (r *Repository) PriceInUse(ctx context.Context, row domain.PriceRow, validTo time.Time) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	used, err := q.PriceInUse(ctx, sqlcgen.PriceInUseParams{Provider: row.Provider, Model: row.Model, ValidFrom: ts(row.ValidFrom),
		ValidTo: ts(validTo), Unit: string(row.Unit)})
	return used, mapError(err, "checking the use of a price")
}

// SetValidTo implements app.Repository.
func (r *Repository) SetValidTo(ctx context.Context, id ids.ID, validTo time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.SetPriceValidTo(ctx, sqlcgen.SetPriceValidToParams{ID: uuid(id), ValidTo: ts(validTo)}), "closing a price")
}

// prices converts a list of price rows.
func prices(rows []sqlcgen.PriceTable, err error, what string) ([]domain.PriceRow, error) {
	if err != nil {
		return nil, mapError(err, what)
	}
	out := make([]domain.PriceRow, len(rows))
	for i, p := range rows {
		out[i] = toPrice(p)
	}
	return out, nil
}

// toPrice converts a row of price_table.
func toPrice(p sqlcgen.PriceTable) domain.PriceRow {
	return domain.PriceRow{ID: idOf(p.ID), Provider: p.Provider, Model: p.Model, Unit: domain.Unit(p.Unit), PriceMicros: p.PriceMicros,
		Currency: p.Currency, ValidFrom: timeOf(p.ValidFrom), ValidTo: timeOf(p.ValidTo), SourceURL: p.SourceUrl,
		RetrievedAt: timeOf(p.RetrievedAt)}
}

// toEntry converts a row of cost_entries.
func toEntry(e sqlcgen.CostEntry) domain.EntryRow {
	return domain.EntryRow{ID: idOf(e.ID), RunID: idOf(e.RunID), AgentSessionID: idOf(e.AgentSessionID), BillingMode: e.BillingMode,
		Provider: e.Provider, Model: e.Model, Tokens: domain.Tokens{In: e.TokensIn, Out: e.TokensOut, CacheRead: e.TokensCacheRead,
			CacheWrite: e.TokensCacheWrite, CacheWrite1h: e.TokensCacheWrite1h}, AmountMicros: e.AmountMicros, Currency: e.Currency,
		IsEstimate: e.IsEstimate, Source: e.Source, OccurredAt: timeOf(e.OccurredAt), SupersedesID: idOf(e.SupersedesID)}
}

// mapError wraps a database error as internal; errs errors pass unchanged. The wrapped error
// keeps its SQLSTATE for the retry of the unit of work.
func mapError(err error, what string) error {
	if err == nil {
		return nil
	}
	if _, ok := errs.As(err); ok {
		return err
	}
	return errs.Wrap(errs.CodeInternalError, err, "billing: "+what+" failed")
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

// ts converts a time for a parameter in UTC, cut to the microseconds the column keeps.
func ts(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.UTC().Truncate(time.Microsecond), Valid: true}
}

// optionalTS converts an optional time: the zero time becomes NULL.
func optionalTS(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return ts(t)
}

// timeOf converts a column in UTC; NULL becomes the zero time.
func timeOf(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time.UTC()
}
