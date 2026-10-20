// SPDX-License-Identifier: Apache-2.0

package chat

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// BehaviorSettings are the writable behaviour of a channel (schema ChannelBehaviorPut): the
// mention switches, auto_thread, the context window, the channel prompt and the ordered skill
// references (03 section 4.5 channels and channel_skill_bindings; 09 "Channel switches").
type BehaviorSettings struct {
	RequireMention, ThreadRequireMention, FreeResponse, AutoThread, AllowMentionedAgents bool
	// HistoryBackfillLimit is the number of earlier messages a run sees (M2-13).
	HistoryBackfillLimit int
	// Prompt is the channel prompt; readable only with manage or as the effective operator.
	Prompt string
	// Skills are the ordered skill references; the server never interprets them.
	Skills []string
}

// ChannelBehavior is the behaviour of a channel with its settings version and, for a bound forum,
// its project binding (schema ChannelBehavior).
type ChannelBehavior struct {
	BehaviorSettings
	// SettingsVersion is channels.settings_version, the ETag of the behaviour.
	SettingsVersion int64
	// ProjectBinding is the binding of a bound forum (M2-03); nil otherwise, then the member is
	// absent.
	ProjectBinding *ProjectBinding
}

// PutChannelBehaviorCmd is the request of putChannelBehavior: the channel, the If-Match of the
// request (06 section 8) and the complete new settings.
type PutChannelBehaviorCmd struct {
	ChannelID ids.ID
	// IfMatch is the required precondition on settings_version.
	IfMatch  Precondition
	Settings BehaviorSettings
}

// ServedChannelConfig is the configuration an operator agent loads for a channel it serves
// (schema AgentChannelConfig).
type ServedChannelConfig struct {
	ChannelID, SpaceID ids.ID
	Behavior           ChannelBehavior
}

// AgentConfig is the incoming port of channel behaviour and agent configuration (02 section 4.3,
// M2-02): people with manage read and change the behaviour of a channel, and the effective
// operator agent reads the configuration of the channels it serves. Every method authorizes the
// caller a; an unknown and an unreadable channel give the same 404 not_found. Prompt and skills
// never leave through events, logs, traces, metrics, audit details or error bodies. Errors are
// kernel/errs errors.
type AgentConfig interface {
	// GetChannelBehavior returns the behaviour of the channel (channel.behavior.read, manage);
	// an archived channel stays readable. Errors: not_found, forbidden capability_missing.
	GetChannelBehavior(ctx context.Context, a actor.Actor, channelID ids.ID) (ChannelBehavior, error)
	// PutChannelBehavior replaces the behaviour of the channel (channel.behavior.update, manage,
	// step-up, If-Match on settings_version) and raises settings_version by one, with
	// channel.settings_changed and, when prompt or skills changed and the channel has an effective
	// operator, agent.config_changed to that agent. An unchanged state answers without a new
	// version. Errors: not_found, forbidden, step_up_required, precondition_required,
	// version_conflict with the current behaviour, validation_failed, conflict for an archived
	// channel.
	PutChannelBehavior(ctx context.Context, a actor.Actor, cmd PutChannelBehaviorCmd) (ChannelBehavior, error)
	// ListServedChannels returns the configuration of the channels that are not archived and
	// whose effective operator is the agent a, ascending by channel id (agent.config.read).
	// Errors: forbidden subject_kind_not_allowed for every other principal.
	ListServedChannels(ctx context.Context, a actor.Actor) ([]ServedChannelConfig, error)
	// GetServedChannelConfig returns the configuration of the channel when a is its effective
	// operator (agent.config.read); an archived channel stays readable. Errors: not_found,
	// forbidden subject_kind_not_allowed.
	GetServedChannelConfig(ctx context.Context, a actor.Actor, channelID ids.ID) (ServedChannelConfig, error)
}

// TriggerChannel are the facts of a channel the trigger decision of runs reads (09 section 3,
// M2-05): the effective operator now (03 channels "Effective operator") and the trigger switches
// of 09 section 3.3. It carries neither prompt nor skills.
type TriggerChannel struct {
	ChannelID, SpaceID ids.ID
	// Kind is text, forum or dm.
	Kind string
	// EffectiveOperatorID is the operator after inheritance; zero for none.
	EffectiveOperatorID ids.ID
	// The switches of 09 section 3.3.
	RequireMention, ThreadRequireMention, FreeResponse, AutoThread, AllowMentionedAgents bool
	// SettingsVersion is channels.settings_version, the member settings_version of
	// AgentInvocationPayload (M2-08).
	SettingsVersion int64
}

// TriggerFacts is the incoming port of the channel facts of the trigger decision (M2-05). It never
// authorizes: runs reads it inside the unit of work of a message whose author message.create
// allowed, and decides with agent.invoke. It reads inside the ambient transaction of ctx.
type TriggerFacts interface {
	// TriggerChannel returns the facts of the channel. Errors: not_found for an unknown channel.
	//
	// Example:
	//
	//	ch, err := facts.TriggerChannel(ctx, channelID)
	TriggerChannel(ctx context.Context, channelID ids.ID) (TriggerChannel, error)
}
