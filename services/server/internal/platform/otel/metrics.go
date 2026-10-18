// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"

	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
)

// Values of bothy.signal.
const (
	signalTraces  = "traces"
	signalMetrics = "metrics"
	signalLogs    = "logs"
)

// metricDropped is the name of bothy.telemetry.dropped; its declaration is
// metrics.TelemetryDropped.
var metricDropped = metrics.TelemetryDropped.Name

// dropCounter is bothy.telemetry.dropped (signals.md 4.6): items of a signal that never
// reached the collector, from a full front queue or a failed export. It is cumulative, so the
// count is visible as soon as the collector is reachable again.
type dropCounter struct {
	counter *metrics.Counter
}

func newDropCounter(mp metric.MeterProvider) (*dropCounter, error) {
	c, err := metrics.NewCounter(mp, metrics.TelemetryDropped, nil)
	if err != nil {
		return nil, err
	}
	return &dropCounter{counter: c}, nil
}

// add counts n dropped items of signal.
func (d *dropCounter) add(signal string, n int64) {
	if n > 0 {
		d.counter.Add(context.Background(), n, signal)
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
