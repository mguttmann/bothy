// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the Authorizer answers the trust facts of the run context (M2-13).
var _ authorize.TriggerHolders = (*Authorizer)(nil)

// WithTrigger implements authorize.TriggerHolders (M2-13 design note 7, rulings D-M2-13-1 and
// D-M2-13-2). Each person is judged with a snapshot without credentials, loaded and cached like
// the snapshot of "view as" at the rights version of the request: active humans only, the
// effective level on the channel path (grants of the person and of the groups in effect, the DM
// rule) and the archive cap. The owner role needs a session in Authorize (R-AZ-14); here an active
// owner counts on every channel that is not a DM, because trust asks whether the person may
// instruct the agent and an owner triggers from any session (05 2.3 point 1). It writes no audit
// entry and records no check. Errors: internal for a resource that is not a channel; the error of
// a source, never a partial result.
func (a *Authorizer) WithTrigger(ctx context.Context, r authorize.Resource, people []ids.ID) ([]ids.ID, error) {
	if r.Kind != authorize.ResourceChannel || r.ID.IsZero() {
		return nil, internal("WithTrigger needs a channel")
	}
	ch, found, err := a.channel(ctx, r.ID)
	if err != nil {
		return nil, sourceError("structure source", err)
	}
	out := []ids.ID{}
	if !found || len(people) == 0 {
		return out, nil
	}
	path := channelPath(ch)
	v, err := a.version(ctx)
	if err != nil {
		return nil, err
	}
	t := a.clock.Now()
	asked := slices.Clone(people)
	slices.SortFunc(asked, func(x, y ids.ID) int { return slices.Compare(x[:], y[:]) })
	asked = slices.Compact(asked)
	for _, id := range asked {
		if id.IsZero() {
			continue
		}
		e, err := a.load(ctx, authz.CredentialRef{PrincipalID: id}, v, t)
		if err != nil {
			return nil, err
		}
		if holdsTrigger(e, id, path) {
			out = append(out, id)
		}
	}
	return out, nil
}

// holdsTrigger reports whether the person id of the snapshot entry e has effective trigger on
// path: an active human whose rights reach trigger, or an active owner on a path that is not a DM,
// both after the archive cap.
func holdsTrigger(e entry, id ids.ID, path domain.Path) bool {
	if !e.found || e.facts.Kind != actor.KindUser || e.facts.Status != authz.PrincipalActive {
		return false
	}
	r := domain.Effective(e.snap, domain.KindUser, id, path)
	if e.facts.IsOwner && !path.DM {
		r = domain.Effective(domain.Snapshot{Owner: true}, domain.KindUser, id, path)
	}
	return r.Level == domain.LevelTrigger
}
