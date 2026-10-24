// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// lim are the limits of the tests: stats.max_range and api.stats_max_buckets at their defaults.
var lim = domain.Limits{MaxRange: 400 * 24 * time.Hour, MaxBuckets: 1000}

// berlin is the time zone of the clock change tests.
func berlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// fieldCode returns the code of the one field error of err.
func fieldCode(t *testing.T, err error) string {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 {
		t.Fatalf("error %v, want validation_failed with one field", err)
	}
	return e.Fields[0].Code
}

// TestBuckets: hour, day, week and month alignment in UTC and in Europe/Berlin across both clock
// changes of 2026 (29 March, 25 October); a range over stats.max_range, a range with more than
// api.stats_max_buckets buckets and an empty or inverted range fail.
func TestBuckets(t *testing.T) {
	t.Parallel()
	be := berlin(t)
	utc := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	cases := []struct {
		name     string
		from, to string
		g        domain.Granularity
		loc      *time.Location
		starts   []string
		end      string
	}{
		{"hour in UTC aligns down and up", "2026-10-10T09:30:00Z", "2026-10-10T11:10:00Z", domain.Hour, be,
			[]string{"2026-10-10T09:00:00Z", "2026-10-10T10:00:00Z", "2026-10-10T11:00:00Z"}, "2026-10-10T12:00:00Z"},
		{"day in UTC", "2026-10-10T09:30:00Z", "2026-10-11T00:00:00Z", domain.Day, time.UTC,
			[]string{"2026-10-10T00:00:00Z"}, "2026-10-11T00:00:00Z"},
		{"day in Berlin across the October change: the 25th has 25 hours", "2026-10-24T12:00:00Z", "2026-10-26T12:00:00Z", domain.Day, be,
			[]string{"2026-10-23T22:00:00Z", "2026-10-24T22:00:00Z", "2026-10-25T23:00:00Z"}, "2026-10-26T23:00:00Z"},
		{"day in Berlin across the March change: the 29th has 23 hours", "2026-03-28T12:00:00Z", "2026-03-30T12:00:00Z", domain.Day, be,
			[]string{"2026-03-27T23:00:00Z", "2026-03-28T23:00:00Z", "2026-03-29T22:00:00Z"}, "2026-03-30T22:00:00Z"},
		{"ISO week starts Monday local midnight", "2026-10-21T10:00:00Z", "2026-10-28T10:00:00Z", domain.Week, be,
			[]string{"2026-10-18T22:00:00Z", "2026-10-25T23:00:00Z"}, "2026-11-01T23:00:00Z"},
		{"month in Berlin", "2026-03-15T00:00:00Z", "2026-04-02T00:00:00Z", domain.Month, be,
			[]string{"2026-02-28T23:00:00Z", "2026-03-31T22:00:00Z"}, "2026-04-30T22:00:00Z"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := domain.Align(utc(c.from), utc(c.to), c.g, c.loc, lim)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, s := range r.Starts() {
				got = append(got, s.UTC().Format(time.RFC3339))
			}
			if !slices.Equal(got, c.starts) || !r.To.Equal(utc(c.end)) {
				t.Fatalf("starts %v to %v, want %v to %s", got, r.To.UTC(), c.starts, c.end)
			}
			ends := r.Ends()
			if len(ends) != len(got) || !ends[len(ends)-1].Equal(r.To) {
				t.Fatalf("ends %v", ends)
			}
		})
	}
	if r, _ := domain.Align(utc("2026-10-10T09:30:00Z"), utc("2026-10-10T10:30:00Z"), domain.Hour, be, lim); r.Zone() != "UTC" {
		t.Fatalf("zone of hour buckets %q", r.Zone())
	}
	if r, _ := domain.Align(utc("2026-10-10T09:30:00Z"), utc("2026-10-11T10:30:00Z"), domain.Day, be, lim); r.Zone() != "Europe/Berlin" {
		t.Fatalf("zone of day buckets %q", r.Zone())
	}

	from := utc("2026-01-01T00:00:00Z")
	for _, c := range []struct {
		name     string
		from, to time.Time
		g        domain.Granularity
		code     string
	}{
		{"empty range", from, from, domain.Day, "invalid_range"},
		{"inverted range", from, from.Add(-time.Hour), domain.Day, "invalid_range"},
		{"range over stats.max_range", from, from.Add(lim.MaxRange + time.Hour), domain.Month, "range_too_long"},
		{"more buckets than api.stats_max_buckets", from, from.Add(lim.MaxRange), domain.Hour, "too_many_buckets"},
	} {
		_, err := domain.Align(c.from, c.to, c.g, time.UTC, lim)
		if got := fieldCode(t, err); got != c.code {
			t.Fatalf("%s: code %q, want %q", c.name, got, c.code)
		}
	}
	if _, err := domain.Align(from, from.Add(1000*time.Hour), domain.Hour, time.UTC, lim); err != nil {
		t.Fatalf("exactly api.stats_max_buckets buckets: %v", err)
	}
	if _, err := domain.ParseGranularity("year"); err == nil {
		t.Fatal("unknown granularity accepted")
	}
	if g, err := domain.ParseGranularity(""); err != nil || g != domain.Day {
		t.Fatalf("default granularity %q %v", g, err)
	}
}

// TestMinGroup: active_people below, at and above privacy.stats_min_group_size; other metrics are
// never withheld.
func TestMinGroup(t *testing.T) {
	t.Parallel()
	const minGroup = 5
	for _, c := range []struct {
		people float64
		below  []string
	}{{minGroup - 1, []string{"active_people"}}, {minGroup, nil}, {minGroup + 1, nil}, {0, []string{"active_people"}}} {
		ms := map[string]float64{"active_people": c.people, "messages": 1}
		below := domain.ApplyMinGroup(ms, minGroup)
		if !slices.Equal(below, c.below) {
			t.Fatalf("people %v: below %v, want %v", c.people, below, c.below)
		}
		_, kept := ms["active_people"]
		if kept == (c.below != nil) || ms["messages"] != 1 {
			t.Fatalf("people %v: metrics %v", c.people, ms)
		}
	}
	if below := domain.ApplyMinGroup(map[string]float64{"messages": 1}, minGroup); below != nil {
		t.Fatalf("no people metric, below %v", below)
	}
}

// TestMerge: per-channel aggregates of two spaces merge into the right group keys and sum; a
// channel outside the allowed set and a time outside the range are dropped even when a fake
// source returns them (defence in depth); a nil group map keeps the keys.
func TestMerge(t *testing.T) {
	t.Parallel()
	r, err := domain.Align(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC), domain.Day, time.UTC, lim)
	if err != nil {
		t.Fatal(err)
	}
	day1, day2 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	m := domain.NewMerger(r, map[string]string{"allgemein": "support", "eskalation": "support", "server": "engineering"})
	for _, c := range []struct {
		v    domain.Value
		kept bool
	}{
		{domain.Value{Bucket: day1, Key: "allgemein", Metric: "messages", Value: 2}, true},
		{domain.Value{Bucket: day1.Add(5 * time.Hour), Key: "eskalation", Metric: "messages", Value: 3}, true},
		{domain.Value{Bucket: day2, Key: "server", Metric: "messages", Value: 7}, true},
		{domain.Value{Bucket: day1, Key: "secret", Metric: "messages", Value: 100}, false},
		{domain.Value{Bucket: day2.AddDate(0, 0, 1), Key: "allgemein", Metric: "messages", Value: 100}, false},
		{domain.Value{Bucket: day1.Add(-time.Second), Key: "allgemein", Metric: "messages", Value: 100}, false},
	} {
		if got := m.Add(c.v); got != c.kept {
			t.Fatalf("Add(%+v) = %v", c.v, got)
		}
	}
	got := m.Buckets(5)
	if len(got) != 2 || got[0].Group != "support" || !got[0].Start.Equal(day1) || got[0].Metrics["messages"] != 5 ||
		got[1].Group != "engineering" || !got[1].Start.Equal(day2) || got[1].Metrics["messages"] != 7 {
		t.Fatalf("buckets %+v", got)
	}

	open := domain.NewMerger(r, nil)
	open.Add(domain.Value{Bucket: day1, Key: "search", Metric: "calls", Value: 1})
	if b := open.Buckets(5); len(b) != 1 || b[0].Group != "search" {
		t.Fatalf("open buckets %+v", b)
	}
}
