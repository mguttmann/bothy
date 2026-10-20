// SPDX-License-Identifier: Apache-2.0

// Package httpapi is the REST adapter of the authz module (M1-48, M1-49): the sixteen operations
// of 06 "Groups" and "Access" (listGroups, createGroup, getGroup, updateGroup, deleteGroup,
// listGroupMembers, addGroupMember, removeGroupMember, listGrants, createGrant, getGrant,
// revokeGrant, and since M1-49 explainSpaceAccess, explainChannelAccess, previewGrant and
// viewAsUser) over the port authz.Admin, and Cursors, the cursor codec of platform/httpserver
// behind app.Cursors. The use cases authorize; the handlers only convert. Errors go to the
// problem writer of the chain unchanged.
//
// Example:
//
//	h := httpapi.New(admin)
//	built, err := routes.Build(wire.Handlers{Access: h}, wire.RouteOptions(), cat)
package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Access implements the group and grant operations of rest.StrictServerInterface.
type Access struct {
	admin authz.Admin
}

// New returns the handlers over the Admin port.
func New(admin authz.Admin) *Access {
	return &Access{admin: admin}
}

// principal returns the actor of the request; outside the chain it is the anonymous principal,
// which authorize refuses.
func principal(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// pageOf converts the paging parameters.
func pageOf(cursor *rest.Cursor, limit *rest.Limit) authz.PageQuery {
	var p authz.PageQuery
	if cursor != nil {
		p.Cursor = *cursor
	}
	if limit != nil {
		p.Limit = *limit
	}
	return p
}

// ListGroups implements rest.StrictServerInterface (GET /groups).
func (h *Access) ListGroups(ctx context.Context, req rest.ListGroupsRequestObject) (rest.ListGroupsResponseObject, error) {
	q := authz.ListGroupsQuery{PageQuery: pageOf(req.Params.Cursor, req.Params.Limit)}
	if req.Params.MemberId != nil {
		q.MemberID = ids.ID(*req.Params.MemberId)
	}
	p, err := h.admin.ListGroups(ctx, principal(ctx), q)
	if err != nil {
		return nil, err
	}
	out := rest.GroupPage{Items: make([]rest.Group, 0, len(p.Items))}
	for _, g := range p.Items {
		out.Items = append(out.Items, groupREST(g))
	}
	out.NextCursor = optional(p.NextCursor)
	return rest.ListGroups200JSONResponse{Body: out}, nil
}

// CreateGroup implements rest.StrictServerInterface (POST /groups).
func (h *Access) CreateGroup(ctx context.Context, req rest.CreateGroupRequestObject) (rest.CreateGroupResponseObject, error) {
	var cmd authz.CreateGroupCmd
	if b := req.Body; b != nil {
		cmd.Name, cmd.Description, cmd.ExternalRef = b.Name, value(b.Description), value(b.ExternalRef)
		if b.Source != nil {
			cmd.Source = string(*b.Source)
		}
		if b.ProviderId != nil {
			cmd.ProviderID = ids.ID(*b.ProviderId)
		}
	}
	g, err := h.admin.CreateGroup(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.CreateGroup201JSONResponse{Body: groupREST(g)}, nil
}

// GetGroup implements rest.StrictServerInterface (GET /groups/{group_id}).
func (h *Access) GetGroup(ctx context.Context, req rest.GetGroupRequestObject) (rest.GetGroupResponseObject, error) {
	g, err := h.admin.GetGroup(ctx, principal(ctx), ids.ID(req.GroupId))
	if err != nil {
		return nil, err
	}
	return rest.GetGroup200JSONResponse{Body: groupREST(g)}, nil
}

// UpdateGroup implements rest.StrictServerInterface (PATCH /groups/{group_id}).
func (h *Access) UpdateGroup(ctx context.Context, req rest.UpdateGroupRequestObject) (rest.UpdateGroupResponseObject, error) {
	var cmd authz.UpdateGroupCmd
	if b := req.Body; b != nil {
		cmd.Name, cmd.Description = b.Name, b.Description
	}
	g, err := h.admin.UpdateGroup(ctx, principal(ctx), ids.ID(req.GroupId), cmd)
	if err != nil {
		return nil, err
	}
	return rest.UpdateGroup200JSONResponse{Body: groupREST(g)}, nil
}

// DeleteGroup implements rest.StrictServerInterface (DELETE /groups/{group_id}).
func (h *Access) DeleteGroup(ctx context.Context, req rest.DeleteGroupRequestObject) (rest.DeleteGroupResponseObject, error) {
	if err := h.admin.DeleteGroup(ctx, principal(ctx), ids.ID(req.GroupId)); err != nil {
		return nil, err
	}
	return rest.DeleteGroup204Response{}, nil
}

// ListGroupMembers implements rest.StrictServerInterface (GET /groups/{group_id}/members).
func (h *Access) ListGroupMembers(ctx context.Context, req rest.ListGroupMembersRequestObject) (rest.ListGroupMembersResponseObject, error) {
	p, err := h.admin.ListGroupMembers(ctx, principal(ctx), ids.ID(req.GroupId), pageOf(req.Params.Cursor, req.Params.Limit))
	if err != nil {
		return nil, err
	}
	out := rest.GroupMemberPage{Items: make([]rest.GroupMember, 0, len(p.Items))}
	for _, m := range p.Items {
		out.Items = append(out.Items, memberREST(m))
	}
	out.NextCursor = optional(p.NextCursor)
	return rest.ListGroupMembers200JSONResponse{Body: out}, nil
}

// AddGroupMember implements rest.StrictServerInterface (POST /groups/{group_id}/members).
func (h *Access) AddGroupMember(ctx context.Context, req rest.AddGroupMemberRequestObject) (rest.AddGroupMemberResponseObject, error) {
	var cmd authz.AddMemberCmd
	if b := req.Body; b != nil {
		cmd.UserID = ids.ID(b.UserId)
		if b.ExpiresAt != nil {
			cmd.ExpiresAt = *b.ExpiresAt
		}
		cmd.ResolveI1 = resolveCmd(b.ResolveI1)
	}
	m, err := h.admin.AddGroupMember(ctx, principal(ctx), ids.ID(req.GroupId), cmd)
	if err != nil {
		return nil, err
	}
	return rest.AddGroupMember201JSONResponse{Body: memberREST(m)}, nil
}

// RemoveGroupMember implements rest.StrictServerInterface (DELETE
// /groups/{group_id}/members/{user_id}).
func (h *Access) RemoveGroupMember(ctx context.Context, req rest.RemoveGroupMemberRequestObject) (rest.RemoveGroupMemberResponseObject, error) {
	if err := h.admin.RemoveGroupMember(ctx, principal(ctx), ids.ID(req.GroupId), ids.ID(req.UserId)); err != nil {
		return nil, err
	}
	return rest.RemoveGroupMember204Response{}, nil
}

// ListGrants implements rest.StrictServerInterface (GET /grants).
func (h *Access) ListGrants(ctx context.Context, req rest.ListGrantsRequestObject) (rest.ListGrantsResponseObject, error) {
	prm := req.Params
	q := authz.ListGrantsQuery{PageQuery: pageOf(prm.Cursor, prm.Limit)}
	if prm.TargetKind != nil {
		q.TargetKind = string(*prm.TargetKind)
	}
	if prm.TargetId != nil {
		q.TargetID = ids.ID(*prm.TargetId)
	}
	if prm.SubjectKind != nil {
		q.SubjectKind = string(*prm.SubjectKind)
	}
	if prm.SubjectId != nil {
		q.SubjectID = ids.ID(*prm.SubjectId)
	}
	if prm.IncludeRevoked != nil {
		q.IncludeRevoked = *prm.IncludeRevoked
	}
	p, err := h.admin.ListGrants(ctx, principal(ctx), q)
	if err != nil {
		return nil, err
	}
	out := rest.GrantPage{Items: make([]rest.Grant, 0, len(p.Items))}
	for _, g := range p.Items {
		out.Items = append(out.Items, grantREST(g))
	}
	out.NextCursor = optional(p.NextCursor)
	return rest.ListGrants200JSONResponse{Body: out}, nil
}

// CreateGrant implements rest.StrictServerInterface (POST /grants).
func (h *Access) CreateGrant(ctx context.Context, req rest.CreateGrantRequestObject) (rest.CreateGrantResponseObject, error) {
	g, err := h.admin.CreateGrant(ctx, principal(ctx), grantCmd(req.Body))
	if err != nil {
		return nil, err
	}
	return rest.CreateGrant201JSONResponse{Body: grantREST(g)}, nil
}

// grantCmd converts the body GrantCreate of createGrant and previewGrant; a nil body gives the
// zero command, which the use case refuses.
func grantCmd(b *rest.GrantCreate) authz.CreateGrantCmd {
	var cmd authz.CreateGrantCmd
	if b == nil {
		return cmd
	}
	cmd.SubjectKind, cmd.SubjectID = string(b.SubjectKind), ids.ID(b.SubjectId)
	cmd.TargetKind, cmd.Reason = string(b.TargetKind), b.Reason
	if b.TargetId != nil {
		cmd.TargetID = ids.ID(*b.TargetId)
	}
	if b.Level != nil {
		cmd.Level = string(*b.Level)
	}
	if b.Capabilities != nil {
		cmd.Capabilities = make([]string, 0, len(*b.Capabilities))
		for _, c := range *b.Capabilities {
			cmd.Capabilities = append(cmd.Capabilities, string(c))
		}
	}
	if b.ExpiresAt != nil {
		cmd.ExpiresAt = *b.ExpiresAt
	}
	cmd.ResolveI1 = resolveCmd(b.ResolveI1)
	return cmd
}

// resolveCmd converts the member resolve_i1 (schema ResolveI1, M2-21); nil stays nil.
func resolveCmd(r *rest.ResolveI1) *authz.ResolveI1 {
	if r == nil {
		return nil
	}
	out := &authz.ResolveI1{Reason: r.Reason, AgentIDs: make([]ids.ID, 0, len(r.SwitchToOrganization))}
	for _, id := range r.SwitchToOrganization {
		out.AgentIDs = append(out.AgentIDs, ids.ID(id))
	}
	return out
}

// GetGrant implements rest.StrictServerInterface (GET /grants/{grant_id}).
func (h *Access) GetGrant(ctx context.Context, req rest.GetGrantRequestObject) (rest.GetGrantResponseObject, error) {
	g, err := h.admin.GetGrant(ctx, principal(ctx), ids.ID(req.GrantId))
	if err != nil {
		return nil, err
	}
	return rest.GetGrant200JSONResponse{Body: grantREST(g)}, nil
}

// RevokeGrant implements rest.StrictServerInterface (POST /grants/{grant_id}/revoke).
func (h *Access) RevokeGrant(ctx context.Context, req rest.RevokeGrantRequestObject) (rest.RevokeGrantResponseObject, error) {
	reason := ""
	if req.Body != nil {
		reason = req.Body.Reason
	}
	g, err := h.admin.RevokeGrant(ctx, principal(ctx), ids.ID(req.GrantId), reason)
	if err != nil {
		return nil, err
	}
	return rest.RevokeGrant200JSONResponse{Body: grantREST(g)}, nil
}

// groupREST converts a group (schema Group); provider_id and external_ref only for a claim
// group, never null (ADR 0053).
func groupREST(g authz.Group) rest.Group {
	return rest.Group{
		Id: rest.Uuid(g.ID), Name: g.Name, Description: g.Description, Source: rest.GroupSource(g.Source),
		ProviderId: optionalID(g.ProviderID), ExternalRef: optional(g.ExternalRef), MemberCount: g.MemberCount, CreatedAt: ms(g.CreatedAt),
	}
}

// memberREST converts a membership (schema GroupMember).
func memberREST(m authz.Membership) rest.GroupMember {
	u := m.User
	return rest.GroupMember{
		User: rest.PrincipalRef{
			Id: rest.Uuid(u.ID), Kind: rest.PrincipalKind(u.Kind), Handle: u.Handle, DisplayName: u.DisplayName,
			AvatarFileId: optionalID(u.AvatarFileID),
		},
		AddedBy: optionalID(m.AddedBy), AddedAt: ms(m.AddedAt), ExpiresAt: optionalTime(m.ExpiresAt),
	}
}

// grantREST converts a grant (schema Grant): target_id absent for the instance, level absent for
// a host, capabilities always an array.
func grantREST(g authz.Grant) rest.Grant {
	caps := make([]rest.Capability, 0, len(g.Capabilities))
	for _, c := range g.Capabilities {
		caps = append(caps, rest.Capability(c))
	}
	out := rest.Grant{
		Id: rest.Uuid(g.ID), SubjectKind: rest.GrantSubjectKind(g.SubjectKind), SubjectId: rest.Uuid(g.SubjectID),
		TargetKind: rest.TargetKind(g.TargetKind), TargetId: optionalID(g.TargetID), Capabilities: caps,
		ExpiresAt: optionalTime(g.ExpiresAt), Reason: g.Reason, GrantedBy: rest.Uuid(g.GrantedBy), GrantedAt: ms(g.GrantedAt),
		RevokedAt: optionalTime(g.RevokedAt), RevokedBy: optionalID(g.RevokedBy), RevokeReason: optional(g.RevokeReason),
	}
	if g.Level != "" {
		l := rest.Level(g.Level)
		out.Level = &l
	}
	return out
}

// value returns *p, or "" for nil.
func value(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// optional returns nil for "", else a pointer to s.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// optionalID returns nil for the zero id.
func optionalID(id ids.ID) *rest.Uuid {
	if id.IsZero() {
		return nil
	}
	u := rest.Uuid(id)
	return &u
}

// ms is t in UTC with millisecond precision (06 "Zeit").
func ms(t time.Time) time.Time { return t.UTC().Truncate(time.Millisecond) }

// optionalTime returns nil for the zero time.
func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	v := ms(t)
	return &v
}
