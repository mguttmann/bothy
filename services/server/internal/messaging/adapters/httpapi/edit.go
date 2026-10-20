// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// ifMatch returns the version of an If-Match value; a value that is not a version gives 0, which
// matches no message, so the use case answers 412 (the chain refused a missing one with 428).
func ifMatch(v string) int64 {
	h := http.Header{}
	h.Set("If-Match", v)
	im := httpserver.ParseIfMatch(h)
	if !im.Valid {
		return 0
	}
	return im.Version
}

// UpdateMessage implements rest.StrictServerInterface (PATCH /messages/{message_id}, M1-58): 200
// with the message and its ETag; the Idempotency-Key stage of the chain answers a repetition.
func (h *Messages) UpdateMessage(ctx context.Context, req rest.UpdateMessageRequestObject) (rest.UpdateMessageResponseObject, error) {
	c := messaging.UpdateCommand{MessageID: ids.ID(req.MessageId), IfMatch: ifMatch(req.Params.IfMatch)}
	if b := req.Body; b != nil {
		c.Content = b.ContentMd
		if b.MentionIds != nil {
			list := idList(*b.MentionIds)
			c.MentionIDs = &list
		}
	}
	m, err := h.messages.Update(ctx, c)
	if err != nil {
		return nil, err
	}
	tag := httpserver.FormatETag(m.Version)
	return rest.UpdateMessage200JSONResponse{Body: MessageREST(m), Headers: rest.UpdateMessage200ResponseHeaders{ETag: &tag}}, nil
}

// DeleteMessage implements rest.StrictServerInterface (DELETE /messages/{message_id}, M1-58): 204,
// also for a message that was already deleted.
func (h *Messages) DeleteMessage(ctx context.Context, req rest.DeleteMessageRequestObject) (rest.DeleteMessageResponseObject, error) {
	if err := h.messages.Delete(ctx, ids.ID(req.MessageId)); err != nil {
		return nil, err
	}
	return rest.DeleteMessage204Response{}, nil
}

// ListMessageRevisions implements rest.StrictServerInterface (GET
// /messages/{message_id}/revisions, M1-58): the earlier versions in ascending order, unpaginated.
func (h *Messages) ListMessageRevisions(ctx context.Context, req rest.ListMessageRevisionsRequestObject) (rest.ListMessageRevisionsResponseObject, error) {
	list, err := h.messages.Revisions(ctx, ids.ID(req.MessageId))
	if err != nil {
		return nil, err
	}
	out := rest.MessageRevisionList{Items: make([]rest.MessageRevision, 0, len(list))}
	for _, r := range list {
		out.Items = append(out.Items, rest.MessageRevision{
			Revision: r.Revision, ContentMd: r.ContentMD, EditedBy: rest.Uuid(r.EditedBy), CreatedAt: r.CreatedAt.UTC().Truncate(time.Millisecond),
		})
	}
	return rest.ListMessageRevisions200JSONResponse{Body: out}, nil
}
