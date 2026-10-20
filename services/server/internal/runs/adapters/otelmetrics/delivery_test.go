// SPDX-License-Identifier: Apache-2.0

package otelmetrics_test

import (
	"context"
	"slices"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestDeliveryMetrics: bothy.agent.invocation.denied carries only bothy.agent.handle and
// bothy.reason, its values are the reasons of M2-08 design note 6, and agent.dispatch is a
// PRODUCER span with run, handle and decision.
func TestDeliveryMetrics(t *testing.T) {
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	d, err := otelmetrics.NewDelivery(mp, tp)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range domain.DeliveryReasons() {
		d.Denied(context.Background(), "lotse", r)
	}
	m := metricstest.Find(t, reader, "bothy.agent.invocation.denied")
	ps := metricstest.Points(m)
	if len(ps) != len(domain.DeliveryReasons()) {
		t.Fatalf("points %v", ps)
	}
	for _, p := range ps {
		if len(p.Attrs) != 2 || p.Attrs["bothy.agent.handle"] != "lotse" || !slices.Contains(domain.DeliveryReasons(), p.Attrs["bothy.reason"]) || p.Value != 1 {
			t.Fatalf("point %+v", p)
		}
	}
	var declared []string
	for _, a := range metrics.AgentInvocationDenied.Attrs {
		if a.Key == "bothy.reason" {
			declared = slices.Clone(a.Values)
		}
	}
	want := domain.DeliveryReasons()
	slices.Sort(declared)
	slices.Sort(want)
	if !slices.Equal(declared, want) {
		t.Fatalf("declared %v, domain %v", declared, want)
	}
	run := ids.New()
	_, end := d.StartDispatch(context.Background(), run, "lotse")
	end("deny")
	spans := rec.Ended()
	if len(spans) != 1 || spans[0].Name() != "agent.dispatch" || spans[0].SpanKind() != trace.SpanKindProducer {
		t.Fatalf("spans %v", spans)
	}
	got := map[string]string{}
	for _, kv := range spans[0].Attributes() {
		got[string(kv.Key)] = kv.Value.AsString()
	}
	if len(got) != 3 || got["bothy.run.id"] != run.String() || got["bothy.agent.handle"] != "lotse" || got["bothy.decision"] != "deny" {
		t.Fatalf("attributes %v", got)
	}
}
