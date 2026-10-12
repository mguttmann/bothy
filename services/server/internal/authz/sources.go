// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// This file declares the source ports through which authz learns facts owned by other modules
// (docs/spec/02-architektur.md section 4.1 rule 4, "Quellen-Port"). authz never reads their
// tables. M1-28 adds CredentialSource, StructureSource and AgentPolicySource; until their owning
// modules have tables, internal/wire passes empty sources (docs/DECISIONS.md M1-28 D2).

// PrincipalStatus is the lifecycle state of a principal as authz sees it.
type PrincipalStatus string

const (
	// PrincipalPending is a person waiting for approval: forbidden except for self information
	// (docs/spec/07-rechte.md SR7).
	PrincipalPending PrincipalStatus = "pending"
	// PrincipalActive is a principal whose grants take effect.
	PrincipalActive PrincipalStatus = "active"
	// PrincipalDisabled is a disabled principal: unauthenticated before any grant is read (SR7).
	PrincipalDisabled PrincipalStatus = "disabled"
	// PrincipalDeleted is a deleted principal: unauthenticated like a disabled one (SR7).
	PrincipalDeleted PrincipalStatus = "deleted"
)

// PrincipalFacts are the facts of one principal that authorize needs.
type PrincipalFacts struct {
	// ID is the principal.
	ID ids.ID
	// Kind is user, agent or app.
	Kind actor.Kind
	// Status is the lifecycle state.
	Status PrincipalStatus
	// IsOwner reports an owners row with revoked_at IS NULL, independent of Status; whether the
	// owner role takes effect (active status, a session instead of a personal token, SR5) is
	// for authz to decide.
	IsOwner bool
}

// PrincipalSource gives authz the facts of principals. The identity module implements it
// (identity/app); internal/wire passes it to authz (M1-28).
type PrincipalSource interface {
	// Principal returns the facts of one principal; ok is false when the id is unknown. It
	// reads inside the ambient transaction of ctx when there is one.
	Principal(ctx context.Context, id ids.ID) (PrincipalFacts, bool, error)
	// ActiveOwnerIDs returns the users with an active owners row whose principal status is
	// active, ascending by id. I1 and SR8 (M1-29) need this set.
	ActiveOwnerIDs(ctx context.Context) ([]ids.ID, error)
}

// ChannelKind is channels.kind.
type ChannelKind string

// The channel kinds of 03 section 4.5.
const (
	ChannelText  ChannelKind = "text"
	ChannelForum ChannelKind = "forum"
	ChannelDM    ChannelKind = "dm"
)

// ChannelFacts are the facts of one channel that authorize needs (03 section 4.5).
type ChannelFacts struct {
	// ID is the channel; SpaceID its space, zero for a dm channel.
	ID, SpaceID ids.ID
	// Kind is text, forum or dm.
	Kind ChannelKind
	// ReadOnly is channels.read_only (05 rule 1.8).
	ReadOnly bool
	// EffectiveOperatorID is operator_inherit ? spaces.default_operator_agent_id :
	// operator_agent_id; zero for none.
	EffectiveOperatorID ids.ID
	// The mention rule of the channel (05 section 5 point 3).
	RequireMention, ThreadRequireMention, FreeResponse, AllowMentionedAgents bool
	// ProjectBound reports a project_bindings row; ProjectReady its ready flag.
	ProjectBound, ProjectReady bool
	// DMParticipants are the users with left_at IS NULL of a dm channel, ascending.
	DMParticipants []ids.ID
}

// ThreadFacts are the facts of one thread (03 section 4.5).
type ThreadFacts struct {
	// ID is the thread; ChannelID its channel.
	ID, ChannelID ids.ID
	// Main reports threads.kind = 'main'.
	Main bool
	// OperatorAgentID is threads.operator_agent_id; zero for main threads.
	OperatorAgentID ids.ID
	// CreatedBy is the creator.
	CreatedBy ids.ID
}

// StructureSource gives authz the structure: spaces, channels, threads and dm participation. The
// chat module implements it (M1-52, M1-53, M1-55). Every method reads fresh, inside the ambient
// transaction of ctx when there is one; lists are ascending by id.
type StructureSource interface {
	// Space reports whether the space exists.
	Space(ctx context.Context, id ids.ID) (ok bool, err error)
	// Spaces returns every space, ascending (Scope lists the spaces of an instance-wide reader).
	Spaces(ctx context.Context) ([]ids.ID, error)
	// Channel returns the facts of one channel; ok is false for an unknown id.
	Channel(ctx context.Context, id ids.ID) (ChannelFacts, bool, error)
	// Thread returns the facts of one thread; ok is false for an unknown id.
	Thread(ctx context.Context, id ids.ID) (ThreadFacts, bool, error)
	// Channels returns every channel that is not a dm channel.
	Channels(ctx context.Context) ([]ChannelFacts, error)
	// ChannelsOperatedBy returns the channels whose effective operator is agentID.
	ChannelsOperatedBy(ctx context.Context, agentID ids.ID) ([]ids.ID, error)
	// DMChannelsOf returns the dm channels userID takes part in.
	DMChannelsOf(ctx context.Context, userID ids.ID) ([]ids.ID, error)
}

// AgentPolicy are the backend policy and tool class of an agent (03 agents, 07 sections 2.11 and
// SR1).
type AgentPolicy struct {
	// BillingMode is personal or organization.
	BillingMode string
	// CredentialHolderID is set exactly for personal.
	CredentialHolderID ids.ID
	// ToolClass is chat, scoped or host.
	ToolClass string
	// ReportedToolClass is the class the last heartbeat reported; "" for none.
	ReportedToolClass string
}

// AgentPolicySource gives authz the policy facts of agents. The agents module implements it
// (M1-51; dev environments in V2).
type AgentPolicySource interface {
	// AgentPolicy returns the policy of one agent; ok is false for an unknown agent.
	AgentPolicy(ctx context.Context, agentID ids.ID) (AgentPolicy, bool, error)
	// DevEnvironmentReady reports a ready own dev environment of userID for channelID (SR3);
	// false until V2.
	DevEnvironmentReady(ctx context.Context, userID, channelID ids.ID) (bool, error)
}

// CredentialRef names the credentials of a request principal.
type CredentialRef struct {
	PrincipalID, SessionID, DeviceID, TokenID ids.ID
}

// DeviceState is the state of a device as authz sees it.
type DeviceState string

// The device states.
const (
	DevicePending DeviceState = "pending"
	DeviceActive  DeviceState = "active"
	DeviceRevoked DeviceState = "revoked"
	DeviceUnknown DeviceState = "unknown"
)

// CredentialFacts are the liveness facts of session, device and token (07 section 4.2 step 1).
type CredentialFacts struct {
	// SessionActive reports a session that exists, belongs to PrincipalID and DeviceID and has
	// revoked_at IS NULL; SessionExpiresAt is its end.
	SessionActive    bool
	SessionExpiresAt time.Time
	// DeviceState is the state of the device; unknown for an id that does not exist or belongs to
	// someone else.
	DeviceState DeviceState
	// TokenActive reports a token that exists, belongs to PrincipalID and has revoked_at IS NULL;
	// TokenExpiresAt is its expiry, zero for none.
	TokenActive    bool
	TokenExpiresAt time.Time
}

// CredentialSource gives authz the liveness of credentials (ruling D1). The identity module
// implements it (M1-33 devices, M1-34 sessions, M1-44 tokens).
type CredentialSource interface {
	// Credentials returns the facts of ref; ids that are zero in ref give zero facts.
	Credentials(ctx context.Context, ref CredentialRef) (CredentialFacts, error)
}
