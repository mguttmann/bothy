// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// Compile-time check: the incoming port of getAgentTriggerers.
var _ agents.Triggerers = (*Service)(nil)

// Triggerers implements agents.Triggerers: agent.triggerers.read on the agent a itself (agent-self,
// 05 section 4.10), then the set of the authz module (Deps.Triggerers), so this module never
// derives a level.
func (s *Service) Triggerers(ctx context.Context, a actor.Actor) (authorize.TriggererSet, error) {
	if s.d.Triggerers == nil {
		return authorize.TriggererSet{}, errs.New(errs.CodeInternalError, "agents: no trigger set bound")
	}
	if err := s.check(ctx, a, authorize.ActionAgentTriggerersRead, agentResource(a.ID)); err != nil {
		return authorize.TriggererSet{}, err
	}
	return s.d.Triggerers.Triggerers(ctx, a.ID)
}
