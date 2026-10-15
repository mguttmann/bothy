// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
)

// AddReaction implements rest.StrictServerInterface (PUT /messages/{message_id}/reactions/{emoji},
// M1-59): 204 for a new and for an existing reaction alike. The chain has decoded the path value;
// the use case checks and normalises it.
func (h *Messages) AddReaction(ctx context.Context, req rest.AddReactionRequestObject) (rest.AddReactionResponseObject, error) {
	if err := h.reactions.Add(ctx, ids.ID(req.MessageId), req.Emoji); err != nil {
		return nil, err
	}
	return rest.AddReaction204Response{}, nil
}

// RemoveReaction implements rest.StrictServerInterface (DELETE
// /messages/{message_id}/reactions/{emoji}, M1-59): 204, also when there was nothing to remove.
func (h *Messages) RemoveReaction(ctx context.Context, req rest.RemoveReactionRequestObject) (rest.RemoveReactionResponseObject, error) {
	if err := h.reactions.Remove(ctx, ids.ID(req.MessageId), req.Emoji); err != nil {
		return nil, err
	}
	return rest.RemoveReaction204Response{}, nil
}

// ListReactions implements rest.StrictServerInterface (GET /messages/{message_id}/reactions,
// M1-59): one keyset page, next_cursor only when more reactions follow.
func (h *Messages) ListReactions(ctx context.Context, req rest.ListReactionsRequestObject) (rest.ListReactionsResponseObject, error) {
	q := messaging.ReactionListQuery{MessageID: ids.ID(req.MessageId), Emoji: req.Params.Emoji}
	if req.Params.Cursor != nil {
		q.Cursor = *req.Params.Cursor
	}
	if req.Params.Limit != nil {
		q.Limit = *req.Params.Limit
	}
	page, err := h.reactions.List(ctx, q)
	if err != nil {
		return nil, err
	}
	out := rest.ReactionPage{Items: make([]rest.Reaction, 0, len(page.Items))}
	for _, r := range page.Items {
		out.Items = append(out.Items, rest.Reaction{
			Emoji: r.Emoji, Principal: principalREST(r.Principal), CreatedAt: r.CreatedAt.UTC().Truncate(time.Millisecond),
		})
	}
	if page.NextCursor != "" {
		c := page.NextCursor
		out.NextCursor = &c
	}
	return rest.ListReactions200JSONResponse{Body: out}, nil
}

// PinMessage implements rest.StrictServerInterface (PUT /messages/{message_id}/pin, M1-59): 204
// for a new and for an existing pin alike.
func (h *Messages) PinMessage(ctx context.Context, req rest.PinMessageRequestObject) (rest.PinMessageResponseObject, error) {
	if err := h.pins.Pin(ctx, ids.ID(req.MessageId)); err != nil {
		return nil, err
	}
	return rest.PinMessage204Response{}, nil
}

// UnpinMessage implements rest.StrictServerInterface (DELETE /messages/{message_id}/pin, M1-59):
// 204, also when the message was not pinned.
func (h *Messages) UnpinMessage(ctx context.Context, req rest.UnpinMessageRequestObject) (rest.UnpinMessageResponseObject, error) {
	if err := h.pins.Unpin(ctx, ids.ID(req.MessageId)); err != nil {
		return nil, err
	}
	return rest.UnpinMessage204Response{}, nil
}

// ListPins implements rest.StrictServerInterface (GET /threads/{thread_id}/pins, M1-59): every pin
// of the thread the caller may read, newest first, unpaginated.
func (h *Messages) ListPins(ctx context.Context, req rest.ListPinsRequestObject) (rest.ListPinsResponseObject, error) {
	list, err := h.pins.List(ctx, ids.ID(req.ThreadId))
	if err != nil {
		return nil, err
	}
	out := rest.PinList{Items: make([]rest.Pin, 0, len(list))}
	for _, p := range list {
		out.Items = append(out.Items, rest.Pin{
			MessageId: rest.Uuid(p.MessageID), ThreadId: rest.Uuid(p.ThreadID), PinnedBy: rest.Uuid(p.PinnedBy),
			PinnedAt: p.PinnedAt.UTC().Truncate(time.Millisecond),
		})
	}
	return rest.ListPins200JSONResponse{Body: out}, nil
}

// principalREST converts a principal (schema PrincipalRef).
func principalREST(p messaging.PrincipalRef) rest.PrincipalRef {
	return rest.PrincipalRef{
		Id: rest.Uuid(p.ID), Kind: rest.PrincipalKind(p.Kind), Handle: p.Handle, DisplayName: p.DisplayName, AvatarFileId: optionalID(p.AvatarFileID),
	}
}

// reactionsREST converts the summary of a message (schema ReactionSummary); never nil.
func reactionsREST(list []messaging.ReactionCount) []rest.ReactionSummary {
	out := make([]rest.ReactionSummary, 0, len(list))
	for _, r := range list {
		out = append(out, rest.ReactionSummary{Emoji: r.Emoji, Count: r.Count})
	}
	return out
}
