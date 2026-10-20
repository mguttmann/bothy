// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
)

// MetricsDeps are the dependencies of SetupMetrics.
type MetricsDeps struct {
	// ServiceName is service.name of the process, for example bothy-backupd; required.
	ServiceName string
	// ServiceVersion is service.version, buildinfo.Version() in the process.
	ServiceVersion string
	// Env is the process environment as the command read it ("NAME=value" entries); SetupMetrics
	// reads OTEL_EXPORTER_OTLP_PROTOCOL, OTEL_EXPORTER_OTLP_METRICS_PROTOCOL and
	// OTEL_METRICS_EXPORTER from it, as Setup does.
	Env []string
	// Clock is the source of the start time in service.instance.id; nil means clock.System().
	Clock clock.Clock
	// Reader replaces the OTLP exporter, for tests (for example sdkmetric.NewManualReader); it
	// reads the meter provider also with telemetry.enabled false.
	Reader sdkmetric.Reader
}

// MeterTelemetry is the metric telemetry of a process that sends metrics only, built by
// SetupMetrics. Its methods are safe for concurrent use.
type MeterTelemetry struct {
	meter *sdkmetric.MeterProvider
}

// SetupMetrics builds the metric telemetry of a process without traces, logs over OTLP and
// pseudonyms, such as bothy-backupd (brief M7-02 rule 18): the resource of Setup with
// service.name deps.ServiceName, the views of the metric registry and the OTLP metric exporter
// that deps.Env selects, through the counting exporter of bothy.telemetry.dropped. With
// cfg.Enabled false nothing is exported; only deps.Reader, when set, reads. SetupMetrics installs
// nothing globally: the caller passes MeterProvider to its instruments.
//
// Errors: ErrConfig without deps.ServiceName; ErrInvalidEnv for an OTEL_* variable it cannot
// use; errors of the OTLP exporter constructor.
//
// Example:
//
//	tel, err := otel.SetupMetrics(ctx, cfg, otel.MetricsDeps{
//		ServiceName: "bothy-backupd", ServiceVersion: buildinfo.Version(), Env: os.Environ(),
//	})
//	defer tel.Shutdown(flushCtx)
func SetupMetrics(ctx context.Context, cfg Config, deps MetricsDeps) (*MeterTelemetry, error) {
	if deps.ServiceName == "" {
		return nil, fmt.Errorf("%w: MetricsDeps.ServiceName is empty", ErrConfig)
	}
	if deps.Clock == nil {
		deps.Clock = clock.System()
	}
	env, err := parseEnv(deps.Env)
	if err != nil {
		return nil, err
	}
	res, err := newResource(ctx, cfg, deps.ServiceName, deps.ServiceVersion, deps.Clock.Now())
	if err != nil {
		return nil, err
	}
	t := &MeterTelemetry{}
	// The exporter counts into drops, which exists once the meter provider does; the reader
	// goroutine starts before that.
	var drops atomic.Pointer[dropCounter]
	dropped := func(n int64) {
		if d := drops.Load(); d != nil {
			d.add(signalMetrics, n)
		}
	}
	if t.meter, err = newMeterProvider(ctx, cfg, env, res, setupOptions{metricReader: deps.Reader}, dropped); err != nil {
		return nil, err
	}
	d, err := newDropCounter(t.meter)
	if err != nil {
		return nil, errors.Join(err, t.meter.Shutdown(ctx))
	}
	drops.Store(d)
	return t, nil
}

// MeterProvider returns the meter provider of the process.
func (t *MeterTelemetry) MeterProvider() metric.MeterProvider { return t.meter }

// ForceFlush exports what the reader holds, within ctx.
func (t *MeterTelemetry) ForceFlush(ctx context.Context) error { return t.meter.ForceFlush(ctx) }

// Shutdown flushes and closes the exporter within the deadline of ctx; later calls return the
// error of the SDK for a provider already shut down.
func (t *MeterTelemetry) Shutdown(ctx context.Context) error { return t.meter.Shutdown(ctx) }
