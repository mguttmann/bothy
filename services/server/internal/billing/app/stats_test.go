// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/billing"
	"github.com/OWNER/bothy/services/server/internal/billing/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

type statsReader struct {
	calls    int
	personal []bool
}

func (r *statsReader) CostStats(_ context.Context, _ billing.CostStatsQuery, showPersonal bool) ([]billing.CostStatsValue, error) {
	r.calls++
	r.personal = append(r.personal, showPersonal)
	return nil, nil
}

func (r *statsReader) CostCurrencies(_ context.Context, _ billing.CostStatsQuery, showPersonal bool) ([]string, error) {
	r.calls++
	r.personal = append(r.personal, showPersonal)
	return nil, nil
}

// TestStats: the source refuses a nil reader, an unknown grouping and a bad query, reads nothing
// for an empty channel set and passes billing.show_personal_estimates to every read.
func TestStats(t *testing.T) {
	t.Parallel()
	if _, err := app.NewStats(app.Config{}, nil); err == nil {
		t.Fatal("NewStats(nil) = nil error")
	}
	r := &statsReader{}
	s, err := app.NewStats(app.Config{ShowPersonalEstimates: true}, r)
	if err != nil {
		t.Fatal(err)
	}
	ctx, one := context.Background(), []ids.ID{ids.New()}
	valid := billing.CostStatsQuery{Granularity: "day", TimeZone: "UTC", GroupBy: billing.GroupBySpace, ChannelIDs: one}
	for _, bad := range []billing.CostStatsQuery{
		{Granularity: "day", TimeZone: "UTC", GroupBy: "person", ChannelIDs: one},
		{Granularity: "year", TimeZone: "UTC", GroupBy: billing.GroupByNone, ChannelIDs: one},
	} {
		if _, err := s.CostStats(ctx, bad); err == nil {
			t.Errorf("CostStats(%+v) = nil error", bad)
		}
		if _, err := s.CostCurrencies(ctx, bad); err == nil {
			t.Errorf("CostCurrencies(%+v) = nil error", bad)
		}
	}
	empty := valid
	empty.ChannelIDs = nil
	if got, err := s.CostStats(ctx, empty); err != nil || got != nil {
		t.Errorf("CostStats(empty) = %v, %v", got, err)
	}
	if got, err := s.CostCurrencies(ctx, empty); err != nil || got != nil {
		t.Errorf("CostCurrencies(empty) = %v, %v", got, err)
	}
	if r.calls != 0 {
		t.Fatalf("the reader was called %d times for empty or bad input", r.calls)
	}
	_, _ = s.CostStats(ctx, valid)
	_, _ = s.CostCurrencies(ctx, valid)
	if r.calls != 2 || !r.personal[0] || !r.personal[1] {
		t.Errorf("reads = %d with show_personal %v, want 2 with true", r.calls, r.personal)
	}
}
