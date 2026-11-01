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
// StructureSource.ServedTargets, ChannelFacts.Archived and AgentPolicySource.PersonalAgents; M1-49
// adds PrincipalSource.Profiles and OwnerIDs, StructureSource.SpaceDetails and ChannelDetails and
// AgentPolicy.BillingModesSupported for "who has access", the preview and "view as"; M2-39 adds
// HostSource; M2-40 adds HostSource.PersonalHosts and SessionUsersOf and the FreshStepUpCheck.
// Until their owning modules have tables, internal/wire passes empty sources (ruling D2 of
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
	// AppInstallationActive reports, for a principal of kind app, that its installation is
	// active (app_installations.state = 'active'); identity reads it through
	// identity.AppInstallations (M2-24). Ignored for other kinds. An app without an active
	// installation has an empty rights snapshot, like a principal that is not active (07 section
	// 2.10), so app.suspend and app.uninstall remove its visible targets and app.resume adds them.
	AppInstallationActive bool
	// AppGrantedScopes are app_installations.granted_scopes of the active installation of an app
	// (empty without one). authz reads them for an app that owns a command or a card (M2-25: the
	// app-owner guard of docs/spec/05-actions.md section 5) and for the receive rule of personal
	// events to apps, where the facts of the owner, not of a request, decide. Ignored for other
	// kinds.
	AppGrantedScopes []string
}

// GrantsInEffect reports whether the grants of the principal take effect: status active or
// pending, and for an app an active installation. authz builds the snapshot only for such a
// principal (SR7, 07 section 2.10).
func (f PrincipalFacts) GrantsInEffect() bool {
	if f.Status != PrincipalActive && f.Status != PrincipalPending {
		return false
	}
	return f.Kind != actor.KindApp || f.AppInstallationActive
}

// PrincipalRef is the minimal public view of a principal that authz embeds in its answers, for
// example the user of a group member (OpenAPI PrincipalRef, M1-48).
type PrincipalRef struct {
	// ID is the principal.
	ID ids.ID
	// Kind is user, agent or app.
	Kind actor.Kind
	// Handle and DisplayName are the public names.
	Handle, DisplayName string
	// AvatarFileID is the avatar file; zero for none.
	AvatarFileID ids.ID
}

// PrincipalProfile describes one principal for the answers of access.explain, access.preview and
// access.view_as (M1-49): the public view, the status and, for people, users.locale and
// users.time_zone (empty for agents and apps).
type PrincipalProfile struct {
	PrincipalRef
	// Status is the lifecycle state.
	Status PrincipalStatus
	// Locale is the BCP 47 tag of users.locale; TimeZone the IANA name of users.time_zone.
	Locale, TimeZone string
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
	// HasEstablishedLocalCredential reports whether the person userID has an established password
	// path: a local_credentials row with a confirmed TOTP enrollment (07 section 4.2 step 8,
	// has_credentials of local_credential.set; M1-39). The rule itself stays in authz.
	HasEstablishedLocalCredential(ctx context.Context, userID ids.ID) (bool, error)
	// Refs returns the public views of the known principals among principals, whatever their status;
	// an unknown id is absent from the map (M1-48, the members of a group). It reads inside the
	// ambient transaction of ctx when there is one.
	Refs(ctx context.Context, principals []ids.ID) (map[ids.ID]PrincipalRef, error)
	// Profiles returns the profiles of the known principals among principals, whatever their
	// status; an unknown id is absent from the map (M1-49). It reads inside the ambient
	// transaction of ctx when there is one.
	Profiles(ctx context.Context, principals []ids.ID) (map[ids.ID]PrincipalProfile, error)
	// OwnerIDs returns the users with an owners row that is not revoked, whatever their status,
	// ascending by id: "who has access" lists inactive owners too (07 section 9.1, M1-49).
	OwnerIDs(ctx context.Context) ([]ids.ID, error)
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

// SpaceDetail is what the answers of authz show of a space (OpenAPI Space; M1-49): its name and
// presentation, its default operator and its lifecycle.
type SpaceDetail struct {
	// ID is the space.
	ID ids.ID
	// Name, Icon, Color and SortKey are the columns of spaces; Icon and Color may be empty.
	Name, Icon, Color, SortKey string
	// DefaultOperatorID is spaces.default_operator_agent_id; zero for none.
	DefaultOperatorID ids.ID
	// ArchivedAt is zero for a live space.
	ArchivedAt time.Time
	// Version is spaces.version; CreatedAt its creation.
	Version   int64
	CreatedAt time.Time
}

// The operator modes of a channel (OpenAPI OperatorMode).
const (
	OperatorInherit = "inherit"
	OperatorNone    = "none"
	OperatorAgent   = "agent"
)

// ChannelDetail is what the answers of authz show of a channel (OpenAPI Channel; M1-49).
type ChannelDetail struct {
	// ID is the channel; SpaceID its space, zero for a dm channel.
	ID, SpaceID ids.ID
	// Kind is text, forum or dm.
	Kind ChannelKind
	// Name, Topic and SortKey are the columns of channels.
	Name, Topic, SortKey string
	// OperatorMode is inherit, none or agent; OperatorAgentID the agent of mode agent and
	// EffectiveOperatorID the operator after inheritance (zero for none).
	OperatorMode                         string
	OperatorAgentID, EffectiveOperatorID ids.ID
	// ForumDefaultSort and DefaultNotificationLevel are empty when not set.
	ForumDefaultSort, DefaultNotificationLevel string
	// ReadOnly is channels.read_only.
	ReadOnly bool
	// ProjectBound reports a project_bindings row of the forum (Channel.project_bound, R-DM-54).
	ProjectBound bool
	// MainThreadID is the main thread of text and dm channels; zero for a forum.
	MainThreadID ids.ID
	// DMParticipantIDs are the participants of a dm channel, ascending.
	DMParticipantIDs []ids.ID
	// ArchivedAt is zero for a live channel.
	ArchivedAt time.Time
	// Version, SettingsVersion and CreatedAt are the columns of channels.
	Version, SettingsVersion int64
	CreatedAt                time.Time
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
	// Deleted reports a deleted thread (03 threads.deleted_at, D-DEL-3): authorize answers
	// not_found for it, except for the actions that read or undo the deletion (07 4.5a).
	Deleted bool
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
	// SpaceDetails returns the details of the known spaces among spaceIDs, archived or not; an
	// unknown id is absent (M1-49: names, default operators and the Space objects of "view as").
	SpaceDetails(ctx context.Context, spaceIDs []ids.ID) (map[ids.ID]SpaceDetail, error)
	// ChannelDetails returns the details of the known channels among channelIDs, archived or not;
	// an unknown id is absent (M1-49).
	ChannelDetails(ctx context.Context, channelIDs []ids.ID) (map[ids.ID]ChannelDetail, error)
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
	// BillingModesSupported are the billing modes the adapter reported it can prove
	// (agents.reported_details); organization among them makes an I1 conflict organization_ready
	// (10 section 11, M1-49).
	BillingModesSupported []string
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

// HostFacts are the facts of one terminal host that authz needs (02 section 4.3: state, holder,
// LLM declaration, server host of the host). The terminal module implements the source (M2-39).
type HostFacts struct {
	// ID is the host.
	ID ids.ID
	// State is pairing, confirming, active, disabled or retired (24 section 3.1).
	State string
	// AssignedUserID is the holder (hosts.assigned_user_id, SR10); zero means owners only.
	AssignedUserID ids.ID
	// LLMCredentials is none, personal or organization; SR11 and I6 read it (M2-40).
	LLMCredentials string
	// CredentialHolderID is set exactly with personal; SR11 and I6 read it (M2-40).
	CredentialHolderID ids.ID
	// DeclaredServerHost is the owner's declaration that the server runs on the host (SR10).
	DeclaredServerHost bool
	// ReportedIsServerHost is the report of the host service; false while not reported.
	ReportedIsServerHost bool
	// Name is the display name of the host, only for the plain text of a preview (M2-40); never
	// in an audit entry, an event or a problem.
	Name string
}

// SessionUserFact is a valid assignment of a person to a session user of a host (07 SR12, M2-40):
// host_session_users with revoked_at IS NULL; Reported reports that the host names OSUser in
// hosts.reported_session_users, compared exactly (case included, 24 section 5.3).
type SessionUserFact struct {
	// HostID is the host; HostSessionUserID the assignment.
	HostID, HostSessionUserID ids.ID
	// OSUser is the name of the session user.
	OSUser string
	// Reported reports the name among the reported session users of the host.
	Reported bool
}

// HostRetired is the state of a retired host (24 section 3.1): final, no grant can work on it.
const HostRetired = "retired"

// HostSource gives authz the facts of terminal hosts, the target outside the tree instance,
// space, channel (07 section 2.12). The terminal module implements it (terminal/app,
// M2-39); internal/wire binds it. Every method reads fresh, inside the ambient transaction of ctx
// when there is one: host facts are few and change rarely, and a stale cache would decide on an
// old holder. Lists are ascending by id.
type HostSource interface {
	// Host returns the facts of one host; ok is false for an unknown id.
	Host(ctx context.Context, id ids.ID) (HostFacts, bool, error)
	// HostIDs returns every host, retired ones included.
	HostIDs(ctx context.Context) ([]ids.ID, error)
	// AssignedTo returns the hosts whose holder is userID.
	AssignedTo(ctx context.Context, userID ids.ID) ([]ids.ID, error)
	// PersonalHosts returns the hosts with llm_credentials = personal, ascending by id: the I6
	// check of every rights change and the effective right terminal of owners (M2-40).
	PersonalHosts(ctx context.Context) ([]HostFacts, error)
	// SessionUsersOf returns the valid session user assignments of userID with Reported, ascending
	// by host (07 SR12, M2-40); at most one per host (24 section 2.2).
	SessionUsersOf(ctx context.Context, userID ids.ID) ([]SessionUserFact, error)
}

// FreshStepUpFacts are the step-up facts of a login session (auth_sessions.step_up_at,
// step_up_consumed_at, step_up_method and client_component; zero for NULL) that the fresh step-up
// of the terminal judges (05 section 2.4, 08 section 6.3; M2-40).
//
// StepUpUserVerified reports that the step-up of StepUpAt was a passkey whose stored WebAuthn
// session required user verification; identity reads it from auth_challenges.webauthn_session of
// the challenge for the console rule (08 section 6.3 point 3, M6-18).
type FreshStepUpFacts struct {
	StepUpAt, StepUpConsumedAt    time.Time
	StepUpMethod, ClientComponent string
	StepUpUserVerified            bool
}

// FreshStepUpCheck judges a fresh step-up with the rule authorize uses in step 8 (07 section 4.2)
// and terminal.step_up_max_age. authz implements it (authz/app); internal/wire passes it to
// identity, whose StepUp.ConsumeFresh checks the locked session row again with it, so both checks
// are one rule.
type FreshStepUpCheck interface {
	// Fresh reports whether f is fresh and unconsumed at now; in the web console only a passkey
	// with user verification is (08 section 6.3 points 1 to 4).
	Fresh(f FreshStepUpFacts, now time.Time) bool
	// PasskeyRequired reports a web console step-up that is no passkey with user verification,
	// the case that answers with reason passkey_required (08 section 6.3 point 3, M6-18).
	PasskeyRequired(f FreshStepUpFacts) bool
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

// AgentDirectory lists the agents for the list "Permission hints" (M6-07): H1 is computed for
// every agent, also one that serves a channel only through the threads it operates. The agents
// module implements it (agents/adapters/authzsource).
type AgentDirectory interface {
	// AgentIDs returns every agent, whatever its status, ascending by id. It reads inside the
	// ambient transaction of ctx when there is one.
	AgentIDs(ctx context.Context) ([]ids.ID, error)
}

// ExpiringToken is an API token of an agent or an app as "Expiring soon" shows it (M6-07): its
// id, its holder and its expiry, never the token value, the hash or the prefix.
type ExpiringToken struct {
	ID, PrincipalID ids.ID
	ExpiresAt       time.Time
}

// TokenExpirySource gives authz the API tokens that expire soon (07 section 9.4, M6-07). The
// identity module implements it (identity/app TokenExpiry); token values never leave identity.
type TokenExpirySource interface {
	// ExpiringTokens returns the tokens with revoked_at IS NULL whose holder is an agent or an app
	// and whose expires_at lies in (t, until], ordered by expires_at, then id. Personal tokens
	// never appear.
	ExpiringTokens(ctx context.Context, t, until time.Time) ([]ExpiringToken, error)
}
