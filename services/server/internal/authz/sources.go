// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// This file declares the source ports through which authz learns facts owned by other modules
// (docs/spec/02-architecture.md section 4.1 rule 4, "Source port"). authz never reads their
// tables. M1-28 adds CredentialSource, StructureSource and AgentPolicySource; M1-29 adds
// StructureSource.ServedTargets, ChannelFacts.Archived and AgentPolicySource.PersonalAgents. Until
// their owning modules have tables, internal/wire passes empty sources (ruling D2 of
// docs/briefs/M1-28-authz-core.md).

// PrincipalStatus is the lifecycle state of a principal as authz sees it.
type PrincipalStatus string

const (
	// PrincipalPending is a person waiting for approval: forbidden except for self information
	// (docs/spec/07-permissions.md SR7).
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
	// HasActivePasskey reports whether the person userID has a passkey with revoked_at IS NULL:
	// hat_anmeldedaten of passkey.register, whose step-up is conditional (07 section 4.2 step 8,
	// M1-40). It reads inside the ambient transaction of ctx when there is one.
	HasActivePasskey(ctx context.Context, userID ids.ID) (bool, error)
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
	// Archived reports that the channel or its space is archived: every effective level there is
	// at most read, owners included (07 section 4.5 point 1). chat reports it from M1-52 and M1-53.
	Archived bool
}

// ChannelRef is one target an agent serves (10 section 5.2, 03 section 6.4): a channel with its
// space, or a space alone (ChannelID zero) whose default operator is the agent.
type ChannelRef struct {
	// ChannelID is the channel; zero for a space served through its default operator only.
	ChannelID ids.ID
	// SpaceID is the space of the channel, or the served space.
	SpaceID ids.ID
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
	// SpaceArchived reports whether the space is archived; false for an unknown space. A grant
	// on an archived space is refused with 409 conflict (M1-29); chat implements it with M1-52.
	SpaceArchived(ctx context.Context, id ids.ID) (bool, error)
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
	// ServedTargets returns the targets agentID serves for I1 (10 sections 5.2 and 5.3, 03
	// section 6.4, BR-02): channels whose effective operator is the agent and that are not
	// archived; channels that are not archived and hold at least one thread with kind thread and
	// operator_agent_id equal to the agent, open or closed; and, with a zero ChannelID, every
	// space that is not archived whose default operator is the agent, also without a channel.
	// Guest channels never count. Ascending by (SpaceID, ChannelID), without duplicates. chat
	// implements it with M1-54 (M1-29).
	ServedTargets(ctx context.Context, agentID ids.ID) ([]ChannelRef, error)
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

// PersonalAgent is an active agent with billing_mode = 'personal' and its credential holder
// (03 agents; I1 in 07 SR1).
type PersonalAgent struct {
	// AgentID is the agent.
	AgentID ids.ID
	// CredentialHolderID is agents.credential_holder_id; the zero id if the row has none, which
	// is itself a violation of I1.
	CredentialHolderID ids.ID
}

// AgentPolicySource gives authz the policy facts of agents. The agents module implements it
// (M1-51; dev environments in V2).
type AgentPolicySource interface {
	// PersonalAgents returns every agent with principal status active and billing_mode =
	// 'personal', ascending by AgentID; VerifyInvariants checks I1 for each (M1-29). It reads
	// inside the ambient transaction of ctx. agents implements it with M1-51.
	PersonalAgents(ctx context.Context) ([]PersonalAgent, error)
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

// CredentialSource gives authz the liveness of credentials (ruling D1 of
// docs/briefs/M1-28-authz-core.md). The identity module implements it (M1-33 devices, M1-34
// sessions, M1-44 tokens).
type CredentialSource interface {
	// Credentials returns the facts of ref; ids that are zero in ref give zero facts.
	Credentials(ctx context.Context, ref CredentialRef) (CredentialFacts, error)
}
