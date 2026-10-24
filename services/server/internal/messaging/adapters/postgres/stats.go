// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
)

// Compile-time check: Repository answers the statistics reads of the app (M6-05).
var _ app.StatsReader = (*Repository)(nil)

// MessageStats implements app.StatsReader with the statement MessageStats (M6-05): one value per
// origin with messages and, when q.CountPeople is set, the active people of the bucket and group.
func (r *Repository) MessageStats(ctx context.Context, q messaging.StatsQuery) ([]messaging.StatsValue, error) {
	rows, err := r.read(ctx).MessageStats(ctx, sqlcgen.MessageStatsParams{Granularity: q.Granularity, TimeZone: q.TimeZone,
		CountPeople: q.CountPeople, ChannelIds: statsUUIDs(q.ChannelIDs), GroupIds: statsUUIDs(q.GroupIDs), FromAt: statsTS(q.From), ToAt: statsTS(q.To)})
	if err != nil {
		return nil, statsError(err, "counting messages")
	}
	var out []messaging.StatsValue
	for _, row := range rows {
		bucket, group := row.Bucket.Time, statsGroup(row.GroupID)
		for _, v := range []struct {
			metric string
			n      int64
		}{{messaging.MetricMessagesHuman, row.Human}, {messaging.MetricMessagesAgent, row.Agent}, {messaging.MetricMessagesApp, row.App},
			{messaging.MetricMessagesImport, row.Import}} {
			if v.n > 0 {
				out = append(out, messaging.StatsValue{Bucket: bucket, Group: group, Metric: v.metric, Value: float64(v.n)})
			}
		}
		if q.CountPeople {
			out = append(out, messaging.StatsValue{Bucket: bucket, Group: group, Metric: messaging.MetricActivePeople, Value: float64(row.People)})
		}
	}
	return out, nil
}

// InvocationDenials implements app.StatsReader with the statement InvocationDenialStats (M6-05).
func (r *Repository) InvocationDenials(ctx context.Context, q messaging.StatsQuery) ([]messaging.StatsValue, error) {
	rows, err := r.read(ctx).InvocationDenialStats(ctx, sqlcgen.InvocationDenialStatsParams{Granularity: q.Granularity, TimeZone: q.TimeZone,
		ChannelIds: statsUUIDs(q.ChannelIDs), GroupIds: statsUUIDs(q.GroupIDs), FromAt: statsTS(q.From), ToAt: statsTS(q.To)})
	if err != nil {
		return nil, statsError(err, "counting invocation denials")
	}
	out := make([]messaging.StatsValue, len(rows))
	for i, row := range rows {
		out[i] = messaging.StatsValue{Bucket: row.Bucket.Time, Group: statsGroup(row.GroupID), Metric: messaging.MetricInvocationDenials, Value: float64(row.Value)}
	}
	return out, nil
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

// statsGroup is the group key of a statistics row as text: the UUID, or "" for NULL.
func statsGroup(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return ids.ID(u.Bytes).String()
}

// statsError wraps a database error of a statistics read as internal_error.
func statsError(err error, what string) error {
	if _, ok := errs.As(err); ok {
		return err
	}
	return errs.Wrap(errs.CodeInternalError, err, ": "+what+" failed")
}
