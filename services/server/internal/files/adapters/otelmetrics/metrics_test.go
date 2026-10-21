// SPDX-License-Identifier: Apache-2.0

package otelmetrics_test

import (
	"context"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/otelmetrics"
)

// TestUploadSizeMetric holds bothy.file.upload.size: one histogram point per completion, the
// declared size, without attributes (in-process reader).
func TestUploadSizeMetric(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	m, err := otelmetrics.New(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	if err != nil {
		t.Fatal(err)
	}
	m.UploadCompleted(context.Background(), 48213)
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	for _, sm := range rm.ScopeMetrics {
		for _, met := range sm.Metrics {
			if met.Name != "bothy.file.upload.size" {
				continue
			}
			h, ok := met.Data.(metricdata.Histogram[float64])
			if !ok || len(h.DataPoints) != 1 {
				t.Fatalf("data %T %+v", met.Data, met.Data)
			}
			dp := h.DataPoints[0]
			if dp.Count != 1 || dp.Sum != 48213 || dp.Attributes.Len() != 0 || met.Unit != "By" {
				t.Fatalf("point %+v unit %s", dp, met.Unit)
			}
			return
		}
	}
	t.Fatal("bothy.file.upload.size not recorded")
}
