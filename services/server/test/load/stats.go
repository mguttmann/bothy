// SPDX-License-Identifier: Apache-2.0

package load

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"time"
)

// Percentile returns the nearest-rank p-th percentile (0 < p <= 100) of samples without changing
// the slice. The error reports no samples or p out of range.
func Percentile(samples []time.Duration, p float64) (time.Duration, error) {
	if len(samples) == 0 {
		return 0, errors.New("load: no samples")
	}
	if p <= 0 || p > 100 {
		return 0, fmt.Errorf("load: percentile %g out of (0, 100]", p)
	}
	s := slices.Clone(samples)
	slices.Sort(s)
	rank := int(math.Ceil(p / 100 * float64(len(s))))
	return s[max(rank, 1)-1], nil
}

// Send is one createMessage of the open-loop schedule: when it was due, when its answer arrived
// and the status (0 for a transport error).
type Send struct {
	Scheduled, Done time.Time
	Status          int
}

// Latency is the time from the scheduled send to the answer (no coordinated omission).
func (s Send) Latency() time.Duration { return s.Done.Sub(s.Scheduled) }

// SendSummary sums up the sends scheduled in a window.
type SendSummary struct {
	// Scheduled counts the sends due in the window; OK those answered 201, Failed the others.
	Scheduled int `json:"scheduled"`
	OK        int `json:"ok"`
	Failed    int `json:"failed"`
	// FailedStatuses counts the failed sends by status (0: transport error).
	FailedStatuses map[int]int `json:"failed_statuses,omitempty"`
	// WithinTarget counts the 201 answers whose latency is at most perf.message_create_p99.
	WithinTarget int `json:"within_target"`
	// AchievedPerS is OK per second of the window; GoodputPerS is WithinTarget per second, the
	// sustained throughput the capacity gate holds.
	AchievedPerS float64 `json:"achieved_per_s"`
	GoodputPerS  float64 `json:"goodput_per_s"`
	// P50, P99 and Max are latencies of the 201 answers.
	P50 time.Duration `json:"p50_ns"`
	P99 time.Duration `json:"p99_ns"`
	Max time.Duration `json:"max_ns"`
}

// SummarizeSends sums up the sends scheduled in [from, to); target is perf.message_create_p99.
func SummarizeSends(sends []Send, from, to time.Time, target time.Duration) SendSummary {
	var s SendSummary
	var lat []time.Duration
	for _, x := range sends {
		if x.Scheduled.Before(from) || !x.Scheduled.Before(to) {
			continue
		}
		s.Scheduled++
		if x.Status != 201 {
			s.Failed++
			if s.FailedStatuses == nil {
				s.FailedStatuses = map[int]int{}
			}
			s.FailedStatuses[x.Status]++
			continue
		}
		s.OK++
		l := x.Latency()
		lat = append(lat, l)
		if l <= target {
			s.WithinTarget++
		}
	}
	if secs := to.Sub(from).Seconds(); secs > 0 {
		s.AchievedPerS = float64(s.OK) / secs
		s.GoodputPerS = float64(s.WithinTarget) / secs
	}
	if len(lat) > 0 {
		s.P50, _ = Percentile(lat, 50)
		s.P99, _ = Percentile(lat, 99)
		s.Max = slices.Max(lat)
	}
	return s
}

// FanoutSummary sums up the fan-out measured by the observers for the sends of a window.
type FanoutSummary struct {
	// Expected counts the dispatches the observers must receive for the 201 sends of the window;
	// Samples those that arrived.
	Expected int `json:"expected"`
	Samples  int `json:"samples"`
	// P50, P99 and Max run from occurred_at of the event (its transaction, an upper bound of the
	// commit) to the receipt of the dispatch at the observer.
	P50 time.Duration `json:"p50_ns"`
	P99 time.Duration `json:"p99_ns"`
	Max time.Duration `json:"max_ns"`
}

// SummarizeFanout sums up the latencies of the dispatches the observers received for the sends of
// a window, of expected dispatches.
func SummarizeFanout(lat []time.Duration, expected int) FanoutSummary {
	f := FanoutSummary{Expected: expected, Samples: len(lat)}
	if len(lat) > 0 {
		f.P50, _ = Percentile(lat, 50)
		f.P99, _ = Percentile(lat, 99)
		f.Max = slices.Max(lat)
	}
	return f
}
