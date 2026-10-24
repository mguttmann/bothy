// SPDX-License-Identifier: Apache-2.0

package otelmetrics_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
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

// TestScanMetrics holds the scan instruments: bothy.file.scan per outcome, bothy.file.scan.duration
// with the long-duration buckets of the registry and no attribute but bothy.outcome, and
// bothy.file.scan.queue without attributes.
func TestScanMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader), sdkmetric.WithView(metrics.View))
	m, err := otelmetrics.New(mp)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m.ScanCompleted(ctx, "clean", 3*time.Second)
	m.ScanCompleted(ctx, "infected", time.Second)
	m.ScanCompleted(ctx, "clean", time.Second)
	m.ScanQueue(ctx, 7)
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, sm := range rm.ScopeMetrics {
		for _, met := range sm.Metrics {
			seen[met.Name] = true
			switch met.Name {
			case "bothy.file.scan":
				sum := met.Data.(metricdata.Sum[int64])
				counts := map[string]int64{}
				for _, dp := range sum.DataPoints {
					if dp.Attributes.Len() != 1 {
						t.Errorf("attributes %v", dp.Attributes)
					}
					v, _ := dp.Attributes.Value(attribute.Key("bothy.outcome"))
					counts[v.AsString()] = dp.Value
				}
				if counts["clean"] != 2 || counts["infected"] != 1 {
					t.Errorf("counts %v", counts)
				}
			case "bothy.file.scan.duration":
				h := met.Data.(metricdata.Histogram[float64])
				for _, dp := range h.DataPoints {
					if dp.Attributes.Len() != 1 || !slices.Equal(dp.Bounds, metrics.BucketsLong) {
						t.Errorf("point %v bounds %v", dp.Attributes, dp.Bounds)
					}
				}
			case "bothy.file.scan.queue":
				g := met.Data.(metricdata.Gauge[float64])
				if len(g.DataPoints) != 1 || g.DataPoints[0].Value != 7 || g.DataPoints[0].Attributes.Len() != 0 {
					t.Errorf("queue %+v", g.DataPoints)
				}
			}
		}
	}
	for _, n := range []string{"bothy.file.scan", "bothy.file.scan.duration", "bothy.file.scan.queue"} {
		if !seen[n] {
			t.Errorf("%s not recorded", n)
		}
	}
}

// TestPurgeBacklogMetric holds bothy.file.purge.backlog: nothing before the first record, then the
// last recorded value without attributes, unit {file}.
func TestPurgeBacklogMetric(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	m, err := otelmetrics.NewPurgeBacklog(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	collect := func() []metricdata.DataPoint[float64] {
		var rm metricdata.ResourceMetrics
		if err := reader.Collect(ctx, &rm); err != nil {
			t.Fatal(err)
		}
		for _, sm := range rm.ScopeMetrics {
			for _, met := range sm.Metrics {
				if met.Name == "bothy.file.purge.backlog" {
					if met.Unit != "{file}" {
						t.Errorf("unit %s", met.Unit)
					}
					return met.Data.(metricdata.Gauge[float64]).DataPoints
				}
			}
		}
		return nil
	}
	if dps := collect(); len(dps) != 0 {
		t.Fatalf("before the first run %+v", dps)
	}
	m.PurgeBacklog(ctx, 4)
	m.PurgeBacklog(ctx, 3)
	dps := collect()
	if len(dps) != 1 || dps[0].Value != 3 || dps[0].Attributes.Len() != 0 {
		t.Fatalf("points %+v", dps)
	}
}
