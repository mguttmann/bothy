// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// The values of bothy.outcome on the span txn.subscribe (docs/observability/signals.md "Spans").
const (
	outcomeOK   = "ok"
	outcomeVeto = "veto"
)

// SubscriberSpan starts the INTERNAL span of one in-transaction subscriber delivery,
// "txn.subscribe <module>" (docs/spec/02-architecture.md "In-Tx events"), and returns the
// context for the subscriber and the function that ends the span with bothy.outcome ok (nil
// error) or veto. It has the shape of kernel/txn.Tracer; internal/wire sets it on the registry.
// It reads the global tracer provider at every call, so it may be set before Setup ran.
func SubscriberSpan(ctx context.Context, name string) (context.Context, func(err error)) {
	return subscriberSpan(otel.GetTracerProvider)(ctx, name)
}

// subscriberSpan returns SubscriberSpan over the provider tp returns.
func subscriberSpan(tp func() trace.TracerProvider) func(context.Context, string) (context.Context, func(error)) {
	return func(ctx context.Context, name string) (context.Context, func(error)) {
		ctx, span := tp().Tracer(instrumentationName).Start(ctx, name, trace.WithSpanKind(trace.SpanKindInternal))
		return ctx, func(err error) {
			outcome := outcomeOK
			if err != nil {
				outcome = outcomeVeto
			}
			span.SetAttributes(attribute.String(attr.Outcome, outcome))
			span.End()
		}
	}
}
