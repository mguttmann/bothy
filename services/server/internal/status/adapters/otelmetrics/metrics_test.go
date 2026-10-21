// SPDX-License-Identifier: Apache-2.0

package otelmetrics_test

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
	"github.com/OWNER/bothy/services/server/internal/status/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/status/app"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// point is one gauge data point: its value and its attributes.
type point struct {
	value float64
	attrs attribute.Set
}

// collect returns the gauge points of rm by instrument name.
func collect(t *testing.T, reader *sdkmetric.ManualReader) (map[string][]point, map[string]string) {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	points, units := map[string][]point{}, map[string]string{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			g, ok := m.Data.(metricdata.Gauge[float64])
			if !ok {
				t.Fatalf("%s: data %T, want a float64 gauge", m.Name, m.Data)
			}
			units[m.Name] = m.Unit
			for _, dp := range g.DataPoints {
				points[m.Name] = append(points[m.Name], point{value: dp.Value, attrs: dp.Attributes})
			}
		}
	}
	return points, units
}

// valueFor returns the value of the point of name whose attribute key has value, and whether
// there is one.
func valueFor(points map[string][]point, name, key, value string) (float64, bool) {
	for _, p := range points[name] {
		if v, ok := p.attrs.Value(attribute.Key(key)); ok && v.AsString() == value {
			return p.value, true
		}
		if key == "" && p.attrs.Len() == 0 {
			return p.value, true
		}
	}
	return 0, false
}

func newRecorder(t *testing.T) (*otelmetrics.Recorder, *sdkmetric.ManualReader) {
	t.Helper()
	metricstest.Strict(t)
	reader := sdkmetric.NewManualReader()
	r, err := otelmetrics.New(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	if err != nil {
		t.Fatal(err)
	}
	return r, reader
}

var _ app.Metrics = (*otelmetrics.Recorder)(nil)

// TestRunFinished: a succeeded run records duration, size and last success under its kind
// attribute; pg_full and pg_diff record physical; a failed run records no last success.
func TestRunFinished(t *testing.T) {
	r, reader := newRecorder(t)
	ctx := context.Background()
	finished := time.Date(2026, 10, 19, 2, 0, 30, 500_000_000, time.UTC)
	r.RunFinished(ctx, domain.KindPGDiff, true, 90*time.Second, 4096, finished)
	r.RunFinished(ctx, domain.KindLogical, false, 3*time.Second, 0, finished)
	points, units := collect(t, reader)

	for _, tc := range []struct {
		name, kind string
		want       float64
	}{
		{"bothy.backup.duration", "physical", 90},
		{"bothy.backup.size", "physical", 4096},
		{"bothy.backup.last_success.timestamp", "physical", float64(finished.UnixNano()) / 1e9},
		{"bothy.backup.duration", "logical", 3},
		{"bothy.backup.size", "logical", 0},
	} {
		got, ok := valueFor(points, tc.name, "bothy.backup.kind", tc.kind)
		if !ok || got != tc.want {
			t.Errorf("%s{%s} = %v (present %v), want %v", tc.name, tc.kind, got, ok, tc.want)
		}
	}
	if _, ok := valueFor(points, "bothy.backup.last_success.timestamp", "bothy.backup.kind", "logical"); ok {
		t.Error("a failed run recorded a last success")
	}
	if units["bothy.backup.duration"] != "s" || units["bothy.backup.size"] != "By" {
		t.Errorf("units %v", units)
	}
}

// TestLastSuccessWALAndVolume: the start values, the WAL time without attributes and both
// volume gauges.
func TestLastSuccessWALAndVolume(t *testing.T) {
	r, reader := newRecorder(t)
	ctx := context.Background()
	at := time.Date(2026, 10, 18, 23, 0, 0, 0, time.UTC)
	r.LastSuccess(ctx, domain.KindObjects, at)
	r.LastSuccess(ctx, domain.KindPGFull, at.Add(time.Hour))
	r.WALArchived(ctx, at.Add(2*time.Hour))
	r.Volume(ctx, "pgbackrest-repo", app.VolumeUsage{FreeBytes: 1 << 30, SizeBytes: 4 << 30})
	points, _ := collect(t, reader)

	for _, tc := range []struct {
		name, key, value string
		want             float64
	}{
		{"bothy.backup.last_success.timestamp", "bothy.backup.kind", "objects", float64(at.Unix())},
		{"bothy.backup.last_success.timestamp", "bothy.backup.kind", "physical", float64(at.Add(time.Hour).Unix())},
		{"bothy.backup.wal.last_archived.timestamp", "", "", float64(at.Add(2 * time.Hour).Unix())},
		{"bothy.storage.volume.free", "bothy.volume", "pgbackrest-repo", 1 << 30},
		{"bothy.storage.volume.size", "bothy.volume", "pgbackrest-repo", 4 << 30},
	} {
		got, ok := valueFor(points, tc.name, tc.key, tc.value)
		if !ok || got != tc.want {
			t.Errorf("%s{%s} = %v (present %v), want %v", tc.name, tc.value, got, ok, tc.want)
		}
	}
}
