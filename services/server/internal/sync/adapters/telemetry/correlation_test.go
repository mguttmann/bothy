// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/telemetry"
)

func TestCorrelation_FromTraceContext(t *testing.T) {
	t.Parallel()
	c := telemetry.Correlator{}
	for _, tc := range []struct {
		name    string
		sampler sdktrace.Sampler
		sampled bool
	}{
		{"sampled", sdktrace.AlwaysSample(), true},
		{"unsampled", sdktrace.NeverSample(), false},
	} {
		tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(tc.sampler))
		ctx, span := tp.Tracer("test").Start(context.Background(), "request")
		sc := span.SpanContext()
		id, trace := c.Correlation(ctx)
		span.End()
		if id != ids.ID(sc.TraceID()) {
			t.Errorf("%s: correlation_id = %v, want the trace ID %s", tc.name, id, sc.TraceID())
		}
		want := ""
		if tc.sampled {
			want = "00-" + sc.TraceID().String() + "-" + sc.SpanID().String() + "-01"
		}
		if trace != want {
			t.Errorf("%s: trace_context = %q, want %q", tc.name, trace, want)
		}
		_ = tp.Shutdown(context.Background())
	}
	if id, trace := c.Correlation(context.Background()); !id.IsZero() || trace != "" {
		t.Errorf("no span: %v %q, want neither", id, trace)
	}
}

// TestCorrelation_FallsBackToTheRequestID: without a valid span context the correlation_id of the
// request (kernel/reqmeta, a random UUID when no tracer issued a trace ID) keeps event, response
// and audit entry together (03 event_log.correlation_id); trace_context stays empty.
func TestCorrelation_FallsBackToTheRequestID(t *testing.T) {
	t.Parallel()
	ctx := reqmeta.WithCorrelationID(context.Background(), "4bf92f35-77b3-4da6-a3ce-929d0e0e4736")
	id, trace := telemetry.Correlator{}.Correlation(ctx)
	if id.String() != "4bf92f35-77b3-4da6-a3ce-929d0e0e4736" || trace != "" {
		t.Fatalf("fallback = %v %q", id, trace)
	}
	bad := reqmeta.WithCorrelationID(context.Background(), "not-a-uuid")
	if id, _ := (telemetry.Correlator{}).Correlation(bad); !id.IsZero() {
		t.Fatalf("an invalid request ID became %v", id)
	}
}
