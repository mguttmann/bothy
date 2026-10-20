// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestCanTransition_AllPairs walks all 36 pairs of the six states: exactly the transitions of
// 03 runs are allowed (nine rows of the table in 09 5.1, one of them with two targets, so ten
// pairs); self loops, every pair out of an end state and the rest are refused.
func TestCanTransition_AllPairs(t *testing.T) {
	q, r, w := domain.StateQueued, domain.StateRunning, domain.StateWaitingInput
	c, f, x := domain.StateCompleted, domain.StateFailed, domain.StateCancelled
	allowed := map[[2]domain.State]bool{
		{q, r}: true, {q, f}: true, {q, x}: true,
		{r, w}: true, {r, c}: true, {r, f}: true, {r, x}: true,
		{w, r}: true, {w, f}: true, {w, x}: true,
	}
	states := domain.States()
	if len(states) != 6 {
		t.Fatalf("States() = %v, want six states", states)
	}
	pairs, yes := 0, 0
	for _, from := range states {
		for _, to := range states {
			pairs++
			want := allowed[[2]domain.State{from, to}]
			if got := domain.CanTransition(from, to); got != want {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", from, to, got, want)
			}
			if want {
				yes++
			}
		}
	}
	if pairs != 36 || yes != len(allowed) {
		t.Fatalf("walked %d pairs with %d allowed", pairs, yes)
	}
	for _, s := range []domain.State{"", "started", "QUEUED"} {
		if domain.CanTransition(s, r) || domain.CanTransition(q, s) {
			t.Errorf("CanTransition accepts the unknown state %q", s)
		}
	}
}

// TestStatePredicates checks Valid, Active and Ended for every state and for unknown values.
func TestStatePredicates(t *testing.T) {
	cases := []struct {
		s                    domain.State
		valid, active, ended bool
	}{
		{domain.StateQueued, true, true, false},
		{domain.StateRunning, true, true, false},
		{domain.StateWaitingInput, true, true, false},
		{domain.StateCompleted, true, false, true},
		{domain.StateFailed, true, false, true},
		{domain.StateCancelled, true, false, true},
		{"", false, false, false},
		{"started", false, false, false},
	}
	for _, c := range cases {
		if got := c.s.Valid(); got != c.valid {
			t.Errorf("%q.Valid() = %v, want %v", c.s, got, c.valid)
		}
		if got := c.s.Active(); got != c.active {
			t.Errorf("%q.Active() = %v, want %v", c.s, got, c.active)
		}
		if got := c.s.Ended(); got != c.ended {
			t.Errorf("%q.Ended() = %v, want %v", c.s, got, c.ended)
		}
	}
}
