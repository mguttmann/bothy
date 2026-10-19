// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"
	"errors"
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

// AccessTargetRef names the target of access.explain: a space or a channel (OpenAPI
// AccessTargetKind without host).
type AccessTargetRef struct {
	// Kind is space or channel.
	Kind string
	ID   ids.ID
}

// LevelNone is the level text of an entry without access (OpenAPI LevelOrNone).
const LevelNone = "none"

// AccessSource is one source of access (OpenAPI AccessSource, 07 section 9.1); zero fields are
// absent.
type AccessSource struct {
	// Kind is owner, direct, group, inherited, dm_participant, operator, via_agent or
	// guard_blocked.
	Kind             string
	GrantID, GroupID ids.ID
	// FromTargetKind and FromTargetID name the ancestor of inherited (no id for the instance).
	FromTargetKind string
	FromTargetID   ids.ID
	// AgentID is the agent of operator, via_agent and guard_blocked; Rule (sr2) that of
	// guard_blocked.
	AgentID ids.ID
	Rule    string
	// ExpiresAt is the expiry of the grant, MembershipExpiresAt that of the membership.
	ExpiresAt, MembershipExpiresAt time.Time
	// Reason is the reason of the grant.
	Reason string
}

// AccessEntry is one principal of an explanation (OpenAPI AccessEntry).
type AccessEntry struct {
	Principal PrincipalRef
	// Level is none, read, write or trigger; none only for an entry through an agent alone.
	Level string
	// Inactive marks a principal whose status is not active; Level is what its grants would give.
	Inactive     bool
	Capabilities []string
	Sources      []AccessSource
}

// AccessExplanation is the answer of access.explain (OpenAPI AccessExplanation).
type AccessExplanation struct {
	TargetKind string
	TargetID   ids.ID
	Entries    []AccessEntry
}

// AccessEffect is one effect of a previewed grant (OpenAPI AccessEffect).
type AccessEffect struct {
	Principal PrincipalRef
	// TargetKind is space, channel or host (M2-40).
	TargetKind              string
	TargetID                ids.ID
	BeforeLevel, AfterLevel string
	CapabilitiesAdded       []string
}

// InvariantViolation is one invariant a previewed grant would break (OpenAPI InvariantViolation):
// i1_violation with the agent, i6_violation with the host (M2-40), or last_owner.
type InvariantViolation struct {
	Code               string
	AgentID            ids.ID
	HostID             ids.ID
	CredentialHolderID ids.ID
	// PrincipalIDs would gain trigger next to the credential holder; never nil.
	PrincipalIDs []ids.ID
	// OrganizationReady reports organization among the agent's billing_modes_supported.
	OrganizationReady bool
	Hint              string
}

// PreviewHint is one hint of a preview (OpenAPI PreviewHint): read_horizon,
// trigger_blocked_tool_class, group_reach, expiry_missing or terminal_not_effective (M2-40).
// HasCount marks a present Count.
type PreviewHint struct {
	Code                                              string
	AgentID, PrincipalID, GroupID, SpaceID, ChannelID ids.ID
	Count                                             int64
	HasCount                                          bool
}

// GrantPreview is the answer of access.preview for a grant (OpenAPI GrantPreview, 07 section 9.3).
type GrantPreview struct {
	// Allowed is false when DenialReason is set or an invariant would reject the grant.
	Allowed      bool
	DenialReason string
	Effects      []AccessEffect
	// InvariantViolations and Hints are never nil.
	InvariantViolations []InvariantViolation
	Hints               []PreviewHint
	// Text is the plain text in TextLocale (de or en).
	Text, TextLocale string
}

// SpaceView is one space of "view as" with the intersected level and capabilities.
type SpaceView struct {
	Space        SpaceDetail
	Level        string
	Capabilities []string
}

// ChannelView is one channel of "view as" with the intersected level and capabilities.
type ChannelView struct {
	Channel      ChannelDetail
	Level        string
	Capabilities []string
}

// ViewAsSnapshot is the answer of access.view_as (OpenAPI ViewAsSnapshot, 07 section 9.2).
type ViewAsSnapshot struct {
	User PrincipalRef
	// ExpiresAt is the start of the view plus authz.view_as_max_duration.
	ExpiresAt time.Time
	Spaces    []SpaceView
	Channels  []ChannelView
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
	// ExplainAccess answers "who has access" for a space or channel (access.explain, 07 section
	// 9.1, M1-49): every principal with its effective level, capabilities and sources, computed
	// with the snapshot code of authorize. No read on the target is 404, read without owner or
	// manage 403. No step-up, no audit.
	ExplainAccess(ctx context.Context, caller actor.Actor, target AccessTargetRef) (AccessExplanation, error)
	// PreviewGrant shows what CreateGrant of cmd would change (access.preview, 07 section 9.3,
	// M1-49): a dry run in the real transaction that is rolled back. It needs what grant.create
	// needs on the target (404 without read, 403 without owner or manage) but no step-up and
	// writes no audit entry; a delegation limit gives Allowed false with DenialReason.
	PreviewGrant(ctx context.Context, caller actor.Actor, cmd CreateGrantCmd) (GrantPreview, error)
	// ViewAs returns what the person userID sees, as the owner caller sees it with "view as"
	// (access.view_as, 07 section 9.2, M1-49): owner, step-up, audit of allow and deny. An unknown
	// id or a principal that is not a person is 404.
	ViewAs(ctx context.Context, caller actor.Actor, userID ids.ID) (ViewAsSnapshot, error)
	// ClaimSync reconciles the claim groups at an OIDC login (M1-42).
	ClaimSync
}

// SyncClaimGroupsCmd is the input of SyncClaimGroups (brief M1-42).
type SyncClaimGroupsCmd struct {
	// UserID is the person of the login; ProviderID the OIDC provider whose claim groups take part.
	UserID     ids.ID
	ProviderID ids.ID
	// ClaimValues are the values of the group claim; order and duplicates do not matter. They can
	// be names, so they are never logged and never written to audit details.
	ClaimValues []string
	// Now is the request time of the login; MaxAge is auth.group_claim_max_age.
	Now    time.Time
	MaxAge time.Duration
}

// SyncClaimGroupsResult names the groups whose membership the sync added, removed or extended,
// each sorted by id; all empty for a login without change.
type SyncClaimGroupsResult struct {
	Added, Removed, Extended []ids.ID
}

// ErrClaimSyncNeedsRightsChange is returned by SyncClaimGroups inside a unit of work that is not
// declared as a rights change when the claim would change a membership: the caller rolls the unit
// back and runs it again declared with txn.DeclareRightsChange for authorize.ActionGroupMemberSync,
// so permission_version is locked before any domain row (03 section 6.3) and only a change raises
// it (08 section 3.4 item 3).
var ErrClaimSyncNeedsRightsChange = errors.New("authz: the claim sync changes memberships; run the unit as a rights change")

// ClaimSync is the part of the Admin port that identity calls inside the unit of work of
// completeOidcLogin (08 section 3.4, system action group.member.sync).
//
// Example (identity, M1-42):
//
//	res, err := claims.SyncClaimGroups(ctx, authz.SyncClaimGroupsCmd{UserID: person, ProviderID: provider,
//		ClaimValues: groups, Now: now, MaxAge: maxAge})
//	if errors.Is(err, authz.ErrClaimSyncNeedsRightsChange) {
//		// roll back and run the unit again with txn.DeclareRightsChange(group.member.sync)
//	}
type ClaimSync interface {
	// SyncClaimGroups makes the memberships of cmd.UserID in the live oidc_claim groups of
	// cmd.ProviderID follow the claim: matched groups without a membership get one with
	// expires_at = Now + MaxAge, present ones (also expired) are extended to it, memberships whose
	// group the claim no longer names are removed; added_by and removed_by are the person. Local
	// groups and claim groups of other providers are never touched, unknown values are ignored.
	// It runs only inside a unit of work (kind internal otherwise). Without a change it writes
	// nothing. With a change in a unit not declared as a rights change it writes nothing and
	// returns ErrClaimSyncNeedsRightsChange; in a unit declared for group.member.sync it applies
	// the change, appends group.changed per added or removed membership and records one allow
	// entry with the person as actor. An I1 violation then fails the unit in the rights check
	// with an InvariantError (409 i1_violation, hint HintI1ClaimSync) and a deny entry naming
	// provider, groups and agents. Errors: kind internal for a malformed command or a unit
	// declared for another action; store errors.
	SyncClaimGroups(ctx context.Context, cmd SyncClaimGroupsCmd) (SyncClaimGroupsResult, error)
}
