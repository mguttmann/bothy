// SPDX-License-Identifier: Apache-2.0

package eventlog

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ActorKind is the kind of the actor of a durable event: event_log.actor_kind and the envelope
// member actor.kind (docs/spec/03-datenmodell.md, R-DM-29; OpenAPI ActorKind). The empty value
// means "no actor" (a job without principal and without system action).
type ActorKind string

// The actor kinds.
const (
	// ActorUser is a human; ActorPrincipalID is set.
	ActorUser ActorKind = "user"
	// ActorAgent is an agent; ActorPrincipalID is set.
	ActorAgent ActorKind = "agent"
	// ActorApp is an app; ActorPrincipalID is set.
	ActorApp ActorKind = "app"
	// ActorSystem is a system action of the server (05 rule 10); ActorPrincipalID stays zero.
	ActorSystem ActorKind = "system"
)

// Event is a durable event as a producer appends it (docs/spec/04-sync-protokoll.md
// "Umschlag (Envelope)"; 03 table event_log). Zero values mean "absent" (NULL in the table, no
// member in the envelope); a zero ID makes the appender generate a UUIDv7. seq, occurred_at,
// permission_version, correlation_id and trace_context are not set by the producer: the sync
// hook and the appender add them.
//
// Example:
//
//	ev := eventlog.Event{
//		Type: "thread.closed", SchemaVersion: 1,
//		ActorKind: eventlog.ActorUser, ActorPrincipalID: actorID,
//		SpaceID: spaceID, ChannelID: channelID, ThreadID: threadID,
//		ResourceKind: "thread", ResourceID: threadID, ResourceVersion: 4,
//		ReceiveAction: "thread.read", Payload: []byte(`{"closed":true}`),
//	}
type Event struct {
	// ID is the event ID (dedupe key of clients); zero means generate one.
	ID ids.ID
	// Type is the event type of 05 "Dauerhafte Ereignistypen", for example "message.created".
	Type string
	// SchemaVersion is the version of the payload schema of Type, at least 1.
	SchemaVersion int
	// ActorKind is the kind of the actor; empty for no actor.
	ActorKind ActorKind
	// ActorPrincipalID is the principal of a user, agent or app actor; zero otherwise.
	ActorPrincipalID ids.ID
	// SpaceID, ChannelID and ThreadID are the scope; zero where not applicable.
	SpaceID, ChannelID, ThreadID ids.ID
	// ResourceKind is the kind of the affected object, for example "message"; required.
	ResourceKind string
	// ResourceID is the ID of the affected object; zero when there is none.
	ResourceID ids.ID
	// ResourceVersion is the version of the object after the change; zero when the event
	// reports no new version of exactly this object (04 "Zustellgarantie" item 3).
	ResourceVersion int64
	// AudiencePrincipalID makes a personal event that only this principal receives.
	AudiencePrincipalID ids.ID
	// ReceiveAction is the action a receiver needs on the resource, "self" for a personal
	// event (05 section 6); required, one value per type.
	ReceiveAction string
	// Payload is a JSON object in UTF-8 below sync.event_payload_max_bytes, without JSON null and
	// without secrets (04 payload rules), in a form jsonb stores as it is: no number with an
	// exponent or with more digits than PostgreSQL numeric holds, no U+0000 and no unpaired
	// surrogate escape. A producer that marshals float64 values must avoid the exponent form
	// encoding/json writes for very small and very large values.
	Payload []byte
	// Truncated reports that the producer built a reduced payload because the object exceeds
	// sync.event_payload_max_bytes (04 payload rule 2).
	Truncated bool
	// ClientMsgID echoes the client ID of the sender; zero when there is none.
	ClientMsgID ids.ID
}

// Appender appends durable events to the event log of the unit of work in ctx. sync implements
// it; every module receives it through its constructor.
type Appender interface {
	// Append validates events and buffers them in the unit of work of ctx, in order; the sync
	// phase hook numbers and writes them before the commit, and a rollback drops them. It may be
	// called in the work and the rights check phase only.
	//
	// Errors: kind internal outside a unit of work, in another phase, or for an event that
	// breaks a rule of 03 event_log (a programming error of the producer); nothing is buffered
	// then.
	//
	// Example:
	//
	//	if err := events.Append(ctx, ev); err != nil {
	//		return err
	//	}
	Append(ctx context.Context, events ...Event) error
}

// RedactFunc returns the redacted payload of one stored durable event of a deleted object from
// its type, schema version and current payload (04 section 2.2 rule 5). The result must stay
// valid against the schema of the type; an error fails the redaction and with it the deletion.
type RedactFunc func(eventType string, schemaVersion int, payload json.RawMessage) (json.RawMessage, error)

// Redactor replaces the payload of stored durable events of one resource (03 section 6.5,
// R-DM-18): a deletion removes content from the log, while the envelope stays. sync implements
// it; messaging receives it through its constructor, because messaging may not import sync (02
// section 4.4).
type Redactor interface {
	// Redact calls fn for every stored event with resource (kind, id) in ascending seq, writes
	// the returned payload, sets redacted_at if unset, and leaves every other column unchanged.
	// Runs in the ambient transaction; returns the number of rows. Events still buffered in the
	// unit of work are not stored yet and are not redacted.
	//
	// Errors: kind internal outside a unit of work, the error of fn unchanged, and an invalid
	// result of fn (not a JSON object) as kind internal; the caller's unit then rolls back.
	//
	// Example:
	//
	//	n, err := redactor.Redact(ctx, "message", msgID, domain.RedactPayload(tombstone))
	Redact(ctx context.Context, kind string, id ids.ID, fn RedactFunc) (int, error)
}
