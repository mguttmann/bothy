// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// MyMentions implements app.Store.
func (r *Repository) MyMentions(ctx context.Context, q app.MentionQuery) ([]app.MentionRow, error) {
	rows, err := r.read(ctx).ListMyMentions(ctx, sqlcgen.ListMyMentionsParams{
		PrincipalID: uuid(q.PrincipalID), VisibleChannelIds: uuids(q.Visible), HasAfter: q.HasAfter,
		AfterAt: pgtype.Timestamptz{Time: q.AfterAt, Valid: true}, AfterID: uuid(q.AfterID), RowLimit: int32(min(q.Limit, 1<<30)), //nolint:gosec // G115: bounded above
	})
	if err != nil {
		return nil, mapError(err, "listing mentions")
	}
	out := make([]app.MentionRow, 0, len(rows))
	for _, row := range rows {
		m := messageRow{
			ID: row.ID, ThreadID: row.ThreadID, ChannelID: row.ChannelID, ThreadSeq: row.ThreadSeq,
			AuthorPrincipalID: row.AuthorPrincipalID, ClientMsgID: row.ClientMsgID, ClientPayloadSha256: row.ClientPayloadSha256,
			Origin: row.Origin, Kind: row.Kind, SystemCode: row.SystemCode, ContentMd: row.ContentMd,
			ContentFormat: row.ContentFormat, ReplyToMessageID: row.ReplyToMessageID, RunID: row.RunID,
			TriggerState: row.TriggerState, TriggerStateReason: row.TriggerStateReason, TriggerStateAt: row.TriggerStateAt,
			Silent: row.Silent, ImportItemID: row.ImportItemID, Version: row.Version, EditedAt: row.EditedAt,
			DeletedAt: row.DeletedAt, DeletedBy: row.DeletedBy, CreatedAt: row.CreatedAt, HasAttachments: row.HasAttachments,
		}
		out = append(out, app.MentionRow{Message: toMessage(m), MentionedAt: timeOf(row.MentionedAt)})
	}
	return out, nil
}

// CountUnreadMentions implements app.Store: the map goes in as two parallel arrays.
func (r *Repository) CountUnreadMentions(ctx context.Context, principal ids.ID, visible []ids.ID, lastRead map[ids.ID]int64) ([]messaging.ThreadMentionCount, error) {
	threads := make([]pgtype.UUID, 0, len(lastRead))
	seqs := make([]int64, 0, len(lastRead))
	for th, seq := range lastRead {
		threads = append(threads, uuid(th))
		seqs = append(seqs, seq)
	}
	rows, err := r.read(ctx).CountUnreadMentions(ctx, sqlcgen.CountUnreadMentionsParams{
		ReadThreadIds: threads, ReadSeqs: seqs, PrincipalID: uuid(principal), VisibleChannelIds: uuids(visible),
	})
	if err != nil {
		return nil, mapError(err, "counting unread mentions")
	}
	out := make([]messaging.ThreadMentionCount, 0, len(rows))
	for _, row := range rows {
		out = append(out, messaging.ThreadMentionCount{ThreadID: idOf(row.ThreadID), ChannelID: idOf(row.ChannelID), Count: int(row.MentionCount)})
	}
	return out, nil
}

// PersonalTemplates implements app.Store.
func (r *Repository) PersonalTemplates(ctx context.Context, owner ids.ID) ([]domain.Template, error) {
	rows, err := r.read(ctx).ListPersonalTemplates(ctx, uuid(owner))
	return templates(rows, err, "listing personal templates")
}

// ChannelTemplates implements app.Store; a zero caller matches no personal template.
func (r *Repository) ChannelTemplates(ctx context.Context, caller, channel ids.ID, visible []ids.ID) ([]domain.Template, error) {
	rows, err := r.read(ctx).ListChannelTemplates(ctx, sqlcgen.ListChannelTemplatesParams{
		CallerID: optionalUUID(caller), ChannelID: uuid(channel), VisibleChannelIds: uuids(visible),
	})
	return templates(rows, err, "listing channel templates")
}

// LockVisibleTemplate implements app.Store.
func (r *Repository) LockVisibleTemplate(ctx context.Context, id, caller ids.ID, visible []ids.ID, withDeleted bool) (domain.Template, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.Template{}, false, err
	}
	row, err := q.GetVisibleTemplate(ctx, sqlcgen.GetVisibleTemplateParams{
		ID: uuid(id), CallerID: uuid(caller), VisibleChannelIds: uuids(visible), IncludeDeleted: withDeleted,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Template{}, false, nil
	}
	if err != nil {
		return domain.Template{}, false, mapError(err, "reading a template")
	}
	return toTemplate(row), true, nil
}

// LockTemplateScope implements app.Store with pg_advisory_xact_lock.
func (r *Repository) LockTemplateScope(ctx context.Context, key int64) error {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", key)
	return mapError(err, "locking a template scope")
}

// CountTemplates implements app.Store.
func (r *Repository) CountTemplates(ctx context.Context, t domain.Template) (int, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, err
	}
	var n int32
	if t.Scope() == domain.ScopePersonal {
		n, err = q.CountPersonalTemplates(ctx, uuid(t.OwnerUserID))
	} else {
		n, err = q.CountChannelTemplates(ctx, uuid(t.ChannelID))
	}
	return int(n), mapError(err, "counting templates")
}

// InsertTemplate implements app.Store.
func (r *Repository) InsertTemplate(ctx context.Context, t domain.Template) (domain.Template, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.Template{}, err
	}
	row, err := q.InsertTemplate(ctx, sqlcgen.InsertTemplateParams{
		ID: uuid(t.ID), OwnerUserID: optionalUUID(t.OwnerUserID), ChannelID: optionalUUID(t.ChannelID), Name: t.Name,
		ContentMd: t.ContentMD, CreatedBy: uuid(t.CreatedBy), CreatedAt: pgtype.Timestamptz{Time: t.CreatedAt, Valid: true},
	})
	if err != nil {
		return domain.Template{}, mapError(err, "inserting a template")
	}
	return toTemplate(row), nil
}

// UpdateTemplate implements app.Store.
func (r *Repository) UpdateTemplate(ctx context.Context, t domain.Template) (domain.Template, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.Template{}, err
	}
	row, err := q.UpdateTemplate(ctx, sqlcgen.UpdateTemplateParams{Name: t.Name, ContentMd: t.ContentMD, ID: uuid(t.ID)})
	if err != nil {
		return domain.Template{}, mapError(err, "updating a template")
	}
	return toTemplate(row), nil
}

// DeleteTemplate implements app.Store.
func (r *Repository) DeleteTemplate(ctx context.Context, id ids.ID, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	_, err = q.DeleteTemplate(ctx, sqlcgen.DeleteTemplateParams{DeletedAt: pgtype.Timestamptz{Time: at, Valid: true}, ID: uuid(id)})
	return mapError(err, "deleting a template")
}

// templates converts a list result.
func templates(rows []sqlcgen.MessageTemplate, err error, what string) ([]domain.Template, error) {
	if err != nil {
		return nil, mapError(err, what)
	}
	out := make([]domain.Template, 0, len(rows))
	for _, row := range rows {
		out = append(out, toTemplate(row))
	}
	return out, nil
}

// toTemplate converts a row.
func toTemplate(row sqlcgen.MessageTemplate) domain.Template {
	return domain.Template{
		ID: idOf(row.ID), OwnerUserID: idOf(row.OwnerUserID), ChannelID: idOf(row.ChannelID), Name: row.Name, ContentMD: row.ContentMd,
		CreatedBy: idOf(row.CreatedBy), CreatedAt: timeOf(row.CreatedAt), DeletedAt: timeOf(row.DeletedAt), Version: row.Version,
	}
}

// uuids converts ids for an array parameter; never nil, so ANY sees an empty array, not NULL.
func uuids(list []ids.ID) []pgtype.UUID {
	out := make([]pgtype.UUID, 0, len(list))
	for _, id := range list {
		out = append(out, uuid(id))
	}
	return out
}
