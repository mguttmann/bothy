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
	if sum == row.CumulativeCostMicros && currency == row.Currency {
		// Another currency (no conversion, 10 12.1) or a zero amount: the row did not change, so
		// its version stays (03 principle 10).
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
