// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/statskit"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Compile-time check: Repository answers the statistics reads of the app (M6-05).
var _ app.StatsReader = (*Repository)(nil)

// StorageBytes implements app.StatsReader with the statement StorageStats (M6-05).
func (r *Repository) StorageBytes(ctx context.Context, channelIDs, groupIDs []ids.ID) ([]files.StatsValue, error) {
	rows, err := r.read(ctx).StorageStats(ctx, sqlcgen.StorageStatsParams{ChannelIds: pg.UUIDs(channelIDs), GroupIds: pg.UUIDs(groupIDs)})
	if err != nil {
		return nil, statskit.ReadError(err, "summing storage")
	}
	out := make([]files.StatsValue, len(rows))
	for i, row := range rows {
		out[i] = files.StatsValue{Group: pg.UUIDText(row.GroupID), Metric: files.MetricStorageBytes, Value: float64(row.Value)}
	}
	return out, nil
}

// ScanStats implements app.StatsReader with the statement ScanStats (M6-05).
func (r *Repository) ScanStats(ctx context.Context, from, to time.Time, granularity, tz string) ([]files.StatsValue, error) {
	rows, err := r.read(ctx).ScanStats(ctx, sqlcgen.ScanStatsParams{Granularity: granularity, TimeZone: tz, FromAt: pg.Timestamptz(from), ToAt: pg.Timestamptz(to)})
	if err != nil {
		return nil, statskit.ReadError(err, "counting scans")
	}
	var out []files.StatsValue
	for _, row := range rows {
		if row.Errors > 0 {
			out = append(out, files.StatsValue{Bucket: row.Bucket.Time, Metric: files.MetricScanErrors, Value: float64(row.Errors)})
		}
		if row.Infected > 0 {
			out = append(out, files.StatsValue{Bucket: row.Bucket.Time, Metric: files.MetricScanInfected, Value: float64(row.Infected)})
		}
	}
	return out, nil
}
