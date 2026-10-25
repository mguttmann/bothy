// SPDX-License-Identifier: Apache-2.0

package domain

// This file holds the pure rules of the statistics queries (M6-05, docs/observability/
// admin-statistics.md sections 3 and 5): bucket arithmetic in a time zone, the range limits, the
// minimum group and the merge of source aggregates into buckets.

import (
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// Granularity is the size of a statistics bucket (schema StatsGranularity).
type Granularity string

// The granularities. Hour buckets are UTC hours; day, week (ISO 8601, Monday) and month buckets
// start at local midnight of the time zone of the range.
const (
	GranularityHour  Granularity = "hour"
	GranularityDay   Granularity = "day"
	GranularityWeek  Granularity = "week"
	GranularityMonth Granularity = "month"
)

// ParseGranularity returns the granularity of s; "" is GranularityDay. Errors: validation_failed
// for any other value (the schema validation of the chain normally refuses it first).
func ParseGranularity(s string) (Granularity, error) {
	switch Granularity(s) {
	case "":
		return GranularityDay, nil
	case GranularityHour, GranularityDay, GranularityWeek, GranularityMonth:
		return Granularity(s), nil
	}
	return "", fieldError("/granularity", "invalid", "Unknown granularity.")
}

// Limits are the bounds of a range: MaxRange is stats.max_range, MaxBuckets api.stats_max_buckets.
type Limits struct {
	MaxRange   time.Duration
	MaxBuckets int
}

// Range is an aligned half-open range of buckets: From is the start of the first bucket and To the
// end of the last, both instants; Location is the time zone of the bucket starts (UTC for Hour).
type Range struct {
	From, To    time.Time
	Granularity Granularity
	Location    *time.Location
	starts      []time.Time
}

// Align checks the requested range from, to and aligns it to g in loc (UTC for Hour): From down to
// the start of its bucket, To up to the end of its bucket. A bucket keeps its local start across a
// clock change, so a day bucket lasts 23 or 25 hours then.
//
// Errors (validation_failed with a field error): from not before to (/from, code invalid_range),
// a requested range longer than lim.MaxRange (/to, code range_too_long) or more than
// lim.MaxBuckets buckets after alignment (/granularity, code too_many_buckets).
//
// Example:
//
//	r, err := domain.Align(from, to, domain.Day, berlin, domain.Limits{MaxRange: 400 * 24 * time.Hour, MaxBuckets: 1000})
func Align(from, to time.Time, g Granularity, loc *time.Location, lim Limits) (Range, error) {
	if !from.Before(to) {
		return Range{}, fieldError("/from", "invalid_range", "from must be before to.")
	}
	if to.Sub(from) > lim.MaxRange {
		return Range{}, fieldError("/to", "range_too_long", "The range is longer than stats.max_range.")
	}
	if g == GranularityHour || loc == nil {
		loc = time.UTC
	}
	r := Range{From: Floor(from, g, loc), Granularity: g, Location: loc}
	end := Floor(to, g, loc)
	if end.Before(to) {
		end = next(end, g, loc)
	}
	r.To = end
	for s := r.From; s.Before(end); s = next(s, g, loc) {
		if len(r.starts) == lim.MaxBuckets {
			return Range{}, fieldError("/granularity", "too_many_buckets", "The range has more buckets than api.stats_max_buckets.")
		}
		r.starts = append(r.starts, s)
	}
	return r, nil
}

// Starts returns the bucket starts in order (a copy).
func (r Range) Starts() []time.Time { return slices.Clone(r.starts) }

// Ends returns the bucket ends in order, parallel to Starts.
func (r Range) Ends() []time.Time {
	out := make([]time.Time, len(r.starts))
	for i := range r.starts {
		if i+1 < len(r.starts) {
			out[i] = r.starts[i+1]
		} else {
			out[i] = r.To
		}
	}
	return out
}

// Zone is the IANA name the sources truncate in: "UTC" for Hour, else the name of Location.
func (r Range) Zone() string {
	if r.Granularity == GranularityHour || r.Location == nil {
		return "UTC"
	}
	return r.Location.String()
}

// Last is the start of the last bucket.
func (r Range) Last() time.Time { return r.starts[len(r.starts)-1] }

// BucketOf returns the start of the bucket that holds t; false outside the range.
func (r Range) BucketOf(t time.Time) (time.Time, bool) {
	if t.Before(r.From) || !t.Before(r.To) {
		return time.Time{}, false
	}
	i := sort.Search(len(r.starts), func(i int) bool { return r.starts[i].After(t) })
	return r.starts[i-1], true
}

// Floor returns the start of the bucket of g in loc that holds t (hour buckets in UTC).
func Floor(t time.Time, g Granularity, loc *time.Location) time.Time {
	l := t.In(loc)
	switch g {
	case GranularityHour:
		return t.UTC().Truncate(time.Hour)
	case GranularityWeek:
		day := time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, loc)
		return day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
	case GranularityMonth:
		return time.Date(l.Year(), l.Month(), 1, 0, 0, 0, 0, loc)
	case GranularityDay:
		return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, loc)
	}
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, loc)
}

// next returns the start of the bucket after the one starting at s.
func next(s time.Time, g Granularity, loc *time.Location) time.Time {
	switch g {
	case GranularityHour:
		return s.Add(time.Hour)
	case GranularityWeek:
		return s.In(loc).AddDate(0, 0, 7)
	case GranularityMonth:
		return s.In(loc).AddDate(0, 1, 0)
	case GranularityDay:
		return s.In(loc).AddDate(0, 0, 1)
	}
	return s.In(loc).AddDate(0, 0, 1)
}

// PeopleMetrics are the metrics that count people; only they fall under the minimum group
// (admin-statistics.md section 3 item 2).
var PeopleMetrics = []string{"active_people"}

// ApplyMinGroup removes from metrics every people metric whose value is below minGroup
// (privacy.stats_min_group_size) and returns the removed names in the order of PeopleMetrics; nil
// when none was removed. No other metric changes.
//
// Example:
//
//	below := domain.ApplyMinGroup(map[string]float64{"active_people": 3, "messages": 40}, 5) // ["active_people"]
func ApplyMinGroup(metrics map[string]float64, minGroup int) []string {
	var below []string
	for _, name := range PeopleMetrics {
		if v, ok := metrics[name]; ok && v < float64(minGroup) {
			delete(metrics, name)
			below = append(below, name)
		}
	}
	return below
}

// Value is one aggregate of a source: the value of Metric for the key Key (a channel, space,
// agent or tool name, as the source groups) in the bucket that holds Bucket.
type Value struct {
	Bucket time.Time
	Key    string
	Metric string
	Value  float64
}

// StatsBucket is one merged bucket of one group.
type StatsBucket struct {
	Start         time.Time
	Group         string
	Metrics       map[string]float64
	BelowMinGroup []string
}

// Merger collects the aggregates of the sources into buckets of a range and groups. It is the
// defence in depth of the permission filter: a value whose key is not in the allowed keys, or
// whose time is outside the range, is dropped even when a source returned it. Values of the same
// bucket, group and metric are summed, so per-channel counts add up to their space.
type Merger struct {
	r       Range
	groupOf map[string]string
	buckets map[mergeKey]map[string]float64
}

// mergeKey is a bucket of one group.
type mergeKey struct {
	start int64
	group string
}

// NewMerger returns a merger over r. groupOf maps an allowed key to its group key; nil allows
// every key and keeps it as the group (agents and tool names, which the thread set already
// bounds).
func NewMerger(r Range, groupOf map[string]string) *Merger {
	return &Merger{r: r, groupOf: groupOf, buckets: map[mergeKey]map[string]float64{}}
}

// Add adds v and reports whether it was kept.
func (m *Merger) Add(v Value) bool {
	group := v.Key
	if m.groupOf != nil {
		g, ok := m.groupOf[v.Key]
		if !ok {
			return false
		}
		group = g
	}
	start, ok := m.r.BucketOf(v.Bucket)
	if !ok {
		return false
	}
	k := mergeKey{start.UnixNano(), group}
	ms := m.buckets[k]
	if ms == nil {
		ms = map[string]float64{}
		m.buckets[k] = ms
	}
	ms[v.Metric] += v.Value
	return true
}

// Each calls f for every bucket with its metrics, which f may change (derived rates, removal).
func (m *Merger) Each(f func(start time.Time, group string, metrics map[string]float64)) {
	for k, ms := range m.buckets {
		f(time.Unix(0, k.start).UTC(), k.group, ms)
	}
}

// Buckets returns the buckets that hold at least one metric, ordered by start, then group, with
// the minimum group applied (minGroup is privacy.stats_min_group_size).
func (m *Merger) Buckets(minGroup int) []StatsBucket {
	out := make([]StatsBucket, 0, len(m.buckets))
	for k, ms := range m.buckets {
		below := ApplyMinGroup(ms, minGroup)
		if len(ms) == 0 && below == nil {
			continue
		}
		out = append(out, StatsBucket{Start: time.Unix(0, k.start).UTC(), Group: k.group, Metrics: ms, BelowMinGroup: below})
	}
	slices.SortFunc(out, func(a, b StatsBucket) int {
		if c := a.Start.Compare(b.Start); c != 0 {
			return c
		}
		return strings.Compare(a.Group, b.Group)
	})
	return out
}

// fieldError is a validation_failed error with one field error.
func fieldError(pointer, code, detail string) error {
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}
