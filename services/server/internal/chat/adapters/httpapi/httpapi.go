// SPDX-License-Identifier: Apache-2.0

// Package httpapi holds the REST handlers of the chat module (M1-52): listSpaces, createSpace,
// getSpace, updateSpace, moveSpace, archiveSpace and putMySidebarSpace of docs/api/openapi.yaml
// as part of rest.StrictServerInterface. The use cases of chat.Structure authorize; the handlers
// only convert, set ETag from the version and pass errors to the problem writer of the chain
// unchanged.
//
// Example:
//
//	h := httpapi.New(svc)
//	built, err := routes.Build(wire.Handlers{Spaces: h}, wire.RouteOptions(), cat)
package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// Spaces implements the seven space and sidebar operations.
type Spaces struct {
	structure chat.Structure
}

// New returns the handlers over the use cases.
func New(s chat.Structure) *Spaces { return &Spaces{structure: s} }

// principal returns the actor of the request; anonymous when the chain set none.
func principal(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// precondition parses the If-Match header value of a request.
func precondition(v string) chat.Precondition {
	h := http.Header{}
	if v != "" {
		h.Set("If-Match", v)
	}
	im := httpserver.ParseIfMatch(h)
	return chat.Precondition{Present: im.Present, Valid: im.Valid, Version: im.Version}
}

// etag returns the ETag header value of version.
func etag(version int64) *string {
	s := httpserver.FormatETag(version)
	return &s
}

// ListSpaces implements rest.StrictServerInterface (GET /spaces).
func (h *Spaces) ListSpaces(ctx context.Context, _ rest.ListSpacesRequestObject) (rest.ListSpacesResponseObject, error) {
	list, err := h.structure.ListSpaces(ctx, principal(ctx))
	if err != nil {
		return nil, err
	}
	out := rest.SpaceViewList{Items: make([]rest.SpaceView, 0, len(list))}
	for _, v := range list {
		out.Items = append(out.Items, viewREST(v))
	}
	return rest.ListSpaces200JSONResponse{Body: out}, nil
}

// CreateSpace implements rest.StrictServerInterface (POST /spaces).
func (h *Spaces) CreateSpace(ctx context.Context, req rest.CreateSpaceRequestObject) (rest.CreateSpaceResponseObject, error) {
	b := req.Body
	cmd := chat.CreateSpaceCmd{Name: b.Name, Icon: b.Icon, Color: b.Color}
	if b.AfterSpaceId != nil {
		cmd.AfterSpaceID = ids.ID(*b.AfterSpaceId)
	}
	sp, err := h.structure.CreateSpace(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.CreateSpace201JSONResponse{Body: spaceREST(sp)}, nil
}

// GetSpace implements rest.StrictServerInterface (GET /spaces/{space_id}).
func (h *Spaces) GetSpace(ctx context.Context, req rest.GetSpaceRequestObject) (rest.GetSpaceResponseObject, error) {
	v, err := h.structure.GetSpace(ctx, principal(ctx), ids.ID(req.SpaceId))
	if err != nil {
		return nil, err
	}
	return rest.GetSpace200JSONResponse{Body: viewREST(v), Headers: rest.GetSpace200ResponseHeaders{ETag: etag(v.Space.Version)}}, nil
}

// UpdateSpace implements rest.StrictServerInterface (PATCH /spaces/{space_id}).
func (h *Spaces) UpdateSpace(ctx context.Context, req rest.UpdateSpaceRequestObject) (rest.UpdateSpaceResponseObject, error) {
	b := req.Body
	sp, err := h.structure.UpdateSpace(ctx, principal(ctx), ids.ID(req.SpaceId), chat.UpdateSpaceCmd{
		Name: b.Name, Icon: b.Icon, Color: b.Color, IfMatch: precondition(req.Params.IfMatch),
	})
	if err != nil {
		return nil, err
	}
	return rest.UpdateSpace200JSONResponse{Body: spaceREST(sp), Headers: rest.UpdateSpace200ResponseHeaders{ETag: etag(sp.Version)}}, nil
}

// MoveSpace implements rest.StrictServerInterface (POST /spaces/{space_id}/move).
func (h *Spaces) MoveSpace(ctx context.Context, req rest.MoveSpaceRequestObject) (rest.MoveSpaceResponseObject, error) {
	var cmd chat.MoveSpaceCmd
	if b := req.Body; b != nil {
		if b.AfterId != nil {
			cmd.AfterID = ids.ID(*b.AfterId)
		}
		if b.BeforeId != nil {
			cmd.BeforeID = ids.ID(*b.BeforeId)
		}
	}
	sp, err := h.structure.MoveSpace(ctx, principal(ctx), ids.ID(req.SpaceId), cmd)
	if err != nil {
		return nil, err
	}
	return rest.MoveSpace200JSONResponse{Body: spaceREST(sp)}, nil
}

// ArchiveSpace implements rest.StrictServerInterface (POST /spaces/{space_id}/archive).
func (h *Spaces) ArchiveSpace(ctx context.Context, req rest.ArchiveSpaceRequestObject) (rest.ArchiveSpaceResponseObject, error) {
	sp, err := h.structure.ArchiveSpace(ctx, principal(ctx), ids.ID(req.SpaceId), req.Body.Reason)
	if err != nil {
		return nil, err
	}
	return rest.ArchiveSpace200JSONResponse{Body: spaceREST(sp)}, nil
}

// PutMySidebarSpace implements rest.StrictServerInterface (PUT /me/sidebar/spaces/{space_id}).
func (h *Spaces) PutMySidebarSpace(ctx context.Context, req rest.PutMySidebarSpaceRequestObject) (rest.PutMySidebarSpaceResponseObject, error) {
	b := req.Body
	cmd := chat.SidebarPutCmd{Collapsed: b.Collapsed, SortKey: b.SortKey, UseGlobalOrder: b.UseGlobalOrder != nil && *b.UseGlobalOrder}
	st, err := h.structure.PutSidebarSpace(ctx, principal(ctx), ids.ID(req.SpaceId), cmd)
	if err != nil {
		return nil, err
	}
	return rest.PutMySidebarSpace200JSONResponse{Body: SidebarREST(st)}, nil
}

// spaceREST converts a space (schema Space); empty values become absent members, never null (ADR
// 0053).
func spaceREST(s chat.Space) rest.Space {
	return rest.Space{
		Id: rest.Uuid(s.ID), Name: s.Name, Icon: optional(s.Icon), Color: optional(s.Color), SortKey: s.SortKey,
		DefaultOperatorAgentId: optionalID(s.DefaultOperatorAgentID), ArchivedAt: optionalTime(s.ArchivedAt),
		CreatedAt: s.CreatedAt.UTC().Truncate(time.Millisecond), Version: s.Version,
	}
}

// viewREST converts a space view (schema SpaceView).
func viewREST(v chat.SpaceView) rest.SpaceView {
	caps := make([]rest.Capability, 0, len(v.Capabilities))
	for _, c := range v.Capabilities {
		caps = append(caps, rest.Capability(c))
	}
	return rest.SpaceView{Space: spaceREST(v.Space), Level: rest.Level(v.Level), Capabilities: caps}
}

// SidebarREST converts a sidebar state (schema SidebarState); getMySidebar (M1-61) reuses it.
func SidebarREST(st chat.SidebarState) rest.SidebarState {
	return rest.SidebarState{
		SpaceId: rest.Uuid(st.SpaceID), Collapsed: st.Collapsed, SortKey: optional(st.SortKey), Version: st.Version,
		UpdatedByDeviceId: optionalID(st.UpdatedByDeviceID),
	}
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
