// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// Compile-time check: the incoming port of the template catalogue.
var _ agents.Templates = (*TemplateService)(nil)

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
