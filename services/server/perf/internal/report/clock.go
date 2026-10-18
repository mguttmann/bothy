// SPDX-License-Identifier: Apache-2.0

package report

import (
	"errors"
	"time"
)

// ClockSample is one bracketed reading of the database clock: the host time before the query,
// the result of SELECT clock_timestamp(), and the host time after the answer.
type ClockSample struct {
	Before, DB, After time.Time
}

// ClockOffset returns how far the database clock runs ahead of the host clock (negative when it
// runs behind), from the sample with the shortest round trip: the database reading minus the
// midpoint of its bracket. The error reports no samples.
func ClockOffset(samples []ClockSample) (time.Duration, error) {
	if len(samples) == 0 {
		return 0, errors.New("report: no clock samples")
	}
	best := samples[0]
	for _, s := range samples[1:] {
		if s.After.Sub(s.Before) < best.After.Sub(best.Before) {
			best = s
		}
	}
	mid := best.Before.Add(best.After.Sub(best.Before) / 2)
	return best.DB.Sub(mid), nil
}

// CorrectedLatency returns the time from an event the database stamped at occurred to its receipt
// at host time received, with occurred moved to host time by offset (ClockOffset).
func CorrectedLatency(received, occurred time.Time, offset time.Duration) time.Duration {
	return received.Sub(occurred.Add(-offset))
}
