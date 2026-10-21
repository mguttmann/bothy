// SPDX-License-Identifier: Apache-2.0

package eventlog

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ActorRef is the actor of an ephemeral event (envelope member actor).
type ActorRef struct {
	// Kind is user, agent, app or system.
	Kind ActorKind
	// PrincipalID is the principal of a user, agent or app; zero for system.
	PrincipalID ids.ID
}

// Scope is the location of an ephemeral event (envelope member scope); zero ids are absent.
type Scope struct {
	SpaceID, ChannelID, ThreadID ids.ID
}

// ResourceRef is the affected object of an ephemeral event (envelope member resource).
type ResourceRef struct {
	// Kind is the resource kind, for example "thread"; required.
	Kind string
	// ID is the object; zero when there is none.
	ID ids.ID
	// Version is the version of the object; zero when the event reports none.
	Version int64
}

// EphemeralEvent is a short-lived event (docs/spec/04-sync-protocol.md "Two kinds"): no seq, no
// permission_version, never stored, delivered at most once and lost during a pause.
//
// Example:
//
//	ev := eventlog.EphemeralEvent{
//		Type: "typing.started", SchemaVersion: 1,
//		Actor:    &eventlog.ActorRef{Kind: eventlog.ActorUser, PrincipalID: alexID},
//		Scope:    eventlog.Scope{SpaceID: spaceID, ChannelID: channelID, ThreadID: threadID},
//		Resource: eventlog.ResourceRef{Kind: "thread", ID: threadID},
//		Payload:  payload,
//	}
type EphemeralEvent struct {
	// Type is the ephemeral type of docs/spec/actions.yaml, for example "typing.started".
	Type string
	// SchemaVersion is the version of the payload schema of Type, at least 1.
	SchemaVersion int
	// Actor is the actor; nil for a system event.
	Actor *ActorRef
	// Scope is the space, channel and thread of the event.
	Scope Scope
	// Resource is the affected object.
	Resource ResourceRef
	// Audience is set only for personal ephemeral types: only this principal receives the event.
	Audience ids.ID
	// OccurredAt is the time of the event; zero means the time of PublishEphemeral. A producer
	// whose payload derives a time from it (expires_at of typing.started) sets it.
	OccurredAt time.Time
	// Payload is encoded as a JSON object with encoding/json; it must contain no JSON null
	// (ADR 0053): use omitzero or omitempty for optional members.
	Payload any
}

// EphemeralPublisher hands an ephemeral event to the gateway of this process, which relays it to
// the connections that may receive it (02 "Ephemeral events"). It never waits on a connection.
// With cluster.mode = multi a decorator adds the forwarding to the other nodes (M7-17) without
// changing callers.
type EphemeralPublisher interface {
	// PublishEphemeral relays ev. Errors only report programming faults of the producer: an
	// unregistered type, a schema version below 1, a payload that is no JSON object or that
	// reaches sync.event_payload_max_bytes. Nothing is sent then.
	//
	// Example:
	//
	//	if err := publisher.PublishEphemeral(ctx, ev); err != nil {
	//		return err
	//	}
	PublishEphemeral(ctx context.Context, ev EphemeralEvent) error
}
