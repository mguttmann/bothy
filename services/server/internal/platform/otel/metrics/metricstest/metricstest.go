// SPDX-License-Identifier: Apache-2.0

// Package metricstest holds the test helpers of the metric registry: the strict switch and a
// meter provider with the server's view over an in-process ManualReader. Only tests import it;
// tests never read metrics from a running collector.
//
// Example:
//
//	func TestX(t *testing.T) {
//		metricstest.Strict(t)
//		mp, reader := metricstest.Provider(t)
//		// record through instruments created on mp
//		m := metricstest.Find(t, reader, "bothy.gateway.close")
//	}
package metricstest

import (
	"context"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
)

// Strict turns the strict switch of package metrics on for the rest of t: an attribute value
// outside its closed set, or a value tuple of the wrong length, fails t at its end. The switch
// is process-wide, so tests that call Strict must not run in parallel with tests that expect the
// production behaviour.
func Strict(t testing.TB) {
	t.Helper()
	var mu sync.Mutex
	var found []error
	restore := metrics.SetStrict(func(err error) {
		mu.Lock()
		defer mu.Unlock()
		found = append(found, err)
	})
	t.Cleanup(func() {
		restore()
		mu.Lock()
		defer mu.Unlock()
		for _, err := range found {
			t.Errorf("strict metrics: %v", err)
		}
	})
}

// Provider returns a meter provider with the view of the server (metrics.View) and the manual
// reader it reads into; the provider shuts down at the end of t.
func Provider(t testing.TB) (*sdkmetric.MeterProvider, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader), sdkmetric.WithView(metrics.View))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	return mp, reader
}

// Collect reads every metric of reader by name.
func Collect(t testing.TB, reader *sdkmetric.ManualReader) map[string]metricdata.Metrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	out := map[string]metricdata.Metrics{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			out[m.Name] = m
		}
	}
	return out
}

// Find returns the metric name of reader; t fails when there is none.
func Find(t testing.TB, reader *sdkmetric.ManualReader, name string) metricdata.Metrics {
	t.Helper()
	m, ok := Collect(t, reader)[name]
	if !ok {
		t.Fatalf("metric %s not collected", name)
	}
	return m
}

// Point is one data point of a metric reduced to its attributes and value: the sum of a counter
// or gauge point, the count of a histogram point (Sum holds its sum then).
type Point struct {
	Attrs map[string]string
	Value float64
	Count uint64
	Sum   float64
	// Bounds are the bucket bounds of a histogram point.
	Bounds []float64
}

// Points returns the data points of m.
func Points(m metricdata.Metrics) []Point {
	var out []Point
	switch d := m.Data.(type) {
	case metricdata.Sum[int64]:
		for _, p := range d.DataPoints {
			out = append(out, Point{Attrs: toMap(p.Attributes.ToSlice()), Value: float64(p.Value)})
		}
	case metricdata.Sum[float64]:
		for _, p := range d.DataPoints {
			out = append(out, Point{Attrs: toMap(p.Attributes.ToSlice()), Value: p.Value})
		}
	case metricdata.Gauge[int64]:
		for _, p := range d.DataPoints {
			out = append(out, Point{Attrs: toMap(p.Attributes.ToSlice()), Value: float64(p.Value)})
		}
	case metricdata.Gauge[float64]:
		for _, p := range d.DataPoints {
			out = append(out, Point{Attrs: toMap(p.Attributes.ToSlice()), Value: p.Value})
		}
	case metricdata.Histogram[float64]:
		for _, p := range d.DataPoints {
			out = append(out, Point{Attrs: toMap(p.Attributes.ToSlice()), Count: p.Count, Sum: p.Sum, Bounds: p.Bounds})
		}
	case metricdata.Histogram[int64]:
		for _, p := range d.DataPoints {
			out = append(out, Point{Attrs: toMap(p.Attributes.ToSlice()), Count: p.Count, Sum: float64(p.Sum), Bounds: p.Bounds})
		}
	}
	return out
}

// toMap returns the attributes as strings by key.
func toMap(kvs []attribute.KeyValue) map[string]string {
	out := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		out[string(kv.Key)] = kv.Value.String()
	}
	return out
}

// Sum returns the sum of the values (counters, gauges) or counts (histograms) of the points of m
// whose attributes contain want.
func Sum(m metricdata.Metrics, want map[string]string) float64 {
	var total float64
	for _, p := range Points(m) {
		if !contains(p.Attrs, want) {
			continue
		}
		if p.Count > 0 || p.Bounds != nil {
			total += float64(p.Count)
			continue
		}
		total += p.Value
	}
	return total
}

// contains reports whether have holds every entry of want.
func contains(have, want map[string]string) bool {
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}
