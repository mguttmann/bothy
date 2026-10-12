// SPDX-License-Identifier: Apache-2.0

// Package otelmetrics is the metrics adapter of the sync module: the lag gauges of the worker
// consumers, bothy.event.consumer.lag and bothy.event.consumer.pending_age
// (docs/observability/signals.md "Ereignisse und Zustellung", alarm BothyEventConsumerLag).
// Every role registers them, so a stopped or missing worker still shows a growing lag
// (docs/DECISIONS.md D-M2-01-5). The metric registry of M1-68 takes them over when it lands.
package otelmetrics

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// Names of the instruments (signals.md 4.2).
const (
	instrumentation  = "github.com/OWNER/bothy/services/server/internal/sync"
	metricLag        = "bothy.event.consumer.lag"
	metricPendingAge = "bothy.event.consumer.pending_age"
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
	if mp == nil {
		mp = otel.GetMeterProvider()
	}
	if c == nil {
		c = clock.System()
	}
	meter := mp.Meter(instrumentation)
	lag, err := meter.Int64ObservableGauge(metricLag, metric.WithUnit("{event}"),
		metric.WithDescription("Events in the log after the cursor of a worker consumer."))
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %s: %w", metricLag, err)
	}
	age, err := meter.Float64ObservableGauge(metricPendingAge, metric.WithUnit("s"),
		metric.WithDescription("Age of the oldest event a worker consumer has not processed."))
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %s: %w", metricPendingAge, err)
	}
	g := gauges{consumers: consumers, clock: c, lag: lag, age: age, sets: map[sync.ConsumerName]metric.ObserveOption{}}
	for _, n := range sync.ConsumerNames() {
		g.sets[n] = metric.WithAttributeSet(attribute.NewSet(attribute.String(attr.ConsumerName, string(n))))
	}
	reg, err := meter.RegisterCallback(g.observe, lag, age)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: register the consumer gauges: %w", err)
	}
	return reg, nil
}

// gauges is the callback state of the two instruments.
type gauges struct {
	consumers sync.Consumers
	clock     clock.Clock
	lag       metric.Int64ObservableGauge
	age       metric.Float64ObservableGauge
	sets      map[sync.ConsumerName]metric.ObserveOption
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
		set, ok := g.sets[s.Name]
		if !ok {
			continue
		}
		o.ObserveInt64(g.lag, max(s.HeadSeq-s.LastSeq, 0), set)
		o.ObserveFloat64(g.age, pendingAge(now, s.OldestPendingAt), set)
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
