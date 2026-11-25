// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres/sqlcgen"
)

// Retention is the participant of runs in the job retention.content (docs/spec/20-privacy.md 4.1,
// AUD-PKG-R2-owner-server): the rows of runs, run_steps and run_wakes stay with their thread (03
// principle 7), but their texts are emptied once the end time lies before the cutoff: status
// line, text excerpt, summary and plan of a run after finished_at, title and detail of its steps
// after the finished_at of the run, label, condition and result of a wake-up after ended_at. A
// run that has not finished and a pending wake-up have no end time and keep their texts. The
// stored event payloads keep their texts until sync.event_retention. It is safe for concurrent
// use.
type Retention struct {
	pool *pg.Pool
}

// Compile-time check: Retention is a participant of kernel/retention.
var _ retention.Participant = (*Retention)(nil)

// NewRetention returns the participant over pool.
func NewRetention(pool *pg.Pool) *Retention { return &Retention{pool: pool} }

// Class returns retention.ClassContent.
func (*Retention) Class() retention.Class { return retention.ClassContent }

// tableRuns is the table runs, which also names the module.
const tableRuns = "runs"

// Tables returns the tables whose texts Purge empties, in that order.
func (*Retention) Tables() []string { return []string{tableRuns, "run_steps", "run_wakes"} }

// Purge implements retention.Participant: at most batch rows per table, each table in its own
// short statement. deleted counts the emptied rows; more is true while a statement emptied a full
// batch.
//
// Errors: those of the storage; deleted then counts the rows of the statements before.
func (r *Retention) Purge(ctx context.Context, cutoff time.Time, batch int) (deleted int, more bool, err error) {
	q := sqlcgen.New(r.pool)
	c := pgtype.Timestamptz{Time: cutoff, Valid: true}
	limit := pg.Limit(batch)
	steps := []struct {
		what string
		run  func() (int64, error)
	}{
		{"the texts of runs", func() (int64, error) {
			return q.ClearExpiredRunTexts(ctx, sqlcgen.ClearExpiredRunTextsParams{Cutoff: c, RowLimit: limit})
		}},
		{"the texts of run steps", func() (int64, error) {
			return q.ClearExpiredRunStepTexts(ctx, sqlcgen.ClearExpiredRunStepTextsParams{Cutoff: c, RowLimit: limit})
		}},
		{"the texts of wake-ups", func() (int64, error) {
			return q.ClearExpiredRunWakeTexts(ctx, sqlcgen.ClearExpiredRunWakeTextsParams{Cutoff: c, RowLimit: limit})
		}},
	}
	for _, st := range steps {
		n, err := st.run()
		if err != nil {
			return deleted, false, mapError(err, "the retention of "+st.what)
		}
		deleted += int(n)
		more = more || n >= int64(limit)
	}
	return deleted, more, nil
}
