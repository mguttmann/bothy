// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// EffectiveRights are the level and capabilities of a principal on one resource (05 section 2.3),
// for views like SpaceView (M1-52).
type EffectiveRights struct {
	// Level is read, write or trigger; "" for none.
	Level string
	// Capabilities are the capability names, ascending; never nil.
	Capabilities []string
}

// RightsQuery answers the effective rights on one resource from the same rights snapshot and the
// same path resolution as authorize (implemented by the Authorizer of authz). It decides nothing:
// the caller has authorized the call before, and the answer only describes what the caller may do.
type RightsQuery interface {
	// EffectiveRights returns the rights of p on r; act is the action the caller authorized (its
	// resource kinds must contain r.Kind). A principal that authorize would refuse before the
	// resource (anonymous, disabled, pending) and an unknown resource give no rights. Errors: an
	// unknown action or resource kind, contradictory path hints, a source error.
	EffectiveRights(ctx context.Context, p authorize.Principal, act authorize.Action, r authorize.Resource) (EffectiveRights, error)
}
