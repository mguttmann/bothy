// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// Media types and header names the chain writes in more than one place.
const (
	mediaJSON    = "application/json"
	mediaProblem = "application/problem+json"

	headerContentType   = "Content-Type"
	headerCacheControl  = "Cache-Control"
	headerCORP          = "Cross-Origin-Resource-Policy"
	headerNoSniff       = "X-Content-Type-Options"
	headerReferrer      = "Referrer-Policy"
	headerCorrelationID = "Correlation-Id"
	headerRetryAfter    = "Retry-After"
	headerAllow         = "Allow"
	headerContentLength = "Content-Length"
	headerIfMatch       = "If-Match"
	headerOrigin        = "Origin"
	headerVary          = "Vary"
	headerAuthorization = "Authorization"
	headerDPoP          = "DPoP"
	headerServer        = "Server"
	headerForwarded     = "Forwarded"
	headerXForwardedFor = "X-Forwarded-For"
	problemTypePrefix   = "/api/v1/problems/"
	tokenPathParam      = "{token}"
	defaultRetryAfter   = 5 * time.Second
	keyMaxJSONBodyBytes = "http.max_json_body_bytes"
	keyMaxUploadBytes   = "file.max_upload_bytes"
	keyMaxJSONDepth     = "http.max_json_depth"
	valueNoStore        = "no-store"
	valueSameOrigin     = "same-origin"
	valueNoSniff        = "nosniff"
	valueNoReferrer     = "no-referrer"
	errorTypeInternal   = "internal_error"
	errorTypePanic      = "panic"
)

// The messages of the chain's log records; tests and the harness look for them.
const (
	// LogRequestFailed is an error response with status 500 or 503 (error.type is the code,
	// bothy.reason the internal cause).
	LogRequestFailed = "request failed"
	// LogPanicRecovered is a panic caught by the recover step.
	LogPanicRecovered = "panic recovered"
	// LogAuthzNotChecked is a success without a check of the route's action (02 9.3).
	LogAuthzNotChecked = "authorization not checked"
	// LogNullRepaired is a response whose null members were removed (ADR 0053).
	LogNullRepaired = "response contained JSON null"
	// LogResponseRefused is a JSON response replaced by 500 (null element, unsafe integer, not JSON).
	LogResponseRefused = "response refused"
)

// problem is the body of application/problem+json (RFC 9457, 06 "Fehler", schema Problem). Every
// optional member is left out when empty, because the server never sends JSON null (ADR 0053).
type problem struct {
	Type          string       `json:"type"`
	Title         string       `json:"title"`
	Status        int          `json:"status"`
	Code          string       `json:"code"`
	CorrelationID string       `json:"correlation_id"`
	Detail        string       `json:"detail,omitempty"`
	Instance      string       `json:"instance,omitempty"`
	Errors        []fieldError `json:"errors,omitempty"`
	Current       any          `json:"current,omitempty"`
	Reason        string       `json:"reason,omitempty"`
	Hint          string       `json:"hint,omitempty"`
}

// fieldError is one entry of errors[] (schema FieldError).
type fieldError struct {
	Pointer string `json:"pointer"`
	Code    string `json:"code"`
	Detail  string `json:"detail,omitempty"`
}

// WriteProblem writes err as application/problem+json (RFC 9457, ADR 0015, 06 "Fehler"). It is
// the one error writer of the server: every stage of the chain and the generated server
// (routes.Options.ResponseErrorHandlerFunc) use it.
//
// The status and code come from the first *errs.Error in the chain of err. Two standard errors
// are mapped as well: *http.MaxBytesError becomes 413 payload_too_large with detail naming the
// configuration key of the route's limit, and an error wrapping context.DeadlineExceeded while
// the request context has run out becomes 503 unavailable (cause deadline_exceeded). Every other
// error becomes 500 internal_error; its type goes to the log, never its text to the body.
//
// The body carries type /api/v1/problems/{code}, the title of the code, status, code,
// correlation_id from kernel/reqmeta, instance (the request path without query, with every
// {token} segment of the route template kept as the literal {token}; unmatched requests have no
// instance, because no route says which segments are secret), and detail, errors[], current,
// reason and hint when the error carries them. The headers of the error (WWW-Authenticate,
// DPoP-Nonce, Allow) are applied; 503 unavailable and maintenance always carry Retry-After
// (http.unavailable_retry_after unless the error names its own), 429 when the error names one.
//
// Example:
//
//	if err := uc.Do(ctx, in); err != nil {
//		httpserver.WriteProblem(w, r, err)
//		return
//	}
func WriteProblem(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		err = errs.New(errs.CodeInternalError, "")
	}
	noteRetryable(r.Context(), err)
	e := toError(r, err)
	writeProblem(w, r, e)
}

// ValidationErrorHandler is the std-http ErrorHandlerFunc of the generated server: a parameter
// that does not bind is 400 validation_failed. Schema validation (StageValidate) runs before the
// generated wrapper, so it fires only for a binding rule the schema does not express.
func ValidationErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	if _, ok := errs.As(err); !ok {
		err = errs.Wrap(errs.CodeValidationFailed, err, "").
			WithFields(errs.FieldError{Pointer: "", Code: fieldCodeInvalid, Detail: "A parameter could not be read."})
	}
	WriteProblem(w, r, err)
}

// MalformedRequestHandler is the strict RequestErrorHandlerFunc of the generated server: a body
// the strict wrapper cannot decode is 400 malformed_request.
func MalformedRequestHandler(w http.ResponseWriter, r *http.Request, err error) {
	if _, ok := errs.As(err); !ok {
		err = errs.Wrap(errs.CodeMalformedRequest, err, "")
	}
	WriteProblem(w, r, err)
}

// toError maps err to the *errs.Error that becomes the response, and logs what a client must
// not see.
func toError(r *http.Request, err error) *errs.Error {
	if e, ok := errs.As(err); ok {
		if e.Kind().Status() >= http.StatusInternalServerError {
			logFailure(r, e.PublicCode(), e.Cause, "")
		}
		return e
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return payloadTooLarge(r.Context())
	}
	if errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) && errors.Is(r.Context().Err(), context.DeadlineExceeded) {
		logFailure(r, errs.CodeUnavailable, errs.CauseDeadlineExceeded, "")
		return errs.Wrap(errs.CodeUnavailable, err, "").WithCause(errs.CauseDeadlineExceeded)
	}
	logFailure(r, errs.CodeInternalError, "", fmt.Sprintf("%T", err))
	return errs.Wrap(errs.CodeInternalError, err, "")
}

// payloadTooLarge is 413 with detail naming the key of the route's body limit.
func payloadTooLarge(ctx context.Context) *errs.Error {
	key := keyMaxJSONBodyBytes
	if op, ok := OperationFrom(ctx); ok && op.RequestBody.transfer() {
		key = keyMaxUploadBytes
	}
	return errs.New(errs.CodePayloadTooLarge, "Body exceeds "+key+".")
}

// logFailure writes "request failed" at level error with the public code as error.type, the
// internal cause as bothy.reason and the route. goType names the Go type of an error without a
// kind; texts never reach the log through the allowlist of platform/otel (signals.md "Logs").
func logFailure(r *http.Request, code errs.Code, cause, goType string) {
	ctx := r.Context()
	attrs := []slog.Attr{slog.String(attr.ErrorType, string(code)), slog.String(attr.CorrelationID, reqmeta.CorrelationID(ctx))}
	if op, ok := OperationFrom(ctx); ok {
		attrs = append(attrs, slog.String(attr.HTTPRoute, op.Path))
	}
	if cause != "" {
		attrs = append(attrs, slog.String(attr.Reason, cause))
	}
	if goType != "" {
		attrs = append(attrs, slog.String(attr.ExceptionType, goType))
	}
	loggerFrom(ctx).LogAttrs(ctx, slog.LevelError, LogRequestFailed, attrs...)
}

// writeProblem writes e with the headers of the chain; the body is marshalled from fixed types,
// so it cannot fail.
func writeProblem(w http.ResponseWriter, r *http.Request, e *errs.Error) {
	ctx := r.Context()
	code := e.PublicCode()
	status := code.Kind().Status()
	p := problem{
		Type:          problemTypePrefix + string(code),
		Title:         code.Title(),
		Status:        status,
		Code:          string(code),
		CorrelationID: correlationOf(ctx),
		Detail:        e.Detail,
		Reason:        e.Reason,
		Hint:          e.Hint,
		Current:       e.Current,
	}
	if code == errs.CodeInternalError {
		// An internal error never explains itself: detail could carry a driver text.
		p.Detail, p.Reason, p.Hint, p.Current = "", "", "", nil
	}
	if op, ok := OperationFrom(ctx); ok {
		p.Instance = instancePath(r.URL.EscapedPath(), op.Path)
	}
	for _, f := range e.Fields {
		p.Errors = append(p.Errors, fieldError(f))
	}
	if code == errs.CodeValidationFailed && len(p.Errors) == 0 {
		p.Errors = []fieldError{{Pointer: "", Code: fieldCodeInvalid}}
	}
	body, err := json.Marshal(p)
	if err != nil {
		// Only Current can fail to marshal; the problem goes out without it.
		p.Current = nil
		body, _ = json.Marshal(p)
	}
	h := w.Header()
	for name, values := range e.Headers {
		h[name] = append([]string(nil), values...)
	}
	if retry := retryAfter(ctx, e); retry > 0 {
		h.Set(headerRetryAfter, strconv.FormatInt(int64(math.Ceil(retry.Seconds())), 10))
	}
	setAPIHeaders(h, mediaProblem)
	h.Set(headerCorrelationID, p.CorrelationID)
	h.Set(headerContentLength, strconv.Itoa(len(body)))
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// retryAfter returns the Retry-After of e: its own value, or http.unavailable_retry_after for
// unavailable and maintenance, which always carry one (02 14.1); 0 means no header.
func retryAfter(ctx context.Context, e *errs.Error) time.Duration {
	if e.RetryAfter > 0 {
		return e.RetryAfter
	}
	if code := e.PublicCode(); code != errs.CodeUnavailable && code != errs.CodeMaintenance {
		return 0
	}
	if c := chainFrom(ctx); c != nil {
		return c.cfg.UnavailableRetryAfter
	}
	return defaultRetryAfter
}

// setAPIHeaders sets the headers of SEC-HDR-1 (HSTS comes from the secure writer of the
// listener, SecurityHeaders) and SEC-HDR-2 on an API response with the given media type.
func setAPIHeaders(h http.Header, media string) {
	h.Set(headerNoSniff, valueNoSniff)
	h.Set(headerReferrer, valueNoReferrer)
	h.Set(headerCacheControl, valueNoStore)
	h.Set(headerCORP, valueSameOrigin)
	h.Set(headerContentType, media)
	h.Del(headerServer)
}

// instancePath returns the escaped request path with every segment whose template segment is
// {token} replaced by the literal {token} (06 "Fehler" item 1, SEC-LOG-3). Paths and templates of
// different length cannot happen for a matched route; the path is then dropped to "" rather than
// risk a secret.
func instancePath(path, template string) string {
	if !strings.Contains(template, tokenPathParam) {
		return path
	}
	ps, ts := strings.Split(path, "/"), strings.Split(template, "/")
	if len(ps) != len(ts) {
		return ""
	}
	for i, t := range ts {
		if t == tokenPathParam {
			ps[i] = tokenPathParam
		}
	}
	return strings.Join(ps, "/")
}
