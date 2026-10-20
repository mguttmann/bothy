// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ActiveGroups returns the data points of the gauge bothy.run.active (signals.md 4.3): the active
// runs per agent handle and state, from one grouped query without a principal (D-M2-05-9) and one
// read of the agent handles. An agent without a handle is left out. The worker role registers it
// (adapters/otelmetrics.RegisterActiveGauge, D-M2-05-15).
//
// Example:
//
//	groups, err := svc.ActiveGroups(ctx) // [{lotse running 2} ...]
func (s *Service) ActiveGroups(ctx context.Context) ([]ActiveGroup, error) {
	counts, err := s.d.Repo.ActiveCounts(ctx)
	if err != nil || len(counts) == 0 {
		return []ActiveGroup{}, err
	}
	agentIDs := make([]ids.ID, 0, len(counts))
	for _, c := range counts {
		agentIDs = append(agentIDs, c.AgentID)
	}
	facts, err := s.d.Agents.RunFacts(ctx, agentIDs)
	if err != nil {
		return nil, err
	}
	out := make([]ActiveGroup, 0, len(counts))
	for _, c := range counts {
		if h := facts[c.AgentID].Handle; h != "" {
			out = append(out, ActiveGroup{AgentHandle: h, State: c.State, Count: c.Count})
		}
	}
	return out, nil
}
