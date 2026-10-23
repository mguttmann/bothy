// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The stages of InteractionFacts; they mirror kernel/authorize.InteractionStageAccept and
// InteractionStageDeliver (brief M2-15 design note 8 b and c).
const (
	// InteractionStageAccept is interaction.create: a person operates a component.
	InteractionStageAccept = "accept"
	// InteractionStageDeliver is agent.invoke before the delivery an interaction caused: the
	// component is already used by that interaction and the run is the one the event goes to.
	InteractionStageDeliver = "deliver"
)

// The states of a component (03 message_components.state).
const (
	ComponentActive   = "active"
	ComponentUsed     = "used"
	ComponentExpired  = "expired"
	ComponentDisabled = "disabled"
)

// The values of restricted_to (03 message_components.restricted_to, 07 section 2.4).
const (
	// RestrictedAny admits every person who passes the other rules.
	RestrictedAny = "any"
	// RestrictedRunStarter admits the person who started the component's run, and owners.
	RestrictedRunStarter = "run_starter"
	// RestrictedOwner admits effective owners only.
	RestrictedOwner = "owner"
)

// actionInteractionCreate is the action that operates a component.
const actionInteractionCreate = "interaction.create"

// componentStates are the states a component can have.
var componentStates = []string{ComponentActive, ComponentUsed, ComponentExpired, ComponentDisabled}

// InteractionFacts are the facts of the component of interaction.create and of the re-check of
// agent.invoke before a delivery an interaction caused (05 section 5 point 6, brief M2-15 design
// note 8); they mirror kernel/authorize.InteractionFacts field for field.
//
// Example (an approval card bound to the active run that Robin started):
//
//	f := &domain.InteractionFacts{Stage: domain.InteractionStageAccept, OwnerKind: domain.KindAgent,
//		ComponentState: domain.ComponentActive, RequiredCapability: "approve_actions",
//		RestrictedTo: domain.RestrictedRunStarter, ComponentRunID: run, ComponentRunActive: true, RunStarterID: robin}
type InteractionFacts struct {
	// Stage is InteractionStageAccept or InteractionStageDeliver.
	Stage string
	// OwnerKind is the kind of the card's author: agent or app.
	OwnerKind string
	// ComponentState is the state of the component, read from the locked row.
	ComponentState string
	// ExpiresAt is the expiry of the component; zero means never.
	ExpiresAt time.Time
	// RequiredCapability is the capability the component requires; "" when none.
	RequiredCapability string
	// RestrictedTo is RestrictedAny, RestrictedRunStarter or RestrictedOwner.
	RestrictedTo string
	// ComponentRunID is the run of the component; zero for none.
	ComponentRunID ids.ID
	// ComponentRunActive reports that that run is queued, running or waiting_input.
	ComponentRunActive bool
	// RunStarterID is the person who started that run; zero without a run.
	RunStarterID ids.ID
}

// interactionGuard is the guard of interaction.create: the agent.invoke guard with the interaction
// facts (brief M2-15 design note 8 b). Missing trigger or interaction facts, a stage other than
// accept and a trigger of an agent other than the card's author are internal errors.
//
// A card of an app (M2-25 design note 3 d) takes appInteractionGuard instead: no agent.invoke rule
// applies, because no agent runtime is involved (05 section 5, R-AZ-18).
func interactionGuard(in *Input) (Decision, error) {
	if in.Interaction != nil && in.Interaction.OwnerKind == KindApp {
		return appInteractionGuard(in)
	}
	if in.Invoke == nil || in.Interaction == nil {
		return Decision{}, internalError(actionInteractionCreate + " without its trigger or component")
	}
	if in.Interaction.Stage != InteractionStageAccept {
		return Decision{}, internalError(actionInteractionCreate + " outside stage accept")
	}
	if in.Invoke.AgentID != in.Resource.OwnerID {
		return Decision{}, internalError(actionInteractionCreate + ": the agent to trigger is not the card's author")
	}
	return invokeGuard(in, in.Invoke)
}

// checkInteraction reports interaction facts that cannot be evaluated as an internal error: a
// card whose author is neither an agent nor an app, and a state, restriction or capability outside its list.
func checkInteraction(f *InteractionFacts) error {
	if f.OwnerKind != KindAgent && f.OwnerKind != KindApp {
		return internalError("interaction facts of a card whose author is neither an agent nor an app")
	}
	if !slices.Contains(componentStates, f.ComponentState) {
		return internalError("interaction facts with an unknown component state")
	}
	switch f.RestrictedTo {
	case RestrictedAny, RestrictedRunStarter, RestrictedOwner:
	default:
		return internalError("interaction facts with an unknown restriction")
	}
	if f.RequiredCapability != "" {
		if _, err := ParseCaps([]string{f.RequiredCapability}); err != nil {
			return internalError("interaction facts with an unknown capability")
		}
	}
	return nil
}

// componentRules is point 6 of the guard with interaction facts (brief M2-15 design note 8 b and
// c): at stage accept an inactive component (a state other than active, an expiry at or before
// now, or an ended run) is component_inactive; at both stages a missing required capability among
// the effective capabilities is capability_missing, and a restriction the principal does not meet
// is component_restricted. Effective owners hold every capability and meet every restriction.
func componentRules(in *Input, f *InteractionFacts) Reason {
	if f.Stage == InteractionStageAccept && !componentActive(f, in.Now) {
		return ReasonComponentInactive
	}
	if f.RequiredCapability != "" {
		// checkInteraction has parsed the name before.
		caps, _ := ParseCaps([]string{f.RequiredCapability})
		if !caps.SubsetOf(in.Eff.Caps) {
			return ReasonCapabilityMissing
		}
	}
	switch f.RestrictedTo {
	case RestrictedRunStarter:
		if !in.Eff.Owner && (f.RunStarterID.IsZero() || f.RunStarterID != in.Principal.ID) {
			return ReasonComponentRestricted
		}
	case RestrictedOwner:
		if !in.Eff.Owner {
			return ReasonComponentRestricted
		}
	}
	return ""
}

// componentActive reports an active component that has not expired at now and whose run, if it
// has one, is still active.
func componentActive(f *InteractionFacts, now time.Time) bool {
	if f.ComponentState != ComponentActive {
		return false
	}
	if !f.ExpiresAt.IsZero() && !now.Before(f.ExpiresAt) {
		return false
	}
	return f.ComponentRunID.IsZero() || f.ComponentRunActive
}

// appInteractionGuard is point 6 of 05 section 5 for a card of an app (brief M2-25 design note 3
// d): the app-owner guard with components:write first, then the component rules. At stage accept
// a false guard, a state other than active or a passed expiry give component_inactive; at both
// stages a missing required capability gives capability_missing and a restriction the person does
// not meet component_restricted (an app card has no run, so run_starter admits only owners). Stage
// deliver is the re-check of the job runs.app_dispatch: the state is not checked (the interaction
// already used the component), the guard is. Missing facts, trigger facts of an agent and a
// component run of an app card are internal errors.
func appInteractionGuard(in *Input) (Decision, error) {
	f := in.Interaction
	switch {
	case in.AppOwner == nil:
		return Decision{}, internalError(actionInteractionCreate + " of an app card without the facts of the app")
	case in.Invoke != nil:
		return Decision{}, internalError(actionInteractionCreate + " of an app card with the trigger of an agent")
	case !f.ComponentRunID.IsZero():
		return Decision{}, internalError(actionInteractionCreate + " of an app card bound to a run")
	case f.Stage != InteractionStageAccept && f.Stage != InteractionStageDeliver:
		return Decision{}, internalError(actionInteractionCreate + " with an unknown stage")
	case in.AppOwner.ID != in.Resource.OwnerID:
		return Decision{}, internalError(actionInteractionCreate + ": the facts of the app are not the card's author")
	}
	if err := checkInteraction(f); err != nil {
		return Decision{}, err
	}
	if !AppOwnerReachable(*in.AppOwner, ScopeComponentsWrite) {
		return deny(OutcomeForbidden, ReasonComponentInactive), nil
	}
	if r := componentRules(in, f); r != "" {
		return deny(OutcomeForbidden, r), nil
	}
	return Allow, nil
}
