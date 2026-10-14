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

// guards applies the guards of the action (07 section 4.2 step 9): agent.invoke, token.create and
// the terminal guards SR10, SR11 and SR12 of terminal.open and terminal.attach (M2-40). The
// delegation limits of grant.create and grant.revoke belong to the requirement (decideDelegation).
func guards(in *Input) (Decision, error) {
	switch in.Action {
	case ActionTerminalOpen, ActionTerminalAttach:
		if in.Terminal == nil {
			return Decision{}, internalError(in.Action + " without its terminal facts")
		}
		if r, ok := TerminalGuard(*in.Terminal); !ok {
			return deny(OutcomeForbidden, r), nil
		}
		return Allow, nil
	case actionTokenCreate:
		if in.Token == nil {
			return Decision{}, internalError("token.create without the holder")
		}
		return tokenCreateGuard(in, in.Token), nil
	case actionAgentInvoke:
		if in.Invoke == nil {
			return Decision{}, internalError("agent.invoke without its trigger")
		}
		return invokeGuard(in, in.Invoke), nil
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
func invokeGuard(in *Input, f *InvokeFacts) Decision {
	if in.Principal.Kind != KindUser || f.TriggerOrigin != OriginHuman {
		return deny(OutcomeForbidden, ReasonOriginNotHuman)
	}
	if !reachable(in.Path, f) {
		return deny(OutcomeForbidden, ReasonAgentNotReachable)
	}
	if f.BillingMode != BillingOrganization && f.CredentialHolderID != in.Principal.ID {
		return deny(OutcomeForbidden, ReasonBillingModePersonalNotHolder)
	}
	if r := devRules(in.Eff.Owner, f); r != "" {
		return deny(OutcomeForbidden, r)
	}
	if !f.ActiveRunAgentID.IsZero() && f.ActiveRunAgentID != f.AgentID {
		return deny(OutcomeForbidden, ReasonRunActiveOtherAgent)
	}
	return Allow
}

// reachable is point 2 (no agents in dms) and point 3: the agent is active and either operates
// the thread with the mention rule fulfilled, or is an allowed guest.
func reachable(path Path, f *InvokeFacts) bool {
	if path.DM || path.Kind != PathChannel || f.AgentID.IsZero() {
		return false
	}
	if !f.AgentFound || f.AgentKind != KindAgent || f.AgentStatus != "active" || !f.PolicyFound {
		return false
	}
	mainThread := path.ThreadID.IsZero() || path.ThreadMain
	mentionRule := f.FreeResponse || f.MentionsAgent || (mainThread && !f.RequireMention) || (!mainThread && !f.ThreadRequireMention)
	if path.ThreadOperator() == f.AgentID && mentionRule {
		return true
	}
	return f.MentionsAgent && f.AllowMentionedAgents && f.AgentLevel >= LevelWrite && f.BillingMode == BillingOrganization
}

// devRules are SR2 (point 5), the project binding (point 7) and SR3 rule 1; owner is the
// effective owner role of the principal.
func devRules(owner bool, f *InvokeFacts) Reason {
	switch {
	case max(toolRank(f.ToolClass), toolRank(f.ReportedToolClass)) >= toolRank(ToolHost) && !owner && !f.DevEnvironmentReady:
		return ReasonToolClassOwnerOnly
	case f.ProjectBound && !f.ProjectReady:
		return ReasonProjectNotReady
	case f.ProjectBound && !owner && !f.DevEnvironmentReady:
		return ReasonDevEnvironmentNotReady
	}
	return ""
}
