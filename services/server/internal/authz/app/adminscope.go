// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// AdminScope is the view of a caller that the list use cases of groups and grants filter with
// (M1-48): the effective owner role and the spaces and channels on which the caller holds manage
// (07 section 2.6 item 6; a channel holds it through its space or the instance as well).
type AdminScope struct {
	// PrincipalID is the caller.
	PrincipalID ids.ID
	// Owner reports the effective owner role: an active owner in a session (SR5).
	Owner bool
	// ManageSpaces and ManageChannels are the targets with effective manage, ascending; empty
	// for an owner, who needs no list.
	ManageSpaces, ManageChannels []ids.ID
}

// AdminScoper gives the list use cases of the Admin port the AdminScope of a caller. The
// Authorizer implements it; it is not part of the kernel port, because only authz lists grants
// and groups.
type AdminScoper interface {
	// AdminScope returns the scope of p for the list action act (group.read or grant.read). It
	// runs the liveness and pending checks of Authorize and records one check of act in the
	// request check log; a caller who fails them gets the error of the decision.
	AdminScope(ctx context.Context, p authorize.Principal, act authorize.Action) (AdminScope, error)
}

// Compile-time check: the Authorizer serves the list use cases.
var _ AdminScoper = (*Authorizer)(nil)

// AdminScope implements AdminScoper with the snapshot of Authorize (07 section 7.2), built per
// request from the current structure, so a new space or channel counts at once. Allowed means
// only that the caller passed the liveness checks; what the caller sees is the scope itself.
func (a *Authorizer) AdminScope(ctx context.Context, p authorize.Principal, act authorize.Action) (AdminScope, error) {
	sc, d, err := a.adminScope(ctx, p, act)
	allowed := err == nil && d.Outcome == domain.OutcomeAllow
	authorize.RecordCheck(ctx, authorize.Check{Action: string(act), Kind: authorize.CheckScope, Allowed: allowed})
	a.audit(ctx, act, "", ids.ID{}, nil, outcome(d), err)
	if err == nil && !allowed {
		return AdminScope{}, outcome(d).Err()
	}
	return sc, err
}

// adminScope is AdminScope without the check log and the audit.
func (a *Authorizer) adminScope(ctx context.Context, p authorize.Principal, act authorize.Action) (AdminScope, domain.Decision, error) {
	if act != authorize.ActionGroupRead && act != authorize.ActionGrantRead {
		return AdminScope{}, domain.Decision{}, internal("AdminScope serves group.read and grant.read, not " + string(act))
	}
	spec, _, err := a.requirement(act)
	if err != nil {
		return AdminScope{}, domain.Decision{}, err
	}
	if p == nil || p.PrincipalKind() == domain.KindAnonymous {
		return AdminScope{}, domain.Decision{Outcome: domain.OutcomeUnauthenticated, Reason: domain.ReasonUnauthenticated}, nil
	}
	t := a.clock.Now()
	v, err := a.version(ctx)
	if err != nil {
		return AdminScope{}, domain.Decision{}, err
	}
	s, d, err := a.principalOf(ctx, p, spec.ID, v, t)
	if err != nil || d.Outcome != domain.OutcomeAllow {
		return AdminScope{}, d, err
	}
	if s.kind == domain.KindApp {
		// The app scope rule of Authorize (05 section 2.6): group.read and grant.read are not open
		// to apps.
		if r := domain.CheckAppScope(spec.ID, p.AppGrantedScopes(), p.AppTokenScopes(), !s.ref.TokenID.IsZero()); r != "" {
			return AdminScope{}, domain.Decision{Outcome: domain.OutcomeForbidden, Reason: r}, nil
		}
	}
	sc, err := a.manageScope(ctx, s)
	if err != nil {
		return AdminScope{}, domain.Decision{}, err
	}
	return sc, domain.Allow, nil
}

// manageScope is the scope of s: the owner role, or the spaces and channels on which s holds
// manage, read from the current structure.
func (a *Authorizer) manageScope(ctx context.Context, s *actingPrincipal) (AdminScope, error) {
	sc := AdminScope{PrincipalID: s.id}
	if inst, _ := s.rights(domain.Path{Kind: domain.PathInstance}); inst.Owner {
		sc.Owner = true
		return sc, nil
	}
	spaces, err := a.structure.Spaces(ctx)
	if err != nil {
		return AdminScope{}, sourceError("structure source", err)
	}
	for _, id := range spaces {
		if f, _ := s.rights(domain.Path{Kind: domain.PathSpace, SpaceID: id}); f.Caps.Has(domain.CapManage) {
			sc.ManageSpaces = append(sc.ManageSpaces, id)
		}
	}
	chans, err := a.structure.Channels(ctx)
	if err != nil {
		return AdminScope{}, sourceError("structure source", err)
	}
	for _, c := range chans {
		if f, _ := s.rights(channelPath(c)); f.Caps.Has(domain.CapManage) {
			sc.ManageChannels = append(sc.ManageChannels, c.ID)
		}
	}
	sc.ManageSpaces, sc.ManageChannels = domain.SortIDs(sc.ManageSpaces), domain.SortIDs(sc.ManageChannels)
	return sc, nil
}
