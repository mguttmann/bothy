// SPDX-License-Identifier: Apache-2.0

package otelmetrics_test

import (
	"context"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/search/adapters/otelmetrics"
)

// TestSearchDurationMetric holds bothy.search.duration: one histogram point per outcome in
// seconds, with bothy.outcome as the only attribute; an unknown outcome counts as other.
func TestSearchDurationMetric(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	m, err := otelmetrics.New(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader), sdkmetric.WithView(metrics.View)))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m.SearchCompleted(ctx, "ok", 250*time.Millisecond)
	m.SearchCompleted(ctx, "timeout", 3*time.Second)
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatal(err)
	}
	for _, sm := range rm.ScopeMetrics {
		for _, met := range sm.Metrics {
			if met.Name != "bothy.search.duration" {
				continue
			}
			h, ok := met.Data.(metricdata.Histogram[float64])
			if !ok || len(h.DataPoints) != 2 || met.Unit != "s" {
				t.Fatalf("data %T %+v unit %s", met.Data, met.Data, met.Unit)
			}
			got := map[string]float64{}
			for _, dp := range h.DataPoints {
				if dp.Attributes.Len() != 1 {
					t.Fatalf("attributes %v, want bothy.outcome only", dp.Attributes)
				}
				v, _ := dp.Attributes.Value("bothy.outcome")
				got[v.AsString()] = dp.Sum
			}
			if got["ok"] != 0.25 || got["timeout"] != 3 {
				t.Fatalf("points %v", got)
			}
			return
		}
	}
	t.Fatal("bothy.search.duration not recorded")
}
