// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"math"
	"slices"
	"strconv"
	"strings"
)

// Kind is the kind of an app metric as the contract names it (ClientMetric.kind).
type Kind string

// The kinds of ClientMetric.kind.
const (
	// KindHistogram is a histogram with buckets.
	KindHistogram Kind = "histogram"
	// KindCounter is a counter increment in value.
	KindCounter Kind = "counter"
	// KindGauge is a gauge value in value.
	KindGauge Kind = "gauge"
)

// catalogue is the closed list of docs/observability/signals.md 4.6a: name in the app and type.
// TestCatalogue_EqualsTheRegistry holds it equal to the metric registry, which the registry's
// own test holds equal to the table.
var catalogue = map[string]Kind{
	"client.cold_start.duration":   KindHistogram,
	"client.thread_open.duration":  KindHistogram,
	"client.send_local.duration":   KindHistogram,
	"client.frames":                KindCounter,
	"client.frames.over_budget":    KindCounter,
	"client.catchup.duration":      KindHistogram,
	"client.catchup.events":        KindCounter,
	"client.resync":                KindCounter,
	"client.reconnects":            KindCounter,
	"client.outbox.depth":          KindGauge,
	"client.push_display.duration": KindHistogram,
	"client.push.placeholder":      KindCounter,
}

// reconnects is the one metric that carries a close code (signals.md 4.6a).
const reconnects = "client.reconnects"

// histogramBounds is the row "durations up to seconds" of signals.md 4.7, the only bounds an app
// histogram may use.
var histogramBounds = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

// KindOf returns the kind of the app metric name and whether the closed list has it.
func KindOf(name string) (Kind, bool) {
	k, ok := catalogue[name]
	return k, ok
}

// Names returns the names of the closed list, sorted; a fresh slice per call.
func Names() []string {
	out := make([]string, 0, len(catalogue))
	for n := range catalogue {
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}

// HistogramBounds returns the bounds of signals.md 4.7 row one; a fresh slice per call.
func HistogramBounds() []float64 { return slices.Clone(histogramBounds) }

// Bucket is one histogram bucket of a batch: upper bound and count.
type Bucket struct {
	LE    float64
	Count int64
}

// Entry is one entry of a batch as the app sent it (schema ClientMetric). CloseCode and Value are
// nil when absent.
type Entry struct {
	Name      string
	Kind      string
	CloseCode *int
	Value     *float64
	Buckets   []Bucket
}

// Batch is a batch of app metrics (schema ClientMetricsBatch); Density and Theme are "" when
// absent.
type Batch struct {
	Platform, AppVersion, Density, Theme string
	Entries                              []Entry
}

// Attrs are the attributes every measurement carries (signals.md 4.6a): each a value of its closed
// list, "other" outside it; Density and Theme "" when the app sent none.
type Attrs struct {
	Platform, Density, Theme, AppMajor string
}

// Measurement is one accepted entry: the name in the app, its kind, the attributes, the close
// code ("" except for client.reconnects), and the value or the buckets.
type Measurement struct {
	Name      string
	Kind      Kind
	Attrs     Attrs
	CloseCode string
	Value     float64
	Buckets   []Bucket
}

// Reason is why an entry was discarded; the values of bothy.reason on
// bothy.client_metrics.discarded.
type Reason string

// The reasons of a discarded entry.
const (
	// ReasonUnknownName: the name is not in the closed list.
	ReasonUnknownName Reason = "unknown_name"
	// ReasonKindMismatch: the kind is not the type of the name.
	ReasonKindMismatch Reason = "kind_mismatch"
	// ReasonInvalidBounds: no buckets, a bound outside 4.7 or a bound named twice.
	ReasonInvalidBounds Reason = "invalid_bounds"
	// ReasonInvalidValue: a negative count, a missing, negative or non-finite counter value, or a
	// missing or non-finite gauge value.
	ReasonInvalidValue Reason = "invalid_value"
)

// Result is the outcome of Filter: the accepted measurements in batch order and the number of
// discarded entries per reason.
type Result struct {
	Accepted  []Measurement
	Discarded map[Reason]int
}

// closed lists of the batch attributes (signals.md 4.6a).
var (
	platforms = map[string]bool{"ios": true, "android": true, "macos": true, "windows": true, "web": true}
	densities = map[string]bool{"comfortable": true, "compact": true, "touch": true}
	themes    = map[string]bool{"light": true, "dark": true}
)

// other is the value of an attribute outside its closed list.
const other = "other"

// Filter checks every entry of b on its own (signals.md 4.6a, ADR 0057): an entry with a name
// outside the closed list, a kind that is not the type of its name, buckets that break the bounds
// of 4.7 or a value outside its rules is discarded with its reason, every other entry is accepted
// with the normalised attributes of the batch. closeCodes are the close codes of 04 6.4; the
// close code of client.reconnects outside them, or a missing one, becomes "other", and every
// other metric drops it. Filter keeps nothing of the batch but these values.
func Filter(b Batch, closeCodes map[int]bool) Result {
	attrs := Attrs{
		Platform: oneOf(b.Platform, platforms), Density: optionalOneOf(b.Density, densities),
		Theme: optionalOneOf(b.Theme, themes), AppMajor: AppMajor(b.AppVersion),
	}
	r := Result{Discarded: map[Reason]int{}}
	for _, e := range b.Entries {
		m, reason := check(e, closeCodes)
		if reason != "" {
			r.Discarded[reason]++
			continue
		}
		m.Attrs = attrs
		r.Accepted = append(r.Accepted, m)
	}
	return r
}

// check returns the measurement of e, or the reason to discard it.
func check(e Entry, closeCodes map[int]bool) (Measurement, Reason) {
	k, ok := catalogue[e.Name]
	if !ok {
		return Measurement{}, ReasonUnknownName
	}
	if Kind(e.Kind) != k {
		return Measurement{}, ReasonKindMismatch
	}
	m := Measurement{Name: e.Name, Kind: k}
	switch k {
	case KindHistogram:
		if reason := checkBuckets(e.Buckets); reason != "" {
			return Measurement{}, reason
		}
		m.Buckets = slices.Clone(e.Buckets)
	case KindCounter:
		if e.Value == nil || !finite(*e.Value) || *e.Value < 0 {
			return Measurement{}, ReasonInvalidValue
		}
		m.Value = *e.Value
	case KindGauge:
		if e.Value == nil || !finite(*e.Value) {
			return Measurement{}, ReasonInvalidValue
		}
		m.Value = *e.Value
	}
	if e.Name == reconnects {
		m.CloseCode = other
		if e.CloseCode != nil && closeCodes[*e.CloseCode] {
			m.CloseCode = strconv.Itoa(*e.CloseCode)
		}
	}
	return m, ""
}

// checkBuckets returns the reason the buckets break 4.7 or the count rule, "" when they are fine:
// at least one bucket, every bound one of 4.7 row one and at most once, every count non-negative.
func checkBuckets(bs []Bucket) Reason {
	if len(bs) == 0 {
		return ReasonInvalidBounds
	}
	seen := make(map[float64]bool, len(bs))
	for _, b := range bs {
		if !slices.Contains(histogramBounds, b.LE) || seen[b.LE] {
			return ReasonInvalidBounds
		}
		seen[b.LE] = true
	}
	for _, b := range bs {
		if b.Count < 0 {
			return ReasonInvalidValue
		}
	}
	return ""
}

// finite reports whether v is neither NaN nor infinite.
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// oneOf returns v when set has it, else "other".
func oneOf(v string, set map[string]bool) string {
	if set[v] {
		return v
	}
	return other
}

// optionalOneOf is oneOf, but keeps "" (absent) as "".
func optionalOneOf(v string, set map[string]bool) string {
	if v == "" {
		return ""
	}
	return oneOf(v, set)
}

// AppMajor returns the major version of the app version v (the part before the first dot) as a
// decimal number without leading zeros when it matches ^[0-9]{1,2}$, else "other" (signals.md
// 4.6a). The rest of v never leaves this function, so a free text in app_version cannot reach the
// telemetry.
func AppMajor(v string) string {
	major, _, _ := strings.Cut(v, ".")
	if len(major) < 1 || len(major) > 2 {
		return other
	}
	for _, c := range major {
		if c < '0' || c > '9' {
			return other
		}
	}
	n, _ := strconv.Atoi(major)
	return strconv.Itoa(n)
}
