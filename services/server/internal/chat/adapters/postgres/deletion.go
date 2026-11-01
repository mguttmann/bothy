// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Compile-time check: the deletion port of the use cases.
var _ app.DeletionStore = (*Repository)(nil)

// SaveSpaceDeletion implements app.DeletionStore.
func (r *Repository) SaveSpaceDeletion(ctx context.Context, sp domain.Space) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.SaveSpaceDeletion(ctx, sqlcgen.SaveSpaceDeletionParams{
		ID: uuid(sp.ID), ArchivedAt: optionalTime(sp.ArchivedAt), DeletedAt: optionalTime(sp.Deletion.At),
		DeletedBy: optionalUUID(sp.Deletion.By), Version: sp.Version,
	})
	return mapError(err, "writing the deletion of a space")
}

// SaveChannelDeletion implements app.DeletionStore.
func (r *Repository) SaveChannelDeletion(ctx context.Context, c domain.Channel) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.SaveChannelDeletion(ctx, sqlcgen.SaveChannelDeletionParams{
		ID: uuid(c.ID), ArchivedAt: optionalTime(c.ArchivedAt), DeletedAt: optionalTime(c.Deletion.At),
		DeletedBy: optionalUUID(c.Deletion.By), Version: c.Version,
	})
	return mapError(err, "writing the deletion of a channel")
}

// SaveThreadDeletion implements app.DeletionStore.
func (r *Repository) SaveThreadDeletion(ctx context.Context, th domain.Thread) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.SaveThreadDeletion(ctx, sqlcgen.SaveThreadDeletionParams{
		ID: uuid(th.ID), DeletedAt: optionalTime(th.Deletion.At), DeletedBy: optionalUUID(th.Deletion.By), Version: th.Version,
	})
	return mapError(err, "writing the deletion of a thread")
}

// DeletionScope implements app.DeletionStore.
func (r *Repository) DeletionScope(ctx context.Context, kind string, id ids.ID) (app.Scope, error) {
	q := r.read(ctx)
	var channels []ids.ID
	switch kind {
	case chat.TargetSpace:
		rows, err := q.ScopeChannelsOfSpace(ctx, uuid(id))
		if err != nil {
			return app.Scope{}, mapError(err, "reading the channels of a deletion")
		}
		channels = idsOf(rows)
	case chat.TargetChannel:
		channels = []ids.ID{id}
	default:
		th, found, err := r.Thread(ctx, id)
		if err != nil || !found {
			return app.Scope{}, err
		}
		return app.Scope{Threads: 1, Messages: int64(th.MessageCount), ThreadIDs: []ids.ID{id}}, nil
	}
	out := app.Scope{Channels: int64(len(channels))}
	if len(channels) == 0 {
		return out, nil
	}
	rows, err := q.ScopeThreads(ctx, uuids(channels))
	if err != nil {
		return app.Scope{}, mapError(err, "reading the threads of a deletion")
	}
	for _, row := range rows {
		if row.Kind == string(domain.ThreadKindThread) {
			out.Threads++
		}
		out.Messages += int64(row.MessageCount)
		out.ThreadIDs = append(out.ThreadIDs, idOf(row.ID))
	}
	return out, nil
}

// BindingTrash implements app.DeletionStore.
func (r *Repository) BindingTrash(ctx context.Context, channel ids.ID) (app.BindingTrash, bool, error) {
	row, err := r.read(ctx).BindingTrash(ctx, uuid(channel))
	return oneBindingTrash(row, err)
}

// LockBindingTrash implements app.DeletionStore.
func (r *Repository) LockBindingTrash(ctx context.Context, channel ids.ID) (app.BindingTrash, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.BindingTrash{}, false, err
	}
	row, err := q.LockBindingTrash(ctx, uuid(channel))
	return oneBindingTrash(sqlcgen.BindingTrashRow(row), err)
}

// oneBindingTrash converts the folder part of a binding; no row is found false.
func oneBindingTrash(row sqlcgen.BindingTrashRow, err error) (app.BindingTrash, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.BindingTrash{}, false, nil
	}
	if err != nil {
		return app.BindingTrash{}, false, mapError(err, "reading the folder of a project binding")
	}
	out := app.BindingTrash{
		WorkspaceRef: row.WorkspaceRef, State: row.TrashState.String, Reason: row.TrashReason.String, AgentID: idOf(row.TrashAgentID),
	}
	if row.TrashCheckOk.Valid {
		out.Check = &chat.ProjectTrashCheck{OK: row.TrashCheckOk.Bool, Reason: row.TrashCheckReason.String, CheckedAt: timeOf(row.TrashCheckedAt)}
	}
	return out, true, nil
}

// RequestTrash implements app.DeletionStore.
func (r *Repository) RequestTrash(ctx context.Context, channel, agent ids.ID, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.RequestTrash(ctx, sqlcgen.RequestTrashParams{ChannelID: uuid(channel), AgentID: uuid(agent), RequestedAt: optionalTime(at)})
	return mapError(err, "requesting a folder move")
}

// DropTrashRequest implements app.DeletionStore.
func (r *Repository) DropTrashRequest(ctx context.Context, channel ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.DropTrashRequest(ctx, uuid(channel)), "dropping a folder move")
}

// SetTrashCheck implements app.DeletionStore.
func (r *Repository) SetTrashCheck(ctx context.Context, channel ids.ID, c chat.ProjectTrashCheckReport, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	reason := ""
	if !c.OK {
		reason = c.Reason
	}
	err = q.SetTrashCheck(ctx, sqlcgen.SetTrashCheckParams{
		ChannelID: uuid(channel), Ok: pgtype.Bool{Bool: c.OK, Valid: true}, Reason: optionalText(reason), CheckedAt: optionalTime(at),
	})
	return mapError(err, "writing a folder check")
}

// DuePurges implements app.DeletionStore.
func (r *Repository) DuePurges(ctx context.Context, before time.Time, limit int) ([]app.PurgeItem, error) {
	rows, err := r.read(ctx).DuePurges(ctx, sqlcgen.DuePurgesParams{Before: optionalTime(before), MaxRows: pg.Limit(limit)})
	if err != nil {
		return nil, mapError(err, "reading the due purges")
	}
	out := make([]app.PurgeItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.PurgeItem{Kind: row.Kind, ID: idOf(row.ID)})
	}
	return out, nil
}

// FinishPurge implements app.DeletionStore: the purged_at of the item first, which locks its row
// and tells a purged item apart, then the titles and topics below it.
func (r *Repository) FinishPurge(ctx context.Context, kind string, id ids.ID, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	switch kind {
	case chat.TargetThread:
		_, err := q.PurgeThread(ctx, sqlcgen.PurgeThreadParams{ID: uuid(id), PurgedAt: optionalTime(at)})
		return mapError(err, "purging a thread")
	case chat.TargetChannel:
		n, err := q.PurgeChannel(ctx, sqlcgen.PurgeChannelParams{ID: uuid(id), PurgedAt: optionalTime(at)})
		if err != nil || n == 0 {
			return mapError(err, "purging a channel")
		}
		return mapError(q.ClearThreadTitlesOfChannels(ctx, uuids([]ids.ID{id})), "clearing thread titles")
	case chat.TargetSpace:
		n, err := q.PurgeSpace(ctx, sqlcgen.PurgeSpaceParams{ID: uuid(id), PurgedAt: optionalTime(at)})
		if err != nil || n == 0 {
			return mapError(err, "purging a space")
		}
		channels, err := r.ChannelIDsOfSpace(ctx, id)
		if err != nil {
			return err
		}
		if err := q.ClearThreadTitlesOfChannels(ctx, uuids(channels)); err != nil {
			return mapError(err, "clearing thread titles")
		}
		return mapError(q.ClearTopicsOfSpace(ctx, uuid(id)), "clearing channel topics")
	}
	return errs.New(errs.CodeInternalError, "chat: an unknown purge target")
}

// PendingTrash implements app.DeletionStore.
func (r *Repository) PendingTrash(ctx context.Context, agent ids.ID) ([]chat.ProjectTrashRequest, error) {
	rows, err := r.read(ctx).PendingTrash(ctx, uuid(agent))
	if err != nil {
		return nil, mapError(err, "reading the folder moves of a runner")
	}
	out := make([]chat.ProjectTrashRequest, 0, len(rows))
	for _, row := range rows {
		out = append(out, chat.ProjectTrashRequest{ChannelID: idOf(row.ChannelID), WorkspaceRef: row.WorkspaceRef})
	}
	return out, nil
}

// LockTrashRequests implements app.DeletionStore.
func (r *Repository) LockTrashRequests(ctx context.Context, agent ids.ID, channels []ids.ID) ([]ids.ID, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockTrashRequests(ctx, sqlcgen.LockTrashRequestsParams{ChannelIds: uuids(channels), AgentID: uuid(agent)})
	if err != nil {
		return nil, mapError(err, "locking folder moves")
	}
	return idsOf(rows), nil
}

// SetTrashResult implements app.DeletionStore.
func (r *Repository) SetTrashResult(ctx context.Context, channel ids.ID, result, reason string) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.SetTrashResult(ctx, sqlcgen.SetTrashResultParams{
		ChannelID: uuid(channel), Result: pgtype.Text{String: result, Valid: true}, Reason: optionalText(reason),
	})
	return mapError(err, "writing a folder move outcome")
}
