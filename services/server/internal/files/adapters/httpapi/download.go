// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"io"
	"net/http"
	"strconv"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The fixed headers of a delivery (12 9.3, SEC-HDR-4).
const (
	valueFileCSP     = "default-src 'none'; sandbox"
	valueSameOrigin  = "same-origin"
	valueNoSniff     = "nosniff"
	valueAcceptBytes = "bytes"
)

// DownloadFile implements rest.StrictServerInterface (GET /files/{file_id}/content): the bytes of
// the file through the server only (ADR 0048), with a ticket in the query parameter ticket for an
// anonymous request. The use case decides; the response streams without buffering.
func (h *Files) DownloadFile(ctx context.Context, req rest.DownloadFileRequestObject) (rest.DownloadFileResponseObject, error) {
	d, err := h.files.Open(ctx, files.OpenRequest{FileID: ids.ID(req.FileId), Variant: files.VariantOriginal,
		Ticket: deref(req.Params.Ticket), Range: deref(req.Params.Range), IfRange: deref(req.Params.IfRange)})
	if err != nil {
		return nil, err
	}
	return deliveryResponse{d: d}, nil
}

// DownloadThumbnail implements rest.StrictServerInterface (GET /files/{file_id}/thumbnails/{variant})
// like DownloadFile for one thumbnail variant.
func (h *Files) DownloadThumbnail(ctx context.Context, req rest.DownloadThumbnailRequestObject) (rest.DownloadThumbnailResponseObject, error) {
	d, err := h.files.Open(ctx, files.OpenRequest{FileID: ids.ID(req.FileId), Variant: string(req.Variant),
		Ticket: deref(req.Params.Ticket), Range: deref(req.Params.Range), IfRange: deref(req.Params.IfRange)})
	if err != nil {
		return nil, err
	}
	return deliveryResponse{d: d}, nil
}

// CreateFileTicket implements rest.StrictServerInterface (POST /files/{file_id}/tickets): 201 with
// the relative URL of the download and its expiry (12 9.2).
func (h *Files) CreateFileTicket(ctx context.Context, req rest.CreateFileTicketRequestObject) (rest.CreateFileTicketResponseObject, error) {
	if req.Body == nil {
		// The validator of the chain refuses a missing body first; this keeps the handler safe.
		return nil, errs.New(errs.CodeValidationFailed, "the request body is missing")
	}
	t, err := h.files.IssueTicket(ctx, ids.ID(req.FileId), string(req.Body.Variant))
	if err != nil {
		return nil, err
	}
	return rest.CreateFileTicket201JSONResponse{Body: rest.FileTicket{Url: t.URL, ExpiresAt: t.ExpiresAt.UTC()}}, nil
}

// GetTranscript implements rest.StrictServerInterface (GET /files/{file_id}/transcript): text,
// language and source only with status done, never JSON null (ADR 0053).
func (h *Files) GetTranscript(ctx context.Context, req rest.GetTranscriptRequestObject) (rest.GetTranscriptResponseObject, error) {
	v, err := h.files.Transcript(ctx, ids.ID(req.FileId))
	if err != nil {
		return nil, err
	}
	return rest.GetTranscript200JSONResponse{Body: TranscriptREST(v), Headers: rest.GetTranscript200ResponseHeaders{ETag: &v.ETag}}, nil
}

// TranscriptREST converts a transcript view into the schema Transcript: text, language and source
// only with status done (the text also when it is empty, a silent recording), never JSON null
// (ADR 0053).
func TranscriptREST(v files.TranscriptView) rest.Transcript {
	out := rest.Transcript{FileId: rest.Uuid(v.FileID), Status: rest.TranscriptStatus(v.Status)}
	if v.Status != transcriptDone {
		return out
	}
	text := v.Text
	out.Text = &text
	if v.Language != "" {
		lang := v.Language
		out.Language = &lang
	}
	if v.Source != "" {
		src := rest.TranscriptSource(v.Source)
		out.Source = &src
	}
	return out
}

// transcriptDone is the status whose text, language and source are shown.
const transcriptDone = "done"

// deliveryResponse writes an opened download: the headers of 12 9.3 before the first body byte,
// then the body streamed from the object storage (brief M3-05 design note 12). It is the response
// object of downloadFile and downloadThumbnail.
type deliveryResponse struct {
	d files.Delivery
}

// VisitDownloadFileResponse implements rest.DownloadFileResponseObject.
func (r deliveryResponse) VisitDownloadFileResponse(w http.ResponseWriter) error { return r.write(w) }

// VisitDownloadThumbnailResponse implements rest.DownloadThumbnailResponseObject.
func (r deliveryResponse) VisitDownloadThumbnailResponse(w http.ResponseWriter) error {
	return r.write(w)
}

// write sends headers and body. An error after the first byte aborts the connection
// (http.ErrAbortHandler), so the client sees a short body, never a wrong one; the body is always
// closed.
func (r deliveryResponse) write(w http.ResponseWriter) error {
	d := r.d
	defer func() { _ = d.Body.Close() }()
	h := w.Header()
	h.Set("Content-Type", d.ContentType)
	h.Set("Content-Disposition", d.Disposition)
	h.Set("X-Content-Type-Options", valueNoSniff)
	h.Set("Content-Security-Policy", valueFileCSP)
	h.Set("Cross-Origin-Resource-Policy", valueSameOrigin)
	h.Set("Cache-Control", d.CacheControl)
	h.Set("ETag", d.ETag)
	h.Set("Repr-Digest", d.ReprDigest)
	h.Set("Accept-Ranges", valueAcceptBytes)
	h.Set("Content-Length", strconv.FormatInt(d.Length(), 10))
	if d.Status == http.StatusPartialContent {
		h.Set("Content-Range", "bytes "+strconv.FormatInt(d.First, 10)+"-"+strconv.FormatInt(d.Last, 10)+"/"+strconv.FormatInt(d.Size, 10))
	}
	w.WriteHeader(d.Status)
	if n, err := io.Copy(w, d.Body); err != nil || n != d.Length() {
		panic(http.ErrAbortHandler)
	}
	return nil
}

// deref returns the value of an optional parameter, "" for none.
func deref[T ~string](p *T) string {
	if p == nil {
		return ""
	}
	return string(*p)
}
