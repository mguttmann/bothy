// SPDX-License-Identifier: Apache-2.0

package otelmetrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// Wakes holds the counters of the wake-ups (signals.md 4.3, AR-04): bothy.run.wake.ended and
// bothy.run.autonomy.cap_reached.
type Wakes struct {
	ended, capReached *metrics.Counter
}

// Compile-time check: Wakes is the outgoing port of app.
var _ app.WakeMetrics = (*Wakes)(nil)

// NewWakes creates the counters on mp; nil means the global provider. The error comes from the
// meter provider.
func NewWakes(mp metric.MeterProvider) (*Wakes, error) {
	ended, err := metrics.NewCounter(mp, metrics.RunWakeEnded, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	capReached, err := metrics.NewCounter(mp, metrics.RunAutonomyCapReached, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return &Wakes{ended: ended, capReached: capReached}, nil
}

// WakeEnded implements app.WakeMetrics: one end with the end state and the wake-up's reason.
func (w *Wakes) WakeEnded(ctx context.Context, state domain.WakeState, reason domain.WakeReason) {
	w.ended.Add(ctx, 1, string(state), string(reason))
}

// AutonomyCapReached implements app.WakeMetrics: one chain at the cap named by code.
func (w *Wakes) AutonomyCapReached(ctx context.Context, code string) {
	w.capReached.Add(ctx, 1, code)
}

// PendingSource counts the pending wake-ups for the gauge (the runs repository).
type PendingSource interface {
	PendingWakeCounts(ctx context.Context) (map[domain.WakeKind]int64, error)
}

// RegisterWakePendingGauge registers bothy.run.wake.pending on mp (nil means the global
// provider) with one callback that reads src once per collection and observes the sum over the
// kinds. A failed read drops the point of that collection and never fails it (17 rule 4). It
// returns the registration. Errors: those of the meter provider.
func RegisterWakePendingGauge(mp metric.MeterProvider, src PendingSource) (metric.Registration, error) {
	g, err := metrics.NewObservableGauge(mp, metrics.RunWakePending, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	reg, err := g.Register(func(ctx context.Context, o metric.Observer) error {
		counts, err := src.PendingWakeCounts(ctx)
		if err != nil {
			return nil
		}
		var n int64
		for _, c := range counts {
			n += c
		}
		g.Observe(o, float64(n))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: register bothy.run.wake.pending: %w", err)
	}
	return reg, nil
}
