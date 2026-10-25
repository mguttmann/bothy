// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/stats"
	"github.com/OWNER/bothy/services/server/internal/stats/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/stats/app"
)

// Compile-time check: Repository is the outgoing port of the views (M6-14).
var _ app.ViewsStore = (*Repository)(nil)

// TimelineRows implements app.ViewsStore with the statement TimelineRows on the pool.
func (r *Repository) TimelineRows(ctx context.Context, q app.TimelineRowsQuery) ([]stats.TimelineBucket, error) {
	rows, err := sqlcgen.New(r.pool).TimelineRows(ctx, sqlcgen.TimelineRowsParams{FromAt: pg.Timestamptz(q.From), ToAt: pg.Timestamptz(q.To),
		Metrics: q.Metrics, ChannelIds: pg.UUIDs(q.ChannelIDs), WithInstance: q.WithInstance})
	if err != nil {
		return nil, err
	}
	out := make([]stats.TimelineBucket, len(rows))
	for i, row := range rows {
		out[i] = stats.TimelineBucket{Start: pg.TimeOf(row.BucketStart), Metric: row.Metric, ChannelID: pg.IDOf(row.ChannelID),
			AgentID: pg.IDOf(row.AgentID), Dimension: row.Dimension, Currency: row.Currency, Count: row.Count, Sum: row.Sum, Good: row.Good}
	}
	return out, nil
}

// BackendRows implements app.ViewsStore with the statement BackendRows on the pool.
func (r *Repository) BackendRows(ctx context.Context, q app.BackendRowsQuery) ([]app.BackendRow, error) {
	rows, err := sqlcgen.New(r.pool).BackendRows(ctx, sqlcgen.BackendRowsParams{FromAt: pg.Timestamptz(q.From), ToAt: pg.Timestamptz(q.To),
		AgentIds: pg.UUIDs(q.AgentIDs), ChannelIds: pg.UUIDs(q.ChannelIDs), WithCost: q.WithCost})
	if err != nil {
		return nil, err
	}
	out := make([]app.BackendRow, len(rows))
	for i, row := range rows {
		out[i] = app.BackendRow{AgentID: pg.IDOf(row.AgentID), Metric: row.Metric, Dimension: row.Dimension, Currency: row.Currency,
			Count: row.Count, Sum: row.Sum}
	}
	return out, nil
}
