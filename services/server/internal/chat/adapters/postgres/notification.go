// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The reads of the notification path of notify (M3-18): the share locks of a preference write
// and the targets of the cleanup on archiving.

// ShareSpace implements app.Store.
func (r *Repository) ShareSpace(ctx context.Context, id ids.ID) (domain.Space, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.Space{}, false, err
	}
	row, err := q.ShareSpace(ctx, uuid(id))
	return one(row, err, "share-locking a space")
}

// ShareChannel implements app.Store.
func (r *Repository) ShareChannel(ctx context.Context, id ids.ID) (app.ChannelRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.ChannelRow{}, false, err
	}
	row, err := q.ShareChannel(ctx, uuid(id))
	return oneChannel(sqlcgen.GetChannelRow(row), err, "share-locking a channel")
}

// ChannelIDsOfSpace implements app.Store.
func (r *Repository) ChannelIDsOfSpace(ctx context.Context, space ids.ID) ([]ids.ID, error) {
	rows, err := r.read(ctx).ChannelIDsOfSpace(ctx, uuid(space))
	if err != nil {
		return nil, mapError(err, "reading the channels of a space")
	}
	return idsOf(rows), nil
}

// ThreadIDsOfChannels implements app.Store.
func (r *Repository) ThreadIDsOfChannels(ctx context.Context, channels []ids.ID) ([]ids.ID, error) {
	if len(channels) == 0 {
		return []ids.ID{}, nil
	}
	rows, err := r.read(ctx).ThreadIDsOfChannels(ctx, uuids(channels))
	if err != nil {
		return nil, mapError(err, "reading the threads of channels")
	}
	return idsOf(rows), nil
}

// idsOf converts a column list.
func idsOf(rows []pgtype.UUID) []ids.ID {
	out := make([]ids.ID, 0, len(rows))
	for _, u := range rows {
		out = append(out, idOf(u))
	}
	return out
}
