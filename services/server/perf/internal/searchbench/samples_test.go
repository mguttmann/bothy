// SPDX-License-Identifier: Apache-2.0

package searchbench_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/perf/internal/report"
	"github.com/OWNER/bothy/services/server/perf/internal/searchbench"
)

// TestSamples_Window: samples of the warm-up and after the window are left out; the p95 is the
// nearest rank of the report; fewer than MinSamples fail; per class and band the counts add up.
func TestSamples_Window(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	from, to := start.Add(searchbench.Warmup), start.Add(searchbench.Warmup+searchbench.Window)
	var all []searchbench.Sample
	classes := []searchbench.Class{searchbench.ClassOneWord, searchbench.ClassPhrase, searchbench.ClassNegation}
	bands := []searchbench.Band{searchbench.BandFrequent, searchbench.BandNone, searchbench.BandRare}
	for i := range 3000 {
		at := start.Add(time.Duration(i) * 100 * time.Millisecond)
		lat := time.Duration(1+i%100) * time.Millisecond
		if at.Before(from) || !at.Before(to) {
			lat = time.Hour // must never count
		}
		all = append(all, searchbench.Sample{Scheduled: at, Done: at.Add(lat), Capped: i%10 == 0,
			Query: searchbench.Query{Class: classes[i%3], Band: bands[i%3]}})
	}
	in := searchbench.InWindow(all, from, to)
	if want := int(searchbench.Window / (100 * time.Millisecond)); len(in) != want {
		t.Fatalf("%d samples in the window, want %d", len(in), want)
	}
	st, err := searchbench.StatsOf(in, searchbench.MinSamples)
	if err != nil {
		t.Fatal(err)
	}
	lat := make([]time.Duration, len(in))
	for i, s := range in {
		lat[i] = s.Latency()
	}
	want, _ := report.DurationsP(lat, 95)
	if st.P95 != want || st.All.Max >= float64(time.Hour/time.Millisecond) {
		t.Errorf("p95 %v, want %v; max %v ms", st.P95, want, st.All.Max)
	}
	sumClass, sumBand := 0, 0
	for _, d := range st.ByClass {
		sumClass += d.Count
	}
	for _, d := range st.ByBand {
		sumBand += d.Count
	}
	banded := 0
	for _, s := range in {
		if s.Query.Band != searchbench.BandNone {
			banded++
		}
	}
	if sumClass != len(in) || sumBand != banded {
		t.Errorf("per class %d, per band %d; want %d and %d", sumClass, sumBand, len(in), banded)
	}
	if st.CappedShare <= 0 || st.CappedShare >= 1 {
		t.Errorf("capped share %g", st.CappedShare)
	}
	if _, err := searchbench.StatsOf(in[:searchbench.MinSamples-1], searchbench.MinSamples); err == nil {
		t.Error("fewer than MinSamples samples passed")
	}
}
