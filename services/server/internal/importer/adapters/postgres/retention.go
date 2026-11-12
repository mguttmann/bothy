// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/importer/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Retention is the participant of importer in the job retention.import (docs/spec/20-privacy.md
// 4.1, docs/spec/22-import-migration.md 13.5 and 14): it deletes the items and then the row of a
// source whose end lies before the cutoff (retention.import_after_completion before now). The end
// time is import_sources.finished_at of a source in state completed or failed (a cancelled source
// is failed); a draft, running or paused source has none and keeps every row, because a running or
// resumed import needs its items to stay idempotent. The imported spaces, channels, threads,
// messages, files and people are not touched: a message only loses the reference to its item
// (messages.import_item_id, ON DELETE SET NULL). After the deletion a repeated import of the same
// Discord server finds no origin key and would copy everything again. It is safe for concurrent
// use.
type Retention struct {
	pool *pg.Pool
}

// Compile-time check: Retention is a participant of kernel/retention.
var _ retention.Participant = (*Retention)(nil)

// NewRetention returns the participant over pool.
func NewRetention(pool *pg.Pool) *Retention { return &Retention{pool: pool} }

// Class returns retention.ClassImport.
func (*Retention) Class() retention.Class { return retention.ClassImport }

// Tables returns the tables Purge deletes from, in the order it deletes them.
func (*Retention) Tables() []string { return []string{"import_items", "import_sources"} }

// Purge implements retention.Participant: at most batch items of ended sources, then at most batch
// rows of ended sources without items, each in its own short statement. more is true while a
// statement removed a full batch.
//
// Errors: those of the storage; deleted then counts the rows the first statement removed.
func (r *Retention) Purge(ctx context.Context, cutoff time.Time, batch int) (deleted int, more bool, err error) {
	q := sqlcgen.New(r.pool)
	c := pgtype.Timestamptz{Time: cutoff, Valid: true}
	limit := pg.Limit(batch)
	items, err := q.PurgeImportItems(ctx, sqlcgen.PurgeImportItemsParams{Cutoff: c, RowLimit: limit})
	if err != nil {
		return 0, false, mapError(err, "the retention of the import items")
	}
	sources, err := q.PurgeImportSources(ctx, sqlcgen.PurgeImportSourcesParams{Cutoff: c, RowLimit: limit})
	if err != nil {
		return int(items), false, mapError(err, "the retention of the import sources")
	}
	return int(items + sources), items >= int64(limit) || sources >= int64(limit), nil
}
