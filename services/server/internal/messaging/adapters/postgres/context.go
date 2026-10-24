// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"math"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// ContextWindow implements app.Store with the one statement ListContextWindow (M2-13 design note
// 3), so the rows and the flag of older rows share a snapshot. Outside a unit of work it reads on
// the pool. Errors: internal for a limit outside 0..MaxInt32.
func (r *Repository) ContextWindow(ctx context.Context, q app.ContextRead) (app.ContextRows, error) {
	if q.Limit < 0 || q.Limit > math.MaxInt32 {
		return app.ContextRows{}, errs.New(errs.CodeInternalError, "messaging: context window limit out of range")
	}
	rows, err := r.read(ctx).ListContextWindow(ctx, sqlcgen.ListContextWindowParams{ThreadID: uuid(q.ThreadID), AfterSeq: q.AfterSeq,
		BeforeSeq: q.BeforeSeq, ExcludeAuthor: optionalUUID(q.ExcludeAuthorID), RowLimit: int32(q.Limit)})
	if err != nil {
		return app.ContextRows{}, mapError(err, "reading a context window")
	}
	out := app.ContextRows{Items: make([]domain.Message, 0, len(rows))}
	for _, row := range rows {
		out.HasOlder = row.HasOlder
		if !row.ID.Valid {
			continue
		}
		out.Items = append(out.Items, historyMessage(historyRow{
			ID: row.ID, ThreadID: row.ThreadID, ChannelID: row.ChannelID, ThreadSeq: row.ThreadSeq, AuthorPrincipalID: row.AuthorPrincipalID,
			ClientMsgID: row.ClientMsgID, Origin: row.Origin, Kind: row.Kind, SystemCode: row.SystemCode, ContentMd: row.ContentMd,
			ContentFormat: row.ContentFormat, ReplyToMessageID: row.ReplyToMessageID, RunID: row.RunID, TriggerState: row.TriggerState,
			TriggerStateReason: row.TriggerStateReason, TriggerStateAt: row.TriggerStateAt, Silent: row.Silent, Version: row.Version,
			EditedAt: row.EditedAt, DeletedAt: row.DeletedAt, DeletedBy: row.DeletedBy, CreatedAt: row.CreatedAt,
		}))
	}
	return out, nil
}
