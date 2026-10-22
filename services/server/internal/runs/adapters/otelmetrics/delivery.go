// SPDX-License-Identifier: Apache-2.0

package otelmetrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
)

// SpanDispatch is the span of one checked delivery to an agent (signals.md "agent.dispatch").
const SpanDispatch = "agent.dispatch"

// instrumentationName names the tracer of the runs module.
const instrumentationName = "github.com/OWNER/bothy/services/server/internal/runs"

// Delivery implements app.DeliveryTelemetry: the counter bothy.agent.invocation.denied through
// the declaration metrics.AgentInvocationDenied and the PRODUCER span agent.dispatch with
// bothy.run.id, bothy.agent.handle and bothy.decision. Neither carries a token, a jti or content.
type Delivery struct {
	denied *metrics.Counter
	tp     func() trace.TracerProvider
}

// Compile-time check: Delivery is the outgoing port of app.
var _ app.DeliveryTelemetry = (*Delivery)(nil)

// NewDelivery creates the counter on mp and the spans on tp; nil means the global providers that
// platform/otel installs in otel.Setup. The error comes from the meter provider.
func NewDelivery(mp metric.MeterProvider, tp trace.TracerProvider) (*Delivery, error) {
	c, err := metrics.NewCounter(mp, metrics.AgentInvocationDenied, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	d := &Delivery{denied: c, tp: otel.GetTracerProvider}
	if tp != nil {
		d.tp = func() trace.TracerProvider { return tp }
	}
	return d, nil
}

// Denied implements app.DeliveryTelemetry: one count with bothy.agent.handle and bothy.reason.
func (d *Delivery) Denied(ctx context.Context, agentHandle, reason string) {
	d.denied.Add(ctx, 1, agentHandle, reason)
}

// StartDispatch implements app.DeliveryTelemetry: the span agent.dispatch as a child of the
// delivery's context; end sets bothy.decision and closes it.
func (d *Delivery) StartDispatch(ctx context.Context, runID ids.ID, agentHandle string) (context.Context, func(decision string)) {
	ctx, span := d.tp().Tracer(instrumentationName).Start(ctx, SpanDispatch, trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(attribute.String(attr.RunID, runID.String()), attribute.String(attr.AgentHandle, agentHandle)))
	return ctx, func(decision string) {
		span.SetAttributes(attribute.String(attr.Decision, decision))
		span.End()
	}
}
