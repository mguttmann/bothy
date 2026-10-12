// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/exemplar"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// instrumentationName is the instrumentation scope of the meter and logger of this package.
const instrumentationName = "github.com/OWNER/bothy/services/server/internal/platform/otel"

// Values of telemetry.logs.exporter.
const (
	logsOTLP   = "otlp"
	logsStdout = "stdout"
	logsBoth   = "both"
)

// msgExportError is the constant message of the OpenTelemetry error handler.
const msgExportError = "telemetry export error"

// exportErrorInterval is the shortest gap between two lines of the error handler: a collector
// that is down fails every export, and one line per interval says so.
const exportErrorInterval = 10 * time.Second

// ErrConfig reports a Config or Deps that Setup cannot use. The message names the key or field,
// never a value.
var ErrConfig = errors.New("otel: invalid telemetry configuration")

// Deps are the dependencies of Setup.
type Deps struct {
	// Secrets resolves telemetry.pseudonym_key_ref; required.
	Secrets *secrets.Provider
	// Clock is the source of now (service.instance.id, pseudonyms, the error handler); nil
	// means clock.System().
	Clock clock.Clock
	// Stdout receives the JSON log lines; nil means os.Stdout.
	Stdout io.Writer
	// ServiceVersion is service.version, buildinfo.Version() in the server.
	ServiceVersion string
	// Env is the process environment as cmd/bothy read it ("NAME=value" entries). Setup reads
	// the OTEL_* variables it interprets itself from it (protocol, exporter none, queue sizes),
	// because no package outside cmd/bothy reads the environment. The OTLP exporters and
	// OTEL_RESOURCE_ATTRIBUTES still read the process environment, which is the same in the
	// server.
	Env []string
}

// Telemetry is the telemetry of a running process, built by Setup. Its methods are safe for
// concurrent use.
type Telemetry struct {
	logger  *slog.Logger
	pseudo  *Pseudonymizer
	tracer  *sdktrace.TracerProvider
	meter   *sdkmetric.MeterProvider
	logs    *sdklog.LoggerProvider
	drops   atomic.Pointer[dropCounter]
	stopped sync.Once
}

// setupOptions replace the OTLP side in tests: spanExporter and logExporter instead of the OTLP
// exporters, metricReader as an additional reader of the meter provider.
type setupOptions struct {
	spanExporter sdktrace.SpanExporter
	logExporter  sdklog.Exporter
	metricReader sdkmetric.Reader
}

// Setup builds the telemetry of the server process and installs it globally: the tracer,
// meter and logger providers and the W3C TraceContext propagator (no baggage). Spans, metrics
// and logs go over OTLP to the collector, asynchronously; an item that does not fit the bounded
// queue of its signal, or whose export fails, is counted in bothy.telemetry.dropped instead of
// delaying anything (D3). With cfg.Enabled false nothing is exported, logs go to stdout and the
// tracer provider still issues trace ids, so every request has a correlation id (17 "Signals").
//
// Call Setup before Middleware, and Shutdown with shutdown.telemetry_flush_timeout before the
// process exits.
//
// Errors: ErrConfig for an unusable cfg or deps; ErrInvalidEnv for an OTEL_* variable Setup
// cannot use; the errors of NewPseudonymizer (a missing or short pseudonym key); errors of the
// OTLP exporter constructors.
func Setup(ctx context.Context, cfg Config, deps Deps) (*Telemetry, error) {
	return setup(ctx, cfg, deps, setupOptions{})
}

func setup(ctx context.Context, cfg Config, deps Deps, opts setupOptions) (*Telemetry, error) {
	level, err := checkConfig(cfg, deps)
	if err != nil {
		return nil, err
	}
	if deps.Clock == nil {
		deps.Clock = clock.System()
	}
	if deps.Stdout == nil {
		deps.Stdout = os.Stdout
	}
	env, err := parseEnv(deps.Env)
	if err != nil {
		return nil, err
	}
	pseudo, err := NewPseudonymizer(ctx, deps.Secrets, cfg.PseudonymKeyRef, cfg.PseudonymRotation, deps.Clock)
	if err != nil {
		return nil, err
	}
	res, err := newResource(ctx, cfg, deps.ServiceVersion, deps.Clock.Now())
	if err != nil {
		return nil, err
	}

	t := &Telemetry{pseudo: pseudo}
	// The exporters count into t.drops, which exists once the meter provider does; the metric
	// reader goroutine starts before that.
	drop := func(signal string) func(int64) {
		return func(n int64) {
			if d := t.drops.Load(); d != nil {
				d.add(signal, n)
			}
		}
	}
	if t.meter, err = newMeterProvider(ctx, cfg, env, res, opts, drop(signalMetrics)); err != nil {
		return nil, err
	}
	drops, err := newDropCounter(t.meter)
	if err != nil {
		return nil, errors.Join(err, t.meter.Shutdown(ctx))
	}
	t.drops.Store(drops)
	if t.tracer, err = newTracerProvider(ctx, cfg, env, res, opts, drop(signalTraces)); err != nil {
		return nil, errors.Join(err, t.meter.Shutdown(ctx))
	}
	otlpLogs := cfg.Enabled && (opts.logExporter != nil || !env.logs.off) && cfg.LogsExporter != logsStdout
	if t.logs, err = newLoggerProvider(ctx, otlpLogs, env, res, opts, drop(signalLogs)); err != nil {
		return nil, errors.Join(err, t.tracer.Shutdown(ctx), t.meter.Shutdown(ctx))
	}
	t.logger = newServerLogger(cfg.LogsExporter, otlpLogs, level, deps.Stdout, t.logs)

	otelapi.SetTracerProvider(t.tracer)
	otelapi.SetMeterProvider(t.meter)
	otelapi.SetLoggerProvider(t.logs)
	otelapi.SetTextMapPropagator(propagation.TraceContext{})
	otelapi.SetErrorHandler(&errorHandler{logger: NewCommandLogger(deps.Stdout, slog.LevelWarn), clock: deps.Clock})
	setHTTPState(cfg.TrustRemoteParent, t.logger)
	return t, nil
}

// checkConfig checks what the configuration schema cannot: the fields of Deps, and the values
// again for callers that build a Config by hand.
func checkConfig(cfg Config, deps Deps) (slog.Level, error) {
	if deps.Secrets == nil {
		return 0, fmt.Errorf("%w: Deps.Secrets is nil", ErrConfig)
	}
	if cfg.SamplerRatio < 0 || cfg.SamplerRatio > 1 {
		return 0, fmt.Errorf("%w: telemetry.traces.sampler_ratio is outside 0 to 1", ErrConfig)
	}
	switch cfg.LogsExporter {
	case logsOTLP, logsStdout, logsBoth:
	default:
		return 0, fmt.Errorf("%w: telemetry.logs.exporter is not otlp, stdout or both", ErrConfig)
	}
	level, err := ParseLevel(cfg.LogsLevel)
	if err != nil {
		return 0, fmt.Errorf("%w: telemetry.logs.level is not debug, info, warn or error", ErrConfig)
	}
	return level, nil
}

// newTracerProvider samples by D4: ParentBased(TraceIDRatioBased(ratio)), so a new root (the
// default for every request, see Middleware) follows the ratio, a trusted remote parent its
// sampled flag and a local parent itself. Without an exporter it samples nothing but still
// issues trace and span ids.
func newTracerProvider(ctx context.Context, cfg Config, env otelEnv, res *resource.Resource, opts setupOptions, dropped func(int64)) (*sdktrace.TracerProvider, error) {
	exp := opts.spanExporter
	if exp == nil && cfg.Enabled && !env.traces.off {
		var err error
		if exp, err = newSpanExporter(ctx, env.traces.protocol); err != nil {
			return nil, err
		}
	}
	if !cfg.Enabled || exp == nil {
		return sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.NeverSample()), sdktrace.WithResource(res)), nil
	}
	b := newBudget(env.spanQueue)
	bopts := []sdktrace.BatchSpanProcessorOption{sdktrace.WithBlocking(), sdktrace.WithMaxQueueSize(env.spanQueue)}
	if env.spanQueue < sdktrace.DefaultMaxExportBatchSize {
		// A batch larger than the budget would only leave on the batch timer.
		bopts = append(bopts, sdktrace.WithMaxExportBatchSize(env.spanQueue))
	}
	batch := sdktrace.NewBatchSpanProcessor(
		filterSpanExporter{next: countingSpanExporter{next: exp, dropped: dropped, budget: b}}, bopts...)
	return sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplerRatio))),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(newQueueSpanProcessor(b, batch, dropped)),
	), nil
}

func newSpanExporter(ctx context.Context, protocol string) (sdktrace.SpanExporter, error) {
	if protocol == protocolHTTP {
		return otlptracehttp.New(ctx)
	}
	return otlptracegrpc.New(ctx)
}

// newMeterProvider reads cumulative metrics periodically into the OTLP exporter. Exemplars are
// off: they would carry the attributes the views remove.
func newMeterProvider(ctx context.Context, cfg Config, env otelEnv, res *resource.Resource, opts setupOptions, dropped func(int64)) (*sdkmetric.MeterProvider, error) {
	mopts := []sdkmetric.Option{
		sdkmetric.WithResource(res),
		sdkmetric.WithView(httpView),
		sdkmetric.WithExemplarFilter(exemplar.AlwaysOffFilter),
	}
	if opts.metricReader != nil {
		mopts = append(mopts, sdkmetric.WithReader(opts.metricReader))
	}
	if cfg.Enabled && !env.metrics.off {
		var exp sdkmetric.Exporter
		var err error
		if env.metrics.protocol == protocolHTTP {
			exp, err = otlpmetrichttp.New(ctx)
		} else {
			exp, err = otlpmetricgrpc.New(ctx)
		}
		if err != nil {
			return nil, err
		}
		mopts = append(mopts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(countingMetricExporter{next: exp, dropped: dropped})))
	}
	return sdkmetric.NewMeterProvider(mopts...), nil
}

// newLoggerProvider builds the OTLP log path when otlp is set; otherwise the provider has no
// processor.
func newLoggerProvider(ctx context.Context, otlp bool, env otelEnv, res *resource.Resource, opts setupOptions, dropped func(int64)) (*sdklog.LoggerProvider, error) {
	if !otlp {
		return sdklog.NewLoggerProvider(sdklog.WithResource(res)), nil
	}
	exp := opts.logExporter
	if exp == nil {
		var err error
		if env.logs.protocol == protocolHTTP {
			exp, err = otlploghttp.New(ctx)
		} else {
			exp, err = otlploggrpc.New(ctx)
		}
		if err != nil {
			return nil, err
		}
	}
	b := newBudget(env.logQueue)
	batch := sdklog.NewBatchProcessor(countingLogExporter{next: exp, dropped: dropped, budget: b},
		sdklog.WithMaxQueueSize(env.logQueue))
	return sdklog.NewLoggerProvider(sdklog.WithResource(res),
		sdklog.WithProcessor(newQueueLogProcessor(b, batch, dropped))), nil
}

// newServerLogger builds the handler chain of D1: level filter, allowlist filter, then the
// stdout JSON handler and/or the OTLP bridge. Without the OTLP path (telemetry.enabled false,
// OTEL_LOGS_EXPORTER=none or telemetry.logs.exporter stdout) the logs go to stdout even when
// telemetry.logs.exporter is otlp, so they are never silently lost.
func newServerLogger(exporter string, otlp bool, level slog.Level, stdout io.Writer, lp *sdklog.LoggerProvider) *slog.Logger {
	var outs []slog.Handler
	if exporter != logsOTLP || !otlp {
		outs = append(outs, newStdoutHandler(stdout))
	}
	if otlp {
		bridge := otelslog.NewHandler(instrumentationName, otelslog.WithLoggerProvider(lp))
		outs = append(outs, newContextFieldsHandler(bridge, false))
	}
	return slog.New(newLevelHandler(level, newAllowlistHandler(newFanout(outs...))))
}

// Logger returns the server logger: JSON on stdout and/or OTLP per telemetry.logs.exporter,
// only catalog attributes, trace fields from the context.
func (t *Telemetry) Logger() *slog.Logger { return t.logger }

// Pseudonymizer returns the pseudonymizer of bothy.principal.pseudo_id.
func (t *Telemetry) Pseudonymizer() *Pseudonymizer { return t.pseudo }

// ForceFlush exports what the processors and readers hold, within ctx.
func (t *Telemetry) ForceFlush(ctx context.Context) error {
	return errors.Join(t.tracer.ForceFlush(ctx), t.logs.ForceFlush(ctx), t.meter.ForceFlush(ctx))
}

// Shutdown flushes and closes all exporters within the deadline of ctx (the caller passes
// shutdown.telemetry_flush_timeout; 02 "Shutdown", step "Flush telemetry") and never waits
// beyond it. Traces and logs go first, so drops they count reach the last metric export. Items
// still queued when the deadline passes are lost without being counted. Later calls do nothing.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	var err error
	t.stopped.Do(func() {
		err = errors.Join(t.tracer.Shutdown(ctx), t.logs.Shutdown(ctx), t.meter.Shutdown(ctx))
	})
	return err
}

// errorHandler receives the errors of the OpenTelemetry SDK and exporters. Their texts carry
// endpoints and foreign messages, so it writes the constant message "telemetry export error"
// with error.type set to the Go type of the error, at most once per exportErrorInterval, and only
// to stdout: an OTLP log record about a failed export could fail the same way.
type errorHandler struct {
	logger *slog.Logger
	clock  clock.Clock
	mu     sync.Mutex
	last   time.Time
}

// Handle logs err as described on errorHandler.
func (h *errorHandler) Handle(err error) {
	now := h.clock.Now()
	h.mu.Lock()
	if !h.last.IsZero() && now.Sub(h.last) < exportErrorInterval {
		h.mu.Unlock()
		return
	}
	h.last = now
	h.mu.Unlock()
	// The SDK reports these errors outside any request.
	h.logger.LogAttrs(context.Background(), slog.LevelWarn, msgExportError,
		slog.String(attr.ErrorType, fmt.Sprintf("%T", err)))
}
