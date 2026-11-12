// SPDX-License-Identifier: Apache-2.0

package otelmetrics_test

import (
	"context"
	"maps"
	"strings"
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

// collect returns the gauge points of rm by instrument name; the two counters are read by sums.
func collect(t *testing.T, reader *sdkmetric.ManualReader) (map[string][]point, map[string]string) {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	points, units := map[string][]point{}, map[string]string{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if _, counter := m.Data.(metricdata.Sum[int64]); counter {
				continue
			}
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

// sums returns the points of the counters as "<instrument>{<value of its one attribute>}", and
// fails on a counter that is not cumulative and monotonic.
func sums(t *testing.T, reader *sdkmetric.ManualReader) map[string]int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	out := map[string]int64{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			d, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				continue
			}
			if !d.IsMonotonic || d.Temporality != metricdata.CumulativeTemporality {
				t.Errorf("%s: monotonic %v, temporality %v; want a cumulative counter", m.Name, d.IsMonotonic, d.Temporality)
			}
			for _, dp := range d.DataPoints {
				if dp.Attributes.Len() != 1 {
					t.Errorf("%s: %d attributes, want 1", m.Name, dp.Attributes.Len())
					continue
				}
				kv, _ := dp.Attributes.Get(0)
				out[m.Name+"{"+kv.Value.AsString()+"}"] = dp.Value
			}
		}
	}
	return out
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

// TestRestoreTestMetrics holds alerting.md for the restore test: a passed test records the
// duration (also bothy.backup.duration{restore_test}) and the last success; a failed one records
// its duration and one failure per failed check with bothy.reason the check name, and no last
// success; LastSuccess of the start sets the gauge.
func TestRestoreTestMetrics(t *testing.T) {
	r, reader := newRecorder(t)
	ctx := context.Background()
	at := time.Date(2026, 10, 24, 5, 40, 0, 0, time.UTC)
	r.LastSuccess(ctx, domain.KindRestoreTest, at.Add(-7*24*time.Hour))
	r.RunFinished(ctx, domain.KindRestoreTest, true, 40*time.Minute, 0, at)
	r.RunFinished(ctx, domain.KindRestoreTest, false, 10*time.Minute, 0, at.Add(time.Hour))
	r.RestoreChecksFailed(ctx, []domain.CheckName{domain.CheckEventLog, domain.CheckAuditChain})
	r.RestoreChecksFailed(ctx, []domain.CheckName{domain.CheckEventLog})

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatal(err)
	}
	gauges := map[string]float64{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if d, ok := m.Data.(metricdata.Gauge[float64]); ok {
				for _, dp := range d.DataPoints {
					key := m.Name
					if v, ok := dp.Attributes.Value("bothy.backup.kind"); ok {
						key += "{" + v.AsString() + "}"
					}
					gauges[key] = dp.Value
				}
			}
		}
	}
	failures := sums(t, reader)
	if got := gauges["bothy.backup.restore_test.last_success.timestamp"]; got != float64(at.Unix()) {
		t.Errorf("last success %v, want the passed test %v", got, at.Unix())
	}
	if got := gauges["bothy.backup.restore_test.duration"]; got != 600 {
		t.Errorf("duration %v, want the last test 600", got)
	}
	if got := gauges["bothy.backup.duration{restore_test}"]; got != 600 {
		t.Errorf("bothy.backup.duration{restore_test} %v, want 600", got)
	}
	if _, ok := gauges["bothy.backup.size{restore_test}"]; ok {
		t.Error("the restore test recorded a size")
	}
	wantFailures := map[string]int64{
		"bothy.backup.restore_test.failures{event_log}": 2, "bothy.backup.restore_test.failures{audit_chain}": 1,
		"bothy.backup.restore_test.failures{pgbackrest}": 0, "bothy.backup.restore_test.failures{manifest}": 0,
		"bothy.backup.restore_test.failures{thread_sample}": 0, "bothy.backup.restore_test.failures{file_sample}": 0,
		"bothy.backup.restore_test.failures{duration}": 0,
		// A failed restore test is no failed backup run.
		"bothy.backup.failures{pg_full}": 0, "bothy.backup.failures{pg_diff}": 0,
		"bothy.backup.failures{logical}": 0, "bothy.backup.failures{objects}": 0,
	}
	if !maps.Equal(failures, wantFailures) {
		t.Errorf("failures %v, want event_log 2, audit_chain 1 and every other series 0", failures)
	}
}

// TestFailureCountersStartAtZero holds what the alerts BothyBackupRunFailed, BothyRestoreTestFailed
// and BothyRestoreTestAuditChainFailed need (docs/observability/alerting.md): every series of
// the two failure counters exists with 0 from the creation of the recorder on, before any run.
// A cumulative counter that first appears with its first failure shows no increase to a rule, so
// the first failure after every start of the process would raise no alert.
func TestFailureCountersStartAtZero(t *testing.T) {
	_, reader := newRecorder(t)
	got := sums(t, reader)
	want := map[string]int64{}
	for _, k := range domain.Kinds() {
		want["bothy.backup.failures{"+string(k)+"}"] = 0
	}
	for _, c := range domain.RestoreCheckNames() {
		want["bothy.backup.restore_test.failures{"+string(c)+"}"] = 0
	}
	if len(want) != 11 {
		t.Fatalf("%d series expected from the domain, want four kinds and seven checks", len(want))
	}
	if !maps.Equal(got, want) {
		t.Errorf("counters at the start %v, want %v", got, want)
	}
}

// TestRunFailed: a failed backup run counts under the kind of its backup_runs row, so pg_full
// and pg_diff count apart (the gauges put both under physical, where a succeeding pg_diff hides
// a failing pg_full); the restore test is no backup kind and counts nothing here.
func TestRunFailed(t *testing.T) {
	r, reader := newRecorder(t)
	ctx := context.Background()
	r.RunFailed(ctx, domain.KindPGFull)
	r.RunFailed(ctx, domain.KindPGFull)
	r.RunFailed(ctx, domain.KindObjects)
	r.RunFailed(ctx, domain.KindRestoreTest)
	got := sums(t, reader)
	for kind, want := range map[string]int64{"pg_full": 2, "pg_diff": 0, "logical": 0, "objects": 1} {
		if v, ok := got["bothy.backup.failures{"+kind+"}"]; !ok || v != want {
			t.Errorf("bothy.backup.failures{%s} = %d (present %v), want %d", kind, v, ok, want)
		}
	}
	for key := range got {
		if strings.HasPrefix(key, "bothy.backup.failures{") && len(got) != 11 {
			t.Errorf("series %v, want only the four backup kinds under bothy.backup.failures", got)
			break
		}
	}
	// A failed run still records the gauges of its metric kind through RunFinished.
	r.RunFinished(ctx, domain.KindPGFull, false, time.Second, 0, time.Date(2026, 10, 19, 2, 0, 0, 0, time.UTC))
	points, _ := collect(t, reader)
	if v, ok := valueFor(points, "bothy.backup.duration", "bothy.backup.kind", "physical"); !ok || v != 1 {
		t.Errorf("bothy.backup.duration{physical} = %v (present %v), want 1", v, ok)
	}
}
