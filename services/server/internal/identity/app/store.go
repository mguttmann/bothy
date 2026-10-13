// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// PrincipalRecord is a principal with its owner flag.
type PrincipalRecord struct {
	// Principal is the row of principals.
	Principal domain.Principal
	// IsOwner reports an owners row with revoked_at IS NULL for this principal.
	IsOwner bool
}

// UserRecord is a person with its owner flag.
type UserRecord struct {
	// User is the row of users with its principal.
	User domain.User
	// IsOwner reports an owners row with revoked_at IS NULL for this person.
	IsOwner bool
}

// OwnerRecord is an owners row with the principal of its person.
type OwnerRecord struct {
	// Owner is the row of owners.
	Owner domain.Owner
	// User is the principal of Owner.UserID.
	User domain.Principal
}

// Store is the outgoing port of the identity application layer over the tables principals,
// users and owners (implemented by adapters/postgres). Reads run in the ambient transaction of
// ctx when there is one and on the pool otherwise. Writes run only inside a unit of work of
// kernel/txn; outside one they fail with kind internal.
//
// Errors are kernel/errs errors and never carry SQL text or values: a unique violation is kind
// conflict with the field (/handle, /email, /user_id), a check, trigger, foreign key or not null
// violation is validation_failed with the column as field, anything else is internal (or the
// unavailable error of the pool).
type Store interface {
	// Principal returns the principal id with its owner flag; found is false for an unknown id.
	Principal(ctx context.Context, id ids.ID) (rec PrincipalRecord, found bool, err error)
	// Principals returns the known principals among ids in ascending id order; unknown and
	// repeated ids are left out.
	Principals(ctx context.Context, ids []ids.ID) ([]domain.Principal, error)
	// PrincipalByHandle returns the principal whose handle equals handle without regard to case.
	PrincipalByHandle(ctx context.Context, handle string) (p domain.Principal, found bool, err error)
	// User returns the person id with its owner flag; found is false for an unknown id and for a
	// principal without a users row.
	User(ctx context.Context, id ids.ID) (rec UserRecord, found bool, err error)
	// ActiveOwners returns the owners rows with revoked_at IS NULL, ordered by granted_at, id.
	ActiveOwners(ctx context.Context) ([]OwnerRecord, error)
	// ActiveOwnerIDs returns the persons with an active owners row and principal status
	// active, ascending by id.
	ActiveOwnerIDs(ctx context.Context) ([]ids.ID, error)

	// InsertPrincipal inserts p and sets p.CreatedAt from the database. The caller runs it inside
	// a unit of work.
	InsertPrincipal(ctx context.Context, p *domain.Principal) error
	// InsertUser inserts the users row of u; its principal must exist (InsertPrincipal in the
	// same unit of work).
	InsertUser(ctx context.Context, u domain.User) error
	// InsertOwner inserts o and sets o.GrantedAt from the database. Granting the owner role is
	// a rights change: the caller runs it inside a unit of work with txn.RightsChange (M1-29).
	InsertOwner(ctx context.Context, o *domain.Owner) error
}
