// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// AdminConfig holds the configuration keys of the Admin use cases (M1-48).
type AdminConfig struct {
	// GroupNameMaxChars is group.name_max_chars; GroupDescriptionMaxChars is
	// group.description_max_chars.
	GroupNameMaxChars        int `config:"group.name_max_chars"`
	GroupDescriptionMaxChars int `config:"group.description_max_chars"`
	// PageSizeDefault is api.page_size_default, the page size without limit; PageSizeMax is
	// api.page_size_max, the cap of a larger limit.
	PageSizeDefault int `config:"api.page_size_default"`
	PageSizeMax     int `config:"api.page_size_max"`
}

// Limits returns the group text limits of c.
func (c AdminConfig) Limits() domain.GroupLimits {
	return domain.GroupLimits{NameMaxChars: c.GroupNameMaxChars, DescriptionMaxChars: c.GroupDescriptionMaxChars}
}

// Cursors encodes and checks the opaque cursors of the lists (06 "Paginierung"); adapters/httpapi
// implements it with the codec of platform/httpserver.
type Cursors interface {
	// Encode returns the cursor of key for list, bound to binding.
	Encode(list string, binding []byte, key any) (string, error)
	// Decode checks s against list and binding and fills key; every mismatch is 400
	// cursor_invalid.
	Decode(list string, binding []byte, s string, key any) error
}

// AdminDeps are the collaborators of AdminService.
type AdminDeps struct {
	// UoW runs every use case; its registry holds the rights hooks.
	UoW txn.UnitOfWork
	// Authorizer decides every call before any write.
	Authorizer authorize.Authorizer
	// Store reads and writes groups, memberships and grants.
	Store Store
	// Principals and Structure check subjects and targets.
	Principals authz.PrincipalSource
	Structure  authz.StructureSource
	// Hosts checks host targets (M2-39); nil means a world without hosts.
	Hosts authz.HostSource
	// Rights names the members of a group as affected (RightsHook).
	Rights authz.RightsChanges
	// Audit writes the allow entries inside the transaction.
	Audit auditing.Recorder
	// Clock gives the time of the writes.
	Clock clock.Clock
	// Events appends grant.changed and group.changed in the unit of work (M1-48).
	Events eventlog.Appender
	// Scopes gives the list use cases the owner role and the manage scope of the caller.
	Scopes AdminScoper
	// Cursors encodes the cursors of the lists.
	Cursors Cursors
	// Config holds the group limits and the page sizes.
	Config AdminConfig
	// Engine and Hook are the Authorizer and the rights hook of authz that access.explain,
	// access.preview and access.view_as compute with (M1-49); nil takes Authorizer and Rights
	// when they are these types. Without them those three use cases fail with kind internal.
	Engine *Authorizer
	Hook   *RightsHook
	// Views stores the view of viewAsUser called from a console session (M6-01); nil refuses that
	// call with kind internal, so a console view is never only a snapshot.
	Views authz.ConsoleViews
}

// AdminService implements authz.Admin (brief M1-29 scope 5). It is safe for concurrent use.
type AdminService struct {
	d AdminDeps
}

// Compile-time check: AdminService is the incoming port.
var _ authz.Admin = (*AdminService)(nil)

// NewAdminService returns the service. Errors: a nil dependency, a page size default below 1 or
// above the maximum.
func NewAdminService(d AdminDeps) (*AdminService, error) {
	if d.UoW == nil || d.Authorizer == nil || d.Store == nil || d.Principals == nil || d.Structure == nil ||
		d.Rights == nil || d.Audit == nil || d.Clock == nil || d.Events == nil || d.Scopes == nil || d.Cursors == nil {
		return nil, errors.New("authz: NewAdminService needs every dependency")
	}
	if d.Config.PageSizeDefault < 1 || d.Config.PageSizeMax < d.Config.PageSizeDefault {
		return nil, errors.New("authz: api.page_size_default must be at least 1 and at most api.page_size_max")
	}
	return &AdminService{d: d}, nil
}

// The audit attribute names of the Admin use cases; values are ids and codes, never names.
const (
	attrNoChange      = "no_change"
	attrReason        = "reason"
	attrUserID        = "user_id"
	attrExpiresAt     = "expires_at"
	attrSubjectKind   = "subject_kind"
	attrSubjectID     = "subject_id"
	attrTargetKind    = "target_kind"
	attrTargetID      = "target_id"
	attrLevel         = "level"
	attrCapabilities  = "capabilities"
	attrRevokedGrants = "revoked_grants"
	reasonClaimGroup  = "claim_managed_group"
	// reasonGroupDeleted is the revoke_reason of the grants of a deleted group (brief M1-48).
	reasonGroupDeleted = "group_deleted"
	// codeUnknownSubject and codeUnknownUser are the field codes of a subject or member that
	// does not exist (brief M1-48).
	codeUnknownSubject = "unknown_subject"
	codeUnknownUser    = "unknown_user"
)

// errNotFound is the one answer for a missing or invisible resource (02 section 9.2).
func errNotFound() error { return errs.New(errs.CodeNotFound, "") }

// withCaller puts caller into ctx as the request principal the recorder and the events read; a
// different principal already in ctx is a programming error.
func withCaller(ctx context.Context, caller actor.Actor) (context.Context, error) {
	if have, ok := actor.FromContext(ctx); ok && (have.ID != caller.ID || have.Kind != caller.Kind) {
		return nil, internal("the caller is not the principal of the request")
	}
	return actor.WithActor(ctx, caller), nil
}

// check authorizes act on r and returns the decision as an error (nil for allow).
func (s *AdminService) check(ctx context.Context, caller actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := s.d.Authorizer.Authorize(ctx, caller, act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// record writes the allow entry of act inside the unit of work.
func (s *AdminService) record(ctx context.Context, act authorize.Action, kind authorize.ResourceKind, id ids.ID, attrs auditing.Attrs) error {
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(act), Decision: auditing.DecisionAllow, ResourceKind: string(kind), ResourceID: id, Attrs: attrs,
	})
}

// noChange is the audit attribute of a call without effect (06 section 2 "No effect").
func noChange() auditing.Attrs { return auditing.Attrs{attrNoChange: auditing.Bool(true)} }

// rights declares a rights change of act on resource.
func rights(act authorize.Action, kind authorize.ResourceKind, id ids.ID, principals ...ids.ID) txn.Option {
	return txn.DeclareRightsChange(txn.RightsChangeSpec{
		Action: string(act), Resource: txn.Ref{Kind: string(kind), ID: id}, Principals: principals,
	})
}

// appendEvents appends evs in the unit of work of ctx.
func (s *AdminService) appendEvents(ctx context.Context, evs ...eventlog.Event) error {
	return s.d.Events.Append(ctx, evs...)
}

// groupEvent appends group.changed for g with change and user (zero for none); member_count is
// counted at t after the change of the unit (memberships neither removed nor expired).
func (s *AdminService) groupEvent(ctx context.Context, act authorize.Action, g domain.Group, change string, user ids.ID, t time.Time) error {
	counts, err := s.d.Store.CountMembersInEffect(ctx, []ids.ID{g.ID}, t)
	if err != nil {
		return err
	}
	kind, actorID := eventActor(ctx, string(act))
	ev, err := groupChangedEvent(g, counts[g.ID], change, user, kind, actorID)
	if err != nil {
		return err
	}
	return s.appendEvents(ctx, ev)
}

// grantEvent appends grant.changed for g with change.
func (s *AdminService) grantEvent(ctx context.Context, act authorize.Action, g domain.Grant, change string) error {
	kind, actorID := eventActor(ctx, string(act))
	ev, err := grantChangedEvent(ctx, s.d.Structure, g, change, kind, actorID)
	if err != nil {
		return err
	}
	return s.appendEvents(ctx, ev)
}

// loadGroup returns the group id after authorizing act on it; a missing group is not found, and
// live requires deleted_at NULL. authorize runs also for a missing group, so a caller it refuses
// before visibility (anonymous, a pending device) gets the same answer for an existing and an
// unknown id (07 section 4.2, M1-31/16).
func (s *AdminService) loadGroup(ctx context.Context, caller actor.Actor, act authorize.Action, id ids.ID, live bool) (domain.Group, error) {
	g, found, err := s.d.Store.GetGroup(ctx, id)
	if err != nil {
		return domain.Group{}, err
	}
	if err := s.check(ctx, caller, act, groupResource(id)); err != nil {
		return domain.Group{}, err
	}
	if !found || (live && !g.DeletedAt.IsZero()) {
		return domain.Group{}, errNotFound()
	}
	return g, nil
}

// groupResource is the descriptor of a group.
func groupResource(id ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceGroup, ID: id}
}

// CreateGroup implements authz.Admin: the shape of the command, then authorize, then the insert
// with group.changed (change created) and the audit entry. A claim group names an existing
// provider (the foreign key answers 400 on /provider_id otherwise).
func (s *AdminService) CreateGroup(ctx context.Context, caller actor.Actor, cmd authz.CreateGroupCmd) (authz.Group, error) {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return authz.Group{}, err
	}
	if err := s.groupShape(cmd); err != nil {
		return authz.Group{}, err
	}
	now := s.d.Clock.Now()
	var g domain.Group
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, caller, authorize.ActionGroupCreate, authorize.Resource{Kind: authorize.ResourceInstance}); err != nil {
			return err
		}
		var err error
		if g, err = s.newGroup(caller, cmd, now); err != nil {
			return err
		}
		if err := s.d.Store.InsertGroup(ctx, g); err != nil {
			return err
		}
		if err := s.groupEvent(ctx, authorize.ActionGroupCreate, g, changeCreated, ids.ID{}, now); err != nil {
			return err
		}
		return s.record(ctx, authorize.ActionGroupCreate, authorize.ResourceGroup, g.ID, groupAttrs(g))
	})
	if err != nil {
		return authz.Group{}, err
	}
	return groupOut(g, 0), nil
}

// groupShape checks cmd before authorize, without a creator (brief M1-48 "Groups"; an anonymous
// caller is refused by authorize afterwards, not by a missing creator): the name trimmed, source
// local by default; a local group has neither provider nor claim value, a claim group needs both.
func (s *AdminService) groupShape(cmd authz.CreateGroupCmd) error {
	if err := domain.CheckGroupText(strings.TrimSpace(cmd.Name), cmd.Description, s.d.Config.Limits()); err != nil {
		return err
	}
	switch cmd.Source {
	case "", domain.GroupSourceLocal:
		if !cmd.ProviderID.IsZero() {
			return fieldCode("/provider_id", "forbidden", "a local group has no provider")
		}
		if cmd.ExternalRef != "" {
			return fieldCode("/external_ref", "forbidden", "a local group has no claim value")
		}
		return nil
	case domain.GroupSourceOIDCClaim:
		return domain.CheckClaim(cmd.ProviderID, cmd.ExternalRef)
	}
	return fieldCode("/source", "invalid", "the source must be local or oidc_claim")
}

// newGroup builds the row of cmd, which groupShape accepted, created by caller at now.
func (s *AdminService) newGroup(caller actor.Actor, cmd authz.CreateGroupCmd, now time.Time) (domain.Group, error) {
	name := strings.TrimSpace(cmd.Name)
	if cmd.Source == domain.GroupSourceOIDCClaim {
		return domain.NewClaimGroup(ids.New(), name, cmd.Description, s.d.Config.Limits(), cmd.ProviderID, cmd.ExternalRef, caller.ID, now)
	}
	return domain.NewGroup(ids.New(), name, cmd.Description, s.d.Config.Limits(), caller.ID, now)
}

// groupAttrs are the audit attributes of a new group: source and, for a claim group, provider.
func groupAttrs(g domain.Group) auditing.Attrs {
	attrs := auditing.Attrs{"source": auditing.Code(g.Source)}
	if !g.ProviderID.IsZero() {
		attrs["provider_id"] = auditing.ID(g.ProviderID)
	}
	return attrs
}

// UpdateGroup implements authz.Admin.
func (s *AdminService) UpdateGroup(ctx context.Context, caller actor.Actor, groupID ids.ID, cmd authz.UpdateGroupCmd) (authz.Group, error) {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return authz.Group{}, err
	}
	if cmd.Name == nil && cmd.Description == nil {
		return authz.Group{}, fieldCode("/name", "required", "name or description is required")
	}
	if cmd.Name != nil {
		trimmed := strings.TrimSpace(*cmd.Name)
		cmd.Name = &trimmed
	}
	var out domain.Group
	count := 0
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		if out, err = s.updateGroup(ctx, caller, groupID, cmd); err != nil {
			return err
		}
		counts, err := s.d.Store.CountMembersInEffect(ctx, []ids.ID{out.ID}, s.d.Clock.Now())
		count = counts[out.ID]
		return err
	})
	if err != nil {
		return authz.Group{}, err
	}
	return groupOut(out, count), nil
}

// updateGroup is UpdateGroup inside its unit of work.
func (s *AdminService) updateGroup(ctx context.Context, caller actor.Actor, groupID ids.ID, cmd authz.UpdateGroupCmd) (domain.Group, error) {
	g, err := s.loadGroup(ctx, caller, authorize.ActionGroupUpdate, groupID, true)
	if err != nil {
		return domain.Group{}, err
	}
	name, desc := orValue(cmd.Name, g.Name), orValue(cmd.Description, g.Description)
	if g.Source == domain.GroupSourceOIDCClaim && name != g.Name {
		return domain.Group{}, errs.New(errs.CodeConflict, "The name of a claim group follows its claim.").WithReason(reasonClaimGroup)
	}
	if err := domain.CheckGroupText(name, desc, s.d.Config.Limits()); err != nil {
		return domain.Group{}, err
	}
	if name == g.Name && desc == g.Description {
		return g, s.record(ctx, authorize.ActionGroupUpdate, authorize.ResourceGroup, g.ID, noChange())
	}
	changed, err := s.d.Store.UpdateGroup(ctx, g.ID, name, desc)
	if err != nil {
		return domain.Group{}, err
	}
	if !changed {
		return domain.Group{}, errNotFound()
	}
	g.Name, g.Description = name, desc
	if err := s.groupEvent(ctx, authorize.ActionGroupUpdate, g, changeUpdated, ids.ID{}, s.d.Clock.Now()); err != nil {
		return domain.Group{}, err
	}
	return g, s.record(ctx, authorize.ActionGroupUpdate, authorize.ResourceGroup, g.ID, nil)
}

// orValue returns *p, or def for nil.
func orValue(p *string, def string) string {
	if p == nil {
		return def
	}
	return *p
}

// DeleteGroup implements authz.Admin. A first unit without rights change decides "without
// change" (no rights version, 06 section 2); the change itself runs in a second unit declared as
// a rights change, which checks everything again under the lock of permission_version.
func (s *AdminService) DeleteGroup(ctx context.Context, caller actor.Actor, groupID ids.ID) error {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return err
	}
	return s.twoUnits(ctx, func(ctx context.Context, apply bool) (bool, error) {
		return s.deleteGroup(ctx, caller, groupID, apply)
	}, func() txn.Option { return rights(authorize.ActionGroupDelete, authorize.ResourceGroup, groupID) })
}

// errRaced reports, inside the unit declared as a rights change, that the call found itself
// without effect under the lock (a concurrent call did the change first): the unit rolls back,
// so no rights version is raised, and twoUnits records the call without effect.
var errRaced = errors.New("authz: the change was done concurrently")

// behindStageKey marks a twoUnits call that runs inside an ambient unit of work: the unit of the
// Idempotency-Key stage, opened with txn.AllowLateRightsChange. There a failed late rights change
// fails the whole unit even when the caller swallows the error (M1-31/2), so the raced step
// records the call without effect itself instead of returning errRaced (docs/DECISIONS.md M1-48).
type behindStageKey struct{}

// behindStage reports a twoUnits call inside an ambient unit of work.
func behindStage(ctx context.Context) bool {
	v, _ := ctx.Value(behindStageKey{}).(bool)
	return v
}

// withoutEffect ends a step that finds no change: without apply it records the call without
// effect; with apply (a concurrent call did the change first, seen under the lock) it returns
// errRaced, or behind the stage records the call without effect itself.
func (s *AdminService) withoutEffect(ctx context.Context, apply bool, act authorize.Action, kind authorize.ResourceKind, id ids.ID, attrs auditing.Attrs) error {
	if apply && !behindStage(ctx) {
		return errRaced
	}
	return s.record(ctx, act, kind, id, merge(attrs, noChange()))
}

// twoUnits runs step without apply in a unit without rights change; when step reports a change
// it runs step with apply in a unit declared with opt (evaluated after the first unit). A step
// that finds no change under the lock returns errRaced; then the first unit runs once more and
// records the call without effect (06 section 2, R-API-27). A call that needs a change again
// after that is a conflict: the resource changed during the call.
func (s *AdminService) twoUnits(ctx context.Context, step func(ctx context.Context, apply bool) (bool, error), opt func() txn.Option) error {
	if txn.InUnit(ctx) {
		ctx = context.WithValue(ctx, behindStageKey{}, true)
	}
	plan := func() (bool, error) {
		change := false
		err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
			var err error
			change, err = step(ctx, false)
			return err
		})
		return change, err
	}
	change, err := plan()
	if err != nil || !change {
		return err
	}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		_, err := step(ctx, true)
		return err
	}, opt())
	if !errors.Is(err, errRaced) {
		return err
	}
	if change, err = plan(); err == nil && change {
		return errs.New(errs.CodeConflict, "The resource changed during the call; repeat it.")
	}
	return err
}

// deleteGroup loads and authorizes; without apply it only reports whether a change is due (and
// audits the call without effect); with apply it revokes the grants of the group in ascending id
// with the reason group_deleted, sets deleted_at, appends one grant.changed per grant and one
// group.changed, and audits. The memberships stay as history.
func (s *AdminService) deleteGroup(ctx context.Context, caller actor.Actor, groupID ids.ID, apply bool) (bool, error) {
	g, err := s.loadGroup(ctx, caller, authorize.ActionGroupDelete, groupID, false)
	if err != nil {
		return false, err
	}
	if !g.DeletedAt.IsZero() {
		return false, s.withoutEffect(ctx, apply, authorize.ActionGroupDelete, authorize.ResourceGroup, g.ID, nil)
	}
	if !apply {
		return true, nil
	}
	now := s.d.Clock.Now()
	if err := s.affectGroup(ctx, domain.Subject{Kind: domain.SubjectGroup, GroupID: g.ID}); err != nil {
		return false, err
	}
	grants, err := s.revokeGroupGrants(ctx, caller, g.ID, now)
	if err != nil {
		return false, err
	}
	if _, err := s.d.Store.SoftDeleteGroup(ctx, g.ID, now); err != nil {
		return false, err
	}
	g.DeletedAt = now
	if err := s.groupEvent(ctx, authorize.ActionGroupDelete, g, changeDeleted, ids.ID{}, now); err != nil {
		return false, err
	}
	return true, s.record(ctx, authorize.ActionGroupDelete, authorize.ResourceGroup, g.ID,
		auditing.Attrs{attrRevokedGrants: auditing.Int(int64(grants)), attrReason: auditing.ReasonText(reasonGroupDeleted)})
}

// revokeGroupGrants revokes the grants of the group in effect at now in ascending id with the
// reason group_deleted, appends grant.changed for each and returns how many it revoked.
func (s *AdminService) revokeGroupGrants(ctx context.Context, caller actor.Actor, group ids.ID, now time.Time) (int, error) {
	grants, err := s.d.Store.ActiveGrantsOfGroups(ctx, []ids.ID{group})
	if err != nil {
		return 0, err
	}
	n := 0
	for _, gr := range grants {
		if !gr.InEffect(now) {
			// An expired grant changes no rights; the expiry fold of this change reports it.
			continue
		}
		revoked, ok, err := s.d.Store.RevokeGrant(ctx, gr.ID, caller.ID, reasonGroupDeleted, now)
		if err != nil {
			return 0, err
		}
		if !ok {
			continue
		}
		n++
		if err := s.grantEvent(ctx, authorize.ActionGroupDelete, revoked, changeRevoked); err != nil {
			return 0, err
		}
	}
	return n, nil
}

// AddGroupMember implements authz.Admin. A membership that is not removed but expired is closed
// (removed_at) before the new one, because the unique index allows one row without removed_at
// per person and group (docs/DECISIONS.md M1-29).
func (s *AdminService) AddGroupMember(ctx context.Context, caller actor.Actor, groupID ids.ID, cmd authz.AddMemberCmd) (authz.Membership, error) {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return authz.Membership{}, err
	}
	var out domain.Membership
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.addMember(ctx, caller, groupID, cmd)
		return err
	}, rights(authorize.ActionGroupMemberAdd, authorize.ResourceGroup, groupID, cmd.UserID))
	if err != nil {
		return authz.Membership{}, err
	}
	refs, err := s.refs(ctx, []ids.ID{out.UserID})
	if err != nil {
		return authz.Membership{}, err
	}
	return membershipOut(out, refs[out.UserID]), nil
}

// refs returns the public views of people; a person the source does not know is an internal
// error, because a membership row always names a principal (foreign key to users).
func (s *AdminService) refs(ctx context.Context, people []ids.ID) (map[ids.ID]authz.PrincipalRef, error) {
	out, err := s.d.Principals.Refs(ctx, people)
	if err != nil {
		return nil, sourceError("principal source", err)
	}
	for _, id := range people {
		if _, ok := out[id]; !ok {
			return nil, internal("a member without a principal")
		}
	}
	return out, nil
}

// addMember is AddGroupMember inside its unit of work.
func (s *AdminService) addMember(ctx context.Context, caller actor.Actor, groupID ids.ID, cmd authz.AddMemberCmd) (domain.Membership, error) {
	g, err := s.loadGroup(ctx, caller, authorize.ActionGroupMemberAdd, groupID, true)
	if err != nil {
		return domain.Membership{}, err
	}
	if g.Source == domain.GroupSourceOIDCClaim {
		return domain.Membership{}, errs.New(errs.CodeConflict, "The members of a claim group follow its claim.").WithReason(reasonClaimGroup)
	}
	if err := s.person(ctx, cmd.UserID); err != nil {
		return domain.Membership{}, err
	}
	now := s.d.Clock.Now()
	m, err := domain.NewMembership(ids.New(), g.ID, cmd.UserID, caller.ID, cmd.ExpiresAt, now)
	if err != nil {
		return domain.Membership{}, err
	}
	if err := s.closeExpired(ctx, g, cmd.UserID, caller.ID, now); err != nil {
		return domain.Membership{}, err
	}
	if err := s.d.Store.InsertMembership(ctx, m); err != nil {
		return domain.Membership{}, err
	}
	attrs := auditing.Attrs{attrUserID: auditing.ID(cmd.UserID)}
	if !m.ExpiresAt.IsZero() {
		attrs[attrExpiresAt] = auditing.Time(m.ExpiresAt)
	}
	if err := s.groupEvent(ctx, authorize.ActionGroupMemberAdd, g, changeMemberAdded, cmd.UserID, now); err != nil {
		return domain.Membership{}, err
	}
	return m, s.record(ctx, authorize.ActionGroupMemberAdd, authorize.ResourceGroup, g.ID, attrs)
}

// closeExpired removes the expired memberships of user in g that are not removed yet; a
// membership in effect is a conflict.
func (s *AdminService) closeExpired(ctx context.Context, g domain.Group, user, by ids.ID, now time.Time) error {
	history, err := s.d.Store.MembershipsOfUserInGroup(ctx, g.ID, user)
	if err != nil {
		return err
	}
	for _, old := range history {
		if !old.RemovedAt.IsZero() {
			continue
		}
		if old.InEffect(g, now) {
			return errs.New(errs.CodeConflict, "The person is already a member.")
		}
		if _, err := s.d.Store.RemoveMembership(ctx, old.ID, by, now); err != nil {
			return err
		}
	}
	return nil
}

// person checks that id is a principal of kind user that is not deleted; otherwise 400
// validation_failed on /user_id with code unknown_user (brief M1-48), decided after authorize.
func (s *AdminService) person(ctx context.Context, id ids.ID) error {
	f, found, err := s.d.Principals.Principal(ctx, id)
	if err != nil {
		return sourceError("principal source", err)
	}
	if !found || f.Kind != actor.KindUser || f.Status == authz.PrincipalDeleted {
		return fieldCode("/user_id", codeUnknownUser, "the user does not exist")
	}
	return nil
}

// RemoveGroupMember implements authz.Admin with the two units of DeleteGroup (twoUnits).
func (s *AdminService) RemoveGroupMember(ctx context.Context, caller actor.Actor, groupID, userID ids.ID) error {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return err
	}
	return s.twoUnits(ctx, func(ctx context.Context, apply bool) (bool, error) {
		return s.removeMember(ctx, caller, groupID, userID, apply)
	}, func() txn.Option {
		return rights(authorize.ActionGroupMemberRemove, authorize.ResourceGroup, groupID, userID)
	})
}

// removeMember loads and authorizes; a person that never was a member is not found, a
// membership already removed or expired is a call without effect; with apply it removes the
// membership in effect.
func (s *AdminService) removeMember(ctx context.Context, caller actor.Actor, groupID, userID ids.ID, apply bool) (bool, error) {
	g, err := s.loadGroup(ctx, caller, authorize.ActionGroupMemberRemove, groupID, true)
	if err != nil {
		return false, err
	}
	if g.Source == domain.GroupSourceOIDCClaim {
		return false, errs.New(errs.CodeConflict, "The members of a claim group follow its claim.").WithReason(reasonClaimGroup)
	}
	history, err := s.d.Store.MembershipsOfUserInGroup(ctx, g.ID, userID)
	if err != nil {
		return false, err
	}
	if len(history) == 0 {
		return false, errNotFound()
	}
	now := s.d.Clock.Now()
	var active *domain.Membership
	for i := range history {
		if history[i].InEffect(g, now) {
			active = &history[i]
		}
	}
	attrs := auditing.Attrs{attrUserID: auditing.ID(userID)}
	if active == nil {
		return false, s.withoutEffect(ctx, apply, authorize.ActionGroupMemberRemove, authorize.ResourceGroup, g.ID, attrs)
	}
	if !apply {
		return true, nil
	}
	if _, err := s.d.Store.RemoveMembership(ctx, active.ID, caller.ID, now); err != nil {
		return false, err
	}
	if err := s.groupEvent(ctx, authorize.ActionGroupMemberRemove, g, changeMemberRemoved, userID, now); err != nil {
		return false, err
	}
	return true, s.record(ctx, authorize.ActionGroupMemberRemove, authorize.ResourceGroup, g.ID, attrs)
}

// CreateGrant implements authz.Admin (brief M1-48 "createGrant"): the shape first, then the
// authorization step (authorizeGrant) with the proposed grant, then the apply step (createGrant:
// target, subject, insert, grant.changed with change created and the audit entry) in one rights
// change. access.preview runs the same apply step in the same rights change and rolls it back
// (M1-49).
func (s *AdminService) CreateGrant(ctx context.Context, caller actor.Actor, cmd authz.CreateGrantCmd) (authz.Grant, error) {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return authz.Grant{}, err
	}
	subject, target, level, caps, err := grantParts(cmd)
	if err != nil {
		return authz.Grant{}, err
	}
	now := s.d.Clock.Now()
	reason := strings.TrimSpace(cmd.Reason)
	if err := domain.CheckGrantShape(subject, target, level, caps, cmd.ExpiresAt, reason, now); err != nil {
		return authz.Grant{}, err
	}
	proposed := domain.Grant{Subject: subject, Target: target, Level: level, Caps: caps}
	var g domain.Grant
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.authorizeGrant(ctx, caller, cmd.TargetID, proposed); err != nil {
			return err
		}
		var err error
		if g, err = domain.NewGrant(ids.New(), subject, target, level, caps, cmd.ExpiresAt, reason, caller.ID, now); err != nil {
			return err
		}
		return s.createGrant(ctx, g)
	}, grantRightsChange(subject, target, cmd.TargetID))
	if err != nil {
		return authz.Grant{}, err
	}
	return grantOut(g), nil
}

// authorizeGrant authorizes grant.create of the proposed grant on its target (visibility, owner or
// manage, the delegation limits, step-up) before the target is checked, so a caller refused before
// visibility learns nothing about the target; a missing target is not_found from authorize.
func (s *AdminService) authorizeGrant(ctx context.Context, caller actor.Actor, targetID ids.ID, g domain.Grant) error {
	kind, ok := authorize.ParseResourceKind(string(g.Target.Kind))
	if !ok {
		return internal("a grant target kind outside the catalogue")
	}
	res := authorize.Resource{Kind: kind, ID: targetID, Grant: &authorize.GrantFacts{
		SubjectKind: string(g.Subject.Kind), Level: g.Level.String(), Caps: g.Caps.Names(), TargetKind: string(g.Target.Kind),
		TargetSpaceID: g.Target.SpaceID, TargetChannelID: g.Target.ChannelID, TargetHostID: g.Target.HostID,
	}}
	return s.check(ctx, caller, authorize.ActionGrantCreate, res)
}

// createGrant is CreateGrant inside its unit of work after authorize: target, archived target,
// subject, the members of a subject group, the insert, the event and the audit.
func (s *AdminService) createGrant(ctx context.Context, g domain.Grant) error {
	if err := s.targetExists(ctx, g.Target); err != nil {
		return err
	}
	if err := s.targetLive(ctx, g.Target); err != nil {
		return err
	}
	if err := s.subjectExists(ctx, g.Subject); err != nil {
		return err
	}
	if err := s.affectGroup(ctx, g.Subject); err != nil {
		return err
	}
	if err := s.d.Store.InsertGrant(ctx, g); err != nil {
		return err
	}
	if err := s.grantEvent(ctx, authorize.ActionGrantCreate, g, changeCreated); err != nil {
		return err
	}
	return s.record(ctx, authorize.ActionGrantCreate, authorize.ResourceGrant, g.ID, grantAttrs(g))
}

// grantParts parses the column values of cmd: subject and target with their ids, level and
// capabilities. Errors: validation_failed on /subject, /target, /level or /capabilities.
func grantParts(cmd authz.CreateGrantCmd) (domain.Subject, domain.Target, domain.Level, domain.Caps, error) {
	subject := domain.Subject{Kind: domain.SubjectKind(cmd.SubjectKind)}
	switch subject.Kind {
	case domain.SubjectGroup:
		subject.GroupID = cmd.SubjectID
	case domain.SubjectUser, domain.SubjectAgent, domain.SubjectApp:
		subject.PrincipalID = cmd.SubjectID
	default:
		return domain.Subject{}, domain.Target{}, 0, 0, fieldErr("/subject_kind", "the subject kind must be user, group, agent or app")
	}
	if cmd.SubjectID.IsZero() {
		return domain.Subject{}, domain.Target{}, 0, 0, fieldCode("/subject_id", "required", "the subject needs an id")
	}
	target := domain.Target{Kind: domain.TargetKind(cmd.TargetKind)}
	switch target.Kind {
	case domain.TargetInstance:
		if !cmd.TargetID.IsZero() {
			return domain.Subject{}, domain.Target{}, 0, 0, fieldCode("/target_id", "forbidden", "the instance has no id")
		}
	case domain.TargetSpace:
		target.SpaceID = cmd.TargetID
	case domain.TargetChannel:
		target.ChannelID = cmd.TargetID
	case domain.TargetHost:
		target.HostID = cmd.TargetID
	default:
		return domain.Subject{}, domain.Target{}, 0, 0, fieldErr("/target_kind", "the target kind must be instance, space, channel or host")
	}
	if target.Kind != domain.TargetInstance && cmd.TargetID.IsZero() {
		return domain.Subject{}, domain.Target{}, 0, 0, fieldCode("/target_id", "required", "the target needs an id")
	}
	level := domain.LevelNone
	if cmd.Level != "" {
		l, err := domain.ParseLevel(cmd.Level)
		if err != nil {
			return domain.Subject{}, domain.Target{}, 0, 0, fieldErr("/level", "the level must be read, write or trigger")
		}
		level = l
	}
	caps, err := domain.ParseCaps(cmd.Capabilities)
	if err != nil {
		return domain.Subject{}, domain.Target{}, 0, 0, fieldErr("/capabilities", "unknown capability")
	}
	if len(slices.Compact(slices.Sorted(slices.Values(cmd.Capabilities)))) != len(cmd.Capabilities) {
		return domain.Subject{}, domain.Target{}, 0, 0, fieldCode("/capabilities", "duplicate", "every capability at most once")
	}
	return subject, target, level, caps, nil
}

// fieldErr is validation_failed with one invalid field.
func fieldErr(pointer, detail string) error { return fieldCode(pointer, "invalid", detail) }

// fieldCode is validation_failed with one field error of code.
func fieldCode(pointer, code, detail string) error {
	return errs.New(errs.CodeValidationFailed, "").WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// targetExists reports a missing target as not found; a host is read from the host source of
// the terminal module (M2-39).
func (s *AdminService) targetExists(ctx context.Context, tg domain.Target) error {
	switch tg.Kind {
	case domain.TargetInstance:
		return nil
	case domain.TargetSpace:
		ok, err := s.d.Structure.Space(ctx, tg.SpaceID)
		if err != nil {
			return sourceError("structure source", err)
		}
		if !ok {
			return errNotFound()
		}
		return nil
	case domain.TargetChannel:
		c, ok, err := s.d.Structure.Channel(ctx, tg.ChannelID)
		if err != nil {
			return sourceError("structure source", err)
		}
		if !ok || c.Kind == authz.ChannelDM {
			return errNotFound()
		}
		return nil
	case domain.TargetHost:
		_, ok, err := hostsOrNone(s.d.Hosts).Host(ctx, tg.HostID)
		if err != nil {
			return sourceError("host source", err)
		}
		if !ok {
			return errNotFound()
		}
		return nil
	}
	return errNotFound()
}

// targetLive refuses an archived space or channel and a retired host with 409 conflict (after
// authorize, so an invisible target stays not found). A retired host is final (24 section 3.1),
// so a grant on it could never work (M2-39).
func (s *AdminService) targetLive(ctx context.Context, tg domain.Target) error {
	archived := false
	switch tg.Kind {
	case domain.TargetSpace:
		a, err := s.d.Structure.SpaceArchived(ctx, tg.SpaceID)
		if err != nil {
			return sourceError("structure source", err)
		}
		archived = a
	case domain.TargetChannel:
		c, _, err := s.d.Structure.Channel(ctx, tg.ChannelID)
		if err != nil {
			return sourceError("structure source", err)
		}
		archived = c.Archived
	case domain.TargetHost:
		h, _, err := hostsOrNone(s.d.Hosts).Host(ctx, tg.HostID)
		if err != nil {
			return sourceError("host source", err)
		}
		if h.State == authz.HostRetired {
			return errs.New(errs.CodeConflict, "The host is retired.")
		}
	case domain.TargetInstance:
	}
	if archived {
		return errs.New(errs.CodeConflict, "The target is archived.").WithReason(string(auditing.ReasonChannelArchived))
	}
	return nil
}

// subjectExists checks the subject: a live group, or a principal of the subject's kind that is
// not deleted; otherwise 400 validation_failed on /subject_id with code unknown_subject. It runs
// after authorize, so a caller without rights learns nothing about ids (brief M1-48).
func (s *AdminService) subjectExists(ctx context.Context, sub domain.Subject) error {
	unknown := fieldCode("/subject_id", codeUnknownSubject, "the subject does not exist")
	if sub.Kind == domain.SubjectGroup {
		g, found, err := s.d.Store.GetGroup(ctx, sub.GroupID)
		if err != nil {
			return err
		}
		if !found || !g.DeletedAt.IsZero() {
			return unknown
		}
		return nil
	}
	f, found, err := s.d.Principals.Principal(ctx, sub.PrincipalID)
	if err != nil {
		return sourceError("principal source", err)
	}
	if !found || string(f.Kind) != string(sub.Kind) || f.Status == authz.PrincipalDeleted {
		return unknown
	}
	return nil
}

// affectGroup names the members of a subject group as affected before the grant row changes.
func (s *AdminService) affectGroup(ctx context.Context, sub domain.Subject) error {
	if sub.Kind != domain.SubjectGroup {
		return nil
	}
	members, err := s.d.Store.ActiveMembershipsOfGroups(ctx, []ids.ID{sub.GroupID})
	if err != nil {
		return err
	}
	list := make([]ids.ID, 0, len(members))
	for _, m := range members {
		list = append(list, m.Membership.UserID)
	}
	return s.d.Rights.AddAffected(ctx, list...)
}

// RevokeGrant implements authz.Admin with the two units of DeleteGroup (twoUnits).
func (s *AdminService) RevokeGrant(ctx context.Context, caller actor.Actor, grantID ids.ID, reason string) (authz.Grant, error) {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return authz.Grant{}, err
	}
	reason = strings.TrimSpace(reason)
	if err := domain.CheckReason(reason); err != nil {
		return authz.Grant{}, err
	}
	attrs, err := reasonAttrs(reason)
	if err != nil {
		return authz.Grant{}, err
	}
	var out domain.Grant
	err = s.twoUnits(ctx, func(ctx context.Context, apply bool) (bool, error) {
		g, change, err := s.revokeGrant(ctx, caller, grantID, reason, attrs, apply)
		if err == nil {
			out = g
		}
		return change, err
	}, func() txn.Option {
		var principals []ids.ID
		if out.Subject.Kind != domain.SubjectGroup {
			principals = append(principals, out.Subject.PrincipalID)
		}
		return rights(authorize.ActionGrantRevoke, authorize.ResourceGrant, grantID, principals...)
	})
	return grantOut(out), err
}

// RevokeSubjectGrants implements authz.Admin (M2-24 design note 9 d).
func (s *AdminService) RevokeSubjectGrants(ctx context.Context, caller actor.Actor, subjectID ids.ID, reason string) (int, error) {
	if !txn.InRightsChange(ctx) {
		return 0, internal("RevokeSubjectGrants needs a unit of work declared as a rights change")
	}
	if _, err := reasonAttrs(reason); err != nil {
		return 0, err
	}
	grants, err := s.d.Store.ActiveGrantsOfPrincipal(ctx, subjectID)
	if err != nil {
		return 0, err
	}
	now := s.d.Clock.Now()
	n := 0
	for _, g := range grants {
		revoked, ok, err := s.d.Store.RevokeGrant(ctx, g.ID, caller.ID, reason, now)
		if err != nil {
			return n, err
		}
		if !ok {
			continue
		}
		n++
		// Every grant write appends grant.changed (M1-48), also this cascade (INT-W19/9).
		if err := s.grantEvent(ctx, authorize.ActionAppUninstall, revoked, changeRevoked); err != nil {
			return n, err
		}
	}
	return n, nil
}

// revokeGrant loads and authorizes (07 section 2.6 point 3 through the guard); a revoked grant is
// a call without effect; with apply it revokes.
func (s *AdminService) revokeGrant(ctx context.Context, caller actor.Actor, grantID ids.ID, reason string, attrs auditing.Attrs, apply bool) (domain.Grant, bool, error) {
	g, found, err := s.d.Store.GetGrant(ctx, grantID)
	if err != nil {
		return domain.Grant{}, false, err
	}
	// authorize also for an unknown id: it answers not_found after the liveness checks (07 4.2).
	if err := s.check(ctx, caller, authorize.ActionGrantRevoke, authorize.Resource{Kind: authorize.ResourceGrant, ID: grantID}); err != nil {
		return domain.Grant{}, false, err
	}
	if !found {
		return domain.Grant{}, false, errNotFound()
	}
	if !g.RevokedAt.IsZero() {
		if err := s.withoutEffect(ctx, apply, authorize.ActionGrantRevoke, authorize.ResourceGrant, g.ID, attrs); err != nil {
			return domain.Grant{}, false, err
		}
		return g, false, nil
	}
	if !apply {
		return g, true, nil
	}
	if err := s.affectGroup(ctx, g.Subject); err != nil {
		return domain.Grant{}, false, err
	}
	revoked, ok, err := s.d.Store.RevokeGrant(ctx, g.ID, caller.ID, reason, s.d.Clock.Now())
	if err != nil {
		return domain.Grant{}, false, err
	}
	if !ok {
		// The lock of permission_version serializes revocations, so this needs a writer outside
		// the rights path; answer like the raced case.
		return domain.Grant{}, false, errRaced
	}
	if err := s.grantEvent(ctx, authorize.ActionGrantRevoke, revoked, changeRevoked); err != nil {
		return domain.Grant{}, false, err
	}
	return revoked, true, s.record(ctx, authorize.ActionGrantRevoke, authorize.ResourceGrant, g.ID, attrs)
}

// reasonAttrs checks an optional reason (SEC-IN-4) and returns its audit attribute.
func reasonAttrs(reason string) (auditing.Attrs, error) {
	attrs := auditing.Attrs{}
	if reason == "" {
		return attrs, nil
	}
	if err := textcheck.DisplayText(reason); err != nil {
		return nil, fieldErr("/reason", "the reason contains a forbidden character")
	}
	attrs[attrReason] = auditing.ReasonText(reason)
	return attrs, nil
}

// grantAttrs are the audit attributes of a new grant: ids and codes, the reason as ReasonText.
func grantAttrs(g domain.Grant) auditing.Attrs {
	attrs := auditing.Attrs{
		attrSubjectKind: auditing.Code(string(g.Subject.Kind)),
		attrTargetKind:  auditing.Code(string(g.Target.Kind)),
		attrReason:      auditing.ReasonText(g.Reason),
	}
	if g.Subject.Kind == domain.SubjectGroup {
		attrs[attrSubjectID] = auditing.ID(g.Subject.GroupID)
	} else {
		attrs[attrSubjectID] = auditing.ID(g.Subject.PrincipalID)
	}
	if id := targetID(g.Target); !id.IsZero() {
		attrs[attrTargetID] = auditing.ID(id)
	}
	if g.Level != domain.LevelNone {
		attrs[attrLevel] = auditing.Code(g.Level.String())
	}
	if names := g.Caps.Names(); len(names) > 0 {
		vs := make([]auditing.Value, 0, len(names))
		for _, n := range names {
			vs = append(vs, auditing.Code(n))
		}
		attrs[attrCapabilities] = auditing.List(vs...)
	}
	if !g.ExpiresAt.IsZero() {
		attrs[attrExpiresAt] = auditing.Time(g.ExpiresAt)
	}
	return attrs
}

// merge returns a with the members of b.
func merge(a, b auditing.Attrs) auditing.Attrs {
	out := auditing.Attrs{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// targetID returns the id of a target, zero for the instance.
func targetID(t domain.Target) ids.ID {
	switch t.Kind {
	case domain.TargetSpace:
		return t.SpaceID
	case domain.TargetChannel:
		return t.ChannelID
	case domain.TargetHost:
		return t.HostID
	case domain.TargetInstance:
	}
	return ids.ID{}
}

// groupOut converts a group with its member_count for the port.
func groupOut(g domain.Group, count int) authz.Group {
	return authz.Group{ID: g.ID, Name: g.Name, Description: g.Description, Source: g.Source, ProviderID: g.ProviderID,
		ExternalRef: g.ExternalRef, MemberCount: count, CreatedBy: g.CreatedBy, CreatedAt: g.CreatedAt, DeletedAt: g.DeletedAt}
}

// membershipOut converts a membership with the public view of its member for the port.
func membershipOut(m domain.Membership, user authz.PrincipalRef) authz.Membership {
	return authz.Membership{ID: m.ID, GroupID: m.GroupID, UserID: m.UserID, User: user, AddedBy: m.AddedBy, AddedAt: m.AddedAt,
		ExpiresAt: m.ExpiresAt}
}

// grantOut converts a grant for the port.
func grantOut(g domain.Grant) authz.Grant {
	subject := g.Subject.PrincipalID
	if g.Subject.Kind == domain.SubjectGroup {
		subject = g.Subject.GroupID
	}
	return authz.Grant{
		ID: g.ID, SubjectKind: string(g.Subject.Kind), SubjectID: subject, TargetKind: string(g.Target.Kind), TargetID: targetID(g.Target),
		Level: g.Level.String(), Capabilities: g.Caps.Names(), ExpiresAt: g.ExpiresAt, Reason: g.Reason, GrantedBy: g.GrantedBy,
		GrantedAt: g.GrantedAt, RevokedAt: g.RevokedAt, RevokedBy: g.RevokedBy, RevokeReason: g.RevokeReason,
	}
}
