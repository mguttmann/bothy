// SPDX-License-Identifier: Apache-2.0

package otelmetrics_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/apps/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
)

// The instrument names of signals.md 4.2.
const (
	deliveries  = "bothy.webhook.delivery"
	transitions = "bothy.webhook.transition"
)

func TestMetrics_RecordsTheWebhookSignals(t *testing.T) {
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	m, err := otelmetrics.New(mp)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m.Deliveries(ctx, "succeeded", 1)
	m.Deliveries(ctx, "dead", 3)
	m.Deliveries(ctx, "retrying", 1)
	m.Deliveries(ctx, "failed", 1)
	for _, cause := range []string{"max_pending", "gap", "gap", "max_consecutive_failures", "retry_max_age", "gone"} {
		m.Transition(ctx, cause)
	}
	got := metricstest.Collect(t, reader)
	for outcome, want := range map[string]float64{"succeeded": 1, "dead": 3, "retrying": 1, "failed": 1} {
		if v := metricstest.Sum(got[deliveries], map[string]string{"bothy.outcome": outcome}); v != want {
			t.Errorf("deliveries %s = %v, want %v", outcome, v, want)
		}
	}
	if v := metricstest.Sum(got[transitions], map[string]string{"bothy.reason": "gap"}); v != 2 {
		t.Errorf("transitions gap = %v, want 2", v)
	}
	if v := metricstest.Sum(got[transitions], map[string]string{"bothy.reason": "max_pending"}); v != 1 {
		t.Errorf("transitions max_pending = %v, want 1", v)
	}
}

// TestNew_SeriesStartAtZero: every cause has a series at 0 before its first transition, so the
// alert BothyWebhookSubscriptionStopped sees the first one as an increase.
func TestNew_SeriesStartAtZero(t *testing.T) {
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	if _, err := otelmetrics.New(mp); err != nil {
		t.Fatal(err)
	}
	got := metricstest.Collect(t, reader)
	if n := len(metricstest.Points(got[transitions])); n != 5 {
		t.Fatalf("%d transition series, want 5", n)
	}
}
