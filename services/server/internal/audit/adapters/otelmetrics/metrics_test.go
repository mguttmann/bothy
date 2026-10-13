// SPDX-License-Identifier: Apache-2.0

package otelmetrics_test

import (
	"context"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/audit/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/audit/app"
)

// Compile-time check: the instruments are the outgoing port of app.
var _ app.Metrics = (*otelmetrics.Metrics)(nil)

// TestMetrics_Catalog: bothy.audit.verify.failures is a counter in {run} and
// bothy.audit.verify.last_success.timestamp a gauge in s holding the last success in Unix
// seconds, both without attributes (docs/observability/signals.md "4.6").
func TestMetrics_Catalog(t *testing.T) {
	t.Parallel()
	reader := sdkmetric.NewManualReader()
	m, err := otelmetrics.New(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m.VerifyFailed(ctx)
	m.VerifyFailed(ctx)
	at := time.Date(2026, 10, 13, 8, 0, 0, 500_000_000, time.UTC)
	m.VerifySucceeded(ctx, at)
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatal(err)
	}
	got := map[string]metricdata.Metrics{}
	for _, sm := range rm.ScopeMetrics {
		for _, mm := range sm.Metrics {
			got[mm.Name] = mm
		}
	}
	failures, ok := got["bothy.audit.verify.failures"].Data.(metricdata.Sum[int64])
	if !ok || !failures.IsMonotonic || len(failures.DataPoints) != 1 || failures.DataPoints[0].Value != 2 ||
		failures.DataPoints[0].Attributes.Len() != 0 || got["bothy.audit.verify.failures"].Unit != "{run}" {
		t.Errorf("failures = %+v", got["bothy.audit.verify.failures"])
	}
	last, ok := got["bothy.audit.verify.last_success.timestamp"].Data.(metricdata.Gauge[float64])
	if !ok || len(last.DataPoints) != 1 || last.DataPoints[0].Value != 1791878400.5 ||
		last.DataPoints[0].Attributes.Len() != 0 || got["bothy.audit.verify.last_success.timestamp"].Unit != "s" {
		t.Errorf("last success = %+v", got["bothy.audit.verify.last_success.timestamp"])
	}
}
