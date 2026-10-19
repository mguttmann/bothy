// SPDX-License-Identifier: Apache-2.0

package metrics_test

import (
	"context"
	"errors"
	"math"
	"slices"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
)

// aggregatedReader returns a manual reader that also reads set, the in-memory exporter side of
// the histogram path (M1-10 readers, brief M4-31 Design note 3).
func aggregatedReader(t *testing.T, set *metrics.AggregatedSet) *sdkmetric.ManualReader {
	t.Helper()
	reader := sdkmetric.NewManualReader(sdkmetric.WithProducer(set))
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader), sdkmetric.WithView(metrics.View))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	return reader
}

// histogramPoint returns the one cumulative data point of the histogram name.
func histogramPoint(t *testing.T, reader *sdkmetric.ManualReader, name string) metricdata.HistogramDataPoint[float64] {
	t.Helper()
	m := metricstest.Find(t, reader, name)
	h, ok := m.Data.(metricdata.Histogram[float64])
	if !ok {
		t.Fatalf("%s is a %T, want a float64 histogram", name, m.Data)
	}
	if h.Temporality != metricdata.CumulativeTemporality || len(h.DataPoints) != 1 {
		t.Fatalf("%s: temporality %v with %d points, want one cumulative point", name, h.Temporality, len(h.DataPoints))
	}
	return h.DataPoints[0]
}

// TestAggregated_HistogramPath is the histogram path test of brief M4-31 Design note 3: the
// buckets two clients sent arrive in the exporter as one cumulative OTLP histogram point with the
// bounds of signals.md 4.7, the exact count per bound and the total count; the sum counts every
// value at the upper bound of its bucket (an upper bound of the true sum, which the clients do not
// send).
func TestAggregated_HistogramPath(t *testing.T) {
	metricstest.Strict(t)
	set := metrics.NewAggregatedSet()
	reader := aggregatedReader(t, set)
	h, err := set.Histogram(metrics.ClientThreadOpenDuration, nil)
	if err != nil {
		t.Fatal(err)
	}
	attrs := []string{"ios", "touch", "", "1"}
	if err := h.Add([]metrics.Bucket{{LE: 0.1, Count: 12}, {LE: 0.25, Count: 3}}, attrs...); err != nil {
		t.Fatal(err)
	}
	if err := h.Add([]metrics.Bucket{{LE: 0.1, Count: 1}, {LE: 10, Count: 2}, {LE: 0.005, Count: 0}}, attrs...); err != nil {
		t.Fatal(err)
	}
	p := histogramPoint(t, reader, "bothy.client.thread_open.duration")
	if !slices.Equal(p.Bounds, metrics.BucketsSeconds) {
		t.Errorf("bounds %v, want the 4.7 row %v", p.Bounds, metrics.BucketsSeconds)
	}
	want := make([]uint64, len(metrics.BucketsSeconds)+1)
	want[slices.Index(metrics.BucketsSeconds, 0.1)] = 13
	want[slices.Index(metrics.BucketsSeconds, 0.25)] = 3
	want[slices.Index(metrics.BucketsSeconds, 10)] = 2
	if !slices.Equal(p.BucketCounts, want) {
		t.Errorf("bucket counts %v, want %v", p.BucketCounts, want)
	}
	if p.Count != 18 {
		t.Errorf("count %d, want 18", p.Count)
	}
	if wantSum := 13*0.1 + 3*0.25 + 2*10.0; math.Abs(p.Sum-wantSum) > 1e-9 {
		t.Errorf("sum %v, want %v (every value at its upper bound)", p.Sum, wantSum)
	}
	if _, ok := p.Min.Value(); ok {
		t.Error("min is set; the buckets do not give it")
	}
	if _, ok := p.Max.Value(); ok {
		t.Error("max is set; the buckets do not give it")
	}
	got := map[string]string{}
	for _, kv := range p.Attributes.ToSlice() {
		got[string(kv.Key)] = kv.Value.AsString()
	}
	if len(got) != 3 || got[metrics.KeyClientPlatform] != "ios" || got[metrics.KeyClientDensity] != "touch" || got[metrics.KeyClientAppMajor] != "1" {
		t.Errorf("attributes %v; want platform, density and app major only (theme absent)", got)
	}
	if p.StartTime.IsZero() || p.Time.Before(p.StartTime) {
		t.Errorf("start %v, time %v", p.StartTime, p.Time)
	}
}

// TestAggregated_RefusesForeignBounds: a bound outside the spec's row, a repeated bound or a
// count above the int64 range of the contract refuses the whole entry and records nothing.
func TestAggregated_RefusesForeignBounds(t *testing.T) {
	set := metrics.NewAggregatedSet()
	reader := aggregatedReader(t, set)
	h, err := set.Histogram(metrics.ClientColdStartDuration, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, b := range map[string][]metrics.Bucket{
		"foreign bound": {{LE: 0.1, Count: 1}, {LE: 0.3, Count: 1}},
		"repeated":      {{LE: 0.1, Count: 1}, {LE: 0.1, Count: 1}},
		"infinite":      {{LE: math.Inf(1), Count: 1}},
		"no buckets":    nil,
	} {
		if err := h.Add(b, "macos", "", "", "2"); !errors.Is(err, metrics.ErrBounds) {
			t.Errorf("%s: err %v, want ErrBounds", name, err)
		}
	}
	if m := metricstest.Collect(t, reader); len(m) != 0 {
		t.Errorf("a refused entry was exported: %v", m)
	}
}

// TestAggregated_SaturatesCounts: counts that would overflow saturate instead of wrapping, so a
// hostile batch cannot turn a large count into a small one.
func TestAggregated_SaturatesCounts(t *testing.T) {
	set := metrics.NewAggregatedSet()
	reader := aggregatedReader(t, set)
	h, err := set.Histogram(metrics.ClientCatchupDuration, nil)
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := h.Add([]metrics.Bucket{{LE: 1, Count: math.MaxUint64 / 2}}, "windows", "compact", "dark", "3"); err != nil {
			t.Fatal(err)
		}
	}
	p := histogramPoint(t, reader, "bothy.client.catchup.duration")
	if p.Count != math.MaxUint64 || p.BucketCounts[slices.Index(metrics.BucketsSeconds, 1)] != math.MaxUint64 {
		t.Errorf("count %d, bucket %v; want saturation", p.Count, p.BucketCounts)
	}
}

// TestAggregated_AttributesAndSpecs: an unknown attribute value records as "other" (and the strict
// switch reports it), a spec that is no histogram is refused, and two histograms keep separate
// points per attribute set.
func TestAggregated_AttributesAndSpecs(t *testing.T) {
	set := metrics.NewAggregatedSet()
	reader := aggregatedReader(t, set)
	if _, err := set.Histogram(metrics.ClientFrames, nil); !errors.Is(err, metrics.ErrSpec) {
		t.Fatalf("a counter spec: err %v, want ErrSpec", err)
	}
	h, err := set.Histogram(metrics.ClientSendLocalDuration, nil)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := set.Histogram(metrics.ClientSendLocalDuration, nil); err != nil || again != h {
		t.Fatalf("a second Histogram of the same spec = %p, %v; want the first", again, err)
	}
	if err := h.Add([]metrics.Bucket{{LE: 0.05, Count: 4}}, "ios", "", "", "123"); err != nil {
		t.Fatal(err)
	}
	if err := h.Add([]metrics.Bucket{{LE: 0.05, Count: 1}}, "android", "", "", "7"); err != nil {
		t.Fatal(err)
	}
	m := metricstest.Find(t, reader, "bothy.client.send_local.duration")
	points := metricstest.Points(m)
	if len(points) != 2 {
		t.Fatalf("%d points, want 2", len(points))
	}
	majors := []string{points[0].Attrs[metrics.KeyClientAppMajor], points[1].Attrs[metrics.KeyClientAppMajor]}
	slices.Sort(majors)
	if !slices.Equal(majors, []string{"7", "other"}) {
		t.Errorf("app majors %v, want [7 other]", majors)
	}
}

// TestAggregated_Default: the process set exists once; otel.Setup attaches it to the OTLP reader.
func TestAggregated_Default(t *testing.T) {
	if metrics.DefaultAggregated() == nil || metrics.DefaultAggregated() != metrics.DefaultAggregated() {
		t.Fatal("DefaultAggregated is not one set per process")
	}
}
