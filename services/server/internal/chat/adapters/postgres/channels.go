// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Channel implements app.Store.
func (r *Repository) Channel(ctx context.Context, id ids.ID) (app.ChannelRow, bool, error) {
	row, err := r.read(ctx).GetChannel(ctx, uuid(id))
	return oneChannel(row, err, "reading a channel")
}

// LockChannel implements app.Store.
func (r *Repository) LockChannel(ctx context.Context, id ids.ID) (app.ChannelRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.ChannelRow{}, false, err
	}
	row, err := q.LockChannel(ctx, uuid(id))
	return oneChannel(sqlcgen.GetChannelRow(row), err, "locking a channel")
}

// ChannelOrder implements app.Store.
func (r *Repository) ChannelOrder(ctx context.Context, space ids.ID) ([]domain.Keyed, error) {
	rows, err := r.read(ctx).ActiveChannelKeys(ctx, uuid(space))
	if err != nil {
		return nil, mapError(err, "reading the order of the channels")
	}
	out := make([]domain.Keyed, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Keyed{ID: idOf(row.ID), SortKey: row.SortKey})
	}
	return out, nil
}

// VisibleChannels implements app.Store.
func (r *Repository) VisibleChannels(ctx context.Context, space ids.ID, instanceWide bool, visible, fullyVisible []ids.ID) ([]app.ChannelRow, error) {
	rows, err := r.read(ctx).ListVisibleChannels(ctx, sqlcgen.ListVisibleChannelsParams{
		SpaceID: uuid(space), Visible: uuids(visible), FullyVisible: uuids(fullyVisible), InstanceWide: instanceWide,
	})
	if err != nil {
		return nil, mapError(err, "listing channels")
	}
	out := make([]app.ChannelRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, toChannel(sqlcgen.GetChannelRow(row)))
	}
	return out, nil
}

// AllChannels implements app.Store.
func (r *Repository) AllChannels(ctx context.Context) ([]app.ChannelRow, error) {
	rows, err := r.read(ctx).ListAllChannels(ctx)
	if err != nil {
		return nil, mapError(err, "reading the channels")
	}
	out := make([]app.ChannelRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, toChannel(sqlcgen.GetChannelRow(row)))
	}
	return out, nil
}

// InsertChannel implements app.Store.
func (r *Repository) InsertChannel(ctx context.Context, c domain.Channel) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	if c.HistoryBackfillLimit < 0 || c.HistoryBackfillLimit > math.MaxInt32 {
		return errs.New(errs.CodeInternalError, "chat: history_backfill_limit outside the integer column")
	}
	limit := int32(c.HistoryBackfillLimit)
	err = q.InsertChannel(ctx, sqlcgen.InsertChannelParams{
		ID: uuid(c.ID), SpaceID: optionalUUID(c.SpaceID), Kind: string(c.Kind), Name: c.Name, Topic: c.Topic, SortKey: c.SortKey,
		OperatorInherit: c.OperatorInherit, RequireMention: c.RequireMention, ThreadRequireMention: c.ThreadRequireMention,
		FreeResponse: c.FreeResponse, AutoThread: c.AutoThread, AllowMentionedAgents: c.AllowMentionedAgents,
		HistoryBackfillLimit: limit, ForumDefaultSort: optionalText(c.ForumDefaultSort),
		CreatedBy: uuid(c.CreatedBy), CreatedAt: pgtype.Timestamptz{Time: c.CreatedAt, Valid: true}, Version: c.Version,
		SettingsVersion: c.SettingsVersion,
	})
	return mapError(err, "inserting a channel")
}

// SaveChannel implements app.Store.
func (r *Repository) SaveChannel(ctx context.Context, c domain.Channel) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.SaveChannel(ctx, sqlcgen.SaveChannelParams{
		ID: uuid(c.ID), Name: c.Name, Topic: c.Topic, ForumDefaultSort: optionalText(c.ForumDefaultSort),
		DefaultNotificationLevel: optionalText(c.DefaultNotificationLevel), SortKey: c.SortKey,
		ArchivedAt: optionalTime(c.ArchivedAt), Version: c.Version,
	})
	return mapError(err, "updating a channel")
}

// InsertMainThread implements app.Store.
func (r *Repository) InsertMainThread(ctx context.Context, th domain.Thread) (domain.Thread, error) {
	if err := searchLanguageSet(th); err != nil {
		return domain.Thread{}, err
	}
	q, err := r.write(ctx)
	if err != nil {
		return domain.Thread{}, err
	}
	row, err := q.InsertMainThread(ctx, sqlcgen.InsertMainThreadParams{
		ID: uuid(th.ID), ChannelID: uuid(th.ChannelID), CreatedBy: uuid(th.CreatedBy),
		CreatedAt: pgtype.Timestamptz{Time: th.CreatedAt, Valid: true}, SearchLanguage: th.SearchLanguage,
	})
	if err != nil {
		return domain.Thread{}, mapError(err, "inserting a main thread")
	}
	return toThread(threadRow(row)), nil
}

// ForumTags implements app.Store.
func (r *Repository) ForumTags(ctx context.Context, channel ids.ID) ([]domain.ForumTag, error) {
	rows, err := r.read(ctx).ListForumTags(ctx, uuid(channel))
	if err != nil {
		return nil, mapError(err, "reading the forum tags")
	}
	out := make([]domain.ForumTag, 0, len(rows))
	for _, row := range rows {
		out = append(out, toTag(row))
	}
	return out, nil
}

// ForumTag implements app.Store.
func (r *Repository) ForumTag(ctx context.Context, id ids.ID) (domain.ForumTag, bool, error) {
	row, err := r.read(ctx).GetForumTag(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ForumTag{}, false, nil
	}
	if err != nil {
		return domain.ForumTag{}, false, mapError(err, "reading a forum tag")
	}
	return toTag(row), true, nil
}

// InsertForumTag implements app.Store.
func (r *Repository) InsertForumTag(ctx context.Context, t domain.ForumTag) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.InsertForumTag(ctx, sqlcgen.InsertForumTagParams{
		ID: uuid(t.ID), ChannelID: uuid(t.ChannelID), Name: t.Name, Color: optionalText(t.Color), SortKey: t.SortKey, Restricted: t.Restricted,
	})
	return mapError(err, "inserting a forum tag")
}

// SaveForumTag implements app.Store.
func (r *Repository) SaveForumTag(ctx context.Context, t domain.ForumTag) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.SaveForumTag(ctx, sqlcgen.SaveForumTagParams{
		ID: uuid(t.ID), Name: t.Name, Color: optionalText(t.Color), SortKey: t.SortKey, Restricted: t.Restricted,
		DeletedAt: optionalTime(t.DeletedAt),
	})
	return mapError(err, "updating a forum tag")
}

// oneChannel converts the result of a single-channel query; no rows is found false.
func oneChannel(row sqlcgen.GetChannelRow, err error, what string) (app.ChannelRow, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ChannelRow{}, false, nil
	}
	if err != nil {
		return app.ChannelRow{}, false, mapError(err, what)
	}
	return toChannel(row), true, nil
}

// toChannel converts a channel row with its space facts and main thread.
func toChannel(row sqlcgen.GetChannelRow) app.ChannelRow {
	c := row.Channel
	return app.ChannelRow{
		Channel: domain.Channel{
			ID: idOf(c.ID), SpaceID: idOf(c.SpaceID), Kind: domain.ChannelKind(c.Kind), Name: c.Name, Topic: c.Topic, SortKey: c.SortKey,
			OperatorInherit: c.OperatorInherit, OperatorAgentID: idOf(c.OperatorAgentID), RequireMention: c.RequireMention,
			ThreadRequireMention: c.ThreadRequireMention, FreeResponse: c.FreeResponse, AutoThread: c.AutoThread,
			AllowMentionedAgents: c.AllowMentionedAgents, HistoryBackfillLimit: int(c.HistoryBackfillLimit),
			ForumDefaultSort: c.ForumDefaultSort.String, DefaultNotificationLevel: c.DefaultNotificationLevel.String, ReadOnly: c.ReadOnly,
			MainThreadID: idOf(row.MainThreadID), CreatedBy: idOf(c.CreatedBy), CreatedAt: timeOf(c.CreatedAt), ArchivedAt: timeOf(c.ArchivedAt),
			Version: c.Version, SettingsVersion: c.SettingsVersion,
		},
		SpaceDefaultOperatorID: idOf(row.SpaceOperatorID),
		SpaceArchived:          row.SpaceArchived,
	}
}

// toTag converts a forum tag row.
func toTag(row sqlcgen.ForumTag) domain.ForumTag {
	return domain.ForumTag{
		ID: idOf(row.ID), ChannelID: idOf(row.ChannelID), Name: row.Name, Color: row.Color.String, SortKey: row.SortKey,
		Restricted: row.Restricted, DeletedAt: timeOf(row.DeletedAt),
	}
}

// uuids converts ids for an array parameter; never nil, so the array is empty rather than NULL.
func uuids(list []ids.ID) []pgtype.UUID {
	out := make([]pgtype.UUID, 0, len(list))
	for _, id := range list {
		out = append(out, uuid(id))
	}
	return out
}

// optionalTime converts an optional time: the zero time becomes NULL.
func optionalTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}
