// SPDX-License-Identifier: Apache-2.0

// Package otelmetrics is the metrics adapter of the search module (docs/observability/signals.md
// 4.4) through the declaration metrics.SearchDuration of the metric registry: bothy.search.duration
// with bothy.outcome only (ok, client_error, timeout, server_error; docs/DECISIONS.md D-M3-14-13).
// A search term, an excerpt or an id never goes on a metric (SEC-LOG-4). It implements app.Metrics.
//
// Example:
//
//	m, err := otelmetrics.New(nil) // the global provider of platform/otel
package otelmetrics

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/search/app"
)

// Metrics holds bothy.search.duration.
type Metrics struct {
	duration *metrics.Histogram
}

// Compile-time check: Metrics is the outgoing port of app.
var _ app.Metrics = (*Metrics)(nil)

// New creates the instrument on mp; nil means the global provider that platform/otel installs in
// otel.Setup. The error comes from the meter provider.
func New(mp metric.MeterProvider) (*Metrics, error) {
	h, err := metrics.NewHistogram(mp, metrics.SearchDuration, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return &Metrics{duration: h}, nil
}

// SearchCompleted implements app.Metrics: one duration with bothy.outcome.
func (m *Metrics) SearchCompleted(ctx context.Context, outcome string, d time.Duration) {
	m.duration.RecordDuration(ctx, d, outcome)
}
