// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// WithDefinitions binds the definitions of built-in agents (A1-02): getAgentDefinition,
// listAgentDefinitionVersions and putAgentDefinition answer internal_error until it is bound.
func (h *Agents) WithDefinitions(d agents.Definitions) *Agents {
	h.definitions = d
	return h
}

// defs returns the bound port or the internal error of a missing binding.
func (h *Agents) defs() (agents.Definitions, error) {
	if h.definitions == nil {
		return nil, errs.New(errs.CodeInternalError, "agents: no definitions bound")
	}
	return h.definitions, nil
}

// GetAgentDefinition implements rest.StrictServerInterface (GET /agents/{agent_id}/definition).
// ETag is the version of the answer.
func (h *Agents) GetAgentDefinition(ctx context.Context, req rest.GetAgentDefinitionRequestObject) (rest.GetAgentDefinitionResponseObject, error) {
	d, err := h.defs()
	if err != nil {
		return nil, err
	}
	var version int64
	if req.Params.Version != nil {
		version = *req.Params.Version
	}
	v, err := d.GetDefinition(ctx, principal(ctx), ids.ID(req.AgentId), version)
	if err != nil {
		return nil, err
	}
	body, err := versionREST(v)
	if err != nil {
		return nil, err
	}
	return rest.GetAgentDefinition200JSONResponse{Body: body, Headers: rest.GetAgentDefinition200ResponseHeaders{ETag: versionETag(v)}}, nil
}

// PutAgentDefinition implements rest.StrictServerInterface (PUT /agents/{agent_id}/definition).
// The chain checked the body against the closed schema; the use case checks the rules of 26 4.3
// on the JSON form of the decoded body, which holds exactly the members the request sent.
func (h *Agents) PutAgentDefinition(ctx context.Context, req rest.PutAgentDefinitionRequestObject) (rest.PutAgentDefinitionResponseObject, error) {
	d, err := h.defs()
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "the body is required").
			WithFields(errs.FieldError{Pointer: "", Code: "required", Detail: "The body is required."})
	}
	raw, err := json.Marshal(req.Body)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "agents: encoding a definition body")
	}
	var match string
	if req.Params.IfMatch != nil {
		match = *req.Params.IfMatch
	}
	v, err := d.PutDefinition(ctx, principal(ctx), ids.ID(req.AgentId), agents.PutDefinitionCmd{Definition: raw, IfMatch: precondition(match)})
	if err != nil {
		return nil, err
	}
	body, err := versionREST(v)
	if err != nil {
		return nil, err
	}
	return rest.PutAgentDefinition200JSONResponse{Body: body, Headers: rest.PutAgentDefinition200ResponseHeaders{ETag: versionETag(v)}}, nil
}

// ListAgentDefinitionVersions implements rest.StrictServerInterface (GET
// /agents/{agent_id}/definition/versions).
func (h *Agents) ListAgentDefinitionVersions(ctx context.Context, req rest.ListAgentDefinitionVersionsRequestObject) (rest.ListAgentDefinitionVersionsResponseObject, error) {
	d, err := h.defs()
	if err != nil {
		return nil, err
	}
	var p agents.Page
	if req.Params.Cursor != nil {
		p.Cursor = *req.Params.Cursor
	}
	if req.Params.Limit != nil {
		p.Limit = *req.Params.Limit
	}
	page, err := d.ListDefinitionVersions(ctx, principal(ctx), ids.ID(req.AgentId), p)
	if err != nil {
		return nil, err
	}
	out := rest.AgentDefinitionVersionPage{Items: make([]rest.AgentDefinitionVersion, 0, len(page.Items))}
	for _, v := range page.Items {
		item, err := versionREST(v)
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, item)
	}
	if page.NextCursor != "" {
		out.NextCursor = &page.NextCursor
	}
	return rest.ListAgentDefinitionVersions200JSONResponse{Body: out}, nil
}

// versionETag returns the ETag of a definition version.
func versionETag(v agents.DefinitionVersion) *string {
	s := httpserver.FormatETag(v.Version)
	return &s
}

// versionREST converts a version (schema AgentDefinitionVersion); the definition only when the
// version carries it. A stored document that does not decode is an internal error.
func versionREST(v agents.DefinitionVersion) (rest.AgentDefinitionVersion, error) {
	out := rest.AgentDefinitionVersion{
		AgentId: rest.Uuid(v.AgentID), Version: v.Version, OriginKind: rest.AgentDefinitionVersionOriginKind(v.OriginKind),
		CreatedAt: v.CreatedAt.UTC().Truncate(time.Millisecond), CreatedBy: rest.Uuid(v.CreatedBy),
	}
	if len(v.Definition) > 0 {
		var def rest.AgentDefinition
		if err := json.Unmarshal(v.Definition, &def); err != nil {
			return rest.AgentDefinitionVersion{}, errs.Wrap(errs.CodeInternalError, err, "agents: a stored definition outside its schema")
		}
		out.Definition = &def
	}
	return out, nil
}
