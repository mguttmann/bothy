// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// This file declares the guided flow "switch first, then grant" (docs/spec/05-aktionen.md rule
// 1.6, R-AZ-17; docs/spec/10-backend-richtlinie.md section 11; M2-21): the request member
// resolve_i1 of grant.create, group.member.add, owner.add, space.operator.set and
// channel.operator.set switches the listed agents to organization in the same transaction as the
// change, before its first write, so I1 never breaks in between.

// ResolveI1 is the request member resolve_i1 (OpenAPI ResolveI1): the agents to switch to
// organization first and the reason of the switch. AgentIDs keep the request order, so a field
// error can name the index of an entry; the resolver authorizes and switches them in ascending id.
type ResolveI1 struct {
	// AgentIDs is switch_to_organization in request order.
	AgentIDs []ids.ID
	// Reason is the mandatory reason of the switch, the only free text in its audit details
	// (SEC-AUD-5).
	Reason string
}

// I1Resolver runs the switch part of the guided flow. authz/app implements it; identity and chat
// get it through their constructors (internal/wire) and call it, while authz calls it in
// createGrant and addGroupMember itself.
//
// Example (identity, inside the unit of work of owner.add, before the owners row is inserted):
//
//	if req != nil {
//		if err := resolver.ResolveI1(ctx, a, authz.ResolveI1{AgentIDs: req.AgentIDs, Reason: req.Reason}); err != nil {
//			return err
//		}
//	}
type I1Resolver interface {
	// CheckResolveI1 validates req without any read (05 rule 1.6, SEC-IN-4, SEC-AUD-5): at least
	// one agent, no repeated id, a reason that is not blank, without control or bidi characters
	// and at most audit.reason_max_chars characters. Errors: validation_failed with pointers below
	// /resolve_i1; the caller joins them with its own field errors (06 section 5 point 3).
	CheckResolveI1(req ResolveI1) error
	// ResolveI1 runs inside the unit of work of the outer action, which declared its rights change
	// (02 section 8.1), after the outer checks and before the outer action's first write. It
	// validates req, authorizes agent.billing_mode.set for every listed agent in ascending id
	// (the first refusal decides; an unknown id, a principal that is not an agent and an invisible
	// agent are the same 404 not_found), names the agents as affected and switches them with
	// AgentPolicySwitch. Errors: validation_failed, not_found, forbidden, step_up_required and
	// those of the switch; every error must roll back the whole unit (BR-07).
	ResolveI1(ctx context.Context, by actor.Actor, req ResolveI1) error
}

// AgentPolicySwitch is the source port through which authz switches agents to organization (02
// section 4.1 rule 4): the agents module implements it (agents/adapters/authzsource), because it
// owns the table agents, the event agent.updated and the audit entry of agent.billing_mode.set.
type AgentPolicySwitch interface {
	// SwitchToOrganization switches the agents of agentIDs (ascending, without duplicates) to
	// organization inside the ambient transaction of ctx, with reason in the audit entry of each
	// agent. It authorizes nothing: only authz calls it, after it authorized every agent. Errors:
	// not_found for an id without an agents row; store errors.
	SwitchToOrganization(ctx context.Context, by actor.Actor, agentIDs []ids.ID, reason string) error
}

// EmptyAgentPolicySwitch is the AgentPolicySwitch before internal/wire binds the agents module
// (02 section 4.4, last paragraph): it fails closed, so a guided flow never commits a change
// without its switch. M2-21 wires the real implementation of the agents module.
type EmptyAgentPolicySwitch struct{}

// Compile-time check: the empty switch is the port.
var _ AgentPolicySwitch = EmptyAgentPolicySwitch{}

// SwitchToOrganization implements AgentPolicySwitch: always an internal error.
func (EmptyAgentPolicySwitch) SwitchToOrganization(context.Context, actor.Actor, []ids.ID, string) error {
	return errs.New(errs.CodeInternalError, "authz: no agent policy switch is bound")
}
