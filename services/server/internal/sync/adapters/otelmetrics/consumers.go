// SPDX-License-Identifier: Apache-2.0

// Package otelmetrics is the metrics adapter of the sync module: the lag gauges of the worker
// consumers, bothy.event.consumer.lag and bothy.event.consumer.pending_age
// (docs/observability/signals.md "Ereignisse und Zustellung", alarm BothyEventConsumerLag).
// Every role registers them, so a stopped or missing worker still shows a growing lag
// (docs/DECISIONS.md D-M2-01-5). Both are declared in the metric registry
// (metrics.EventConsumerLag, metrics.EventConsumerPendingAge).
package otelmetrics

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// RegisterConsumerGauges registers both gauges on mp (nil means the global provider that
// platform/otel installs in otel.Setup) with one callback: it reads consumers.States once per
// collection (bounded by database.statement_timeout inside States) and observes per cursor row
// the lag head_seq minus last_seq and the age now minus occurred_at of the oldest pending event,
// both floored at zero and zero at the head, with bothy.consumer.name as the only attribute from
// the closed set sync.ConsumerNames. A failed read drops the data points of that collection and
// never blocks it (17 rule 4). now comes from c.
//
// It returns the registration, whose Unregister removes the callback.
//
// Errors: those of the meter provider.
func RegisterConsumerGauges(mp metric.MeterProvider, consumers sync.Consumers, c clock.Clock) (metric.Registration, error) {
	if c == nil {
		c = clock.System()
	}
	lag, err := metrics.NewObservableGauge(mp, metrics.EventConsumerLag, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	age, err := metrics.NewObservableGauge(mp, metrics.EventConsumerPendingAge, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	g := gauges{consumers: consumers, clock: c, lag: lag, age: age, known: map[sync.ConsumerName]bool{}}
	for _, n := range sync.ConsumerNames() {
		g.known[n] = true
	}
	reg, err := lag.Meter().RegisterCallback(g.observe, lag.Observable(), age.Observable())
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: register the consumer gauges: %w", err)
	}
	return reg, nil
}

// gauges is the callback state of the two instruments.
type gauges struct {
	consumers sync.Consumers
	clock     clock.Clock
	lag, age  *metrics.ObservableGauge
	known     map[sync.ConsumerName]bool
}

// observe reads the states once and observes both gauges per known consumer. A failed read
// drops the points of this collection: the alarm sees the series go stale, and the collection
// never blocks.
func (g gauges) observe(ctx context.Context, o metric.Observer) error {
	states, err := g.consumers.States(ctx)
	if err != nil {
		return nil
	}
	now := g.clock.Now()
	for _, s := range states {
		if !g.known[s.Name] {
			continue
		}
		g.lag.Observe(o, float64(max(s.HeadSeq-s.LastSeq, 0)), string(s.Name))
		g.age.Observe(o, pendingAge(now, s.OldestPendingAt), string(s.Name))
	}
	return nil
}

// pendingAge is now minus oldest in seconds, floored at zero; zero without a pending event.
func pendingAge(now, oldest time.Time) float64 {
	if oldest.IsZero() {
		return 0
	}
	return max(now.Sub(oldest).Seconds(), 0)
}
