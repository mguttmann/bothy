// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

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
	// HasEstablishedLocalCredential reports a local_credentials row of an active identity of
	// userID with totp_enabled_at IS NOT NULL: the established password path (M1-39, 07 section
	// 4.2 step 8).
	HasEstablishedLocalCredential(ctx context.Context, userID ids.ID) (bool, error)
	// Principal returns the principal id with its owner flag; found is false for an unknown id.
	Principal(ctx context.Context, id ids.ID) (rec PrincipalRecord, found bool, err error)
	// Principals returns the known principals among ids in ascending id order; unknown and
	// repeated ids are left out.
	Principals(ctx context.Context, ids []ids.ID) ([]domain.Principal, error)
	// PrincipalByHandle returns the principal whose handle equals handle without regard to case.
	PrincipalByHandle(ctx context.Context, handle string) (p domain.Principal, found bool, err error)
	// ActivePrincipalsByHandles returns the active people and agents whose lower-cased handle is
	// in handles (lower case), in one read.
	ActivePrincipalsByHandles(ctx context.Context, handles []string) ([]domain.Principal, error)
	// User returns the person id with its owner flag; found is false for an unknown id and for a
	// principal without a users row.
	User(ctx context.Context, id ids.ID) (rec UserRecord, found bool, err error)
	// Users returns the people among list with their owner flags, ascending by id; unknown ids and
	// principals without a users row are absent.
	Users(ctx context.Context, list []ids.ID) ([]UserRecord, error)
	// ActiveOwners returns the owners rows with revoked_at IS NULL, ordered by granted_at, id.
	ActiveOwners(ctx context.Context) ([]OwnerRecord, error)
	// ActiveOwnerIDs returns the persons with an active owners row and principal status
	// active, ascending by id.
	ActiveOwnerIDs(ctx context.Context) ([]ids.ID, error)
	// HasActivePasskey reports whether the person userID has a passkey with revoked_at IS NULL
	// (authz.PrincipalSource, M1-40).
	HasActivePasskey(ctx context.Context, userID ids.ID) (bool, error)

	// InsertPrincipal inserts p and sets p.CreatedAt from the database. The caller runs it inside
	// a unit of work.
	InsertPrincipal(ctx context.Context, p *domain.Principal) error
	// SetBotPrincipalStatus sets the status of an agent or app principal and its disabled_at
	// (zero clears it); found is false for an unknown id or a person. The caller runs it inside a
	// unit of work declared as a rights change.
	SetBotPrincipalStatus(ctx context.Context, id ids.ID, status domain.Status, disabledAt time.Time) (found bool, err error)
	// UpdateBotDisplayName sets the display name of an agent or app principal; found is false for
	// an unknown id or a person. The caller runs it inside a unit of work.
	UpdateBotDisplayName(ctx context.Context, id ids.ID, name domain.DisplayName) (found bool, err error)
	// InsertUser inserts the users row of u; its principal must exist (InsertPrincipal in the
	// same unit of work).
	InsertUser(ctx context.Context, u domain.User) error
	// InsertOwner inserts o and sets o.GrantedAt from the database. Granting the owner role is
	// a rights change: the caller runs it inside a unit of work with txn.DeclareRightsChange
	// (M1-29).
	InsertOwner(ctx context.Context, o *domain.Owner) error

	// ListUsers returns up to f.Limit people matching f, ordered by (lower(handle), id) and
	// after the keyset position of f (M1-31).
	ListUsers(ctx context.Context, f UserFilter) ([]UserRecord, error)
	// LockUser returns the person id like User and locks its row of principals until the end
	// of the unit of work. The caller runs it inside one.
	LockUser(ctx context.Context, id ids.ID) (rec UserRecord, found bool, err error)
	// UpdateUser writes display name, e-mail, locale and time zone of u (handle, kind and
	// status stay). The caller runs it inside a unit of work.
	UpdateUser(ctx context.Context, u domain.User) error
	// SetAvatar sets avatar_file_id of the person id to fileID; the zero ID clears it (M3-10). The
	// caller locks the person first (LockUser) and runs it inside that unit of work. Errors:
	// internal for a fileID without files row (the foreign key; the id comes from
	// AvatarFiles.Stage, never from the client) and for an unknown person.
	SetAvatar(ctx context.Context, id, fileID ids.ID) error
	// EnableUser sets a pending or disabled person active and clears disabled_at. user.enable
	// is a rights change: the caller declares it.
	EnableUser(ctx context.Context, id ids.ID) error
	// LockActiveOwners locks the owners rows with revoked_at IS NULL in ascending id and
	// returns their persons. The caller runs it inside a unit of work.
	LockActiveOwners(ctx context.Context) ([]ids.ID, error)
	// ActiveOwner returns the active owners row of userID with its principal; found is false
	// when there is none.
	ActiveOwner(ctx context.Context, userID ids.ID) (rec OwnerRecord, found bool, err error)
	// RevokeOwner revokes the active owners row of userID by revokedBy; revoked is false when
	// there was none. owner.remove is a rights change: the caller declares it.
	RevokeOwner(ctx context.Context, userID, revokedBy ids.ID) (revoked bool, err error)
}

// UserFilter selects a page of people (ListUsers).
type UserFilter struct {
	// Q is a case-insensitive literal prefix of handle or display name; "" for none.
	Q string
	// Status is a principal status; "" for every status.
	Status domain.Status
	// AfterHandle and AfterID are the keyset position (lower(handle), id) of the last row of
	// the previous page; AfterHandle "" for the first page.
	AfterHandle string
	AfterID     ids.ID
	// Limit is the number of rows to read, at least 1.
	Limit int
}
