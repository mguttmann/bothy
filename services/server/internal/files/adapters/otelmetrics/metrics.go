// SPDX-License-Identifier: Apache-2.0

// Package otelmetrics is the metrics adapter of the files module: bothy.file.upload.size
// (docs/observability/signals.md 4.4) through the declaration metrics.FileUploadSize of the
// metric registry, without attributes (a file id never goes on a metric). It implements
// app.Metrics.
//
// Example:
//
//	m, err := otelmetrics.New(nil) // the global provider of platform/otel
package otelmetrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
)

// Metrics is the histogram of the completed upload sizes.
type Metrics struct {
	size *metrics.Histogram
}

// Compile-time check: Metrics is the outgoing port of app.
var _ app.Metrics = (*Metrics)(nil)

// New creates the histogram on mp; nil means the global provider that platform/otel installs in
// otel.Setup. The error comes from the meter provider.
func New(mp metric.MeterProvider) (*Metrics, error) {
	h, err := metrics.NewHistogram(mp, metrics.FileUploadSize, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return &Metrics{size: h}, nil
}

// UploadCompleted implements app.Metrics.
func (m *Metrics) UploadCompleted(ctx context.Context, sizeBytes int64) {
	m.size.Record(ctx, float64(sizeBytes))
}
