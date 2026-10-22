// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Scope is the instrumentation scope of every instrument this package creates.
const Scope = "github.com/OWNER/bothy/services/server"

// Values passes the closed sets of the Source attributes of a spec at construction: the owner of
// the instrument knows them (the rate-limit classes, the job kinds registered at startup, ...).
type Values map[Source][]string

// ErrSpec reports a spec the constructors cannot use: a structural problem (Check), the wrong
// kind for the constructor, or a Source attribute whose values were not passed.
var ErrSpec = errors.New("metrics: unusable instrument spec")

// strictReport is the strict switch: nil in production, where an unknown value becomes "other";
// set by metricstest.Strict, which fails the test instead.
var strictReport atomic.Pointer[func(error)]

// SetStrict turns the strict switch on with report, which receives every unknown attribute value
// and every value tuple of the wrong length; restore turns it back to the previous state. Test
// helpers call it (metricstest.Strict); production never does. The value still records as
// "other", so the strict switch never changes what a request returns.
func SetStrict(report func(error)) (restore func()) {
	prev := strictReport.Swap(&report)
	return func() { strictReport.Store(prev) }
}

// binder turns a value tuple into the cached attribute set of a spec.
type binder struct {
	spec  Spec
	sets  []map[string]bool
	forms []*regexp.Regexp // the form of a form source, nil for a list
	cache sync.Map         // string -> attribute.Set
}

// newBinder checks s against kind and builds the closed sets from the literal values and v; a
// pattern source takes no values from v.
func newBinder(s Spec, kinds []Kind, v Values) (*binder, error) {
	if problems := Check([]Spec{s}); len(problems) > 0 {
		return nil, fmt.Errorf("%w: %w", ErrSpec, errors.Join(problems...))
	}
	ok := false
	for _, k := range kinds {
		ok = ok || s.Kind == k
	}
	if !ok {
		return nil, fmt.Errorf("%w: %s is a %s", ErrSpec, s.Name, s.Kind)
	}
	b := &binder{spec: s, sets: make([]map[string]bool, len(s.Attrs)), forms: make([]*regexp.Regexp, len(s.Attrs))}
	for i, a := range s.Attrs {
		vals := a.Values
		if re, ok := formSources[a.Source]; ok {
			b.forms[i], vals = re, nil
		} else if a.Source != "" {
			got, given := v[a.Source]
			if !given {
				return nil, fmt.Errorf("%w: %s: no values for the source %s of %s", ErrSpec, s.Name, a.Source, a.Key)
			}
			vals = got
		}
		set := make(map[string]bool, len(vals)+1)
		for _, x := range vals {
			set[x] = true
		}
		set[Other] = true
		b.sets[i] = set
	}
	return b, nil
}

// attrs returns the attribute set of values, each outside its closed set (or its pattern)
// replaced by "other".
// The sets are cached by the normalised tuple, so the cache stays as small as the closed sets.
func (b *binder) attrs(values []string) attribute.Set {
	if len(values) != len(b.spec.Attrs) {
		report(fmt.Errorf("metrics: %s takes %d attribute values, got %d", b.spec.Name, len(b.spec.Attrs), len(values)))
	}
	norm := make([]string, len(b.spec.Attrs))
	for i, a := range b.spec.Attrs {
		v := ""
		if i < len(values) {
			v = values[i]
		}
		if (v != "" || !a.Optional) && !b.sets[i][v] && (b.forms[i] == nil || !b.forms[i].MatchString(v)) {
			report(fmt.Errorf("metrics: %s: value %q of %s is not in its closed set", b.spec.Name, v, a.Key))
			v = Other
		}
		norm[i] = v
	}
	key := strings.Join(norm, "\x00")
	if s, ok := b.cache.Load(key); ok {
		return s.(attribute.Set)
	}
	kvs := make([]attribute.KeyValue, 0, len(norm))
	for i, v := range norm {
		if v != "" {
			kvs = append(kvs, attribute.String(b.spec.Attrs[i].Key, v))
		}
	}
	set := attribute.NewSet(kvs...)
	b.cache.Store(key, set)
	return set
}

// report hands err to the strict switch when it is on.
func report(err error) {
	if r := strictReport.Load(); r != nil && *r != nil {
		(*r)(err)
	}
}

// meterOf returns the meter of this package on mp; nil means the global provider that
// platform/otel installs in Setup.
func meterOf(mp metric.MeterProvider) metric.Meter {
	if mp == nil {
		mp = otelapi.GetMeterProvider()
	}
	return mp.Meter(Scope)
}

// Counter is a monotonic counter of the catalogue.
type Counter struct {
	b   *binder
	i64 metric.Int64Counter
	f64 metric.Float64Counter
}

// NewCounter creates the counter s on mp (nil means the global provider). Errors: ErrSpec, or an
// error of the meter provider.
func NewCounter(mp metric.MeterProvider, s Spec, v Values) (*Counter, error) {
	b, err := newBinder(s, []Kind{KindCounter}, v)
	if err != nil {
		return nil, err
	}
	c := &Counter{b: b}
	opts := []metric.Int64CounterOption{metric.WithUnit(s.Unit), metric.WithDescription(s.Description)}
	if s.Float {
		c.f64, err = meterOf(mp).Float64Counter(s.Name, metric.WithUnit(s.Unit), metric.WithDescription(s.Description))
	} else {
		c.i64, err = meterOf(mp).Int64Counter(s.Name, opts...)
	}
	if err != nil {
		return nil, fmt.Errorf("metrics: %s: %w", s.Name, err)
	}
	return c, nil
}

// Add adds n with the attribute values in the order of the spec; a negative n is ignored.
func (c *Counter) Add(ctx context.Context, n int64, values ...string) {
	if n < 0 {
		return
	}
	opt := metric.WithAttributeSet(c.b.attrs(values))
	if c.f64 != nil {
		c.f64.Add(ctx, float64(n), opt)
		return
	}
	c.i64.Add(ctx, n, opt)
}

// AddFloat adds v with the attribute values in the order of the spec: the increments the apps send
// as JSON numbers (signals.md 4.6a). A negative, NaN or infinite v is ignored; on an int64 counter
// v is truncated toward zero.
func (c *Counter) AddFloat(ctx context.Context, v float64, values ...string) {
	if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return
	}
	if c.f64 == nil {
		c.Add(ctx, int64(v), values...)
		return
	}
	c.f64.Add(ctx, v, metric.WithAttributeSet(c.b.attrs(values)))
}

// UpDownCounter is a counter of the catalogue that also decreases.
type UpDownCounter struct {
	b *binder
	i metric.Int64UpDownCounter
}

// NewUpDownCounter creates the up-down counter s on mp (nil means the global provider). Errors:
// ErrSpec, or an error of the meter provider.
func NewUpDownCounter(mp metric.MeterProvider, s Spec, v Values) (*UpDownCounter, error) {
	b, err := newBinder(s, []Kind{KindUpDownCounter}, v)
	if err != nil {
		return nil, err
	}
	i, err := meterOf(mp).Int64UpDownCounter(s.Name, metric.WithUnit(s.Unit), metric.WithDescription(s.Description))
	if err != nil {
		return nil, fmt.Errorf("metrics: %s: %w", s.Name, err)
	}
	return &UpDownCounter{b: b, i: i}, nil
}

// Add adds n (negative to decrease) with the attribute values in the order of the spec.
func (c *UpDownCounter) Add(ctx context.Context, n int64, values ...string) {
	c.i.Add(ctx, n, metric.WithAttributeSet(c.b.attrs(values)))
}

// Histogram is a histogram of the catalogue; the view gives it the bounds of its spec.
type Histogram struct {
	b *binder
	i metric.Float64Histogram
}

// NewHistogram creates the histogram s on mp (nil means the global provider) with the bucket
// bounds of s also as the instrument's advice. Errors: ErrSpec, or an error of the meter
// provider.
func NewHistogram(mp metric.MeterProvider, s Spec, v Values) (*Histogram, error) {
	b, err := newBinder(s, []Kind{KindHistogram}, v)
	if err != nil {
		return nil, err
	}
	i, err := meterOf(mp).Float64Histogram(s.Name, metric.WithUnit(s.Unit), metric.WithDescription(s.Description),
		metric.WithExplicitBucketBoundaries(s.Buckets...))
	if err != nil {
		return nil, fmt.Errorf("metrics: %s: %w", s.Name, err)
	}
	return &Histogram{b: b, i: i}, nil
}

// Record records v with the attribute values in the order of the spec.
func (h *Histogram) Record(ctx context.Context, v float64, values ...string) {
	h.i.Record(ctx, v, metric.WithAttributeSet(h.b.attrs(values)))
}

// RecordDuration records d in seconds; a negative d records 0.
func (h *Histogram) RecordDuration(ctx context.Context, d time.Duration, values ...string) {
	h.Record(ctx, max(d, 0).Seconds(), values...)
}

// Gauge is a synchronous float64 gauge of the catalogue.
type Gauge struct {
	b *binder
	i metric.Float64Gauge
}

// NewGauge creates the synchronous gauge s on mp (nil means the global provider). Errors:
// ErrSpec, or an error of the meter provider.
func NewGauge(mp metric.MeterProvider, s Spec, v Values) (*Gauge, error) {
	b, err := newBinder(s, []Kind{KindGauge}, v)
	if err != nil {
		return nil, err
	}
	i, err := meterOf(mp).Float64Gauge(s.Name, metric.WithUnit(s.Unit), metric.WithDescription(s.Description))
	if err != nil {
		return nil, fmt.Errorf("metrics: %s: %w", s.Name, err)
	}
	return &Gauge{b: b, i: i}, nil
}

// Record sets the gauge to v with the attribute values in the order of the spec.
func (g *Gauge) Record(ctx context.Context, v float64, values ...string) {
	g.i.Record(ctx, v, metric.WithAttributeSet(g.b.attrs(values)))
}

// ObservableGauge is an asynchronous gauge of the catalogue; its owner registers a callback
// (Register, or metric.Meter.RegisterCallback with Observable for several gauges at once).
type ObservableGauge struct {
	b     *binder
	meter metric.Meter
	i64   metric.Int64ObservableGauge
	f64   metric.Float64ObservableGauge
}

// NewObservableGauge creates the observable gauge s on mp (nil means the global provider): int64
// unless s.Float. Errors: ErrSpec, or an error of the meter provider.
func NewObservableGauge(mp metric.MeterProvider, s Spec, v Values) (*ObservableGauge, error) {
	b, err := newBinder(s, []Kind{KindGauge}, v)
	if err != nil {
		return nil, err
	}
	g := &ObservableGauge{b: b, meter: meterOf(mp)}
	if s.Float {
		g.f64, err = g.meter.Float64ObservableGauge(s.Name, metric.WithUnit(s.Unit), metric.WithDescription(s.Description))
	} else {
		g.i64, err = g.meter.Int64ObservableGauge(s.Name, metric.WithUnit(s.Unit), metric.WithDescription(s.Description))
	}
	if err != nil {
		return nil, fmt.Errorf("metrics: %s: %w", s.Name, err)
	}
	return g, nil
}

// Observable returns the instrument, for metric.Meter.RegisterCallback.
func (g *ObservableGauge) Observable() metric.Observable {
	if g.f64 != nil {
		return g.f64
	}
	return g.i64
}

// Meter returns the meter the gauge was created on, for a callback over several gauges.
func (g *ObservableGauge) Meter() metric.Meter { return g.meter }

// Observe observes v (rounded toward zero for an int64 gauge) with the attribute values in the
// order of the spec; o is the observer of the running callback.
func (g *ObservableGauge) Observe(o metric.Observer, v float64, values ...string) {
	opt := metric.WithAttributeSet(g.b.attrs(values))
	if g.f64 != nil {
		o.ObserveFloat64(g.f64, v, opt)
		return
	}
	o.ObserveInt64(g.i64, int64(v), opt)
}

// Register registers cb as the only callback of g; cb observes through g.Observe. The callback
// runs at every collection and must not block (17 rule 4). Errors: those of the meter.
func (g *ObservableGauge) Register(cb func(ctx context.Context, o metric.Observer) error) (metric.Registration, error) {
	return g.meter.RegisterCallback(cb, g.Observable())
}
