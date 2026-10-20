// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the service is the agent facts port of runs as well (M2-05).
var _ agents.Facts = (*Service)(nil)

// RunFacts implements agents.Facts: one read of the records and one Directory.Principals call,
// both in the ambient transaction of ctx and without a unit of work of their own, so runs sees
// the facts of its own transaction. It never authorizes, because runs decides with agent.invoke
// and never shows the facts. Ids without an agent record (people, apps, unknown ids) are absent;
// a record without its principal is an internal error, as in List.
func (s *Service) RunFacts(ctx context.Context, agentIDs []ids.ID) (map[ids.ID]agents.RunFacts, error) {
	out := map[ids.ID]agents.RunFacts{}
	recs, err := s.d.Store.Agents(ctx, agentIDs)
	if err != nil || len(recs) == 0 {
		return out, err
	}
	list := make([]ids.ID, 0, len(recs))
	for _, rec := range recs {
		list = append(list, rec.ID)
	}
	principals, err := s.d.Directory.Principals(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, rec := range recs {
		p, ok := principals[rec.ID]
		if !ok {
			return nil, errs.New(errs.CodeInternalError, "agents: a record without its principal")
		}
		out[rec.ID] = agents.RunFacts{
			ID: rec.ID, Handle: p.Handle, Status: string(p.Status), BillingMode: string(rec.Policy.Mode),
			CredentialHolderID: rec.Policy.HolderID, ToolClass: string(rec.ToolClass),
		}
	}
	return out, nil
}
