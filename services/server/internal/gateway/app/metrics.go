// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"
)

// Metrics is the outgoing port of the gateway metrics (02 "Layers within a module" rule 5): the
// gateway, delivery and fan-out instruments of signals.md 4.1 and 4.2. adapters/otelmetrics
// implements it with the declarations of the metric registry; the application layer knows no
// metrics library. Implementations are safe for concurrent use and never block.
type Metrics interface {
	// ConnectionOpened adds one connection of kind (user, agent, app) that reached ready or
	// resumed (bothy.gateway.connections.active +1).
	ConnectionOpened(ctx context.Context, kind string)
	// ConnectionClosed removes such a connection when it closes (-1).
	ConnectionClosed(ctx context.Context, kind string)
	// Connected records the time d from the upgrade to ready or resumed (outcome ok), or to the
	// close before it (client_error or server_error), with the kind of the actor or anonymous
	// before identification (bothy.gateway.connect.duration).
	Connected(ctx context.Context, d time.Duration, outcome, kind string)
	// CatchupDone records the time d from ready or resumed to catchup.complete (complete) or to
	// resync_required (bothy.gateway.catchup.duration).
	CatchupDone(ctx context.Context, d time.Duration, outcome string)
	// Resync counts one resync_required with its reason (bothy.gateway.resync).
	Resync(ctx context.Context, reason string)
	// Closed counts one close with its close code, sent by the server or the client
	// (bothy.gateway.close).
	Closed(ctx context.Context, code int)
	// Dropped counts n outbound frames discarded for reason (bothy.gateway.outbound.dropped).
	Dropped(ctx context.Context, reason string, n int)
	// Delivered records the time d from occurred_at of an event to the write of its frame on a
	// live connection, with the durability (durable, ephemeral) and the receiver's kind
	// (bothy.delivery.duration).
	Delivered(ctx context.Context, d time.Duration, durability, kind string)
}

// The values the gateway records (signals.md 4.1 and 4.2).
const (
	// OutcomeComplete and OutcomeResyncRequired are bothy.outcome of the catch-up duration.
	OutcomeComplete       = "complete"
	OutcomeResyncRequired = "resync_required"
	// ReasonSlowConsumer is bothy.reason of frames discarded at a 4014 close.
	ReasonSlowConsumer = "slow_consumer"
	// KindAnonymous is bothy.principal.kind before identification.
	KindAnonymous = "anonymous"
	// DurabilityDurable and DurabilityEphemeral are bothy.event.durability.
	DurabilityDurable   = "durable"
	DurabilityEphemeral = "ephemeral"
)

// NopMetrics is the Metrics of a gateway, hub or adapter built without one: it records nothing.
type NopMetrics struct{}

// ConnectionOpened implements Metrics.
func (NopMetrics) ConnectionOpened(context.Context, string) {}

// ConnectionClosed implements Metrics.
func (NopMetrics) ConnectionClosed(context.Context, string) {}

// Connected implements Metrics.
func (NopMetrics) Connected(context.Context, time.Duration, string, string) {}

// CatchupDone implements Metrics.
func (NopMetrics) CatchupDone(context.Context, time.Duration, string) {}

// Resync implements Metrics.
func (NopMetrics) Resync(context.Context, string) {}

// Closed implements Metrics.
func (NopMetrics) Closed(context.Context, int) {}

// Dropped implements Metrics.
func (NopMetrics) Dropped(context.Context, string, int) {}

// Delivered implements Metrics.
func (NopMetrics) Delivered(context.Context, time.Duration, string, string) {}

// orNop returns m, or NopMetrics when m is nil.
func orNop(m Metrics) Metrics {
	if m == nil {
		return NopMetrics{}
	}
	return m
}

// knownBatch is a batch of the log tail the hub has not fully distributed: its last seq and the
// time it became known.
type knownBatch struct {
	last int64
	at   time.Time
}

// Known tells the hub that the log tail knows the events up to last (read after a NOTIFY or a
// poll), before it hands them to Dispatch. bothy.event.fanout.lag measures from this moment.
// internal/wire calls it in the sink of the tail.
func (h *Hub) Known(last int64) {
	now := h.d.Clock.Now()
	h.lagMu.Lock()
	defer h.lagMu.Unlock()
	if n := len(h.known); last <= h.distributed || n > 0 && last <= h.known[n-1].last {
		return
	}
	h.known = append(h.known, knownBatch{last: last, at: now})
}

// distributedUpTo records that Dispatch distributed every event up to seq.
func (h *Hub) distributedUpTo(seq int64) {
	h.lagMu.Lock()
	defer h.lagMu.Unlock()
	h.distributed = max(h.distributed, seq)
	i := 0
	for i < len(h.known) && h.known[i].last <= h.distributed {
		i++
	}
	h.known = h.known[i:]
}

// FanoutLag returns bothy.event.fanout.lag (signals.md 4.2): 0 when the hub has distributed every
// event the tail knows of, otherwise now minus the time the oldest undistributed event became
// known. It reads only the hub's own counters, so a metric collection never waits.
func (h *Hub) FanoutLag() time.Duration {
	now := h.d.Clock.Now()
	h.lagMu.Lock()
	defer h.lagMu.Unlock()
	if len(h.known) == 0 {
		return 0
	}
	return max(now.Sub(h.known[0].at), 0)
}
