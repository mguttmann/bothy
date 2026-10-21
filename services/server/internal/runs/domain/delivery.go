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
	// Control marks the delivery of command.invoked of a control command, whose grant may lack a
	// run (R-AZ-25, M2-16 design note 13 c).
	Control bool
}

// DeliveryGrant returns the grant of the token of an allowed delivery: trigger_kind per event type
// (runs.trigger_kind for agent.invocation, followup, interaction, command), the triggering object
// in the member of its kind, level trigger and the configured policy. Errors: ErrUnknownDelivery
// for a type outside InvocationBearing or a run trigger kind the event cannot carry.
func DeliveryGrant(f DeliveryFacts) (InvocationGrant, error) {
	g := InvocationGrant{
		AgentID: f.AgentID, TriggeredBy: f.TriggeredBy, RunID: f.RunID, ThreadID: f.ThreadID, ChannelID: f.ChannelID,
		SpaceID: f.SpaceID, Level: LevelTrigger, BillingMode: f.BillingMode, ToolClass: f.ToolClass,
		PermissionVersion: f.PermissionVersion, Control: f.Control,
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

// InvocationPayloadInput are the members of AgentInvocationPayload (M2-08 design note 14) of a
// message trigger, with TriggerKind command of a command run (M2-16 design note 12 c) and with
// TriggerKind interaction of an interaction trigger (M2-15); agent_session comes with M2-12.
type InvocationPayloadInput struct {
	RunID, ThreadID, ChannelID, SpaceID ids.ID
	// MessageID is the triggering message of trigger kind message; zero for command.
	MessageID ids.ID
	// TriggeredBy is the triggering person as PrincipalRef.
	TriggeredBy PrincipalRefPayload
	// BillingMode is the configured mode at the acceptance.
	BillingMode string
	// SettingsVersion is channels.settings_version; 0 omits the member.
	SettingsVersion int64
	// TriggerKind is message ("" means message), command or interaction.
	TriggerKind string
	// InteractionID is the interaction of trigger kind interaction; zero otherwise.
	InteractionID ids.ID
	// CommandInvocationID, CommandID, CommandName and Options are set exactly for command: the
	// invocation_id of the answer, the command and the parsed options in definition order.
	CommandInvocationID, CommandID ids.ID
	CommandName                    string
	Options                        []OptionValue
}

// invocationPayload is AgentInvocationPayload in schema order.
type invocationPayload struct {
	RunID               string                `json:"run_id"`
	ThreadID            string                `json:"thread_id"`
	ChannelID           string                `json:"channel_id"`
	SpaceID             string                `json:"space_id,omitzero"`
	TriggerKind         string                `json:"trigger_kind"`
	MessageID           string                `json:"message_id,omitzero"`
	InteractionID       string                `json:"interaction_id,omitzero"`
	CommandInvocationID string                `json:"command_invocation_id,omitzero"`
	CommandID           string                `json:"command_id,omitzero"`
	CommandName         string                `json:"command_name,omitzero"`
	Options             *[]optionValuePayload `json:"options,omitempty"`
	TriggeredBy         PrincipalRefPayload   `json:"triggered_by"`
	BillingMode         string                `json:"billing_mode"`
	SettingsVersion     int64                 `json:"settings_version,omitzero"`
}

// errPayload reports a payload input with a zero ID or an empty required member.
var errPayload = errors.New("runs: incomplete event payload")

// InvocationPayload encodes the payload of agent.invocation: IDs, the trigger kind, the triggering
// person, the billing mode and the settings version, for a message trigger the message, for a
// command run the invocation, the command and its parsed options, for an interaction trigger the
// interaction (no message_id; M2-15 design note 11 c), never content, a prompt, skills, form
// values or a project binding (04 2.2 rules 1 and 4), and no JSON null (ADR 0053). Errors: a zero
// ID, an empty person or billing mode, or the members of another trigger kind.
func InvocationPayload(in InvocationPayloadInput) ([]byte, error) {
	if in.RunID.IsZero() || in.ThreadID.IsZero() || in.ChannelID.IsZero() || in.TriggeredBy.ID == "" ||
		in.TriggeredBy.Kind == "" || in.BillingMode == "" {
		return nil, errPayload
	}
	p := invocationPayload{
		RunID: in.RunID.String(), ThreadID: in.ThreadID.String(), ChannelID: in.ChannelID.String(),
		TriggeredBy: in.TriggeredBy, BillingMode: in.BillingMode, SettingsVersion: in.SettingsVersion,
	}
	commandSet := !in.CommandID.IsZero() || !in.CommandInvocationID.IsZero() || in.CommandName != "" || in.Options != nil
	switch TriggerKind(in.TriggerKind) {
	case "", TriggerMessage:
		if in.MessageID.IsZero() || !in.InteractionID.IsZero() || !in.CommandID.IsZero() || !in.CommandInvocationID.IsZero() || in.CommandName != "" || in.Options != nil {
			return nil, errPayload
		}
		p.TriggerKind, p.MessageID = string(TriggerMessage), in.MessageID.String()
	case TriggerCommand:
		if !in.MessageID.IsZero() || !in.InteractionID.IsZero() || in.CommandID.IsZero() || in.CommandInvocationID.IsZero() || in.CommandName == "" {
			return nil, errPayload
		}
		opts := optionPayloads(in.Options)
		p.TriggerKind, p.CommandInvocationID, p.CommandID, p.CommandName, p.Options = string(TriggerCommand),
			in.CommandInvocationID.String(), in.CommandID.String(), in.CommandName, &opts
	case TriggerInteraction:
		if !in.MessageID.IsZero() || in.InteractionID.IsZero() || commandSet {
			return nil, errPayload
		}
		p.TriggerKind, p.InteractionID = string(TriggerInteraction), in.InteractionID.String()
	case TriggerFollowup, TriggerAutocomplete:
		// agent.invocation carries message, command and interaction runs.
		return nil, errPayload
	default:
		return nil, errPayload
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
