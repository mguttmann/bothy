// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/billing"
	billingapp "github.com/OWNER/bothy/services/server/internal/billing/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// personalOnlyReader holds the valid sums of an instance whose runs all ran on a personal
// subscription (the live case of FIX-COST/1) and applies the display rule of the query in
// queries/stats.sql: entries of mode personal count only with showPersonal.
type personalOnlyReader struct{}

var personalSums = []billing.CostStatsValue{
	{Group: "agent-a", BillingMode: "personal", Currency: "USD", AmountMicros: 3279543, IsEstimate: true},
	{Group: "agent-b", BillingMode: "personal", Currency: "USD", AmountMicros: 278094, IsEstimate: true},
}

func (personalOnlyReader) CostStats(_ context.Context, _ billing.CostStatsQuery, showPersonal bool) ([]billing.CostStatsValue, error) {
	var out []billing.CostStatsValue
	for _, v := range personalSums {
		if showPersonal || v.BillingMode != "personal" {
			out = append(out, v)
		}
	}
	return out, nil
}

func (personalOnlyReader) CostCurrencies(_ context.Context, _ billing.CostStatsQuery, showPersonal bool) ([]string, error) {
	if !showPersonal {
		return nil, nil
	}
	return []string{"USD"}, nil
}

// TestCostDisplay_PersonalEstimatesShownByDefault (FIX-COST/1): a server started with the
// default configuration answers the cost statistics (per agent, per space, per billing mode) and the
// backend view with the amounts of personal runs, which the apps label "information only"; on the
// default of before, every view of an instance on personal subscriptions showed no costs.
func TestCostDisplay_PersonalEstimatesShownByDefault(t *testing.T) {
	t.Parallel()
	v, err := config.Load(schema(t), config.Sources{Env: testEnv()})
	if err != nil {
		t.Fatalf("Load with defaults only: %v", err)
	}
	bc, err := wire.BillingConfig(v)
	if err != nil {
		t.Fatal(err)
	}
	s, err := billingapp.NewStats(bc, personalOnlyReader{})
	if err != nil {
		t.Fatal(err)
	}
	q := billing.CostStatsQuery{Granularity: "day", TimeZone: "UTC", GroupBy: billing.GroupByAgent, ChannelIDs: []ids.ID{ids.New()}}
	got, err := s.CostStats(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	var sum int64
	for _, g := range got {
		sum += g.AmountMicros
	}
	if len(got) != len(personalSums) || sum != 3279543+278094 {
		t.Errorf("CostStats with the default configuration = %+v, want both personal sums", got)
	}
	cur, err := s.CostCurrencies(context.Background(), q)
	if err != nil || len(cur) != 1 || cur[0] != "USD" {
		t.Errorf("CostCurrencies = %v, %v; want [USD]", cur, err)
	}
	vc, err := wire.StatsViewsConfig(v)
	if err != nil {
		t.Fatal(err)
	}
	if !vc.ShowPersonalEstimates {
		t.Error("the backend view hides personal amounts with the default configuration")
	}
	off, err := config.Load(schema(t), config.Sources{Env: append(testEnv(), "BOTHY_BILLING__SHOW_PERSONAL_ESTIMATES=false")})
	if err != nil {
		t.Fatal(err)
	}
	bcOff, err := wire.BillingConfig(off)
	if err != nil {
		t.Fatal(err)
	}
	if bcOff.ShowPersonalEstimates {
		t.Error("the switch can no longer hide personal amounts")
	}
}
