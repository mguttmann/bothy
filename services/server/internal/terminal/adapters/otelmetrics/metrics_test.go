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
