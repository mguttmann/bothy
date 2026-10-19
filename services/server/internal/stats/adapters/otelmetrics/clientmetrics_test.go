// SPDX-License-Identifier: Apache-2.0

package otelmetrics_test

import (
	"context"
	"slices"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
	"github.com/OWNER/bothy/services/server/internal/stats/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// TestRecorder_ServerNamesAndAttributes: every accepted kind reaches the exporter under its
// server name bothy.client.* with exactly the attributes of signals.md 4.6a (no principal, no
// device, brief M4-31 Tests), and discards are counted by reason.
func TestRecorder_ServerNamesAndAttributes(t *testing.T) {
	metricstest.Strict(t)
	set := metrics.NewAggregatedSet()
	reader := sdkmetric.NewManualReader(sdkmetric.WithProducer(set))
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader), sdkmetric.WithView(metrics.View))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	r, err := otelmetrics.New(mp, set, []int{1000, 4009})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	attrs := domain.Attrs{Platform: "windows", Density: "compact", Theme: "dark", AppMajor: "3"}
	r.Record(ctx, domain.Measurement{Name: "client.frames", Kind: domain.KindCounter, Attrs: attrs, Value: 600})
	r.Record(ctx, domain.Measurement{Name: "client.reconnects", Kind: domain.KindCounter, Attrs: attrs, CloseCode: "4009", Value: 2})
	r.Record(ctx, domain.Measurement{Name: "client.reconnects", Kind: domain.KindCounter, Attrs: attrs, CloseCode: "other", Value: 1})
	r.Record(ctx, domain.Measurement{Name: "client.outbox.depth", Kind: domain.KindGauge, Attrs: attrs, Value: 4})
	r.Record(ctx, domain.Measurement{Name: "client.cold_start.duration", Kind: domain.KindHistogram, Attrs: attrs,
		Buckets: []domain.Bucket{{LE: 1, Count: 2}, {LE: 2.5, Count: 1}}})
	r.Record(ctx, domain.Measurement{Name: "client.unknown", Kind: domain.KindCounter, Attrs: attrs, Value: 1})
	r.Discarded(ctx, domain.ReasonUnknownName, 2)
	r.Discarded(ctx, domain.ReasonInvalidBounds, 0)

	got := metricstest.Collect(t, reader)
	var names []string
	for n := range got {
		names = append(names, n)
	}
	slices.Sort(names)
	want := []string{"bothy.client.cold_start.duration", "bothy.client.frames", "bothy.client.outbox.depth", "bothy.client.reconnects", "bothy.client_metrics.discarded"}
	if !slices.Equal(names, want) {
		t.Fatalf("exported %v, want %v", names, want)
	}
	common := map[string]string{"bothy.client.platform": "windows", "bothy.client.density": "compact", "bothy.client.theme": "dark", "bothy.client.app_major": "3"}
	for _, n := range want[:4] {
		for _, p := range metricstest.Points(got[n]) {
			for k, v := range common {
				if p.Attrs[k] != v {
					t.Errorf("%s: %s = %q, want %q", n, k, p.Attrs[k], v)
				}
			}
			extra := len(p.Attrs) - len(common)
			if n == "bothy.client.reconnects" {
				extra--
			}
			if extra != 0 {
				t.Errorf("%s: attributes %v beyond 4.6a", n, p.Attrs)
			}
		}
	}
	codes := map[string]float64{}
	for _, p := range metricstest.Points(got["bothy.client.reconnects"]) {
		codes[p.Attrs["bothy.close.code"]] = p.Value
	}
	if codes["4009"] != 2 || codes["other"] != 1 {
		t.Errorf("reconnects by close code %v", codes)
	}
	if p := metricstest.Points(got["bothy.client.cold_start.duration"]); len(p) != 1 || p[0].Count != 3 {
		t.Errorf("cold start %+v", p)
	}
	if p := metricstest.Points(got["bothy.client.outbox.depth"]); len(p) != 1 || p[0].Value != 4 {
		t.Errorf("outbox depth %+v", p)
	}
	d := metricstest.Points(got["bothy.client_metrics.discarded"])
	if len(d) != 1 || d[0].Attrs["bothy.reason"] != "unknown_name" || d[0].Value != 2 {
		t.Errorf("discarded %+v", d)
	}
}

// TestNew_NeedsTheSet: without a set there is no histogram path.
func TestNew_NeedsTheSet(t *testing.T) {
	if _, err := otelmetrics.New(nil, nil, []int{1000}); err == nil {
		t.Fatal("no error without a set")
	}
}
