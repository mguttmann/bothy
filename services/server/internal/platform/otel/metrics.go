// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// Names of signals.md 4.1 and 4.6.
const (
	metricDropped             = "bothy.telemetry.dropped"
	metricHTTPRequestDuration = "http.server.request.duration"
)

// Values of bothy.signal.
const (
	signalTraces  = "traces"
	signalMetrics = "metrics"
	signalLogs    = "logs"
)

// durationBuckets are the boundaries "Durations up to seconds" of signals.md 4.7.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

// httpMetricKeys are the attributes of http.server.request.duration in signals.md 4.1.
var httpMetricKeys = []attribute.Key{
	attribute.Key(attr.HTTPRequestMethod), attribute.Key(attr.HTTPRoute),
	attribute.Key(attr.HTTPResponseStatusCode), attribute.Key(attr.ErrorType),
}

// httpView keeps of the otelhttp instruments only http.server.request.duration, with exactly
// the attributes of the catalog and the boundaries of 4.7 (signals.md "Cardinality" item 5:
// otelhttp also sets server.address from the Host header of the client, an open set). The body
// size histograms are not in the catalog and are dropped.
func httpView(i sdkmetric.Instrument) (sdkmetric.Stream, bool) {
	if i.Scope.Name != otelhttp.ScopeName {
		return sdkmetric.Stream{}, false
	}
	if i.Name != metricHTTPRequestDuration {
		return sdkmetric.Stream{Aggregation: sdkmetric.AggregationDrop{}}, true
	}
	return sdkmetric.Stream{
		Name:            i.Name,
		Description:     i.Description,
		Unit:            i.Unit,
		Aggregation:     sdkmetric.AggregationExplicitBucketHistogram{Boundaries: durationBuckets},
		AttributeFilter: attribute.NewAllowKeysFilter(httpMetricKeys...),
	}, true
}

// dropCounter is bothy.telemetry.dropped (signals.md 4.6): items of a signal that never
// reached the collector, from a full front queue or a failed export. It is cumulative, so the
// count is visible as soon as the collector is reachable again.
type dropCounter struct {
	counter metric.Int64Counter
	sets    map[string]metric.AddOption
}

func newDropCounter(mp metric.MeterProvider) (*dropCounter, error) {
	c, err := mp.Meter(instrumentationName).Int64Counter(metricDropped,
		metric.WithUnit("{item}"),
		metric.WithDescription("Telemetry items that never reached the collector"))
	if err != nil {
		return nil, err
	}
	d := &dropCounter{counter: c, sets: make(map[string]metric.AddOption)}
	for _, s := range []string{signalTraces, signalMetrics, signalLogs} {
		d.sets[s] = metric.WithAttributeSet(attribute.NewSet(attribute.String(attr.Signal, s)))
	}
	return d, nil
}

// add counts n dropped items of signal.
func (d *dropCounter) add(signal string, n int64) {
	if n > 0 {
		d.counter.Add(context.Background(), n, d.sets[signal])
	}
}

// countingMetricExporter counts every data point of a failed export in bothy.telemetry.dropped
// and keeps the temporality cumulative for every instrument (D2, the Prometheus path; the
// collector converts per backend).
type countingMetricExporter struct {
	next    sdkmetric.Exporter
	dropped func(int64)
}

// Temporality is cumulative for every instrument kind.
func (countingMetricExporter) Temporality(sdkmetric.InstrumentKind) metricdata.Temporality {
	return metricdata.CumulativeTemporality
}

// Aggregation is the aggregation of the next exporter.
func (e countingMetricExporter) Aggregation(k sdkmetric.InstrumentKind) sdkmetric.Aggregation {
	return e.next.Aggregation(k)
}

// Export exports and counts the data points of a failed export.
func (e countingMetricExporter) Export(ctx context.Context, rm *metricdata.ResourceMetrics) error {
	err := e.next.Export(ctx, rm)
	if err != nil {
		e.dropped(dataPoints(rm))
	}
	return err
}

// ForceFlush flushes the next exporter.
func (e countingMetricExporter) ForceFlush(ctx context.Context) error {
	return e.next.ForceFlush(ctx)
}

// Shutdown shuts the next exporter down.
func (e countingMetricExporter) Shutdown(ctx context.Context) error {
	return e.next.Shutdown(ctx)
}

// dataPoints counts the data points of rm.
func dataPoints(rm *metricdata.ResourceMetrics) int64 {
	var n int
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch d := m.Data.(type) {
			case metricdata.Gauge[int64]:
				n += len(d.DataPoints)
			case metricdata.Gauge[float64]:
				n += len(d.DataPoints)
			case metricdata.Sum[int64]:
				n += len(d.DataPoints)
			case metricdata.Sum[float64]:
				n += len(d.DataPoints)
			case metricdata.Histogram[int64]:
				n += len(d.DataPoints)
			case metricdata.Histogram[float64]:
				n += len(d.DataPoints)
			case metricdata.ExponentialHistogram[int64]:
				n += len(d.DataPoints)
			case metricdata.ExponentialHistogram[float64]:
				n += len(d.DataPoints)
			case metricdata.Summary:
				n += len(d.DataPoints)
			}
		}
	}
	return int64(n)
}
