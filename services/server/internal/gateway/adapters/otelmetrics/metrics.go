// SPDX-License-Identifier: Apache-2.0

// Package otelmetrics is the metrics adapter of the gateway module: the gateway, delivery and
// fan-out instruments of docs/observability/signals.md 4.1 and 4.2 through the declarations of
// the metric registry. It implements app.Metrics; RegisterFanoutLag registers the gauge
// bothy.event.fanout.lag over the hub.
//
// Example:
//
//	m, err := otelmetrics.New(nil) // the global provider of platform/otel
//	reg, err := otelmetrics.RegisterFanoutLag(nil, hub.FanoutLag)
package otelmetrics

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
)

// Metrics are the instruments of the gateway.
type Metrics struct {
	active    *metrics.UpDownCounter
	connect   *metrics.Histogram
	catchup   *metrics.Histogram
	resync    *metrics.Counter
	closes    *metrics.Counter
	dropped   *metrics.Counter
	delivered *metrics.Histogram
}

// Compile-time check: Metrics is the outgoing port of app.
var _ app.Metrics = (*Metrics)(nil)

// New creates the instruments on mp; nil means the global provider that platform/otel installs
// in otel.Setup. bothy.close.code takes its values from the close codes of 04 (domain.CloseCodes)
// as decimal strings. Errors: those of the meter provider.
func New(mp metric.MeterProvider) (*Metrics, error) {
	codes := domain.CloseCodes()
	names := make([]string, len(codes))
	for i, c := range codes {
		names[i] = strconv.Itoa(int(c))
	}
	var m Metrics
	var err error
	if m.active, err = metrics.NewUpDownCounter(mp, metrics.GatewayConnectionsActive, nil); err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	if m.connect, err = metrics.NewHistogram(mp, metrics.GatewayConnectDuration, nil); err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	if m.catchup, err = metrics.NewHistogram(mp, metrics.GatewayCatchupDuration, nil); err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	if m.resync, err = metrics.NewCounter(mp, metrics.GatewayResync, nil); err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	if m.closes, err = metrics.NewCounter(mp, metrics.GatewayClose, metrics.Values{metrics.SourceCloseCodes: names}); err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	if m.dropped, err = metrics.NewCounter(mp, metrics.GatewayOutboundDropped, nil); err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	if m.delivered, err = metrics.NewHistogram(mp, metrics.DeliveryDuration, nil); err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return &m, nil
}

// ConnectionOpened implements app.Metrics.
func (m *Metrics) ConnectionOpened(ctx context.Context, kind string) { m.active.Add(ctx, 1, kind) }

// ConnectionClosed implements app.Metrics.
func (m *Metrics) ConnectionClosed(ctx context.Context, kind string) { m.active.Add(ctx, -1, kind) }

// Connected implements app.Metrics.
func (m *Metrics) Connected(ctx context.Context, d time.Duration, outcome, kind string) {
	m.connect.RecordDuration(ctx, d, outcome, kind)
}

// CatchupDone implements app.Metrics.
func (m *Metrics) CatchupDone(ctx context.Context, d time.Duration, outcome string) {
	m.catchup.RecordDuration(ctx, d, outcome)
}

// Resync implements app.Metrics.
func (m *Metrics) Resync(ctx context.Context, reason string) { m.resync.Add(ctx, 1, reason) }

// Closed implements app.Metrics: a code outside 04 "Close codes" (a client may send any code)
// counts as other.
func (m *Metrics) Closed(ctx context.Context, code int) {
	m.closes.Add(ctx, 1, strconv.Itoa(code))
}

// Dropped implements app.Metrics.
func (m *Metrics) Dropped(ctx context.Context, reason string, n int) {
	m.dropped.Add(ctx, int64(n), reason)
}

// Delivered implements app.Metrics.
func (m *Metrics) Delivered(ctx context.Context, d time.Duration, durability, kind string) {
	m.delivered.RecordDuration(ctx, d, durability, kind)
}

// RegisterFanoutLag registers bothy.event.fanout.lag on mp (nil means the global provider) with a
// callback that observes lag() in seconds; lag is Hub.FanoutLag, which reads only counters of the
// hub, so a collection never waits. Errors: those of the meter provider.
func RegisterFanoutLag(mp metric.MeterProvider, lag func() time.Duration) (metric.Registration, error) {
	g, err := metrics.NewObservableGauge(mp, metrics.EventFanoutLag, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return g.Register(func(_ context.Context, o metric.Observer) error {
		g.Observe(o, lag().Seconds())
		return nil
	})
}
