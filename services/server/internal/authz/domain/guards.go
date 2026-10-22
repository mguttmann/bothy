// SPDX-License-Identifier: Apache-2.0

package domain

import "github.com/OWNER/bothy/services/server/internal/kernel/ids"

// The billing modes and tool classes of agents (03 agents, 07 section 2.11).
const (
	BillingPersonal     = "personal"
	BillingOrganization = "organization"
	ToolChat            = "chat"
	ToolScoped          = "scoped"
	ToolHost            = "host"
	OriginHuman         = "human"
)

// InvokeFacts are the facts of the agent.invoke guard (05 section 5), gathered by the application
// layer from the descriptor, the principal and policy sources and the channel.
type InvokeFacts struct {
	// AgentID is the agent to trigger; TriggerOrigin the origin of the trigger.
	AgentID       ids.ID
	TriggerOrigin string
	// MentionsAgent reports a mention of the agent in the trigger.
	MentionsAgent bool
	// ActiveRunAgentID is the agent of the thread's active run; zero for none.
	ActiveRunAgentID ids.ID
	// NewThread marks the auto thread of the trigger: the main-thread mention rule applies (05
	// section 5 point 3 "for new threads", M2-08).
	NewThread bool
	// AgentFound, AgentKind and AgentStatus come from the principal source.
	AgentFound  bool
	AgentKind   string
	AgentStatus string
	// PolicyFound, BillingMode, CredentialHolderID, ToolClass and ReportedToolClass come from the
	// agent policy source.
	PolicyFound        bool
	BillingMode        string
	CredentialHolderID ids.ID
	ToolClass          string
	ReportedToolClass  string
	// AgentLevel is the agent's own effective level on the channel (guest rule).
	AgentLevel Level
	// DevEnvironmentReady is a ready own dev environment of the principal for the channel (SR3).
	DevEnvironmentReady bool
	// The channel's mention settings and project binding.
	RequireMention, ThreadRequireMention, FreeResponse, AllowMentionedAgents bool
	ProjectBound, ProjectReady                                               bool
}

// toolRank orders the tool classes chat < scoped < host; an unknown class counts as host (fail
// closed).
func toolRank(c string) int {
	switch c {
	case "", ToolChat:
		return 0
	case ToolScoped:
		return 1
	}
	return 2
}

// The actions with a guard.
const (
	actionAgentInvoke = "agent.invoke"
	actionGrantCreate = "grant.create"
	actionGrantRevoke = "grant.revoke"
	actionTokenCreate = "token.create"
)

// guards applies the guards of the action (07 section 4.2 step 9): agent.invoke, the same guard
// with the interaction rules on interaction.create (M2-15), token.create, the mode guard of
// run.start (M2-09) and the terminal guards SR10, SR11 and SR12 of terminal.open and
// terminal.attach (M2-40). The
// delegation limits of grant.create and grant.revoke belong to the requirement (decideDelegation).
func guards(in *Input) (Decision, error) {
	switch in.Action {
	case ActionTerminalOpen, ActionTerminalAttach, ActionTerminalStream:
		return terminalGuards(in)
	case actionTokenCreate:
		if in.Token == nil {
			return Decision{}, internalError("token.create without the holder")
		}
		return tokenCreateGuard(in, in.Token), nil
	case actionCommandRegister:
		return commandRegisterGuard(in)
	case actionCommandInvoke:
		return commandInvokeGuard(in)
	case actionCommandAutocomplete:
		return commandAutocompleteGuard(in)
	case actionAgentInvoke:
		if in.Invoke == nil {
			return Decision{}, internalError("agent.invoke without its trigger")
		}
		if in.Interaction != nil && in.Interaction.Stage != InteractionStageDeliver {
			return Decision{}, internalError("agent.invoke with interaction facts outside stage deliver")
		}
		return invokeGuard(in, in.Invoke)
	case actionInteractionCreate:
		return interactionGuard(in)
	case actionRunStart:
		if in.Invocation == nil {
			return Decision{}, internalError("run.start without its token facts")
		}
		return runStartGuard(in.Invocation), nil
	}
	return Allow, nil
}

// decideDelegation applies the delegation limits of 07 section 2.6 (CheckCreate for grant.create,
// CheckRevoke for grant.revoke) as part of the requirement step 6 of 07 section 4.2, so a caller
// beyond the limits is refused with the delegation reason before any step-up is asked for (brief
// M1-48 scope 3: "requirement before step-up"). Other actions pass.
func decideDelegation(in *Input) (Decision, bool, error) {
	if in.Action != actionGrantCreate && in.Action != actionGrantRevoke {
		return Decision{}, false, nil
	}
	if in.Grant == nil {
		return Decision{}, true, internalError(in.Action + " without the grant")
	}
	d := Delegator{IsOwner: in.Eff.Owner, Level: in.Eff.Level, Caps: in.Eff.Caps}
	var r Reason
	if in.Action == actionGrantCreate {
		r = CheckCreate(d, in.Grant.Shape)
	} else {
		r = CheckRevoke(d, in.Grant.Shape)
	}
	if r != "" {
		return deny(OutcomeForbidden, r), true, nil
	}
	return Decision{}, false, nil
}

// tokenCreateGuard gives a personal token only to the person itself (brief M1-44): a personal
// token is the person's own credential, so an owner who minted one for someone else could act as
// that person. Tokens for agents and an unknown holder (the use case answers 404) pass.
func tokenCreateGuard(in *Input, f *TokenFacts) Decision {
	if f.HolderFound && f.HolderKind == KindUser && in.Resource.ID != in.Principal.ID {
		return deny(OutcomeForbidden, ReasonNotSelf)
	}
	return Allow
}

// notSelfForPersonalToken is the reason of a refused token.create requirement: not_self when the
// holder is a person other than the caller (08 section 12, brief M1-44), else reason unchanged
// (owner_only for an agent or app holder).
func notSelfForPersonalToken(in *Input, reason Reason) Reason {
	if in.Action == actionTokenCreate && in.Token != nil && in.Token.HolderFound && in.Token.HolderKind == KindUser &&
		in.Resource.ID != in.Principal.ID {
		return ReasonNotSelf
	}
	return reason
}

// invokeGuard applies 05 section 5 points 1 to 5, 7 and 8 in this order (brief M1-28 "Guards").
// With interaction facts (interaction.create, and agent.invoke at stage deliver; brief M2-15
// design note 8 b and c) the mention rule of point 3 counts as met, point 6 (componentRules)
// comes between SR2 and the project rules, and at stage accept run_active closes point 8.
// Interaction facts that cannot be evaluated are an internal error.
func invokeGuard(in *Input, f *InvokeFacts) (Decision, error) {
	ia := in.Interaction
	if ia != nil {
		if err := checkInteraction(ia); err != nil {
			return Decision{}, err
		}
	}
	if r := invokeReason(in, f, ia); r != "" {
		return deny(OutcomeForbidden, r), nil
	}
	return Allow, nil
}

// invokeReason is the reason of the first refusing point of invokeGuard; "" allows.
func invokeReason(in *Input, f *InvokeFacts, ia *InteractionFacts) Reason {
	if r := triggerReason(in, f, ia != nil); r != "" {
		return r
	}
	if ia != nil {
		if r := componentRules(in, ia); r != "" {
			return r
		}
	}
	if r := projectRules(in.Eff.Owner, f); r != "" {
		return r
	}
	return activeRunReason(f, ia)
}

// triggerReason applies points 1 to 5 of 05 section 5: a human trigger, a reachable agent (card
// marks the mention rule met for an interaction), I2 and SR2.
func triggerReason(in *Input, f *InvokeFacts, card bool) Reason {
	switch {
	case in.Principal.Kind != KindUser || f.TriggerOrigin != OriginHuman:
		return ReasonOriginNotHuman
	case !reachable(in.Path, f, card):
		return ReasonAgentNotReachable
	case f.BillingMode != BillingOrganization && f.CredentialHolderID != in.Principal.ID:
		return ReasonBillingModePersonalNotHolder
	case toolClassOwnerOnly(in.Eff.Owner, f):
		return ReasonToolClassOwnerOnly
	}
	return ""
}

// activeRunReason applies point 8: an active run of another agent, and at stage accept a component
// without a run while the agent's run is active, which would need a second run that cannot exist
// (R-DM-14); a follow-up is a message (05 section 5 point 8).
func activeRunReason(f *InvokeFacts, ia *InteractionFacts) Reason {
	if !f.ActiveRunAgentID.IsZero() && f.ActiveRunAgentID != f.AgentID {
		return ReasonRunActiveOtherAgent
	}
	if ia != nil && ia.Stage == InteractionStageAccept && ia.ComponentRunID.IsZero() && f.ActiveRunAgentID == f.AgentID {
		return ReasonRunActive
	}
	return ""
}

// reachable is point 2 (no agents in dms) and point 3: the agent is active and either operates
// the thread with the mention rule fulfilled, or is an allowed guest. card marks a trigger by a
// component of the agent's own card: the card is the agent's address to the person, so it counts
// as a mention (brief M2-15 design note 8 b); a guest still needs allow_mentioned_agents, its own
// write and billing mode organization.
func reachable(path Path, f *InvokeFacts, card bool) bool {
	if path.DM || path.Kind != PathChannel || f.AgentID.IsZero() {
		return false
	}
	if !f.AgentFound || f.AgentKind != KindAgent || f.AgentStatus != "active" || !f.PolicyFound {
		return false
	}
	mentioned := f.MentionsAgent || card
	mainThread := path.ThreadID.IsZero() || path.ThreadMain || f.NewThread
	mentionRule := f.FreeResponse || mentioned || (mainThread && !f.RequireMention) || (!mainThread && !f.ThreadRequireMention)
	if path.ThreadOperator() == f.AgentID && mentionRule {
		return true
	}
	return mentioned && f.AllowMentionedAgents && f.AgentLevel >= LevelWrite && f.BillingMode == BillingOrganization
}

// toolClassOwnerOnly is SR2 (point 5): an agent of tool class host, configured or reported, is
// triggered only by an effective owner or a person with a ready own dev environment.
func toolClassOwnerOnly(owner bool, f *InvokeFacts) bool {
	return max(toolRank(f.ToolClass), toolRank(f.ReportedToolClass)) >= toolRank(ToolHost) && !owner && !f.DevEnvironmentReady
}

// projectRules are the project binding (point 7) and SR3 rule 1; owner is the effective owner role
// of the principal. They follow SR2 and, with interaction facts, point 6.
func projectRules(owner bool, f *InvokeFacts) Reason {
	switch {
	case f.ProjectBound && !f.ProjectReady:
		return ReasonProjectNotReady
	case f.ProjectBound && !owner && !f.DevEnvironmentReady:
		return ReasonDevEnvironmentNotReady
	}
	return ""
}

// terminalGuards are the guards of the terminal actions: SR10, SR11 and SR12 and then the live
// guards of M2-44 (host_not_active, terminal_profile_unknown) for terminal.open and
// terminal.attach; for terminal.stream only the device of the current session token passes
// (not_self). Missing facts are internal errors.
func terminalGuards(in *Input) (Decision, error) {
	if in.Action == ActionTerminalStream {
		if in.Live == nil {
			return Decision{}, internalError(in.Action + " without its terminal facts")
		}
		if in.Live.SessionDeviceID.IsZero() || in.Live.SessionDeviceID != in.Principal.DeviceID {
			return deny(OutcomeForbidden, ReasonNotSelf), nil
		}
		return Allow, nil
	}
	if in.Terminal == nil {
		return Decision{}, internalError(in.Action + " without its terminal facts")
	}
	if r, ok := TerminalGuard(*in.Terminal); !ok {
		return deny(OutcomeForbidden, r), nil
	}
	if r, ok := TerminalLiveGuard(in.Action, in.Terminal.Host, in.Live); !ok {
		return deny(OutcomeForbidden, r), nil
	}
	return Allow, nil
}
