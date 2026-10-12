// SPDX-License-Identifier: Apache-2.0

// Package domain holds the domain values of the authz module. This ticket (M1-25) brings the
// closed action catalogue: the metadata of every action, event type, WebSocket op and terminal
// channel op of docs/spec/actions.yaml (ADR 0049), generated into catalog_gen.go by
// tools/catalogen and read through the lookups of this file. The values are keyed by plain
// strings because the domain layer imports nothing outside kernel-core
// (docs/spec/02-architektur.md "Schichten im Modul"); the typed identifiers live in
// kernel/authorize.
//
// Example:
//
//	spec, ok := domain.LookupAction("message.read")
//	if ok && spec.StepUp {
//		// the session needs a fresh confirmation (05 section 2.4)
//	}
package domain

import (
	"slices"
	"strings"
)

// Values of ActionSpec.Effect (05 section 2.7).
const (
	EffectRead  = "read"
	EffectWrite = "write"
)

// Directions of a gateway op (OpSpec.Direction), as docs/spec/actions.yaml writes them.
const (
	DirectionClientToServer     = "client_to_server"
	DirectionAgentToServer      = "agent_to_server"
	DirectionAgentOrAppToServer = "agent_or_app_to_server"
	DirectionServerToClient     = "server_to_client"
)

// channelOpSeparator joins channel and op in the id of a channel op.
const channelOpSeparator = "/"

// ActionSpec is one action of 05 section 4 with its columns.
type ActionSpec struct {
	// ID is the action id, for example message.read.
	ID string
	// Resources are the resource kinds of the column "Ressource"; empty for an action whose
	// resource is that of its target action (access.preview).
	Resources []string
	// Right is the column "Recht" verbatim (05 text); M1-28 interprets it.
	Right string
	// Owner is true when only owners may run the action (column "Owner" ja).
	Owner bool
	// OwnerAsTarget is true when the column "Owner" says "wie Zielaktion": the owner rule of the
	// target action applies (access.preview).
	OwnerAsTarget bool
	// StepUp is true for "ja", "bedingt" and "frisch" (05 2.4). Until the ticket that supplies the
	// condition, "bedingt" counts as "ja" (07 4.2).
	StepUp bool
	// StepUpConditional marks "bedingt"; M1-39 and M1-40 add the condition in authz.
	StepUpConditional bool
	// StepUpFresh marks "frisch": a fresh, unconsumed step-up that the action consumes; only the
	// terminal actions of 05 4.14, evaluated from M2-40.
	StepUpFresh bool
	// Audit is the column "Audit" as 05 writes it: ja, abl, nein or the conditional text.
	Audit string
	// Rights is true when the action raises permission_version (05 rule 7).
	Rights bool
	// RateLimitClass is the class of 05 section 3, "" when 05 says "keine".
	RateLimitClass string
	// System is true when Right is exactly "system" (05 rule 10).
	System bool
	// Effect is EffectRead or EffectWrite (05 2.7); the maintenance layer reads it (M7-08).
	Effect string
	// DeliveryCheck marks an internal delivery check without a rate-limit class (push.deliver).
	DeliveryCheck bool
	// AppScope is the scope an app needs for the action (05 2.6), "" when none.
	AppScope string
}

// EventSpec is one durable or ephemeral event type of 05 section 6.
type EventSpec struct {
	// Type is the event type, for example message.created.
	Type string
	// Durable is true for durable, false for ephemeral types.
	Durable bool
	// Resource is the column "Ressource" verbatim.
	Resource string
	// Resources are the resource kinds named in Resource.
	Resources []string
	// ReceiveAction is the column "Empfangs-Aktion" verbatim (05 text).
	ReceiveAction string
	// MentionedActions are the catalogue actions named in ReceiveAction, sorted.
	MentionedActions []string
	// AudienceSelf is true when only the subject itself receives the event.
	AudienceSelf bool
	// Receivers says which principals receive a self event: principal, agent or agent_or_app;
	// "" when AudienceSelf is false.
	Receivers string
	// ResourceVersion is true when the envelope carries resource.version (04 "Zustellgarantie" 3).
	ResourceVersion bool
	// Note is the column "Hinweis" (durable) or "Zusatzbedingung" (ephemeral) verbatim, "" when
	// empty; for example the extra check before an agent invocation is delivered.
	Note string
}

// OpSpec is one WebSocket op of the gateway (05 section 7, first table).
type OpSpec struct {
	// Op is the op name, for example heartbeat.
	Op string
	// Direction is one of the Direction constants.
	Direction string
	// ActionsText is the column "Aktion" verbatim (05 text).
	ActionsText string
	// MentionedActions are the catalogue actions the op checks, sorted.
	MentionedActions []string
	// PerEventReceiveAction is true for dispatch, which checks the receive action of each event.
	PerEventReceiveAction bool
}

// Inbound reports whether a client, an agent or an app sends the op to the server; only such ops
// are registered by modules (server-to-client ops are part of the session).
func (o OpSpec) Inbound() bool {
	return o.Direction == DirectionClientToServer || o.Direction == DirectionAgentToServer ||
		o.Direction == DirectionAgentOrAppToServer
}

// ChannelOpSpec is one op of a terminal channel (05 section 7, second table); it is never a gateway
// op.
type ChannelOpSpec struct {
	// Channel is terminal_stream, host_control or host_data.
	Channel string
	// Op is the message name; noise_frame for binary frames.
	Op string
	// Direction is the direction as docs/spec/actions.yaml writes it (for example server_to_host).
	Direction string
	// Action is the action the channel checks.
	Action string
	// ActionText is the column "Aktion" verbatim when it says more than the action, else "".
	ActionText string
}

// LookupAction returns the action with id; ok is false for an id the catalogue does not know.
// The returned value shares no slice with the catalogue.
func LookupAction(id string) (ActionSpec, bool) {
	a, ok := actionSpecs[id]
	a.Resources = slices.Clone(a.Resources)
	return a, ok
}

// LookupEvent returns the event type t; ok is false for a type the catalogue does not know.
func LookupEvent(t string) (EventSpec, bool) {
	e, ok := eventSpecs[t]
	e.Resources = slices.Clone(e.Resources)
	e.MentionedActions = slices.Clone(e.MentionedActions)
	return e, ok
}

// LookupOp returns the gateway op named op; ok is false for an unknown op and for every terminal
// channel op.
func LookupOp(op string) (OpSpec, bool) {
	o, ok := opSpecs[op]
	o.MentionedActions = slices.Clone(o.MentionedActions)
	return o, ok
}

// LookupChannelOp returns the op of a terminal channel; ok is false for an unknown pair. A
// gateway op never matches, because every channel op belongs to a channel.
func LookupChannelOp(channel, op string) (ChannelOpSpec, bool) {
	if channel == "" || strings.Contains(channel, channelOpSeparator) {
		return ChannelOpSpec{}, false
	}
	c, ok := channelOpSpecs[channel+channelOpSeparator+op]
	return c, ok
}

// LookupChannelOpID returns the channel op written as <channel>/<op> (the value of
// kernel/authorize.ChannelOp).
func LookupChannelOpID(id string) (ChannelOpSpec, bool) {
	channel, op, found := strings.Cut(id, channelOpSeparator)
	if !found {
		return ChannelOpSpec{}, false
	}
	return LookupChannelOp(channel, op)
}

// ActionIDs returns the ids of all actions, sorted ascending, in a new slice.
func ActionIDs() []string { return slices.Clone(actionIDs) }

// EventTypes returns all event types, sorted ascending, in a new slice.
func EventTypes() []string { return slices.Clone(eventTypes) }

// OpNames returns the names of all gateway ops, sorted ascending, in a new slice.
func OpNames() []string { return slices.Clone(opNames) }

// ChannelOpIDs returns all terminal channel ops as <channel>/<op>, sorted ascending, in a new slice.
func ChannelOpIDs() []string { return slices.Clone(channelOpIDs) }

// ResourceKinds returns the resource kinds of all actions and event types, sorted ascending, in a
// new slice.
func ResourceKinds() []string { return slices.Clone(resourceKinds) }
