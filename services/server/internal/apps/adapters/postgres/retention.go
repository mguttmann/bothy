// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/apps/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Retention is the participant of apps in the job retention.transient (M7-05,
// docs/spec/20-privacy.md 2 and 4.1): it deletes finished webhook deliveries whose end
// (delivered_at, or created_at of a failed or dead delivery) lies before the cutoff
// (retention.webhook_deliveries before now). A pending delivery never matches.
type Retention struct {
	pool *pg.Pool
}

// Compile-time check: Retention is a participant of kernel/retention.
var _ retention.Participant = (*Retention)(nil)

// NewRetention returns the participant over pool.
func NewRetention(pool *pg.Pool) *Retention { return &Retention{pool: pool} }

// Class returns retention.ClassTransient.
func (*Retention) Class() retention.Class { return retention.ClassTransient }

// Tables returns webhook_deliveries.
func (*Retention) Tables() []string { return []string{"webhook_deliveries"} }

// Purge implements retention.Participant: one batch of at most batch rows in one short
// transaction. Errors: those of the storage.
func (r *Retention) Purge(ctx context.Context, cutoff time.Time, batch int) (int, bool, error) {
	c := pgtype.Timestamptz{Time: cutoff, Valid: true}
	return pg.PurgeStep(ctx, r.pool, batch,
		func(ctx context.Context, q pg.DBTX) ([]pgtype.UUID, error) {
			return sqlcgen.New(q).ExpiredWebhookDeliveries(ctx, sqlcgen.ExpiredWebhookDeliveriesParams{Cutoff: c, RowLimit: pg.Limit(batch)})
		},
		func(ctx context.Context, q pg.DBTX, id pgtype.UUID) (int64, error) {
			return sqlcgen.New(q).DeleteWebhookDelivery(ctx, sqlcgen.DeleteWebhookDeliveryParams{ID: id, Cutoff: c})
		})
}
