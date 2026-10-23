// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// Compile-time check: the store of the imported messages (M8-11).
var _ app.ImportStore = (*Repository)(nil)

// InsertImportedMessage implements app.ImportStore with the query of queries/import.sql.
func (r *Repository) InsertImportedMessage(ctx context.Context, m domain.Message) (domain.Message, error) {
	if m.SearchLanguage == "" || m.ImportItemID.IsZero() {
		return domain.Message{}, errs.New(errs.CodeInternalError, "messaging: an imported message without its search language or item")
	}
	q, err := r.write(ctx)
	if err != nil {
		return domain.Message{}, err
	}
	row, err := q.InsertImportedMessage(ctx, sqlcgen.InsertImportedMessageParams{
		ID: uuid(m.ID), ThreadID: uuid(m.ThreadID), ChannelID: uuid(m.ChannelID), ThreadSeq: m.ThreadSeq,
		AuthorPrincipalID: uuid(m.AuthorID), ClientMsgID: uuid(m.ClientMsgID), ClientPayloadSha256: m.PayloadSHA256,
		ContentMd: m.ContentMD, ContentFormat: m.ContentFormat, ReplyToMessageID: optionalUUID(m.ReplyToID), ImportItemID: uuid(m.ImportItemID),
		EditedAt: pgtype.Timestamptz{Time: m.EditedAt, Valid: !m.EditedAt.IsZero()}, CreatedAt: pgtype.Timestamptz{Time: m.CreatedAt, Valid: true},
		SearchLanguage: m.SearchLanguage, HasAttachments: m.HasAttachments,
	})
	if err != nil {
		return domain.Message{}, mapError(err, "inserting an imported message")
	}
	return toMessage(messageRow(row)), nil
}
