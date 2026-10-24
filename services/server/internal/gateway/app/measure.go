// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
)

// measured is the Metrics of a gateway that also records the own measurement: every call goes to
// the telemetry first and then, with the same values, to the recorder
// (docs/spec/25-visualization.md 3.2). The two cannot diverge in their notion of an event, because
// both see the one call of the measuring point; neither depends on the other.
type measured struct {
	Metrics
	rec measure.Recorder
}

// Measured returns m extended by the own measurement of the gateway through rec (M6-13):
// sli.reconnect on Connected (all connections without client_error; good is ok within the
// threshold), catchups on a complete catch-up with its duration, resyncs per reason, slow
// consumer closes on the close code 4014 and sli.delivery_latency on every live frame of a
// durable event. internal/wire passes the result to the gateway, the hub and the WebSocket
// adapter. A nil m means NopMetrics, a nil rec returns m unchanged.
func Measured(m Metrics, rec measure.Recorder) Metrics {
	m = orNop(m)
	if rec == nil {
		return m
	}
	return measured{Metrics: m, rec: rec}
}

// Connected implements Metrics.
func (m measured) Connected(ctx context.Context, d time.Duration, outcome, kind string) {
	m.Metrics.Connected(ctx, d, outcome, kind)
	if outcome == domain.OutcomeClientError {
		return
	}
	m.rec.Record(measure.SLIReconnect, measure.Key{}, measure.SLI(d, outcome == domain.OutcomeOK))
}

// CatchupDone implements Metrics.
func (m measured) CatchupDone(ctx context.Context, d time.Duration, outcome string) {
	m.Metrics.CatchupDone(ctx, d, outcome)
	if outcome == OutcomeComplete {
		m.rec.Record(measure.Catchups, measure.Key{}, measure.Timed(d))
	}
}

// Resync implements Metrics.
func (m measured) Resync(ctx context.Context, reason string) {
	m.Metrics.Resync(ctx, reason)
	m.rec.Record(measure.Resyncs, measure.Key{Dimension: reason}, measure.One())
}

// Closed implements Metrics.
func (m measured) Closed(ctx context.Context, code int) {
	m.Metrics.Closed(ctx, code)
	if code == int(domain.CloseSlowConsumer) {
		m.rec.Record(measure.SlowConsumerCloses, measure.Key{}, measure.One())
	}
}

// Delivered implements Metrics.
func (m measured) Delivered(ctx context.Context, d time.Duration, durability, kind string) {
	m.Metrics.Delivered(ctx, d, durability, kind)
	if durability == DurabilityDurable {
		m.rec.Record(measure.SLIDeliveryLatency, measure.Key{}, measure.SLI(d, true))
	}
}
