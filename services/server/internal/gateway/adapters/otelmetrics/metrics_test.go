// SPDX-License-Identifier: Apache-2.0

package otelmetrics_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
)

// TestGatewayMetrics_CloseCodeForeignCodeIsOther: a close code of 04 counts as its decimal string,
// any other code a client sends counts as other.
func TestGatewayMetrics_CloseCodeForeignCodeIsOther(t *testing.T) {
	mp, reader := metricstest.Provider(t)
	m, err := otelmetrics.New(mp)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m.Closed(ctx, 4014)
	m.Closed(ctx, 1000)
	m.Closed(ctx, 4999)
	m.Closed(ctx, 3000)
	got := metricstest.Find(t, reader, "bothy.gateway.close")
	for code, want := range map[string]float64{"4014": 1, "1000": 1, "other": 2} {
		if n := metricstest.Sum(got, map[string]string{"bothy.close.code": code}); n != want {
			t.Errorf("close %s = %v, want %v", code, n, want)
		}
	}
	if n := len(metricstest.Points(got)); n != 3 {
		t.Errorf("%d series, want 3", n)
	}
}

// TestInstruments_ClosedAttributes: every instrument of the adapter records only catalogue keys
// and values, and the fan-out lag gauge observes the hub's lag in seconds.
func TestInstruments_ClosedAttributes(t *testing.T) {
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	m, err := otelmetrics.New(mp)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m.ConnectionOpened(ctx, "user")
	m.ConnectionOpened(ctx, "agent")
	m.ConnectionClosed(ctx, "user")
	m.Connected(ctx, time.Second, "ok", "user")
	m.Connected(ctx, 0, "client_error", "anonymous")
	m.CatchupDone(ctx, 2*time.Second, "complete")
	m.Resync(ctx, "too_far_behind")
	m.Dropped(ctx, "slow_consumer", 3)
	m.Delivered(ctx, 40*time.Millisecond, "ephemeral", "app")
	reg, err := otelmetrics.RegisterFanoutLag(mp, func() time.Duration { return 1500 * time.Millisecond })
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reg.Unregister() }()
	got := metricstest.Collect(t, reader)
	checks := map[string]struct {
		attrs map[string]string
		want  float64
	}{
		"bothy.gateway.connections.active": {map[string]string{"bothy.principal.kind": "user"}, 0},
		"bothy.gateway.connect.duration":   {map[string]string{"bothy.outcome": "client_error", "bothy.principal.kind": "anonymous"}, 1},
		"bothy.gateway.catchup.duration":   {map[string]string{"bothy.outcome": "complete"}, 1},
		"bothy.gateway.resync":             {map[string]string{"bothy.reason": "too_far_behind"}, 1},
		"bothy.gateway.outbound.dropped":   {map[string]string{"bothy.reason": "slow_consumer"}, 3},
		"bothy.delivery.duration":          {map[string]string{"bothy.event.durability": "ephemeral", "bothy.principal.kind": "app"}, 1},
		"bothy.event.fanout.lag":           {nil, 1.5},
	}
	for name, c := range checks {
		if n := metricstest.Sum(got[name], c.attrs); n != c.want {
			t.Errorf("%s %v = %v, want %v", name, c.attrs, n, c.want)
		}
	}
	if n := metricstest.Sum(got["bothy.gateway.connections.active"], map[string]string{"bothy.principal.kind": "agent"}); n != 1 {
		t.Errorf("active agents %v, want 1", n)
	}
}
