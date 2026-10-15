// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// The statements of reactions and pins (M1-59, queries/reactions.sql and queries/pins.sql). Every
// one of them joins messages with the visible channels of the caller (brief design note 13).

// ReactionStats implements app.Store.
func (r *Repository) ReactionStats(ctx context.Context, message ids.ID, emoji string, visible []ids.ID) (int, int, error) {
	row, err := r.read(ctx).ReactionStats(ctx, sqlcgen.ReactionStatsParams{Emoji: emoji, MessageID: uuid(message), Visible: uuids(visible)})
	if err != nil {
		return 0, 0, mapError(err, "counting reactions")
	}
	return int(row.DistinctEmoji), int(row.EmojiCount), nil
}

// InsertReaction implements app.Store.
func (r *Repository) InsertReaction(ctx context.Context, message, principal ids.ID, emoji string, at time.Time, visible []ids.ID) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.InsertReaction(ctx, sqlcgen.InsertReactionParams{
		PrincipalID: uuid(principal), Emoji: emoji, CreatedAt: pgtype.Timestamptz{Time: at, Valid: true},
		MessageID: uuid(message), Visible: uuids(visible),
	})
	return n > 0, mapError(err, "inserting a reaction")
}

// DeleteReaction implements app.Store.
func (r *Repository) DeleteReaction(ctx context.Context, message, principal ids.ID, emoji string, visible []ids.ID) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.DeleteReaction(ctx, sqlcgen.DeleteReactionParams{
		MessageID: uuid(message), Visible: uuids(visible), PrincipalID: uuid(principal), Emoji: emoji,
	})
	return n > 0, mapError(err, "deleting a reaction")
}

// DeleteReactions implements app.Store.
func (r *Repository) DeleteReactions(ctx context.Context, message ids.ID, visible []ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.DeleteMessageReactions(ctx, sqlcgen.DeleteMessageReactionsParams{MessageID: uuid(message), Visible: uuids(visible)}),
		"deleting the reactions of a message")
}

// ReactionGroups implements app.Store.
func (r *Repository) ReactionGroups(ctx context.Context, messages, visible []ids.ID) (map[ids.ID][]domain.ReactionGroup, error) {
	rows, err := r.read(ctx).ReactionGroups(ctx, sqlcgen.ReactionGroupsParams{MessageIds: uuids(messages), Visible: uuids(visible)})
	if err != nil {
		return nil, mapError(err, "reading reaction summaries")
	}
	out := make(map[ids.ID][]domain.ReactionGroup, len(messages))
	for _, row := range rows {
		id := idOf(row.MessageID)
		out[id] = append(out[id], domain.ReactionGroup{Emoji: row.Emoji, Count: int(row.ReactionCount), FirstAt: timeOf(row.FirstAt)})
	}
	return out, nil
}

// OwnReactions implements app.Store.
func (r *Repository) OwnReactions(ctx context.Context, principal ids.ID, messages, visible []ids.ID) (map[ids.ID]map[string]bool, error) {
	rows, err := r.read(ctx).OwnReactions(ctx, sqlcgen.OwnReactionsParams{
		PrincipalID: uuid(principal), MessageIds: uuids(messages), Visible: uuids(visible),
	})
	if err != nil {
		return nil, mapError(err, "reading own reactions")
	}
	out := make(map[ids.ID]map[string]bool, len(messages))
	for _, row := range rows {
		id := idOf(row.MessageID)
		if out[id] == nil {
			out[id] = map[string]bool{}
		}
		out[id][row.Emoji] = true
	}
	return out, nil
}

// ListReactions implements app.Store.
func (r *Repository) ListReactions(ctx context.Context, q app.ReactionRead) ([]app.ReactionRow, error) {
	if q.Limit < 1 || q.Limit > 1<<20 {
		return nil, errs.New(errs.CodeInternalError, "messaging: reaction row limit out of range")
	}
	p := sqlcgen.ListReactionsParams{
		MessageID: uuid(q.MessageID), Visible: uuids(q.Visible), Emoji: q.Emoji, HasAfter: q.HasAfter,
		AfterAt: pgtype.Timestamptz{Time: q.After.At, Valid: true}, AfterPrincipal: uuid(q.After.PrincipalID), AfterEmoji: q.After.Emoji,
		RowLimit: int32(q.Limit),
	}
	rows, err := r.read(ctx).ListReactions(ctx, p)
	if err != nil {
		return nil, mapError(err, "listing reactions")
	}
	out := make([]app.ReactionRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.ReactionRow{PrincipalID: idOf(row.PrincipalID), Emoji: row.Emoji, CreatedAt: timeOf(row.CreatedAt)})
	}
	return out, nil
}

// Pinned implements app.Store.
func (r *Repository) Pinned(ctx context.Context, messages, visible []ids.ID) (map[ids.ID]bool, error) {
	rows, err := r.read(ctx).PinnedMessages(ctx, sqlcgen.PinnedMessagesParams{MessageIds: uuids(messages), Visible: uuids(visible)})
	if err != nil {
		return nil, mapError(err, "reading pins")
	}
	out := make(map[ids.ID]bool, len(rows))
	for _, id := range rows {
		out[idOf(id)] = true
	}
	return out, nil
}

// Pin implements app.Store.
func (r *Repository) Pin(ctx context.Context, message ids.ID, visible []ids.ID) (app.PinRow, bool, error) {
	row, err := r.read(ctx).GetPin(ctx, sqlcgen.GetPinParams{MessageID: uuid(message), Visible: uuids(visible)})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.PinRow{}, false, nil
	}
	if err != nil {
		return app.PinRow{}, false, mapError(err, "reading a pin")
	}
	return pinOf(row), true, nil
}

// CountPins implements app.Store.
func (r *Repository) CountPins(ctx context.Context, thread ids.ID, visible []ids.ID) (int, error) {
	n, err := r.read(ctx).CountThreadPins(ctx, sqlcgen.CountThreadPinsParams{ThreadID: uuid(thread), Visible: uuids(visible)})
	return int(n), mapError(err, "counting pins")
}

// InsertPin implements app.Store.
func (r *Repository) InsertPin(ctx context.Context, p app.PinRow, visible []ids.ID) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.InsertPin(ctx, sqlcgen.InsertPinParams{
		PinnedBy: uuid(p.PinnedBy), PinnedAt: pgtype.Timestamptz{Time: p.PinnedAt, Valid: true}, MessageID: uuid(p.MessageID), Visible: uuids(visible),
	})
	return n > 0, mapError(err, "inserting a pin")
}

// DeletePin implements app.Store.
func (r *Repository) DeletePin(ctx context.Context, message ids.ID, visible []ids.ID) (app.PinRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.PinRow{}, false, err
	}
	rows, err := q.DeletePin(ctx, sqlcgen.DeletePinParams{MessageID: uuid(message), Visible: uuids(visible)})
	if err != nil {
		return app.PinRow{}, false, mapError(err, "deleting a pin")
	}
	if len(rows) == 0 {
		return app.PinRow{}, false, nil
	}
	return pinOf(rows[0]), true, nil
}

// ThreadPins implements app.Store.
func (r *Repository) ThreadPins(ctx context.Context, thread ids.ID, visible []ids.ID) ([]app.PinRow, error) {
	rows, err := r.read(ctx).ListThreadPins(ctx, sqlcgen.ListThreadPinsParams{ThreadID: uuid(thread), Visible: uuids(visible)})
	if err != nil {
		return nil, mapError(err, "listing pins")
	}
	out := make([]app.PinRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.PinRow{
			MessageID: idOf(row.MessageID), ThreadID: idOf(row.ThreadID), PinnedBy: idOf(row.PinnedBy), PinnedAt: timeOf(row.PinnedAt),
			ChannelID: idOf(row.ChannelID), AuthorID: idOf(row.AuthorPrincipalID),
		})
	}
	return out, nil
}

// pinOf converts a row of pins.
func pinOf(p sqlcgen.Pin) app.PinRow {
	return app.PinRow{MessageID: idOf(p.MessageID), ThreadID: idOf(p.ThreadID), PinnedBy: idOf(p.PinnedBy), PinnedAt: timeOf(p.PinnedAt)}
}
