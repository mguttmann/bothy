// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// EffectiveState is the state of a component at now: an active component at or after expiresAt
// is expired, whether or not the expiry job has run yet (correctness never depends on the job,
// Design note 15 a); a zero expiresAt never expires. Every other state is returned unchanged.
func EffectiveState(state string, expiresAt, now time.Time) string {
	if state == ComponentActive && !expiresAt.IsZero() && !now.Before(expiresAt) {
		return ComponentExpired
	}
	return state
}

// componentStates are the stored states of a component.
var componentStates = []string{ComponentActive, ComponentUsed, ComponentExpired, ComponentDisabled}

// StateTransition decides a state change of updateComponent (Design note 13 c). to is "" when
// the request changes no state. groupUsed reports that a single_use component of the same
// message is used.
//
// changed false and nil error: no change (to is "" or equals the effective state). Error: 409
// invalid_state_transition (errs.CodeInvalidStateTransition) for every other change: used and
// expired are final (an active component past expiresAt counts as expired); active -> disabled
// is allowed; disabled -> active only before expiresAt and, for a single_use component, only
// while groupUsed is false, so a reactivation never allows a second decision of the single-use
// group (09 11.3). An internal error for a stored state outside the four.
//
// Example:
//
//	changed, err := domain.StateTransition(row.State, row.ExpiresAt, now, *cmd.State, row.SingleUse, groupUsed)
func StateTransition(state string, expiresAt, now time.Time, to string, singleUse, groupUsed bool) (changed bool, err error) {
	if !oneOf(state, componentStates) {
		return false, errs.New(errs.CodeInternalError, "runs: unknown component state")
	}
	eff := EffectiveState(state, expiresAt, now)
	switch {
	case to == "" || to == eff:
		return false, nil
	case eff == ComponentActive && to == ComponentDisabled:
		return true, nil
	case eff == ComponentDisabled && to == ComponentActive &&
		(expiresAt.IsZero() || now.Before(expiresAt)) && (!singleUse || !groupUsed):
		return true, nil
	}
	return false, errs.New(errs.CodeInvalidStateTransition, "the component cannot change to this state")
}

// RouteKind is the way an interaction reaches a run (Design note 10 h).
type RouteKind uint8

// The routes. The zero value is invalid, so a forgotten route never reads as one.
const (
	// RouteComponentRun: the component's own active run.
	RouteComponentRun RouteKind = iota + 1
	// RouteOpenRun: no run; open a new one with trigger_kind interaction.
	RouteOpenRun
	// RouteRefused: see Route.Reason.
	RouteRefused
)

// The reasons of RouteRefused, the reason codes authorize gives for the same facts (Design note
// 8 b).
const (
	routeComponentInactive   = "component_inactive"
	routeRunActive           = "run_active"
	routeRunActiveOtherAgent = "run_active_other_agent"
)

// RouteFacts are the facts of the locked rows that decide the route of an interaction.
type RouteFacts struct {
	CardAgentID        ids.ID // author of the card
	ComponentRunID     ids.ID // message_components.run_id; zero without run
	ComponentRunActive bool   // that run is queued, running or waiting_input
	ActiveRunAgentID   ids.ID // agent of the thread's active run; zero for none
}

// Route is the decision of RouteInteraction.
type Route struct {
	Kind RouteKind
	// Reason is component_inactive, run_active or run_active_other_agent for RouteRefused, else "".
	Reason string
}

// RouteInteraction routes an interaction to a run (Design note 10 h). A component bound to a run
// goes to that run while it is active (else component_inactive: a component of an ended run is
// inactive). A run-less component opens a new run when the thread has no active run; the
// agent's own active run gives run_active (a second run cannot exist, R-DM-14) and another
// agent's run gives run_active_other_agent. authorize decided the refusals before, so the use
// case treats RouteRefused as a consistency failure with the given reason.
func RouteInteraction(f RouteFacts) Route {
	switch {
	case !f.ComponentRunID.IsZero() && f.ComponentRunActive:
		return Route{Kind: RouteComponentRun}
	case !f.ComponentRunID.IsZero():
		return Route{Kind: RouteRefused, Reason: routeComponentInactive}
	case f.ActiveRunAgentID.IsZero():
		return Route{Kind: RouteOpenRun}
	case f.ActiveRunAgentID == f.CardAgentID:
		return Route{Kind: RouteRefused, Reason: routeRunActive}
	}
	return Route{Kind: RouteRefused, Reason: routeRunActiveOtherAgent}
}

// AuditFacts are the facts of an allowed interaction on an approve_actions component (Design
// note 17).
type AuditFacts struct {
	InteractionID, MessageID, RunID  ids.ID // RunID zero without run
	ComponentType                    string
	RequiredCapability, RestrictedTo string
	Definition                       json.RawMessage // normalized
}

// AuditDetails is the detail set of the allow entry of interaction.create.
type AuditDetails struct {
	InteractionID, MessageID, RunID  ids.ID // RunID zero: absent
	ComponentType                    string
	RequiredCapability, RestrictedTo string
	Choice                           string // "" when absent
}

// approvalChoices are the button values of the approval card (09 11.5), the only values the
// audit names.
var approvalChoices = []string{"once", "session", "always", "deny"}

// InteractionAuditDetails returns the detail set of Design note 17: the IDs, the component type,
// required_capability, restricted_to, and choice only for a button whose value is one of once,
// session, always, deny (09 11.5). It never carries custom_id, a label, a value of another kind
// or a form value: they are agent or human text (SEC-AUD-5). A definition that does not decode
// gives no choice.
func InteractionAuditDetails(f AuditFacts) AuditDetails {
	d := AuditDetails{
		InteractionID: f.InteractionID, MessageID: f.MessageID, RunID: f.RunID,
		ComponentType: f.ComponentType, RequiredCapability: f.RequiredCapability, RestrictedTo: f.RestrictedTo,
	}
	if f.ComponentType != ComponentButton {
		return d
	}
	var b struct {
		Value string `json:"value"`
	}
	if json.Unmarshal(f.Definition, &b) == nil && oneOf(b.Value, approvalChoices) {
		d.Choice = b.Value
	}
	return d
}

// InteractionView is the content of the schema Interaction as runs holds it.
type InteractionView struct {
	ID, ComponentID, MessageID, PrincipalID, ClientInteractionID ids.ID
	CustomID                                                     string
	ThreadID, RunID                                              ids.ID          // zero: absent
	Values                                                       json.RawMessage // {} for nil
	State                                                        string
	ResponseText                                                 *string // nil: no response member
	HasResponse                                                  bool    // true: "response" member present ({} when ResponseText nil)
	CreatedAt                                                    time.Time
}

// interactionStates are the states of an interaction (CHECK of 03 interactions.state).
var interactionStates = []string{"received", "dispatched", "acknowledged", "failed", "rejected"}

// interactionJSON is the schema Interaction without response, members in schema order; empty
// optional members are left out because the server never sends JSON null (ADR 0053).
type interactionJSON struct {
	ID                  string          `json:"id"`
	ComponentID         string          `json:"component_id"`
	CustomID            string          `json:"custom_id,omitempty"`
	MessageID           string          `json:"message_id"`
	ThreadID            string          `json:"thread_id,omitempty"`
	RunID               string          `json:"run_id,omitempty"`
	PrincipalID         string          `json:"principal_id"`
	ClientInteractionID string          `json:"client_interaction_id"`
	Values              json.RawMessage `json:"values"`
	State               string          `json:"state"`
	CreatedAt           string          `json:"created_at"`
}

// InteractionCreatedPayload encodes the payload of interaction.created ({"interaction": ...},
// schema InteractionCreatedPayload) with the members of schema Interaction in schema order, no
// JSON null (ADR 0053), created_at in RFC 3339 UTC with milliseconds and Z, and without
// "response": the event announces a new interaction, which has none yet, so ResponseText and
// HasResponse are ignored.
//
// Errors: an internal error for a zero ID among id, component_id, message_id, principal_id and
// client_interaction_id, a zero CreatedAt, a state outside the five, and values that are not a
// JSON object or contain null, so a broken event is never written.
//
// Example:
//
//	payload, err := domain.InteractionCreatedPayload(view)
func InteractionCreatedPayload(v InteractionView) ([]byte, error) {
	for _, id := range []ids.ID{v.ID, v.ComponentID, v.MessageID, v.PrincipalID, v.ClientInteractionID} {
		if id.IsZero() {
			return nil, errs.New(errs.CodeInternalError, "runs: interaction payload without a required id")
		}
	}
	if v.CreatedAt.IsZero() || !oneOf(v.State, interactionStates) {
		return nil, errs.New(errs.CodeInternalError, "runs: interaction payload without created_at or with an unknown state")
	}
	values := emptyAsObject(v.Values)
	doc, ok := decodeJSON(values)
	if _, isObject := doc.(map[string]any); !ok || !isObject || containsNull(doc) {
		return nil, errs.New(errs.CodeInternalError, "runs: interaction values are not an object without null")
	}
	payload := struct {
		Interaction interactionJSON `json:"interaction"`
	}{interactionJSON{
		ID: v.ID.String(), ComponentID: v.ComponentID.String(), CustomID: v.CustomID, MessageID: v.MessageID.String(),
		ThreadID: idText(v.ThreadID), RunID: idText(v.RunID),
		PrincipalID: v.PrincipalID.String(), ClientInteractionID: v.ClientInteractionID.String(),
		Values: values, State: v.State, CreatedAt: v.CreatedAt.UTC().Format(canonicalTime),
	}}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "runs: interaction payload does not encode")
	}
	return b, nil
}

// containsNull reports whether a decoded JSON value holds null anywhere.
func containsNull(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case map[string]any:
		for _, e := range x {
			if containsNull(e) {
				return true
			}
		}
	case []any:
		for _, e := range x {
			if containsNull(e) {
				return true
			}
		}
	}
	return false
}
