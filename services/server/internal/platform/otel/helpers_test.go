// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// testKeyVar is the variable the test secrets provider resolves telemetry.pseudonym_key_ref from.
const testKeyVar = "BOTHY_TELEMETRY_TEST_KEY"

// testKey is a 32-byte master key of the pseudonyms.
var testKey = []byte("0123456789abcdef0123456789abcdef")

// testSecrets returns a provider whose env driver holds key under testKeyVar.
func testSecrets(t *testing.T, key []byte) *secrets.Provider {
	t.Helper()
	p, err := secrets.NewProvider(secrets.Options{
		Dir:   t.TempDir(),
		Clock: clock.System(),
		Env:   []string{testKeyVar + "=" + string(key)},
	})
	if err != nil {
		t.Fatalf("secrets provider: %v", err)
	}
	return p
}

// testConfig is the default configuration of the chapter tables with the test key.
func testConfig() Config {
	return Config{
		Enabled:           true,
		SamplerRatio:      1,
		LogsExporter:      logsBoth,
		LogsLevel:         "info",
		PseudonymKeyRef:   "env:" + testKeyVar,
		PseudonymRotation: 720 * time.Hour,
		Role:              "all",
		Environment:       "prod",
	}
}

// syncBuffer is a bytes.Buffer for writers on several goroutines.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// testTelemetry is a Setup with in-memory or fake exporters and a manual metric reader.
type testTelemetry struct {
	*Telemetry
	stdout *syncBuffer
	spans  *tracetest.InMemoryExporter
	logs   *memLogExporter
	reader *sdkmetric.ManualReader
}

// setupTest calls setup with the given exporters (nil: in-memory ones) and shuts down at the
// end of the test. It installs globals, so tests that call it never run in parallel. The OTLP
// metric exporter is off, so no test talks to a collector it did not open.
func setupTest(t *testing.T, cfg Config, env []string, spanExp sdktrace.SpanExporter, logExp sdklog.Exporter) *testTelemetry {
	t.Helper()
	env = append([]string{"OTEL_METRICS_EXPORTER=none"}, env...)
	tt := &testTelemetry{stdout: &syncBuffer{}, reader: sdkmetric.NewManualReader()}
	if spanExp == nil {
		tt.spans = tracetest.NewInMemoryExporter()
		spanExp = tt.spans
	}
	if logExp == nil {
		tt.logs = &memLogExporter{}
		logExp = tt.logs
	}
	tel, err := setup(context.Background(), cfg, Deps{
		Secrets:        testSecrets(t, testKey),
		Stdout:         tt.stdout,
		ServiceVersion: "1.2.3",
		Env:            env,
	}, setupOptions{spanExporter: spanExp, logExporter: logExp, metricReader: tt.reader})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	tt.Telemetry = tel
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = tel.Shutdown(ctx)
	})
	return tt
}

// flush calls ForceFlush with a deadline and ignores the errors of failing exporters.
func (tt *testTelemetry) flush(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = tt.ForceFlush(ctx)
}

// dropped reads bothy.telemetry.dropped per bothy.signal from the manual reader.
func (tt *testTelemetry) dropped(t *testing.T) map[string]int64 {
	t.Helper()
	got := map[string]int64{}
	for _, dp := range sumPoints(t, tt.reader, metricDropped) {
		s, _ := dp.Attributes.Value(attribute.Key(attr.Signal))
		got[s.AsString()] += dp.Value
	}
	return got
}

// sumPoints returns the data points of the int64 sum name.
func sumPoints(t *testing.T, r *sdkmetric.ManualReader, name string) []metricdata.DataPoint[int64] {
	t.Helper()
	m := findMetric(t, r, name)
	if m == nil {
		return nil
	}
	s, ok := m.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("%s is %T, want an int64 sum", name, m.Data)
	}
	return s.DataPoints
}

func findMetric(t *testing.T, r *sdkmetric.ManualReader, name string) *metricdata.Metrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := r.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	for _, sm := range rm.ScopeMetrics {
		for i := range sm.Metrics {
			if sm.Metrics[i].Name == name {
				return &sm.Metrics[i]
			}
		}
	}
	return nil
}

// memLogExporter keeps copies of the exported log records.
type memLogExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *memLogExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range records {
		e.records = append(e.records, records[i].Clone())
	}
	return nil
}

func (*memLogExporter) ForceFlush(context.Context) error { return nil }
func (*memLogExporter) Shutdown(context.Context) error   { return nil }

func (e *memLogExporter) get() []sdklog.Record {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]sdklog.Record(nil), e.records...)
}

// dumpRecord writes body and attributes of a log record as text.
func dumpRecord(r *sdklog.Record) string {
	var b strings.Builder
	fmt.Fprintf(&b, "body=%s;", r.Body().String())
	r.WalkAttributes(func(kv attribute.KeyValue) bool {
		fmt.Fprintf(&b, "%s=%s;", kv.Key, kv.Value.String())
		return true
	})
	return b.String()
}

// recordAttr returns the value of key in a log record.
func recordAttr(r *sdklog.Record, key string) (attribute.Value, bool) {
	var v attribute.Value
	found := false
	r.WalkAttributes(func(kv attribute.KeyValue) bool {
		if string(kv.Key) == key {
			v, found = kv.Value, true
			return false
		}
		return true
	})
	return v, found
}

// dumpSpan writes everything of a span a telemetry backend would see, except the resource.
func dumpSpan(s tracetest.SpanStub) string {
	var b strings.Builder
	fmt.Fprintf(&b, "name=%s;status=%s;tracestate=%s;parent.tracestate=%s;", s.Name, s.Status.Description,
		s.SpanContext.TraceState().String(), s.Parent.TraceState().String())
	for _, kv := range s.Attributes {
		fmt.Fprintf(&b, "%s=%s;", kv.Key, kv.Value.String())
	}
	for _, e := range s.Events {
		fmt.Fprintf(&b, "event=%s;", e.Name)
		for _, kv := range e.Attributes {
			fmt.Fprintf(&b, "%s=%s;", kv.Key, kv.Value.String())
		}
	}
	for _, l := range s.Links {
		fmt.Fprintf(&b, "link.tracestate=%s;", l.SpanContext.TraceState().String())
		for _, kv := range l.Attributes {
			fmt.Fprintf(&b, "link.%s=%s;", kv.Key, kv.Value.String())
		}
	}
	return b.String()
}

// spanAttr returns the value of key on a span.
func spanAttr(s tracetest.SpanStub, key string) (attribute.Value, bool) {
	for _, kv := range s.Attributes {
		if string(kv.Key) == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

var errExport = errors.New("export failed")

// failingSpanExporter fails every export.
type failingSpanExporter struct{}

func (failingSpanExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error {
	return errExport
}
func (failingSpanExporter) Shutdown(context.Context) error { return nil }

// failingLogExporter fails every export.
type failingLogExporter struct{}

func (failingLogExporter) Export(context.Context, []sdklog.Record) error { return errExport }
func (failingLogExporter) ForceFlush(context.Context) error              { return nil }
func (failingLogExporter) Shutdown(context.Context) error                { return nil }

// blockingSpanExporter blocks every export until release is closed, ignoring the context like a
// stuck network call.
type blockingSpanExporter struct {
	release chan struct{}
	started chan struct{}
	once    sync.Once
}

func newBlockingSpanExporter(t *testing.T) *blockingSpanExporter {
	e := &blockingSpanExporter{release: make(chan struct{}), started: make(chan struct{})}
	t.Cleanup(func() { close(e.release) })
	return e
}

func (e *blockingSpanExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error {
	e.once.Do(func() { close(e.started) })
	<-e.release
	return nil
}
func (*blockingSpanExporter) Shutdown(context.Context) error { return nil }

func newManualReader() *sdkmetric.ManualReader { return sdkmetric.NewManualReader() }
