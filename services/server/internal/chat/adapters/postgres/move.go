// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ShareChannel implements app.Store (thread.move, M1-62).
func (r *Repository) ShareChannel(ctx context.Context, id ids.ID) (app.ChannelRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.ChannelRow{}, false, err
	}
	row, err := q.ShareChannel(ctx, uuid(id))
	return oneChannel(sqlcgen.GetChannelRow(row), err, "share-locking a channel")
}

// ShareSpace implements app.Store (thread.move, M1-62).
func (r *Repository) ShareSpace(ctx context.Context, id ids.ID) (domain.Space, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.Space{}, false, err
	}
	row, err := q.ShareSpace(ctx, uuid(id))
	return one(row, err, "share-locking a space")
}

// MoveThread implements app.Store; no row (an unknown thread) is internal.
func (r *Repository) MoveThread(ctx context.Context, id, channel ids.ID, at time.Time) (domain.Thread, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.Thread{}, err
	}
	row, err := q.MoveThread(ctx, sqlcgen.MoveThreadParams{ID: uuid(id), ChannelID: uuid(channel), MovedAt: pgtype.Timestamptz{Time: at, Valid: true}})
	if err != nil {
		return domain.Thread{}, mapError(err, "moving a thread")
	}
	return toThread(threadRow(row)), nil
}

// DeleteThreadTags implements app.Store.
func (r *Repository) DeleteThreadTags(ctx context.Context, thread ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.DeleteThreadTagsOfThread(ctx, uuid(thread)), "deleting the tags of a thread")
}
