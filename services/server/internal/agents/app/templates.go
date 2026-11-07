// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain/definition"
	"github.com/OWNER/bothy/services/server/internal/agents/domain/interview"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// Compile-time checks: the incoming port of the template catalogue and the catalogue of the
// interview.
var (
	_ agents.Templates = (*TemplateService)(nil)
	_ CatalogueSource  = (*TemplateService)(nil)
)

// TemplateService implements agents.Templates over the templates the server read at start
// (adapters/templatefs.Load, docs/spec/26-builtin-agents.md 7.2). It is safe for concurrent use:
// the list never changes after NewTemplates.
type TemplateService struct {
	az    authorize.Authorizer
	items []agents.Template
}

// NewTemplates returns the catalogue of items, authorized by az. Errors: a missing Authorizer.
func NewTemplates(az authorize.Authorizer, items []agents.Template) (*TemplateService, error) {
	if az == nil {
		return nil, errors.New("agents: NewTemplates needs an Authorizer")
	}
	return &TemplateService{az: az, items: slices.Clone(items)}, nil
}

// ListTemplates implements agents.Templates: agent.templates.read on the instance (owner, no
// step-up, no audit; 07 2.13), then a copy of the catalogue, so no caller changes it.
func (s *TemplateService) ListTemplates(ctx context.Context, a actor.Actor) ([]agents.Template, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return nil, err
	}
	d, err := s.az.Authorize(ctx, a, authorize.ActionAgentTemplatesRead, instanceResource)
	if err != nil {
		return nil, err
	}
	if err := d.Err(); err != nil {
		return nil, err
	}
	return slices.Clone(s.items), nil
}

// InterviewCatalogue implements CatalogueSource: the templates in the order of ListTemplates as the
// catalogue the interview sends to the builder and checks a draft against (26 6.1, 7.4), each with
// slug, name, description, the tool class wish and the MCP servers of its definition. It reads no
// right: the interview authorized its caller already. Errors: a definition that is no stored
// definition (templatefs.Load checked every one, so this is a programming fault).
func (s *TemplateService) InterviewCatalogue(context.Context) ([]interview.CatalogueEntry, error) {
	out := make([]interview.CatalogueEntry, 0, len(s.items))
	for _, t := range s.items {
		d, err := definition.Decode(t.Definition)
		if err != nil {
			return nil, fmt.Errorf("agents: the definition of template %s: %w", t.Slug, err)
		}
		out = append(out, interview.CatalogueEntry{
			Slug: t.Slug, Name: t.Name, Description: t.Description, ToolClassWish: d.Settings.ToolClassWish, MCPServers: d.MCPServers,
		})
	}
	return out, nil
}
