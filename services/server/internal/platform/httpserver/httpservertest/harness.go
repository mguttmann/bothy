// SPDX-License-Identifier: Apache-2.0

// Package httpservertest builds the full HTTP chain of platform/httpserver around test operations
// and handlers, for the tests of httpserver itself and of every handler ticket (M1-19, M1-20 and
// the modules' adapters/httpapi). It serves on 127.0.0.1 with a port the system picks (P-10),
// records authorize checks, reads metrics through a manual OpenTelemetry reader, captures the
// chain's log records and the server spans, and fails a test whose handler sent JSON null.
//
// Example:
//
//	op := httpservertest.Op("getThing", http.MethodGet, "/api/v1/things/{thing_id}", "thing.read")
//	h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{
//		Op: op,
//		Handler: httpservertest.Authorized("thing.read", func(w http.ResponseWriter, r *http.Request) {
//			httpservertest.JSON(w, http.StatusOK, map[string]any{"id": r.PathValue("thing_id")})
//		}),
//	}}})
//	resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+id, "", "X-Mode", "fast"))
package httpservertest

import (
	"context"
	_ "embed" // the test contract
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// headerContentType is the name of the Content-Type header.
const headerContentType = "Content-Type"

// Contract is the test contract (OpenAPI 3.1) the harness validates against by default: the
// operations getThing, updateThing, createThing, createShape, createComposed, uploadBlob, postHook
// and getDiscovery with the schema features of docs/api/openapi.yaml.
//
//go:embed contract.yaml
var Contract []byte

// PublicURL is server.public_url of the harness.
const PublicURL = "https://chat.example.com"

// Route is one test operation with its handler.
type Route struct {
	Op      httpserver.Operation
	Handler http.HandlerFunc
}

// Options configure a harness; the zero value serves no route with the defaults.
type Options struct {
	// Config replaces httpserver.DefaultChainConfig(PublicURL) when set.
	Config *httpserver.ChainConfig
	// Routes are mounted through Chain.Wrap at their Op.Pattern.
	Routes []Route
	// Contract replaces the test contract; every route's Op.ID must be in it.
	Contract []byte
	// Authenticator replaces FakeAuthenticator{}.
	Authenticator actor.Authenticator
	// Stages are installed with Chain.Use before the routes are mounted (the slots of later
	// tickets).
	Stages map[httpserver.Stage]func(http.Handler) http.Handler
	// AllowNullRepair keeps the test green when a handler sent JSON null; only the tests of the
	// repair itself set it.
	AllowNullRepair bool
}

// Harness is a running chain with its observation points.
type Harness struct {
	t      testing.TB
	Chain  *httpserver.Chain
	Server *httptest.Server
	Reader *sdkmetric.ManualReader
	Logs   *LogCapture
}

var (
	spansOnce sync.Once
	spans     *tracetest.SpanRecorder
)

// Spans returns the recorder of every server span the harnesses of this test binary started. The
// harness installs one global tracer provider that samples everything, because the server span
// comes from platform/otel's middleware, which reads the global provider.
func Spans() *tracetest.SpanRecorder {
	spansOnce.Do(func() {
		spans = tracetest.NewSpanRecorder()
		otelapi.SetTracerProvider(sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithSpanProcessor(spans)))
	})
	return spans
}

// New builds and starts the chain; the server stops when the test ends. It fails the test when
// the chain cannot be built, and at the end when a handler sent JSON null (unless
// AllowNullRepair).
func New(t testing.TB, opts Options) *Harness {
	t.Helper()
	Spans()
	cfg := httpserver.DefaultChainConfig(PublicURL)
	if opts.Config != nil {
		cfg = *opts.Config
	}
	contract := opts.Contract
	if contract == nil {
		contract = Contract
	}
	idList := make([]string, 0, len(opts.Routes))
	for _, rt := range opts.Routes {
		idList = append(idList, rt.Op.ID)
	}
	v, err := httpserver.NewValidator(contract, idList)
	if err != nil {
		t.Fatalf("httpservertest: validator: %v", err)
	}
	auth := opts.Authenticator
	if auth == nil {
		auth = FakeAuthenticator{}
	}
	reader := sdkmetric.NewManualReader()
	logs := &LogCapture{}
	chain, err := httpserver.NewChain(cfg, httpserver.Deps{
		Logger:        slog.New(logs),
		MeterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)),
		Authenticator: auth,
		Validator:     v,
	})
	if err != nil {
		t.Fatalf("httpservertest: chain: %v", err)
	}
	for stage, mw := range opts.Stages {
		chain.Use(stage, mw)
	}
	mux := http.NewServeMux()
	for _, rt := range opts.Routes {
		mux.Handle(rt.Op.Pattern, chain.Wrap(rt.Op, rt.Handler))
	}
	srv := httptest.NewServer(chain.Public(mux))
	h := &Harness{t: t, Chain: chain, Server: srv, Reader: reader, Logs: logs}
	t.Cleanup(func() {
		srv.Close()
		if !opts.AllowNullRepair && logs.Has(httpserver.LogNullRepaired) {
			t.Errorf("httpservertest: a handler sent JSON null (%q in the log); the server never sends null (ADR 0053)", httpserver.LogNullRepaired)
		}
	})
	return h
}

// Response is a finished response with its body read.
type Response struct {
	*http.Response
	Body []byte
}

// Problem decodes the body as a problem; it fails the test when the body is not one.
func (r *Response) Problem(t testing.TB) map[string]any {
	t.Helper()
	if ct := r.Header.Get(headerContentType); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json; body %s", ct, r.Body)
	}
	var p map[string]any
	if err := json.Unmarshal(r.Body, &p); err != nil {
		t.Fatalf("problem body: %v: %s", err, r.Body)
	}
	return p
}

// NewRequest builds a request to the harness: method, path (with query), a body ("" for none,
// sent as application/json unless headers set Content-Type) and header name/value pairs.
func (h *Harness) NewRequest(method, path, body string, headers ...string) *http.Request {
	h.t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, h.Server.URL+path, rd)
	if err != nil {
		h.t.Fatalf("httpservertest: request: %v", err)
	}
	if body != "" {
		req.Header.Set(headerContentType, "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	return req
}

// Do sends req and returns the response with its body.
func (h *Harness) Do(req *http.Request) *Response {
	h.t.Helper()
	resp, err := h.Server.Client().Do(req) //nolint:gosec // G704: the request goes to the harness's own test server on 127.0.0.1
	if err != nil {
		h.t.Fatalf("httpservertest: %s %s: %v", req.Method, req.URL.Path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		h.t.Fatalf("httpservertest: body: %v", err)
	}
	return &Response{Response: resp, Body: body}
}

// Counter returns the sum of the int64 counter name over the points whose attributes contain
// every attribute of want, read through the manual reader.
func (h *Harness) Counter(name string, want ...attribute.KeyValue) int64 {
	h.t.Helper()
	var rm metricdata.ResourceMetrics
	if err := h.Reader.Collect(context.Background(), &rm); err != nil {
		h.t.Fatalf("httpservertest: collect: %v", err)
	}
	var total int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			sum, ok := m.Data.(metricdata.Sum[int64])
			if m.Name != name || !ok {
				continue
			}
			for _, dp := range sum.DataPoints {
				if hasAll(dp.Attributes, want) {
					total += dp.Value
				}
			}
		}
	}
	return total
}

func hasAll(set attribute.Set, want []attribute.KeyValue) bool {
	for _, kv := range want {
		if v, ok := set.Value(kv.Key); !ok || v != kv.Value {
			return false
		}
	}
	return true
}

// Op returns a test operation: Pattern is "<method> <path>", the body kind follows the method
// (JSON for POST, PUT and PATCH, none otherwise) and the body is required when there is one.
func Op(id, method, path, action string) httpserver.Operation {
	op := httpserver.Operation{ID: id, Method: method, Path: path, Pattern: method + " " + path, Action: action}
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		op.RequestBody, op.BodyRequired = httpserver.BodyJSON, true
	}
	return op
}

// RecordCheck records an allowed Authorize check of action in the request's check log, as an
// Authorizer would.
func RecordCheck(r *http.Request, action string) {
	authorize.RecordCheck(r.Context(), authorize.Check{Action: action, Kind: authorize.CheckAuthorize, Allowed: true})
}

// Authorized returns next with an allowed check of action recorded first.
func Authorized(action string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		RecordCheck(r, action)
		next(w, r)
	}
}

// JSON writes v as application/json with status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set(headerContentType, "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// GoodToken is the credential FakeAuthenticator accepts: "Authorization: Bearer good".
const GoodToken = "Bearer good"

// FakeUserID is the principal FakeAuthenticator returns for GoodToken.
var FakeUserID = ids.MustParse("0192f4a0-7b1c-7577-8bf0-c4f4ee4f887e")

// FakeAuthenticator accepts GoodToken as a user, treats requests without Authorization as
// anonymous and answers 401 unauthenticated for anything else. LastCredentials keeps the
// credentials of the last call when set.
type FakeAuthenticator struct {
	LastCredentials *actor.Credentials
}

// Authenticate implements actor.Authenticator.
func (f FakeAuthenticator) Authenticate(_ context.Context, c actor.Credentials) (actor.Actor, error) {
	if f.LastCredentials != nil {
		*f.LastCredentials = c
	}
	switch c.Authorization {
	case "":
		return actor.Anonymous(), nil
	case GoodToken:
		return actor.Actor{ID: FakeUserID, Kind: actor.KindUser}, nil
	}
	return actor.Actor{}, errs.New(errs.CodeUnauthenticated, "").WithHeader("WWW-Authenticate", "Bearer")
}

// Revalidate implements actor.Authenticator.
func (FakeAuthenticator) Revalidate(context.Context, actor.Actor) error { return nil }

// Record is one captured log record with its attributes as text.
type Record struct {
	Level   slog.Level
	Message string
	Attrs   map[string]string
}

// LogCapture is a slog.Handler that keeps every record; safe for concurrent use.
type LogCapture struct {
	mu      sync.Mutex
	records []Record
	attrs   []slog.Attr
}

// Enabled reports true for every level.
func (*LogCapture) Enabled(context.Context, slog.Level) bool { return true }

// Handle keeps the record.
func (l *LogCapture) Handle(_ context.Context, r slog.Record) error {
	rec := Record{Level: r.Level, Message: r.Message, Attrs: map[string]string{}}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, a := range l.attrs {
		rec.Attrs[a.Key] = a.Value.String()
	}
	r.Attrs(func(a slog.Attr) bool {
		rec.Attrs[a.Key] = a.Value.String()
		return true
	})
	l.records = append(l.records, rec)
	return nil
}

// WithAttrs returns the same capture with attrs added to later records; the capture is shared.
func (l *LogCapture) WithAttrs(attrs []slog.Attr) slog.Handler {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.attrs = append(l.attrs, attrs...)
	return l
}

// WithGroup returns the capture unchanged; the chain logs no groups.
func (l *LogCapture) WithGroup(string) slog.Handler { return l }

// Records returns a copy of the captured records.
func (l *LogCapture) Records() []Record {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Record(nil), l.records...)
}

// Has reports whether a record with the message was captured.
func (l *LogCapture) Has(message string) bool {
	for _, r := range l.Records() {
		if r.Message == message {
			return true
		}
	}
	return false
}

// Find returns the first record with the message and true, or false.
func (l *LogCapture) Find(message string) (Record, bool) {
	for _, r := range l.Records() {
		if r.Message == message {
			return r, true
		}
	}
	return Record{}, false
}
