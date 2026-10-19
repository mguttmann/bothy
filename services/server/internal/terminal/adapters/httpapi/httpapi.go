// SPDX-License-Identifier: Apache-2.0

// Package httpapi holds the REST handlers of the terminal module (M2-39): listHosts, createHost,
// getHost, updateHost and setHostState of docs/api/openapi.yaml as part of
// rest.StrictServerInterface. The use cases of terminal.Hosts authorize; the handlers only
// convert, set ETag from the version and pass errors to the problem writer of the chain unchanged.
// The pairing code leaves the server only in the answer of createHost, once.
//
// Example:
//
//	h := httpapi.New(svc)
//	built, err := routes.Build(wire.Handlers{Hosts: h}, wire.RouteOptions(), cat)
package httpapi

import (
	"context"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/terminal"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
)

// Hosts implements listHosts, createHost, getHost, updateHost and setHostState.
type Hosts struct {
	hosts terminal.Hosts
}

// New returns the handlers over the use cases.
func New(h terminal.Hosts) *Hosts { return &Hosts{hosts: h} }

// Cursors is the codec of the list cursors (platform/httpserver), the outgoing port app.Cursors.
type Cursors struct{}

// Compile-time check: the codec is the port of the use cases.
var _ app.Cursors = Cursors{}

// Encode implements app.Cursors with httpserver.EncodeCursor.
func (Cursors) Encode(list string, binding []byte, key any) (string, error) {
	return httpserver.EncodeCursor(list, binding, key)
}

// Decode implements app.Cursors with httpserver.DecodeCursor (400 cursor_invalid on a mismatch).
func (Cursors) Decode(list string, binding []byte, s string, key any) error {
	return httpserver.DecodeCursor(list, binding, s, key)
}

// principal returns the actor of the request; anonymous when the chain set none.
func principal(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// precondition parses the If-Match header value of a request.
func precondition(v string) terminal.Precondition {
	h := http.Header{}
	if v != "" {
		h.Set("If-Match", v)
	}
	im := httpserver.ParseIfMatch(h)
	return terminal.Precondition{Present: im.Present, Valid: im.Valid, Version: im.Version}
}

// etag returns the ETag header value of v.
func etag(v terminal.HostView) *string {
	s := httpserver.FormatETag(v.Version)
	return &s
}

// ListHosts implements rest.StrictServerInterface (GET /hosts).
func (h *Hosts) ListHosts(ctx context.Context, req rest.ListHostsRequestObject) (rest.ListHostsResponseObject, error) {
	var p terminal.Page
	if req.Params.Cursor != nil {
		p.Cursor = *req.Params.Cursor
	}
	if req.Params.Limit != nil {
		p.Limit = *req.Params.Limit
	}
	page, err := h.hosts.List(ctx, principal(ctx), p)
	if err != nil {
		return nil, err
	}
	out := rest.HostPage{Items: make([]rest.Host, 0, len(page.Items))}
	for _, v := range page.Items {
		out.Items = append(out.Items, toREST(v))
	}
	if page.NextCursor != "" {
		out.NextCursor = &page.NextCursor
	}
	return rest.ListHosts200JSONResponse{Body: out}, nil
}

// CreateHost implements rest.StrictServerInterface (POST /hosts). The pairing code leaves the
// server only here, once (Token.Reveal); the Idempotency-Key stage stores no copy of it
// (MarkOneTimeSecret: a replay answers 409 conflict with reason one_time_secret, M1-03/6), and
// the chain answers with Cache-Control: no-store.
func (h *Hosts) CreateHost(ctx context.Context, req rest.CreateHostRequestObject) (rest.CreateHostResponseObject, error) {
	idempotency.MarkOneTimeSecret(ctx)
	var cmd terminal.CreateHost
	if b := req.Body; b != nil {
		cmd.Name, cmd.Reason = b.Name, b.Reason
		if b.AssignedUserId != nil {
			cmd.AssignedUserID = ids.ID(*b.AssignedUserId)
		}
	}
	issued, err := h.hosts.Create(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.CreateHost201JSONResponse{Body: rest.HostPairingCode{
		HostId: rest.Uuid(issued.HostID), PairingCode: issued.Code.Reveal(), ExpiresAt: issued.ExpiresAt.UTC().Truncate(time.Millisecond),
	}}, nil
}

// GetHost implements rest.StrictServerInterface (GET /hosts/{host_id}).
func (h *Hosts) GetHost(ctx context.Context, req rest.GetHostRequestObject) (rest.GetHostResponseObject, error) {
	v, err := h.hosts.Get(ctx, principal(ctx), ids.ID(req.HostId))
	if err != nil {
		return nil, err
	}
	return rest.GetHost200JSONResponse{Body: toREST(v), Headers: rest.GetHost200ResponseHeaders{ETag: etag(v)}}, nil
}

// UpdateHost implements rest.StrictServerInterface (PATCH /hosts/{host_id}).
func (h *Hosts) UpdateHost(ctx context.Context, req rest.UpdateHostRequestObject) (rest.UpdateHostResponseObject, error) {
	cmd := terminal.UpdateHost{HostID: ids.ID(req.HostId), IfMatch: precondition(req.Params.IfMatch)}
	if b := req.Body; b != nil {
		updateBody(&cmd, *b)
	}
	v, err := h.hosts.Update(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.UpdateHost200JSONResponse{Body: toREST(v), Headers: rest.UpdateHost200ResponseHeaders{ETag: etag(v)}}, nil
}

// updateBody copies the members of HostUpdate into cmd; absent members stay unset.
func updateBody(cmd *terminal.UpdateHost, b rest.HostUpdate) {
	cmd.Reason = b.Reason
	if b.Name != nil {
		cmd.Name, cmd.NameSet = *b.Name, true
	}
	if b.AssignedUserId != nil {
		cmd.AssignedUserID = ids.ID(*b.AssignedUserId)
	}
	if b.ClearAssignedUser != nil {
		cmd.ClearAssignedUser = *b.ClearAssignedUser
	}
	if b.AddSessionUsers != nil {
		for _, a := range *b.AddSessionUsers {
			cmd.Add = append(cmd.Add, terminal.SessionUserAssign{UserID: ids.ID(a.UserId), OSUser: a.OsUser})
		}
	}
	if b.RevokeSessionUsers != nil {
		for _, u := range *b.RevokeSessionUsers {
			cmd.Revoke = append(cmd.Revoke, ids.ID(u))
		}
	}
}

// SetHostState implements rest.StrictServerInterface (POST /hosts/{host_id}/state).
func (h *Hosts) SetHostState(ctx context.Context, req rest.SetHostStateRequestObject) (rest.SetHostStateResponseObject, error) {
	cmd := terminal.SetHostState{HostID: ids.ID(req.HostId), IfMatch: precondition(req.Params.IfMatch)}
	if b := req.Body; b != nil {
		cmd.State, cmd.Reason = string(b.State), b.Reason
	}
	v, err := h.hosts.SetState(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.SetHostState200JSONResponse{Body: toREST(v), Headers: rest.SetHostState200ResponseHeaders{ETag: etag(v)}}, nil
}

// toREST converts a host (schema Host); empty values become absent members, never null (ADR
// 0053); profiles and doctor are always present.
func toREST(v terminal.HostView) rest.Host {
	out := rest.Host{
		Id: rest.Uuid(v.ID), Name: v.Name, State: rest.HostState(v.State), AssignedUserId: optionalID(v.AssignedUserID),
		LlmCredentials: rest.LlmCredentials(v.LLMCredentials), CredentialHolderId: optionalID(v.CredentialHolderID),
		DeclaredServerHost: v.DeclaredServerHost, ReportedIsServerHost: v.ReportedIsServerHost, KeyFingerprint: optional(v.KeyFingerprint),
		KeyGeneration: int(v.KeyGeneration), RequireLocalConfirm: v.RequireLocalConfirm, Connected: v.Connected,
		HostVersion: optional(v.HostVersion), OsVersion: optional(v.OSVersion), LastSeenAt: optionalTime(v.LastSeenAt),
		CreatedAt: v.CreatedAt.UTC().Truncate(time.Millisecond), Version: v.Version,
		Profiles: make([]rest.HostProfile, 0, len(v.Profiles)), Doctor: make([]rest.HostDoctorCheck, 0, len(v.Doctor)),
	}
	if v.DisabledReason != "" {
		r := rest.HostDisabledReason(v.DisabledReason)
		out.DisabledReason = &r
	}
	if v.Platform != "" {
		p := rest.HostPlatform(v.Platform)
		out.Platform = &p
	}
	if len(v.NoisePublicKey) > 0 {
		k := base64.RawURLEncoding.EncodeToString(v.NoisePublicKey)
		out.NoisePublicKey = &k
	}
	for _, p := range v.Profiles {
		out.Profiles = append(out.Profiles, rest.HostProfile{Name: p.Name, DisplayName: p.DisplayName, Kind: rest.HostProfileKind(p.Kind)})
	}
	for _, c := range v.Doctor {
		out.Doctor = append(out.Doctor, rest.HostDoctorCheck{Check: rest.HostDoctorCheckCheck(c.Check), Ok: c.OK, Code: optional(c.Code),
			SessionUser: optional(c.SessionUser)})
	}
	if len(v.SessionUsers) > 0 {
		list := make([]rest.HostSessionUser, 0, len(v.SessionUsers))
		for _, u := range v.SessionUsers {
			list = append(list, rest.HostSessionUser{UserId: rest.Uuid(u.UserID), OsUser: u.OSUser, CreatedAt: u.CreatedAt.UTC().Truncate(time.Millisecond)})
		}
		out.SessionUsers = &list
	}
	if v.ReportedSessionUsers != nil {
		list := make([]rest.HostReportedSessionUser, 0, len(v.ReportedSessionUsers))
		for _, u := range v.ReportedSessionUsers {
			workdirs := u.Workdirs
			if workdirs == nil {
				workdirs = []string{}
			}
			list = append(list, rest.HostReportedSessionUser{OsUser: u.OSUser, Workdirs: workdirs})
		}
		out.ReportedSessionUsers = &list
	}
	return out
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func optionalID(id ids.ID) *rest.Uuid {
	if id.IsZero() {
		return nil
	}
	u := rest.Uuid(id)
	return &u
}

func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	v := t.UTC().Truncate(time.Millisecond)
	return &v
}
