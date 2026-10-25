// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the service is the source of the backend view of stats (M6-14).
var _ agents.BackendSource = (*Service)(nil)

// BackendStates implements agents.BackendSource with one read of the records, in the ambient
// transaction of ctx when there is one. It never authorizes and copies no adapter kind, runtime,
// version, evidence or credential holder (ruling T8, I5).
func (s *Service) BackendStates(ctx context.Context, agentIDs []ids.ID) ([]agents.BackendState, error) {
	if len(agentIDs) == 0 {
		return []agents.BackendState{}, nil
	}
	recs, err := s.d.Store.Agents(ctx, agentIDs)
	if err != nil {
		return nil, err
	}
	out := make([]agents.BackendState, 0, len(recs))
	for _, rec := range recs {
		st := agents.BackendState{AgentID: rec.ID, BillingMode: string(rec.Policy.Mode), ReportedBillingMode: string(rec.Reported.BillingMode)}
		for _, m := range rec.Reported.BillingModesSupported {
			st.BillingModesSupported = append(st.BillingModesSupported, string(m))
		}
		if l := rec.Reported.RateLimit; l != nil {
			st.RateLimit = &agents.RateLimitState{State: string(l.State), ResetsAt: l.ResetsAt}
		}
		out = append(out, st)
	}
	slices.SortFunc(out, func(a, b agents.BackendState) int { return bytes.Compare(a.AgentID[:], b.AgentID[:]) })
	return out, nil
}
