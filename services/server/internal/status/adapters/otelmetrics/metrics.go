// SPDX-License-Identifier: Apache-2.0

// Package otelmetrics is the metrics adapter of bothy-backupd: the instruments bothy.backup.*
// (with bothy.backup.restore_test.* of M7-03) and bothy.storage.volume.* of
// docs/observability/signals.md 4.6 through the declarations of the metric registry
// (platform/otel/metrics). It implements app.Metrics. pg_full and pg_diff record
// bothy.backup.kind=physical; timestamps are Unix seconds.
//
// Example:
//
//	rec, err := otelmetrics.New(tel.MeterProvider()) // otel.SetupMetrics
package otelmetrics

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/status/app"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// Recorder holds the instruments of bothy-backupd. It is safe for concurrent use.
type Recorder struct {
	lastSuccess, wal, duration, size, free, total *metrics.Gauge
	// restoreLast and restoreDuration are the gauges of the restore test, restoreFailures its
	// counter of failed checks (M7-03).
	restoreLast, restoreDuration *metrics.Gauge
	restoreFailures              *metrics.Counter
}

// Compile-time check: Recorder is the outgoing port of app.
var _ app.Metrics = (*Recorder)(nil)

// New creates the gauges on mp; nil means the global provider. The error is a metrics.ErrSpec
// or an error of the meter provider.
func New(mp metric.MeterProvider) (*Recorder, error) {
	r := &Recorder{}
	for _, g := range []struct {
		dst  **metrics.Gauge
		spec metrics.Spec
	}{
		{&r.lastSuccess, metrics.BackupLastSuccess},
		{&r.wal, metrics.BackupWALLastArchived},
		{&r.duration, metrics.BackupDuration},
		{&r.size, metrics.BackupSize},
		{&r.free, metrics.StorageVolumeFree},
		{&r.total, metrics.StorageVolumeSize},
		{&r.restoreLast, metrics.BackupRestoreTestLastSuccess},
		{&r.restoreDuration, metrics.BackupRestoreTestDuration},
	} {
		gauge, err := metrics.NewGauge(mp, g.spec, nil)
		if err != nil {
			return nil, fmt.Errorf("otelmetrics: %w", err)
		}
		*g.dst = gauge
	}
	c, err := metrics.NewCounter(mp, metrics.BackupRestoreTestFailures, nil)
	if err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	r.restoreFailures = c
	return r, nil
}

// kindValue is the bothy.backup.kind of k: physical for both pgBackRest kinds.
func kindValue(k domain.Kind) string {
	if k.Physical() {
		return "physical"
	}
	return string(k)
}

// unixSeconds is t as Unix seconds with the fraction of the second.
func unixSeconds(t time.Time) float64 { return float64(t.UnixNano()) / 1e9 }

// RunFinished implements app.Metrics: duration and size of every run, the last success only of
// a succeeded one.
func (r *Recorder) RunFinished(ctx context.Context, kind domain.Kind, succeeded bool, duration time.Duration, sizeBytes int64, finishedAt time.Time) {
	v := kindValue(kind)
	if kind == domain.KindRestoreTest {
		// The restore test adds nothing to a repository, so it records no size.
		r.duration.Record(ctx, max(duration, 0).Seconds(), v)
		r.restoreDuration.Record(ctx, max(duration, 0).Seconds())
		if succeeded {
			r.restoreLast.Record(ctx, unixSeconds(finishedAt))
		}
		return
	}
	r.duration.Record(ctx, max(duration, 0).Seconds(), v)
	r.size.Record(ctx, float64(max(sizeBytes, 0)), v)
	if succeeded {
		r.lastSuccess.Record(ctx, unixSeconds(finishedAt), v)
	}
}

// LastSuccess implements app.Metrics.
func (r *Recorder) LastSuccess(ctx context.Context, kind domain.Kind, at time.Time) {
	if kind == domain.KindRestoreTest {
		r.restoreLast.Record(ctx, unixSeconds(at))
		return
	}
	r.lastSuccess.Record(ctx, unixSeconds(at), kindValue(kind))
}

// RestoreChecksFailed implements app.Metrics: one per failed check, bothy.reason the check name.
func (r *Recorder) RestoreChecksFailed(ctx context.Context, checks []domain.CheckName) {
	for _, c := range checks {
		r.restoreFailures.Add(ctx, 1, string(c))
	}
}

// WALArchived implements app.Metrics.
func (r *Recorder) WALArchived(ctx context.Context, at time.Time) {
	r.wal.Record(ctx, unixSeconds(at))
}

// Volume implements app.Metrics.
func (r *Recorder) Volume(ctx context.Context, volume string, usage app.VolumeUsage) {
	r.free.Record(ctx, float64(usage.FreeBytes), volume)
	r.total.Record(ctx, float64(usage.SizeBytes), volume)
}
