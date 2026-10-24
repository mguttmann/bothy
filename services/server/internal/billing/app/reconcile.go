// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/billing"
	"github.com/OWNER/bothy/services/server/internal/billing/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The audit attributes of a reconciliation (10 section 14 row "Cost reconciliation": period,
// number of entries, sums per currency; M2-28 design note 3 h).
const (
	attrPeriodStart     = "period_start"
	attrPeriodEnd       = "period_end"
	attrEntryCount      = "entry_count"
	attrReconciledCount = "reconciled_count"
	attrUnmatchedCount  = "unmatched_count"
	attrTotalPrefix     = "total_micros_"
)

// Reconcile implements billing.Costs (reconcileCosts and submitCostReconciliation; M2-28 design
// note 3) in one unit of work: authorize in.Action on the instance first (authz audits a denial),
// the rules of domain.Validate, the exclusive advisory lock of every run in ascending order, the
// valid entries of the runs in the period in a new statement, the matching per entry in request
// order, one reconciled entry per valid entry in the period of each matched run that is no no-op,
// the job billing.reconcile with these runs, and the audit entry. It only inserts rows.
func (s *Service) Reconcile(ctx context.Context, in billing.ReconcileInput) (billing.ReconcileResult, error) {
	if in.Action != authorize.ActionCostReconcile && in.Action != authorize.ActionCostReconcileSubmit {
		return billing.ReconcileResult{}, errs.New(errs.CodeInternalError, "billing: Reconcile with an action that is no reconciliation")
	}
	req := toRequest(in)
	var out billing.ReconcileResult
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, in.Action, instance); err != nil {
			return err
		}
		if v := domain.Validate(req); len(v) > 0 {
			return violations(v)
		}
		var err error
		out, err = s.reconcile(ctx, in.Action, req)
		return err
	})
	if err != nil {
		return billing.ReconcileResult{}, err
	}
	return out, nil
}

// reconcile is Reconcile after authorize and validation, inside its unit of work.
func (s *Service) reconcile(ctx context.Context, act authorize.Action, req domain.Request) (billing.ReconcileResult, error) {
	runIDs := sortedRuns(req.Entries)
	if err := s.d.Repo.LockReconcile(ctx, runIDs); err != nil {
		return billing.ReconcileResult{}, err
	}
	valid, err := s.d.Repo.ValidEntries(ctx, runIDs, req.PeriodStart, req.PeriodEnd)
	if err != nil {
		return billing.ReconcileResult{}, err
	}
	out := billing.ReconcileResult{UnmatchedRunIDs: []ids.ID{}}
	now := s.d.Clock.Now()
	totals := map[string]int64{}
	var rows []domain.Entry
	var written []ids.ID
	for _, e := range req.Entries {
		matched, noop, _ := domain.Match(valid, req, e)
		if !matched {
			out.UnmatchedRunIDs = append(out.UnmatchedRunIDs, e.RunID)
			continue
		}
		out.Reconciled++
		totals[e.Currency] += e.AmountMicros
		if noop {
			continue
		}
		rows = append(rows, successors(domain.InPeriod(valid, req, e.RunID), e, now)...)
		written = append(written, e.RunID)
	}
	if len(rows) > 0 {
		if err := s.d.Repo.InsertReconciled(ctx, rows); err != nil {
			return billing.ReconcileResult{}, err
		}
		if s.d.Scheduler == nil {
			return billing.ReconcileResult{}, errs.New(errs.CodeInternalError, "billing: no scheduler of billing.reconcile")
		}
		slices.SortFunc(written, compareIDs)
		if err := s.d.Scheduler.Schedule(ctx, written); err != nil {
			return billing.ReconcileResult{}, err
		}
	}
	return out, s.auditReconcile(ctx, act, req, out, totals)
}

// successors returns one reconciled entry per valid entry in the period: the copies of the
// replaced entry, its occurred_at (the day stays the same), the share of the amount, the currency
// of the request entry and supersedes_id on the replaced entry (design note 3 f).
func successors(in []domain.Entry, e domain.RequestEntry, now time.Time) []domain.Entry {
	shares := domain.Split(e.AmountMicros, in)
	out := make([]domain.Entry, len(in))
	for i, old := range in {
		n := old
		n.ID = ids.New()
		n.AmountMicros = shares[i].AmountMicros
		n.Currency = e.Currency
		n.IsEstimate = false
		n.Source = billing.SourceReconciled
		n.ReconciledAt = now
		n.SupersedesID = old.ID
		out[i] = n
	}
	return out
}

// auditReconcile records the allow entry: action, the instance without ID, the period, the counts
// and one sum per currency of the matched entries; nothing else (no provider, no run IDs).
func (s *Service) auditReconcile(ctx context.Context, act authorize.Action, req domain.Request, res billing.ReconcileResult,
	totals map[string]int64) error {
	attrs := auditing.Attrs{
		attrPeriodStart: auditing.Time(req.PeriodStart), attrPeriodEnd: auditing.Time(req.PeriodEnd),
		attrEntryCount: auditing.Int(int64(len(req.Entries))), attrReconciledCount: auditing.Int(int64(res.Reconciled)),
		attrUnmatchedCount: auditing.Int(int64(len(res.UnmatchedRunIDs))),
	}
	for cur, sum := range totals {
		attrs[attrTotalPrefix+strings.ToLower(cur)] = auditing.Int(sum)
	}
	return s.d.Audit.Record(ctx, auditing.Entry{Action: string(act), Decision: auditing.DecisionAllow,
		ResourceKind: string(authorize.ResourceInstance), Attrs: attrs})
}

// toRequest maps the port input to the domain.
func toRequest(in billing.ReconcileInput) domain.Request {
	req := domain.Request{Provider: in.Provider, PeriodStart: in.PeriodStart, PeriodEnd: in.PeriodEnd,
		Entries: make([]domain.RequestEntry, len(in.Entries))}
	for i, e := range in.Entries {
		req.Entries[i] = domain.RequestEntry{RunID: e.RunID, AmountMicros: e.AmountMicros, Currency: e.Currency}
	}
	return req
}

// sortedRuns returns the distinct run IDs of es in ascending order.
func sortedRuns(es []domain.RequestEntry) []ids.ID {
	out := make([]ids.ID, 0, len(es))
	for _, e := range es {
		out = append(out, e.RunID)
	}
	slices.SortFunc(out, compareIDs)
	return slices.Compact(out)
}

// compareIDs orders IDs by their bytes, which is the order of PostgreSQL's uuid.
func compareIDs(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) }

// violations converts the violations to one validation_failed.
func violations(v []domain.Violation) error {
	fields := make([]errs.FieldError, len(v))
	for i, x := range v {
		fields[i] = errs.FieldError{Pointer: x.Pointer, Code: x.Code}
	}
	return errs.New(errs.CodeValidationFailed, "the reconciliation breaks a rule").WithFields(fields...)
}
