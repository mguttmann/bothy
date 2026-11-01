// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Deletion implements deleteSpace, restoreSpace, getSpaceDeletion, deleteChannel, restoreChannel,
// getChannelDeletion, deleteThread, restoreThread and getThreadDeletion
// (FEAT-delete-spaces-chats) as part of rest.StrictServerInterface. The use cases authorize; the
// handlers only convert.
type Deletion struct {
	d chat.Deletions
}

// NewDeletion returns the handlers over the deletion use cases of chat.
func NewDeletion(d chat.Deletions) *Deletion { return &Deletion{d: d} }

// DeleteSpace implements rest.StrictServerInterface (POST /spaces/{space_id}/delete).
func (h *Deletion) DeleteSpace(ctx context.Context, req rest.DeleteSpaceRequestObject) (rest.DeleteSpaceResponseObject, error) {
	info, err := h.d.DeleteSpace(ctx, principal(ctx), ids.ID(req.SpaceId))
	if err != nil {
		return nil, err
	}
	return rest.DeleteSpace200JSONResponse{Body: deletionREST(info)}, nil
}

// RestoreSpace implements rest.StrictServerInterface (POST /spaces/{space_id}/restore).
func (h *Deletion) RestoreSpace(ctx context.Context, req rest.RestoreSpaceRequestObject) (rest.RestoreSpaceResponseObject, error) {
	sp, err := h.d.RestoreSpace(ctx, principal(ctx), ids.ID(req.SpaceId))
	if err != nil {
		return nil, err
	}
	return rest.RestoreSpace200JSONResponse{Body: spaceREST(sp)}, nil
}

// GetSpaceDeletion implements rest.StrictServerInterface (GET /spaces/{space_id}/deletion).
func (h *Deletion) GetSpaceDeletion(ctx context.Context, req rest.GetSpaceDeletionRequestObject) (rest.GetSpaceDeletionResponseObject, error) {
	info, err := h.d.SpaceDeletion(ctx, principal(ctx), ids.ID(req.SpaceId))
	if err != nil {
		return nil, err
	}
	return rest.GetSpaceDeletion200JSONResponse{Body: deletionREST(info)}, nil
}

// DeleteChannel implements rest.StrictServerInterface (POST /channels/{channel_id}/delete); a
// missing body is trash_project_folder false.
func (h *Deletion) DeleteChannel(ctx context.Context, req rest.DeleteChannelRequestObject) (rest.DeleteChannelResponseObject, error) {
	trash := req.Body != nil && req.Body.TrashProjectFolder != nil && *req.Body.TrashProjectFolder
	info, err := h.d.DeleteChannel(ctx, principal(ctx), ids.ID(req.ChannelId), trash)
	if err != nil {
		return nil, err
	}
	return rest.DeleteChannel200JSONResponse{Body: deletionREST(info)}, nil
}

// RestoreChannel implements rest.StrictServerInterface (POST /channels/{channel_id}/restore).
func (h *Deletion) RestoreChannel(ctx context.Context, req rest.RestoreChannelRequestObject) (rest.RestoreChannelResponseObject, error) {
	c, err := h.d.RestoreChannel(ctx, principal(ctx), ids.ID(req.ChannelId))
	if err != nil {
		return nil, err
	}
	return rest.RestoreChannel200JSONResponse{Body: channelREST(c)}, nil
}

// GetChannelDeletion implements rest.StrictServerInterface (GET /channels/{channel_id}/deletion).
func (h *Deletion) GetChannelDeletion(ctx context.Context, req rest.GetChannelDeletionRequestObject) (rest.GetChannelDeletionResponseObject, error) {
	info, err := h.d.ChannelDeletion(ctx, principal(ctx), ids.ID(req.ChannelId))
	if err != nil {
		return nil, err
	}
	return rest.GetChannelDeletion200JSONResponse{Body: deletionREST(info)}, nil
}

// DeleteThread implements rest.StrictServerInterface (POST /threads/{thread_id}/delete).
func (h *Deletion) DeleteThread(ctx context.Context, req rest.DeleteThreadRequestObject) (rest.DeleteThreadResponseObject, error) {
	info, err := h.d.DeleteThread(ctx, principal(ctx), ids.ID(req.ThreadId))
	if err != nil {
		return nil, err
	}
	return rest.DeleteThread200JSONResponse{Body: deletionREST(info)}, nil
}

// RestoreThread implements rest.StrictServerInterface (POST /threads/{thread_id}/restore).
func (h *Deletion) RestoreThread(ctx context.Context, req rest.RestoreThreadRequestObject) (rest.RestoreThreadResponseObject, error) {
	th, err := h.d.RestoreThread(ctx, principal(ctx), ids.ID(req.ThreadId))
	if err != nil {
		return nil, err
	}
	return rest.RestoreThread200JSONResponse{Body: ThreadREST(th)}, nil
}

// GetThreadDeletion implements rest.StrictServerInterface (GET /threads/{thread_id}/deletion).
func (h *Deletion) GetThreadDeletion(ctx context.Context, req rest.GetThreadDeletionRequestObject) (rest.GetThreadDeletionResponseObject, error) {
	info, err := h.d.ThreadDeletion(ctx, principal(ctx), ids.ID(req.ThreadId))
	if err != nil {
		return nil, err
	}
	return rest.GetThreadDeletion200JSONResponse{Body: deletionREST(info)}, nil
}

// deletionREST converts a deletion (schema DeletionInfo); absent members are never null.
func deletionREST(d chat.DeletionInfo) rest.DeletionInfo {
	c := d.Contents
	out := rest.DeletionInfo{
		TargetKind: rest.DeletionInfoTargetKind(d.TargetKind), TargetId: rest.Uuid(d.TargetID), State: rest.DeletionInfoState(d.State),
		DeletedAt: optionalTime(d.DeletedAt), UndoUntil: optionalTime(d.UndoUntil),
		Contents: rest.DeletionContents{Channels: c.Channels, Threads: c.Threads, Messages: c.Messages, Files: c.Files, Sessions: c.Sessions},
	}
	if f := d.ProjectFolder; f != nil {
		pf := rest.ProjectFolderDeletion{WorkspaceRef: f.WorkspaceRef, TrashReason: optional(f.TrashReason)}
		if f.TrashState != "" {
			st := rest.ProjectFolderDeletionTrashState(f.TrashState)
			pf.TrashState = &st
		}
		if f.TrashCheck != nil {
			pf.TrashCheck = &rest.ProjectTrashCheck{Ok: f.TrashCheck.OK, Reason: optional(f.TrashCheck.Reason), CheckedAt: optionalTime(f.TrashCheck.CheckedAt)}
		}
		out.ProjectFolder = &pf
	}
	return out
}
