// SPDX-License-Identifier: Apache-2.0

package testkit_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// fakeTimes returns an operation whose n-th call (from 0) reports the duration f(n); calls counts
// the calls. No real time passes, so the cases below are exact.
func fakeTimes(f func(n int) time.Duration) (func() time.Duration, *int) {
	calls := 0
	return func() time.Duration {
		d := f(calls)
		calls++
		return d
	}, &calls
}

// The class of TimingSame (AZ-P4 and its relatives): load only adds delay, so a difference that
// shows in every round is a real one, and noise that slows some samples, or one whole round,
// never fails the comparison.
func TestTimingSame(t *testing.T) {
	t.Parallel()
	const tol = 20 * time.Millisecond
	bound := testkit.TimingBound(tol)
	perRound := testkit.TimingWarmup + testkit.TimingSamples
	for _, c := range []struct {
		name   string
		a, b   func(n int) time.Duration
		same   bool
		rounds int
	}{
		{"equal", func(int) time.Duration { return 5 * time.Millisecond }, func(int) time.Duration { return 5 * time.Millisecond }, true, 1},
		{"noise on single samples", func(n int) time.Duration {
			if n%3 == 0 {
				return 5*time.Millisecond + 10*bound
			}
			return 5 * time.Millisecond
		}, func(int) time.Duration { return 6 * time.Millisecond }, true, 1},
		{"one slow round on one side", func(n int) time.Duration {
			if n < perRound {
				return 5*time.Millisecond + 10*bound
			}
			return 5 * time.Millisecond
		}, func(int) time.Duration { return 5 * time.Millisecond }, true, 2},
		{"a real difference in every round", func(int) time.Duration { return 5 * time.Millisecond },
			func(int) time.Duration { return 5*time.Millisecond + 2*bound }, false, testkit.TimingRounds},
		{"a difference just inside the bound", func(int) time.Duration { return 5 * time.Millisecond },
			func(int) time.Duration { return 5*time.Millisecond + bound }, true, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, na := fakeTimes(c.a)
			b, nb := fakeTimes(c.b)
			d, same := testkit.TimingSame(tol, a, b)
			if same != c.same {
				t.Fatalf("same = %v (difference %s), want %v", same, d, c.same)
			}
			if *na != c.rounds*perRound || *nb != c.rounds*perRound {
				t.Fatalf("calls %d and %d, want %d rounds of %d", *na, *nb, c.rounds, perRound)
			}
			if !same && d != 2*bound {
				t.Fatalf("difference %s, want the smallest of the rounds, %s", d, 2*bound)
			}
		})
	}
}

// TimingSame alternates the order within a round, so neither side always runs first (a warm
// cache of the first call would otherwise favour the second).
func TestTimingSameAlternatesTheOrder(t *testing.T) {
	t.Parallel()
	var order []string
	a := func() time.Duration { order = append(order, "a"); return time.Millisecond }
	b := func() time.Duration { order = append(order, "b"); return time.Millisecond }
	if _, same := testkit.TimingSame(time.Millisecond, a, b); !same {
		t.Fatal("equal operations differ")
	}
	if len(order) < 4 || order[0] != "a" || order[1] != "b" || order[2] != "b" || order[3] != "a" {
		t.Fatalf("order %v, want a b b a ...", order)
	}
}
