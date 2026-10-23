// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// historyRow is the common row of the four history statements (queries/history.sql); the
// generated row types have the same fields and convert to it. ID is NULL in the one row of an
// empty page.
type historyRow sqlcgen.ListThreadNewestRow

// History implements app.Store: one statement per mode, so the rows and the opposite flag share a
// snapshot (brief M1-57 note 4).
func (r *Repository) History(ctx context.Context, q app.HistoryRead) (app.HistoryRows, error) {
	if q.RowLimit < 1 || q.RowLimit > 1<<20 {
		return app.HistoryRows{}, errs.New(errs.CodeInternalError, "messaging: history row limit out of range")
	}
	db := r.read(ctx)
	visible := uuids(q.Visible)
	limit := int32(q.RowLimit)
	var rows []historyRow
	var err error
	switch q.Mode {
	case domain.HistoryNewest:
		var got []sqlcgen.ListThreadNewestRow
		got, err = db.ListThreadNewest(ctx, sqlcgen.ListThreadNewestParams{ThreadID: uuid(q.ThreadID), VisibleChannelIds: visible, RowLimit: limit})
		rows = convert(got, func(x sqlcgen.ListThreadNewestRow) historyRow { return historyRow(x) })
	case domain.HistoryBefore:
		var got []sqlcgen.ListThreadBeforeRow
		got, err = db.ListThreadBefore(ctx, sqlcgen.ListThreadBeforeParams{ThreadID: uuid(q.ThreadID), VisibleChannelIds: visible, Anchor: q.Anchor, RowLimit: limit})
		rows = convert(got, func(x sqlcgen.ListThreadBeforeRow) historyRow { return historyRow(x) })
	case domain.HistoryAfter:
		var got []sqlcgen.ListThreadAfterRow
		got, err = db.ListThreadAfter(ctx, sqlcgen.ListThreadAfterParams{ThreadID: uuid(q.ThreadID), VisibleChannelIds: visible, Anchor: q.Anchor, RowLimit: limit})
		rows = convert(got, func(x sqlcgen.ListThreadAfterRow) historyRow { return historyRow(x) })
	case domain.HistoryAround:
		var got []sqlcgen.ListThreadAroundRow
		got, err = db.ListThreadAround(ctx, sqlcgen.ListThreadAroundParams{ThreadID: uuid(q.ThreadID), VisibleChannelIds: visible, Anchor: q.Anchor, RowLimit: limit})
		rows = convert(got, func(x sqlcgen.ListThreadAroundRow) historyRow { return historyRow(x) })
	default:
		return app.HistoryRows{}, errs.New(errs.CodeInternalError, "messaging: unknown history mode")
	}
	if err != nil {
		return app.HistoryRows{}, mapError(err, "reading the history")
	}
	var out app.HistoryRows
	for _, row := range rows {
		out.Opposite = row.Opposite
		if !row.ID.Valid {
			continue
		}
		if row.BeforeSide {
			out.Below = append(out.Below, historyMessage(row))
		} else {
			out.Above = append(out.Above, historyMessage(row))
		}
	}
	// The statements order by thread_seq ascending; the side below the anchor is read downwards.
	slices.Reverse(out.Below)
	return out, nil
}

// VisibleMessage implements app.Store.
func (r *Repository) VisibleMessage(ctx context.Context, id ids.ID, visible []ids.ID) (domain.Message, bool, error) {
	row, err := r.read(ctx).GetVisibleMessage(ctx, sqlcgen.GetVisibleMessageParams{ID: uuid(id), VisibleChannelIds: uuids(visible)})
	return one(messageRow(row), err, "reading a message")
}

// convert maps the generated rows to historyRow.
func convert[T any](in []T, f func(T) historyRow) []historyRow {
	out := make([]historyRow, 0, len(in))
	for _, x := range in {
		out = append(out, f(x))
	}
	return out
}

// historyMessage converts a non-empty history row; the history statements leave out the request
// hash, which only the idempotency lookup needs.
func historyMessage(row historyRow) domain.Message {
	return domain.Message{
		ID: idOf(row.ID), ThreadID: idOf(row.ThreadID), ChannelID: idOf(row.ChannelID), ThreadSeq: row.ThreadSeq.Int64,
		AuthorID: idOf(row.AuthorPrincipalID), ClientMsgID: idOf(row.ClientMsgID), Origin: domain.Origin(row.Origin.String),
		Kind: domain.Kind(row.Kind.String), SystemCode: row.SystemCode.String, ContentMD: row.ContentMd.String,
		ContentFormat: row.ContentFormat.Int16, ReplyToID: idOf(row.ReplyToMessageID), RunID: idOf(row.RunID),
		TriggerState: row.TriggerState.String, TriggerStateReason: row.TriggerStateReason.String, TriggerStateAt: timeOf(row.TriggerStateAt),
		Silent: row.Silent.Bool, Version: row.Version.Int64,
		EditedAt: timeOf(row.EditedAt), DeletedAt: timeOf(row.DeletedAt), DeletedBy: idOf(row.DeletedBy), CreatedAt: timeOf(row.CreatedAt),
		HasAttachments: row.HasAttachments.Bool,
	}
}
