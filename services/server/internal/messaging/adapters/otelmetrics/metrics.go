// SPDX-License-Identifier: Apache-2.0

// Package otelmetrics is the metrics adapter of the messaging module: bothy.message.created
// (docs/observability/signals.md 4.2) through the declaration metrics.MessageCreated of the
// metric registry. It implements app.Metrics.
//
// Example:
//
//	m, err := otelmetrics.New(nil) // the global provider of platform/otel
package otelmetrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
)

// Metrics is the counter of the created messages.
type Metrics struct {
	created *metrics.Counter
}

// Compile-time check: Metrics is the outgoing port of app.
var _ app.Metrics = (*Metrics)(nil)

// New creates the counter on mp; nil means the global provider that platform/otel installs in
// otel.Setup. The error comes from the meter provider.
func New(mp metric.MeterProvider) (*Metrics, error) {
	c, err := metrics.NewCounter(mp, metrics.MessageCreated, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return &Metrics{created: c}, nil
}

// MessageCreated implements app.Metrics.
func (m *Metrics) MessageCreated(ctx context.Context, origin, kind string) {
	m.created.Add(ctx, 1, origin, kind)
}
