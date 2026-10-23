// SPDX-License-Identifier: Apache-2.0

package pushhandoff

import (
	"time"

	"github.com/OWNER/bothy/services/server/perf/internal/report"
)

// Sample returns one measurement of design note 7: from t0, the occurred_at of the transaction
// of the mention (database clock), to t1, the AcceptedAt of Fake-APNs (host clock), with t0 moved
// to the host clock by offset (how far the database clock runs ahead, report.ClockOffset).
func Sample(accepted, occurred time.Time, offset time.Duration) time.Duration {
	return report.CorrectedLatency(accepted, occurred, offset)
}

// P95 returns the nearest-rank 95th percentile of samples, the value of perf.push_handoff_p95.
// The error reports no samples.
func P95(samples []time.Duration) (time.Duration, error) {
	v, err := report.DurationsP(samples, 95)
	return time.Duration(v), err
}

// inWindow reports whether a send scheduled at at belongs to the measured window [from, to).
func inWindow(at, from, to time.Time) bool { return !at.Before(from) && at.Before(to) }
