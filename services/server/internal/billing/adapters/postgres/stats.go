// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/billing"
	"github.com/OWNER/bothy/services/server/internal/billing/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/billing/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository answers the statistics reads of the app (M6-05).
var _ app.StatsReader = (*Repository)(nil)

// CostStats implements app.StatsReader with the statement CostStats (M6-05); showPersonal is
// billing.show_personal_estimates.
func (r *Repository) CostStats(ctx context.Context, q billing.CostStatsQuery, showPersonal bool) ([]billing.CostStatsValue, error) {
	rows, err := r.read(ctx).CostStats(ctx, sqlcgen.CostStatsParams{Granularity: q.Granularity, TimeZone: q.TimeZone, GroupBy: q.GroupBy,
		ChannelIds: statsUUIDs(q.ChannelIDs), FromAt: statsTS(q.From), ToAt: statsTS(q.To), AgentID: statsAgent(q.AgentID),
		Currency: q.Currency, ShowPersonal: showPersonal})
	if err != nil {
		return nil, statsError(err, "summing costs")
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
	out, err := r.read(ctx).CostCurrencies(ctx, sqlcgen.CostCurrenciesParams{ChannelIds: statsUUIDs(q.ChannelIDs), FromAt: statsTS(q.From),
		ToAt: statsTS(q.To), AgentID: statsAgent(q.AgentID), ShowPersonal: showPersonal})
	if err != nil {
		return nil, statsError(err, "reading cost currencies")
	}
	return out, nil
}

// statsAgent is the optional agent filter: NULL for every agent.
func statsAgent(id ids.ID) pgtype.UUID {
	if id.IsZero() {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

// statsUUIDs converts ids for an array parameter of the statistics reads.
func statsUUIDs(list []ids.ID) []pgtype.UUID {
	out := make([]pgtype.UUID, len(list))
	for i, id := range list {
		out[i] = pgtype.UUID{Bytes: id, Valid: true}
	}
	return out
}

// statsTS converts an instant for a parameter of the statistics reads.
func statsTS(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

// statsError wraps a database error of a statistics read as internal_error.
func statsError(err error, what string) error {
	if _, ok := errs.As(err); ok {
		return err
	}
	return errs.Wrap(errs.CodeInternalError, err, ": "+what+" failed")
}
