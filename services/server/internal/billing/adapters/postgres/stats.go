// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/billing"
	"github.com/OWNER/bothy/services/server/internal/billing/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/billing/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/statskit"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Compile-time check: Repository answers the statistics reads of the app (M6-05).
var _ app.StatsReader = (*Repository)(nil)

// CostStats implements app.StatsReader with the statement CostStats (M6-05); showPersonal is
// billing.show_personal_estimates.
func (r *Repository) CostStats(ctx context.Context, q billing.CostStatsQuery, showPersonal bool) ([]billing.CostStatsValue, error) {
	rows, err := r.read(ctx).CostStats(ctx, sqlcgen.CostStatsParams{Granularity: q.Granularity, TimeZone: q.TimeZone, GroupBy: q.GroupBy,
		ChannelIds: pg.UUIDs(q.ChannelIDs), FromAt: pg.Timestamptz(q.From), ToAt: pg.Timestamptz(q.To), AgentID: pg.NullableUUID(q.AgentID),
		Currency: q.Currency, ShowPersonal: showPersonal})
	if err != nil {
		return nil, statskit.ReadError(err, "summing costs")
	}
	out := make([]billing.CostStatsValue, len(rows))
	for i, row := range rows {
		out[i] = billing.CostStatsValue{Bucket: row.Bucket.Time, Group: row.GroupKey, BillingMode: row.BillingMode, Currency: row.Currency,
			AmountMicros: row.AmountMicros, IsEstimate: row.IsEstimate}
	}
	return out, nil
}

// CostCurrencies implements app.StatsReader with the statement CostCurrencies (M6-05).
func (r *Repository) CostCurrencies(ctx context.Context, q billing.CostStatsQuery, showPersonal bool) ([]string, error) {
	out, err := r.read(ctx).CostCurrencies(ctx, sqlcgen.CostCurrenciesParams{ChannelIds: pg.UUIDs(q.ChannelIDs), FromAt: pg.Timestamptz(q.From),
		ToAt: pg.Timestamptz(q.To), AgentID: pg.NullableUUID(q.AgentID), ShowPersonal: showPersonal})
	if err != nil {
		return nil, statskit.ReadError(err, "reading cost currencies")
	}
	return out, nil
}
