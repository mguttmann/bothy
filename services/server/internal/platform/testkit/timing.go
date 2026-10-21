// SPDX-License-Identifier: Apache-2.0

package testkit

import "time"

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
