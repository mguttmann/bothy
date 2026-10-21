// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// Bucket is one bucket of a histogram a client aggregated itself (ClientMetric.buckets): LE is
// the upper bound, Count the number of values in the bucket that ends at LE.
type Bucket struct {
	LE    float64
	Count uint64
}

// ErrBounds reports buckets the histogram path cannot carry without distortion: none at all, a
// bound that is not one of the spec's (signals.md 4.7), or a bound named twice.
var ErrBounds = errors.New("metrics: bucket bounds outside the catalogue row")

// AggregatedSet holds the histograms whose values arrive already bucketed (the app histograms of
// signals.md 4.6a, ADR 0057) and hands them to the exporter as cumulative OTLP histogram data
// points. It is an sdkmetric.Producer: a reader built with sdkmetric.WithProducer(set) exports its
// points next to the meter provider's own, so the bucket counts reach the collector exactly as
// the clients sent them, with the bounds of the spec (docs/DECISIONS.md D-M4-31-1). It is safe for
// concurrent use; Add never blocks on the export.
type AggregatedSet struct {
	mu    sync.Mutex
	hists []*AggregatedHistogram
	byKey map[string]*AggregatedHistogram
	now   func() time.Time
}

// NewAggregatedSet returns an empty set.
//
// Example:
//
//	set := metrics.NewAggregatedSet()
//	reader := sdkmetric.NewManualReader(sdkmetric.WithProducer(set))
//	h, err := set.Histogram(metrics.ClientThreadOpenDuration, nil)
//	err = h.Add([]metrics.Bucket{{LE: 0.1, Count: 12}}, "ios", "touch", "dark", "1")
func NewAggregatedSet() *AggregatedSet {
	return &AggregatedSet{byKey: map[string]*AggregatedHistogram{}, now: time.Now}
}

// defaultAggregated is the set of the process; see DefaultAggregated.
var defaultAggregated = NewAggregatedSet()

// DefaultAggregated returns the set of the process. platform/otel attaches it to the OTLP metric
// reader in Setup, like the meter provider it installs globally; the stats module records into
// it, and test readers attach it themselves (sdkmetric.WithProducer).
func DefaultAggregated() *AggregatedSet { return defaultAggregated }

// Histogram returns the histogram of s in the set, created on the first call; a later call with
// the same name returns the first histogram. Errors: ErrSpec when s fails Check, is no histogram,
// or lacks the values of a Source attribute.
func (set *AggregatedSet) Histogram(s Spec, v Values) (*AggregatedHistogram, error) {
	b, err := newBinder(s, []Kind{KindHistogram}, v)
	if err != nil {
		return nil, err
	}
	set.mu.Lock()
	defer set.mu.Unlock()
	if h, ok := set.byKey[s.Name]; ok {
		return h, nil
	}
	h := &AggregatedHistogram{set: set, b: b, bounds: slices.Clone(s.Buckets), points: map[string]*aggregatedPoint{}}
	set.byKey[s.Name] = h
	set.hists = append(set.hists, h)
	return h, nil
}

// Produce implements sdkmetric.Producer: one scope with one cumulative histogram per histogram
// that holds points, each point with the spec's bounds, its bucket counts, count, sum and the time
// of its first value as start. Min and max are absent: buckets do not give them.
func (set *AggregatedSet) Produce(context.Context) ([]metricdata.ScopeMetrics, error) {
	set.mu.Lock()
	defer set.mu.Unlock()
	now := set.now()
	var ms []metricdata.Metrics
	for _, h := range set.hists {
		if len(h.points) == 0 {
			continue
		}
		keys := make([]string, 0, len(h.points))
		for k := range h.points {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		dps := make([]metricdata.HistogramDataPoint[float64], 0, len(keys))
		for _, k := range keys {
			p := h.points[k]
			dps = append(dps, metricdata.HistogramDataPoint[float64]{
				Attributes: p.attrs, StartTime: p.start, Time: now, Count: p.count, Sum: p.sum,
				Bounds: slices.Clone(h.bounds), BucketCounts: slices.Clone(p.counts),
			})
		}
		ms = append(ms, metricdata.Metrics{
			Name: h.b.spec.Name, Description: h.b.spec.Description, Unit: h.b.spec.Unit,
			Data: metricdata.Histogram[float64]{Temporality: metricdata.CumulativeTemporality, DataPoints: dps},
		})
	}
	if len(ms) == 0 {
		return nil, nil
	}
	return []metricdata.ScopeMetrics{{Scope: instrumentation.Scope{Name: Scope}, Metrics: ms}}, nil
}

// AggregatedHistogram is one histogram of an AggregatedSet.
type AggregatedHistogram struct {
	set    *AggregatedSet
	b      *binder
	bounds []float64
	points map[string]*aggregatedPoint // by attribute set, guarded by set.mu
}

// aggregatedPoint is the cumulative state of one attribute set.
type aggregatedPoint struct {
	attrs  attribute.Set
	start  time.Time
	counts []uint64 // len(bounds)+1; the last is the overflow bucket above the top bound
	count  uint64
	sum    float64
}

// Add adds the buckets of one client entry with the attribute values in the order of the spec
// (each outside its closed set records as "other"). Every bound must be one of the spec's bounds,
// at most once; any subset is fine (the request example of submitClientMetrics sends two). The
// sum grows by bound times count: the clients send no sum, so the exported sum is an upper bound
// of the true one, while counts per bound and the total count are exact. Counts saturate at the
// uint64 maximum instead of wrapping. Errors: ErrBounds; then nothing is added.
func (h *AggregatedHistogram) Add(buckets []Bucket, values ...string) error {
	if len(buckets) == 0 {
		return fmt.Errorf("%w: no buckets", ErrBounds)
	}
	idx := make([]int, len(buckets))
	seen := make(map[int]bool, len(buckets))
	for i, b := range buckets {
		j := slices.Index(h.bounds, b.LE)
		if j < 0 || seen[j] {
			return fmt.Errorf("%w: %s", ErrBounds, h.b.spec.Name)
		}
		seen[j], idx[i] = true, j
	}
	set := h.b.attrs(values)
	key := set.Encoded(attribute.DefaultEncoder())
	h.set.mu.Lock()
	defer h.set.mu.Unlock()
	p, ok := h.points[key]
	if !ok {
		p = &aggregatedPoint{attrs: set, start: h.set.now(), counts: make([]uint64, len(h.bounds)+1)}
		h.points[key] = p
	}
	for i, b := range buckets {
		p.counts[idx[i]] = addSaturating(p.counts[idx[i]], b.Count)
		p.count = addSaturating(p.count, b.Count)
		p.sum += b.LE * float64(b.Count)
	}
	return nil
}

// addSaturating returns a+b, or the uint64 maximum when the sum would overflow.
func addSaturating(a, b uint64) uint64 {
	if a > math.MaxUint64-b {
		return math.MaxUint64
	}
	return a + b
}
