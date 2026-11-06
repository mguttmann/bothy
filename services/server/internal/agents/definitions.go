// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// AdapterKindBuiltin is the adapter kind of a built-in agent (docs/spec/26-builtin-agents.md 2,
// R-DM-58): only such an agent has a definition.
const AdapterKindBuiltin = "claude_agent"

// DefinitionVersion is one stored version of the definition of a built-in agent (schema
// AgentDefinitionVersion, 03 agent_definitions). It is never part of an event, a log line or an
// audit entry.
type DefinitionVersion struct {
	// AgentID is the built-in agent.
	AgentID ids.ID
	// Version is 1, 2, 3 ... per agent; the highest is the current definition.
	Version int64
	// OriginKind is template, interview or manual, a copy of definition.origin.kind.
	OriginKind string
	// CreatedAt is when the version was written; CreatedBy the owner who wrote it.
	CreatedAt time.Time
	CreatedBy ids.ID
	// Definition is the stored agent_definition.v1 document; nil in a list of versions.
	Definition json.RawMessage
}

// DefinitionVersionPage is one page of versions, newest first; NextCursor is "" on the last page.
type DefinitionVersionPage struct {
	Items      []DefinitionVersion
	NextCursor string
}

// PutDefinitionCmd is the request of putAgentDefinition: the body as sent (schema AgentDefinition)
// and If-Match of the current definition version, absent only for the first version.
type PutDefinitionCmd struct {
	Definition json.RawMessage
	IfMatch    Precondition
}

// Definitions is the incoming port of the definitions of built-in agents
// (docs/spec/26-builtin-agents.md 4, A1-02). Writing is part of agent.update (owner, step-up,
// audit); reading is agent.definition.read (owner or agent-self). A missing and an invisible
// agent give the same 404 not_found; errors are kernel/errs errors.
type Definitions interface {
	// PutDefinition checks cmd.Definition against 26 4.2 and 4.3 for the agent's tool class and
	// stores it as the next version: in one unit of work with the agents row locked first, it
	// raises agents.version (agent.updated), writes the audit entry of agent.update with the
	// version and the names of the changed parts, and appends agent.definition_changed with
	// definition_version to the agent. A definition equal to the current one stores nothing and
	// answers the current version. Errors: validation_failed with one field error per finding,
	// conflict with reason not_a_builtin_agent, precondition_required, version_conflict with the
	// current version.
	//
	// Example:
	//
	//	v, err := defs.PutDefinition(ctx, owner, agentID, agents.PutDefinitionCmd{Definition: raw})
	PutDefinition(ctx context.Context, a actor.Actor, agentID ids.ID, cmd PutDefinitionCmd) (DefinitionVersion, error)
	// GetDefinition returns the current version of the definition of agentID, or the stored
	// version when version is above zero (owners only; the agent itself reads only its current
	// version). Errors: not_found for an agent that is no built-in agent, has no definition or no
	// such version.
	GetDefinition(ctx context.Context, a actor.Actor, agentID ids.ID, version int64) (DefinitionVersion, error)
	// ListDefinitionVersions returns one page of the versions of agentID, newest first, without
	// the definition text. Errors: not_found for an agent that is no built-in agent,
	// cursor_invalid.
	ListDefinitionVersions(ctx context.Context, a actor.Actor, agentID ids.ID, p Page) (DefinitionVersionPage, error)
}
