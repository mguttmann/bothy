// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// appOwnerOf returns the app that owns the command or the card of r (brief M2-25 design note 3 a):
// CommandFacts.OwnerID (or Resource.OwnerID) of an app command, Resource.OwnerID (the card's
// author) of an app card; zero when the owner is not an app.
func appOwnerOf(r authorize.Resource) ids.ID {
	if c := r.Command; c != nil && c.OwnerKind == domain.KindApp {
		if !c.OwnerID.IsZero() {
			return c.OwnerID
		}
		return r.OwnerID
	}
	if f := r.Interaction; f != nil && f.OwnerKind == domain.KindApp {
		return r.OwnerID
	}
	return ids.ID{}
}

// appOwnerFacts gathers the facts of the app-owner guard (design note 3 a): the installation state
// and the granted scopes of the app from the principal source by its ID, and, where a channel is
// resolved for the check, the app's own effective level there from its snapshot (07 sections 2.1
// and 2.10). command.read without a list channel (getCommand) resolves no channel for the guard
// (design note 3 b). nil when r has no app owner.
func (a *Authorizer) appOwnerFacts(ctx context.Context, s *actingPrincipal, spec domain.ActionSpec, r authorize.Resource, res resolved,
	t time.Time) (*domain.AppOwnerFacts, error) {
	owner := appOwnerOf(r)
	if owner.IsZero() {
		return nil, nil
	}
	out := &domain.AppOwnerFacts{ID: owner}
	pf, found, err := a.principal.Principal(ctx, owner)
	if err != nil {
		return nil, sourceError("principal source", err)
	}
	if !found || string(pf.Kind) != domain.KindApp {
		return out, nil
	}
	out.Active = pf.Status == authz.PrincipalActive
	out.InstallationActive = pf.AppInstallationActive
	out.GrantedScopes = slices.Clone(pf.AppGrantedScopes)
	offered := r.Command != nil && !r.Command.OfferedInChannelID.IsZero()
	if res.path.Kind != domain.PathChannel || (spec.ID == string(authorize.ActionCommandRead) && !offered) {
		return out, nil
	}
	out.OnChannel, out.DM = true, res.path.DM
	if !out.Active || !out.InstallationActive {
		return out, nil
	}
	oe, err := a.load(ctx, authz.CredentialRef{PrincipalID: owner}, s.entry.snap.PermissionVersion, t)
	if err != nil {
		return nil, err
	}
	// The archive and read-only rules hold for the person in their own step (05 rule 1.8, 07
	// section 4.5); the guard asks only whether the app was granted write on the place, so the
	// cap of an archived channel does not hide the command before the person learns
	// channel_archived.
	path := res.path
	path.Archived, path.ReadOnly = false, false
	out.Level = domain.Effective(oe.snap, domain.KindApp, owner, path).Level
	return out, nil
}

// authorizeAppReceive decides agent.events.consume for a principal of kind app in filter mode (brief
// M2-25 design note 4): the liveness of the app, then domain.AppReceive with the audience of the
// event (the agent resource of the audience, or the resource's OwnerID) and the installation facts
// read fresh from the principal source.
func (a *Authorizer) authorizeAppReceive(ctx context.Context, p authorize.Principal, r authorize.Resource) (authorize.Decision, error) {
	spec, _, err := a.requirement(authorize.ActionAgentEventsConsume)
	if err != nil {
		return authorize.Decision{}, err
	}
	t := a.clock.Now()
	s, d, err := a.acting(ctx, p, spec.ID, t)
	if err != nil || d.Outcome != domain.OutcomeAllow {
		return outcome(d), err
	}
	audience := r.OwnerID
	if r.Kind == authorize.ResourceAgent || r.Kind == authorize.ResourceApp {
		audience = r.ID
	}
	active := s.entry.found && string(s.entry.facts.Kind) == domain.KindApp && s.entry.facts.AppInstallationActive
	return outcome(domain.AppReceive(s.id, active, s.entry.facts.AppGrantedScopes, audience)), nil
}
