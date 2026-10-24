// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// maxCostMicros is the integer bound of 06 "Basic rules" (2^53): every stored sum stays below it,
// so a client reads it exactly as a JSON number.
const maxCostMicros = int64(1) << 53

// AddCost implements agents.Sessions (M2-22 design note 5 g) in the caller's unit of work. It
// never authorizes and writes no event and no audit entry: runs authorized run.finish, and a
// booking is no cause of agent_session.changed (05 section 6).
func (s *SessionService) AddCost(ctx context.Context, cmd agents.AddSessionCost) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "agents: AddCost outside a unit of work")
	}
	// billing validated the reports before the unit of work; a value outside them is a
	// programming error of the caller, never a client error.
	if cmd.AmountMicros < 0 || cmd.AmountMicros >= maxCostMicros || cmd.Currency == "" {
		return errs.New(errs.CodeInternalError, "agents: AddCost with an invalid amount or currency")
	}
	rows, err := s.d.Store.LockSessions(ctx, []ids.ID{cmd.SessionID})
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		// cost_entries.agent_session_id has its foreign key, so the row exists.
		return errs.New(errs.CodeInternalError, "agents: AddCost for an unknown session")
	}
	row := rows[0]
	sum, currency := addedCost(row, cmd)
	if sum >= maxCostMicros {
		return errs.New(errs.CodeInternalError, "agents: the session sum reached the integer bound")
	}
	if sum == row.CumulativeCostMicros && currency == row.Currency && row.CostIsEstimate {
		// Another currency (no conversion, 10 12.1) or a zero amount on an estimate: the row did not
		// change, so its version stays (03 principle 10). A row a reconciliation marked final becomes
		// an estimate again with the booking (M2-28), which SetSessionCost writes.
		return nil
	}
	_, changed, err := s.d.Store.SetSessionCost(ctx, SessionCost{ID: row.ID, Version: row.Version, CumulativeCostMicros: sum, Currency: currency})
	if err != nil {
		return err
	}
	if !changed {
		return errs.New(errs.CodeInternalError, "agents: a locked session row changed")
	}
	return nil
}

// addedCost returns the session sum and currency of row after cmd: a row without any cost yet
// (cumulative and baseline 0) still carries the column default and takes the currency of its
// first booking; an amount is added only in the row's currency.
func addedCost(row agents.Session, cmd agents.AddSessionCost) (sum int64, currency string) {
	sum, currency = row.CumulativeCostMicros, row.Currency
	if sum == 0 && row.CostBaselineMicros == 0 {
		currency = cmd.Currency
	}
	if currency == cmd.Currency {
		sum += cmd.AmountMicros
	}
	return sum, currency
}

// ApplyCostTotals implements agents.Sessions (M2-28 design note 7) in the caller's unit of work.
// It never authorizes and writes no event and no audit entry.
func (s *SessionService) ApplyCostTotals(ctx context.Context, totals []agents.SessionCostTotal) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "agents: ApplyCostTotals outside a unit of work")
	}
	byID, sessionIDs, err := indexTotals(totals)
	if err != nil {
		return err
	}
	if len(sessionIDs) == 0 {
		return nil
	}
	// LockSessions locks in ascending id (03 6.3) and returns the rows by id.
	rows, err := s.d.Store.LockSessions(ctx, sessionIDs)
	if err != nil {
		return err
	}
	for _, row := range rows {
		t := byID[row.ID]
		if t.Currency != row.Currency || (t.AmountMicros == row.CumulativeCostMicros && t.IsEstimate == row.CostIsEstimate) {
			// Another currency is never converted (10 12.1); an equal row keeps its version (03
			// principle 10).
			continue
		}
		_, changed, err := s.d.Store.SetSessionTotal(ctx, SessionTotal{ID: row.ID, Version: row.Version,
			CumulativeCostMicros: t.AmountMicros, CostIsEstimate: t.IsEstimate})
		if err != nil {
			return err
		}
		if !changed {
			return errs.New(errs.CodeInternalError, "agents: a locked session row changed")
		}
	}
	return nil
}

// indexTotals returns the totals by session and the distinct sessions in input order. Errors:
// internal_error for a negative amount or one at or above 2^53 (billing summed valid entries, so
// such a value is a programming error).
func indexTotals(totals []agents.SessionCostTotal) (map[ids.ID]agents.SessionCostTotal, []ids.ID, error) {
	byID := make(map[ids.ID]agents.SessionCostTotal, len(totals))
	sessionIDs := make([]ids.ID, 0, len(totals))
	for _, t := range totals {
		if t.AmountMicros < 0 || t.AmountMicros >= maxCostMicros {
			return nil, nil, errs.New(errs.CodeInternalError, "agents: ApplyCostTotals with an invalid amount")
		}
		if _, ok := byID[t.SessionID]; !ok {
			sessionIDs = append(sessionIDs, t.SessionID)
		}
		byID[t.SessionID] = t
	}
	return byID, sessionIDs, nil
}
