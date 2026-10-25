// SPDX-License-Identifier: Apache-2.0

package otelmetrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/stats/app"
)

// LiveMetrics is the gauge of the live stream (signals.md 4.3b, M6-15):
// bothy.stats.live.subscribers.
type LiveMetrics struct {
	subscribers *metrics.UpDownCounter
}

// Compile-time check: LiveMetrics is the outgoing port of app.
var _ app.LiveMetrics = (*LiveMetrics)(nil)

// NewLive creates the instrument on mp (nil means the global provider of otel.Setup). Errors:
// those of the registry or the meter provider.
func NewLive(mp metric.MeterProvider) (*LiveMetrics, error) {
	c, err := metrics.NewUpDownCounter(mp, metrics.StatsLiveSubscribers, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return &LiveMetrics{subscribers: c}, nil
}

// Subscribers implements app.LiveMetrics.
func (l *LiveMetrics) Subscribers(ctx context.Context, delta int64) { l.subscribers.Add(ctx, delta) }
