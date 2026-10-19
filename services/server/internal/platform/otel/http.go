// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// msgHTTPRequest is the constant message of the access log.
const msgHTTPRequest = "http request"

// routeUnmatched is http.route of a request no route matched (signals.md "Kardinalität" item 2).
const routeUnmatched = "unmatched"

// methodOther is http.request.method of a method outside the standard set; spans use HTTP.
const (
	methodOther     = "_OTHER"
	spanMethodOther = "HTTP"
)

// standardMethods are the methods of the HTTP semantic conventions; every other method is one
// value, so a client cannot open new span names or metric series.
var standardMethods = map[string]bool{
	http.MethodGet: true, http.MethodHead: true, http.MethodPost: true, http.MethodPut: true,
	http.MethodDelete: true, http.MethodConnect: true, http.MethodOptions: true,
	http.MethodTrace: true, http.MethodPatch: true,
}

// httpState is what Middleware reads per request; Setup installs it.
type httpState struct {
	trustRemoteParent bool
	logger            *slog.Logger
	// pseudo derives bothy.principal.pseudo_id in AnnotatePrincipal; nil before Setup.
	pseudo *Pseudonymizer
}

var currentHTTP atomic.Pointer[httpState]

func setHTTPState(trust bool, logger *slog.Logger, pseudo *Pseudonymizer) {
	currentHTTP.Store(&httpState{trustRemoteParent: trust, logger: logger, pseudo: pseudo})
}

// kindUser is the principal kind of a person; only people get a pseudonym (privacy.md
// "Pseudonyms" items 1 and 4).
const kindUser = "user"

// AnnotatePrincipal sets the principal of the request on the span of ctx (signals.md, the server
// span row): bothy.principal.kind always, and for a person (kind "user") its pseudonym
// bothy.principal.pseudo_id from the Pseudonymizer of Setup. The clear ID never goes onto the
// span (SEC-LOG-4, DP-T09); agents and apps appear by handle elsewhere, never by ID. A span that
// does not record, a missing Setup or a failing pseudonym key leave the span without the
// pseudonym. The HTTP chain calls it after the authentication.
func AnnotatePrincipal(ctx context.Context, kind, principalID string) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}
	span.SetAttributes(attribute.String(attr.PrincipalKind, kind))
	st := currentHTTP.Load()
	if kind != kindUser || principalID == "" || st == nil || st.pseudo == nil {
		return
	}
	if kv, err := st.pseudo.Attr(ctx, principalID); err == nil {
		span.SetAttributes(kv)
	}
}

// loadHTTPState returns the state of the last Setup. Before any Setup a remote parent is not
// trusted and the access log goes through slog.Default behind the allowlist filter.
func loadHTTPState() *httpState {
	if s := currentHTTP.Load(); s != nil {
		return s
	}
	return &httpState{logger: slog.New(newAllowlistHandler(slog.Default().Handler()))}
}

// routeHolder carries the route pattern from RecordRoute, deep in the chain, back to Middleware:
// a request copy below otelhttp cannot carry it back.
type routeHolder struct {
	route string
}

type routeKey struct{}

func holderFrom(ctx context.Context) *routeHolder {
	h, _ := ctx.Value(routeKey{}).(*routeHolder)
	return h
}

// Middleware is the outer HTTP middleware of the server (02 "Kette im api-Knoten" step 2): an
// otelhttp server span, the http.server.request.duration metric and one access log record per
// request. It reads the tracer and meter providers of the last Setup, so serve calls Setup first.
//
// By default an incoming traceparent only becomes a span link: the server span is a new root
// whose trace id and sampling come from the server (signals.md "Weitergabe" item 2). With
// telemetry.traces.trust_remote_parent the incoming context is the parent. The span is named
// "<METHOD>" until RecordRoute names the route, then "<METHOD> <route>"; a request without a
// route gets http.route "unmatched". The access log has the message "http request" and only
// http.request.method, http.route, http.response.status_code, error.type (5xx) and
// user_agent.original: User-Agent is the only header read for logging, and the path and query
// never appear (SEC-LOG-2, SEC-LOG-3).
//
// Example:
//
//	mux := http.NewServeMux()
//	mux.HandleFunc("GET /api/v1/threads/{id}", func(w http.ResponseWriter, r *http.Request) {
//		otel.RecordRoute(r, "/api/v1/threads/{id}")
//		// ...
//	})
//	handler := otel.Middleware(mux)
func Middleware(next http.Handler) http.Handler {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveInner(w, r, next)
	})
	traced := otelhttp.NewHandler(inner, "",
		otelhttp.WithPropagators(propagation.TraceContext{}),
		otelhttp.WithPublicEndpointFn(func(*http.Request) bool { return !loadHTTPState().trustRemoteParent }),
		otelhttp.WithSpanNameFormatter(spanName),
	)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), routeKey{}, &routeHolder{})
		ctx = otelhttp.ContextWithLabeler(ctx, &otelhttp.Labeler{})
		traced.ServeHTTP(w, r.WithContext(ctx))
	})
}

// serveInner runs inside the otelhttp span: it serves next, completes the route and writes the
// access log before otelhttp records the metric. r is the request otelhttp passes on, the one
// whose Pattern otelhttp reads for the metric afterwards.
//
// A panic of next goes on unchanged to the recovery layer outside (02 "Kette im api-Knoten"
// step 1), and it is re-raised inside the deferred call, so the stack the recovery layer logs
// still shows where it happened. Before that the access log and the span record status 500 (or
// the status already sent). otelhttp records http.server.request.duration only when the handler
// returns, so a panicking request is not in that metric.
func serveInner(w http.ResponseWriter, r *http.Request, next http.Handler) {
	sw := &statusWriter{ResponseWriter: w}
	returned := false
	defer func() {
		if returned {
			return
		}
		p := recover()
		status := sw.status
		if status == 0 {
			status = http.StatusInternalServerError
		}
		span := trace.SpanFromContext(r.Context())
		span.SetAttributes(attribute.Int(attr.HTTPResponseStatusCode, status))
		if status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, "")
		}
		finishRequest(r, status)
		if p != nil {
			panic(p)
		}
		// p is nil after runtime.Goexit, which goes on by itself.
	}()
	next.ServeHTTP(sw, r)
	returned = true
	finishRequest(r, sw.code())
}

// finishRequest completes the route, sets error.type for a 5xx status and writes the access log.
func finishRequest(r *http.Request, status int) {
	ctx := r.Context()
	route := ""
	if h := holderFrom(ctx); h != nil {
		route = h.route
	}
	if route == "" {
		// A ServeMux directly below sets the pattern on this request.
		if route = routeOf(r.Pattern); route == "" {
			route = routeUnmatched
		}
		markRoute(ctx, r.Method, route)
	}
	// otelhttp takes http.route of the metric from r.Pattern after the handler, and that value
	// wins over the labeler. An outer ServeMux that only knows a prefix (with a request copy below
	// it, as http.StripPrefix makes) left its own pattern here, so the route RecordRoute named is
	// written back.
	if route == routeUnmatched {
		r.Pattern = ""
	} else {
		r.Pattern = route
	}
	attrs := make([]slog.Attr, 0, 5)
	attrs = append(attrs,
		slog.String(attr.HTTPRequestMethod, logMethod(r.Method)),
		slog.String(attr.HTTPRoute, route),
		slog.Int(attr.HTTPResponseStatusCode, status))
	if status >= http.StatusInternalServerError {
		errType := strconv.Itoa(status)
		trace.SpanFromContext(ctx).SetAttributes(attribute.String(attr.ErrorType, errType))
		if l, ok := otelhttp.LabelerFromContext(ctx); ok {
			l.Add(attribute.String(attr.ErrorType, errType))
		}
		attrs = append(attrs, slog.String(attr.ErrorType, errType))
	}
	if ua := r.UserAgent(); ua != "" {
		attrs = append(attrs, slog.String(attr.UserAgentOriginal, ua))
	}
	loadHTTPState().logger.LogAttrs(ctx, slog.LevelInfo, msgHTTPRequest, attrs...)
}

// RecordRoute records the route pattern of the request: it names the server span
// "<METHOD> <route>" and sets http.route on the span, on the http.server.request.duration metric
// and on the access log. The innermost layer calls it after routing (M1-18), from the request
// goroutine. pattern is the route as registered, with or without a method prefix
// ("GET /api/v1/threads/{id}" or "/api/v1/threads/{id}"); only the part from the first slash is
// used, never the raw path.
func RecordRoute(r *http.Request, pattern string) {
	if route := routeOf(pattern); route != "" {
		markRoute(r.Context(), r.Method, route)
	}
}

func markRoute(ctx context.Context, method, route string) {
	if h := holderFrom(ctx); h != nil {
		h.route = route
	}
	span := trace.SpanFromContext(ctx)
	span.SetName(spanMethod(method) + " " + route)
	span.SetAttributes(attribute.String(attr.HTTPRoute, route))
	if l, ok := otelhttp.LabelerFromContext(ctx); ok {
		l.Add(attribute.String(attr.HTTPRoute, route))
	}
}

// routeOf returns the path part of a ServeMux pattern, the same way otelhttp derives http.route.
func routeOf(pattern string) string {
	if i := strings.IndexByte(pattern, '/'); i >= 0 {
		return pattern[i:]
	}
	return ""
}

// spanName is the span name formatter of otelhttp: "<METHOD>" until the route is known.
func spanName(_ string, r *http.Request) string {
	m := spanMethod(r.Method)
	if h := holderFrom(r.Context()); h != nil && h.route != "" {
		return m + " " + h.route
	}
	if route := routeOf(r.Pattern); route != "" {
		return m + " " + route
	}
	return m
}

func spanMethod(m string) string {
	if standardMethods[m] {
		return m
	}
	return spanMethodOther
}

func logMethod(m string) string {
	if standardMethods[m] {
		return m
	}
	return methodOther
}

// statusWriter records the status code for the access log. Flush and Hijack go through
// http.ResponseController, and Unwrap exposes the writer below, so streaming and protocol
// upgrades keep working behind it.
type statusWriter struct {
	http.ResponseWriter
	status   int
	hijacked bool
}

// WriteHeader records the first final status code (or 101) and passes the call on.
func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 && (code >= http.StatusOK || code == http.StatusSwitchingProtocols) {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

// Write passes the call on; a body without WriteHeader means 200.
func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// Flush flushes the writer below when it can.
func (w *statusWriter) Flush() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

// Hijack hands the connection over when the writer below allows it.
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		w.hijacked = true
	}
	return conn, rw, err
}

// Unwrap returns the writer below for http.ResponseController.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) code() int {
	switch {
	case w.status != 0:
		return w.status
	case w.hijacked:
		return http.StatusSwitchingProtocols
	default:
		return http.StatusOK
	}
}
