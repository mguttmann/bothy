// SPDX-License-Identifier: Apache-2.0

package load

import (
	"testing"
	"time"
)

func TestStats_NearestRank(t *testing.T) {
	t.Parallel()
	ms := func(xs ...int) []time.Duration {
		out := make([]time.Duration, len(xs))
		for i, x := range xs {
			out[i] = time.Duration(x) * time.Millisecond
		}
		return out
	}
	samples := ms(5, 1, 4, 2, 3, 10, 9, 8, 7, 6)
	for p, want := range map[float64]time.Duration{50: 5 * time.Millisecond, 90: 9 * time.Millisecond, 99: 10 * time.Millisecond, 100: 10 * time.Millisecond, 10: time.Millisecond} {
		got, err := Percentile(samples, p)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("p%g = %v, want %v", p, got, want)
		}
	}
	if samples[0] != 5*time.Millisecond {
		t.Error("Percentile sorted the caller's slice")
	}
	if _, err := Percentile(nil, 99); err == nil {
		t.Error("no samples passed")
	}
	if _, err := Percentile(samples, 0); err == nil {
		t.Error("p0 passed")
	}
}

func TestStats_SummarizeSends(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	from, to := start.Add(10*time.Second), start.Add(20*time.Second)
	target := 100 * time.Millisecond
	var sends []Send
	// Before the window: never counted.
	sends = append(sends, Send{Scheduled: start, Done: start.Add(time.Second), Status: 201})
	// In the window: 8 within the target, 1 slow, 1 failed.
	for i := range 8 {
		at := from.Add(time.Duration(i) * time.Second)
		sends = append(sends, Send{Scheduled: at, Done: at.Add(20 * time.Millisecond), Status: 201})
	}
	sends = append(sends, Send{Scheduled: from.Add(8 * time.Second), Done: from.Add(8*time.Second + 300*time.Millisecond), Status: 201})
	sends = append(sends, Send{Scheduled: from.Add(9 * time.Second), Done: from.Add(9*time.Second + 10*time.Millisecond), Status: 500})
	// After the window: never counted.
	sends = append(sends, Send{Scheduled: to, Done: to.Add(time.Millisecond), Status: 201})
	s := SummarizeSends(sends, from, to, target)
	if s.Scheduled != 10 || s.OK != 9 || s.Failed != 1 || s.WithinTarget != 8 {
		t.Fatalf("summary %+v", s)
	}
	if s.AchievedPerS != 0.9 || s.GoodputPerS != 0.8 {
		t.Errorf("achieved %g, goodput %g; want 0.9 and 0.8 per second", s.AchievedPerS, s.GoodputPerS)
	}
	if s.P99 != 300*time.Millisecond || s.Max != 300*time.Millisecond {
		t.Errorf("p99 %v, max %v", s.P99, s.Max)
	}
	if len(s.FailedStatuses) != 1 || s.FailedStatuses[500] != 1 {
		t.Errorf("failed statuses %v", s.FailedStatuses)
	}
}
