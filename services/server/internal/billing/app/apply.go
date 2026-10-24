// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log/slog"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/billing"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// The log record of a run or session the job leaves unchanged.
const (
	msgMixedCurrencies = "billing.reconcile: mixed currencies, totals unchanged"
	logKeyRunID        = "bothy.run.id"
	logKeySessionID    = "bothy.agent_session.id"
)

// ApplyReconciliation is the use case of the job billing.reconcile (M2-28 design note 6) in one
// unit of work: the shared advisory lock of each run in ascending order, then the valid totals of
// the runs and of their agent sessions over all valid entries, published as the in-transaction
// event billing.CostsReconciled, to which runs subscribes. It reads its state again on every run,
// so a repeated job has no second effect. A run or session whose valid entries mix currencies is
// left out and logged by ID without amounts (no conversion, 10 section 12.1). It never authorizes:
// the request that inserted the job did.
//
// Example:
//
//	err := svc.ApplyReconciliation(ctx, []ids.ID{run})
func (s *Service) ApplyReconciliation(ctx context.Context, runIDs []ids.ID) error {
	sorted := slices.Clone(runIDs)
	slices.SortFunc(sorted, compareIDs)
	sorted = slices.Compact(sorted)
	if len(sorted) == 0 {
		return nil
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.d.Repo.LockReconcileShared(ctx, sorted); err != nil {
			return err
		}
		runs, err := s.d.Repo.RunTotals(ctx, sorted)
		if err != nil {
			return err
		}
		sessions, err := s.d.Repo.SessionTotals(ctx, sorted)
		if err != nil {
			return err
		}
		ev := billing.CostsReconciled{Runs: []billing.RunCostTotal{}, Sessions: []billing.SessionCostTotal{}}
		for _, t := range s.single(ctx, runs, logKeyRunID) {
			ev.Runs = append(ev.Runs, billing.RunCostTotal{RunID: t.ID, AmountMicros: t.AmountMicros, Currency: t.Currency, IsEstimate: t.IsEstimate})
		}
		for _, t := range s.single(ctx, sessions, logKeySessionID) {
			ev.Sessions = append(ev.Sessions, billing.SessionCostTotal{SessionID: t.ID, AmountMicros: t.AmountMicros, Currency: t.Currency,
				IsEstimate: t.IsEstimate})
		}
		return txn.Publish(ctx, ev)
	})
}

// single returns the totals of the IDs with exactly one currency in ascending ID order; an ID
// with several currencies is logged under the attribute key without amounts and left out.
func (s *Service) single(ctx context.Context, ts []CurrencyTotal, key string) []CurrencyTotal {
	slices.SortStableFunc(ts, func(a, b CurrencyTotal) int { return compareIDs(a.ID, b.ID) })
	var out []CurrencyTotal
	for i := 0; i < len(ts); {
		j := i + 1
		for j < len(ts) && ts[j].ID == ts[i].ID {
			j++
		}
		if j-i == 1 {
			out = append(out, ts[i])
		} else {
			s.d.Logger.LogAttrs(ctx, slog.LevelWarn, msgMixedCurrencies, slog.String(key, ts[i].ID.String()))
		}
		i = j
	}
	return out
}
