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

// ListMyMentions implements rest.StrictServerInterface (GET /me/mentions, M1-60).
func (h *Messages) ListMyMentions(ctx context.Context, req rest.ListMyMentionsRequestObject) (rest.ListMyMentionsResponseObject, error) {
	var q messaging.MentionListQuery
	if req.Params.Cursor != nil {
		q.Cursor = *req.Params.Cursor
	}
	if req.Params.Limit != nil {
		q.Limit = *req.Params.Limit
	}
	page, err := h.mentions.ListMine(ctx, q)
	if err != nil {
		return nil, err
	}
	out := rest.MentionItemPage{Items: make([]rest.MentionItem, 0, len(page.Items))}
	for _, it := range page.Items {
		out.Items = append(out.Items, rest.MentionItem{Message: MessageREST(it.Message), CreatedAt: it.CreatedAt.UTC().Truncate(time.Millisecond)})
	}
	if page.NextCursor != "" {
		out.NextCursor = &page.NextCursor
	}
	return rest.ListMyMentions200JSONResponse{Body: out}, nil
}

// ListMyTemplates implements rest.StrictServerInterface (GET /me/templates).
func (h *Messages) ListMyTemplates(ctx context.Context, _ rest.ListMyTemplatesRequestObject) (rest.ListMyTemplatesResponseObject, error) {
	list, err := h.templates.ListMine(ctx)
	if err != nil {
		return nil, err
	}
	return rest.ListMyTemplates200JSONResponse{Body: templateList(list)}, nil
}

// ListTemplates implements rest.StrictServerInterface (GET /channels/{channel_id}/templates).
func (h *Messages) ListTemplates(ctx context.Context, req rest.ListTemplatesRequestObject) (rest.ListTemplatesResponseObject, error) {
	list, err := h.templates.List(ctx, ids.ID(req.ChannelId))
	if err != nil {
		return nil, err
	}
	return rest.ListTemplates200JSONResponse{Body: templateList(list)}, nil
}

// CreatePersonalTemplate implements rest.StrictServerInterface (POST /me/templates); the chain
// answers a repeated Idempotency-Key with the stored response.
func (h *Messages) CreatePersonalTemplate(ctx context.Context, req rest.CreatePersonalTemplateRequestObject) (rest.CreatePersonalTemplateResponseObject, error) {
	v, err := h.templates.CreatePersonal(ctx, messaging.TemplateInput{Name: req.Body.Name, ContentMD: req.Body.ContentMd})
	if err != nil {
		return nil, err
	}
	return rest.CreatePersonalTemplate201JSONResponse{Body: TemplateREST(v)}, nil
}

// CreateChannelTemplate implements rest.StrictServerInterface (POST
// /channels/{channel_id}/templates).
func (h *Messages) CreateChannelTemplate(ctx context.Context, req rest.CreateChannelTemplateRequestObject) (rest.CreateChannelTemplateResponseObject, error) {
	v, err := h.templates.CreateForChannel(ctx, ids.ID(req.ChannelId), messaging.TemplateInput{Name: req.Body.Name, ContentMD: req.Body.ContentMd})
	if err != nil {
		return nil, err
	}
	return rest.CreateChannelTemplate201JSONResponse{Body: TemplateREST(v)}, nil
}

// UpdateTemplate implements rest.StrictServerInterface (PATCH /templates/{template_id}): 200
// with the ETag of the version. The chain answers a missing If-Match with 428 before the handler.
func (h *Messages) UpdateTemplate(ctx context.Context, req rest.UpdateTemplateRequestObject) (rest.UpdateTemplateResponseObject, error) {
	c := messaging.TemplateChange{TemplateID: ids.ID(req.TemplateId), IfMatch: precondition(req.Params.IfMatch)}
	if b := req.Body; b != nil {
		c.Name, c.ContentMD = b.Name, b.ContentMd
	}
	v, err := h.templates.Update(ctx, c)
	if err != nil {
		return nil, err
	}
	tag := httpserver.FormatETag(v.Version)
	return rest.UpdateTemplate200JSONResponse{Body: TemplateREST(v), Headers: rest.UpdateTemplate200ResponseHeaders{ETag: &tag}}, nil
}

// DeleteTemplate implements rest.StrictServerInterface (DELETE /templates/{template_id}): 204,
// also for an already deleted template.
func (h *Messages) DeleteTemplate(ctx context.Context, req rest.DeleteTemplateRequestObject) (rest.DeleteTemplateResponseObject, error) {
	if err := h.templates.Delete(ctx, ids.ID(req.TemplateId)); err != nil {
		return nil, err
	}
	return rest.DeleteTemplate204Response{}, nil
}

// precondition parses the If-Match header value of a request.
func precondition(v string) messaging.Precondition {
	hd := http.Header{}
	if v != "" {
		hd.Set("If-Match", v)
	}
	im := httpserver.ParseIfMatch(hd)
	return messaging.Precondition{Present: im.Present, Valid: im.Valid, Version: im.Version}
}

// templateList converts templates (schema TemplateList); items is never nil.
func templateList(list []messaging.TemplateView) rest.TemplateList {
	out := rest.TemplateList{Items: make([]rest.Template, 0, len(list))}
	for _, v := range list {
		out.Items = append(out.Items, TemplateREST(v))
	}
	return out
}

// TemplateREST converts a template (schema Template): channel_id only for the channel scope.
func TemplateREST(v messaging.TemplateView) rest.Template {
	return rest.Template{
		Id: rest.Uuid(v.ID), Scope: rest.TemplateScope(v.Scope), ChannelId: optionalID(v.ChannelID), Name: v.Name,
		ContentMd: v.ContentMD, Version: v.Version,
	}
}
