// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Register declares what the agents module emits and accepts (M1-25, docs/spec/05-aktionen.md
// sections 6 and 7): the durable event agent.updated (M1-51), the ephemeral event
// agent.presence_changed (M2-20) and the client op ack of agent consumers (M2-07). internal/wire
// lists it in moduleRegistrations.
func Register(r *authorize.Registry) {
	r.Event("agents", authorize.EventAgentUpdated)
	r.Event("agents", authorize.EventAgentPresenceChanged)
	r.Op("agents", authorize.OpAck)
}

// AgentDisabled is the in-transaction event of agent.disable (02 section 4.3): it is published
// inside the unit of work that disables the agent, after its tokens are revoked. No module
// subscribes in M1; runs (cancel the active runs with reason agent_disabled) and status subscribe
// later.
type AgentDisabled struct {
	// AgentID is the disabled agent.
	AgentID ids.ID
}

// The values of the backend policy, the tool class and the runtime state as the API names them
// (schemas BillingMode, ToolClass, AgentRuntimeState).
const (
	BillingPersonal     = "personal"
	BillingOrganization = "organization"
	ToolChat            = "chat"
	ToolScoped          = "scoped"
	ToolHost            = "host"
)

// Agent is an agent as the API shows it (schema Agent): the principal of identity with the
// record of agents. Zero values are absent members; nothing in it is a credential.
type Agent struct {
	ID ids.ID
	// Handle, DisplayName, AvatarFileID and Status come from the principal.
	Handle, DisplayName string
	AvatarFileID        ids.ID
	// Status is active, disabled or deleted.
	Status string
	// AdapterKind is the free identifier of the adapter.
	AdapterKind string
	// Description may be empty.
	Description string
	// BillingMode is personal or organization; CredentialHolderID is set exactly for personal.
	BillingMode        string
	CredentialHolderID ids.ID
	// BillingModeChangedAt is the time of the last switch; zero before one.
	BillingModeChangedAt time.Time
	// ToolClass is the target tool class.
	ToolClass string
	// Presence is the presence and reported state (M2-20: from Presence.Statuses).
	Presence PresenceStatus
	// OperatorConfig is agent_operator_config.v1 as a JSON object value.
	OperatorConfig map[string]any
	// Version is the ETag.
	Version int64
}

// Precondition is the If-Match header of a request (06 section 8): Present when the header was
// sent, Valid when it names one strong ETag, Version that ETag.
type Precondition struct {
	Present, Valid bool
	Version        int64
}

// Page asks for one page of a list: Cursor is next_cursor of the previous page, Limit the page
// size (0 for api.page_size_default, capped at api.page_size_max).
type Page struct {
	Cursor string
	Limit  int
}

// AgentPage is one page of agents ordered by id; NextCursor is "" on the last page.
type AgentPage struct {
	Items      []Agent
	NextCursor string
}

// CreateCmd is the body of createAgent (schema AgentCreate). OperatorConfig nil means the empty
// object.
type CreateCmd struct {
	Handle, DisplayName, AdapterKind, Description string
	OperatorConfig                                map[string]any
}

// UpdateCmd is the body of updateAgent (schema AgentUpdate); nil fields stay unchanged.
type UpdateCmd struct {
	DisplayName, Description *string
	// OperatorConfig replaces the whole configuration when not nil.
	OperatorConfig map[string]any
	// IfMatch is the required precondition.
	IfMatch Precondition
}

// BillingModeCmd is the body of setAgentBillingMode (schema BillingModeSet).
type BillingModeCmd struct {
	BillingMode string
	// CredentialHolderID is required for personal and refused for organization.
	CredentialHolderID ids.ID
	Reason             string
	IfMatch            Precondition
}

// BillingModeAllCmd is the body of setAllAgentsBillingMode (schema BillingModeSetAll).
type BillingModeAllCmd struct {
	BillingMode        string
	CredentialHolderID ids.ID
	// AgentIDs are the agents to switch; AgentIDsGiven tells an empty list from an absent one.
	AgentIDs      []ids.ID
	AgentIDsGiven bool
	Reason        string
}

// CredentialHolderCmd is the body of setCredentialHolder (schema CredentialHolderSet).
type CredentialHolderCmd struct {
	CredentialHolderID ids.ID
	Reason             string
	IfMatch            Precondition
}

// ToolClassCmd is the body of setAgentToolClass (schema ToolClassSet).
type ToolClassCmd struct {
	ToolClass, Reason string
	IfMatch           Precondition
}

// Agents is the incoming port of the agent records (M1-51, docs/spec/09-agenten.md section 2,
// docs/spec/10-backend-richtlinie.md section 9). Every method loads its resource, authorizes the
// caller a with the action of 05 section 4.10 and runs its writes in one unit of work with audit
// and agent.updated; the six actions with "Rechte" = ja run as a declared rights change, so I1
// holds after every call. A missing and an invisible agent give the same 404 not_found. Errors
// are kernel/errs errors; an invariant violation is an *authz.InvariantError (409 i1_violation).
//
// A call without effect (06 section 2 "Ohne Wirkung") answers the current agent, writes no new
// version, no event and no rights version, and audits no_change.
type Agents interface {
	// List returns one page of the agents the caller may read (agent.read per item, 07 section
	// 4.3), ascending by id, without totals. Errors: cursor_invalid.
	List(ctx context.Context, a actor.Actor, p Page) (AgentPage, error)
	// Get returns the agent id (agent.read: owner, the agent itself, a reader of a channel it
	// serves).
	Get(ctx context.Context, a actor.Actor, id ids.ID) (Agent, error)
	// Create creates an active agent principal with its record (agent.create): organization, tool
	// class chat, state unknown, version 1. Errors: validation_failed, conflict on /handle.
	Create(ctx context.Context, a actor.Actor, cmd CreateCmd) (Agent, error)
	// Update changes display name, description or operator configuration (agent.update).
	// Errors: precondition_required, version_conflict with current, validation_failed.
	Update(ctx context.Context, a actor.Actor, id ids.ID, cmd UpdateCmd) (Agent, error)
	// Disable disables the agent, revokes all its API tokens and publishes AgentDisabled
	// (agent.disable). Errors: validation_failed on /reason.
	Disable(ctx context.Context, a actor.Actor, id ids.ID, reason string) (Agent, error)
	// Enable enables the agent again; I1 is checked for it (agent.enable). Tokens stay revoked.
	Enable(ctx context.Context, a actor.Actor, id ids.ID, reason string) (Agent, error)
	// SetBillingMode switches the backend policy of one agent (agent.billing_mode.set, I4).
	// Errors: validation_failed (/credential_holder_id, /reason), version_conflict,
	// i1_violation.
	SetBillingMode(ctx context.Context, a actor.Actor, id ids.ID, cmd BillingModeCmd) (Agent, error)
	// SetBillingModeAll switches several agents in one transaction, all or nothing
	// (agent.billing_mode.set_all, BR-08), and returns the agents that actually changed,
	// ascending. Errors: validation_failed (/agent_ids/<index>, /credential_holder_id, /reason),
	// i1_violation.
	SetBillingModeAll(ctx context.Context, a actor.Actor, cmd BillingModeAllCmd) ([]ids.ID, error)
	// SetCredentialHolder changes the holder of a personal agent (agent.credential_holder.set).
	// Errors: conflict for an organization agent, validation_failed, version_conflict,
	// i1_violation.
	SetCredentialHolder(ctx context.Context, a actor.Actor, id ids.ID, cmd CredentialHolderCmd) (Agent, error)
	// SetToolClass sets the target tool class (agent.tool_class.set). Errors: validation_failed,
	// version_conflict.
	SetToolClass(ctx context.Context, a actor.Actor, id ids.ID, cmd ToolClassCmd) (Agent, error)
}
