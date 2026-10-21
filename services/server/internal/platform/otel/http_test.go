// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/trace"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// hookPattern is a route with a secret in its path (a webhook token).
const hookPattern = "/api/v1/hooks/{webhook_id}/{token}"

// incoming is the context of a client that sends a sampled traceparent.
var incoming = trace.NewSpanContext(trace.SpanContextConfig{
	TraceID:    trace.TraceID{0xa1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
	SpanID:     trace.SpanID{0xb1, 2, 3, 4, 5, 6, 7, 8},
	TraceFlags: trace.FlagsSampled,
	Remote:     true,
})

func traceparent(sc trace.SpanContext) string {
	return "00-" + sc.TraceID().String() + "-" + sc.SpanID().String() + "-" + sc.TraceFlags().String()
}

// testMux routes hookPattern; its handler records the route and the span context it sees.
func testMux(seen *trace.SpanContext, status int) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+hookPattern, func(w http.ResponseWriter, r *http.Request) {
		RecordRoute(r, r.Pattern)
		if seen != nil {
			*seen = trace.SpanContextFromContext(r.Context())
		}
		w.WriteHeader(status)
	})
	return mux
}

func serve(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func hookRequest(t *testing.T, sc trace.SpanContext) *http.Request {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/hooks/w-1/t-1", nil)
	if sc.IsValid() {
		r.Header.Set("traceparent", traceparent(sc))
	}
	return r
}

// TestRemoteParentIsLinkByDefault: an incoming traceparent is a span link; trace id and sampling
// come from the server (ratio 0: not sampled although the incoming flag is sampled).
func TestRemoteParentIsLinkByDefault(t *testing.T) {
	tt := setupTest(t, testConfig(), nil, nil, nil)
	var seen trace.SpanContext
	serve(Middleware(testMux(&seen, http.StatusOK)), hookRequest(t, incoming))
	tt.flush(t)
	spans := tt.spans.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	s := spans[0]
	if s.SpanContext.TraceID() == incoming.TraceID() || seen.TraceID() != s.SpanContext.TraceID() {
		t.Errorf("trace id %s, incoming %s, handler %s: want a new trace", s.SpanContext.TraceID(), incoming.TraceID(), seen.TraceID())
	}
	if s.Parent.IsValid() {
		t.Errorf("parent = %v, want a new root", s.Parent)
	}
	if len(s.Links) != 1 || s.Links[0].SpanContext.TraceID() != incoming.TraceID() || s.Links[0].SpanContext.SpanID() != incoming.SpanID() {
		t.Errorf("links = %v, want one link to the incoming span", s.Links)
	}
	if s.Name != "POST "+hookPattern {
		t.Errorf("span name = %q", s.Name)
	}
	if err := tt.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	cfg := testConfig()
	cfg.SamplerRatio = 0
	tt = setupTest(t, cfg, nil, nil, nil)
	serve(Middleware(testMux(&seen, http.StatusOK)), hookRequest(t, incoming))
	tt.flush(t)
	if seen.IsSampled() || len(tt.spans.GetSpans()) != 0 {
		t.Errorf("ratio 0: sampled %v, %d spans; want not sampled despite the incoming flag", seen.IsSampled(), len(tt.spans.GetSpans()))
	}
}

// TestTrustRemoteParent: with telemetry.traces.trust_remote_parent the incoming context is the
// parent and its sampled flag decides, whatever the ratio.
func TestTrustRemoteParent(t *testing.T) {
	notSampled := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: incoming.TraceID(), SpanID: incoming.SpanID(), Remote: true,
	})
	for _, tc := range []struct {
		ratio       float64
		in          trace.SpanContext
		wantSampled bool
	}{
		{ratio: 0, in: incoming, wantSampled: true},
		{ratio: 1, in: notSampled, wantSampled: false},
	} {
		cfg := testConfig()
		cfg.TrustRemoteParent = true
		cfg.SamplerRatio = tc.ratio
		tt := setupTest(t, cfg, nil, nil, nil)
		var seen trace.SpanContext
		serve(Middleware(testMux(&seen, http.StatusOK)), hookRequest(t, tc.in))
		tt.flush(t)
		if seen.TraceID() != incoming.TraceID() || seen.IsSampled() != tc.wantSampled {
			t.Errorf("ratio %v, incoming sampled %v: trace %s sampled %v; want the incoming trace, sampled %v",
				tc.ratio, tc.in.IsSampled(), seen.TraceID(), seen.IsSampled(), tc.wantSampled)
		}
		if spans := tt.spans.GetSpans(); tc.wantSampled && (len(spans) != 1 || spans[0].Parent.SpanID() != incoming.SpanID()) {
			t.Errorf("spans = %v, want one span with the incoming parent", spans)
		}
		if err := tt.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}

// TestEveryRequestHasCorrelationID: every request has a trace id as UUID text, also with
// telemetry.enabled = false, and the access log carries it.
func TestEveryRequestHasCorrelationID(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		cfg := testConfig()
		cfg.Enabled = enabled
		cfg.LogsExporter = logsStdout
		tt := setupTest(t, cfg, nil, nil, nil)
		var got string
		var sc trace.SpanContext
		h := Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			got = CorrelationID(r.Context())
			sc = trace.SpanContextFromContext(r.Context())
		}))
		serve(h, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
		if got == "" || got != uuidText(sc.TraceID()) || len(got) != 36 {
			t.Errorf("enabled %v: correlation id %q for trace %s", enabled, got, sc.TraceID())
		}
		recs := decodeLines(t, tt.stdout.String())
		if len(recs) != 1 || recs[0][attr.CorrelationID] != got {
			t.Errorf("enabled %v: access log %v, want correlation id %q", enabled, recs, got)
		}
		if err := tt.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}

// TestLogsNeverContainSecretsHeadersOrRawPaths holds SEC-LOG-2 and SEC-LOG-3: path values,
// the query, Authorization, DPoP, Cookie, X-Forwarded-For, the Host and a custom header reach
// neither stdout nor the exported log records nor the exported spans; http.route is the pattern
// and an unrouted request logs "unmatched".
func TestLogsNeverContainSecretsHeadersOrRawPaths(t *testing.T) {
	tt := setupTest(t, testConfig(), nil, nil, nil)
	h := Middleware(testMux(nil, http.StatusAccepted))

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/hooks/canary-webhook/canary-token?token=canary-query", nil)
	r.Host = "canary-host.example.com:8443"
	r.RemoteAddr = "192.0.2.10:40000"
	r.Header.Set("Authorization", "Bearer canary-authorization")
	r.Header.Set("DPoP", "canary-dpop")
	r.Header.Set("Cookie", "session=canary-cookie")
	r.Header.Set("X-Forwarded-For", "canary-forwarded")
	r.Header.Set("X-Custom", "canary-custom")
	r.Header.Set("User-Agent", "test-agent/1.0")
	r.Header.Set("traceparent", traceparent(incoming))
	r.Header.Set("tracestate", "vendor=canary-tracestate")
	if w := serve(h, r); w.Code != http.StatusAccepted {
		t.Fatalf("status = %d", w.Code)
	}
	serve(h, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/canary-raw-path/x?q=canary-q", nil))
	tt.flush(t)

	stdout := tt.stdout.String()
	var exported, spanText strings.Builder
	records := tt.logs.get()
	for i := range records {
		exported.WriteString(dumpRecord(&records[i]))
	}
	spans := tt.spans.GetSpans()
	for _, s := range spans {
		spanText.WriteString(dumpSpan(s))
	}
	for name, out := range map[string]string{"stdout": stdout, "log records": exported.String(), "spans": spanText.String()} {
		if strings.Contains(out, "canary") || strings.Contains(out, "192.0.2.10") {
			t.Errorf("%s contain a canary:\n%s", name, out)
		}
	}

	recs := decodeLines(t, stdout)
	if len(recs) != 2 || len(records) != 2 || len(spans) != 2 {
		t.Fatalf("got %d stdout lines, %d log records, %d spans; want 2 each", len(recs), len(records), len(spans))
	}
	if recs[0][attr.HTTPRoute] != hookPattern || recs[0][attr.UserAgentOriginal] != "test-agent/1.0" ||
		recs[0][attr.HTTPResponseStatusCode] != float64(http.StatusAccepted) || recs[0]["msg"] != msgHTTPRequest {
		t.Errorf("access log = %v", recs[0])
	}
	if recs[1][attr.HTTPRoute] != routeUnmatched {
		t.Errorf("unrouted access log = %v, want http.route unmatched", recs[1])
	}
	if v, _ := recordAttr(&records[0], attr.HTTPRoute); v.AsString() != hookPattern {
		t.Errorf("exported record http.route = %q", v.AsString())
	}
	if v, _ := spanAttr(spans[0], attr.HTTPRoute); v.AsString() != hookPattern {
		t.Errorf("span http.route = %q", v.AsString())
	}
	if v, _ := spanAttr(spans[1], attr.HTTPRoute); v.AsString() != routeUnmatched || spans[1].Name != "GET "+routeUnmatched {
		t.Errorf("unrouted span %q http.route = %q", spans[1].Name, v.AsString())
	}
}

// TestUnknownMethodNormalized: a method outside the standard set opens no new span name, log
// value or metric series.
func TestUnknownMethodNormalized(t *testing.T) {
	tt := setupTest(t, testConfig(), nil, nil, nil)
	serve(Middleware(http.NotFoundHandler()), httptest.NewRequestWithContext(t.Context(), "FOO", "/x", nil))
	tt.flush(t)
	if spans := tt.spans.GetSpans(); len(spans) != 1 || spans[0].Name != "HTTP unmatched" {
		t.Errorf("spans = %v, want one span named %q", spans, "HTTP unmatched")
	}
	if recs := decodeLines(t, tt.stdout.String()); len(recs) != 1 || recs[0][attr.HTTPRequestMethod] != methodOther {
		t.Errorf("access log = %v, want method %s", recs, methodOther)
	}
	for _, dp := range histogramPoints(t, tt) {
		if v, _ := dp.Attributes.Value(attribute.Key(attr.HTTPRequestMethod)); v.AsString() != methodOther {
			t.Errorf("metric method = %q, want %s", v.AsString(), methodOther)
		}
	}
}

func histogramPoints(t *testing.T, tt *testTelemetry) []metricdata.HistogramDataPoint[float64] {
	t.Helper()
	m := findMetric(t, tt.reader, metrics.HTTPServerRequestDuration.Name)
	if m == nil {
		t.Fatalf("%s missing", metrics.HTTPServerRequestDuration.Name)
	}
	h, ok := m.Data.(metricdata.Histogram[float64])
	if !ok {
		t.Fatalf("%s is %T", metrics.HTTPServerRequestDuration.Name, m.Data)
	}
	return h.DataPoints
}

// TestHTTPMetric_OnlyAllowedAttributes: http.server.request.duration carries only the four
// catalog attributes (a request with an unusual Host header leaves no server.address), the
// boundaries of signals.md 4.7 and no exemplars; the body size histograms of otelhttp are dropped.
func TestHTTPMetric_OnlyAllowedAttributes(t *testing.T) {
	tt := setupTest(t, testConfig(), nil, nil, nil)
	h := Middleware(testMux(nil, http.StatusBadGateway))
	r := hookRequest(t, trace.SpanContext{})
	r.Host = "canary-host.example.com:8443"
	serve(h, r)

	points := histogramPoints(t, tt)
	if len(points) != 1 {
		t.Fatalf("got %d data points, want 1", len(points))
	}
	dp := points[0]
	want := map[string]string{
		attr.HTTPRequestMethod: http.MethodPost, attr.HTTPRoute: hookPattern,
		attr.HTTPResponseStatusCode: "502", attr.ErrorType: "502",
	}
	if dp.Attributes.Len() != len(want) {
		t.Errorf("attributes = %v, want exactly %v", dp.Attributes.ToSlice(), want)
	}
	for k, v := range want {
		if got, _ := dp.Attributes.Value(attribute.Key(k)); got.String() != v {
			t.Errorf("%s = %q, want %q", k, got.String(), v)
		}
	}
	if len(dp.Exemplars) != 0 {
		t.Errorf("exemplars = %v, want none", dp.Exemplars)
	}
	if !slices.Equal(dp.Bounds, metrics.BucketsSeconds) {
		t.Errorf("bounds = %v, want %v", dp.Bounds, metrics.BucketsSeconds)
	}
	for _, name := range []string{"http.server.request.body.size", "http.server.response.body.size"} {
		if findMetric(t, tt.reader, name) != nil {
			t.Errorf("%s is exported, want dropped", name)
		}
	}
}

// TestCollectorUnreachableDoesNotDelayRequests: the OTLP endpoint accepts connections and never
// answers. Requests stay fast, and the spans that did not fit or whose export failed are
// counted.
func TestCollectorUnreachableDoesNotDelayRequests(t *testing.T) {
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		var conns []net.Conn
		defer func() {
			for _, c := range conns {
				_ = c.Close()
			}
		}()
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			conns = append(conns, c) // held open, never read, never answered
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://"+ln.Addr().String())
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	// A request that waited for the exporter would take this long (milliseconds, twice
	// testkit.Patience); the bound below is testkit.Patience, far above the time a request needs
	// on a loaded host.
	t.Setenv("OTEL_EXPORTER_OTLP_TIMEOUT", strconv.FormatInt((2*testkit.Patience).Milliseconds(), 10))

	tt := &testTelemetry{stdout: &syncBuffer{}}
	reader := newManualReader()
	tel, err := setup(context.Background(), testConfig(), Deps{
		Secrets: testSecrets(t, testKey),
		Stdout:  tt.stdout,
		Env:     []string{"OTEL_BSP_MAX_QUEUE_SIZE=8", "OTEL_BLRP_MAX_QUEUE_SIZE=8", "OTEL_METRICS_EXPORTER=none"},
	}, setupOptions{metricReader: reader})
	if err != nil {
		t.Fatal(err)
	}
	tt.Telemetry, tt.reader = tel, reader
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second) // nosemgrep: bothy-go-test-wall-clock-wait -- a bounded give-up; the exporter never answers
		defer cancel()
		_ = tel.Shutdown(ctx)
	})

	const requests, bound = 200, testkit.Patience
	h := Middleware(testMux(nil, http.StatusOK))
	for i := range requests {
		start := time.Now()
		serve(h, hookRequest(t, trace.SpanContext{}))
		if d := time.Since(start); d > bound {
			t.Fatalf("request %d took %v, bound %v", i, d, bound)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second) // nosemgrep: bothy-go-test-wall-clock-wait -- a bounded give-up; the exporter never answers
	defer cancel()
	_ = tel.ForceFlush(ctx)
	got := tt.dropped(t)
	if got[signalTraces] == 0 || got[signalLogs] == 0 {
		t.Errorf("dropped = %v, want traces and logs above zero", got)
	}
}

// TestHTTPMetric_RouteTemplateAndUnmatched: http.route of the metric is the route template for a
// routed request, never the raw path, and "unmatched" for a request no route matched.
func TestHTTPMetric_RouteTemplateAndUnmatched(t *testing.T) {
	tt := setupTest(t, testConfig(), nil, nil, nil)
	h := Middleware(testMux(nil, http.StatusOK))
	serve(h, hookRequest(t, trace.SpanContext{}))
	serve(h, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/no/such/route/secret-token", nil))

	routes := map[string]bool{}
	for _, dp := range histogramPoints(t, tt) {
		v, _ := dp.Attributes.Value(attribute.Key(attr.HTTPRoute))
		routes[v.AsString()] = true
	}
	if len(routes) != 2 || !routes[hookPattern] || !routes["unmatched"] {
		t.Errorf("http.route values = %v, want exactly %q and unmatched", routes, hookPattern)
	}
}

// TestAnnotatePrincipal (DP-T09, signals.md "Spans" row of the server span): the request span
// carries bothy.principal.kind and, for a person, bothy.principal.pseudo_id; never the clear ID,
// and an agent gets no pseudonym.
func TestAnnotatePrincipal(t *testing.T) {
	tt := setupTest(t, testConfig(), nil, nil, nil)
	for _, c := range []struct {
		kind       string
		wantPseudo bool
	}{{"user", true}, {"agent", false}, {"anonymous", false}} {
		ctx, span := otelapi.Tracer("probe").Start(context.Background(), "probe "+c.kind)
		AnnotatePrincipal(ctx, c.kind, principal)
		span.End()
		tt.flush(t)
		spans := tt.spans.GetSpans()
		got := map[string]string{}
		for _, kv := range spans[len(spans)-1].Attributes {
			got[string(kv.Key)] = kv.Value.String()
		}
		if got[attr.PrincipalKind] != c.kind {
			t.Errorf("%s: kind %q", c.kind, got[attr.PrincipalKind])
		}
		pseudo, has := got[attr.PrincipalPseudoID]
		if has != c.wantPseudo || has && (len(pseudo) != 32 || strings.Contains(pseudo, principal)) {
			t.Errorf("%s: pseudo_id %q present=%v", c.kind, pseudo, has)
		}
		for k, v := range got {
			if strings.Contains(v, principal) {
				t.Errorf("%s: clear principal ID in %s", c.kind, k)
			}
		}
	}
}
