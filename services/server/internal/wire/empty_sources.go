// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The empty sources of authz (ruling D2 of docs/briefs/M1-28-authz-core.md, 02 section 4.4):
// until the owning modules have tables they report the truth of an instance without those
// tables, so authorize decides fail closed. Every request with credentials is unauthenticated
// until M1-34, which is correct because no credential can exist before it.

// emptyStructure knows no space, channel or thread. M1-52 (spaces), M1-53 (channels) and M1-55
// (threads, dm participation) replace it with the StructureSource of chat.
type emptyStructure struct{}

var _ authz.StructureSource = emptyStructure{}

func (emptyStructure) Space(context.Context, ids.ID) (bool, error) { return false, nil }
func (emptyStructure) Spaces(context.Context) ([]ids.ID, error)    { return nil, nil }

// SpaceArchived knows no space until M1-52.
func (emptyStructure) SpaceArchived(context.Context, ids.ID) (bool, error) { return false, nil }
func (emptyStructure) Channel(context.Context, ids.ID) (authz.ChannelFacts, bool, error) {
	return authz.ChannelFacts{}, false, nil
}
func (emptyStructure) Thread(context.Context, ids.ID) (authz.ThreadFacts, bool, error) {
	return authz.ThreadFacts{}, false, nil
}
func (emptyStructure) Channels(context.Context) ([]authz.ChannelFacts, error)       { return nil, nil }
func (emptyStructure) ChannelsOperatedBy(context.Context, ids.ID) ([]ids.ID, error) { return nil, nil }
func (emptyStructure) DMChannelsOf(context.Context, ids.ID) ([]ids.ID, error)       { return nil, nil }

// ServedTargets knows no served target until M1-54 implements it in chat.
func (emptyStructure) ServedTargets(context.Context, ids.ID) ([]authz.ChannelRef, error) {
	return nil, nil
}

// emptyPolicies knows no agent policy and no ready dev environment. M1-51 replaces it with the
// AgentPolicySource of agents (dev environments in V2).
type emptyPolicies struct{}

var _ authz.AgentPolicySource = emptyPolicies{}

func (emptyPolicies) AgentPolicy(context.Context, ids.ID) (authz.AgentPolicy, bool, error) {
	return authz.AgentPolicy{}, false, nil
}
func (emptyPolicies) DevEnvironmentReady(context.Context, ids.ID, ids.ID) (bool, error) {
	return false, nil
}

// PersonalAgents knows no personal agent until M1-51 implements it in agents: without agents
// there is none, so I1 holds trivially.
func (emptyPolicies) PersonalAgents(context.Context) ([]authz.PersonalAgent, error) {
	return nil, nil
}

// emptyCredentials knows no live session, device or token. M1-33 (devices), M1-34 (sessions) and
// M1-44 (tokens) replace it with the CredentialSource of identity.
type emptyCredentials struct{}

var _ authz.CredentialSource = emptyCredentials{}

func (emptyCredentials) Credentials(context.Context, authz.CredentialRef) (authz.CredentialFacts, error) {
	return authz.CredentialFacts{DeviceState: authz.DeviceUnknown}, nil
}
