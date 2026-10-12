// SPDX-License-Identifier: Apache-2.0

package sync

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Envelope is the one envelope of an event on the gateway, in the REST catch-up and in webhooks
// (ADR 0052, docs/spec/04-sync-protocol.md "Envelope"; OpenAPI EventEnvelope plus
// type and payload). It has no JSON null (ADR 0053, R-SY-16): every optional member carries
// omitzero, so an absent value is an absent member. Seq and PermissionVersion are pointers,
// so a durable event with permission_version 0 still carries the member.
type Envelope struct {
	// ID is the event ID.
	ID ids.ID `json:"id"`
	// Seq is the global sequence; nil for ephemeral events.
	Seq *int64 `json:"seq,omitzero"`
	// Type is the event type.
	Type string `json:"type"`
	// SchemaVersion is the version of the payload schema.
	SchemaVersion int `json:"schema_version"`
	// OccurredAt is the time of the transaction in RFC 3339, UTC.
	OccurredAt string `json:"occurred_at"`
	// Actor is the actor; nil for an event without actor.
	Actor *EnvelopeActor `json:"actor,omitzero"`
	// Scope is the location; always an object, its members absent where not applicable.
	Scope EnvelopeScope `json:"scope"`
	// Resource is the affected object.
	Resource EnvelopeResource `json:"resource"`
	// Audience is "self" for a personal event.
	Audience string `json:"audience,omitzero"`
	// Payload is the payload object.
	Payload json.RawMessage `json:"payload"`
	// Redacted is true when the payload was redacted after a deletion.
	Redacted bool `json:"redacted,omitzero"`
	// Truncated is true when the producer reduced the payload.
	Truncated bool `json:"truncated,omitzero"`
	// CorrelationID is the trace ID of the triggering request.
	CorrelationID ids.ID `json:"correlation_id,omitzero"`
	// TraceContext is the traceparent of a recorded request, only in deliveries to agents.
	TraceContext string `json:"trace_context,omitzero"`
	// ClientMsgID echoes the client ID of the sender.
	ClientMsgID ids.ID `json:"client_msg_id,omitzero"`
	// PermissionVersion is the rights version at the commit; nil for ephemeral events.
	PermissionVersion *int64 `json:"permission_version,omitzero"`
	// InvocationToken is the delivery member with a fresh invocation token.
	InvocationToken string `json:"invocation_token,omitzero"`
	// Redelivered is the delivery member of a redelivery.
	Redelivered bool `json:"redelivered,omitzero"`
	// SyncEpoch is the delivery member of webhook deliveries.
	SyncEpoch ids.ID `json:"sync_epoch,omitzero"`
}

// EnvelopeActor is the member actor (OpenAPI EventActor).
type EnvelopeActor struct {
	// ID is the actor principal; absent for system.
	ID ids.ID `json:"id,omitzero"`
	// Kind is user, agent, app or system.
	Kind string `json:"kind"`
}

// EnvelopeScope is the member scope (OpenAPI EventScope).
type EnvelopeScope struct {
	// SpaceID is the space.
	SpaceID ids.ID `json:"space_id,omitzero"`
	// ChannelID is the channel.
	ChannelID ids.ID `json:"channel_id,omitzero"`
	// ThreadID is the thread.
	ThreadID ids.ID `json:"thread_id,omitzero"`
}

// EnvelopeResource is the member resource (OpenAPI EventResource).
type EnvelopeResource struct {
	// Kind is the kind of the object.
	Kind string `json:"kind"`
	// ID is the object.
	ID ids.ID `json:"id,omitzero"`
	// Version is the version after the change; absent when the event reports none.
	Version int64 `json:"version,omitzero"`
}

// audienceSelf is the only value of the member audience.
const audienceSelf = "self"

// emptyObject is the payload of an event whose payload is missing, so payload is never null.
var emptyObject = json.RawMessage(`{}`)

// Envelope maps the stored event to its envelope for one delivery with d (ADR 0052): seq and
// permission_version always; actor iff an actor kind is set, actor.id iff a principal is set;
// scope always as an object; resource.version only when set; audience "self" iff an audience
// principal is set; redacted and truncated only when true; trace_context only with
// d.IncludeTraceContext; invocation_token, redelivered and sync_epoch only from d; occurred_at in
// RFC 3339, UTC. The mapping lives in the port because the port may import only kernel packages
// (docs/DECISIONS.md M1-16/4).
//
// Example:
//
//	env := ev.Envelope(sync.DeliveryFields{})
//	b, err := json.Marshal(env)
func (e Event) Envelope(d DeliveryFields) Envelope {
	seq, pv := e.Seq, e.PermissionVersion
	env := Envelope{
		ID: e.ID, Seq: &seq, Type: e.Type, SchemaVersion: e.SchemaVersion,
		OccurredAt: e.OccurredAt.UTC().Format(time.RFC3339Nano),
		Scope:      EnvelopeScope{SpaceID: e.SpaceID, ChannelID: e.ChannelID, ThreadID: e.ThreadID},
		Resource:   EnvelopeResource{Kind: e.ResourceKind, ID: e.ResourceID, Version: e.ResourceVersion},
		Payload:    e.Payload, Redacted: e.Redacted, Truncated: e.Truncated,
		CorrelationID: e.CorrelationID, ClientMsgID: e.ClientMsgID, PermissionVersion: &pv,
		InvocationToken: d.InvocationToken, Redelivered: d.Redelivered, SyncEpoch: d.SyncEpoch,
	}
	if p := bytes.TrimSpace(env.Payload); len(p) == 0 || bytes.Equal(p, []byte("null")) {
		env.Payload = emptyObject
	}
	if e.ActorKind != "" {
		env.Actor = &EnvelopeActor{ID: e.ActorPrincipalID, Kind: e.ActorKind}
	}
	if !e.AudiencePrincipalID.IsZero() {
		env.Audience = audienceSelf
	}
	if d.IncludeTraceContext {
		env.TraceContext = e.TraceContext
	}
	return env
}
