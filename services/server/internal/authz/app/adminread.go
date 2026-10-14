// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The read use cases of the Admin port (M1-48). Each reads the clock once (t), shows nothing the
// caller may not see and filters in the query; lists carry no totals (06 section 6).

// The list names of the cursors.
const (
	cursorGroups  = "authz.groups"
	cursorMembers = "authz.group_members"
	cursorGrants  = "authz.grants"
)

// idCursor is the keyset position of a page ordered by id: the id of its last row.
type idCursor struct {
	ID string `json:"id"`
}

// pageLimit returns the page size of q: api.page_size_default for none, capped at
// api.page_size_max.
func (s *AdminService) pageLimit(q authz.PageQuery) int {
	switch {
	case q.Limit <= 0:
		return s.d.Config.PageSizeDefault
	case q.Limit > s.d.Config.PageSizeMax:
		return s.d.Config.PageSizeMax
	}
	return q.Limit
}

// after decodes the cursor of q for list and binding; no cursor starts at the zero id.
func (s *AdminService) after(q authz.PageQuery, list string, binding []byte) (ids.ID, error) {
	if q.Cursor == "" {
		return ids.ID{}, nil
	}
	var c idCursor
	if err := s.d.Cursors.Decode(list, binding, q.Cursor, &c); err != nil {
		return ids.ID{}, errs.New(errs.CodeCursorInvalid, "")
	}
	id, err := ids.Parse(c.ID)
	if err != nil {
		return ids.ID{}, errs.New(errs.CodeCursorInvalid, "")
	}
	return id, nil
}

// next returns the cursor after last when the page was full (more rows exist), else "".
func (s *AdminService) next(full bool, list string, binding []byte, last ids.ID) (string, error) {
	if !full {
		return "", nil
	}
	c, err := s.d.Cursors.Encode(list, binding, idCursor{ID: last.String()})
	if err != nil {
		return "", errs.Wrap(errs.CodeInternalError, err, "authz: list cursor")
	}
	return c, nil
}

// binding joins the parts of a cursor binding with a separator no id or code contains.
func binding(parts ...string) []byte {
	var b []byte
	for _, p := range parts {
		b = append(b, p...)
		b = append(b, 0)
	}
	return b
}

// ListGroups implements authz.Admin.
func (s *AdminService) ListGroups(ctx context.Context, caller actor.Actor, q authz.ListGroupsQuery) (authz.GroupPage, error) {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return authz.GroupPage{}, err
	}
	sc, err := s.d.Scopes.AdminScope(ctx, caller, authorize.ActionGroupRead)
	if err != nil {
		return authz.GroupPage{}, err
	}
	bind := binding(caller.ID.String(), q.MemberID.String())
	after, err := s.after(q.PageQuery, cursorGroups, bind)
	if err != nil {
		return authz.GroupPage{}, err
	}
	member := q.MemberID
	if !sc.Owner {
		// Anyone else sees only the own groups; a filter on another member narrows that to none.
		if !member.IsZero() && member != caller.ID {
			return authz.GroupPage{Items: []authz.Group{}}, nil
		}
		member = caller.ID
	}
	t := s.d.Clock.Now()
	limit := s.pageLimit(q.PageQuery)
	rows, err := s.d.Store.ListLiveGroups(ctx, after, member, t, limit+1)
	if err != nil {
		return authz.GroupPage{}, err
	}
	full := len(rows) > limit
	rows = rows[:min(len(rows), limit)]
	list := make([]ids.ID, 0, len(rows))
	for _, g := range rows {
		list = append(list, g.ID)
	}
	counts, err := s.d.Store.CountMembersInEffect(ctx, list, t)
	if err != nil {
		return authz.GroupPage{}, err
	}
	out := authz.GroupPage{Items: make([]authz.Group, 0, len(rows))}
	for _, g := range rows {
		out.Items = append(out.Items, groupOut(g, counts[g.ID]))
	}
	if len(rows) > 0 {
		out.NextCursor, err = s.next(full, cursorGroups, bind, rows[len(rows)-1].ID)
	}
	return out, err
}

// visibleGroup authorizes group.read on the group and loads it when live: owner or in-effect
// member, otherwise 404 like an unknown id.
func (s *AdminService) visibleGroup(ctx context.Context, caller actor.Actor, groupID ids.ID) (domain.Group, error) {
	return s.loadGroup(ctx, caller, authorize.ActionGroupRead, groupID, true)
}

// GetGroup implements authz.Admin.
func (s *AdminService) GetGroup(ctx context.Context, caller actor.Actor, groupID ids.ID) (authz.Group, error) {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return authz.Group{}, err
	}
	g, err := s.visibleGroup(ctx, caller, groupID)
	if err != nil {
		return authz.Group{}, err
	}
	counts, err := s.d.Store.CountMembersInEffect(ctx, []ids.ID{g.ID}, s.d.Clock.Now())
	if err != nil {
		return authz.Group{}, err
	}
	return groupOut(g, counts[g.ID]), nil
}

// ListGroupMembers implements authz.Admin.
func (s *AdminService) ListGroupMembers(ctx context.Context, caller actor.Actor, groupID ids.ID, p authz.PageQuery) (authz.MemberPage, error) {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return authz.MemberPage{}, err
	}
	g, err := s.visibleGroup(ctx, caller, groupID)
	if err != nil {
		return authz.MemberPage{}, err
	}
	bind := binding(caller.ID.String(), g.ID.String())
	after, err := s.after(p, cursorMembers, bind)
	if err != nil {
		return authz.MemberPage{}, err
	}
	limit := s.pageLimit(p)
	rows, err := s.d.Store.ListMembersInEffect(ctx, g.ID, after, s.d.Clock.Now(), limit+1)
	if err != nil {
		return authz.MemberPage{}, err
	}
	full := len(rows) > limit
	rows = rows[:min(len(rows), limit)]
	people := make([]ids.ID, 0, len(rows))
	for _, m := range rows {
		people = append(people, m.UserID)
	}
	refs, err := s.refs(ctx, people)
	if err != nil {
		return authz.MemberPage{}, err
	}
	out := authz.MemberPage{Items: make([]authz.Membership, 0, len(rows))}
	for _, m := range rows {
		out.Items = append(out.Items, membershipOut(m, refs[m.UserID]))
	}
	if len(rows) > 0 {
		out.NextCursor, err = s.next(full, cursorMembers, bind, rows[len(rows)-1].ID)
	}
	return out, err
}

// grantReadResource is the resource of grant.read on the target of a grant (05 grant.read:
// space, channel, host, principal). An instance grant is owner only: grant.read on the caller as
// a principal has the requirement owner. A host grant is read like an instance grant (owner only,
// the requirement of grant.read on a host): the holder sees the host but no grant on it, and a
// non-owner gets the same answer for a host grant and an unknown id (M2-39).
func grantReadResource(caller actor.Actor, kind domain.TargetKind, id ids.ID) (authorize.Resource, bool) {
	switch kind {
	case domain.TargetSpace:
		return authorize.Resource{Kind: authorize.ResourceSpace, ID: id}, true
	case domain.TargetChannel:
		return authorize.Resource{Kind: authorize.ResourceChannel, ID: id}, true
	case domain.TargetInstance, domain.TargetHost:
		return authorize.Resource{Kind: authorize.ResourcePrincipal, ID: caller.ID, OwnerID: caller.ID}, true
	}
	return authorize.Resource{}, false
}

// checkTarget authorizes grant.read on the target filter r of listGrants. A refused non-owner gets
// 404 for a host filter, the answer of an unknown host (D-M2-39-6), except for the reasons of a
// pending person or device, which refuse before visibility.
func (s *AdminService) checkTarget(ctx context.Context, caller actor.Actor, kind domain.TargetKind, r authorize.Resource) error {
	d, err := s.d.Authorizer.Authorize(ctx, caller, authorize.ActionGrantRead, r)
	if err != nil {
		return err
	}
	if kind == domain.TargetHost && d.Outcome == authorize.OutcomeForbidden && !isPendingReason(d.Reason) {
		return errNotFound()
	}
	return d.Err()
}

// GetGrant implements authz.Admin: outside the caller's manage scope, and for an instance grant
// unless the caller is an owner, the grant answers 404 like an unknown id (AZ-P4).
func (s *AdminService) GetGrant(ctx context.Context, caller actor.Actor, grantID ids.ID) (authz.Grant, error) {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return authz.Grant{}, err
	}
	g, found, err := s.d.Store.GetGrant(ctx, grantID)
	if err != nil {
		return authz.Grant{}, err
	}
	// An unknown id is checked like an instance grant, so a caller refused before visibility
	// gets the same answer for an existing and an unknown id (07 4.2, M1-31/16).
	kind, target := domain.TargetInstance, ids.ID{}
	if found {
		kind, target = g.Target.Kind, targetID(g.Target)
	}
	r, ok := grantReadResource(caller, kind, target)
	if !ok {
		r, _ = grantReadResource(caller, domain.TargetInstance, ids.ID{})
	}
	d, err := s.d.Authorizer.Authorize(ctx, caller, authorize.ActionGrantRead, r)
	if err != nil {
		return authz.Grant{}, err
	}
	switch {
	case d.Outcome == authorize.OutcomeForbidden && isPendingReason(d.Reason):
		return authz.Grant{}, d.Err()
	case d.Outcome == authorize.OutcomeAllow && found && ok:
		return grantOut(g), nil
	case d.Outcome == authorize.OutcomeAllow, d.Outcome == authorize.OutcomeNotFound, d.Outcome == authorize.OutcomeForbidden:
		return authz.Grant{}, errNotFound()
	}
	return authz.Grant{}, d.Err()
}

// isPendingReason reports the reasons of a pending person or device, which refuse before
// visibility and so answer alike for every id.
func isPendingReason(r authorize.Reason) bool {
	return r == authorize.Reason(domain.ReasonPrincipalPending) || r == authorize.Reason(domain.ReasonDevicePending)
}

// ListGrants implements authz.Admin (brief M1-48 "listGrants filters").
func (s *AdminService) ListGrants(ctx context.Context, caller actor.Actor, q authz.ListGrantsQuery) (authz.GrantPage, error) {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return authz.GrantPage{}, err
	}
	f, err := grantFilterOf(q)
	if err != nil {
		return authz.GrantPage{}, err
	}
	if f.TargetKind != "" {
		r, ok := grantReadResource(caller, f.TargetKind, f.TargetID)
		if !ok {
			return authz.GrantPage{}, errNotFound()
		}
		if err := s.checkTarget(ctx, caller, f.TargetKind, r); err != nil {
			return authz.GrantPage{}, err
		}
	}
	sc, err := s.d.Scopes.AdminScope(ctx, caller, authorize.ActionGrantRead)
	if err != nil {
		return authz.GrantPage{}, err
	}
	f.AllTargets, f.ManageSpaces, f.ManageChannels = sc.Owner, sc.ManageSpaces, sc.ManageChannels
	bind := binding(caller.ID.String(), string(f.TargetKind), f.TargetID.String(), string(f.SubjectKind), f.SubjectID.String(),
		boolText(f.IncludeRevoked))
	after, err := s.after(q.PageQuery, cursorGrants, bind)
	if err != nil {
		return authz.GrantPage{}, err
	}
	limit := s.pageLimit(q.PageQuery)
	rows, err := s.d.Store.ListGrants(ctx, f, after, s.d.Clock.Now(), limit+1)
	if err != nil {
		return authz.GrantPage{}, err
	}
	full := len(rows) > limit
	rows = rows[:min(len(rows), limit)]
	out := authz.GrantPage{Items: make([]authz.Grant, 0, len(rows))}
	for _, g := range rows {
		out.Items = append(out.Items, grantOut(g))
	}
	if len(rows) > 0 {
		out.NextCursor, err = s.next(full, cursorGrants, bind, rows[len(rows)-1].ID)
	}
	return out, err
}

// boolText is "1" or "0".
func boolText(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// grantFilterOf checks the filter pairs of q: target_kind with target_id (none for instance),
// subject_kind with subject_id; a half pair is 400 validation_failed on #query/<missing name>.
func grantFilterOf(q authz.ListGrantsQuery) (GrantFilter, error) {
	f := GrantFilter{TargetKind: domain.TargetKind(q.TargetKind), TargetID: q.TargetID, SubjectKind: domain.SubjectKind(q.SubjectKind),
		SubjectID: q.SubjectID, IncludeRevoked: q.IncludeRevoked}
	switch {
	case f.TargetKind == "" && !f.TargetID.IsZero():
		return GrantFilter{}, fieldCode("#query/target_kind", "required", "target_id needs target_kind")
	case f.TargetKind == domain.TargetInstance && !f.TargetID.IsZero():
		return GrantFilter{}, fieldCode("#query/target_id", "forbidden", "the instance has no id")
	case f.TargetKind != "" && f.TargetKind != domain.TargetInstance && f.TargetID.IsZero():
		return GrantFilter{}, fieldCode("#query/target_id", "required", "target_kind needs target_id")
	case f.SubjectKind == "" && !f.SubjectID.IsZero():
		return GrantFilter{}, fieldCode("#query/subject_kind", "required", "subject_id needs subject_kind")
	case f.SubjectKind != "" && f.SubjectID.IsZero():
		return GrantFilter{}, fieldCode("#query/subject_id", "required", "subject_kind needs subject_id")
	}
	switch f.TargetKind {
	case "", domain.TargetInstance, domain.TargetSpace, domain.TargetChannel, domain.TargetHost:
	default:
		return GrantFilter{}, fieldCode("#query/target_kind", "invalid_enum", "unknown target kind")
	}
	switch f.SubjectKind {
	case "", domain.SubjectUser, domain.SubjectGroup, domain.SubjectAgent, domain.SubjectApp:
	default:
		return GrantFilter{}, fieldCode("#query/subject_kind", "invalid_enum", "unknown subject kind")
	}
	return f, nil
}
