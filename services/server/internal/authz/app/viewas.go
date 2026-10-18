// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The audit attributes of access.view_as (brief M1-49): ids and times, never names.
const attrTargetUserID = "target_user_id"

// ViewAs implements authz.Admin (access.view_as, 07 section 9.2, brief M1-49). authorize decides
// access.view_as on the person (owner in a session, step-up; Authorize audits every denial); an
// unknown id or a principal that is not a person is 404. The view starts at the request time:
// the caller gets actor.ViewAs, and Scope and the rights of authorize with that context give the
// visible spaces and channels, so the rights are the intersection of owner and person (07 section
// 4.2 step 4). Direct messages (SR6) and archived structure never appear. The allow entry with
// target_user_id and expires_at is written in a unit of work. In M1 the view lives only in this
// request.
func (s *AdminService) ViewAs(ctx context.Context, caller actor.Actor, userID ids.ID) (authz.ViewAsSnapshot, error) {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return authz.ViewAsSnapshot{}, err
	}
	eng, err := s.engine()
	if err != nil {
		return authz.ViewAsSnapshot{}, err
	}
	res := authorize.Resource{Kind: authorize.ResourceUser, ID: userID, OwnerID: userID}
	if err := s.check(ctx, caller, authorize.ActionAccessViewAs, res); err != nil {
		return authz.ViewAsSnapshot{}, err
	}
	profiles, err := eng.principal.Profiles(ctx, []ids.ID{userID})
	if err != nil {
		return authz.ViewAsSnapshot{}, sourceError("principal source", err)
	}
	person, ok := profiles[userID]
	if !ok || person.Kind != actor.KindUser {
		return authz.ViewAsSnapshot{}, errNotFound()
	}
	start := s.d.Clock.Now()
	as := caller
	as.ViewAs = &actor.ViewAs{UserID: userID, StartedAt: start}
	out := authz.ViewAsSnapshot{User: person.PrincipalRef, ExpiresAt: start.Add(eng.cfg.ViewAsMaxDuration),
		Spaces: []authz.SpaceView{}, Channels: []authz.ChannelView{}}
	if out.Spaces, out.Channels, err = viewOfPerson(ctx, eng, as); err != nil {
		return authz.ViewAsSnapshot{}, err
	}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		return s.record(ctx, authorize.ActionAccessViewAs, authorize.ResourceUser, userID, auditing.Attrs{
			attrTargetUserID: auditing.ID(userID), attrExpiresAt: auditing.Time(out.ExpiresAt),
		})
	})
	if err != nil {
		return authz.ViewAsSnapshot{}, err
	}
	return out, nil
}

// viewOfPerson returns the spaces and channels as sees them (as carries actor.ViewAs): the sets of
// Scope for message.read and space.read, without dm channels and archived structure, each with the
// rights of authorize on its path. A space visible only through one of its channels reports read
// without capabilities (OpenAPI SpaceView).
func viewOfPerson(ctx context.Context, a *Authorizer, as actor.Actor) ([]authz.SpaceView, []authz.ChannelView, error) {
	chanScope, err := a.Scope(ctx, as, authorize.ActionMessageRead)
	if err != nil {
		return nil, nil, err
	}
	spaceScope, err := a.Scope(ctx, as, authorize.ActionSpaceRead)
	if err != nil {
		return nil, nil, err
	}
	v, err := a.version(ctx)
	if err != nil {
		return nil, nil, err
	}
	sp, d, err := a.principalOf(ctx, as, string(authorize.ActionMessageRead), v, a.clock.Now())
	if err != nil {
		return nil, nil, err
	}
	if d.Outcome != domain.OutcomeAllow {
		return nil, nil, outcome(d).Err()
	}
	spaces, err := a.structure.SpaceDetails(ctx, spaceScope.SpaceIDs)
	if err != nil {
		return nil, nil, sourceError("structure source", err)
	}
	chOut, err := channelViews(ctx, a, sp, chanScope.ChannelIDs, spaces)
	if err != nil {
		return nil, nil, err
	}
	full := map[ids.ID]bool{}
	for _, id := range spaceScope.FullyVisibleSpaceIDs {
		full[id] = true
	}
	spOut := []authz.SpaceView{}
	for _, id := range spaceScope.SpaceIDs {
		d, ok := spaces[id]
		if !ok || !d.ArchivedAt.IsZero() {
			continue
		}
		view := authz.SpaceView{Space: d, Level: domain.LevelRead.String(), Capabilities: []string{}}
		if full[id] {
			r, _ := sp.rights(domain.Path{Kind: domain.PathSpace, SpaceID: id})
			view.Level, view.Capabilities = r.Level.String(), r.Caps.Names()
		}
		spOut = append(spOut, view)
	}
	return spOut, chOut, nil
}

// channelViews returns the channels of list with the rights of sp, without dm channels, archived
// channels and channels of spaces that are archived or not among spaces.
func channelViews(ctx context.Context, a *Authorizer, sp *actingPrincipal, list []ids.ID, spaces map[ids.ID]authz.SpaceDetail) ([]authz.ChannelView, error) {
	channels, err := a.structure.ChannelDetails(ctx, list)
	if err != nil {
		return nil, sourceError("structure source", err)
	}
	out := []authz.ChannelView{}
	for _, id := range list {
		facts, found, err := a.structure.Channel(ctx, id)
		if err != nil {
			return nil, sourceError("structure source", err)
		}
		c, ok := channels[id]
		sd, live := spaces[facts.SpaceID]
		if !ok || !found || facts.Kind == authz.ChannelDM || facts.Archived || !c.ArchivedAt.IsZero() || !live || !sd.ArchivedAt.IsZero() {
			continue
		}
		r, _ := sp.rights(channelPath(facts))
		out = append(out, authz.ChannelView{Channel: c, Level: r.Level.String(), Capabilities: r.Caps.Names()})
	}
	return out, nil
}
