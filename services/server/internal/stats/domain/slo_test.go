// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"math"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// minutes sums generated minute rows: n minutes with perMinute events each, of which the first
// bad minutes have badPerMinute bad events, as the traffic light reads them from stats_minutes.
func minutes(n int, perMinute, bad, badPerMinute int64) (total, good int64) {
	for i := range n {
		total += perMinute
		good += perMinute
		if int64(i) < bad {
			good -= badPerMinute
		}
	}
	return total, good
}

// TestSloState is VZ-T09 (docs/spec/25-visualization.md 6.4): generated minutes whose remaining
// budget is above and below stats.slo_warn_budget_ratio, exactly 0, negative, and fewer than
// stats.slo_min_events events give green, amber, red, red and insufficient_data, and the formula
// matches hand-computed values.
func TestSloState(t *testing.T) {
	t.Parallel()
	const warn, minEvents = 0.25, int64(100)
	// Objective 99: the error budget is 1 % of the events. 100 minutes of 100 events are 10000
	// events and a budget of 100 bad events.
	b := domain.SLIBound{SLO: "bothy-agents/run-start", Objective: 99, Period: 720 * time.Hour}
	for _, c := range []struct {
		name       string
		bad, per   int64
		wantBudget float64
		wantState  string
	}{
		{"half the budget left", 50, 1, 0.5, "green"},
		{"exactly the warn ratio", 75, 1, 0.25, "green"},
		{"below the warn ratio", 80, 1, 0.2, "amber"},
		{"budget exactly spent", 100, 1, 0, "red"},
		{"budget overspent", 75, 2, -0.5, "red"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			total, good := minutes(100, 100, c.bad, c.per)
			e := domain.EvaluateSLO(b, total, good, warn, minEvents)
			if e.State != c.wantState || !e.HasRatio || math.Abs(e.BudgetRemaining-c.wantBudget) > 1e-9 ||
				math.Abs(e.Ratio-float64(good)/float64(total)) > 1e-12 || e.Total != total || e.Good != good {
				t.Fatalf("%+v, want budget %v state %s", e, c.wantBudget, c.wantState)
			}
		})
	}
	t.Run("fewer than stats.slo_min_events", func(t *testing.T) {
		t.Parallel()
		total, good := minutes(9, 11, 9, 11)
		if total >= minEvents {
			t.Fatal("the case needs fewer events than the minimum")
		}
		e := domain.EvaluateSLO(b, total, good, warn, minEvents)
		if e.State != "insufficient_data" || !e.HasRatio || e.Ratio != 0 {
			t.Fatalf("%+v", e)
		}
	})
	t.Run("no events", func(t *testing.T) {
		t.Parallel()
		e := domain.EvaluateSLO(b, 0, 0, warn, minEvents)
		if e.State != "insufficient_data" || e.HasRatio {
			t.Fatalf("%+v", e)
		}
	})
	t.Run("hand computed", func(t *testing.T) {
		t.Parallel()
		// Objective 99.5, 1000 events, 997 good: share 0.997, budget 1 - 0.003/0.005 = 0.4.
		e := domain.EvaluateSLO(domain.SLIBound{Objective: 99.5}, 1000, 997, warn, minEvents)
		if math.Abs(e.Ratio-0.997) > 1e-12 || math.Abs(e.BudgetRemaining-0.4) > 1e-9 || e.State != "green" {
			t.Fatalf("%+v", e)
		}
		// Objective 95, 200 events, 185 good: share 0.925, budget 1 - 0.075/0.05 = -0.5.
		e = domain.EvaluateSLO(domain.SLIBound{Objective: 95}, 200, 185, warn, minEvents)
		if math.Abs(e.BudgetRemaining+0.5) > 1e-9 || e.State != "red" {
			t.Fatalf("%+v", e)
		}
	})
}

// TestValidateTimeline is 25 section 11 for the timeline (M6-14 design note 1): full minutes,
// from before to, at most stats.timeline_max_range and not before now - retention.stats_minutes.
func TestValidateTimeline(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 10, 12, 0, 30, 0, time.UTC)
	lim := domain.TimelineLimits{MaxRange: 24 * time.Hour, Retention: 90 * 24 * time.Hour}
	from := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	ok := func(from, to time.Time, lim domain.TimelineLimits) {
		t.Helper()
		if err := domain.ValidateTimeline(from, to, now, lim); err != nil {
			t.Fatalf("%v to %v: %v", from, to, err)
		}
	}
	bad := func(from, to time.Time, lim domain.TimelineLimits, code string) {
		t.Helper()
		err := domain.ValidateTimeline(from, to, now, lim)
		if got := fieldCode(t, err); got != code {
			t.Fatalf("%v to %v: %v (%s), want %s", from, to, err, got, code)
		}
	}
	ok(from, from.Add(time.Minute), lim)
	ok(from, from.Add(lim.MaxRange), lim)
	bad(from, from, lim, "invalid_range")
	bad(from, from.Add(-time.Minute), lim, "invalid_range")
	bad(from, from.Add(lim.MaxRange+time.Minute), lim, "range_too_long")
	bad(from.Add(time.Second), from.Add(time.Hour), lim, "not_full_minute")
	bad(from, from.Add(time.Hour+time.Millisecond), lim, "not_full_minute")
	// The oldest minute inside retention.stats_minutes is the first full minute at or after
	// now - retention; a range may start there, not one minute earlier.
	long := domain.TimelineLimits{MaxRange: lim.Retention + 24*time.Hour, Retention: lim.Retention}
	oldest := now.Add(-lim.Retention).Truncate(time.Minute).Add(time.Minute)
	ok(oldest, from, long)
	bad(oldest.Add(-time.Minute), from, long, "outside_retention")
}
