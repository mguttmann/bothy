// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/statskit"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Compile-time check: Repository answers the statistics reads of the app (M6-05).
var _ app.StatsReader = (*Repository)(nil)

// MessageStats implements app.StatsReader with the statement MessageStats (M6-05): one value per
// origin with messages and, when q.CountPeople is set, the active people of the bucket and group.
func (r *Repository) MessageStats(ctx context.Context, q messaging.StatsQuery) ([]messaging.StatsValue, error) {
	rows, err := r.read(ctx).MessageStats(ctx, sqlcgen.MessageStatsParams{Granularity: q.Granularity, TimeZone: q.TimeZone,
		CountPeople: q.CountPeople, ChannelIds: pg.UUIDs(q.ChannelIDs), GroupIds: pg.UUIDs(q.GroupIDs), FromAt: pg.Timestamptz(q.From), ToAt: pg.Timestamptz(q.To)})
	if err != nil {
		return nil, statskit.ReadError(err, "counting messages")
	}
	var out []messaging.StatsValue
	for _, row := range rows {
		bucket, group := row.Bucket.Time, pg.UUIDText(row.GroupID)
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
		ChannelIds: pg.UUIDs(q.ChannelIDs), GroupIds: pg.UUIDs(q.GroupIDs), FromAt: pg.Timestamptz(q.From), ToAt: pg.Timestamptz(q.To)})
	if err != nil {
		return nil, statskit.ReadError(err, "counting invocation denials")
	}
	out := make([]messaging.StatsValue, len(rows))
	for i, row := range rows {
		out[i] = messaging.StatsValue{Bucket: row.Bucket.Time, Group: pg.UUIDText(row.GroupID), Metric: messaging.MetricInvocationDenials, Value: float64(row.Value)}
	}
	return out, nil
}
