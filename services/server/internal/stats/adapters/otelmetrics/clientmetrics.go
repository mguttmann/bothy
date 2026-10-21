// SPDX-License-Identifier: Apache-2.0

// Package otelmetrics is the telemetry adapter of the stats module: it implements app.Recorder
// over the declarations of the metric registry (docs/observability/signals.md 4.6a). Counters and
// the gauge are SDK instruments; the app histograms go through an metrics.AggregatedSet, whose
// cumulative data points carry the clients' bucket counts to the exporter unchanged
// (docs/DECISIONS.md D-M4-31-1).
//
// Example:
//
//	r, err := otelmetrics.New(nil, metrics.DefaultAggregated(), closeCodes)
package otelmetrics

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/stats/app"
	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// ClientMetrics are the instruments of the app metrics.
type ClientMetrics struct {
	counters  map[string]*metrics.Counter
	gauges    map[string]*metrics.Gauge
	hists     map[string]*metrics.AggregatedHistogram
	discarded *metrics.Counter
}

// Compile-time check: ClientMetrics is the outgoing port of app.
var _ app.Recorder = (*ClientMetrics)(nil)

// New creates the instruments of metrics.Client on mp (nil means the global provider of
// otel.Setup) and the histograms in set. closeCodes are the close codes of 04 6.4, the closed set
// of bothy.close.code. Errors: no set, or an error of the registry or the meter provider.
func New(mp metric.MeterProvider, set *metrics.AggregatedSet, closeCodes []int) (*ClientMetrics, error) {
	if set == nil {
		return nil, errors.New("otelmetrics: client metrics need the aggregated set")
	}
	codes := make([]string, len(closeCodes))
	for i, c := range closeCodes {
		codes[i] = strconv.Itoa(c)
	}
	v := metrics.Values{metrics.SourceCloseCodes: codes}
	c := &ClientMetrics{counters: map[string]*metrics.Counter{}, gauges: map[string]*metrics.Gauge{}, hists: map[string]*metrics.AggregatedHistogram{}}
	var err error
	for _, s := range metrics.Client() {
		name := strings.TrimPrefix(s.Name, "bothy.")
		switch s.Kind {
		case metrics.KindCounter:
			c.counters[name], err = metrics.NewCounter(mp, s, v)
		case metrics.KindGauge:
			c.gauges[name], err = metrics.NewGauge(mp, s, v)
		case metrics.KindHistogram:
			c.hists[name], err = set.Histogram(s, v)
		case metrics.KindUpDownCounter:
			// No app metric of signals.md 4.6a is an up-down counter; one would be a catalogue error.
			err = fmt.Errorf("%s is a %s", s.Name, s.Kind)
		default:
			err = fmt.Errorf("%s is a %s", s.Name, s.Kind)
		}
		if err != nil {
			return nil, fmt.Errorf("otelmetrics: %w", err)
		}
	}
	if c.discarded, err = metrics.NewCounter(mp, metrics.ClientMetricsDiscarded, nil); err != nil {
		return nil, fmt.Errorf("otelmetrics: %w", err)
	}
	return c, nil
}

// Record implements app.Recorder: the attribute values in the order of the registry (platform,
// density, theme, app major, then the close code of client.reconnects). A name outside the
// registry, which domain.Filter never passes, is ignored.
func (c *ClientMetrics) Record(ctx context.Context, m domain.Measurement) {
	values := []string{m.Attrs.Platform, m.Attrs.Density, m.Attrs.Theme, m.Attrs.AppMajor}
	if m.CloseCode != "" {
		values = append(values, m.CloseCode)
	}
	switch m.Kind {
	case domain.KindCounter:
		if ctr, ok := c.counters[m.Name]; ok {
			ctr.AddFloat(ctx, m.Value, values...)
		}
	case domain.KindGauge:
		if g, ok := c.gauges[m.Name]; ok {
			g.Record(ctx, m.Value, values...)
		}
	case domain.KindHistogram:
		h, ok := c.hists[m.Name]
		if !ok {
			return
		}
		buckets := make([]metrics.Bucket, len(m.Buckets))
		for i, b := range m.Buckets {
			buckets[i] = metrics.Bucket{LE: b.LE, Count: uint64(max(b.Count, 0))} // #nosec G115 -- non-negative
		}
		// domain.Filter checked the bounds against the same row; an error here would be a
		// programming error, and recording never fails the request.
		_ = h.Add(buckets, values...)
	}
}

// Discarded implements app.Recorder; n of zero or less records nothing.
func (c *ClientMetrics) Discarded(ctx context.Context, reason domain.Reason, n int) {
	if n > 0 {
		c.discarded.Add(ctx, int64(n), string(reason))
	}
}
