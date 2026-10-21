// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/platform/otel"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// errProofNotChecked is what Hijack returns when the handler never checked its route's action.
var errProofNotChecked = errors.New("httpserver: authorization not checked; the connection is not handed over")

// routeStage is StageRoute: the operation goes into the context and onto the span, the API
// headers of SEC-HDR-2 are preset, the body limit and the deadline of the route class apply, and
// the response writer buffers JSON to repair or refuse null and integers beyond 2^53 and to
// enforce the proof of the authorize check.
func (c *Chain) routeStage(op Operation, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		otel.RecordRoute(r, op.Pattern)
		h := w.Header()
		h.Set(headerCacheControl, valueNoStore)
		h.Set(headerCORP, valueSameOrigin)

		limit, timeout := c.cfg.MaxJSONBodyBytes, c.cfg.RequestTimeout
		if op.RequestBody.transfer() {
			limit = c.cfg.MaxUploadBytes
		}
		if op.transfer() {
			timeout = c.cfg.TransferTimeout
		}
		// A body is read under the deadline of the route class (02 "Deadlines"), so a client that
		// trickles its body cannot hold a request beyond it; transfers also bound the write. A JSON
		// body is read whole by StageBody, which lifts the read deadline again: an expired read
		// deadline makes net/http cancel the request context, which would turn the context
		// deadline of a running handler into a cancellation. A writer without deadline support (a
		// test recorder) keeps the context deadline only.
		deadline := time.Now().Add(timeout)
		rc := http.NewResponseController(w)
		if r.Body != nil && r.Body != http.NoBody {
			_ = rc.SetReadDeadline(deadline)
		}
		if op.transfer() {
			_ = rc.SetWriteDeadline(deadline)
		}
		ctx, cancel := context.WithTimeout(withOperation(r.Context(), op), timeout)
		defer cancel()
		r = r.WithContext(ctx)

		rw := &routeWriter{w: w, r: r, c: c, op: op}
		if r.ContentLength > limit {
			// SEC-IN-3: the limit applies before reading; a declared length above it is refused at
			// once.
			writeProblem(rw, r, payloadTooLarge(ctx))
			rw.finish()
			return
		}
		if r.Body != nil && r.Body != http.NoBody {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		next.ServeHTTP(rw, r.WithContext(context.WithValue(ctx, routeWriterKey{}, rw)))
		rw.finish()
	})
}

type routeWriterKey struct{}

// routeWriter is the response writer of StageRoute. At the first WriteHeader or Write it decides:
// a success (status below 400) without a check of the route's action is discarded and replaced
// by 500 internal_error at the end (02 9.3); a JSON or problem body is buffered and scanned at the
// end (ADR 0053); any other body streams through unchanged.
type routeWriter struct {
	w  http.ResponseWriter
	r  *http.Request
	c  *Chain
	op Operation

	// checks is the context with the check log, set by StageProof; nil before. The writer lives
	// for one request, and the context is the one of that request.
	checks context.Context // NOSONAR: per-request writer, the context never outlives the request
	// base is the header set before the handler ran (StageProof); a refused response goes out
	// with these headers only.
	base http.Header

	// replayed is set by StageIdempotency for a stored response: the check of the first
	// request counts, authorize does not run again (06 "Idempotency" point 4).
	replayed bool

	decided   bool
	status    int
	buffering bool
	refused   bool
	buf       bytes.Buffer
}

// Header returns the header map of the response.
func (rw *routeWriter) Header() http.Header { return rw.w.Header() }

// WriteHeader decides the response at its first final status, see routeWriter.
func (rw *routeWriter) WriteHeader(code int) {
	if rw.decided {
		return
	}
	if code < http.StatusOK && code != http.StatusSwitchingProtocols {
		rw.w.WriteHeader(code) // 1xx informational responses pass
		return
	}
	rw.decided, rw.status = true, code
	if code < http.StatusBadRequest && !rw.proven() {
		rw.refused = true
		return
	}
	if isJSONMedia(rw.w.Header().Get(headerContentType)) {
		rw.buffering = true
		return
	}
	rw.w.Header().Del(headerServer)
	rw.w.WriteHeader(code)
}

// Write writes body bytes; without WriteHeader it means 200, as in net/http.
func (rw *routeWriter) Write(b []byte) (int, error) {
	if !rw.decided {
		rw.WriteHeader(http.StatusOK)
	}
	switch {
	case rw.refused:
		return len(b), nil
	case rw.buffering:
		return rw.buf.Write(b)
	}
	return rw.w.Write(b)
}

// Flush flushes a streamed body; buffered and refused bodies wait for finish.
func (rw *routeWriter) Flush() {
	if !rw.decided {
		rw.WriteHeader(http.StatusOK)
	}
	if rw.refused || rw.buffering {
		return
	}
	_ = http.NewResponseController(rw.w).Flush()
}

// Hijack hands the connection over only after the proof: a protocol switch counts as success.
func (rw *routeWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if !rw.proven() {
		rw.decided, rw.status, rw.refused = true, http.StatusSwitchingProtocols, true
		return nil, nil, errProofNotChecked
	}
	rw.decided = true
	return http.NewResponseController(rw.w).Hijack()
}

// SetReadDeadline passes the deadline to the writer below (http.ResponseController).
func (rw *routeWriter) SetReadDeadline(t time.Time) error {
	return http.NewResponseController(rw.w).SetReadDeadline(t)
}

// SetWriteDeadline passes the deadline to the writer below (http.ResponseController).
func (rw *routeWriter) SetWriteDeadline(t time.Time) error {
	return http.NewResponseController(rw.w).SetWriteDeadline(t)
}

// proven reports whether the check log holds a check of the route's action (Authorize or Scope).
func (rw *routeWriter) proven() bool {
	if rw.replayed {
		return true
	}
	if rw.checks == nil {
		return false
	}
	return slices.ContainsFunc(authorize.Checks(rw.checks), func(ch authorize.Check) bool {
		return ch.Action == rw.op.Action
	})
}

// finish completes the response after the handler returned: an empty response is an implicit 200
// and goes through the proof as well; a refused response becomes 500; a buffered body is scanned.
func (rw *routeWriter) finish() {
	if !rw.decided {
		rw.WriteHeader(http.StatusOK)
	}
	ctx := rw.r.Context()
	switch {
	case rw.refused:
		rw.notChecked(ctx)
	case rw.buffering:
		rw.sendBuffered(ctx)
	}
}

// notChecked replaces a success without proof by 500 internal_error, counts it in
// bothy.authz.errors and logs it with the route and the action (02 9.3, alarm BothyAuthzError).
func (rw *routeWriter) notChecked(ctx context.Context) {
	rw.c.authzErrors.Add(ctx, 1, errs.CauseAuthzNotChecked)
	rw.c.log.LogAttrs(ctx, slog.LevelError, LogAuthzNotChecked,
		slog.String(attr.HTTPRoute, rw.op.Path),
		slog.String(attr.Action, rw.op.Action),
		slog.String(attr.Reason, errs.CauseAuthzNotChecked),
		slog.String(attr.CorrelationID, reqmeta.CorrelationID(ctx)))
	rw.replace(errs.New(errs.CodeInternalError, "").WithCause(errs.CauseAuthzNotChecked))
}

// sendBuffered scans the buffered JSON: null members are removed and logged, a null array element,
// a top-level null, an integer beyond 2^53 - 1 or a body that is not JSON replace the response by
// 500 internal_error. A clean body goes out byte for byte.
func (rw *routeWriter) sendBuffered(ctx context.Context) {
	if rw.buf.Len() == 0 {
		// A JSON content type without a body (204, an empty answer) has nothing to scan.
		rw.w.Header().Del(headerServer)
		rw.w.WriteHeader(rw.status)
		return
	}
	out, removed, err := SanitizeJSON(rw.buf.Bytes())
	if err != nil {
		rw.c.log.LogAttrs(ctx, slog.LevelError, LogResponseRefused,
			slog.String(attr.HTTPRoute, rw.op.Path),
			slog.String(attr.ErrorType, sanitizeErrorType(err)),
			slog.String(attr.CorrelationID, reqmeta.CorrelationID(ctx)))
		rw.replace(errs.New(errs.CodeInternalError, ""))
		return
	}
	if removed > 0 {
		rw.c.log.LogAttrs(ctx, slog.LevelError, LogNullRepaired,
			slog.String(attr.HTTPRoute, rw.op.Path),
			slog.String(attr.ErrorType, errorTypeNullMember),
			slog.Int(attr.HTTPResponseStatusCode, rw.status),
			slog.String(attr.CorrelationID, reqmeta.CorrelationID(ctx)))
	}
	h := rw.w.Header()
	h.Del(headerServer)
	h.Set(headerContentLength, strconv.Itoa(len(out)))
	rw.w.WriteHeader(rw.status)
	_, _ = rw.w.Write(out)
}

// replace discards the handler's headers and body and writes e as a problem with the headers
// the response had before the handler ran.
func (rw *routeWriter) replace(e *errs.Error) {
	h := rw.w.Header()
	for k := range h {
		delete(h, k)
	}
	for k, v := range rw.base {
		h[k] = slices.Clone(v)
	}
	writeProblem(rw.w, rw.r, e)
}

// isJSONMedia reports whether the Content-Type is application/json or application/problem+json.
func isJSONMedia(contentType string) bool {
	if contentType == "" {
		return false
	}
	media, _, err := mime.ParseMediaType(contentType)
	return err == nil && (media == mediaJSON || media == mediaProblem)
}

// proofStage is StageProof: it installs the check log of kernel/authorize and arms the response
// writer with it, together with the headers the response has before the handler runs.
func (c *Chain) proofStage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := authorize.WithCheckLog(r.Context())
		if rw, ok := ctx.Value(routeWriterKey{}).(*routeWriter); ok {
			rw.checks = ctx
			rw.base = rw.w.Header().Clone()
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
