// SPDX-License-Identifier: Apache-2.0

package searchbench

import (
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/perf/internal/report"
)

// Sample is one request of a phase: the query, the scheduled send time and the moment the whole
// answer was read (the measuring point of design note 7), whether the answer reported
// candidates_capped, and the failure of its checks.
type Sample struct {
	Phase     string
	Query     Query
	Scheduled time.Time
	Done      time.Time
	Capped    bool
	Err       error
}

// Latency is the time from the scheduled send to the fully read answer: an upper bound of the
// server time that avoids coordinated omission.
func (s Sample) Latency() time.Duration { return s.Done.Sub(s.Scheduled) }

// InWindow returns the samples scheduled at or after from and before to, the measured window of
// a phase (design note 7): warm-up and samples after the window are left out.
func InWindow(samples []Sample, from, to time.Time) []Sample {
	var out []Sample
	for _, s := range samples {
		if !s.Scheduled.Before(from) && s.Scheduled.Before(to) {
			out = append(out, s)
		}
	}
	return out
}

// Dist is the distribution of a set of samples in milliseconds.
type Dist struct {
	Count              int
	P50, P95, P99, Max float64
}

// distOf returns the nearest-rank percentiles of the latencies of samples (report.NearestRank).
func distOf(samples []Sample) Dist {
	d := Dist{Count: len(samples)}
	if len(samples) == 0 {
		return d
	}
	xs := make([]time.Duration, len(samples))
	for i, s := range samples {
		xs[i] = s.Latency()
	}
	for _, x := range []struct {
		p   float64
		dst *float64
	}{{50, &d.P50}, {95, &d.P95}, {99, &d.P99}, {100, &d.Max}} {
		v, _ := report.DurationsP(xs, x.p)
		*x.dst = v / float64(time.Millisecond)
	}
	return d
}

// Stats are the statistics of a phase: the distribution of all samples, per query class and per
// frequency band, and the share of answers with candidates_capped.
type Stats struct {
	All         Dist
	ByClass     map[Class]Dist
	ByBand      map[Band]Dist
	CappedShare float64
	// P95 is the nearest-rank p95 of all samples in nanoseconds, the value of the target.
	P95 float64
}

// StatsOf returns the statistics of the window samples of a phase. The error reports fewer than
// minSamples samples; failed samples are counted like the others (the checks fail the run).
func StatsOf(samples []Sample, minSamples int) (Stats, error) {
	if len(samples) < minSamples {
		return Stats{}, fmt.Errorf("searchbench: %d samples in the window, fewer than MinSamples %d", len(samples), minSamples)
	}
	st := Stats{All: distOf(samples), ByClass: map[Class]Dist{}, ByBand: map[Band]Dist{}}
	byClass, byBand := map[Class][]Sample{}, map[Band][]Sample{}
	capped := 0
	xs := make([]time.Duration, len(samples))
	for i, s := range samples {
		xs[i] = s.Latency()
		byClass[s.Query.Class] = append(byClass[s.Query.Class], s)
		if s.Query.Band != BandNone {
			byBand[s.Query.Band] = append(byBand[s.Query.Band], s)
		}
		if s.Capped {
			capped++
		}
	}
	for c, xs := range byClass {
		st.ByClass[c] = distOf(xs)
	}
	for b, xs := range byBand {
		st.ByBand[b] = distOf(xs)
	}
	st.CappedShare = float64(capped) / float64(len(samples))
	p95, err := report.DurationsP(xs, 95)
	if err != nil {
		return Stats{}, err
	}
	st.P95 = p95
	return st, nil
}

// Map returns the statistics for extra.search of results.json.
func (st Stats) Map() map[string]any {
	dist := func(d Dist) map[string]any {
		return map[string]any{"count": d.Count, "p50_ms": d.P50, "p95_ms": d.P95, "p99_ms": d.P99, "max_ms": d.Max}
	}
	classes, bands := map[string]any{}, map[string]any{}
	for _, c := range slices.Sorted(maps.Keys(st.ByClass)) {
		classes[string(c)] = dist(st.ByClass[c])
	}
	for _, b := range slices.Sorted(maps.Keys(st.ByBand)) {
		bands[string(b)] = dist(st.ByBand[b])
	}
	return map[string]any{"all": dist(st.All), "by_class": classes, "by_band": bands, "candidates_capped_share": st.CappedShare}
}

// Failures returns the errors of the samples (warm-up included: every answer must be correct).
func Failures(samples []Sample) []error {
	var out []error
	for _, s := range samples {
		if s.Err != nil {
			out = append(out, s.Err)
		}
	}
	return out
}
