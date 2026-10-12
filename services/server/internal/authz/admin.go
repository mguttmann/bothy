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
// change (M1-29). grant.changed and group.changed follow with M1-48.
func Register(r *authorize.Registry) {
	r.Event("authz", authorize.EventAccessChanged)
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

// Group is a group as the Admin port returns it (03 groups).
type Group struct {
	ID          ids.ID
	Name        string
	Description string
	// Source is local or oidc_claim.
	Source    string
	CreatedBy ids.ID
	CreatedAt time.Time
	// DeletedAt is zero for a live group.
	DeletedAt time.Time
}

// Membership is a membership as the Admin port returns it (03 group_memberships).
type Membership struct {
	ID, GroupID, UserID, AddedBy ids.ID
	AddedAt                      time.Time
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

// CreateGroupCmd is the input of CreateGroup.
type CreateGroupCmd struct {
	Name, Description string
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

// Admin is the incoming port of authz for groups, memberships and grants (05 section 4.4). Every
// method takes the caller as actor.Actor, loads the resource, calls authorize with the action
// before any write, runs in one unit of work (rights changes with permission_version, I1, SR8 and
// access.changed through the rights hooks, M1-29) and writes the allow audit entry in the same
// transaction. A resource that is missing or invisible gives the same 404 not_found (02 section
// 9.2). Errors are kernel/errs errors: 400 validation_failed, 401 step_up_required, 403 forbidden
// with the reasons of 07 section 4.1, 404 not_found, 409 conflict, and the InvariantError of
// 409 i1_violation or last_owner. The HTTP routes come with M1-48.
type Admin interface {
	// CreateGroup creates a local group (group.create, no rights change). The name is not empty,
	// free of control and bidi characters and unique among live groups without regard to case.
	CreateGroup(ctx context.Context, caller actor.Actor, cmd CreateGroupCmd) (Group, error)
	// UpdateGroup renames or describes a live group (group.update, no rights change); a claim
	// group keeps the name of its claim (409 conflict, reason claim_managed_group). A command
	// without change succeeds with an audit entry marked no_change.
	UpdateGroup(ctx context.Context, caller actor.Actor, groupID ids.ID, cmd UpdateGroupCmd) (Group, error)
	// DeleteGroup sets deleted_at and revokes every active grant of the group with reason
	// (group.delete, rights change for the members). An already deleted group succeeds without
	// change.
	DeleteGroup(ctx context.Context, caller actor.Actor, groupID ids.ID, reason string) error
	// AddGroupMember adds a person to a local group (group.member.add, rights change for the
	// person). An existing active membership is 409 conflict.
	AddGroupMember(ctx context.Context, caller actor.Actor, groupID ids.ID, cmd AddMemberCmd) (Membership, error)
	// RemoveGroupMember removes a person from a local group (group.member.remove). A person
	// that never was a member is 404; a membership already removed or expired succeeds without
	// change.
	RemoveGroupMember(ctx context.Context, caller actor.Actor, groupID, userID ids.ID) error
	// CreateGrant creates a grant (grant.create, rights change for the subject or the members of
	// the subject group) within the delegation limits of 07 section 2.6.
	CreateGrant(ctx context.Context, caller actor.Actor, cmd CreateGrantCmd) (Grant, error)
	// RevokeGrant revokes a grant (grant.revoke) within 07 section 2.6 point 3; an already
	// revoked grant succeeds without change and is returned as stored.
	RevokeGrant(ctx context.Context, caller actor.Actor, grantID ids.ID, reason string) (Grant, error)
}
