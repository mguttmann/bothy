// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// Triggerers is the incoming port of the allow list of an agent adapter (getAgentTriggerers,
// agent.triggerers.read, docs/spec/09-agents.md section 17.7).
type Triggerers interface {
	// Triggerers authorizes agent.triggerers.read on the calling agent a itself and returns the
	// people who may trigger it with the rights version of the request
	// (authorize.AgentTriggerers). It writes no audit entry.
	//
	// Errors: forbidden (subject_kind_not_allowed for people and apps), unauthenticated for an
	// anonymous caller, internal_error when no source is bound, the error of a source.
	//
	// Example:
	//
	//	set, err := svc.Triggerers(ctx, caller)
	Triggerers(ctx context.Context, a actor.Actor) (authorize.TriggererSet, error)
}
