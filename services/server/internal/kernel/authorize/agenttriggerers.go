// SPDX-License-Identifier: Apache-2.0

package authorize

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TriggererSet is the allow list of an agent adapter (docs/spec/09-agents.md section 17.7, schema
// AgentTriggerers): the people who may trigger the agent and the rights version the set was read
// at.
type TriggererSet struct {
	// PrincipalIDs are the people, ascending and without duplicates; never nil.
	PrincipalIDs []ids.ID
	// PermissionVersion is the rights version of the request. The set is read after it, so it
	// holds every change up to this version; a token with a higher version makes the adapter read
	// the set again.
	PermissionVersion int64
}

// AgentTriggerers answers which people may trigger an agent (agent.triggerers.read, 05 section
// 4.10). The authz module implements it; the agents module serves it as getAgentTriggerers, so
// agents never derives a level itself (05 rule 1.1). It authorizes nothing: the caller has
// authorized agent.triggerers.read for the agent before.
type AgentTriggerers interface {
	// Triggerers returns the trigger set of agent: the people of T(A) (07 section 6: active
	// owners and active people with effective trigger on a target the agent operates, without
	// those SR2 blocks for the agent's effective tool class), plus, for an agent with billing mode
	// organization, the people with trigger on a guest channel of the agent (05 section 5 point
	// 3: allow_mentioned_agents, not archived, not read-only, the agent has at least write); for
	// an agent with billing mode personal only the credential holder, when the set holds it (05
	// section 5 point 4). An unknown, inactive or policy-less agent has the empty set. It writes
	// no audit entry and no check-log entry. An error never means a partial set.
	//
	// Example:
	//
	//	set, err := at.Triggerers(ctx, agentID)
	Triggerers(ctx context.Context, agent ids.ID) (TriggererSet, error)
}
