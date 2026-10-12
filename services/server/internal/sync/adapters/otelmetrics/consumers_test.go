// SPDX-License-Identifier: Apache-2.0

package otelmetrics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/otelmetrics"
)

// fakeStates is a sync.Consumers whose States answers from a list.
type fakeStates struct {
	states []sync.ConsumerState
	err    error
	ctxOK  bool
}

func (f *fakeStates) Register(sync.ConsumerName, sync.ConsumerHandler) error { return nil }

func (f *fakeStates) States(ctx context.Context) ([]sync.ConsumerState, error) {
	f.ctxOK = ctx != nil
	return f.states, f.err
}

// points collects the gauges of rm by name: consumer name to value.
func points(t *testing.T, rm metricdata.ResourceMetrics) (map[string]map[string]float64, map[string]string) {
	t.Helper()
	got := map[string]map[string]float64{}
	units := map[string]string{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			units[m.Name] = m.Unit
			values := map[string]float64{}
			add := func(set attribute.Set, v float64) {
				if set.Len() != 1 {
					t.Errorf("%s carries %d attributes, want only bothy.consumer.name", m.Name, set.Len())
				}
				name, ok := set.Value("bothy.consumer.name")
				if !ok {
					t.Errorf("%s lacks bothy.consumer.name", m.Name)
				}
				values[name.AsString()] = v
			}
			switch d := m.Data.(type) {
			case metricdata.Gauge[int64]:
				for _, p := range d.DataPoints {
					add(p.Attributes, float64(p.Value))
				}
			case metricdata.Gauge[float64]:
				for _, p := range d.DataPoints {
					add(p.Attributes, p.Value)
				}
			default:
				t.Errorf("%s is %T, want a gauge", m.Name, m.Data)
			}
			got[m.Name] = values
		}
	}
	return got, units
}

func collect(t *testing.T, states *fakeStates, now time.Time) (map[string]map[string]float64, map[string]string) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	if _, err := otelmetrics.RegisterConsumerGauges(mp, states, clock.NewFake(now)); err != nil {
		t.Fatal(err)
	}
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	return points(t, rm)
}

// TestConsumerGaugesValues: lag is head minus cursor floored at zero (a cursor ahead of the head
// after a restore is M7-04's case), the age is now minus the oldest pending event floored at
// zero, and both are zero at the head.
func TestConsumerGaugesValues(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	states := &fakeStates{states: []sync.ConsumerState{
		{Name: sync.ConsumerPush, LastSeq: 10, HeadSeq: 10},
		{Name: sync.ConsumerWebhooks, LastSeq: 4, HeadSeq: 10, OldestPendingAt: now.Add(-90 * time.Second)},
	}}
	got, units := collect(t, states, now)
	lag, age := got["bothy.event.consumer.lag"], got["bothy.event.consumer.pending_age"]
	if lag["push"] != 0 || lag["webhooks"] != 6 || len(lag) != 2 {
		t.Errorf("lag = %v", lag)
	}
	if age["push"] != 0 || age["webhooks"] != 90 || len(age) != 2 {
		t.Errorf("pending_age = %v", age)
	}
	if units["bothy.event.consumer.lag"] != "{event}" || units["bothy.event.consumer.pending_age"] != "s" {
		t.Errorf("units = %v", units)
	}
	if !states.ctxOK {
		t.Error("States got no context")
	}

	floored := &fakeStates{states: []sync.ConsumerState{
		{Name: sync.ConsumerPush, LastSeq: 12, HeadSeq: 10},
		{Name: sync.ConsumerWebhooks, LastSeq: 4, HeadSeq: 10, OldestPendingAt: now.Add(time.Minute)},
	}}
	got, _ = collect(t, floored, now)
	if got["bothy.event.consumer.lag"]["push"] != 0 || got["bothy.event.consumer.pending_age"]["webhooks"] != 0 {
		t.Errorf("floored values = %v", got)
	}
}

// TestConsumerGaugesDropOnError: a failing read drops the data points and does not fail the
// collection (17 rule 4: telemetry never blocks).
func TestConsumerGaugesDropOnError(t *testing.T) {
	t.Parallel()
	got, _ := collect(t, &fakeStates{err: errors.New("database down")}, time.Now())
	for name, values := range got {
		if len(values) != 0 {
			t.Errorf("%s has points %v after a failed read", name, values)
		}
	}
}

// TestConsumerGaugesOnlyKnownNames: the attribute is a closed set; a state with a name outside
// it is not exported.
func TestConsumerGaugesOnlyKnownNames(t *testing.T) {
	t.Parallel()
	got, _ := collect(t, &fakeStates{states: []sync.ConsumerState{{Name: "agents", LastSeq: 1, HeadSeq: 5}}}, time.Now())
	for name, values := range got {
		if len(values) != 0 {
			t.Errorf("%s exports an unknown consumer: %v", name, values)
		}
	}
}
