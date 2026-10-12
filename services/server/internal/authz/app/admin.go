// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

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
	// Rights names the members of a group as affected (RightsHook).
	Rights authz.RightsChanges
	// Audit writes the allow entries inside the transaction.
	Audit auditing.Recorder
	// Clock gives the time of the writes.
	Clock clock.Clock
	// Limits are group.name_max_chars and group.description_max_chars; M1-48 binds them.
	Limits domain.GroupLimits
}

// AdminService implements authz.Admin (brief M1-29 scope 5). It is safe for concurrent use.
type AdminService struct {
	d AdminDeps
}

// Compile-time check: AdminService is the incoming port.
var _ authz.Admin = (*AdminService)(nil)

// NewAdminService returns the service. Errors: a nil dependency.
func NewAdminService(d AdminDeps) (*AdminService, error) {
	if d.UoW == nil || d.Authorizer == nil || d.Store == nil || d.Principals == nil || d.Structure == nil ||
		d.Rights == nil || d.Audit == nil || d.Clock == nil {
		return nil, errors.New("authz: NewAdminService needs every dependency")
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
	attrGroupID       = "group_id"
	reasonClaimGroup  = "claim_managed_group"
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

// noChange is the audit attribute of a call without effect (06 section 2 "Ohne Wirkung").
func noChange() auditing.Attrs { return auditing.Attrs{attrNoChange: auditing.Bool(true)} }

// rights declares a rights change of act on resource.
func rights(act authorize.Action, kind authorize.ResourceKind, id ids.ID, principals ...ids.ID) txn.Option {
	return txn.DeclareRightsChange(txn.RightsChangeSpec{
		Action: string(act), Resource: txn.Ref{Kind: string(kind), ID: id}, Principals: principals,
	})
}

// loadGroup returns the group id; a missing group is not found. live requires deleted_at NULL.
func (s *AdminService) loadGroup(ctx context.Context, id ids.ID, live bool) (domain.Group, error) {
	g, found, err := s.d.Store.GetGroup(ctx, id)
	if err != nil {
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

// CreateGroup implements authz.Admin.
func (s *AdminService) CreateGroup(ctx context.Context, caller actor.Actor, cmd authz.CreateGroupCmd) (authz.Group, error) {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return authz.Group{}, err
	}
	var out domain.Group
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, caller, authorize.ActionGroupCreate, authorize.Resource{Kind: authorize.ResourceInstance}); err != nil {
			return err
		}
		g, err := domain.NewGroup(ids.New(), cmd.Name, cmd.Description, s.d.Limits, caller.ID, s.d.Clock.Now())
		if err != nil {
			return err
		}
		if err := s.d.Store.InsertGroup(ctx, g); err != nil {
			return err
		}
		out = g
		return s.record(ctx, authorize.ActionGroupCreate, authorize.ResourceGroup, g.ID, nil)
	})
	return groupOut(out), err
}

// UpdateGroup implements authz.Admin.
func (s *AdminService) UpdateGroup(ctx context.Context, caller actor.Actor, groupID ids.ID, cmd authz.UpdateGroupCmd) (authz.Group, error) {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return authz.Group{}, err
	}
	var out domain.Group
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.updateGroup(ctx, caller, groupID, cmd)
		return err
	})
	return groupOut(out), err
}

// updateGroup is UpdateGroup inside its unit of work.
func (s *AdminService) updateGroup(ctx context.Context, caller actor.Actor, groupID ids.ID, cmd authz.UpdateGroupCmd) (domain.Group, error) {
	g, err := s.loadGroup(ctx, groupID, true)
	if err != nil {
		return domain.Group{}, err
	}
	if err := s.check(ctx, caller, authorize.ActionGroupUpdate, groupResource(g.ID)); err != nil {
		return domain.Group{}, err
	}
	name, desc := orValue(cmd.Name, g.Name), orValue(cmd.Description, g.Description)
	if g.Source == domain.GroupSourceOIDCClaim && name != g.Name {
		return domain.Group{}, errs.New(errs.CodeConflict, "The name of a claim group follows its claim.").WithReason(reasonClaimGroup)
	}
	if err := domain.CheckGroupText(name, desc, s.d.Limits); err != nil {
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
func (s *AdminService) DeleteGroup(ctx context.Context, caller actor.Actor, groupID ids.ID, reason string) error {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return err
	}
	attrs, err := reasonAttrs(reason)
	if err != nil {
		return err
	}
	return s.twoUnits(ctx, func(ctx context.Context, apply bool) (bool, error) {
		return s.deleteGroup(ctx, caller, groupID, reason, attrs, apply)
	}, func() txn.Option { return rights(authorize.ActionGroupDelete, authorize.ResourceGroup, groupID) })
}

// errRaced reports, inside the unit declared as a rights change, that the call found itself
// without effect under the lock (a concurrent call did the change first): the unit rolls back,
// so no rights version is raised, and twoUnits records the call without effect.
var errRaced = errors.New("authz: the change was done concurrently")

// twoUnits runs step without apply in a unit without rights change; when step reports a change
// it runs step with apply in a unit declared with opt (evaluated after the first unit). A step
// that finds no change under the lock returns errRaced; then the first unit runs once more and
// records the call without effect (06 section 2, R-API-27). A call that needs a change again
// after that is a conflict: the resource changed during the call.
func (s *AdminService) twoUnits(ctx context.Context, step func(ctx context.Context, apply bool) (bool, error), opt func() txn.Option) error {
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
// audits the call without effect); with apply it revokes the grants of the group in ascending id,
// sets deleted_at and audits.
func (s *AdminService) deleteGroup(ctx context.Context, caller actor.Actor, groupID ids.ID, reason string, attrs auditing.Attrs, apply bool) (bool, error) {
	g, err := s.loadGroup(ctx, groupID, false)
	if err != nil {
		return false, err
	}
	if err := s.check(ctx, caller, authorize.ActionGroupDelete, groupResource(g.ID)); err != nil {
		return false, err
	}
	if !g.DeletedAt.IsZero() {
		if apply {
			return false, errRaced
		}
		return false, s.record(ctx, authorize.ActionGroupDelete, authorize.ResourceGroup, g.ID, merge(attrs, noChange()))
	}
	if !apply {
		return true, nil
	}
	now := s.d.Clock.Now()
	members, err := s.d.Store.ActiveMembershipsOfGroups(ctx, []ids.ID{g.ID})
	if err != nil {
		return false, err
	}
	affected := make([]ids.ID, 0, len(members))
	for _, m := range members {
		affected = append(affected, m.Membership.UserID)
	}
	if err := s.d.Rights.AddAffected(ctx, affected...); err != nil {
		return false, err
	}
	grants, err := s.d.Store.ActiveGrantsOfGroups(ctx, []ids.ID{g.ID})
	if err != nil {
		return false, err
	}
	for _, gr := range grants {
		if _, _, err := s.d.Store.RevokeGrant(ctx, gr.ID, caller.ID, reason, now); err != nil {
			return false, err
		}
	}
	if _, err := s.d.Store.SoftDeleteGroup(ctx, g.ID, now); err != nil {
		return false, err
	}
	return true, s.record(ctx, authorize.ActionGroupDelete, authorize.ResourceGroup, g.ID,
		merge(attrs, auditing.Attrs{attrRevokedGrants: auditing.Int(int64(len(grants)))}))
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
	return membershipOut(out), err
}

// addMember is AddGroupMember inside its unit of work.
func (s *AdminService) addMember(ctx context.Context, caller actor.Actor, groupID ids.ID, cmd authz.AddMemberCmd) (domain.Membership, error) {
	g, err := s.loadGroup(ctx, groupID, true)
	if err != nil {
		return domain.Membership{}, err
	}
	if err := s.check(ctx, caller, authorize.ActionGroupMemberAdd, groupResource(g.ID)); err != nil {
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

// person checks that id is a principal of kind user that is not deleted; otherwise not found.
func (s *AdminService) person(ctx context.Context, id ids.ID) error {
	f, found, err := s.d.Principals.Principal(ctx, id)
	if err != nil {
		return sourceError("principal source", err)
	}
	if !found || f.Kind != actor.KindUser || f.Status == authz.PrincipalDeleted {
		return errNotFound()
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
	g, err := s.loadGroup(ctx, groupID, true)
	if err != nil {
		return false, err
	}
	if err := s.check(ctx, caller, authorize.ActionGroupMemberRemove, groupResource(g.ID)); err != nil {
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
		if apply {
			return false, errRaced
		}
		return false, s.record(ctx, authorize.ActionGroupMemberRemove, authorize.ResourceGroup, g.ID, merge(attrs, noChange()))
	}
	if !apply {
		return true, nil
	}
	if _, err := s.d.Store.RemoveMembership(ctx, active.ID, caller.ID, now); err != nil {
		return false, err
	}
	return true, s.record(ctx, authorize.ActionGroupMemberRemove, authorize.ResourceGroup, g.ID, attrs)
}

// CreateGrant implements authz.Admin.
func (s *AdminService) CreateGrant(ctx context.Context, caller actor.Actor, cmd authz.CreateGrantCmd) (authz.Grant, error) {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return authz.Grant{}, err
	}
	subject, target, level, caps, err := grantParts(cmd)
	if err != nil {
		return authz.Grant{}, err
	}
	var principals []ids.ID
	if subject.Kind != domain.SubjectGroup {
		principals = append(principals, subject.PrincipalID)
	}
	var out domain.Grant
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.createGrant(ctx, caller, cmd, subject, target, level, caps)
		return err
	}, txn.DeclareRightsChange(txn.RightsChangeSpec{
		Action: string(authorize.ActionGrantCreate), Resource: txn.Ref{Kind: string(target.Kind), ID: cmd.TargetID}, Principals: principals,
	}))
	return grantOut(out), err
}

// createGrant is CreateGrant inside its unit of work: target, authorize, archived target,
// subject, the new row, the members of a subject group, the insert and the audit.
func (s *AdminService) createGrant(ctx context.Context, caller actor.Actor, cmd authz.CreateGrantCmd, subject domain.Subject,
	target domain.Target, level domain.Level, caps domain.Caps) (domain.Grant, error) {
	if err := s.targetExists(ctx, target); err != nil {
		return domain.Grant{}, err
	}
	kind, ok := authorize.ParseResourceKind(string(target.Kind))
	if !ok {
		return domain.Grant{}, internal("a grant target kind outside the catalogue")
	}
	res := authorize.Resource{Kind: kind, ID: cmd.TargetID, Grant: &authorize.GrantFacts{
		SubjectKind: string(subject.Kind), Level: level.String(), Caps: caps.Names(), TargetKind: string(target.Kind),
		TargetSpaceID: target.SpaceID, TargetChannelID: target.ChannelID, TargetHostID: target.HostID,
	}}
	if err := s.check(ctx, caller, authorize.ActionGrantCreate, res); err != nil {
		return domain.Grant{}, err
	}
	if err := s.targetLive(ctx, target); err != nil {
		return domain.Grant{}, err
	}
	if err := s.subjectExists(ctx, subject); err != nil {
		return domain.Grant{}, err
	}
	g, err := domain.NewGrant(ids.New(), subject, target, level, caps, cmd.ExpiresAt, cmd.Reason, caller.ID, s.d.Clock.Now())
	if err != nil {
		return domain.Grant{}, err
	}
	if err := s.affectGroup(ctx, subject); err != nil {
		return domain.Grant{}, err
	}
	if err := s.d.Store.InsertGrant(ctx, g); err != nil {
		return domain.Grant{}, err
	}
	return g, s.record(ctx, authorize.ActionGrantCreate, authorize.ResourceGrant, g.ID, grantAttrs(g))
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
		return domain.Subject{}, domain.Target{}, 0, 0, fieldErr("/subject", "the subject kind must be user, group, agent or app")
	}
	if cmd.SubjectID.IsZero() {
		return domain.Subject{}, domain.Target{}, 0, 0, fieldErr("/subject", "the subject needs an id")
	}
	target := domain.Target{Kind: domain.TargetKind(cmd.TargetKind)}
	switch target.Kind {
	case domain.TargetInstance:
		if !cmd.TargetID.IsZero() {
			return domain.Subject{}, domain.Target{}, 0, 0, fieldErr("/target", "the instance has no id")
		}
	case domain.TargetSpace:
		target.SpaceID = cmd.TargetID
	case domain.TargetChannel:
		target.ChannelID = cmd.TargetID
	case domain.TargetHost:
		target.HostID = cmd.TargetID
	default:
		return domain.Subject{}, domain.Target{}, 0, 0, fieldErr("/target", "the target kind must be instance, space, channel or host")
	}
	if target.Kind != domain.TargetInstance && cmd.TargetID.IsZero() {
		return domain.Subject{}, domain.Target{}, 0, 0, fieldErr("/target", "the target needs an id")
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
	return subject, target, level, caps, nil
}

// fieldErr is validation_failed with one invalid field.
func fieldErr(pointer, detail string) error {
	return errs.New(errs.CodeValidationFailed, "").WithFields(errs.FieldError{Pointer: pointer, Code: "invalid", Detail: detail})
}

// targetExists reports a missing target as not found. Hosts come with M2-39: before it no host
// exists (docs/DECISIONS.md M1-29).
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
		return errNotFound()
	}
	return errNotFound()
}

// targetLive refuses an archived space or channel with 409 conflict (after authorize, so an
// invisible target stays not found).
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
	case domain.TargetInstance, domain.TargetHost:
	}
	if archived {
		return errs.New(errs.CodeConflict, "The target is archived.").WithReason(string(auditing.ReasonChannelArchived))
	}
	return nil
}

// subjectExists checks the subject: a live group, or a principal of the subject's kind that is
// not deleted; otherwise not found.
func (s *AdminService) subjectExists(ctx context.Context, sub domain.Subject) error {
	if sub.Kind == domain.SubjectGroup {
		_, err := s.loadGroup(ctx, sub.GroupID, true)
		return err
	}
	f, found, err := s.d.Principals.Principal(ctx, sub.PrincipalID)
	if err != nil {
		return sourceError("principal source", err)
	}
	if !found || string(f.Kind) != string(sub.Kind) || f.Status == authz.PrincipalDeleted {
		return errNotFound()
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

// revokeGrant loads and authorizes (07 section 2.6 point 3 through the guard); a revoked grant is
// a call without effect; with apply it revokes.
func (s *AdminService) revokeGrant(ctx context.Context, caller actor.Actor, grantID ids.ID, reason string, attrs auditing.Attrs, apply bool) (domain.Grant, bool, error) {
	g, found, err := s.d.Store.GetGrant(ctx, grantID)
	if err != nil {
		return domain.Grant{}, false, err
	}
	if !found {
		return domain.Grant{}, false, errNotFound()
	}
	if err := s.check(ctx, caller, authorize.ActionGrantRevoke, authorize.Resource{Kind: authorize.ResourceGrant, ID: g.ID}); err != nil {
		return domain.Grant{}, false, err
	}
	if !g.RevokedAt.IsZero() {
		if apply {
			return domain.Grant{}, false, errRaced
		}
		return g, false, s.record(ctx, authorize.ActionGrantRevoke, authorize.ResourceGrant, g.ID, merge(attrs, noChange()))
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
		return domain.Grant{}, false, errRaced
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
		attrs[attrGroupID] = auditing.ID(g.Subject.GroupID)
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

// groupOut converts a group for the port.
func groupOut(g domain.Group) authz.Group {
	return authz.Group{ID: g.ID, Name: g.Name, Description: g.Description, Source: g.Source, CreatedBy: g.CreatedBy,
		CreatedAt: g.CreatedAt, DeletedAt: g.DeletedAt}
}

// membershipOut converts a membership for the port.
func membershipOut(m domain.Membership) authz.Membership {
	return authz.Membership{ID: m.ID, GroupID: m.GroupID, UserID: m.UserID, AddedBy: m.AddedBy, AddedAt: m.AddedAt, ExpiresAt: m.ExpiresAt}
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
