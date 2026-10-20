// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
)

// The values of the abort record of an export (brief M3-11 note 8): the route pattern and a cause
// code, never an id, a name or content.
const (
	mediaNDJSON     = "application/x-ndjson"
	exportPattern   = "GET /api/v1/channels/{channel_id}/export"
	msgExportAbort  = "messaging: channel export aborted after the first byte"
	keyRoute        = "http.route"
	keyCause        = "bothy.cause"
	causeAborted    = "export_aborted"
	causeDeadline   = "deadline"
	causeCanceled   = "canceled"
	causeReadFailed = "read_failed"
	causeEncoding   = "encoding_failed"
)

// ExportChannel implements rest.StrictServerInterface (GET /channels/{channel_id}/export,
// M3-11): messaging.Export checks the range, export.channel and records the audit entry before
// the handler returns, so an error is a problem document and no line has been sent. The response
// streams the batches when the strict server writes it.
func (h *Messages) ExportChannel(ctx context.Context, req rest.ExportChannelRequestObject) (rest.ExportChannelResponseObject, error) {
	st, err := h.export.Channel(ctx, messaging.ChannelExportQuery{ChannelID: ids.ID(req.ChannelId), From: req.Params.From, To: req.Params.To})
	if err != nil {
		return nil, err
	}
	return exportResponse{ctx: ctx, stream: st, log: h.log}, nil
}

// exportResponse streams one export into the response writer.
type exportResponse struct {
	ctx    context.Context //nolint:containedctx // the strict server hands the writer to the response, not to the handler
	stream messaging.ExportStream
	log    *slog.Logger
}

// VisitExportChannelResponse writes 200 with Content-Type application/x-ndjson and then each
// batch as lines, flushed per batch. Correlation-Id, the RateLimit headers, Cache-Control,
// Cross-Origin-Resource-Policy and X-Content-Type-Options come from the chain. After the first
// byte no problem document can follow: an aborted stream, a read error, the deadline or a shutdown
// is logged with its cause code and the route pattern and ends the response with
// http.ErrAbortHandler, so the client sees a cut transfer, never a complete-looking one
// (D-M3-11-6). A write error (the client went away) ends the stream quietly. A range without
// messages gives 200 with an empty body.
func (r exportResponse) VisitExportChannelResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", mediaNDJSON)
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)
	if err := rc.Flush(); err != nil {
		return nil //nolint:nilerr // the client is gone; nothing more can reach it
	}
	for {
		batch, err := r.stream.Next(r.ctx)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			r.abort(cause(r.ctx, err))
		}
		buf, err := ExportLines(batch)
		if err != nil {
			r.abort(causeEncoding)
		}
		if _, err := w.Write(buf); err != nil {
			return nil //nolint:nilerr // the client is gone; nothing more can reach it
		}
		if err := rc.Flush(); err != nil {
			return nil //nolint:nilerr // as above
		}
	}
}

// abort logs the cause code and ends the response visibly (the recover stage of M1-18 passes
// http.ErrAbortHandler on to net/http).
func (r exportResponse) abort(code string) {
	r.log.LogAttrs(r.ctx, slog.LevelWarn, msgExportAbort, slog.String(keyRoute, exportPattern), slog.String(keyCause, code))
	panic(http.ErrAbortHandler)
}

// cause maps the error of an aborted export to its code.
func cause(ctx context.Context, err error) string {
	switch {
	case errors.Is(err, messaging.ErrExportAborted):
		return causeAborted
	case errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded):
		return causeDeadline
	case errors.Is(err, context.Canceled) || ctx.Err() != nil:
		return causeCanceled
	}
	return causeReadFailed
}

// ExportLines encodes a batch of the export as NDJSON (brief M3-11 note 4): per message the REST
// representation Message (MessageREST) with the whole content, without content_truncated and
// my_reactions (the caller's state, not channel content), marshalled with encoding/json and ended
// by exactly one newline. json.Marshal escapes control characters, so no line holds a raw
// newline. Errors: an encoding error of encoding/json.
//
// Example:
//
//	buf, err := httpapi.ExportLines(batch) // "{...}\n{...}\n"
func ExportLines(batch []messaging.Message) ([]byte, error) {
	var buf []byte
	for _, m := range batch {
		v := MessageREST(m)
		v.ContentTruncated, v.MyReactions = nil, nil
		line, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		buf = append(append(buf, line...), '\n')
	}
	return buf, nil
}
