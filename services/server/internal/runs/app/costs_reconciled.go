// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/billing"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// OnCostsReconciled is the in-transaction subscriber of billing.CostsReconciled (02 section 4.3;
// M2-28 design note 7), published by the job billing.reconcile: in ascending id it sets
// cost_micros, currency and cost_is_estimate of each run whose values differ, with version + 1,
// then hands the session totals to agents.Sessions.ApplyCostTotals (lock order runs before
// agent_sessions, 03 6.3). It writes no durable event: 05 section 6 has no event for costs, so
// clients see the reconciled amounts on the next read. Until Bind it does nothing. Errors: those
// of the database and of agents, which roll the job back so River retries it.
func (a *Acceptor) OnCostsReconciled(ctx context.Context, ev billing.CostsReconciled) error {
	s := a.svc.Load()
	if s == nil {
		return nil
	}
	return s.onCostsReconciled(ctx, ev)
}

// onCostsReconciled is OnCostsReconciled with the bound use cases.
func (s *Service) onCostsReconciled(ctx context.Context, ev billing.CostsReconciled) error {
	if err := needUnit(ctx, "OnCostsReconciled"); err != nil {
		return err
	}
	totals := slices.Clone(ev.Runs)
	slices.SortFunc(totals, func(a, b billing.RunCostTotal) int { return bytes.Compare(a.RunID[:], b.RunID[:]) })
	for _, t := range totals {
		if _, err := s.d.Repo.SetCostTotal(ctx, t); err != nil {
			return err
		}
	}
	if len(ev.Sessions) == 0 {
		return nil
	}
	if s.d.Sessions == nil {
		return errs.New(errs.CodeInternalError, "runs: reconciled session totals without the sessions of agents")
	}
	out := make([]agents.SessionCostTotal, len(ev.Sessions))
	for i, t := range ev.Sessions {
		out[i] = agents.SessionCostTotal{SessionID: t.SessionID, AmountMicros: t.AmountMicros, Currency: t.Currency, IsEstimate: t.IsEstimate}
	}
	return s.d.Sessions.ApplyCostTotals(ctx, out)
}
