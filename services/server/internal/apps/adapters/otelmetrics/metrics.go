// SPDX-License-Identifier: Apache-2.0

// Package otelmetrics is the metrics adapter of the apps module (#925): the counters of the
// outgoing webhooks of docs/observability/signals.md 4.2 through the declarations
// metrics.WebhookDelivery and metrics.WebhookTransition of the metric registry (M1-68). Metrics
// implements app.WebhookMetrics. Attribute values are codes from closed sets only (the outcome,
// the cause), never a subscription, an app or an address; a value outside its set records as
// "other".
//
// Example:
//
//	m, err := otelmetrics.New(nil) // the global provider of platform/otel
package otelmetrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
)

// Metrics holds the instruments of the outgoing webhooks.
type Metrics struct {
	deliveries, transitions *metrics.Counter
}

// Compile-time check: Metrics is the outgoing port of app.
var _ app.WebhookMetrics = (*Metrics)(nil)

// New creates the instruments on mp; nil means the global provider that platform/otel installs
// in otel.Setup. The error comes from the meter provider.
func New(mp metric.MeterProvider) (*Metrics, error) {
	deliveries, err := metrics.NewCounter(mp, metrics.WebhookDelivery, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	transitions, err := metrics.NewCounter(mp, metrics.WebhookTransition, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	// Every series starts at 0, so the first system transition is an increase that
	// BothyWebhookSubscriptionStopped sees (alerting.md 3.2).
	ctx := context.Background()
	for _, outcome := range []string{"succeeded", "retrying", "failed", "dead"} {
		deliveries.Add(ctx, 0, outcome)
	}
	for _, cause := range []string{"gone", "max_consecutive_failures", "retry_max_age", "gap", "max_pending"} {
		transitions.Add(ctx, 0, cause)
	}
	return &Metrics{deliveries: deliveries, transitions: transitions}, nil
}

// Deliveries implements app.WebhookMetrics: n on bothy.webhook.delivery with bothy.outcome.
func (m *Metrics) Deliveries(ctx context.Context, outcome string, n int) {
	m.deliveries.Add(ctx, int64(n), outcome)
}

// Transition implements app.WebhookMetrics: bothy.webhook.transition with bothy.reason.
func (m *Metrics) Transition(ctx context.Context, cause string) {
	m.transitions.Add(ctx, 1, cause)
}
