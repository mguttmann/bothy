// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the Authorizer answers the allow list of an agent adapter.
var _ authorize.AgentTriggerers = (*Authorizer)(nil)

// Triggerers implements authorize.AgentTriggerers (agent.triggerers.read, 09 17.7). The rights
// version is read first, so the set read after it holds every change up to that version. T(A)
// comes from domain.Triggerers over the trigger world of the I1 check and of access.explain
// (triggerWorld), so all three see the same people; the guest channels of an organization agent
// join its served targets, and a personal agent keeps only its credential holder. Every read is
// fresh, never from the snapshot cache.
func (a *Authorizer) Triggerers(ctx context.Context, agent ids.ID) (authorize.TriggererSet, error) {
	v, err := a.version(ctx)
	if err != nil {
		return authorize.TriggererSet{}, err
	}
	people, err := a.triggerers(ctx, agent)
	if err != nil {
		return authorize.TriggererSet{}, err
	}
	return authorize.TriggererSet{PrincipalIDs: people, PermissionVersion: v}, nil
}

// triggerers returns the set of Triggerers, never nil.
func (a *Authorizer) triggerers(ctx context.Context, agent ids.ID) ([]ids.ID, error) {
	x := &explainer{a: a, t: a.clock.Now(), states: map[ids.ID]*principalState{}}
	st, err := x.state(ctx, agent)
	if err != nil {
		return nil, err
	}
	if !st.found || string(st.facts.Kind) != domain.KindAgent || st.facts.Status != authz.PrincipalActive {
		return []ids.ID{}, nil
	}
	pol, ok, err := a.policies.AgentPolicy(ctx, agent)
	if err != nil {
		return nil, sourceError("agent policy source", err)
	}
	if !ok {
		return []ids.ID{}, nil
	}
	served, err := servedOf(ctx, a.structure, agent)
	if err != nil {
		return nil, err
	}
	if pol.BillingMode == domain.BillingOrganization {
		guests, err := a.guestTargets(ctx, st.snap, agent)
		if err != nil {
			return nil, err
		}
		served = append(served, guests...)
	}
	owners, err := a.principal.ActiveOwnerIDs(ctx)
	if err != nil {
		return nil, sourceError("principal source", err)
	}
	world, err := triggerWorld(ctx, a.store, a.principal, owners)
	if err != nil {
		return nil, err
	}
	world.Served[agent] = served
	set := domain.Triggerers(world, agent, domain.EffectiveToolClass(pol.ToolClass, pol.ReportedToolClass), x.t)
	if pol.BillingMode == domain.BillingPersonal {
		set = slices.DeleteFunc(set, func(p ids.ID) bool { return p != pol.CredentialHolderID })
	}
	if set == nil {
		set = []ids.ID{}
	}
	return set, nil
}

// guestTargets returns the guest channels of agent as served targets (05 section 5 point 3): the
// channels that are not dm channels, not archived and not read-only, with allow_mentioned_agents,
// on which the snapshot snap of the agent reaches at least write.
func (a *Authorizer) guestTargets(ctx context.Context, snap domain.Snapshot, agent ids.ID) ([]domain.ServedTarget, error) {
	chans, err := a.structure.Channels(ctx)
	if err != nil {
		return nil, sourceError("structure source", err)
	}
	var out []domain.ServedTarget
	for _, c := range chans {
		if c.Kind == authz.ChannelDM || !c.AllowMentionedAgents || c.Archived || c.ReadOnly {
			continue
		}
		if domain.Effective(snap, domain.KindAgent, agent, channelPath(c)).Level >= domain.LevelWrite {
			out = append(out, domain.ServedTarget{ChannelID: c.ID, SpaceID: c.SpaceID})
		}
	}
	return out, nil
}
