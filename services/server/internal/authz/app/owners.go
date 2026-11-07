// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the Authorizer answers the owner role of a wake-up's person (AR-04).
var _ authorize.Owners = (*Authorizer)(nil)

// IsActiveOwner implements authorize.Owners (07 2.14, AR-04): the person's facts are loaded like
// the snapshot of WithTrigger, at the rights version of the request; an active human with an
// owners row is an owner. Unlike Authorize it needs no session, because the wake-up acts for the
// person without a request of that person (R-AZ-29). It writes no audit entry and records no
// check. Errors: the error of a source, never "owner".
func (a *Authorizer) IsActiveOwner(ctx context.Context, personID ids.ID) (bool, error) {
	if personID.IsZero() {
		return false, nil
	}
	v, err := a.version(ctx)
	if err != nil {
		return false, err
	}
	e, err := a.load(ctx, authz.CredentialRef{PrincipalID: personID}, v, a.clock.Now())
	if err != nil {
		return false, err
	}
	return e.found && e.facts.Kind == actor.KindUser && e.facts.Status == authz.PrincipalActive && e.facts.IsOwner, nil
}
