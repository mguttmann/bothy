// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"strings"
	"time"
)

// Threshold returns the latency threshold of the SLO whose bothy_sli_metric is metric, from
// SLIBounds (slibounds.gen.go, docs/spec/25-visualization.md 3.3); 0 for a ratio SLO without one
// and for every metric that is no SLI.
func Threshold(metric string) time.Duration {
	return SLIBounds[metric].Threshold
}

// IsGood reports whether an SLI event of metric that took d is good by its duration: d at most the
// threshold, the bound included, exactly as a histogram bucket with le = threshold counts it
// (R-OB-06, VZ-T02). Without a threshold every duration is good, so only the outcome decides.
//
// Example:
//
//	domain.IsGood("sli.delivery_latency", 500*time.Millisecond) // true: the bound is inclusive
func IsGood(metric string, d time.Duration) bool {
	t := Threshold(metric)
	return t == 0 || d <= t
}

// Counters are the three counting columns of stats_minutes for one key.
type Counters struct {
	// Count is the number of events.
	Count int64
	// Sum is the sum: milliseconds for durations, micro-units for cost, otherwise 0.
	Sum int64
	// Good is the number of good SLI events; 0 for every other metric.
	Good int64
}

// Add returns c plus o.
func (c Counters) Add(o Counters) Counters {
	return Counters{Count: c.Count + o.Count, Sum: c.Sum + o.Sum, Good: c.Good + o.Good}
}

// Event returns the counters of one Record call of metric: count events, sum plus the whole
// milliseconds of elapsed (a negative elapsed counts as 0), and for an sli.* metric good = count
// when ok holds and IsGood holds for elapsed, else 0. good never exceeds count (03 4.15).
func Event(metric string, count, sum int64, elapsed time.Duration, ok bool) Counters {
	elapsed = max(elapsed, 0)
	c := Counters{Count: count, Sum: sum + elapsed.Milliseconds()}
	if strings.HasPrefix(metric, "sli.") && ok && IsGood(metric, elapsed) {
		c.Good = count
	}
	return c
}

// Minute returns the start of the UTC minute of t, the bucket_start of a key (25 3.4).
func Minute(t time.Time) time.Time {
	return t.UTC().Truncate(time.Minute)
}

// Day returns the start of the UTC day of t, the lower bound of its partition of stats_minutes.
func Day(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// LongestSLOPeriod returns the longest Period of SLIBounds, the shortest retention.stats_minutes
// that `bothy config check` accepts (25 3.5, VZ-T14).
func LongestSLOPeriod() time.Duration {
	var p time.Duration
	for _, b := range SLIBounds {
		p = max(p, b.Period)
	}
	return p
}

// RetentionCutoff returns the cutoff of the job stats.retention at now with the period retention:
// now - retention. The job drops every partition whose UTC day ends at or before it, so no minute
// younger than the period is ever dropped (25 3.5, VZ-T15).
func RetentionCutoff(now time.Time, retention time.Duration) time.Time {
	return now.UTC().Add(-retention)
}
