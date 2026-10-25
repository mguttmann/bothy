// SPDX-License-Identifier: Apache-2.0

// Package httpapi holds the REST handlers of the module privacy as part of
// rest.StrictServerInterface (M7-06): listGdprRequests, createGdprRequest, getGdprRequest,
// decideGdprRequest and downloadGdprExport of docs/api/openapi.yaml. The use cases authorize; the
// handlers only convert and pass errors to the problem writer of the chain unchanged. The
// Idempotency-Key of the writes is handled by the middleware of the chain (M1-20).
//
// Example:
//
//	h := httpapi.New(svc)
//	built, err := routes.Build(wire.Handlers{Privacy: h}, wire.RouteOptions(), cat)
package httpapi

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/privacy"
	"github.com/OWNER/bothy/services/server/internal/privacy/app"
)

// Privacy implements the five REST operations of the privacy requests over privacy.Requests.
type Privacy struct {
	requests privacy.Requests
}

// New returns the handlers over the port.
func New(r privacy.Requests) *Privacy { return &Privacy{requests: r} }

// principal returns the actor of the request, the anonymous actor outside one.
func principal(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// ListGdprRequests implements rest.StrictServerInterface (GET /gdpr-requests).
func (h *Privacy) ListGdprRequests(ctx context.Context, req rest.ListGdprRequestsRequestObject) (rest.ListGdprRequestsResponseObject, error) {
	q := privacy.ListQuery{}
	p := req.Params
	if p.SubjectUserId != nil {
		q.SubjectUserID = ids.ID(*p.SubjectUserId)
	}
	if p.State != nil {
		q.State = string(*p.State)
	}
	if p.Cursor != nil {
		q.Cursor = *p.Cursor
	}
	if p.Limit != nil {
		q.Limit = *p.Limit
	}
	page, err := h.requests.List(ctx, principal(ctx), q)
	if err != nil {
		return nil, err
	}
	out := rest.GdprRequestPage{Items: make([]rest.GdprRequest, 0, len(page.Items))}
	for _, r := range page.Items {
		out.Items = append(out.Items, RequestREST(r))
	}
	if page.NextCursor != "" {
		c := page.NextCursor
		out.NextCursor = &c
	}
	return rest.ListGdprRequests200JSONResponse{Body: out}, nil
}

// CreateGdprRequest implements rest.StrictServerInterface (POST /gdpr-requests): 201.
func (h *Privacy) CreateGdprRequest(ctx context.Context, req rest.CreateGdprRequestRequestObject) (rest.CreateGdprRequestResponseObject, error) {
	if req.Body == nil {
		return nil, missingBody()
	}
	cmd := privacy.CreateCmd{Kind: string(req.Body.Kind), Reason: req.Body.Reason}
	if req.Body.SubjectUserId != nil {
		cmd.SubjectUserID = ids.ID(*req.Body.SubjectUserId)
	}
	r, err := h.requests.Create(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.CreateGdprRequest201JSONResponse{Body: RequestREST(r)}, nil
}

// GetGdprRequest implements rest.StrictServerInterface (GET /gdpr-requests/{request_id}).
func (h *Privacy) GetGdprRequest(ctx context.Context, req rest.GetGdprRequestRequestObject) (rest.GetGdprRequestResponseObject, error) {
	r, err := h.requests.Get(ctx, principal(ctx), ids.ID(req.RequestId))
	if err != nil {
		return nil, err
	}
	return rest.GetGdprRequest200JSONResponse{Body: RequestREST(r)}, nil
}

// DecideGdprRequest implements rest.StrictServerInterface (POST
// /gdpr-requests/{request_id}/decide).
func (h *Privacy) DecideGdprRequest(ctx context.Context, req rest.DecideGdprRequestRequestObject) (rest.DecideGdprRequestResponseObject, error) {
	if req.Body == nil {
		return nil, missingBody()
	}
	cmd := privacy.DecideCmd{RequestID: ids.ID(req.RequestId), Decision: string(req.Body.Decision), Reason: req.Body.Reason}
	if req.Body.ExtendMonths != nil {
		cmd.ExtendMonths = int(*req.Body.ExtendMonths)
	}
	r, err := h.requests.Decide(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.DecideGdprRequest200JSONResponse{Body: RequestREST(r)}, nil
}

// DownloadGdprExport implements rest.StrictServerInterface (GET
// /gdpr-requests/{request_id}/export): the ZIP archive as application/zip. The generated
// response closes nothing, so the body is closed once it was read to the end or failed.
func (h *Privacy) DownloadGdprExport(ctx context.Context, req rest.DownloadGdprExportRequestObject) (rest.DownloadGdprExportResponseObject, error) {
	pkg, err := h.requests.Download(ctx, principal(ctx), ids.ID(req.RequestId))
	if err != nil {
		return nil, err
	}
	body := &closingReader{rc: pkg.Body}
	// A client that leaves before the end never makes Read fail: the end of the request closes it.
	context.AfterFunc(ctx, body.close)
	return rest.DownloadGdprExport200ApplicationzipResponse{Body: body, ContentLength: pkg.Size}, nil
}

// closingReader closes the package when reading ends or the request ends, because the generated
// response never closes its body.
type closingReader struct {
	rc   io.ReadCloser
	once sync.Once
}

// Read implements io.Reader and closes the body at the end or on an error.
func (c *closingReader) Read(p []byte) (int, error) {
	n, err := c.rc.Read(p)
	if err != nil {
		c.close()
	}
	return n, err
}

// close closes the body once.
func (c *closingReader) close() { c.once.Do(func() { _ = c.rc.Close() }) }

// RequestREST converts a request into the schema GdprRequest.
func RequestREST(r privacy.Request) rest.GdprRequest {
	out := rest.GdprRequest{
		Id: rest.Uuid(r.ID), SubjectUserId: rest.Uuid(r.SubjectUserID), Kind: rest.GdprRequestKind(r.Kind),
		State: rest.GdprRequestState(r.State), RequestedBy: rest.Uuid(r.RequestedBy), Reason: r.Reason, DueAt: stamp(r.DueAt),
		CreatedAt: stamp(r.CreatedAt),
	}
	if !r.ApprovedBy.IsZero() {
		v := rest.Uuid(r.ApprovedBy)
		out.ApprovedBy = &v
	}
	if !r.ResultExpiresAt.IsZero() {
		v := stamp(r.ResultExpiresAt)
		out.ResultExpiresAt = &v
	}
	if !r.CompletedAt.IsZero() {
		v := stamp(r.CompletedAt)
		out.CompletedAt = &v
	}
	if r.Error != "" {
		v := r.Error
		out.Error = &v
	}
	if len(r.Progress) > 0 {
		v := rest.JsonObject(r.Progress)
		out.Progress = &v
	}
	return out
}

// stamp is the time form of the API: UTC with milliseconds.
func stamp(t time.Time) time.Time { return t.UTC().Truncate(time.Millisecond) }

// missingBody is the 422 of a write without a body.
func missingBody() error {
	return errs.New(errs.CodeValidationFailed, "the request body is required").
		WithFields(errs.FieldError{Pointer: "", Code: "required", Detail: "the request body is required"})
}

// Cursors is the codec of the opaque list cursors of the privacy requests (platform/httpserver).
type Cursors struct{}

// Compile-time check.
var _ app.Cursors = Cursors{}

// Encode implements app.Cursors with httpserver.EncodeCursor.
func (Cursors) Encode(list string, binding []byte, key any) (string, error) {
	return httpserver.EncodeCursor(list, binding, key)
}

// Decode implements app.Cursors with httpserver.DecodeCursor (400 cursor_invalid on a mismatch).
func (Cursors) Decode(list string, binding []byte, s string, key any) error {
	return httpserver.DecodeCursor(list, binding, s, key)
}
