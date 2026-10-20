// SPDX-License-Identifier: Apache-2.0

// Package otelmetrics is the metrics adapter of the runs module: the run instruments of
// docs/observability/signals.md 4.3 through the declarations metrics.RunStartLatency,
// metrics.RunDuration, metrics.RunFinished and metrics.RunActive of the metric registry. Metrics
// implements app.Metrics; RegisterActiveGauge registers the gauge bothy.run.active.
//
// Example:
//
//	m, err := otelmetrics.New(nil) // the global provider of platform/otel
package otelmetrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// Metrics holds the instruments of the committed run transitions.
type Metrics struct {
	startLatency, duration *metrics.Histogram
	finished               *metrics.Counter
}

// Compile-time check: Metrics is the outgoing port of app.
var _ app.Metrics = (*Metrics)(nil)

// New creates the instruments on mp; nil means the global provider that platform/otel installs
// in otel.Setup. bothy.agent.handle needs no values: its set is the handle pattern
// (metrics.SourceAgentHandles). The error comes from the meter provider.
func New(mp metric.MeterProvider) (*Metrics, error) {
	start, err := metrics.NewHistogram(mp, metrics.RunStartLatency, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	dur, err := metrics.NewHistogram(mp, metrics.RunDuration, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	fin, err := metrics.NewCounter(mp, metrics.RunFinished, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return &Metrics{startLatency: start, duration: dur, finished: fin}, nil
}

// outcomes maps the target of a transition out of queued to bothy.outcome.
var outcomes = map[domain.State]string{
	domain.StateRunning:   "started",
	domain.StateFailed:    "failed",
	domain.StateCancelled: "cancelled",
}

// RunTransitioned implements app.Metrics: bothy.run.start.latency when the run left queued (the
// reported billing mode, else the expected one, because a run that failed before its start has
// no reported mode), bothy.run.duration when a started run ended and bothy.run.finished on every
// end state, with bothy.reason only for failed and cancelled. Every other transition records
// nothing.
func (m *Metrics) RunTransitioned(ctx context.Context, t app.Transitioned) {
	if t.From == domain.StateQueued {
		mode := t.Run.BillingModeUsed
		if mode == "" {
			mode = t.Run.BillingModeExpected
		}
		m.startLatency.RecordDuration(ctx, t.QueuedFor, t.AgentHandle, outcomes[t.To], mode)
	}
	if !t.To.Ended() {
		return
	}
	if t.Started {
		m.duration.RecordDuration(ctx, t.Ran, t.AgentHandle, string(t.To))
	}
	reason := ""
	if t.To != domain.StateCompleted {
		reason = t.Run.StateReason
	}
	m.finished.Add(ctx, 1, t.AgentHandle, string(t.To), reason)
}

// ActiveSource reads the active runs grouped by agent handle and state, the data points of
// bothy.run.active. The read is bounded by database.statement_timeout in the implementation.
type ActiveSource interface {
	// ActiveGroups returns one group per agent handle and active state with at least one run.
	ActiveGroups(ctx context.Context) ([]app.ActiveGroup, error)
}

// RegisterActiveGauge registers bothy.run.active on mp (nil means the global provider) with one
// callback: it reads src once per collection and observes one point per group with
// bothy.agent.handle and bothy.run.state, never an ID. A failed read drops the points of that
// collection and never fails it (17 rule 4: telemetry never blocks); the series go stale instead.
//
// It returns the registration, whose Unregister removes the callback.
//
// Errors: those of the meter provider.
func RegisterActiveGauge(mp metric.MeterProvider, src ActiveSource) (metric.Registration, error) {
	g, err := metrics.NewObservableGauge(mp, metrics.RunActive, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	reg, err := g.Register(func(ctx context.Context, o metric.Observer) error {
		groups, err := src.ActiveGroups(ctx)
		if err != nil {
			return nil
		}
		for _, gr := range groups {
			g.Observe(o, float64(gr.Count), gr.AgentHandle, string(gr.State))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: register bothy.run.active: %w", err)
	}
	return reg, nil
}
