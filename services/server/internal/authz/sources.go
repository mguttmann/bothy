// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// This file declares the source ports through which authz learns facts owned by other modules
// (docs/spec/02-architektur.md section 4.1 rule 4, "Quellen-Port"). authz never reads their
// tables. M1-28 adds CredentialSource, StructureSource and AgentPolicySource here.

// PrincipalStatus is the lifecycle state of a principal as authz sees it.
type PrincipalStatus string

const (
	// PrincipalPending is a person waiting for approval: forbidden except for self information
	// (docs/spec/07-rechte.md SR7).
	PrincipalPending PrincipalStatus = "pending"
	// PrincipalActive is a principal whose grants take effect.
	PrincipalActive PrincipalStatus = "active"
	// PrincipalDisabled is a disabled principal: unauthenticated before any grant is read (SR7).
	PrincipalDisabled PrincipalStatus = "disabled"
	// PrincipalDeleted is a deleted principal: unauthenticated like a disabled one (SR7).
	PrincipalDeleted PrincipalStatus = "deleted"
)

// PrincipalFacts are the facts of one principal that authorize needs.
type PrincipalFacts struct {
	// ID is the principal.
	ID ids.ID
	// Kind is user, agent or app.
	Kind actor.Kind
	// Status is the lifecycle state.
	Status PrincipalStatus
	// IsOwner reports an owners row with revoked_at IS NULL, independent of Status; whether the
	// owner role takes effect (active status, a session instead of a personal token, SR5) is
	// for authz to decide.
	IsOwner bool
}

// PrincipalSource gives authz the facts of principals. The identity module implements it
// (identity/app); internal/wire passes it to authz (M1-28).
type PrincipalSource interface {
	// Principal returns the facts of one principal; ok is false when the id is unknown. It
	// reads inside the ambient transaction of ctx when there is one.
	Principal(ctx context.Context, id ids.ID) (PrincipalFacts, bool, error)
	// ActiveOwnerIDs returns the users with an active owners row whose principal status is
	// active, ascending by id. I1 and SR8 (M1-29) need this set.
	ActiveOwnerIDs(ctx context.Context) ([]ids.ID, error)
}
