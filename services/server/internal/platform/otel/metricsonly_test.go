// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// TestSetupMetrics_ServiceNameAndReader: the meter telemetry of bothy-backupd carries
// service.name bothy-backupd and the version, and a gauge of the registry reaches the reader;
// an instrument the catalogue does not name is dropped by the view.
func TestSetupMetrics_ServiceNameAndReader(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	cfg := testConfig()
	cfg.Enabled = false
	cfg.Role = ""
	tel, err := SetupMetrics(context.Background(), cfg, MetricsDeps{
		ServiceName: "bothy-backupd", ServiceVersion: "1.2.3", Clock: clock.System(), Reader: reader,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tel.Shutdown(context.Background()) })
	g, err := metrics.NewGauge(tel.MeterProvider(), metrics.BackupWALLastArchived, nil)
	if err != nil {
		t.Fatal(err)
	}
	g.Record(context.Background(), 1760839200)
	other, err := tel.MeterProvider().Meter("test").Int64Counter("not.in.catalogue")
	if err != nil {
		t.Fatal(err)
	}
	other.Add(context.Background(), 1)
	if err := tel.ForceFlush(context.Background()); err != nil {
		t.Fatal(err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	set := rm.Resource.Set()
	for key, want := range map[string]string{resServiceName: "bothy-backupd", resServiceNamespace: serviceNamespace, resServiceVersion: "1.2.3"} {
		if v, _ := set.Value(attribute.Key(key)); v.AsString() != want {
			t.Errorf("%s = %q, want %q", key, v.AsString(), want)
		}
	}
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "not.in.catalogue" {
				t.Error("an instrument outside the catalogue passed the view")
			}
			if g, ok := m.Data.(metricdata.Gauge[float64]); ok && m.Name == metrics.BackupWALLastArchived.Name {
				found = len(g.DataPoints) == 1 && g.DataPoints[0].Value == 1760839200
			}
		}
	}
	if !found {
		t.Errorf("the gauge did not reach the reader: %+v", rm.ScopeMetrics)
	}
}

// TestSetupMetrics_NeedsServiceName: an empty service name is a configuration error.
func TestSetupMetrics_NeedsServiceName(t *testing.T) {
	_, err := SetupMetrics(context.Background(), testConfig(), MetricsDeps{})
	if !errors.Is(err, ErrConfig) {
		t.Errorf("SetupMetrics without a service name = %v, want ErrConfig", err)
	}
}

// TestSetupMetrics_ExporterNone: enabled telemetry with OTEL_METRICS_EXPORTER=none builds no
// exporter and still shuts down within the deadline.
func TestSetupMetrics_ExporterNone(t *testing.T) {
	tel, err := SetupMetrics(context.Background(), testConfig(), MetricsDeps{
		ServiceName: "bothy-backupd", Env: []string{"OTEL_METRICS_EXPORTER=none"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := tel.Shutdown(testkit.Context(t)); err != nil {
		t.Fatal(err)
	}
}
