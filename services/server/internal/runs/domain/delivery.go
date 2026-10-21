// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The invocation-bearing event types of 04 section 15.3 and 05 section 6: every delivery of one of
// them to an agent passes runs.Delivery (M2-08).
const (
	EventAgentInvocation    = "agent.invocation"
	EventAgentFollowup      = "agent.followup"
	EventInteractionCreated = "interaction.created"
	EventCommandInvoked     = "command.invoked"
)

// DeliveryDenial says what a denial of agent.invoke before a delivery changes (M2-08 design note
// 3 e to g).
type DeliveryDenial uint8

// The consequences of a denial before a delivery.
const (
	// DenyKeepRun sends nothing and leaves the run unchanged (agent.invocation of a started run,
	// command.invoked; ending such a run is the revocation path of M2-09).
	DenyKeepRun DeliveryDenial = iota + 1
	// DenyFailRun fails the queued run with the mapped reason and the triggering message with it.
	DenyFailRun
	// DenyFailFollowup refuses the follow-up: its message gets trigger_state failed, the run is
	// unchanged (ruling D-M2-08-5).
	DenyFailFollowup
)

// DeliveryRule is one cell of the delivery matrix: whether the delivery is checked at all, whether
// an allowed delivery records the jti of its token in runs.invocation_jti and what a denial does.
type DeliveryRule struct {
	// Skip: the run has ended; nothing is checked, nothing is sent (nothing can start).
	Skip bool
	// Record: an allowed delivery replaces runs.invocation_jti and clears renewed_from_jti.
	Record bool
	// Denial is the consequence of a denial; zero when Skip.
	Denial DeliveryDenial
}

// ErrUnknownDelivery is the error of DeliveryMatrix for an event type that bears no invocation or
// a state outside 09 5.1.
var ErrUnknownDelivery = errors.New("runs: no delivery rule for this event type and run state")

// DeliveryMatrix returns the rule of a delivery of eventType for a run in state (M2-08 design note
// 2, ruling D-M2-08-3): terminal runs are skipped without a check; agent.invocation records its
// jti only while queued, because in waiting_input the original invocation must not resume the run
// without an answer of a person (D-W-44); agent.followup and interaction.created record it only in
// waiting_input, where they may resume the run; command.invoked never records it (09 5.1 lists it
// under no start).
//
// Example:
//
//	r, _ := domain.DeliveryMatrix(domain.EventAgentInvocation, domain.StateQueued) // Record, DenyFailRun
func DeliveryMatrix(eventType string, state State) (DeliveryRule, error) {
	switch state {
	case StateCompleted, StateFailed, StateCancelled:
		if !InvocationBearing(eventType) {
			return DeliveryRule{}, ErrUnknownDelivery
		}
		return DeliveryRule{Skip: true}, nil
	case StateQueued, StateRunning, StateWaitingInput:
	default:
		return DeliveryRule{}, ErrUnknownDelivery
	}
	switch eventType {
	case EventAgentInvocation:
		if state == StateQueued {
			return DeliveryRule{Record: true, Denial: DenyFailRun}, nil
		}
		return DeliveryRule{Denial: DenyKeepRun}, nil
	case EventAgentFollowup, EventInteractionCreated:
		return DeliveryRule{Record: state == StateWaitingInput, Denial: DenyFailFollowup}, nil
	case EventCommandInvoked:
		return DeliveryRule{Denial: DenyKeepRun}, nil
	}
	return DeliveryRule{}, ErrUnknownDelivery
}

// InvocationBearing reports whether an event of eventType carries an invocation and therefore
// passes runs.Delivery before every delivery to an agent: agent.invocation, agent.followup,
// interaction.created and command.invoked (04 section 15.3, 05 section 6).
func InvocationBearing(eventType string) bool {
	switch eventType {
	case EventAgentInvocation, EventAgentFollowup, EventInteractionCreated, EventCommandInvoked:
		return true
	}
	return false
}

// The deny reasons of authorize (07 4.1) the delivery maps to their own run reason.
const (
	denyBillingPersonal  = "billing_mode_personal_not_holder"
	denyToolClass        = "tool_class_owner_only"
	denyProjectNotReady  = "project_not_ready"
	denyChannelReadOnly  = "channel_read_only"
	denyChannelArchived  = "channel_archived"
	denyPrincipalInactve = "principal_inactive"
	denyNotReachable     = "agent_not_reachable"
)

// DeliveryReason maps the reason of a denial of agent.invoke before a delivery to the code of
// runs.state_reason, trigger_state_reason and the counter value of bothy.agent.invocation.denied
// (M2-08 design note 6, ruling D-M2-08-4). agentActive is whether the agent is active now (the
// agents port): agent_not_reachable of an inactive agent is agent_disabled. Every other reason,
// unknown ones included, is access_revoked (AZ-M51). The audit keeps the exact authorize reason.
//
// Example:
//
//	domain.DeliveryReason("no_grant", true)            // access_revoked
//	domain.DeliveryReason("agent_not_reachable", false) // agent_disabled
func DeliveryReason(authzReason string, agentActive bool) string {
	switch authzReason {
	case denyBillingPersonal:
		return RunReasonBillingModePersonalNotHolder
	case denyToolClass:
		return RunReasonToolClassOwnerOnly
	case denyProjectNotReady:
		return RunReasonProjectNotReady
	case denyChannelReadOnly, denyChannelArchived:
		return RunReasonChannelReadOnly
	case denyPrincipalInactve:
		return RunReasonUserDisabled
	case denyNotReachable:
		if !agentActive {
			return RunReasonAgentDisabled
		}
	}
	return RunReasonAccessRevoked
}

// DeliveryReasons returns every value DeliveryReason yields, in the order of design note 6: the
// value list of bothy.reason of bothy.agent.invocation.denied (signals.md).
func DeliveryReasons() []string {
	return []string{
		RunReasonBillingModePersonalNotHolder, RunReasonToolClassOwnerOnly, RunReasonProjectNotReady, RunReasonChannelReadOnly,
		RunReasonUserDisabled, RunReasonAgentDisabled, RunReasonAccessRevoked,
	}
}

// QueuedTooOld reports whether a run queued at queuedAt has reached run.queued_max_age at now: at
// exactly queuedAt plus maxAge it is too old (09 5.5, D-AG-16).
func QueuedTooOld(queuedAt, now time.Time, maxAge time.Duration) bool {
	return !now.Before(queuedAt.Add(maxAge))
}

// LevelTrigger is the claim level of every allowed delivery: an allow of agent.invoke implies the
// effective level trigger, the highest (05 4.10, 07 2.3; M2-08 design note 7).
const LevelTrigger = "trigger"

// DeliveryFacts are the facts of one allowed delivery from which DeliveryGrant builds the grant of
// the token (M2-08 design note 7).
type DeliveryFacts struct {
	// EventType is the invocation-bearing type of the event.
	EventType string
	// RunTriggerKind is runs.trigger_kind (message, command or interaction).
	RunTriggerKind string
	// RunID, AgentID and TriggeredBy are the run, its agent (aud) and the trigger (sub).
	RunID, AgentID, TriggeredBy ids.ID
	// ObjectID is the triggering object of the resolver: the message of agent.invocation and
	// agent.followup, the interaction or the command of the others.
	ObjectID ids.ID
	// ThreadID, ChannelID and SpaceID are the run's thread now and its channel and space.
	ThreadID, ChannelID, SpaceID ids.ID
	// BillingMode and ToolClass are the configured agents.billing_mode and agents.tool_class now,
	// never the reported class.
	BillingMode, ToolClass string
	// PermissionVersion is the version the check pinned.
	PermissionVersion int64
}

// DeliveryGrant returns the grant of the token of an allowed delivery: trigger_kind per event type
// (runs.trigger_kind for agent.invocation, followup, interaction, command), the triggering object
// in the member of its kind, level trigger and the configured policy. Errors: ErrUnknownDelivery
// for a type outside InvocationBearing or a run trigger kind the event cannot carry.
func DeliveryGrant(f DeliveryFacts) (InvocationGrant, error) {
	g := InvocationGrant{
		AgentID: f.AgentID, TriggeredBy: f.TriggeredBy, RunID: f.RunID, ThreadID: f.ThreadID, ChannelID: f.ChannelID,
		SpaceID: f.SpaceID, Level: LevelTrigger, BillingMode: f.BillingMode, ToolClass: f.ToolClass,
		PermissionVersion: f.PermissionVersion,
	}
	switch f.EventType {
	case EventAgentInvocation:
		g.TriggerKind = TriggerKind(f.RunTriggerKind)
	case EventAgentFollowup:
		g.TriggerKind = TriggerFollowup
	case EventInteractionCreated:
		g.TriggerKind = TriggerInteraction
	case EventCommandInvoked:
		g.TriggerKind = TriggerCommand
	default:
		return InvocationGrant{}, ErrUnknownDelivery
	}
	switch g.TriggerKind {
	case TriggerMessage, TriggerFollowup:
		g.MessageID = f.ObjectID
	case TriggerInteraction:
		g.InteractionID = f.ObjectID
	case TriggerCommand:
		g.CommandID = f.ObjectID
	case TriggerAutocomplete:
		// Autocomplete has no run and no delivery of these events (M2-17).
		return InvocationGrant{}, ErrUnknownDelivery
	default:
		return InvocationGrant{}, ErrUnknownDelivery
	}
	return g, nil
}

// PrincipalRefPayload is the schema PrincipalRef inside an event payload.
type PrincipalRefPayload struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Handle       string `json:"handle"`
	DisplayName  string `json:"display_name"`
	AvatarFileID string `json:"avatar_file_id,omitzero"`
}

// InvocationPayloadInput are the members of AgentInvocationPayload of a message trigger (M2-08
// design note 14) or of an interaction trigger (M2-15), except the session, which InvocationPayload
// takes as its own required argument (M2-12).
type InvocationPayloadInput struct {
	RunID, ThreadID, ChannelID, SpaceID ids.ID
	// TriggerKind is message ("" means message) or interaction; MessageID is the triggering
	// message of a message trigger, InteractionID the interaction of an interaction trigger.
	TriggerKind              string
	MessageID, InteractionID ids.ID
	// TriggeredBy is the author as the messaging port returns it.
	TriggeredBy PrincipalRefPayload
	// BillingMode is the configured mode at the acceptance.
	BillingMode string
	// SettingsVersion is channels.settings_version; 0 omits the member.
	SettingsVersion int64
}

// invocationPayload is AgentInvocationPayload in schema order.
type invocationPayload struct {
	RunID           string              `json:"run_id"`
	ThreadID        string              `json:"thread_id"`
	ChannelID       string              `json:"channel_id"`
	SpaceID         string              `json:"space_id,omitzero"`
	TriggerKind     string              `json:"trigger_kind"`
	MessageID       string              `json:"message_id,omitzero"`
	InteractionID   string              `json:"interaction_id,omitzero"`
	TriggeredBy     PrincipalRefPayload `json:"triggered_by"`
	BillingMode     string              `json:"billing_mode"`
	AgentSession    json.RawMessage     `json:"agent_session,omitempty"`
	SessionMode     string              `json:"session_mode,omitempty"`
	SettingsVersion int64               `json:"settings_version,omitzero"`
}

// errPayload reports a payload input with a zero ID or an empty required member.
var errPayload = errors.New("runs: incomplete event payload")

// InvocationPayload encodes the payload of agent.invocation for a message trigger (the trigger
// kind message and message_id) or an interaction trigger (trigger_kind interaction and
// interaction_id, no message_id; M2-15 design note 11 c): IDs, the triggering person, the billing
// mode, the session of DecideSession and the settings version, never content, a prompt, skills,
// form values or a project binding (04 2.2 rules 1 and 4), and no JSON null (ADR 0053). The
// session decision is a required argument, so every writer of agent.invocation decides it (M2-12
// design note 9): an operator run carries session_mode and, where a row exists, agent_session; a
// guest run carries neither (09 3.6). Errors: a zero ID, the object of the other trigger kind, an
// unknown trigger kind, an empty person or billing mode, a session without mode, a guest session
// with a row, or resume or bind without a row.
//
// Example:
//
//	p, err := domain.InvocationPayload(in, domain.DecideSession(true, nil)) // session_mode new
func InvocationPayload(in InvocationPayloadInput, session SessionDecision) ([]byte, error) {
	kind := in.TriggerKind
	if kind == "" {
		kind = string(TriggerMessage)
	}
	var object ids.ID
	switch TriggerKind(kind) {
	case TriggerMessage:
		object = in.MessageID
		if !in.InteractionID.IsZero() {
			return nil, errPayload
		}
	case TriggerInteraction:
		object = in.InteractionID
		if !in.MessageID.IsZero() {
			return nil, errPayload
		}
	case TriggerFollowup, TriggerCommand, TriggerAutocomplete:
		return nil, errPayload
	default:
		return nil, errPayload
	}
	if in.RunID.IsZero() || in.ThreadID.IsZero() || in.ChannelID.IsZero() || object.IsZero() || in.TriggeredBy.ID == "" ||
		in.TriggeredBy.Kind == "" || in.BillingMode == "" {
		return nil, errPayload
	}
	if err := session.check(); err != nil {
		return nil, err
	}
	p := invocationPayload{
		RunID: in.RunID.String(), ThreadID: in.ThreadID.String(), ChannelID: in.ChannelID.String(), TriggerKind: kind,
		TriggeredBy: in.TriggeredBy, BillingMode: in.BillingMode, SettingsVersion: in.SettingsVersion,
	}
	if kind == string(TriggerMessage) {
		p.MessageID = object.String()
	} else {
		p.InteractionID = object.String()
	}
	if session.Mode != SessionGuest {
		p.SessionMode, p.AgentSession = string(session.Mode), session.Row
	}
	if !in.SpaceID.IsZero() {
		p.SpaceID = in.SpaceID.String()
	}
	return json.Marshal(p)
}

// followupPayload is AgentFollowupPayload.
type followupPayload struct {
	RunID     string `json:"run_id"`
	ThreadID  string `json:"thread_id"`
	MessageID string `json:"message_id"`
}

// FollowupPayload encodes the payload of agent.followup (run, thread and the follow-up message),
// which M2-11 appends. Errors: a zero ID.
func FollowupPayload(runID, threadID, messageID ids.ID) ([]byte, error) {
	if runID.IsZero() || threadID.IsZero() || messageID.IsZero() {
		return nil, errPayload
	}
	return json.Marshal(followupPayload{RunID: runID.String(), ThreadID: threadID.String(), MessageID: messageID.String()})
}

// interactionPayloadIDs is the part of InteractionCreatedPayload the delivery reads.
type interactionPayloadIDs struct {
	Interaction struct {
		ID    string `json:"id"`
		RunID string `json:"run_id"`
	} `json:"interaction"`
}

// InteractionDeliveryIDs reads interaction.id and interaction.run_id of an
// InteractionCreatedPayload, the interaction and the run of a delivery of interaction.created
// (M2-15 design note 11 a). Errors: a payload that is not the schema or names no valid IDs.
func InteractionDeliveryIDs(payload []byte) (interaction, run ids.ID, err error) {
	var p interactionPayloadIDs
	if err := json.Unmarshal(payload, &p); err != nil {
		return ids.ID{}, ids.ID{}, errPayload
	}
	interaction, err = ids.Parse(p.Interaction.ID)
	if err != nil || interaction.IsZero() {
		return ids.ID{}, ids.ID{}, errPayload
	}
	run, err = ids.Parse(p.Interaction.RunID)
	if err != nil || run.IsZero() {
		return ids.ID{}, ids.ID{}, errPayload
	}
	return interaction, run, nil
}

// InvocationInteractionID reads interaction_id of an AgentInvocationPayload of trigger_kind
// interaction (M2-15 design note 11 b); zero when the payload names none. Errors: a payload that
// is not JSON or an interaction_id that is not an ID.
func InvocationInteractionID(payload []byte) (ids.ID, error) {
	if len(payload) == 0 {
		return ids.ID{}, nil
	}
	var p struct {
		InteractionID string `json:"interaction_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return ids.ID{}, errPayload
	}
	if p.InteractionID == "" {
		return ids.ID{}, nil
	}
	id, err := ids.Parse(p.InteractionID)
	if err != nil {
		return ids.ID{}, errPayload
	}
	return id, nil
}

// FollowupMessageID reads message_id of an AgentFollowupPayload, the trigger of agent.followup at
// delivery (M2-08 design note 5). Errors: a payload that is not the schema or names no valid ID.
func FollowupMessageID(payload []byte) (ids.ID, error) {
	var p followupPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ids.ID{}, errPayload
	}
	id, err := ids.Parse(p.MessageID)
	if err != nil || id.IsZero() {
		return ids.ID{}, errPayload
	}
	return id, nil
}
