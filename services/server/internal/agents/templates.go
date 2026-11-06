// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
)

// The sources of a template (schema AgentTemplate, docs/spec/26-builtin-agents.md 7.2).
const (
	// TemplateShipped is a template of the release (templates/agents/, embedded in the server).
	TemplateShipped = "shipped"
	// TemplatePrivate is a template of the folder agents.templates_dir of the installation.
	TemplatePrivate = "private"
)

// Template is one agent template as the server lists it (schema AgentTemplate, 26 7.1): the
// catalogue entry of template.yaml and the template as a definition draft.
type Template struct {
	// Slug is the folder name, unique across shipped and private templates.
	Slug string
	// Source is TemplateShipped or TemplatePrivate.
	Source string
	// Name and Description are one line each; Version is the template version.
	Name, Description, Version string
	// RequiredSecrets are the names the template's MCP servers name as env:<NAME>; never values.
	RequiredSecrets []string
	// Definition is the draft as agent_definition.v1 with origin.kind template, in the stored form
	// of the validator (definition.Definition.JSON).
	Definition json.RawMessage
}

// Templates is the incoming port of the template catalogue (26 7.2, A1-06): the shipped and the
// private templates the server read at start, listed to owners (action agent.templates.read).
type Templates interface {
	// ListTemplates returns the shipped templates by slug, then the private templates by slug.
	// Errors: the refusal of authorize (403 for a principal that is no owner).
	//
	// Example:
	//
	//	items, err := catalogue.ListTemplates(ctx, owner)
	ListTemplates(ctx context.Context, a actor.Actor) ([]Template, error)
}
