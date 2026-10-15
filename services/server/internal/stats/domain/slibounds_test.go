// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"sort"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// The generated bounds (tools/observability/sli_bounds.py, VZ-T03) cover exactly the six sli.*
// metrics of stats_minutes, and no SLO has a negative threshold or an empty period.
func TestSLIBoundsCoverTheSixMetrics(t *testing.T) {
	want := []string{
		"sli.api_availability",
		"sli.delivery_latency",
		"sli.push_acceptance",
		"sli.reconnect",
		"sli.run_start",
		"sli.turn_duration",
	}
	got := make([]string, 0, len(domain.SLIBounds))
	for metric := range domain.SLIBounds {
		got = append(got, metric)
	}
	sort.Strings(got)
	if len(got) != len(want) {
		t.Fatalf("metrics = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("metrics = %v, want %v", got, want)
		}
	}
	for metric, bound := range domain.SLIBounds {
		if bound.Threshold < 0 {
			t.Errorf("%s: threshold %v is negative", metric, bound.Threshold)
		}
		if bound.Period <= 0 {
			t.Errorf("%s: period %v is not positive", metric, bound.Period)
		}
		if bound.Objective <= 0 || bound.Objective >= 100 {
			t.Errorf("%s: objective %v is not a percentage below 100", metric, bound.Objective)
		}
		if bound.SLO == "" {
			t.Errorf("%s: no SLO name", metric)
		}
	}
}
