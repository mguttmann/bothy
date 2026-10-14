// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Register declares what authz emits (M1-25): the personal event access.changed of every rights
// change (M1-29); grant.changed, group.changed and the personal event access.expiring of the
// expiry jobs (M1-30); the Admin use cases append grant.changed and group.changed on every write
// (M1-48).
func Register(r *authorize.Registry) {
	r.Event("authz", authorize.EventAccessChanged)
	r.Event("authz", authorize.EventGrantChanged)
	r.Event("authz", authorize.EventGroupChanged)
	r.Event("authz", authorize.EventAccessExpiring)
}

// RightsChanges is the port another module uses during the work phase of a rights change it
// declared with txn.DeclareRightsChange (02 section 8.1): it names principals it learned only
// after the declaration (for example the new operator agent), before it writes the rows that
// concern them, so their visible targets before the change are recorded. authz/app implements it
// (RightsHook).
//
// Example (chat, M1-54):
//
//	err := uow.Do(ctx, func(ctx context.Context) error {
//		if err := rights.AddAffected(ctx, newOperator); err != nil {
//			return err
//		}
//		return repo.SetOperator(ctx, channel, newOperator)
//	}, txn.DeclareRightsChange(txn.RightsChangeSpec{Action: "channel.operator.set",
//		Resource: txn.Ref{Kind: "channel", ID: channel}, Principals: []ids.ID{oldOperator}}))
type RightsChanges interface {
	// AddAffected adds principals to the affected set of the rights change of ctx and records
	// their visible targets now. Errors: kind internal outside the work phase of a declared
	// rights change; source errors.
	AddAffected(ctx context.Context, principals ...ids.ID) error
}

// Group is a group as the Admin port returns it (03 groups, OpenAPI Group).
type Group struct {
	ID          ids.ID
	Name        string
	Description string
	// Source is local or oidc_claim.
	Source string
	// ProviderID and ExternalRef bind a claim group to its provider and claim value; zero and
	// empty for a local group.
	ProviderID  ids.ID
	ExternalRef string
	// MemberCount counts the memberships in effect at the time of the request (M1-48).
	MemberCount int
	CreatedBy   ids.ID
	CreatedAt   time.Time
	// DeletedAt is zero for a live group.
	DeletedAt time.Time
}

// Membership is a membership as the Admin port returns it (03 group_memberships, OpenAPI
// GroupMember).
type Membership struct {
	ID, GroupID, UserID, AddedBy ids.ID
	// User is the public view of the member (M1-48); its ID equals UserID.
	User    PrincipalRef
	AddedAt time.Time
	// ExpiresAt is zero for none.
	ExpiresAt time.Time
}

// Grant is a grant as the Admin port returns it (03 grants). Level and Capabilities are the
// column values.
type Grant struct {
	ID ids.ID
	// SubjectKind is user, group, agent or app; SubjectID the principal or group.
	SubjectKind string
	SubjectID   ids.ID
	// TargetKind is instance, space, channel or host; TargetID zero for the instance.
	TargetKind   string
	TargetID     ids.ID
	Level        string
	Capabilities []string
	ExpiresAt    time.Time
	Reason       string
	GrantedBy    ids.ID
	GrantedAt    time.Time
	RevokedAt    time.Time
	RevokedBy    ids.ID
	RevokeReason string
}

// CreateGroupCmd is the input of CreateGroup. Source is local (the default for "") or
// oidc_claim; a claim group needs ProviderID and ExternalRef, a local group has neither.
type CreateGroupCmd struct {
	Name, Description string
	Source            string
	ProviderID        ids.ID
	ExternalRef       string
}

// UpdateGroupCmd is the input of UpdateGroup; a nil field stays unchanged.
type UpdateGroupCmd struct {
	Name, Description *string
}

// AddMemberCmd is the input of AddGroupMember; a zero ExpiresAt means no expiry.
type AddMemberCmd struct {
	UserID    ids.ID
	ExpiresAt time.Time
}

// CreateGrantCmd is the input of CreateGrant with the column values of grants; a zero TargetID
// belongs to the instance, a zero ExpiresAt means no expiry.
type CreateGrantCmd struct {
	SubjectKind  string
	SubjectID    ids.ID
	TargetKind   string
	TargetID     ids.ID
	Level        string
	Capabilities []string
	ExpiresAt    time.Time
	Reason       string
}

// PageQuery is the cursor and the requested page size of a list; a zero Limit means
// api.page_size_default, a larger one than api.page_size_max is capped.
type PageQuery struct {
	Cursor string
	Limit  int
}

// ListGroupsQuery is the input of ListGroups; a zero MemberID lists without the member filter.
type ListGroupsQuery struct {
	PageQuery
	MemberID ids.ID
}

// ListGrantsQuery is the input of ListGrants (OpenAPI listGrants). TargetKind with TargetID
// (none for instance) and SubjectKind with SubjectID are pairs; a half pair is 400
// validation_failed. Without IncludeRevoked only the grants in effect at the request are listed.
type ListGrantsQuery struct {
	PageQuery
	TargetKind     string
	TargetID       ids.ID
	SubjectKind    string
	SubjectID      ids.ID
	IncludeRevoked bool
}

// GroupPage is one page of groups; NextCursor is empty on the last page.
type GroupPage struct {
	Items      []Group
	NextCursor string
}

// MemberPage is one page of memberships; NextCursor is empty on the last page.
type MemberPage struct {
	Items      []Membership
	NextCursor string
}

// GrantPage is one page of grants; NextCursor is empty on the last page.
type GrantPage struct {
	Items      []Grant
	NextCursor string
}

// Admin is the incoming port of authz for groups, memberships and grants (05 section 4.4). Every
// method takes the caller as actor.Actor, loads the resource, calls authorize with the action
// before any write, runs in one unit of work (rights changes with permission_version, I1, SR8 and
// access.changed through the rights hooks, M1-29) and writes the allow audit entry in the same
// transaction. A resource that is missing or invisible gives the same 404 not_found (02 section
// 9.2). Errors are kernel/errs errors: 400 validation_failed, 401 step_up_required, 403 forbidden
// with the reasons of 07 section 4.1, 404 not_found, 409 conflict, and the InvariantError of
// 409 i1_violation or last_owner. Every write appends grant.changed or group.changed for the
// administrators; the reads show nothing outside the caller's view (M1-48, the twelve REST
// operations of 06 "Groups" and "Access").
type Admin interface {
	// ListGroups lists live groups ascending by id (group.read): an owner all of them, anyone
	// else the groups with an in-effect membership of the caller; MemberID narrows to the groups
	// of that member, and for a non-owner a member other than the caller gives an empty page.
	ListGroups(ctx context.Context, caller actor.Actor, q ListGroupsQuery) (GroupPage, error)
	// GetGroup returns a live group (group.read); a group the caller is neither owner of nor an
	// in-effect member of is 404 like an unknown id.
	GetGroup(ctx context.Context, caller actor.Actor, groupID ids.ID) (Group, error)
	// ListGroupMembers lists the memberships of a live group in effect at the request, ascending
	// by membership id (group.read, visibility of GetGroup).
	ListGroupMembers(ctx context.Context, caller actor.Actor, groupID ids.ID, p PageQuery) (MemberPage, error)
	// ListGrants lists grants ascending by id (grant.read): an owner every grant, anyone else
	// the grants on spaces and channels inside the caller's manage scope; filters narrow and
	// never widen the set (07 section 2.6 item 6). A target filter the caller cannot read is 404,
	// a readable target without manage 403.
	ListGrants(ctx context.Context, caller actor.Actor, q ListGrantsQuery) (GrantPage, error)
	// GetGrant returns a grant, revoked or not (grant.read); outside the caller's manage scope,
	// and for an instance grant unless the caller is an owner, it is 404 like an unknown id.
	GetGrant(ctx context.Context, caller actor.Actor, grantID ids.ID) (Grant, error)
	// CreateGroup creates a group (group.create, no rights change): local, or bound to the
	// group claim of an OIDC provider. The name is not empty, free of control and bidi
	// characters and unique among live groups without regard to case.
	CreateGroup(ctx context.Context, caller actor.Actor, cmd CreateGroupCmd) (Group, error)
	// UpdateGroup renames or describes a live group (group.update, no rights change); a claim
	// group keeps the name of its claim (409 conflict, reason claim_managed_group). A command
	// without change succeeds with an audit entry marked no_change.
	UpdateGroup(ctx context.Context, caller actor.Actor, groupID ids.ID, cmd UpdateGroupCmd) (Group, error)
	// DeleteGroup sets deleted_at and revokes every active grant of the group with the reason
	// group_deleted (group.delete, rights change for the members); the memberships stay as
	// history. An already deleted group succeeds without change.
	DeleteGroup(ctx context.Context, caller actor.Actor, groupID ids.ID) error
	// AddGroupMember adds a person to a local group (group.member.add, rights change for the
	// person). An existing membership in effect is 409 conflict; a user id that is no person or
	// is deleted is 400 validation_failed on /user_id with code unknown_user.
	AddGroupMember(ctx context.Context, caller actor.Actor, groupID ids.ID, cmd AddMemberCmd) (Membership, error)
	// RemoveGroupMember removes a person from a local group (group.member.remove). A person
	// that never was a member is 404; a membership already removed or expired succeeds without
	// change.
	RemoveGroupMember(ctx context.Context, caller actor.Actor, groupID, userID ids.ID) error
	// CreateGrant creates a grant (grant.create, rights change for the subject or the members of
	// the subject group) within the delegation limits of 07 section 2.6. A subject that does not
	// exist is 400 validation_failed on /subject_id with code unknown_subject, decided after
	// authorize, so a caller without rights learns nothing about ids.
	CreateGrant(ctx context.Context, caller actor.Actor, cmd CreateGrantCmd) (Grant, error)
	// RevokeGrant revokes a grant with a reason that is not blank (grant.revoke) within 07
	// section 2.6 point 3; an already revoked grant succeeds without change and is returned as
	// stored.
	RevokeGrant(ctx context.Context, caller actor.Actor, grantID ids.ID, reason string) (Grant, error)
}
