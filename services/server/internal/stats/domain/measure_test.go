// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// histogramOf names, per sli.* metric with a threshold, the OpenTelemetry histogram whose bucket
// boundary the threshold is (slos.md 2, signals.md 4.7).
var histogramOf = map[string]metrics.Spec{
	"sli.delivery_latency": metrics.DeliveryDuration,
	"sli.reconnect":        metrics.GatewayConnectDuration,
	"sli.run_start":        metrics.RunStartLatency,
	"sli.turn_duration":    metrics.RunDuration,
}

// TestIsGoodAtThreshold is VZ-T02: for every threshold of slibounds.gen.go a duration exactly at
// it and just below it is good and one just above it is not, and the histogram of the same
// instrument, read through the manual reader of M1-68, puts each value into a bucket whose upper
// bound is at most the threshold exactly when IsGood says good.
func TestIsGoodAtThreshold(t *testing.T) {
	t.Parallel()
	withThreshold := 0
	for metric, b := range domain.SLIBounds {
		if b.Threshold == 0 {
			if !domain.IsGood(metric, time.Hour) {
				t.Errorf("%s has no threshold, so every duration is good", metric)
			}
			continue
		}
		withThreshold++
		spec, ok := histogramOf[metric]
		if !ok {
			t.Fatalf("%s has a threshold but no histogram in this test", metric)
		}
		for _, c := range []struct {
			d    time.Duration
			good bool
		}{{b.Threshold - time.Millisecond, true}, {b.Threshold, true}, {b.Threshold + time.Millisecond, false}} {
			if got := domain.IsGood(metric, c.d); got != c.good {
				t.Errorf("IsGood(%s, %v) = %v, want %v", metric, c.d, got, c.good)
			}
			if got := inGoodBucket(t, spec, c.d, b.Threshold); got != c.good {
				t.Errorf("%s: the histogram counts %v as good = %v, IsGood %v", spec.Name, c.d, got, c.good)
			}
		}
	}
	if withThreshold != len(histogramOf) {
		t.Fatalf("%d SLIs with a threshold, %d histograms", withThreshold, len(histogramOf))
	}
}

// inGoodBucket records d into a fresh histogram of spec and reports whether the bucket it landed in
// has an upper bound of at most threshold, the good events of the SLO's error query.
func inGoodBucket(t *testing.T, spec metrics.Spec, d, threshold time.Duration) bool {
	t.Helper()
	mp, reader := metricstest.Provider(t)
	h, err := metrics.NewHistogram(mp, spec, metrics.Values{})
	if err != nil {
		t.Fatal(err)
	}
	h.RecordDuration(context.Background(), d)
	hist, ok := metricstest.Find(t, reader, spec.Name).Data.(metricdata.Histogram[float64])
	if !ok || len(hist.DataPoints) != 1 {
		t.Fatalf("%s: no single histogram data point", spec.Name)
	}
	p := hist.DataPoints[0]
	for i, n := range p.BucketCounts {
		if n == 0 {
			continue
		}
		return i < len(p.Bounds) && p.Bounds[i] <= threshold.Seconds()
	}
	t.Fatalf("%s: %v landed in no bucket", spec.Name, d)
	return false
}

// TestEvent shows the counters of one event: whole milliseconds into sum, a negative duration as
// 0, good only for an sli.* metric with a good outcome within the threshold, never above count.
func TestEvent(t *testing.T) {
	t.Parallel()
	thr := domain.Threshold("sli.delivery_latency")
	cases := []struct {
		name    string
		metric  string
		count   int64
		sum     int64
		elapsed time.Duration
		ok      bool
		want    domain.Counters
	}{
		{"plain event", "messages_created", 1, 0, 0, true, domain.Counters{Count: 1}},
		{"duration in whole ms", "runs_finished", 1, 0, 1500*time.Millisecond + 999*time.Microsecond, true, domain.Counters{Count: 1, Sum: 1500}},
		{"cost micros", "cost", 1, 2500000, 0, false, domain.Counters{Count: 1, Sum: 2500000}},
		{"ok outside an sli", "catchups", 1, 0, time.Second, true, domain.Counters{Count: 1, Sum: 1000}},
		{"negative duration", "sli.run_start", 1, 0, -time.Second, true, domain.Counters{Count: 1, Good: 1}},
		{"good at the bound", "sli.delivery_latency", 1, 0, thr, true, domain.Counters{Count: 1, Sum: thr.Milliseconds(), Good: 1}},
		{"bad above the bound", "sli.delivery_latency", 1, 0, thr + time.Nanosecond, true, domain.Counters{Count: 1, Sum: thr.Milliseconds()}},
		{"bad outcome", "sli.api_availability", 1, 0, 0, false, domain.Counters{Count: 1}},
		{"good ratio", "sli.push_acceptance", 3, 0, 0, true, domain.Counters{Count: 3, Good: 3}},
	}
	for _, c := range cases {
		got := domain.Event(c.metric, c.count, c.sum, c.elapsed, c.ok)
		if got != c.want {
			t.Errorf("%s: Event = %+v, want %+v", c.name, got, c.want)
		}
		if got.Good > got.Count {
			t.Errorf("%s: good %d above count %d", c.name, got.Good, got.Count)
		}
	}
	if got := (domain.Counters{Count: 1, Sum: 2, Good: 1}).Add(domain.Counters{Count: 2, Sum: 3}); got != (domain.Counters{Count: 3, Sum: 5, Good: 1}) {
		t.Errorf("Add = %+v", got)
	}
}

// TestMinuteDayAndPeriods shows the minute and day truncation in UTC, the longest SLO period and
// the retention cutoff.
func TestMinuteDayAndPeriods(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 3, 4, 23, 59, 59, 999, time.FixedZone("UTC-2", -2*3600))
	if got, want := domain.Minute(at), time.Date(2026, 3, 5, 1, 59, 0, 0, time.UTC); !got.Equal(want) || got.Location() != time.UTC {
		t.Errorf("Minute = %v, want %v", got, want)
	}
	if got, want := domain.Day(at), time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("Day = %v, want %v", got, want)
	}
	var longest time.Duration
	for _, b := range domain.SLIBounds {
		longest = max(longest, b.Period)
	}
	if got := domain.LongestSLOPeriod(); got != longest || got <= 0 {
		t.Errorf("LongestSLOPeriod = %v, want %v", got, longest)
	}
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	if got, want := domain.RetentionCutoff(now, 72*time.Hour), time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("RetentionCutoff = %v, want %v", got, want)
	}
}
