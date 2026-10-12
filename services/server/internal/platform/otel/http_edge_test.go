// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// TestTraceStateNeverExported: the tracestate header is free text of the client. Exported spans
// and their links carry no trace state, with and without trust in the remote parent.
func TestTraceStateNeverExported(t *testing.T) {
	for _, trust := range []bool{false, true} {
		cfg := testConfig()
		cfg.TrustRemoteParent = trust
		tt := setupTest(t, cfg, nil, nil, nil)
		r := hookRequest(t, incoming)
		r.Header.Set("tracestate", "vendor=canary-tracestate")
		serve(Middleware(testMux(nil, http.StatusOK)), r)
		tt.flush(t)
		spans := tt.spans.GetSpans()
		if len(spans) != 1 {
			t.Fatalf("trust %v: got %d spans, want 1", trust, len(spans))
		}
		if out := dumpSpan(spans[0]); strings.Contains(out, "canary") {
			t.Errorf("trust %v: exported span carries the client trace state: %s", trust, out)
		}
		if trust && spans[0].SpanContext.TraceID() != incoming.TraceID() {
			t.Errorf("trust %v: trace id %s, want the incoming one", trust, spans[0].SpanContext.TraceID())
		}
	}
}

// TestRecordRouteWinsOverOuterPattern: below an outer ServeMux that only knows a prefix pattern
// and a request copy (http.StripPrefix), the route RecordRoute names is the http.route of the
// span, the access log and the duration metric alike.
func TestRecordRouteWinsOverOuterPattern(t *testing.T) {
	tt := setupTest(t, testConfig(), nil, nil, nil)
	inner := http.NewServeMux()
	inner.HandleFunc("POST /v1/hooks/{webhook_id}/{token}", func(w http.ResponseWriter, r *http.Request) {
		RecordRoute(r, "POST "+hookPattern)
		w.WriteHeader(http.StatusNoContent)
	})
	outer := http.NewServeMux()
	outer.Handle("/api/", http.StripPrefix("/api", inner))
	serve(Middleware(outer), hookRequest(t, trace.SpanContext{}))
	tt.flush(t)

	points := histogramPoints(t, tt)
	if len(points) != 1 {
		t.Fatalf("got %d data points, want 1", len(points))
	}
	if v, _ := points[0].Attributes.Value(attribute.Key(attr.HTTPRoute)); v.AsString() != hookPattern {
		t.Errorf("metric http.route = %q, want %q", v.AsString(), hookPattern)
	}
	spans := tt.spans.GetSpans()
	if len(spans) != 1 || spans[0].Name != "POST "+hookPattern {
		t.Fatalf("spans = %v, want one named POST %s", spans, hookPattern)
	}
	if v, _ := spanAttr(spans[0], attr.HTTPRoute); v.AsString() != hookPattern {
		t.Errorf("span http.route = %q", v.AsString())
	}
	if recs := decodeLines(t, tt.stdout.String()); len(recs) != 1 || recs[0][attr.HTTPRoute] != hookPattern {
		t.Errorf("access log = %v, want http.route %s", recs, hookPattern)
	}
}

// TestPanicKeepsAccessLog: a handler panic passes on to the recovery layer outside (02 "Chain in
// the api node" step 1) with its value unchanged; the access log still has one record with status
// 500 and error.type, and the span carries both.
func TestPanicKeepsAccessLog(t *testing.T) {
	tt := setupTest(t, testConfig(), nil, nil, nil)
	errBoom := errors.New("canary-panic-value")
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+hookPattern, panickingHandler(errBoom))
	h := Middleware(mux)
	var recovered any
	var stack []byte
	func() {
		defer func() { recovered, stack = recover(), debug.Stack() }()
		h.ServeHTTP(httptest.NewRecorder(), hookRequest(t, trace.SpanContext{}))
	}()
	if err, ok := recovered.(error); !ok || !errors.Is(err, errBoom) {
		t.Fatalf("recovered %v, want the panic value of the handler", recovered)
	}
	// The recovery layer still sees the frame that panicked.
	if !strings.Contains(string(stack), "panickingHandler") {
		t.Errorf("stack at the recovery layer lost the panicking frame:\n%s", stack)
	}
	tt.flush(t)

	recs := decodeLines(t, tt.stdout.String())
	if len(recs) != 1 {
		t.Fatalf("got %d access log records, want 1:\n%s", len(recs), tt.stdout.String())
	}
	if recs[0][attr.HTTPResponseStatusCode] != float64(http.StatusInternalServerError) ||
		recs[0][attr.ErrorType] != "500" || recs[0][attr.HTTPRoute] != hookPattern {
		t.Errorf("access log = %v, want status 500, error.type 500, route %s", recs[0], hookPattern)
	}
	spans := tt.spans.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	if v, _ := spanAttr(spans[0], attr.HTTPResponseStatusCode); v.AsInt64() != http.StatusInternalServerError {
		t.Errorf("span http.response.status_code = %v", v)
	}
	if v, _ := spanAttr(spans[0], attr.ErrorType); v.AsString() != "500" {
		t.Errorf("span error.type = %q", v.AsString())
	}
	if out := tt.stdout.String() + dumpSpan(spans[0]); strings.Contains(out, "canary") {
		t.Errorf("the panic value reached telemetry: %s", out)
	}
}

// panickingHandler returns a handler that panics with v; its name shows in a stack trace.
func panickingHandler(v any) http.HandlerFunc {
	return func(http.ResponseWriter, *http.Request) {
		panic(v)
	}
}
