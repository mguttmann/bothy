// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// People implements the people, owner and self operations of rest.StrictServerInterface (M1-31):
// listUsers, createUser, getUser, updateUser, enableUser, listOwners, addOwner, revokeOwner, getMe
// and updateMyProfile. The use cases authorize; the handler only converts. Errors go to the
// problem writer of the chain unchanged.
type People struct {
	people identity.People
	owners identity.Owners
}

// NewPeople returns the handlers over the two ports.
func NewPeople(people identity.People, owners identity.Owners) *People {
	return &People{people: people, owners: owners}
}

// ListUsers implements rest.StrictServerInterface (GET /users).
func (h *People) ListUsers(ctx context.Context, req rest.ListUsersRequestObject) (rest.ListUsersResponseObject, error) {
	q := identity.ListUsersQuery{}
	p := pageOf(req.Params.Cursor, req.Params.Limit)
	q.Cursor, q.Limit = p.Cursor, p.Limit
	if req.Params.Q != nil {
		q.Q = *req.Params.Q
	}
	if req.Params.Status != nil {
		q.Status = identity.Status(*req.Params.Status)
	}
	page, err := h.people.ListUsers(ctx, principal(ctx), q)
	if err != nil {
		return nil, err
	}
	out := rest.UserPage{Items: make([]rest.User, 0, len(page.Items))}
	for _, u := range page.Items {
		out.Items = append(out.Items, toRESTUser(u))
	}
	if page.NextCursor != "" {
		out.NextCursor = &page.NextCursor
	}
	return rest.ListUsers200JSONResponse{Body: out}, nil
}

// CreateUser implements rest.StrictServerInterface (POST /users).
func (h *People) CreateUser(ctx context.Context, req rest.CreateUserRequestObject) (rest.CreateUserResponseObject, error) {
	b := req.Body
	cmd := identity.CreateUserCmd{Handle: b.Handle, DisplayName: b.DisplayName, Locale: b.Locale, TimeZone: b.TimeZone}
	if b.Email != nil {
		e := emailOf(b.Email)
		cmd.Email = &e
	}
	if b.LocalAccount != nil {
		cmd.LocalAccount = *b.LocalAccount
	}
	u, err := h.people.CreateUser(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	out := rest.UserCreated{User: toRESTUser(u.User)}
	if u.Enrollment != nil {
		// The code is a one-time secret: the Idempotency-Key stage stores no response that holds it
		// (06 "Idempotenz", R-API-26).
		idempotency.MarkOneTimeSecret(ctx)
		e := enrollmentREST(*u.Enrollment)
		out.Enrollment = &e
	}
	return rest.CreateUser201JSONResponse{Body: out}, nil
}

// GetUser implements rest.StrictServerInterface (GET /users/{user_id}).
func (h *People) GetUser(ctx context.Context, req rest.GetUserRequestObject) (rest.GetUserResponseObject, error) {
	u, err := h.people.GetUser(ctx, principal(ctx), ids.ID(req.UserId))
	if err != nil {
		return nil, err
	}
	return rest.GetUser200JSONResponse{Body: toRESTUser(u)}, nil
}

// UpdateUser implements rest.StrictServerInterface (PATCH /users/{user_id}).
func (h *People) UpdateUser(ctx context.Context, req rest.UpdateUserRequestObject) (rest.UpdateUserResponseObject, error) {
	b := req.Body
	cmd := identity.UpdateUserCmd{DisplayName: b.DisplayName, Locale: b.Locale, TimeZone: b.TimeZone}
	if b.Email != nil {
		e := emailOf(b.Email)
		cmd.Email = &e
	}
	u, err := h.people.UpdateUser(ctx, principal(ctx), ids.ID(req.UserId), cmd)
	if err != nil {
		return nil, err
	}
	return rest.UpdateUser200JSONResponse{Body: toRESTUser(u)}, nil
}

// EnableUser implements rest.StrictServerInterface (POST /users/{user_id}/enable).
func (h *People) EnableUser(ctx context.Context, req rest.EnableUserRequestObject) (rest.EnableUserResponseObject, error) {
	u, err := h.people.EnableUser(ctx, principal(ctx), ids.ID(req.UserId), req.Body.Reason)
	if err != nil {
		return nil, err
	}
	return rest.EnableUser200JSONResponse{Body: toRESTUser(u)}, nil
}

// ListOwners implements rest.StrictServerInterface (GET /owners).
func (h *People) ListOwners(ctx context.Context, _ rest.ListOwnersRequestObject) (rest.ListOwnersResponseObject, error) {
	list, err := h.owners.ListOwners(ctx, principal(ctx))
	if err != nil {
		return nil, err
	}
	out := rest.OwnerList{Items: make([]rest.Owner, 0, len(list))}
	for _, o := range list {
		out.Items = append(out.Items, toRESTOwner(o))
	}
	return rest.ListOwners200JSONResponse{Body: out}, nil
}

// AddOwner implements rest.StrictServerInterface (POST /owners).
func (h *People) AddOwner(ctx context.Context, req rest.AddOwnerRequestObject) (rest.AddOwnerResponseObject, error) {
	o, err := h.owners.AddOwner(ctx, principal(ctx), ids.ID(req.Body.UserId), req.Body.Reason, ownerResolve(req.Body.ResolveI1))
	if err != nil {
		return nil, err
	}
	return rest.AddOwner201JSONResponse{Body: toRESTOwner(o)}, nil
}

// RevokeOwner implements rest.StrictServerInterface (POST /owners/{user_id}/revoke).
func (h *People) RevokeOwner(ctx context.Context, req rest.RevokeOwnerRequestObject) (rest.RevokeOwnerResponseObject, error) {
	if err := h.owners.RevokeOwner(ctx, principal(ctx), ids.ID(req.UserId), req.Body.Reason); err != nil {
		return nil, err
	}
	return rest.RevokeOwner204Response{}, nil
}

// GetMe implements rest.StrictServerInterface (GET /me).
func (h *People) GetMe(ctx context.Context, _ rest.GetMeRequestObject) (rest.GetMeResponseObject, error) {
	me, err := h.people.GetMe(ctx, principal(ctx))
	if err != nil {
		return nil, err
	}
	out := rest.Me{
		Principal: toRESTPrincipal(me.Principal), IsOwner: me.IsOwner, PermissionVersion: me.PermissionVersion,
		Locale: optional(me.Locale), TimeZone: optional(me.TimeZone), SessionId: optionalID(me.SessionID),
		DeviceId: optionalID(me.DeviceID), StepUpValidUntil: optionalTime(me.StepUpValidUntil),
	}
	if me.Email != "" {
		setString(&out.Email, me.Email)
	}
	return rest.GetMe200JSONResponse{Body: out}, nil
}

// UpdateMyProfile implements rest.StrictServerInterface (PATCH /me/profile).
func (h *People) UpdateMyProfile(ctx context.Context, req rest.UpdateMyProfileRequestObject) (rest.UpdateMyProfileResponseObject, error) {
	b := req.Body
	u, err := h.people.UpdateMyProfile(ctx, principal(ctx), identity.ProfileCmd{DisplayName: b.DisplayName, Locale: b.Locale, TimeZone: b.TimeZone})
	if err != nil {
		return nil, err
	}
	return rest.UpdateMyProfile200JSONResponse{Body: toRESTUser(u)}, nil
}

// toRESTUser converts a person (schema User, which has no e-mail member); empty values become
// absent members, never null (ADR 0053).
func toRESTUser(u identity.UserView) rest.User {
	return rest.User{
		Id: rest.Uuid(u.ID), Handle: u.Handle, DisplayName: u.DisplayName, AvatarFileId: optionalID(u.AvatarFileID),
		Status: rest.UserStatus(u.Status), IsOwner: u.IsOwner, Locale: optional(u.Locale), TimeZone: optional(u.TimeZone),
		CreatedAt: ms(u.CreatedAt),
	}
}

// toRESTPrincipal converts a principal (schema PrincipalRef).
func toRESTPrincipal(p identity.PrincipalView) rest.PrincipalRef {
	return rest.PrincipalRef{
		Id: rest.Uuid(p.ID), Kind: rest.PrincipalKind(p.Kind), Handle: p.Handle, DisplayName: p.DisplayName,
		AvatarFileId: optionalID(p.AvatarFileID),
	}
}

// toRESTOwner converts an owner (schema Owner); granted_by is absent for the first owner.
func toRESTOwner(o identity.OwnerView) rest.Owner {
	return rest.Owner{User: toRESTPrincipal(o.User), GrantedBy: optionalID(o.GrantedBy), GrantedAt: ms(o.GrantedAt), Reason: o.Reason}
}

func optionalID(id ids.ID) *rest.Uuid {
	if id.IsZero() {
		return nil
	}
	u := rest.Uuid(id)
	return &u
}

// emailOf returns the e-mail member of a request, "" when absent. It is generic over the string
// type of the generated model, so this package needs no runtime package of oapi-codegen, which
// the module register does not grant to identity adapters.
func emailOf[T ~string](e *T) string {
	if e == nil {
		return ""
	}
	return string(*e)
}

// setString points *dst at a copy of s converted to the string type of the generated model.
func setString[T ~string](dst **T, s string) {
	v := T(s)
	*dst = &v
}

// ownerResolve converts the member resolve_i1 of OwnerAdd (schema ResolveI1, M2-21); nil stays nil.
func ownerResolve(r *rest.ResolveI1) *identity.ResolveI1 {
	if r == nil {
		return nil
	}
	out := &identity.ResolveI1{Reason: r.Reason, AgentIDs: make([]ids.ID, 0, len(r.SwitchToOrganization))}
	for _, id := range r.SwitchToOrganization {
		out.AgentIDs = append(out.AgentIDs, ids.ID(id))
	}
	return out
}
