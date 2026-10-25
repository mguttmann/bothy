// SPDX-License-Identifier: Apache-2.0

package stats

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Register declares what the stats module emits and accepts (M1-25, docs/spec/05-actions.md
// sections 6 and 7): the ephemeral event stats.live and the gateway ops stats.subscribe and
// stats.unsubscribe, whose handlers the gateway runs on the port Live (M6-15). internal/wire lists
// it in moduleRegistrations.
func Register(r *authorize.Registry) {
	r.Event("stats", authorize.EventStatsLive)
	r.Op("stats", authorize.OpStatsSubscribe)
	r.Op("stats", authorize.OpStatsUnsubscribe)
}

// LiveQuery is the target of a live subscription or of getStatsLive: SpaceID zero is the whole
// instance (owner), otherwise one space (manage), docs/spec/25-visualization.md 5.1 item 1.
type LiveQuery struct {
	SpaceID ids.ID
}

// Instance reports the instance scope.
func (q LiveQuery) Instance() bool { return q.SpaceID.IsZero() }

// LiveFrame is one live frame filtered for one receiver (schema StatsLiveFrame, 25 5.2): the
// build time of its base frame and the encoded JSON object, below sync.event_payload_max_bytes.
// The same bytes are the payload of the ephemeral event stats.live and the body of getStatsLive.
type LiveFrame struct {
	// At is the build time of the base frame (member at).
	At time.Time
	// Payload is the JSON object StatsLiveFrame, never containing null.
	Payload json.RawMessage
}

// LiveRound is one built base frame of the node (25 5.1 item 4). It is never sent unfiltered:
// For filters it for one receiver in memory.
type LiveRound interface {
	// At returns the build time.
	At() time.Time
	// For filters the base frame for a with the subscription q (25 4, 5.2): the visible channels
	// of a (Scope of channel.read; for a space only that space's channels), cost members only with
	// cost.read.aggregate on the target, sli and connections only for the instance, the runs cut
	// to stats.live_max_runs and to sync.event_payload_max_bytes. ctx decides the mode of
	// authorize: the stream passes authorize.Delivery. Errors: the error of Scope or authorize, or
	// internal_error when even a frame without runs reaches the size bound.
	For(ctx context.Context, a actor.Actor, q LiveQuery) (LiveFrame, error)
}

// LiveSink receives every base frame the node builds, in build order, from the builder's
// goroutine. The gateway implements it and delivers one filtered frame per subscribed
// connection (M6-15 design note 5). It must not block for long; it never builds.
type LiveSink interface {
	LiveBuilt(ctx context.Context, r LiveRound)
}

// LiveConnections are the gateway connections of this node by kind (schema StatsLiveConnections).
type LiveConnections struct {
	User, Agent, App int
}

// LiveConnectionCounter is the source port of the member connections of the live frame: the
// gateway hub of the node implements it (25 5.2 row connections, only with cluster.mode = single).
type LiveConnectionCounter interface {
	// LiveConnections counts the connections of this node by the kind of their principal.
	LiveConnections() LiveConnections
}

// Live is the incoming port of the live stream stats.live and of getStatsLive (M6-15, 25 5).
// One instance per api node builds at most one base frame per stats.live_interval and only while
// a subscription exists or getStatsLive was polled within the last interval. It is safe for
// concurrent use.
type Live interface {
	// Check is the subscription check of 25 5.1 items 1 and 6 for a and q: stats.live_enabled off
	// is feature_disabled; then stats.read on the instance (zero SpaceID) or the space, whose
	// denial is the decision's error (not_found for an unreadable space, forbidden otherwise);
	// agents and apps are forbidden (04 12 item 4). It counts nothing.
	//
	// Example:
	//
	//	if err := live.Check(ctx, a, stats.LiveQuery{SpaceID: support}); err != nil { ... }
	Check(ctx context.Context, a actor.Actor, q LiveQuery) error
	// Acquire takes one subscription slot of the node and raises bothy.stats.live.subscribers;
	// the first slot starts the builder. Errors: rate_limited above stats.live_max_subscribers.
	Acquire() error
	// Release returns one slot taken by Acquire and lowers the gauge.
	Release()
	// Current returns the most recently built base frame filtered for the caller of ctx (getStatsLive,
	// 25 5.1 last paragraph, R-VZ-06): Check first, then the base frame of the current
	// stats.live_interval, built once in a single flight shared by all callers when the node holds
	// none that young. A call keeps the builder running for one interval. Errors: those of Check,
	// of the build and of LiveRound.For.
	Current(ctx context.Context, q LiveQuery) (LiveFrame, error)
	// BindSink makes s the receiver of every base frame from now on; internal/wire binds the
	// gateway once before serving.
	BindSink(s LiveSink)
}
