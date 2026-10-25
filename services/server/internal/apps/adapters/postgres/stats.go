// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/apps"
	"github.com/OWNER/bothy/services/server/internal/apps/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/statskit"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Compile-time check: Repository answers the statistics reads of the app (M6-05).
var _ app.StatsReader = (*Repository)(nil)

// WebhookStats implements app.StatsReader with the statement WebhookStats (M6-05).
func (r *Repository) WebhookStats(ctx context.Context, from, to time.Time, granularity, tz string) ([]apps.StatsValue, error) {
	rows, err := r.read(ctx).WebhookStats(ctx, sqlcgen.WebhookStatsParams{Granularity: granularity, TimeZone: tz, FromAt: pg.Timestamptz(from), ToAt: pg.Timestamptz(to)})
	if err != nil {
		return nil, statskit.ReadError(err, "counting webhook deliveries")
	}
	var out []apps.StatsValue
	for _, row := range rows {
		if row.Finished > 0 {
			out = append(out, apps.StatsValue{Bucket: row.Bucket.Time, Metric: apps.MetricWebhookFinished, Value: float64(row.Finished)})
		}
		if row.Failed > 0 {
			out = append(out, apps.StatsValue{Bucket: row.Bucket.Time, Metric: apps.MetricWebhookFailed, Value: float64(row.Failed)})
		}
	}
	return out, nil
}
