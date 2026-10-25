// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/billing/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Retention is the participant of billing in the job retention.cost (M7-05,
// docs/spec/20-privacy.md 2 and 4.1): cost entries whose occurred_at and closed price rows whose
// valid_to lie before the cutoff (retention.cost before now). A price row a kept entry was
// computed with is closed before that entry occurred, so it ends no later than the entry.
type Retention struct {
	pool *pg.Pool
}

// Compile-time check: Retention is a participant of kernel/retention.
var _ retention.Participant = (*Retention)(nil)

// NewRetention returns the participant over pool.
func NewRetention(pool *pg.Pool) *Retention { return &Retention{pool: pool} }

// Class returns retention.ClassCost.
func (*Retention) Class() retention.Class { return retention.ClassCost }

// Tables returns the tables in the order Purge deletes from them.
func (*Retention) Tables() []string { return []string{"cost_entries", "price_table"} }

// Purge implements retention.Participant: one batch of at most batch rows per table, each in one
// short transaction. Errors: those of the storage.
func (r *Retention) Purge(ctx context.Context, cutoff time.Time, batch int) (int, bool, error) {
	c := pgtype.Timestamptz{Time: cutoff, Valid: true}
	limit := pg.Limit(batch)
	n1, m1, err := pg.PurgeStep(ctx, r.pool, batch,
		func(ctx context.Context, q pg.DBTX) ([]pgtype.UUID, error) {
			return sqlcgen.New(q).ExpiredCostEntries(ctx, sqlcgen.ExpiredCostEntriesParams{Cutoff: c, RowLimit: limit})
		},
		func(ctx context.Context, q pg.DBTX, id pgtype.UUID) (int64, error) {
			return sqlcgen.New(q).DeleteCostEntry(ctx, sqlcgen.DeleteCostEntryParams{ID: id, Cutoff: c})
		})
	if err != nil {
		return n1, false, err
	}
	n2, m2, err := pg.PurgeStep(ctx, r.pool, batch,
		func(ctx context.Context, q pg.DBTX) ([]pgtype.UUID, error) {
			return sqlcgen.New(q).ExpiredPrices(ctx, sqlcgen.ExpiredPricesParams{Cutoff: c, RowLimit: limit})
		},
		func(ctx context.Context, q pg.DBTX, id pgtype.UUID) (int64, error) {
			return sqlcgen.New(q).DeletePrice(ctx, sqlcgen.DeletePriceParams{ID: id, Cutoff: c})
		})
	return n1 + n2, m1 || m2, err
}
