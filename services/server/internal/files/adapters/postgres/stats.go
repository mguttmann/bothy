// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository answers the statistics reads of the app (M6-05).
var _ app.StatsReader = (*Repository)(nil)

// StorageBytes implements app.StatsReader with the statement StorageStats (M6-05).
func (r *Repository) StorageBytes(ctx context.Context, channelIDs, groupIDs []ids.ID) ([]files.StatsValue, error) {
	rows, err := r.read(ctx).StorageStats(ctx, sqlcgen.StorageStatsParams{ChannelIds: statsUUIDs(channelIDs), GroupIds: statsUUIDs(groupIDs)})
	if err != nil {
		return nil, statsError(err, "summing storage")
	}
	out := make([]files.StatsValue, len(rows))
	for i, row := range rows {
		out[i] = files.StatsValue{Group: statsGroup(row.GroupID), Metric: files.MetricStorageBytes, Value: float64(row.Value)}
	}
	return out, nil
}

// ScanStats implements app.StatsReader with the statement ScanStats (M6-05).
func (r *Repository) ScanStats(ctx context.Context, from, to time.Time, granularity, tz string) ([]files.StatsValue, error) {
	rows, err := r.read(ctx).ScanStats(ctx, sqlcgen.ScanStatsParams{Granularity: granularity, TimeZone: tz, FromAt: statsTS(from), ToAt: statsTS(to)})
	if err != nil {
		return nil, statsError(err, "counting scans")
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
