// SPDX-License-Identifier: Apache-2.0

package sync

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Event is one stored row of event_log as the log tail and catch-up read it
// (docs/spec/03-data-model.md table event_log). Zero values stand for NULL; Redacted is
// redacted_at IS NOT NULL.
type Event struct {
	// Seq is the global, commit-ordered sequence number.
	Seq int64
	// ID is the event ID.
	ID ids.ID
	// Type is the event type.
	Type string
	// SchemaVersion is the version of the payload schema.
	SchemaVersion int
	// OccurredAt is the time of the transaction.
	OccurredAt time.Time
	// ActorKind is user, agent, app, system or empty (no actor).
	ActorKind string
	// ActorPrincipalID is the actor principal of user, agent and app.
	ActorPrincipalID ids.ID
	// SpaceID, ChannelID and ThreadID are the scope.
	SpaceID, ChannelID, ThreadID ids.ID
	// ResourceKind is the kind of the affected object.
	ResourceKind string
	// ResourceID is the ID of the affected object.
	ResourceID ids.ID
	// ResourceVersion is the version after the change; 0 for none.
	ResourceVersion int64
	// AudiencePrincipalID is the only receiver of a personal event.
	AudiencePrincipalID ids.ID
	// ReceiveAction is the action a receiver needs.
	ReceiveAction string
	// Payload is the JSON payload.
	Payload json.RawMessage
	// Truncated reports a reduced payload.
	Truncated bool
	// Redacted reports a payload redacted after a deletion (R-DM-18).
	Redacted bool
	// CorrelationID is the W3C trace ID of the triggering request, written as a UUID.
	CorrelationID ids.ID
	// TraceContext is the W3C traceparent of a recorded request.
	TraceContext string
	// ClientMsgID echoes the client ID of the sender.
	ClientMsgID ids.ID
	// PermissionVersion is the rights version at the commit.
	PermissionVersion int64
}

// DeliveryFields are the members that exist only per delivery and are never stored
// (R-SY-17, ADR 0052): the gateway, the REST catch-up and the webhooks set them.
type DeliveryFields struct {
	// IncludeTraceContext adds trace_context; only deliveries to agent adapters set it
	// (04 "Envelope").
	IncludeTraceContext bool
	// InvocationToken is a freshly issued invocation token (R-SY-07); empty for none.
	InvocationToken string
	// Redelivered marks a redelivery to an agent consumer after agent.ack_timeout.
	Redelivered bool
	// SyncEpoch is the time line of the log, set only in webhook deliveries (ADR 0059); zero
	// for none.
	SyncEpoch ids.ID
}

// Sink receives the events the log tail read, in seq order and from one goroutine. A sink must
// not keep batch after it returns. An error ends the tail and with it the process (02 "Panics":
// no process without fan-out).
type Sink func(ctx context.Context, batch []Event) error

// Tail is the one log reader of an api process (02 "Log tail per api process").
type Tail interface {
	// Run starts at the current head, reads every later event once and in seq order and hands
	// it to the sinks, until ctx ends (then it returns nil). A lost LISTEN connection is
	// reconnected while polling continues; a database that cannot be read is retried.
	//
	// Errors: the error of a sink, ErrGap for a hole in the sequence, and the error of reading
	// the head at the start.
	Run(ctx context.Context) error
	// Head returns the highest seq the sinks received, or the head Run started at.
	Head() int64
}

// ErrGap reports a hole in the sequence the tail read. The counter pattern rules it out (04
// "Pattern with a counter row" item 3), so it is a bug, and the process must not deliver further.
var ErrGap = errors.New("sync: gap in the event sequence")
