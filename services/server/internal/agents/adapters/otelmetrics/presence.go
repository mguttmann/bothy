// SPDX-License-Identifier: Apache-2.0

// Package otelmetrics is the metrics adapter of the agents module (M2-20): the observable gauges
// bothy.agent.heartbeat.age and bothy.agent.billing_mode.mismatch_age of
// docs/observability/signals.md 4.3, declared in the metric registry, over the memory of the
// presence tracker. The alarm BothyBillingModeMismatch reads the second one.
//
// Example:
//
//	reg, err := otelmetrics.RegisterPresence(nil, tracker.Samples) // nil: the global provider
package otelmetrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
)

// RegisterPresence creates both gauges on mp (nil means the global provider of platform/otel)
// and registers one callback that reads samples at each collection: the heartbeat age per agent
// with a known last heartbeat, and the mismatch age per agent (0 without a difference). samples
// reads only memory, so the callback never blocks (17 rule 4); the registration lives as long as
// the process. Errors: those of the meter provider.
func RegisterPresence(mp metric.MeterProvider, samples func() []app.GaugeSample) (metric.Registration, error) {
	age, err := metrics.NewObservableGauge(mp, metrics.AgentHeartbeatAge, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	mismatch, err := metrics.NewObservableGauge(mp, metrics.AgentBillingModeMismatchAge, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	reg, err := age.Meter().RegisterCallback(func(_ context.Context, o metric.Observer) error {
		for _, s := range samples() {
			if s.HasHeartbeat {
				age.Observe(o, s.HeartbeatAge.Seconds(), s.Handle)
			}
			mismatch.Observe(o, s.MismatchAge.Seconds(), s.Handle)
		}
		return nil
	}, age.Observable(), mismatch.Observable())
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return reg, nil
}
