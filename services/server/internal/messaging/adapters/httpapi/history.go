// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// modes maps the windows of the domain onto the port.
var modes = map[domain.HistoryMode]messaging.HistoryMode{
	domain.HistoryNewest: messaging.HistoryNewest,
	domain.HistoryBefore: messaging.HistoryBefore,
	domain.HistoryAfter:  messaging.HistoryAfter,
	domain.HistoryAround: messaging.HistoryAround,
}

// ListThreadMessages implements rest.StrictServerInterface (GET /threads/{thread_id}/messages,
// M1-57): the anchors are checked before any lookup (brief note 2 step 1), then
// messaging.Messages.History reads the page; the items are ascending with tombstones, the flags
// always present.
func (h *Messages) ListThreadMessages(ctx context.Context, req rest.ListThreadMessagesRequestObject) (rest.ListThreadMessagesResponseObject, error) {
	p := req.Params
	mode, anchor, err := domain.ValidateHistoryRequest(p.BeforeSeq, p.AfterSeq, p.AroundSeq)
	if err != nil {
		return nil, err
	}
	q := messaging.HistoryQuery{ThreadID: ids.ID(req.ThreadId), Mode: modes[mode], Anchor: anchor}
	if p.Limit != nil {
		q.Limit = *p.Limit
	}
	page, err := h.messages.History(ctx, q)
	if err != nil {
		return nil, err
	}
	out := rest.MessagePage{Items: make([]rest.Message, 0, len(page.Items)), HasMoreBefore: page.HasMoreBefore, HasMoreAfter: page.HasMoreAfter}
	for _, m := range page.Items {
		out.Items = append(out.Items, MessageREST(m))
	}
	return rest.ListThreadMessages200JSONResponse{Body: out}, nil
}

// GetMessage implements rest.StrictServerInterface (GET /messages/{message_id}, M1-57): the whole
// message, a tombstone included, with ETag "<version>" (06 section 8 item 1).
func (h *Messages) GetMessage(ctx context.Context, req rest.GetMessageRequestObject) (rest.GetMessageResponseObject, error) {
	m, err := h.messages.Get(ctx, ids.ID(req.MessageId))
	if err != nil {
		return nil, err
	}
	etag := httpserver.FormatETag(m.Version)
	return rest.GetMessage200JSONResponse{Body: MessageREST(m), Headers: rest.GetMessage200ResponseHeaders{ETag: &etag}}, nil
}
