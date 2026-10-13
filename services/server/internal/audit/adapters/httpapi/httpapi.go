// SPDX-License-Identifier: Apache-2.0

// Package httpapi holds the REST handlers of the audit module (adapters/httpapi, 02 "5. Schichten
// im Modul"): listAuditEvents, exportAuditEvents and verifyAuditChain of the generated strict
// server over the port audit.Log (M1-50). The use cases authorize (owner with fresh step-up) and
// audit; the handlers only convert. Errors go to the problem writer of the chain unchanged.
//
// Example:
//
//	h := httpapi.New(log) // embedded into wire.Handlers
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/audit"
	"github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// ndjson is the media type of the export (docs/api/openapi.yaml exportAuditEvents).
const ndjson = "application/x-ndjson"

// Log implements listAuditEvents, exportAuditEvents and verifyAuditChain of
// rest.StrictServerInterface over audit.Log.
type Log struct {
	log audit.Log
}

// New returns the handlers over the port.
func New(log audit.Log) *Log { return &Log{log: log} }

// ListAuditEvents implements rest.StrictServerInterface (GET /audit/events).
func (h *Log) ListAuditEvents(ctx context.Context, req rest.ListAuditEventsRequestObject) (rest.ListAuditEventsResponseObject, error) {
	p := req.Params
	q := audit.Query{
		ActorID: idOf(p.ActorId), ResourceID: idOf(p.ResourceId), Action: text(p.Action), ResourceKind: text(p.ResourceKind),
		From: timeOf(p.From), To: timeOf(p.To), Cursor: text(p.Cursor),
	}
	if p.Limit != nil {
		q.Limit = *p.Limit
	}
	if p.Decision != nil {
		q.Decision = string(*p.Decision)
	}
	page, err := h.log.List(ctx, q)
	if err != nil {
		return nil, err
	}
	return listResponse{page: page}, nil
}

// listResponse writes a page with every item in exactly the canonical form of the stored row plus
// hash (EventJSON), so each item verifies offline like an export line; a generic JSON encoder
// would write occurred_at without the trailing zeros of its milliseconds (19 SEC-AUD-4).
type listResponse struct {
	page audit.Page
}

// VisitListAuditEventsResponse writes 200 with the page (AuditEventPage).
func (r listResponse) VisitListAuditEventsResponse(w http.ResponseWriter) error {
	buf := []byte(`{"items":[`)
	for i, e := range r.page.Items {
		line, err := domain.EventJSON(app.RowOf(e))
		if err != nil {
			return err
		}
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, line...)
	}
	buf = append(buf, ']')
	if r.page.NextCursor != "" {
		cursor, err := json.Marshal(r.page.NextCursor)
		if err != nil {
			return err
		}
		buf = append(append(buf, `,"next_cursor":`...), cursor...)
	}
	buf = append(buf, '}')
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(buf)
	return err
}

// ExportAuditEvents implements rest.StrictServerInterface (GET /audit/export): the use case runs
// when the response is written, so it streams into the response writer batch by batch.
func (h *Log) ExportAuditEvents(ctx context.Context, req rest.ExportAuditEventsRequestObject) (rest.ExportAuditEventsResponseObject, error) {
	return exportResponse{ctx: ctx, log: h.log, from: req.Params.From, to: req.Params.To}, nil
}

// exportResponse runs the export into the response writer.
type exportResponse struct {
	ctx      context.Context //nolint:containedctx // the strict server hands the writer to the response, not to the handler
	log      audit.Log
	from, to time.Time
}

// VisitExportAuditEventsResponse runs the export. An error before the first line goes to the
// problem writer; an error after it aborts the response (http.ErrAbortHandler), so the client
// sees a broken transfer, never a complete-looking stream.
func (r exportResponse) VisitExportAuditEventsResponse(w http.ResponseWriter) error {
	sw := &streamWriter{w: w}
	err := r.log.Export(r.ctx, r.from, r.to, sw)
	switch {
	case err == nil:
		sw.start()
		return nil
	case sw.started:
		panic(http.ErrAbortHandler)
	}
	return err
}

// streamWriter writes the export as NDJSON: one canonical AuditEvent per line, flushed per batch.
type streamWriter struct {
	w       http.ResponseWriter
	started bool
}

// start writes the status and the media type once.
func (s *streamWriter) start() {
	if s.started {
		return
	}
	s.started = true
	s.w.Header().Set("Content-Type", ndjson)
	s.w.WriteHeader(http.StatusOK)
}

// WriteEvents writes one batch and flushes it (audit.EventWriter).
func (s *streamWriter) WriteEvents(_ context.Context, events []audit.Event) error {
	var buf []byte
	for _, e := range events {
		line, err := domain.EventJSON(app.RowOf(e))
		if err != nil {
			s.start()
			return err
		}
		buf = append(append(buf, line...), '\n')
	}
	s.start()
	if _, err := s.w.Write(buf); err != nil {
		return err
	}
	return http.NewResponseController(s.w).Flush()
}

// VerifyAuditChain implements rest.StrictServerInterface (POST /audit/verify).
func (h *Log) VerifyAuditChain(ctx context.Context, req rest.VerifyAuditChainRequestObject) (rest.VerifyAuditChainResponseObject, error) {
	var vr audit.VerifyRequest
	if req.Body != nil {
		vr.FromSeq, vr.ToSeq = req.Body.FromSeq, req.Body.ToSeq
	}
	res, err := h.log.Verify(ctx, vr)
	if err != nil {
		return nil, err
	}
	body := rest.AuditVerifyResult{
		Ok: res.OK, CheckedFromSeq: res.CheckedFromSeq, CheckedToSeq: res.CheckedToSeq,
		CheckpointsCompared: res.CheckpointsCompared, Warnings: res.Warnings,
	}
	if !res.OK {
		seq, reason := res.FirstBrokenSeq, rest.AuditVerifyResultFailureReason(res.FailureReason)
		body.FirstBrokenSeq, body.FailureReason = &seq, &reason
	}
	if res.LastCheckpointSeq > 0 {
		last := res.LastCheckpointSeq
		body.LastCheckpointSeq = &last
	}
	return rest.VerifyAuditChain200JSONResponse{Body: body}, nil
}

// idOf converts an optional query parameter.
func idOf(u *rest.Uuid) ids.ID {
	if u == nil {
		return ids.ID{}
	}
	return ids.ID(*u)
}

// text converts an optional query parameter.
func text(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// timeOf converts an optional query parameter.
func timeOf(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

// Cursors is the cursor codec of platform/httpserver behind app.Cursors.
type Cursors struct{}

// Compile-time check: the codec is the outgoing port of List.
var _ app.Cursors = Cursors{}

// Encode implements app.Cursors with httpserver.EncodeCursor.
func (Cursors) Encode(list string, binding []byte, key any) (string, error) {
	return httpserver.EncodeCursor(list, binding, key)
}

// Decode implements app.Cursors with httpserver.DecodeCursor (400 cursor_invalid on a mismatch).
func (Cursors) Decode(list string, binding []byte, s string, key any) error {
	return httpserver.DecodeCursor(list, binding, s, key)
}
