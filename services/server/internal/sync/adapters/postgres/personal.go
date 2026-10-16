// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// PersonalRepository implements app.PersonalStore over the sqlc queries of read_states, drafts
// and user_settings (M1-61). Reads use the ambient transaction of ctx when there is one and the
// pool otherwise; writes and locks need a unit of work. Errors are kernel/errs errors without SQL
// text or values. It is safe for concurrent use.
type PersonalRepository struct {
	pool *pg.Pool
}

// Compile-time check: the repository is the outgoing port of app.
var _ app.PersonalStore = (*PersonalRepository)(nil)

// NewPersonalRepository returns the repository over pool; it opens no connection.
func NewPersonalRepository(pool *pg.Pool) *PersonalRepository { return &PersonalRepository{pool: pool} }

// read returns the queries on the ambient transaction or the pool.
func (r *PersonalRepository) read(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(pg.Querier(ctx, r.pool))
}

// write returns the queries on the transaction of the unit of work; internal outside one.
func (r *PersonalRepository) write(ctx context.Context) (*sqlcgen.Queries, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return nil, err
	}
	return sqlcgen.New(tx), nil
}

// failed wraps a database error of what as internal; errs errors pass unchanged.
func failed(err error, what string) error {
	if _, ok := errs.As(err); ok {
		return err
	}
	return errs.Wrap(errs.CodeInternalError, err, "sync: "+what+" failed")
}

// ReadState implements app.PersonalStore.
func (r *PersonalRepository) ReadState(ctx context.Context, user, thread ids.ID) (sync.ReadState, bool, error) {
	row, err := r.read(ctx).GetReadState(ctx, sqlcgen.GetReadStateParams{UserID: uuid(user), ThreadID: uuid(thread)})
	return oneReadState(row, err, "reading a read state")
}

// LockReadState implements app.PersonalStore.
func (r *PersonalRepository) LockReadState(ctx context.Context, user, thread ids.ID) (sync.ReadState, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return sync.ReadState{}, false, err
	}
	row, err := q.LockReadState(ctx, sqlcgen.LockReadStateParams{UserID: uuid(user), ThreadID: uuid(thread)})
	return oneReadState(row, err, "locking a read state")
}

// AdvanceReadState implements app.PersonalStore with one upsert; no returned row is no change.
func (r *PersonalRepository) AdvanceReadState(ctx context.Context, user ids.ID, rs sync.ReadState) (sync.ReadState, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return sync.ReadState{}, false, err
	}
	row, err := q.AdvanceReadState(ctx, sqlcgen.AdvanceReadStateParams{
		UserID: uuid(user), ThreadID: uuid(rs.ThreadID), ChannelID: uuid(rs.ChannelID), LastReadSeq: rs.LastReadSeq,
		UpdatedByDeviceID: uuid(rs.UpdatedByDeviceID), UpdatedAt: timestamptz(rs.UpdatedAt),
	})
	return oneReadState(row, err, "advancing a read state")
}

// SetReadState implements app.PersonalStore.
func (r *PersonalRepository) SetReadState(ctx context.Context, user, thread ids.ID, seq int64, device ids.ID, at time.Time) (sync.ReadState, error) {
	q, err := r.write(ctx)
	if err != nil {
		return sync.ReadState{}, err
	}
	row, err := q.SetReadState(ctx, sqlcgen.SetReadStateParams{
		UserID: uuid(user), ThreadID: uuid(thread), LastReadSeq: seq, UpdatedByDeviceID: uuid(device), UpdatedAt: timestamptz(at),
	})
	if err != nil {
		return sync.ReadState{}, failed(err, "moving a read state back")
	}
	return readStateOf(row), nil
}

// ReadStatesOfChannel implements app.PersonalStore.
func (r *PersonalRepository) ReadStatesOfChannel(ctx context.Context, user, channel ids.ID) ([]sync.ReadState, error) {
	rows, err := r.read(ctx).ListReadStatesOfChannel(ctx, sqlcgen.ListReadStatesOfChannelParams{UserID: uuid(user), ChannelID: uuid(channel)})
	if err != nil {
		return nil, failed(err, "listing read states")
	}
	out := make([]sync.ReadState, 0, len(rows))
	for _, row := range rows {
		out = append(out, readStateOf(row))
	}
	return out, nil
}

// ReadStatesInChannels implements app.SnapshotStore: the rows of user in channels, ascending by
// thread (M1-63).
func (r *PersonalRepository) ReadStatesInChannels(ctx context.Context, user ids.ID, channels []ids.ID) ([]sync.ReadState, error) {
	rows, err := r.read(ctx).ListReadStatesInChannels(ctx, sqlcgen.ListReadStatesInChannelsParams{UserID: uuid(user), ChannelIds: uuids(channels)})
	if err != nil {
		return nil, failed(err, "listing read states")
	}
	out := make([]sync.ReadState, 0, len(rows))
	for _, row := range rows {
		out = append(out, readStateOf(row))
	}
	return out, nil
}

// oneReadState converts the result of a single-row query; no row is found false.
func oneReadState(row sqlcgen.ReadState, err error, what string) (sync.ReadState, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return sync.ReadState{}, false, nil
	}
	if err != nil {
		return sync.ReadState{}, false, failed(err, what)
	}
	return readStateOf(row), true, nil
}

// readStateOf converts a row.
func readStateOf(row sqlcgen.ReadState) sync.ReadState {
	return sync.ReadState{
		ThreadID: idOf(row.ThreadID), ChannelID: idOf(row.ChannelID), LastReadSeq: row.LastReadSeq, Version: row.Version,
		UpdatedByDeviceID: idOf(row.UpdatedByDeviceID), UpdatedAt: row.UpdatedAt.Time,
	}
}

// UpsertDraft implements app.PersonalStore on the partial unique index of the thread or of the
// channel draft.
func (r *PersonalRepository) UpsertDraft(ctx context.Context, user ids.ID, d sync.Draft) (sync.Draft, error) {
	q, err := r.write(ctx)
	if err != nil {
		return sync.Draft{}, err
	}
	var row sqlcgen.Draft
	if d.ThreadID.IsZero() {
		row, err = q.UpsertChannelDraft(ctx, sqlcgen.UpsertChannelDraftParams{
			ID: uuid(d.ID), UserID: uuid(user), ChannelID: uuid(d.ChannelID), ContentMd: d.ContentMD, FileIds: uuids(d.FileIDs),
			UpdatedByDeviceID: uuid(d.UpdatedByDeviceID), UpdatedAt: timestamptz(d.UpdatedAt),
		})
	} else {
		row, err = q.UpsertThreadDraft(ctx, sqlcgen.UpsertThreadDraftParams{
			ID: uuid(d.ID), UserID: uuid(user), ThreadID: uuid(d.ThreadID), ChannelID: uuid(d.ChannelID), ContentMd: d.ContentMD,
			ReplyToMessageID: uuid(d.ReplyToID), FileIds: uuids(d.FileIDs), UpdatedByDeviceID: uuid(d.UpdatedByDeviceID),
			UpdatedAt: timestamptz(d.UpdatedAt),
		})
	}
	if err != nil {
		return sync.Draft{}, failed(err, "writing a draft")
	}
	return draftOf(row), nil
}

// DeleteDraft implements app.PersonalStore and app.DraftRows.
func (r *PersonalRepository) DeleteDraft(ctx context.Context, user, thread, channel ids.ID) (sync.Draft, bool, error) {
	if thread.IsZero() == channel.IsZero() {
		return sync.Draft{}, false, errs.New(errs.CodeInternalError, "sync: DeleteDraft needs exactly one of thread and channel")
	}
	q, err := r.write(ctx)
	if err != nil {
		return sync.Draft{}, false, err
	}
	var row sqlcgen.Draft
	if thread.IsZero() {
		row, err = q.DeleteChannelDraft(ctx, sqlcgen.DeleteChannelDraftParams{UserID: uuid(user), ChannelID: uuid(channel)})
	} else {
		row, err = q.DeleteThreadDraft(ctx, sqlcgen.DeleteThreadDraftParams{UserID: uuid(user), ThreadID: uuid(thread)})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return sync.Draft{}, false, nil
	}
	if err != nil {
		return sync.Draft{}, false, failed(err, "deleting a draft")
	}
	return draftOf(row), true, nil
}

// DraftsInChannels implements app.PersonalStore.
func (r *PersonalRepository) DraftsInChannels(ctx context.Context, user ids.ID, channels []ids.ID) ([]sync.Draft, error) {
	rows, err := r.read(ctx).ListDraftsInChannels(ctx, sqlcgen.ListDraftsInChannelsParams{UserID: uuid(user), ChannelIds: uuids(channels)})
	if err != nil {
		return nil, failed(err, "listing drafts")
	}
	out := make([]sync.Draft, 0, len(rows))
	for _, row := range rows {
		out = append(out, draftOf(row))
	}
	return out, nil
}

// draftOf converts a row; file ids are never nil.
func draftOf(row sqlcgen.Draft) sync.Draft {
	files := make([]ids.ID, 0, len(row.FileIds))
	for _, f := range row.FileIds {
		files = append(files, idOf(f))
	}
	return sync.Draft{
		ID: idOf(row.ID), ThreadID: idOf(row.ThreadID), ChannelID: idOf(row.ChannelID), ContentMD: row.ContentMd,
		ReplyToID: idOf(row.ReplyToMessageID), FileIDs: files, Version: row.Version, UpdatedByDeviceID: idOf(row.UpdatedByDeviceID),
		UpdatedAt: row.UpdatedAt.Time,
	}
}

// uuids converts ids for an array parameter; never nil, so the array is '{}' and not NULL.
func uuids(list []ids.ID) []pgtype.UUID {
	out := make([]pgtype.UUID, 0, len(list))
	for _, id := range list {
		out = append(out, uuid(id))
	}
	return out
}

// PutSetting implements app.PersonalStore with one upsert; no returned row is the same value.
func (r *PersonalRepository) PutSetting(ctx context.Context, user ids.ID, st sync.UserSetting) (sync.UserSetting, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return sync.UserSetting{}, false, err
	}
	row, err := q.PutSetting(ctx, sqlcgen.PutSettingParams{
		UserID: uuid(user), Key: st.Key, Value: st.Value, UpdatedByDeviceID: uuid(st.UpdatedByDeviceID), UpdatedAt: timestamptz(st.UpdatedAt),
	})
	return oneSetting(row, err, "writing a setting")
}

// Setting implements app.PersonalStore.
func (r *PersonalRepository) Setting(ctx context.Context, user ids.ID, key string) (sync.UserSetting, bool, error) {
	row, err := r.read(ctx).GetSetting(ctx, sqlcgen.GetSettingParams{UserID: uuid(user), Key: key})
	return oneSetting(row, err, "reading a setting")
}

// DeleteSetting implements app.PersonalStore.
func (r *PersonalRepository) DeleteSetting(ctx context.Context, user ids.ID, key string) (sync.UserSetting, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return sync.UserSetting{}, false, err
	}
	row, err := q.DeleteSetting(ctx, sqlcgen.DeleteSettingParams{UserID: uuid(user), Key: key})
	return oneSetting(row, err, "deleting a setting")
}

// Settings implements app.PersonalStore.
func (r *PersonalRepository) Settings(ctx context.Context, user ids.ID) ([]sync.UserSetting, error) {
	rows, err := r.read(ctx).ListSettings(ctx, uuid(user))
	if err != nil {
		return nil, failed(err, "listing settings")
	}
	out := make([]sync.UserSetting, 0, len(rows))
	for _, row := range rows {
		out = append(out, settingOf(row))
	}
	return out, nil
}

// SettingValues implements app.PersonalStore with one query.
func (r *PersonalRepository) SettingValues(ctx context.Context, users []ids.ID, keys []string) (map[ids.ID]map[string]json.RawMessage, error) {
	rows, err := r.read(ctx).SettingValues(ctx, sqlcgen.SettingValuesParams{UserIds: uuids(users), Keys: keys})
	if err != nil {
		return nil, failed(err, "reading setting values")
	}
	out := map[ids.ID]map[string]json.RawMessage{}
	for _, row := range rows {
		u := idOf(row.UserID)
		if out[u] == nil {
			out[u] = map[string]json.RawMessage{}
		}
		out[u][row.Key] = json.RawMessage(row.Value)
	}
	return out, nil
}

// oneSetting converts the result of a single-row query; no row is found false.
func oneSetting(row sqlcgen.UserSetting, err error, what string) (sync.UserSetting, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return sync.UserSetting{}, false, nil
	}
	if err != nil {
		return sync.UserSetting{}, false, failed(err, what)
	}
	return settingOf(row), true, nil
}

// settingOf converts a row; the value is the jsonb text as PostgreSQL stores it.
func settingOf(row sqlcgen.UserSetting) sync.UserSetting {
	return sync.UserSetting{
		Key: row.Key, Value: json.RawMessage(row.Value), Version: row.Version, UpdatedByDeviceID: idOf(row.UpdatedByDeviceID),
		UpdatedAt: row.UpdatedAt.Time,
	}
}
