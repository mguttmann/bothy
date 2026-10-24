// SPDX-License-Identifier: Apache-2.0

package otelmetrics

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/stats/app"
)

// The values of bothy.outcome on bothy.stats.flush.duration.
const (
	outcomeOK    = "ok"
	outcomeError = "error"
)

// FlushMetrics are the operating instruments of the own measurement (signals.md 4.3b):
// bothy.stats.flush.dropped and bothy.stats.flush.duration.
type FlushMetrics struct {
	dropped  *metrics.Counter
	duration *metrics.Histogram
}

// Compile-time check: FlushMetrics is the outgoing port of app.
var _ app.FlushMetrics = (*FlushMetrics)(nil)

// NewFlush creates the two instruments on mp (nil means the global provider of otel.Setup).
// Errors: those of the registry or the meter provider.
func NewFlush(mp metric.MeterProvider) (*FlushMetrics, error) {
	d, err := metrics.NewCounter(mp, metrics.StatsFlushDropped, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	h, err := metrics.NewHistogram(mp, metrics.StatsFlushDuration, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return &FlushMetrics{dropped: d, duration: h}, nil
}

// Dropped implements app.FlushMetrics: n keys discarded with reason.
func (f *FlushMetrics) Dropped(ctx context.Context, reason string, n int) {
	f.dropped.Add(ctx, int64(n), reason)
}

// Flushed implements app.FlushMetrics: one flush that took d.
func (f *FlushMetrics) Flushed(ctx context.Context, d time.Duration, ok bool) {
	outcome := outcomeOK
	if !ok {
		outcome = outcomeError
	}
	f.duration.RecordDuration(ctx, d, outcome)
}
