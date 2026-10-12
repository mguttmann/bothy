// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// TestFailedExportCountsExactly: every span and every log record of a failed export is counted
// once, under its own signal.
func TestFailedExportCountsExactly(t *testing.T) {
	const spans, records = 7, 5
	tt := setupTest(t, testConfig(), nil, failingSpanExporter{}, failingLogExporter{})
	ctx := context.Background()
	tracer := otelapi.Tracer("test")
	for range spans {
		_, s := tracer.Start(ctx, "span")
		s.End()
	}
	for range records {
		tt.Logger().InfoContext(ctx, "record")
	}
	tt.flush(t)

	got := tt.dropped(t)
	if got[signalTraces] != spans || got[signalLogs] != records || got[signalMetrics] != 0 {
		t.Errorf("dropped = %v, want traces %d, logs %d, metrics 0", got, spans, records)
	}
}

// TestQueueFullDropsAndCounts: with a budget of Q spans and an exporter that never returns, Q+K
// ended spans count at least K as dropped, and OnEnd never waits.
func TestQueueFullDropsAndCounts(t *testing.T) {
	const q, k = 4, 6
	exp := newBlockingSpanExporter(t)
	tt := setupTest(t, testConfig(), []string{"OTEL_BSP_MAX_QUEUE_SIZE=4"}, exp, nil)
	tracer := otelapi.Tracer("test")
	ctx := context.Background()
	for i := range q + k {
		_, s := tracer.Start(ctx, "span")
		start := time.Now()
		s.End()
		// A waiting End would wait for the exporter, which never returns.
		if d := time.Since(start); d > 500*time.Millisecond {
			t.Fatalf("End of span %d took %v", i, d)
		}
	}
	if got := tt.dropped(t)[signalTraces]; got < k {
		t.Errorf("dropped traces = %d, want at least %d", got, k)
	}
}

// TestShutdownRespectsDeadline: an exporter stuck in a call that ignores its context does not
// hold Shutdown beyond the deadline of its context.
func TestShutdownRespectsDeadline(t *testing.T) {
	exp := newBlockingSpanExporter(t)
	tt := setupTest(t, testConfig(), []string{"OTEL_BSP_MAX_QUEUE_SIZE=2"}, exp, nil)
	tracer := otelapi.Tracer("test")
	for range 4 {
		_, s := tracer.Start(context.Background(), "span")
		s.End()
	}
	select {
	case <-exp.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the exporter was never called")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := tt.Shutdown(ctx)
	if d := time.Since(start); d > time.Second {
		t.Errorf("Shutdown took %v with a 100ms deadline", d)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Shutdown error = %v, want the deadline", err)
	}
}

// TestResourceAttributesNotOverridden: OTEL_RESOURCE_ATTRIBUTES cannot replace the process
// attributes, but sets deployment.environment.name; without it instance.environment does.
func TestResourceAttributesNotOverridden(t *testing.T) {
	cfg := testConfig()
	cfg.Role = "api"
	cfg.Environment = "prod"
	for _, tc := range []struct {
		env, wantEnvironment string
	}{
		{"service.name=x,bothy.server.role=y,deployment.environment.name=staging", "staging"},
		{"", "prod"},
	} {
		t.Setenv("OTEL_RESOURCE_ATTRIBUTES", tc.env)
		tt := setupTest(t, cfg, nil, nil, nil)
		_, s := otelapi.Tracer("test").Start(context.Background(), "span")
		s.End()
		tt.flush(t)
		got := tt.spans.GetSpans()
		if len(got) != 1 {
			t.Fatalf("got %d spans, want 1", len(got))
		}
		set := got[0].Resource.Set()
		for key, want := range map[string]string{
			resServiceName: serviceName, resServiceNamespace: serviceNamespace, attr.ServerRole: "api",
			resEnvironment: tc.wantEnvironment, resServiceVersion: "1.2.3",
		} {
			if v, _ := set.Value(attribute.Key(key)); v.AsString() != want {
				t.Errorf("OTEL_RESOURCE_ATTRIBUTES=%q: %s = %q, want %q", tc.env, key, v.AsString(), want)
			}
		}
		if v, _ := set.Value(resServiceInstanceID); !strings.Contains(v.AsString(), "-") {
			t.Errorf("service.instance.id = %q, want host name and start time", v.AsString())
		}
		if err := tt.Shutdown(context.Background()); err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	}
}

func TestPseudonymKeyTooShortFailsSetup(t *testing.T) {
	_, err := Setup(context.Background(), testConfig(), Deps{Secrets: testSecrets(t, testKey[:31])})
	if !errors.Is(err, ErrKeyTooShort) {
		t.Errorf("Setup with a 31-byte key = %v, want ErrKeyTooShort", err)
	}
}

func TestSetupRejectsBadConfig(t *testing.T) {
	for name, mutate := range map[string]func(*Config){
		"ratio":    func(c *Config) { c.SamplerRatio = 1.5 },
		"exporter": func(c *Config) { c.LogsExporter = "file" },
		"level":    func(c *Config) { c.LogsLevel = "trace" },
		"rotation": func(c *Config) { c.PseudonymRotation = time.Millisecond },
	} {
		cfg := testConfig()
		mutate(&cfg)
		if _, err := Setup(context.Background(), cfg, Deps{Secrets: testSecrets(t, testKey)}); err == nil {
			t.Errorf("%s: Setup succeeded, want an error", name)
		}
	}
	if _, err := Setup(context.Background(), testConfig(), Deps{}); !errors.Is(err, ErrConfig) {
		t.Errorf("Setup without secrets = %v, want ErrConfig", err)
	}
	if _, err := Setup(context.Background(), testConfig(), Deps{Secrets: testSecrets(t, testKey), Env: []string{"OTEL_TRACES_EXPORTER=zipkin"}}); !errors.Is(err, ErrInvalidEnv) {
		t.Errorf("Setup with OTEL_TRACES_EXPORTER=zipkin = %v, want ErrInvalidEnv", err)
	}
}

// TestDisabledTelemetryKeepsStdoutLogs holds D1: without an OTLP log path the logs go to stdout,
// also when telemetry.logs.exporter is otlp, and never get lost.
func TestDisabledTelemetryKeepsStdoutLogs(t *testing.T) {
	// No OTLP exporter of this test talks to a collector.
	off := []string{"OTEL_TRACES_EXPORTER=none", "OTEL_METRICS_EXPORTER=none"}
	for name, tc := range map[string]struct {
		enabled, memLogs bool
		env              []string
	}{
		"telemetry disabled":       {enabled: false},
		"OTEL_LOGS_EXPORTER none":  {enabled: true, env: []string{"OTEL_LOGS_EXPORTER=none"}},
		"telemetry enabled (otlp)": {enabled: true, memLogs: true},
	} {
		cfg := testConfig()
		cfg.Enabled = tc.enabled
		cfg.LogsExporter = logsOTLP
		tt := &testTelemetry{stdout: &syncBuffer{}, logs: &memLogExporter{}}
		opts := setupOptions{}
		if tc.memLogs {
			opts.logExporter = tt.logs
		}
		env := append(append([]string{}, off...), tc.env...)
		tel, err := setup(context.Background(), cfg, Deps{Secrets: testSecrets(t, testKey), Stdout: tt.stdout, Env: env}, opts)
		if err != nil {
			t.Fatalf("%s: setup: %v", name, err)
		}
		tel.Logger().InfoContext(context.Background(), "probe")
		if err := tel.Shutdown(context.Background()); err != nil {
			t.Fatalf("%s: shutdown: %v", name, err)
		}
		onStdout := strings.Contains(tt.stdout.String(), `"msg":"probe"`)
		overOTLP := len(tt.logs.get()) == 1
		if wantStdout := name != "telemetry enabled (otlp)"; onStdout != wantStdout || overOTLP == wantStdout {
			t.Errorf("%s: stdout %v, OTLP %v", name, onStdout, overOTLP)
		}
	}
}

// TestErrorHandlerConstantAndRateLimited: SDK errors become one constant line per interval with
// the Go type of the error, never its text.
func TestErrorHandlerConstantAndRateLimited(t *testing.T) {
	t.Parallel()
	buf := &syncBuffer{}
	clk := clock.NewFake(time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC))
	h := &errorHandler{logger: NewCommandLogger(buf, slog.LevelWarn), clock: clk}
	h.Handle(errors.New("dial tcp canary.example.com:4317: refused"))
	h.Handle(errors.New("canary again"))
	clk.Advance(exportErrorInterval)
	h.Handle(errors.New("canary later"))
	out := buf.String()
	if strings.Contains(out, "canary") {
		t.Fatalf("error handler keeps the error text:\n%s", out)
	}
	recs := decodeLines(t, out)
	if len(recs) != 2 {
		t.Fatalf("got %d lines, want 2 (one per interval):\n%s", len(recs), out)
	}
	for _, rec := range recs {
		if rec["msg"] != msgExportError || rec[attr.ErrorType] != "*errors.errorString" {
			t.Errorf("line = %v", rec)
		}
	}
}
