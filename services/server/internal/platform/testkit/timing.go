// SPDX-License-Identifier: Apache-2.0

package testkit

import (
	"slices"
	"time"
)

// raceTimingFactor widens a timing bound under the race detector, which makes Go code 2 to 20
// times slower (https://go.dev/doc/articles/race_detector, "Runtime Overhead") and turns a
// difference of about a millisecond, a database write for example, into one of tens of
// milliseconds on a loaded runner.
const raceTimingFactor = 5

// TimingBound returns d, the bound of a timing comparison of a test (the allowed difference of
// two medians, for example), widened by raceTimingFactor under the race detector. Without the
// race detector the bound stays d, so the comparison measures the real code there.
func TimingBound(d time.Duration) time.Duration {
	if RaceEnabled {
		return d * raceTimingFactor
	}
	return d
}

// The shape of TimingSame: per round TimingWarmup pairs that are not counted, then TimingSamples
// counted pairs, at most TimingRounds rounds.
const (
	TimingWarmup  = 3
	TimingSamples = 15
	TimingRounds  = 5
)

// TimingSame compares the duration of two operations that must not be told apart by their time
// (AZ-P4: an invisible and an unknown ID answer alike). a and b each run once and return the
// duration they measured. Per round the pairs run in alternating order, and the round compares
// the shortest counted duration of each side: load and the scheduler only add delay, while a
// real difference, an extra query for example, also lengthens the shortest run. The first round
// whose difference is at most TimingBound(tolerance) ends the comparison with same true; a
// difference in every one of TimingRounds rounds is a real one and returns false with the
// smallest difference seen. A slow round on a loaded machine therefore never fails the test,
// and the bound is never widened beyond TimingBound. Load that lasts the whole comparison is
// different: it lengthens a real, allowed difference by the same factor as the operations (an
// invisible channel that also decides and audits, against an unknown one that does not, beyond
// the bound in every round next to the parallel tests of its package). Call TimingSame only from a top-level
// test that does not call t.Parallel, so the parallel tests of the package wait until it is done
// (Semgrep rule bothy-go-test-timing-parallel); other packages running at the same time are not
// excluded.
func TimingSame(tolerance time.Duration, a, b func() time.Duration) (time.Duration, bool) {
	bound := TimingBound(tolerance)
	smallest := time.Duration(-1)
	for range TimingRounds {
		d := timingRound(a, b)
		if d <= bound {
			return d, true
		}
		if smallest < 0 || d < smallest {
			smallest = d
		}
	}
	return smallest, false
}

// timingRound runs one round of TimingSame and returns the difference of the shortest counted
// durations of a and b.
func timingRound(a, b func() time.Duration) time.Duration {
	var as, bs []time.Duration
	for i := range TimingWarmup + TimingSamples {
		var da, db time.Duration
		if i%2 == 0 {
			da, db = a(), b()
		} else {
			db, da = b(), a()
		}
		if i >= TimingWarmup {
			as, bs = append(as, da), append(bs, db)
		}
	}
	return (slices.Min(as) - slices.Min(bs)).Abs()
}
