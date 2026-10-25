// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/stats"
	"github.com/OWNER/bothy/services/server/internal/stats/adapters/postgres/sqlcgen"
)

// Compile-time check: Repository is the read port of the automatic checks of the status page.
var _ stats.SLIWindows = (*Repository)(nil)

// SLITotals implements stats.SLIWindows with one aggregate on the pool (M7-09).
func (r *Repository) SLITotals(ctx context.Context, metric string, from, to time.Time) ([]stats.SLITotal, error) {
	rows, err := sqlcgen.New(r.pool).SLITotals(ctx, sqlcgen.SLITotalsParams{
		Metric: metric, WindowFrom: pgtype.Timestamptz{Time: from, Valid: true}, WindowTo: pgtype.Timestamptz{Time: to, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	out := make([]stats.SLITotal, len(rows))
	for i, row := range rows {
		out[i] = stats.SLITotal{Dimension: row.Dimension, Count: row.Count, Sum: row.Sum, Good: row.Good}
	}
	return out, nil
}
