// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Compile-time check: Repository answers the statistics reads of the app (M6-05).
var _ app.StatsReader = (*Repository)(nil)

// PushStats implements app.StatsReader with the statement PushStats (M6-05).
func (r *Repository) PushStats(ctx context.Context, starts, ends []time.Time) ([]notify.StatsValue, error) {
	rows, err := r.read(ctx).PushStats(ctx, sqlcgen.PushStatsParams{Starts: pg.Timestamptzs(starts), Ends: pg.Timestamptzs(ends)})
	if err != nil {
		if _, ok := errs.As(err); ok {
			return nil, err
		}
		return nil, errs.Wrap(errs.CodeInternalError, err, "notify: counting push registrations failed")
	}
	var out []notify.StatsValue
	for _, row := range rows {
		out = append(out, notify.StatsValue{Bucket: row.Bucket.Time, Group: row.Provider, Metric: notify.MetricPushRegistrations, Value: float64(row.Registrations)})
		if row.Disabled > 0 {
			out = append(out, notify.StatsValue{Bucket: row.Bucket.Time, Group: row.Provider, Metric: notify.MetricPushDisabled, Value: float64(row.Disabled)})
		}
	}
	return out, nil
}
