// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The scope kinds of a command (03 commands).
const (
	commandScopeInstance = "instance"
	commandScopeSpace    = "space"
	commandScopeChannel  = "channel"
)

// resolveCommand resolves the path of a command resource (brief M2-16 design note 8 b): the
// channel of a list item (OfferedInChannelID), else the scope target of the facts (the instance,
// the space or the channel), else, for the receive check of command.changed without facts, the
// scope of the event. An unknown space or channel is not found.
func (a *Authorizer) resolveCommand(ctx context.Context, r authorize.Resource) (resolved, bool, error) {
	c := r.Command
	switch {
	case c == nil && (!r.ThreadID.IsZero() || !r.ChannelID.IsZero()):
		return a.resolveChannel(ctx, r.ThreadID, r.ChannelID, ids.ID{})
	case c == nil && !r.SpaceID.IsZero():
		return a.resolveSpace(ctx, r.SpaceID)
	case c == nil:
		return resolved{path: domain.Path{Kind: domain.PathInstance}}, true, nil
	case !c.OfferedInChannelID.IsZero():
		return a.resolveChannel(ctx, ids.ID{}, c.OfferedInChannelID, ids.ID{})
	}
	switch c.ScopeKind {
	case commandScopeInstance:
		return resolved{path: domain.Path{Kind: domain.PathInstance}}, true, nil
	case commandScopeSpace:
		if c.ScopeSpaceID.IsZero() {
			return resolved{}, false, internal("command scoped to a space without the space")
		}
		return a.resolveSpace(ctx, c.ScopeSpaceID)
	case commandScopeChannel:
		if c.ScopeChannelID.IsZero() {
			return resolved{}, false, internal("command scoped to a channel without the channel")
		}
		return a.resolveChannel(ctx, ids.ID{}, c.ScopeChannelID, ids.ID{})
	}
	return resolved{}, false, internal("command with an unknown scope kind")
}

// checkCommandFacts refuses, in request mode, a command resource and command.invoke without the
// command facts (brief M2-16 design note 8 b): only the filter-mode receive check of
// command.changed takes the path from the event scope, so no request widens access by leaving
// the facts out.
func checkCommandFacts(ctx context.Context, act authorize.Action, r authorize.Resource) error {
	if r.Command != nil || authorize.IsFilter(ctx) && r.Event != nil {
		return nil
	}
	if act == authorize.ActionCommandAutocompleteRespond && r.Kind == authorize.ResourceCommand && !r.ChannelID.IsZero() {
		// The answer of an autocomplete request (M2-17 design note 3) is decided on the place of
		// the request, which the resource names as its path; the owner is OwnerID.
		return nil
	}
	if r.Kind == authorize.ResourceCommand || act == authorize.ActionCommandInvoke || act == authorize.ActionCommandAutocomplete {
		return internal("command resource without its facts in request mode")
	}
	return nil
}

// commandFacts gathers the facts of the command rules (brief M2-16 design note 8): the stored or
// submitted definition, for command.register on a space whether the principal is its default
// operator, and for an item of a channel list the owner's status, its own effective level on the
// channel from its snapshot without session (the mechanism of the agent.invoke guest rule), its
// billing mode and the channel's allow_mentioned_agents. nil without facts.
func (a *Authorizer) commandFacts(ctx context.Context, s *actingPrincipal, spec domain.ActionSpec, r authorize.Resource, res resolved,
	t time.Time) (*domain.CommandFacts, error) {
	f := r.Command
	if f == nil {
		return nil, nil
	}
	owner := r.OwnerID
	if !f.OwnerID.IsZero() {
		owner = f.OwnerID
	}
	out := &domain.CommandFacts{
		OwnerID: owner, OwnerKind: f.OwnerKind, ScopeKind: f.ScopeKind, ScopeSpaceID: f.ScopeSpaceID, ScopeChannelID: f.ScopeChannelID,
		Offered: !f.OfferedInChannelID.IsZero(), RequiredLevel: f.RequiredLevel, Control: f.Control, ActiveRun: f.ActiveRun,
	}
	if spec.ID == string(authorize.ActionCommandRegister) && f.ScopeKind == commandScopeSpace {
		details, err := a.structure.SpaceDetails(ctx, []ids.ID{f.ScopeSpaceID})
		if err != nil {
			return nil, sourceError("structure source", err)
		}
		d, ok := details[f.ScopeSpaceID]
		out.SpaceDefaultOperator = ok && !d.DefaultOperatorID.IsZero() && d.DefaultOperatorID == s.id
	}
	if !out.Offered || res.path.Kind != domain.PathChannel || owner.IsZero() {
		return out, nil
	}
	if err := a.ownerOffer(ctx, s, owner, res, t, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ownerOffer fills the owner facts of a list item: active status, the owner's own effective
// level on the channel and, for an agent, its billing mode.
func (a *Authorizer) ownerOffer(ctx context.Context, s *actingPrincipal, owner ids.ID, res resolved, t time.Time, out *domain.CommandFacts) error {
	out.AllowMentionedAgents = res.channel.AllowMentionedAgents
	pf, found, err := a.principal.Principal(ctx, owner)
	if err != nil {
		return sourceError("principal source", err)
	}
	kind := string(pf.Kind)
	if !found || kind != out.OwnerKind || (kind != domain.KindAgent && kind != domain.KindApp) {
		return nil
	}
	out.OwnerActive = pf.Status == authz.PrincipalActive
	if !out.OwnerActive {
		return nil
	}
	oe, err := a.load(ctx, authz.CredentialRef{PrincipalID: owner}, s.entry.snap.PermissionVersion, t)
	if err != nil {
		return err
	}
	out.OwnerLevel = domain.Effective(oe.snap, kind, owner, res.path).Level
	if kind != domain.KindAgent {
		return nil
	}
	pol, ok, err := a.policies.AgentPolicy(ctx, owner)
	if err != nil {
		return sourceError("agent policy source", err)
	}
	if ok {
		out.OwnerBillingMode = pol.BillingMode
	}
	return nil
}
