// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// Compile-time check: the Authorizer answers the rights of a view.
var _ authz.RightsQuery = (*Authorizer)(nil)

// EffectiveRights implements authz.RightsQuery with the snapshot, the liveness and pending checks
// and the resolution of Authorize (07 sections 4.2 and 7.2): the final rights on the path of r
// after "view as" and acting for, with the archive cap. It records no check and audits nothing.
func (a *Authorizer) EffectiveRights(ctx context.Context, p authorize.Principal, act authorize.Action, r authorize.Resource) (authz.EffectiveRights, error) {
	none := authz.EffectiveRights{Capabilities: []string{}}
	spec, _, _, err := a.actionFor(act, r)
	if err != nil {
		return none, err
	}
	if p == nil || p.PrincipalKind() == domain.KindAnonymous {
		return none, nil
	}
	v, err := a.version(ctx)
	if err != nil {
		return none, err
	}
	s, d, err := a.principalOf(ctx, p, spec.ID, v, a.clock.Now())
	if err != nil || d.Outcome != domain.OutcomeAllow {
		return none, err
	}
	res, found, err := a.resolve(ctx, act, r)
	if err != nil || !found {
		return none, err
	}
	final, _ := s.rights(res.path)
	return authz.EffectiveRights{Level: final.Level.String(), Capabilities: final.Caps.Names()}, nil
}
