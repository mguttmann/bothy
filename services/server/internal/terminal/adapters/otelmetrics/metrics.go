// SPDX-License-Identifier: Apache-2.0

// Package otelmetrics is the metrics adapter of the terminal module (M2-44, D-W-105): the counter
// bothy.terminal.relay.unpaired of docs/observability/signals.md 4.3a through the declaration of
// the metric registry, with the attribute bothy.terminal.side. It implements app.Metrics; M2-46
// adds the other terminal instruments here.
//
// Example:
//
//	m, err := otelmetrics.New(nil) // the global provider of platform/otel
//	svc.BindMetrics(m)
package otelmetrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// Metrics are the instruments of the terminal module.
type Metrics struct {
	unpaired *metrics.Counter
}

// Compile-time check: Metrics is the outgoing port of app.
var _ app.Metrics = (*Metrics)(nil)

// New creates the instruments on mp; nil means the global provider that platform/otel installs
// in otel.Setup. Errors: those of the meter provider.
func New(mp metric.MeterProvider) (*Metrics, error) {
	c, err := metrics.NewCounter(mp, metrics.TerminalRelayUnpaired, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return &Metrics{unpaired: c}, nil
}

// RelayUnpaired implements app.Metrics.
func (m *Metrics) RelayUnpaired(ctx context.Context, side domain.Side) {
	m.unpaired.Add(ctx, 1, string(side))
}
