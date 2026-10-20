// SPDX-License-Identifier: Apache-2.0

package otelmetrics_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// TestRelayUnpaired_Sides: the counter takes only the catalogue values client and host.
func TestRelayUnpaired_Sides(t *testing.T) {
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	m, err := otelmetrics.New(mp)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m.RelayUnpaired(ctx, domain.SideClient)
	m.RelayUnpaired(ctx, domain.SideClient)
	m.RelayUnpaired(ctx, domain.SideHost)
	got := metricstest.Find(t, reader, "bothy.terminal.relay.unpaired")
	for side, want := range map[string]float64{"client": 2, "host": 1} {
		if n := metricstest.Sum(got, map[string]string{"bothy.terminal.side": side}); n != want {
			t.Errorf("%s = %v, want %v", side, n, want)
		}
	}
}

// TestRelayUnpairedAttributeSet (M2-47 requirement 17): the only attribute of the counter is
// bothy.terminal.side, with exactly the values client and host.
func TestRelayUnpairedAttributeSet(t *testing.T) {
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	m, err := otelmetrics.New(mp)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m.RelayUnpaired(ctx, domain.SideClient)
	m.RelayUnpaired(ctx, domain.SideHost)
	values := map[string]bool{}
	for _, p := range metricstest.Points(metricstest.Find(t, reader, "bothy.terminal.relay.unpaired")) {
		if len(p.Attrs) != 1 {
			t.Fatalf("attributes %v, want only bothy.terminal.side", p.Attrs)
		}
		side, ok := p.Attrs["bothy.terminal.side"]
		if !ok {
			t.Fatalf("attributes %v", p.Attrs)
		}
		values[side] = true
	}
	if len(values) != 2 || !values["client"] || !values["host"] {
		t.Fatalf("values %v, want client and host", values)
	}
}
