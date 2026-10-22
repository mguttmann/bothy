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
// The writes marked "Rights change" also need a unit of work declared as a rights change
// (txn.RightsChange or txn.DeclareRightsChange) and fail with kind internal otherwise, so the
// rights hooks always raise permission_version and verify I1 and SR8 (M1-29). Creating and
// renaming a group change no rights.
//
// Errors are kernel/errs errors and never carry SQL text or values: a unique violation is kind
// conflict with the field (/name, /user_id, /external_ref); a check, trigger, foreign key or
// not null violation is validation_failed with the field of the constraint; anything else is
// internal (or the unavailable error of the pool).
type Store interface {
	// InsertGroup inserts g with g.CreatedAt as created_at. A live group with the same name
	// without regard to case is a conflict on /name. No rights change: a new group has no
	// members and no grants.
	InsertGroup(ctx context.Context, g domain.Group) error
	// GetGroup returns the group id, deleted or not; found is false for an unknown id.
	GetGroup(ctx context.Context, id ids.ID) (g domain.Group, found bool, err error)
	// UpdateGroup sets name and description of a live group and reports whether a row changed;
	// false for an unknown or deleted group. A taken name is a conflict on /name. No rights
	// change.
	UpdateGroup(ctx context.Context, id ids.ID, name, description string) (bool, error)
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
	// ActiveMembershipsOfGroups returns the memberships that are not removed of the groups, each
	// with the row of its group, ordered by membership id; an empty list gives none.
	ActiveMembershipsOfGroups(ctx context.Context, groupIDs []ids.ID) ([]domain.MembershipWithGroup, error)
	// MembershipsOfUserInGroup returns every membership of userID in groupID, removed ones
	// included, ordered by id.
	MembershipsOfUserInGroup(ctx context.Context, groupID, userID ids.ID) ([]domain.Membership, error)
	// LockClaimMembershipsOfUser returns the memberships of userID that are not removed (expired
	// ones included) in the live oidc_claim groups of providerID, each with its group, ordered by
	// membership id and locked FOR UPDATE inside the ambient transaction (the claim sync, M1-42).
	LockClaimMembershipsOfUser(ctx context.Context, userID, providerID ids.ID) ([]domain.MembershipWithGroup, error)
	// ClaimGroupsByRefs returns the live oidc_claim groups of providerID whose external_ref is one
	// of refs, ordered by id; an empty list gives none without a query.
	ClaimGroupsByRefs(ctx context.Context, providerID ids.ID, refs []string) ([]domain.Group, error)
	// ExtendMembership sets expires_at of a membership that is not removed and clears
	// expiry_reminded_at and expiry_reported_at, so the new expiry is reminded and reported once;
	// it reports whether a row changed. at is the time of the change. Rights change.
	ExtendMembership(ctx context.Context, id ids.ID, expiresAt, at time.Time) (bool, error)

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
	// ActiveTriggerGrants returns every grant that is not revoked with level trigger, ordered by
	// id (the I1 check, 03 section 6.4).
	ActiveTriggerGrants(ctx context.Context) ([]domain.Grant, error)
	// ActiveGrantsOnTargets returns the grants that are not revoked on the instance, on one of
	// spaceIDs or on one of channelIDs, ordered by id (the affected principals of a rights change
	// on a target, 02 section 8.1).
	ActiveGrantsOnTargets(ctx context.Context, spaceIDs, channelIDs []ids.ID) ([]domain.Grant, error)
	// ActiveGrantsOnHosts returns the grants that are not revoked on one of hostIDs, ordered by id
	// (the affected principals of a rights change on a host, M2-39); an empty list gives none.
	ActiveGrantsOnHosts(ctx context.Context, hostIDs []ids.ID) ([]domain.Grant, error)

	// ListLiveGroups returns at most limit live groups with an id above after, ascending; with a
	// non-zero memberID only the groups in which memberID has a membership in effect at t (M1-48).
	ListLiveGroups(ctx context.Context, after, memberID ids.ID, t time.Time, limit int) ([]domain.Group, error)
	// CountMembersInEffect returns, per group of groupIDs, the memberships that are neither
	// removed nor expired at t; a group without such memberships is absent (member_count).
	CountMembersInEffect(ctx context.Context, groupIDs []ids.ID, t time.Time) (map[ids.ID]int, error)
	// ListMembersInEffect returns at most limit memberships of groupID in effect at t with an id
	// above after, ascending (listGroupMembers).
	ListMembersInEffect(ctx context.Context, groupID, after ids.ID, t time.Time, limit int) ([]domain.Membership, error)
	// ListGrants returns at most limit grants with an id above after that match f at t,
	// ascending (listGrants).
	ListGrants(ctx context.Context, f GrantFilter, after ids.ID, t time.Time, limit int) ([]domain.Grant, error)
	// ExpiringGrants returns the grants that are not revoked, not of a deleted group, whose
	// expires_at lies in (t, until], ordered by expires_at, then id (the list "Expiring soon",
	// M6-07); a read without lock.
	ExpiringGrants(ctx context.Context, t, until time.Time) ([]domain.Grant, error)
	// ExpiringMemberships returns the memberships that are not removed, of live groups, whose
	// expires_at lies in (t, until], ordered by expires_at, then id, each with its group (M6-07);
	// a read without lock.
	ExpiringMemberships(ctx context.Context, t, until time.Time) ([]domain.MembershipWithGroup, error)
}

// GrantFilter selects the grants of ListGrants. Zero values do not filter. Without AllTargets
// only grants on ManageSpaces (space targets) and ManageChannels (channel targets) qualify: the
// manage scope of a caller who is no owner, applied in the query (07 section 2.6 item 6).
type GrantFilter struct {
	// TargetKind with TargetID (zero for the instance) and SubjectKind with SubjectID.
	TargetKind  domain.TargetKind
	TargetID    ids.ID
	SubjectKind domain.SubjectKind
	SubjectID   ids.ID
	// IncludeRevoked adds revoked and expired grants to the ones in effect.
	IncludeRevoked bool
	// AllTargets lifts the scope (owners).
	AllTargets                   bool
	ManageSpaces, ManageChannels []ids.ID
}

// ExpiryStore is the outgoing port of the expiry jobs (M1-30, 07 sections 8.3 and 9.4) over
// grants and group_memberships, implemented by adapters/postgres. Every method runs only inside
// a unit of work (kind internal otherwise). The Due methods lock the rows they return (FOR
// UPDATE, ascending id, at most limit rows), so concurrent runs never select the same row; the
// Mark methods set a marker from NULL and fail with kind internal when a row was already marked,
// because the selection under the lock rules that out. The Reported marks are rights writes
// (the unit must be declared as a rights change); the Reminded marks are not.
type ExpiryStore interface {
	// DueExpiredGrants returns the grants that are not revoked, with expires_at <= t and
	// expiry_reported_at NULL.
	DueExpiredGrants(ctx context.Context, t time.Time, limit int) ([]domain.Grant, error)
	// MarkGrantsExpiryReported sets expiry_reported_at = at on the grants. Rights change.
	MarkGrantsExpiryReported(ctx context.Context, grantIDs []ids.ID, at time.Time) error
	// DueExpiredMemberships returns the memberships that are not removed, with expires_at <= t
	// and expiry_reported_at NULL, each with its group (deleted groups included).
	DueExpiredMemberships(ctx context.Context, t time.Time, limit int) ([]domain.MembershipWithGroup, error)
	// MarkMembershipsExpiryReported sets expiry_reported_at = at on the memberships. Rights change.
	MarkMembershipsExpiryReported(ctx context.Context, membershipIDs []ids.ID, at time.Time) error
	// DueReminderGrants returns the grants that are not revoked, with t < expires_at <= until
	// and expiry_reminded_at NULL, without grants of deleted groups.
	DueReminderGrants(ctx context.Context, t, until time.Time, limit int) ([]domain.Grant, error)
	// MarkGrantsExpiryReminded sets expiry_reminded_at = at on the grants.
	MarkGrantsExpiryReminded(ctx context.Context, grantIDs []ids.ID, at time.Time) error
	// DueReminderMemberships returns the memberships that are not removed, of live groups, with
	// t < expires_at <= until and expiry_reminded_at NULL, each with its group.
	DueReminderMemberships(ctx context.Context, t, until time.Time, limit int) ([]domain.MembershipWithGroup, error)
	// MarkMembershipsExpiryReminded sets expiry_reminded_at = at on the memberships.
	MarkMembershipsExpiryReminded(ctx context.Context, membershipIDs []ids.ID, at time.Time) error
}
