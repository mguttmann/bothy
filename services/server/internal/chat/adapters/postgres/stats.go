// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/statskit"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Compile-time check: Repository answers the statistics reads of the app (M6-05).
var _ app.StatsReader = (*Repository)(nil)

// StatsChannels implements app.StatsReader with the statement StatsChannels (M6-05).
func (r *Repository) StatsChannels(ctx context.Context, channelIDs []ids.ID) ([]chat.StatsChannel, error) {
	rows, err := r.read(ctx).StatsChannels(ctx, pg.UUIDs(channelIDs))
	if err != nil {
		return nil, statskit.ReadError(err, "reading the channels of the statistics")
	}
	out := make([]chat.StatsChannel, len(rows))
	for i, row := range rows {
		out[i] = chat.StatsChannel{ChannelID: ids.ID(row.ID.Bytes), SpaceID: ids.ID(row.SpaceID.Bytes)}
	}
	return out, nil
}

// StatsThreads implements app.StatsReader with the statement StatsThreads (M6-05).
func (r *Repository) StatsThreads(ctx context.Context, channelIDs []ids.ID, before time.Time) ([]chat.StatsThread, error) {
	rows, err := r.read(ctx).StatsThreads(ctx, sqlcgen.StatsThreadsParams{ChannelIds: pg.UUIDs(channelIDs), BeforeAt: pg.Timestamptz(before)})
	if err != nil {
		return nil, statskit.ReadError(err, "reading the threads of the statistics")
	}
	out := make([]chat.StatsThread, len(rows))
	for i, row := range rows {
		out[i] = chat.StatsThread{ThreadID: ids.ID(row.ID.Bytes), ChannelID: ids.ID(row.ChannelID.Bytes)}
	}
	return out, nil
}

// StatsRunChats implements app.StatsReader with the statement StatsRunChats (owner request
// 2026-11-01).
func (r *Repository) StatsRunChats(ctx context.Context, channelIDs, threadIDs []ids.ID) ([]chat.StatsRunChat, error) {
	rows, err := r.read(ctx).StatsRunChats(ctx, sqlcgen.StatsRunChatsParams{ChannelIds: pg.UUIDs(channelIDs), ThreadIds: pg.UUIDs(threadIDs)})
	if err != nil {
		return nil, statskit.ReadError(err, "reading the chats of the statistics")
	}
	out := make([]chat.StatsRunChat, len(rows))
	for i, row := range rows {
		out[i] = chat.StatsRunChat{ThreadID: ids.ID(row.ID.Bytes), ChannelID: ids.ID(row.ChannelID.Bytes), ChannelName: row.ChannelName,
			ThreadTitle: row.ThreadTitle}
	}
	return out, nil
}

// ThreadStats implements app.StatsReader with the statement ThreadStats (M6-05).
func (r *Repository) ThreadStats(ctx context.Context, q chat.StatsQuery) ([]chat.StatsValue, error) {
	rows, err := r.read(ctx).ThreadStats(ctx, sqlcgen.ThreadStatsParams{Granularity: q.Granularity, TimeZone: q.TimeZone,
		ChannelIds: pg.UUIDs(q.ChannelIDs), GroupIds: pg.UUIDs(q.GroupIDs), FromAt: pg.Timestamptz(q.From), ToAt: pg.Timestamptz(q.To)})
	if err != nil {
		return nil, statskit.ReadError(err, "counting threads")
	}
	out := make([]chat.StatsValue, len(rows))
	for i, row := range rows {
		out[i] = chat.StatsValue{Bucket: row.Bucket.Time, Group: pg.UUIDText(row.GroupID), Metric: row.Metric, Value: float64(row.Value)}
	}
	return out, nil
}
