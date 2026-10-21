// SPDX-License-Identifier: Apache-2.0

// Package otelmetrics is the metrics adapter of the notify module (M3-19): the push instruments
// of docs/observability/signals.md 4.4 through the declarations metrics.PushSent,
// metrics.PushSendDuration, metrics.PushSkipped and metrics.PushRegistrationsActive of the metric
// registry (M1-68). Metrics implements app.Metrics; RegisterActiveGauge registers the gauge
// bothy.push.registrations.active. Attribute values are codes from closed sets only (the
// provider, the outcome, the skip reason), never a person, a thread or a token; a value outside
// its set records as "other".
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

	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
)

// Metrics holds the instruments of the push path.
type Metrics struct {
	sent, skipped *metrics.Counter
	duration      *metrics.Histogram
}

// Compile-time check: Metrics is the outgoing port of app.
var _ app.Metrics = (*Metrics)(nil)

// New creates the instruments on mp; nil means the global provider that platform/otel installs
// in otel.Setup. The error comes from the meter provider.
func New(mp metric.MeterProvider) (*Metrics, error) {
	sent, err := metrics.NewCounter(mp, metrics.PushSent, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	dur, err := metrics.NewHistogram(mp, metrics.PushSendDuration, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	skipped, err := metrics.NewCounter(mp, metrics.PushSkipped, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return &Metrics{sent: sent, skipped: skipped, duration: dur}, nil
}

// Sent implements app.Metrics: bothy.push.sent with bothy.push.provider and bothy.outcome.
func (m *Metrics) Sent(ctx context.Context, provider, outcome string) {
	m.sent.Add(ctx, 1, provider, outcome)
}

// SendDuration implements app.Metrics: bothy.push.send.duration in seconds with
// bothy.push.provider.
func (m *Metrics) SendDuration(ctx context.Context, provider string, d time.Duration) {
	m.duration.RecordDuration(ctx, d, provider)
}

// Skipped implements app.Metrics: bothy.push.skipped with bothy.reason.
func (m *Metrics) Skipped(ctx context.Context, reason string) {
	m.skipped.Add(ctx, 1, reason)
}

// CountSource reads the active registrations per provider, the data points of
// bothy.push.registrations.active; app.RegistrationStore.ActiveCounts (adapters/postgres
// Registrations) is the implementation. The read is bounded by database.statement_timeout there.
type CountSource interface {
	// ActiveCounts returns the rows with disabled_at IS NULL per provider.
	ActiveCounts(ctx context.Context) (map[string]int64, error)
}

// RegisterActiveGauge registers bothy.push.registrations.active on mp (nil means the global
// provider) with one callback: it reads src once per collection and observes one point per
// provider with an active row, with bothy.push.provider only. A failed read drops the points of
// that collection and never fails it (17 rule 4: telemetry never blocks); the series go stale
// instead.
//
// It returns the registration, whose Unregister removes the callback.
//
// Errors: those of the meter provider.
func RegisterActiveGauge(mp metric.MeterProvider, src CountSource) (metric.Registration, error) {
	g, err := metrics.NewObservableGauge(mp, metrics.PushRegistrationsActive, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	reg, err := g.Register(func(ctx context.Context, o metric.Observer) error {
		counts, err := src.ActiveCounts(ctx)
		if err != nil {
			return nil
		}
		for provider, n := range counts {
			g.Observe(o, float64(n), provider)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: register bothy.push.registrations.active: %w", err)
	}
	return reg, nil
}
