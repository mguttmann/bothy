// SPDX-License-Identifier: Apache-2.0

// Package otelmetrics is the metrics adapter of the files module (docs/observability/signals.md
// 4.4) through the declarations of the metric registry: bothy.file.upload.size without
// attributes (M3-03), and bothy.file.scan, bothy.file.scan.duration with bothy.outcome only and
// bothy.file.scan.queue without attributes (M3-04). A file id, name or signature never goes on a
// metric. It implements app.Metrics and app.ScanMetrics.
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

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
)

// Metrics holds the instruments of the files module.
type Metrics struct {
	size     *metrics.Histogram
	scans    *metrics.Counter
	duration *metrics.Histogram
	queue    *metrics.Gauge
}

// Compile-time checks: Metrics is the outgoing port of app.
var (
	_ app.Metrics     = (*Metrics)(nil)
	_ app.ScanMetrics = (*Metrics)(nil)
)

// New creates the instruments on mp; nil means the global provider that platform/otel installs
// in otel.Setup. The error comes from the meter provider.
func New(mp metric.MeterProvider) (*Metrics, error) {
	h, err := metrics.NewHistogram(mp, metrics.FileUploadSize, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	c, err := metrics.NewCounter(mp, metrics.FileScan, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	d, err := metrics.NewHistogram(mp, metrics.FileScanDuration, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	q, err := metrics.NewGauge(mp, metrics.FileScanQueue, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return &Metrics{size: h, scans: c, duration: d, queue: q}, nil
}

// ScanCompleted implements app.ScanMetrics: one count and the duration with bothy.outcome.
func (m *Metrics) ScanCompleted(ctx context.Context, outcome string, d time.Duration) {
	m.scans.Add(ctx, 1, outcome)
	m.duration.RecordDuration(ctx, d, outcome)
}

// ScanQueue implements app.ScanMetrics.
func (m *Metrics) ScanQueue(ctx context.Context, n int64) {
	m.queue.Record(ctx, float64(n))
}

// UploadCompleted implements app.Metrics.
func (m *Metrics) UploadCompleted(ctx context.Context, sizeBytes int64) {
	m.size.Record(ctx, float64(sizeBytes))
}

// PurgeBacklog records bothy.file.purge.backlog (signals.md 4.4, M3-08): the number of files marked
// deleted whose purge is still due, set at the end of every run of files.cleanup_pending. It is a
// synchronous gauge without attributes: it reports the last value a run recorded, nothing before
// the first run, and never queries the database at collection (docs/decisions.d/M3-08.md).
type PurgeBacklog struct {
	g *metrics.Gauge
}

// Compile-time check: PurgeBacklog is the outgoing port of the cleanup.
var _ app.PurgeMetrics = (*PurgeBacklog)(nil)

// NewPurgeBacklog creates the gauge on mp; nil means the global provider. The error comes from the
// meter provider.
//
// Example:
//
//	m, err := otelmetrics.NewPurgeBacklog(nil)
func NewPurgeBacklog(mp metric.MeterProvider) (*PurgeBacklog, error) {
	g, err := metrics.NewGauge(mp, metrics.FilePurgeBacklog, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return &PurgeBacklog{g: g}, nil
}

// PurgeBacklog implements app.PurgeMetrics.
func (m *PurgeBacklog) PurgeBacklog(ctx context.Context, n int64) {
	m.g.Record(ctx, float64(n))
}
