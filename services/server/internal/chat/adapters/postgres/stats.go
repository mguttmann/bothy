// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository answers the statistics reads of the app (M6-05).
var _ app.StatsReader = (*Repository)(nil)

// StatsChannels implements app.StatsReader with the statement StatsChannels (M6-05).
func (r *Repository) StatsChannels(ctx context.Context, channelIDs []ids.ID) ([]chat.StatsChannel, error) {
	rows, err := r.read(ctx).StatsChannels(ctx, statsUUIDs(channelIDs))
	if err != nil {
		return nil, statsError(err, "reading the channels of the statistics")
	}
	out := make([]chat.StatsChannel, len(rows))
	for i, row := range rows {
		out[i] = chat.StatsChannel{ChannelID: ids.ID(row.ID.Bytes), SpaceID: ids.ID(row.SpaceID.Bytes)}
	}
	return out, nil
}

// StatsThreads implements app.StatsReader with the statement StatsThreads (M6-05).
func (r *Repository) StatsThreads(ctx context.Context, channelIDs []ids.ID, before time.Time) ([]chat.StatsThread, error) {
	rows, err := r.read(ctx).StatsThreads(ctx, sqlcgen.StatsThreadsParams{ChannelIds: statsUUIDs(channelIDs), BeforeAt: statsTS(before)})
	if err != nil {
		return nil, statsError(err, "reading the threads of the statistics")
	}
	out := make([]chat.StatsThread, len(rows))
	for i, row := range rows {
		out[i] = chat.StatsThread{ThreadID: ids.ID(row.ID.Bytes), ChannelID: ids.ID(row.ChannelID.Bytes)}
	}
	return out, nil
}

// ThreadStats implements app.StatsReader with the statement ThreadStats (M6-05).
func (r *Repository) ThreadStats(ctx context.Context, q chat.StatsQuery) ([]chat.StatsValue, error) {
	rows, err := r.read(ctx).ThreadStats(ctx, sqlcgen.ThreadStatsParams{Granularity: q.Granularity, TimeZone: q.TimeZone,
		ChannelIds: statsUUIDs(q.ChannelIDs), GroupIds: statsUUIDs(q.GroupIDs), FromAt: statsTS(q.From), ToAt: statsTS(q.To)})
	if err != nil {
		return nil, statsError(err, "counting threads")
	}
	out := make([]chat.StatsValue, len(rows))
	for i, row := range rows {
		out[i] = chat.StatsValue{Bucket: row.Bucket.Time, Group: statsGroup(row.GroupID), Metric: row.Metric, Value: float64(row.Value)}
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
