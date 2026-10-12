// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// TestSubscriberSpan: one INTERNAL span per call with the given name, bothy.outcome ok or veto,
// a child of the caller's span, and the subscriber's context carries the new span.
func TestSubscriberSpan(t *testing.T) {
	t.Parallel()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	start := subscriberSpan(func() trace.TracerProvider { return tp })
	parentCtx, parent := tp.Tracer("test").Start(context.Background(), "parent")

	ctx, end := start(parentCtx, "txn.subscribe runs")
	if trace.SpanContextFromContext(ctx).SpanID() == parent.SpanContext().SpanID() {
		t.Fatal("the subscriber context does not carry its own span")
	}
	end(nil)
	_, end = start(parentCtx, "txn.subscribe chat")
	end(errors.New("veto"))
	parent.End()

	spans := rec.Ended()
	if len(spans) != 3 {
		t.Fatalf("%d spans, want 3", len(spans))
	}
	for i, want := range []struct{ name, outcome string }{{"txn.subscribe runs", "ok"}, {"txn.subscribe chat", "veto"}} {
		s := spans[i]
		if s.Name() != want.name || s.SpanKind() != trace.SpanKindInternal || s.Parent().SpanID() != parent.SpanContext().SpanID() {
			t.Errorf("span %d = %s kind %s parent %s", i, s.Name(), s.SpanKind(), s.Parent().SpanID())
		}
		if got := s.Attributes(); len(got) != 1 || got[0] != attribute.String(attr.Outcome, want.outcome) {
			t.Errorf("span %s attributes = %v, want only %s=%s", s.Name(), got, attr.Outcome, want.outcome)
		}
	}
}

// TestSubscriberSpanUsesGlobalProvider: the exported tracer reads the global provider at each
// call, so it works when internal/wire sets it before otel.Setup installed the provider.
func TestSubscriberSpanUsesGlobalProvider(t *testing.T) {
	t.Parallel()
	ctx, end := SubscriberSpan(context.Background(), "txn.subscribe runs")
	if ctx == nil {
		t.Fatal("nil context")
	}
	end(nil)
}
