// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// Threads implements the six thread operations of M1-55 (listChannelThreads, getThread,
// setThreadTitle, closeThread, reopenThread, setThreadTags) and moveThread (M1-62) as part of
// rest.StrictServerInterface. createThread follows with M1-56, because its request carries the
// first message. The use cases authorize; the handlers only convert and set ETag.
type Threads struct {
	threads chat.Threads
}

// NewThreads returns the thread handlers over the use cases.
func NewThreads(t chat.Threads) *Threads { return &Threads{threads: t} }

// Cursors is the codec of the list cursors (platform/httpserver), the outgoing port app.Cursors
// of listChannelThreads: decoding is an adapter concern (brief M1-55 D6).
type Cursors struct{}

// Compile-time check: the codec is the port of the use cases.
var _ app.Cursors = Cursors{}

// Encode implements app.Cursors with httpserver.EncodeCursor.
func (Cursors) Encode(list string, binding []byte, key any) (string, error) {
	return httpserver.EncodeCursor(list, binding, key)
}

// Decode implements app.Cursors with httpserver.DecodeCursor (400 cursor_invalid on a mismatch).
func (Cursors) Decode(list string, binding []byte, s string, key any) error {
	return httpserver.DecodeCursor(list, binding, s, key)
}

// ListChannelThreads implements rest.StrictServerInterface (GET /channels/{channel_id}/threads).
func (h *Threads) ListChannelThreads(ctx context.Context, req rest.ListChannelThreadsRequestObject) (rest.ListChannelThreadsResponseObject, error) {
	p := req.Params
	q := chat.ListThreadsQuery{ChannelID: ids.ID(req.ChannelId)}
	if p.Cursor != nil {
		q.Cursor = *p.Cursor
	}
	if p.Limit != nil {
		q.Limit = *p.Limit
	}
	if p.State != nil {
		q.State = string(*p.State)
	}
	if p.TagId != nil {
		q.TagID = ids.ID(*p.TagId)
	}
	if p.Sort != nil {
		q.Sort = string(*p.Sort)
	}
	page, err := h.threads.List(ctx, principal(ctx), q)
	if err != nil {
		return nil, err
	}
	out := rest.ThreadPage{Items: make([]rest.Thread, 0, len(page.Items))}
	for _, th := range page.Items {
		out.Items = append(out.Items, ThreadREST(th))
	}
	if page.NextCursor != "" {
		out.NextCursor = &page.NextCursor
	}
	return rest.ListChannelThreads200JSONResponse{Body: out}, nil
}

// GetThread implements rest.StrictServerInterface (GET /threads/{thread_id}).
func (h *Threads) GetThread(ctx context.Context, req rest.GetThreadRequestObject) (rest.GetThreadResponseObject, error) {
	th, err := h.threads.Get(ctx, principal(ctx), ids.ID(req.ThreadId))
	if err != nil {
		return nil, err
	}
	return rest.GetThread200JSONResponse{Body: ThreadREST(th), Headers: rest.GetThread200ResponseHeaders{ETag: etag(th.Version)}}, nil
}

// SetThreadTitle implements rest.StrictServerInterface (PUT /threads/{thread_id}/title).
func (h *Threads) SetThreadTitle(ctx context.Context, req rest.SetThreadTitleRequestObject) (rest.SetThreadTitleResponseObject, error) {
	th, err := h.threads.SetTitle(ctx, principal(ctx), chat.SetTitleCmd{
		ThreadID: ids.ID(req.ThreadId), Title: req.Body.Title, IfMatch: precondition(req.Params.IfMatch),
	})
	if err != nil {
		return nil, err
	}
	return rest.SetThreadTitle200JSONResponse{Body: ThreadREST(th), Headers: rest.SetThreadTitle200ResponseHeaders{ETag: etag(th.Version)}}, nil
}

// CloseThread implements rest.StrictServerInterface (POST /threads/{thread_id}/close).
func (h *Threads) CloseThread(ctx context.Context, req rest.CloseThreadRequestObject) (rest.CloseThreadResponseObject, error) {
	th, err := h.threads.Close(ctx, principal(ctx), ids.ID(req.ThreadId))
	if err != nil {
		return nil, err
	}
	return rest.CloseThread200JSONResponse{Body: ThreadREST(th)}, nil
}

// ReopenThread implements rest.StrictServerInterface (POST /threads/{thread_id}/reopen).
func (h *Threads) ReopenThread(ctx context.Context, req rest.ReopenThreadRequestObject) (rest.ReopenThreadResponseObject, error) {
	th, err := h.threads.Reopen(ctx, principal(ctx), ids.ID(req.ThreadId))
	if err != nil {
		return nil, err
	}
	return rest.ReopenThread200JSONResponse{Body: ThreadREST(th)}, nil
}

// SetThreadTags implements rest.StrictServerInterface (PUT /threads/{thread_id}/tags).
func (h *Threads) SetThreadTags(ctx context.Context, req rest.SetThreadTagsRequestObject) (rest.SetThreadTagsResponseObject, error) {
	tags := make([]ids.ID, 0, len(req.Body.TagIds))
	for _, t := range req.Body.TagIds {
		tags = append(tags, ids.ID(t))
	}
	var im string
	if req.Params.IfMatch != nil {
		im = *req.Params.IfMatch
	}
	th, err := h.threads.SetTags(ctx, principal(ctx), chat.SetTagsCmd{ThreadID: ids.ID(req.ThreadId), TagIDs: tags, IfMatch: precondition(im)})
	if err != nil {
		return nil, err
	}
	return rest.SetThreadTags200JSONResponse{Body: ThreadREST(th), Headers: rest.SetThreadTags200ResponseHeaders{ETag: etag(th.Version)}}, nil
}

// MoveThread implements rest.StrictServerInterface (POST /threads/{thread_id}/move, M1-62). The
// request validator of the chain guarantees the body; Idempotency-Key is handled by the
// idempotency stage of the chain (M1-20).
func (h *Threads) MoveThread(ctx context.Context, req rest.MoveThreadRequestObject) (rest.MoveThreadResponseObject, error) {
	cmd := chat.MoveThread{ThreadID: ids.ID(req.ThreadId)}
	if req.Body != nil {
		cmd.TargetChannelID, cmd.Reason = ids.ID(req.Body.TargetChannelId), req.Body.Reason
	}
	th, err := h.threads.Move(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.MoveThread200JSONResponse{Body: ThreadREST(th)}, nil
}

// ThreadREST converts a thread (schema Thread, brief D7): empty values become absent members,
// never null (ADR 0053); unread_floor_seq only when not 0; tag_ids always present.
func ThreadREST(th chat.Thread) rest.Thread { return rest.ThreadOf(rest.ThreadFields(th)) }
