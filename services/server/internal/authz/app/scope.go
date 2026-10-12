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

// scopeMemoKey keys the lists of Scope in the request memo: the principal with its credentials,
// "view as", acting for, the version and the app scope of the action.
type scopeMemoKey struct {
	key               cacheKey
	viewAs, actingFor ids.ID
	scope             string
}

// Scope implements authorize.Authorizer (07 sections 7.2 and 7.4). The lists are built per request
// from the snapshot and the current structure and memoised only in the request scope, so a channel
// or space created without a new rights version appears at once. It records (a, allow) in the
// check log; a principal that fails the liveness or pending checks gets the error of its decision,
// which is audited like a denial of Authorize.
func (a *Authorizer) Scope(ctx context.Context, p authorize.Principal, act authorize.Action) (authorize.Scope, error) {
	sc, d, err := a.scope(ctx, p, act)
	allowed := err == nil && d.Outcome == domain.OutcomeAllow
	authorize.RecordCheck(ctx, authorize.Check{Action: string(act), Kind: authorize.CheckScope, Allowed: allowed})
	a.audit(ctx, act, "", ids.ID{}, outcome(d), err)
	if err == nil && !allowed {
		return authorize.Scope{}, outcome(d).Err()
	}
	return sc, err
}

// scope is Scope without the check log and the audit.
func (a *Authorizer) scope(ctx context.Context, p authorize.Principal, act authorize.Action) (authorize.Scope, domain.Decision, error) {
	spec, err := a.listAction(act)
	if err != nil {
		return authorize.Scope{}, domain.Decision{}, err
	}
	if p == nil || p.PrincipalKind() == domain.KindAnonymous {
		return authorize.Scope{}, domain.Decision{Outcome: domain.OutcomeUnauthenticated, Reason: domain.ReasonUnauthenticated}, nil
	}
	t := a.clock.Now()
	v, err := a.version(ctx)
	if err != nil {
		return authorize.Scope{}, domain.Decision{}, err
	}
	s, d, err := a.principalOf(ctx, p, spec.ID, v, t)
	if err != nil || d.Outcome != domain.OutcomeAllow {
		return authorize.Scope{}, d, err
	}
	appScope := ""
	if s.kind == domain.KindApp {
		appScope, _ = domain.AppScopeOf(spec.ID)
	}
	mk := scopeMemoKey{key: s.entry.key, viewAs: p.ViewAsPerson(), actingFor: p.ActingForPerson(), scope: appScope}
	memo, hasMemo := authorize.RequestMemoFrom(ctx)
	if sc, ok := memoScope(memo, mk); ok {
		return sc, domain.Allow, nil
	}
	sc := authorize.Scope{PermissionVersion: v}
	if s.kind != domain.KindApp || domain.CheckAppScope(spec.ID, p.AppGrantedScopes()) == "" {
		if sc, err = a.buildScope(ctx, s, v); err != nil {
			return authorize.Scope{}, domain.Decision{}, err
		}
	}
	if hasMemo {
		memo.Store(mk, cloneScope(sc))
	}
	return sc, domain.Allow, nil
}

// memoScope returns a copy of the scope stored under k in memo, which may be nil.
func memoScope(memo *authorize.RequestMemo, k scopeMemoKey) (authorize.Scope, bool) {
	if memo == nil {
		return authorize.Scope{}, false
	}
	cached, ok := memo.Load(k)
	if !ok {
		return authorize.Scope{}, false
	}
	sc, ok := cached.(authorize.Scope)
	return cloneScope(sc), ok
}

// listAction returns the spec of act when Scope may serve it: a requirement of exactly read on
// channel-path kinds or spaces (07 section 7.4); any other action is an internal error.
func (a *Authorizer) listAction(act authorize.Action) (domain.ActionSpec, error) {
	spec, req, err := a.requirement(act)
	if err != nil {
		return domain.ActionSpec{}, err
	}
	ok := req.IsReadLevel() && len(spec.Resources) > 0
	for _, k := range spec.Resources {
		ok = ok && (domain.IsChannelPathKind(k) || k == string(authorize.ResourceSpace))
	}
	if !ok {
		return domain.ActionSpec{}, internal("Scope needs a list action with read on a channel path or space.read, not " + spec.ID)
	}
	return spec, nil
}

// buildScope builds the visible sets of s from the current structure (07 section 7.4 rule 1).
func (a *Authorizer) buildScope(ctx context.Context, s *actingPrincipal, v int64) (authorize.Scope, error) {
	sc := authorize.Scope{PermissionVersion: v}
	spaces := map[ids.ID]bool{}
	var err error
	if sc.ChannelIDs, err = a.visibleChannels(ctx, s, spaces); err != nil {
		return authorize.Scope{}, err
	}
	all, err := a.structure.Spaces(ctx)
	if err != nil {
		return authorize.Scope{}, sourceError("structure source", err)
	}
	for _, id := range all {
		if f, _ := s.rights(domain.Path{Kind: domain.PathSpace, SpaceID: id}); f.Level >= domain.LevelRead {
			sc.FullyVisibleSpaceIDs = append(sc.FullyVisibleSpaceIDs, id)
			spaces[id] = true
		}
	}
	for id := range spaces {
		sc.SpaceIDs = append(sc.SpaceIDs, id)
	}
	inst, _ := s.rights(domain.Path{Kind: domain.PathInstance})
	sc.InstanceWide = inst.Level >= domain.LevelRead
	sc.SpaceIDs = domain.SortIDs(sc.SpaceIDs)
	sc.FullyVisibleSpaceIDs = domain.SortIDs(sc.FullyVisibleSpaceIDs)
	return sc, nil
}

// visibleChannels returns the channels s reads, ascending: every non-dm channel with read on its
// path and, for people, the dm channels of participation. It adds the spaces of the channels to
// spaces.
func (a *Authorizer) visibleChannels(ctx context.Context, s *actingPrincipal, spaces map[ids.ID]bool) ([]ids.ID, error) {
	chans, err := a.structure.Channels(ctx)
	if err != nil {
		return nil, sourceError("structure source", err)
	}
	if s.kind == domain.KindUser {
		dms, err := a.dmChannels(ctx, s.id)
		if err != nil {
			return nil, err
		}
		chans = append(chans, dms...)
	}
	var out []ids.ID
	for _, c := range chans {
		if f, _ := s.rights(channelPath(c)); f.Level >= domain.LevelRead {
			out = append(out, c.ID)
			if !c.SpaceID.IsZero() {
				spaces[c.SpaceID] = true
			}
		}
	}
	return slices.Compact(domain.SortIDs(out)), nil
}

// dmChannels returns the facts of the dm channels user takes part in.
func (a *Authorizer) dmChannels(ctx context.Context, user ids.ID) ([]authz.ChannelFacts, error) {
	list, err := a.structure.DMChannelsOf(ctx, user)
	if err != nil {
		return nil, sourceError("structure source", err)
	}
	var out []authz.ChannelFacts
	for _, id := range list {
		c, ok, err := a.structure.Channel(ctx, id)
		if err != nil {
			return nil, sourceError("structure source", err)
		}
		if ok && c.Kind == authz.ChannelDM {
			out = append(out, c)
		}
	}
	return out, nil
}

// cloneScope copies the lists, so a caller never changes the memo.
func cloneScope(sc authorize.Scope) authorize.Scope {
	sc.ChannelIDs = slices.Clone(sc.ChannelIDs)
	sc.SpaceIDs = slices.Clone(sc.SpaceIDs)
	sc.FullyVisibleSpaceIDs = slices.Clone(sc.FullyVisibleSpaceIDs)
	return sc
}
