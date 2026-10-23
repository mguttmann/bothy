// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the Authorizer answers the candidates of hints (M3-19 design note 6).
var _ authz.Recipients = (*Authorizer)(nil)

// Readers implements authz.Recipients on the Authorizer that internal/wire builds, because the
// answer comes from the same reads as the candidates of "who has access" (M1-49), which live
// here: the owners (not for a dm channel, SR6), the subjects of the grants not revoked on the
// instance, the space and the channel with the members of such groups (grantSubjects), and the
// participants of a dm channel. It applies no guard and no validity: the caller decides each
// candidate with authorize(r, push.deliver, thread), so the superset never leaks a hint. Only
// active humans are kept, because an agent, an app or a person who is not active never receives
// one (13 section 2). The list is ascending without duplicates. Errors: not_found for an unknown
// channel; internal for a failing source.
func (a *Authorizer) Readers(ctx context.Context, channelID ids.ID) ([]ids.ID, error) {
	res, found, err := a.resolveChannel(ctx, ids.ID{}, channelID, ids.ID{})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errNotFound()
	}
	path := res.path
	var candidates []ids.ID
	if !path.DM {
		owners, err := a.principal.OwnerIDs(ctx)
		if err != nil {
			return nil, sourceError("principal source", err)
		}
		candidates = append(candidates, owners...)
	}
	var spaces []ids.ID
	if !path.SpaceID.IsZero() {
		spaces = append(spaces, path.SpaceID)
	}
	subjects, _, err := (&explainer{a: a}).grantSubjects(ctx, spaces, []ids.ID{path.ChannelID})
	if err != nil {
		return nil, err
	}
	candidates = append(append(candidates, subjects...), path.DMParticipants...)
	candidates = uniqueIDs(candidates)
	if len(candidates) == 0 {
		return []ids.ID{}, nil
	}
	profiles, err := a.principal.Profiles(ctx, candidates)
	if err != nil {
		return nil, sourceError("principal source", err)
	}
	out := make([]ids.ID, 0, len(candidates))
	for _, id := range candidates {
		if p, ok := profiles[id]; ok && p.Kind == actor.KindUser && p.Status == authz.PrincipalActive {
			out = append(out, id)
		}
	}
	return out, nil
}
