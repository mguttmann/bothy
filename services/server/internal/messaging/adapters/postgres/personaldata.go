// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Compile-time check: the repository is the store of the personal data participant (M7-06).
var _ app.PersonalStore = (*Repository)(nil)

// OwnMessages implements app.PersonalStore.
func (r *Repository) OwnMessages(ctx context.Context, author, after ids.ID, limit int) ([]app.PersonalMessage, error) {
	rows, err := r.read(ctx).ListOwnMessages(ctx, sqlcgen.ListOwnMessagesParams{AuthorID: uuid(author), AfterID: uuid(after), RowLimit: pg.Limit(limit)})
	if err != nil {
		return nil, mapError(err, "reading the messages of a person")
	}
	out := make([]app.PersonalMessage, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.PersonalMessage{
			ID: idOf(row.ID), ThreadID: idOf(row.ThreadID), ChannelID: idOf(row.ChannelID), Kind: row.Kind, ContentMD: row.ContentMd,
			HasAttachments: row.HasAttachments, CreatedAt: timeOf(row.CreatedAt), EditedAt: timeOf(row.EditedAt), DeletedAt: timeOf(row.DeletedAt),
		})
	}
	return out, nil
}

// OwnRevisions implements app.PersonalStore.
func (r *Repository) OwnRevisions(ctx context.Context, author ids.ID, messages []ids.ID) ([]domain.Revision, error) {
	rows, err := r.read(ctx).ListOwnRevisions(ctx, sqlcgen.ListOwnRevisionsParams{MessageIds: uuids(messages), AuthorID: uuid(author)})
	if err != nil {
		return nil, mapError(err, "reading the revisions of a person")
	}
	out := make([]domain.Revision, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Revision{
			MessageID: idOf(row.MessageID), Number: int(row.Revision), ContentMD: row.ContentMd, EditedBy: author, CreatedAt: timeOf(row.CreatedAt),
		})
	}
	return out, nil
}

// PersonalReactions implements app.PersonalStore.
func (r *Repository) PersonalReactions(ctx context.Context, principal ids.ID, after app.PersonalReaction, limit int) ([]app.PersonalReaction, error) {
	rows, err := r.read(ctx).ListOwnReactions(ctx, sqlcgen.ListOwnReactionsParams{
		PrincipalID: uuid(principal), AfterMessageID: uuid(after.MessageID), AfterEmoji: after.Emoji, RowLimit: pg.Limit(limit),
	})
	if err != nil {
		return nil, mapError(err, "reading the reactions of a person")
	}
	out := make([]app.PersonalReaction, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.PersonalReaction{MessageID: idOf(row.MessageID), Emoji: row.Emoji, CreatedAt: timeOf(row.CreatedAt)})
	}
	return out, nil
}

// MentionsOf implements app.PersonalStore.
func (r *Repository) MentionsOf(ctx context.Context, principal, after ids.ID, limit int) ([]app.PersonalMention, error) {
	rows, err := r.read(ctx).ListMentionsOf(ctx, sqlcgen.ListMentionsOfParams{PrincipalID: uuid(principal), AfterMessageID: uuid(after), RowLimit: pg.Limit(limit)})
	if err != nil {
		return nil, mapError(err, "reading the mentions of a person")
	}
	out := make([]app.PersonalMention, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.PersonalMention{MessageID: idOf(row.MessageID), CreatedAt: timeOf(row.CreatedAt)})
	}
	return out, nil
}

// AllPersonalTemplates implements app.PersonalStore.
func (r *Repository) AllPersonalTemplates(ctx context.Context, owner ids.ID) ([]domain.Template, error) {
	rows, err := r.read(ctx).ListAllPersonalTemplates(ctx, uuid(owner))
	if err != nil {
		return nil, mapError(err, "reading the personal templates of a person")
	}
	out := make([]domain.Template, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Template{
			ID: idOf(row.ID), OwnerUserID: owner, Name: row.Name, ContentMD: row.ContentMd, CreatedAt: timeOf(row.CreatedAt), DeletedAt: timeOf(row.DeletedAt),
		})
	}
	return out, nil
}

// DeletePersonalTemplates implements app.PersonalStore: one statement on the pool (or in the
// ambient transaction of ctx when there is one).
func (r *Repository) DeletePersonalTemplates(ctx context.Context, owner ids.ID) (int64, error) {
	n, err := sqlcgen.New(pg.Querier(ctx, r.pool)).DeletePersonalTemplates(ctx, uuid(owner))
	return n, mapError(err, "deleting the personal templates of a person")
}

// LiveOwnMessages implements app.PersonalStore.
func (r *Repository) LiveOwnMessages(ctx context.Context, author ids.ID, limit int) ([]app.ExpiredMessage, error) {
	rows, err := r.read(ctx).ListLiveOwnMessages(ctx, sqlcgen.ListLiveOwnMessagesParams{AuthorID: uuid(author), RowLimit: pg.Limit(limit)})
	if err != nil {
		return nil, mapError(err, "reading the live messages of a person")
	}
	out := make([]app.ExpiredMessage, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.ExpiredMessage{ID: idOf(row.ID), ChannelID: idOf(row.ChannelID)})
	}
	return out, nil
}

// LiveOwnReactions implements app.PersonalStore.
func (r *Repository) LiveOwnReactions(ctx context.Context, principal ids.ID, limit int) ([]app.OwnReaction, error) {
	rows, err := r.read(ctx).ListLiveOwnReactions(ctx, sqlcgen.ListLiveOwnReactionsParams{PrincipalID: uuid(principal), RowLimit: pg.Limit(limit)})
	if err != nil {
		return nil, mapError(err, "reading the live reactions of a person")
	}
	out := make([]app.OwnReaction, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.OwnReaction{MessageID: idOf(row.MessageID), ChannelID: idOf(row.ChannelID), Emoji: row.Emoji})
	}
	return out, nil
}
