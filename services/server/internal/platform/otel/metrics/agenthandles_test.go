// SPDX-License-Identifier: Apache-2.0

package metrics_test

import (
	"context"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
)

// TestAgentHandlesSource: bothy.agent.handle of the presence gauges (M2-20) admits every value of
// the handle form without values at construction, and maps anything else to "other"
// (docs/DECISIONS.md D-M2-20-13).
func TestAgentHandlesSource(t *testing.T) {
	mp, reader := metricstest.Provider(t)
	for _, s := range []metrics.Spec{metrics.AgentHeartbeatAge, metrics.AgentBillingModeMismatchAge} {
		if problems := metrics.Check([]metrics.Spec{s}); len(problems) > 0 {
			t.Fatalf("%s: %v", s.Name, problems)
		}
	}
	g, err := metrics.NewObservableGauge(mp, metrics.AgentHeartbeatAge, nil)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := metrics.NewObservableGauge(mp, metrics.AgentBillingModeMismatchAge, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Register(func(_ context.Context, o metric.Observer) error {
		g.Observe(o, 4, "support-agent")
		g.Observe(o, 5, "Not A Handle")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := m2.Register(func(_ context.Context, o metric.Observer) error {
		m2.Observe(o, 6, strings.Repeat("a", 65))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	got := metricstest.Collect(t, reader)
	age, mismatch := got["bothy.agent.heartbeat.age"], got["bothy.agent.billing_mode.mismatch_age"]
	if v := metricstest.Sum(age, map[string]string{"bothy.agent.handle": "support-agent"}); v != 4 {
		t.Errorf("support-agent = %v, want 4", v)
	}
	if v := metricstest.Sum(age, map[string]string{"bothy.agent.handle": metrics.Other}); v != 5 {
		t.Errorf("a value outside the form = %v under other, want 5", v)
	}
	if v := metricstest.Sum(mismatch, map[string]string{"bothy.agent.handle": metrics.Other}); v != 6 {
		t.Errorf("a handle above 64 characters = %v under other, want 6", v)
	}
}
