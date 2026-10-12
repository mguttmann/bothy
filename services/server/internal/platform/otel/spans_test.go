// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// testSpanAllowlist: span, event and link attributes keep only allowed keys with catalog values;
// the status description is cleared; a server span loses server.address and server.port, a
// client span keeps them.
func testSpanAllowlist(t *testing.T) {
	t.Parallel()
	mem := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(filterSpanExporter{next: mem}))
	tracer := tp.Tracer("test")
	link := trace.Link{SpanContext: incoming, Attributes: []attribute.KeyValue{attribute.String("x", "canary-link")}}

	_, s := tracer.Start(context.Background(), "GET /a/{id}", trace.WithSpanKind(trace.SpanKindServer), trace.WithLinks(link))
	s.SetAttributes(
		attribute.String("url.path", "/a/canary-path"),
		attribute.String("url.query", "token=canary-query"),
		attribute.String(attr.HTTPRoute, "/a/{id}"),
		attribute.String(attr.ServerAddress, "canary-host.example.com"),
		attribute.Int(attr.ServerPort, 8443),
		attribute.ByteSlice(attr.Reason, []byte("canary-bytes")),
	)
	s.AddEvent("exception", trace.WithAttributes(
		attribute.String("exception.message", "canary-message"),
		attribute.String(attr.ExceptionType, "*errs.Error"),
	))
	s.SetStatus(codes.Error, "canary-description")
	s.End()

	_, c := tracer.Start(context.Background(), "webhook.deliver", trace.WithSpanKind(trace.SpanKindClient))
	c.SetAttributes(attribute.String(attr.ServerAddress, "hooks.example.com"))
	c.End()

	spans := mem.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("got %d spans, want 2", len(spans))
	}
	if out := dumpSpan(spans[0]); strings.Contains(out, "canary") {
		t.Errorf("server span keeps a dropped value: %s", out)
	}
	if len(spans[0].Attributes) != 1 || spans[0].Attributes[0].Key != attr.HTTPRoute {
		t.Errorf("server span attributes = %v, want only http.route", spans[0].Attributes)
	}
	ev := spans[0].Events
	if len(ev) != 1 || len(ev[0].Attributes) != 1 || ev[0].Attributes[0].Key != attr.ExceptionType {
		t.Errorf("events = %v, want exception.type only", ev)
	}
	if spans[0].Status.Code != codes.Error || spans[0].Status.Description != "" {
		t.Errorf("status = %v, want Error without description", spans[0].Status)
	}
	if len(spans[0].Links) != 1 || len(spans[0].Links[0].Attributes) != 0 || spans[0].Links[0].SpanContext.TraceID() != incoming.TraceID() {
		t.Errorf("links = %v, want the link without attributes", spans[0].Links)
	}
	if v, ok := spanAttr(spans[1], attr.ServerAddress); !ok || v.AsString() != "hooks.example.com" {
		t.Errorf("client span server.address = %v, want kept", v)
	}
}
