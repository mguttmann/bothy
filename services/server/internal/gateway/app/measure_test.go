// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
)

// countingMetrics counts the calls that reach the telemetry behind the measurement.
type countingMetrics struct {
	app.NopMetrics
	calls int
}

func (c *countingMetrics) Connected(context.Context, time.Duration, string, string) { c.calls++ }
func (c *countingMetrics) CatchupDone(context.Context, time.Duration, string)       { c.calls++ }
func (c *countingMetrics) Resync(context.Context, string)                           { c.calls++ }
func (c *countingMetrics) Closed(context.Context, int)                              { c.calls++ }
func (c *countingMetrics) Delivered(context.Context, time.Duration, string, string) { c.calls++ }

// TestMeasured holds the own measurement of the gateway (25 3.2, M6-13) next to its telemetry:
// sli.reconnect for every connection except client_error (good only ok), catchups only for a
// complete catch-up with its duration, resyncs per reason, slow_consumer_closes only for 4014 and
// sli.delivery_latency only for durable live frames; every call still reaches the telemetry.
func TestMeasured(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tel := &countingMetrics{}
	rec := &measure.Recording{}
	m := app.Measured(tel, rec)
	m.Connected(ctx, time.Second, "ok", "user")
	m.Connected(ctx, 0, "server_error", "anonymous")
	m.Connected(ctx, time.Second, "client_error", "user")
	m.CatchupDone(ctx, 3*time.Second, app.OutcomeComplete)
	m.CatchupDone(ctx, time.Second, app.OutcomeResyncRequired)
	m.Resync(ctx, "too_far_behind")
	m.Closed(ctx, 4014)
	m.Closed(ctx, 1000)
	m.Delivered(ctx, 200*time.Millisecond, app.DurabilityDurable, "user")
	m.Delivered(ctx, 200*time.Millisecond, app.DurabilityEphemeral, "user")
	m.ConnectionOpened(ctx, "user")
	if tel.calls != 10 {
		t.Fatalf("telemetry saw %d calls, want 10", tel.calls)
	}
	if len(rec.Invalid()) != 0 {
		t.Fatalf("invalid calls %v", rec.Invalid())
	}
	want := []measure.Call{
		{Metric: measure.SLIReconnect, Value: measure.SLI(time.Second, true)},
		{Metric: measure.SLIReconnect, Value: measure.SLI(0, false)},
		{Metric: measure.Catchups, Value: measure.Timed(3 * time.Second)},
		{Metric: measure.Resyncs, Key: measure.Key{Dimension: "too_far_behind"}, Value: measure.One()},
		{Metric: measure.SlowConsumerCloses, Value: measure.One()},
		{Metric: measure.SLIDeliveryLatency, Value: measure.SLI(200*time.Millisecond, true)},
	}
	got := rec.Calls("")
	if len(got) != len(want) {
		t.Fatalf("calls %+v\nwant %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("call %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if app.Measured(tel, nil) != app.Metrics(tel) {
		t.Error("Measured without a recorder changed the metrics")
	}
}
