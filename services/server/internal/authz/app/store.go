// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Store is the outgoing port of the authz application layer over the tables groups,
// group_memberships and grants (implemented by adapters/postgres). "Active" means not revoked,
// not removed; whether a row is in effect at a time t is for the domain to decide. Lists are
// ordered by id so results are deterministic.
//
// Reads run in the ambient transaction of ctx when there is one and on the pool otherwise.
// Writes run only inside a unit of work of kernel/txn; outside one they fail with kind internal.
// Every write changes rights, so it must run inside a unit of work marked as a rights change
// (txn.RightsChange); M1-29 enforces this and raises permission_version.
//
// Errors are kernel/errs errors and never carry SQL text or values: a unique violation is kind
// conflict with the field (/name, /user_id, /external_ref); a check, trigger, foreign key or
// not null violation is validation_failed with the field of the constraint; anything else is
// internal (or the unavailable error of the pool).
type Store interface {
	// InsertGroup inserts g with g.CreatedAt as created_at. A live group with the same name
	// without regard to case is a conflict on /name. Rights change.
	InsertGroup(ctx context.Context, g domain.Group) error
	// GetGroup returns the group id, deleted or not; found is false for an unknown id.
	GetGroup(ctx context.Context, id ids.ID) (g domain.Group, found bool, err error)
	// SoftDeleteGroup sets deleted_at of a live group to at and reports whether a row changed;
	// false for an unknown or already deleted group. Rights change.
	SoftDeleteGroup(ctx context.Context, id ids.ID, at time.Time) (bool, error)

	// InsertMembership inserts m with m.AddedAt as added_at. A second active membership of the
	// same person in the same group is a conflict on /user_id; a user id without a users row
	// (an agent or app) is validation_failed on /user_id (R-DM-07). Rights change.
	InsertMembership(ctx context.Context, m domain.Membership) error
	// RemoveMembership sets removed_at and removed_by of an active membership and reports
	// whether a row changed; false for an unknown or already removed membership. A zero by
	// leaves removed_by NULL. Rights change.
	RemoveMembership(ctx context.Context, id, by ids.ID, at time.Time) (bool, error)
	// ActiveMembershipsOfUser returns the memberships of userID that are not removed, each with
	// the row of its group (also when the group is deleted), ordered by membership id.
	ActiveMembershipsOfUser(ctx context.Context, userID ids.ID) ([]domain.MembershipWithGroup, error)

	// InsertGrant inserts g with g.GrantedAt as granted_at. The checks of grants (bot limits,
	// shapes, host rules) are the last line behind domain.NewGrant. Rights change.
	InsertGrant(ctx context.Context, g domain.Grant) error
	// GetGrant returns the grant id, revoked or not; found is false for an unknown id.
	GetGrant(ctx context.Context, id ids.ID) (g domain.Grant, found bool, err error)
	// RevokeGrant sets revoked_at, revoked_by and revoke_reason of a grant that is not revoked
	// and returns the revoked row; found is false for an unknown or already revoked grant, so a
	// grant is revoked exactly once. A zero by and an empty reason are stored as NULL. Rights
	// change.
	RevokeGrant(ctx context.Context, id, by ids.ID, reason string, at time.Time) (g domain.Grant, found bool, err error)
	// ActiveGrantsOfPrincipal returns the grants that are not revoked with subject_principal_id
	// equal to principalID, ordered by id.
	ActiveGrantsOfPrincipal(ctx context.Context, principalID ids.ID) ([]domain.Grant, error)
	// ActiveGrantsOfGroups returns the grants that are not revoked whose subject is one of
	// groupIDs, ordered by id; an empty list gives none without a query.
	ActiveGrantsOfGroups(ctx context.Context, groupIDs []ids.ID) ([]domain.Grant, error)
}
