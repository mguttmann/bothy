// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// WithTemplates binds the template catalogue (A1-06): listAgentTemplates answers internal_error
// until it is bound.
func (h *Agents) WithTemplates(t agents.Templates) *Agents {
	h.templates = t
	return h
}

// ListAgentTemplates implements rest.StrictServerInterface (GET /agent-templates): the shipped
// and private templates with their definition drafts (26 7.2).
func (h *Agents) ListAgentTemplates(ctx context.Context, _ rest.ListAgentTemplatesRequestObject) (rest.ListAgentTemplatesResponseObject, error) {
	if h.templates == nil {
		return nil, errs.New(errs.CodeInternalError, "agents: no template catalogue bound")
	}
	items, err := h.templates.ListTemplates(ctx, principal(ctx))
	if err != nil {
		return nil, err
	}
	out := rest.AgentTemplateList{Items: make([]rest.AgentTemplate, 0, len(items))}
	for _, t := range items {
		var def rest.AgentDefinition
		if err := json.Unmarshal(t.Definition, &def); err != nil {
			return nil, errs.Wrap(errs.CodeInternalError, err, "agents: a template draft outside its schema")
		}
		out.Items = append(out.Items, rest.AgentTemplate{
			Slug: t.Slug, Source: rest.AgentTemplateSource(t.Source), Name: t.Name, Description: t.Description,
			Version: t.Version, RequiredSecrets: slices.Clip(append([]string{}, t.RequiredSecrets...)), Definition: def,
		})
	}
	return rest.ListAgentTemplates200JSONResponse{Body: out}, nil
}
