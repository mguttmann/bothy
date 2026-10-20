// SPDX-License-Identifier: Apache-2.0

package otelmetrics_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
)

// TestRegisterPresence: both gauges observe the samples of the tracker per handle; an agent
// without a heartbeat has no heartbeat age, a mismatch age of 0 without a difference.
func TestRegisterPresence(t *testing.T) {
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	samples := []app.GaugeSample{
		{Handle: "support-agent", HasHeartbeat: true, HeartbeatAge: 4 * time.Second, MismatchAge: 301 * time.Second},
		{Handle: "runner", HasHeartbeat: true, HeartbeatAge: 1500 * time.Millisecond},
		{Handle: "fresh"},
	}
	if _, err := otelmetrics.RegisterPresence(mp, func() []app.GaugeSample { return samples }); err != nil {
		t.Fatal(err)
	}
	got := metricstest.Collect(t, reader)
	age, mismatch := got["bothy.agent.heartbeat.age"], got["bothy.agent.billing_mode.mismatch_age"]
	for handle, want := range map[string]float64{"support-agent": 4, "runner": 1.5} {
		if v := metricstest.Sum(age, map[string]string{"bothy.agent.handle": handle}); v != want {
			t.Errorf("heartbeat age of %s = %v, want %v", handle, v, want)
		}
	}
	if n := len(metricstest.Points(age)); n != 2 {
		t.Errorf("%d heartbeat age series, want 2 (none without a heartbeat)", n)
	}
	for handle, want := range map[string]float64{"support-agent": 301, "runner": 0, "fresh": 0} {
		if v := metricstest.Sum(mismatch, map[string]string{"bothy.agent.handle": handle}); v != want {
			t.Errorf("mismatch age of %s = %v, want %v", handle, v, want)
		}
	}
	if n := len(metricstest.Points(mismatch)); n != 3 {
		t.Errorf("%d mismatch series, want 3", n)
	}
}
