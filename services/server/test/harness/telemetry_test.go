// SPDX-License-Identifier: Apache-2.0

package harness

import (
	"context"
	"net"
	"net/http"
	"sync/atomic"
	"testing"

	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// TestTelemetrySinks: with Options.Telemetry the server exports its request spans, OTLP log
// records and metrics into the sinks, and ProofObserver sees the DPoP proofs. Not parallel: the
// server installs the global providers.
func TestTelemetrySinks(t *testing.T) {
	sinks := NewTelemetrySinks()
	var proofs, tokens atomic.Int64
	s := Start(t, Options{EgressDenyAll: true, Telemetry: sinks, ProofObserver: func(string) { proofs.Add(1) },
		TokenObserver: func(access, refresh string) {
			if access != "" && refresh != "" {
				tokens.Add(1)
			}
		},
		Config: map[string]string{"telemetry.traces.sampler_ratio": "1.0", "telemetry.logs.exporter": "both", "telemetry.logs.level": "debug"}})
	MustDo(t, s.Owner, Request{Method: http.MethodGet, Path: "/me"}, http.StatusOK)
	ctx, cancel := context.WithTimeout(t.Context(), testkit.Patience)
	defer cancel()
	if err := s.FlushTelemetry(ctx); err != nil {
		t.Fatal(err)
	}
	if len(sinks.Spans.GetSpans()) == 0 {
		t.Error("no spans exported")
	}
	if len(sinks.Logs.Records()) == 0 {
		t.Error("no log records exported")
	}
	var rm metricdata.ResourceMetrics
	if err := sinks.Metrics.Collect(ctx, &rm); err != nil || len(rm.ScopeMetrics) == 0 {
		t.Errorf("metrics: %v, %d scopes", err, len(rm.ScopeMetrics))
	}
	if proofs.Load() == 0 {
		t.Error("ProofObserver saw no proof")
	}
	if tokens.Load() == 0 {
		t.Error("TokenObserver saw no token pair (the owner's enrollment)")
	}
	if s.SigningKeyDir == "" {
		t.Error("SigningKeyDir is empty")
	}
}

// TestTelemetrySinksAfterAPortRetry: when the first start of a server with Options.Telemetry
// fails on a taken public port, the second start still feeds the sinks; the metric reader of
// the failed start is shut down with its meter provider and a reader serves one provider only
// (a CI run of TestTelemetry_NoSpanPerFrame failed with "reader is shutdown"). Not parallel:
// the server installs the global providers.
func TestTelemetrySinksAfterAPortRetry(t *testing.T) {
	taken, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = taken.Close() })
	calls := 0
	ports := func() (int, error) {
		calls++
		if calls == 1 {
			return taken.Addr().(*net.TCPAddr).Port, nil
		}
		return freePort()
	}
	sinks := NewTelemetrySinks()
	s := Start(t, Options{EgressDenyAll: true, Telemetry: sinks, ports: ports})
	if calls != 2 {
		t.Fatalf("%d port reservations, want the taken one and one more", calls)
	}
	MustDo(t, s.Owner, Request{Method: http.MethodGet, Path: "/me"}, http.StatusOK)
	ctx, cancel := context.WithTimeout(t.Context(), testkit.Patience)
	defer cancel()
	if err := s.FlushTelemetry(ctx); err != nil {
		t.Fatal(err)
	}
	if len(sinks.Spans.GetSpans()) == 0 {
		t.Error("no spans exported")
	}
	var rm metricdata.ResourceMetrics
	if err := sinks.Metrics.Collect(ctx, &rm); err != nil || len(rm.ScopeMetrics) == 0 {
		t.Errorf("metrics: %v, %d scopes", err, len(rm.ScopeMetrics))
	}
}

// TestMemoryLogExporterKeepsRecords: records stay readable after Shutdown.
func TestMemoryLogExporterKeepsRecords(t *testing.T) {
	t.Parallel()
	e := &MemoryLogExporter{}
	var r sdklog.Record
	r.SetSeverityText("probe")
	if err := e.Export(t.Context(), []sdklog.Record{r}); err != nil {
		t.Fatal(err)
	}
	_ = e.Shutdown(t.Context())
	if got := e.Records(); len(got) != 1 || got[0].SeverityText() != "probe" {
		t.Fatalf("records %v", got)
	}
}
