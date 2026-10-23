// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// exportLimit converts the page size of an export read; it refuses what the statements cannot
// take.
func exportLimit(n int) (int32, error) {
	if n < 1 || n > 1<<20 {
		return 0, errs.New(errs.CodeInternalError, "messaging: export page size out of range")
	}
	return int32(n), nil
}

// stamptz converts a bound of the export range.
func stamptz(q app.ExportRead) (from, to pgtype.Timestamptz) {
	return pgtype.Timestamptz{Time: q.From, Valid: true}, pgtype.Timestamptz{Time: q.To, Valid: true}
}

// ExportThreads implements app.Store (ListExportThreadIDs, brief M3-11 note 5).
func (r *Repository) ExportThreads(ctx context.Context, q app.ExportRead, after ids.ID) ([]ids.ID, error) {
	limit, err := exportLimit(q.Limit)
	if err != nil {
		return nil, err
	}
	from, to := stamptz(q)
	rows, err := r.read(ctx).ListExportThreadIDs(ctx, sqlcgen.ListExportThreadIDsParams{
		ChannelID: uuid(q.ChannelID), VisibleChannelIds: uuids(q.Visible), FromAt: from, ToAt: to, AfterThreadID: uuid(after), RowLimit: limit,
	})
	if err != nil {
		return nil, mapError(err, "reading the threads of an export")
	}
	out := make([]ids.ID, 0, len(rows))
	for _, id := range rows {
		out = append(out, idOf(id))
	}
	return out, nil
}

// ExportMessages implements app.Store (ListExportThreadMessages, brief M3-11 note 5).
func (r *Repository) ExportMessages(ctx context.Context, q app.ExportRead, c domain.ExportCursor) ([]domain.Message, error) {
	limit, err := exportLimit(q.Limit)
	if err != nil {
		return nil, err
	}
	from, to := stamptz(q)
	rows, err := r.read(ctx).ListExportThreadMessages(ctx, sqlcgen.ListExportThreadMessagesParams{
		ThreadID: uuid(c.ThreadID), ChannelID: uuid(q.ChannelID), VisibleChannelIds: uuids(q.Visible), FromAt: from, ToAt: to,
		AfterSeq: c.ThreadSeq, RowLimit: limit,
	})
	if err != nil {
		return nil, mapError(err, "reading the messages of an export")
	}
	out := make([]domain.Message, 0, len(rows))
	for _, row := range rows {
		out = append(out, toMessage(messageRow(row)))
	}
	return out, nil
}
