// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/authz/adapters/telemetry"
)

func TestErrorCounter_CountsWithReason(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	c, err := telemetry.NewErrorCounter(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	if err != nil {
		t.Fatal(err)
	}
	c.CountError(context.Background(), "unknown_action")
	c.CountError(context.Background(), "unknown_action")
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	if len(rm.ScopeMetrics) != 1 || len(rm.ScopeMetrics[0].Metrics) != 1 {
		t.Fatalf("metrics = %+v", rm.ScopeMetrics)
	}
	m := rm.ScopeMetrics[0].Metrics[0]
	sum, ok := m.Data.(metricdata.Sum[int64])
	if m.Name != "bothy.authz.errors" || m.Unit != "{check}" || !ok || !sum.IsMonotonic || len(sum.DataPoints) != 1 {
		t.Fatalf("metric = %+v", m)
	}
	dp := sum.DataPoints[0]
	if v, _ := dp.Attributes.Value(attribute.Key("bothy.reason")); dp.Value != 2 || v.AsString() != "unknown_action" || dp.Attributes.Len() != 1 {
		t.Fatalf("data point = %+v", dp)
	}
}

func TestNewErrorCounter_NilUsesGlobal(t *testing.T) {
	c, err := telemetry.NewErrorCounter(nil)
	if err != nil || c == nil {
		t.Fatalf("NewErrorCounter(nil) = %v, %v", c, err)
	}
	c.CountError(context.Background(), "unknown_action")
}
