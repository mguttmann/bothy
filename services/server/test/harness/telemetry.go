// SPDX-License-Identifier: Apache-2.0

package harness

import (
	"context"
	"errors"
	"sync"

	otelapi "go.opentelemetry.io/otel"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/OWNER/bothy/services/server/internal/platform/otel"
)

// TelemetrySinks are the in-process exporters of a harness server (M1-69): spans, OTLP log
// records and a manual metric reader, in place of the OTLP exporters (otel.Exporters). Start
// with Options.Telemetry turns telemetry.enabled on unless Options.Config sets it.
//
// otel.Setup installs the global tracer, meter and logger providers, so a test that sets
// Telemetry must not run in parallel with another harness start of the same test binary.
type TelemetrySinks struct {
	// Spans holds every exported span until the server stops (its Shutdown clears it).
	Spans *tracetest.InMemoryExporter
	// Logs holds every exported OTLP log record.
	Logs *MemoryLogExporter
	// Metrics is collected on demand (Collect of sdkmetric.ManualReader).
	Metrics *sdkmetric.ManualReader
}

// NewTelemetrySinks returns empty sinks.
func NewTelemetrySinks() *TelemetrySinks {
	return &TelemetrySinks{Spans: tracetest.NewInMemoryExporter(), Logs: &MemoryLogExporter{}, Metrics: sdkmetric.NewManualReader()}
}

// exporters returns the sinks as otel.Exporters.
func (t *TelemetrySinks) exporters() otel.Exporters {
	return otel.Exporters{Span: t.Spans, Log: t.Logs, MetricReader: t.Metrics}
}

// MemoryLogExporter is an sdklog.Exporter that keeps copies of every record. It keeps them after
// Shutdown, so a test reads them after the server stopped. It is safe for concurrent use.
type MemoryLogExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

// Export keeps copies of records.
func (e *MemoryLogExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, r := range records {
		e.records = append(e.records, r.Clone())
	}
	return nil
}

// ForceFlush does nothing; the records are kept at Export.
func (*MemoryLogExporter) ForceFlush(context.Context) error { return nil }

// Shutdown does nothing; the records stay readable.
func (*MemoryLogExporter) Shutdown(context.Context) error { return nil }

// Records returns copies of the kept records in export order.
func (e *MemoryLogExporter) Records() []sdklog.Record {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]sdklog.Record, len(e.records))
	for i, r := range e.records {
		out[i] = r.Clone()
	}
	return out
}

// FlushTelemetry exports what the global tracer and logger providers of the server still hold,
// within ctx, so the sinks of Options.Telemetry are complete. It reads the globals otel.Setup
// installed; with telemetry off there is nothing to flush.
func (s *Server) FlushTelemetry(ctx context.Context) error {
	var errs []error
	if tp, ok := otelapi.GetTracerProvider().(*sdktrace.TracerProvider); ok {
		errs = append(errs, tp.ForceFlush(ctx))
	}
	if lp, ok := otelapi.GetLoggerProvider().(*sdklog.LoggerProvider); ok {
		errs = append(errs, lp.ForceFlush(ctx))
	}
	return errors.Join(errs...)
}
