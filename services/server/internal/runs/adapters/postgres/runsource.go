// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres/sqlcgen"
)

// RunSource implements the declared source agents.RunSource over runs (02 section 4.4; M2-12
// design note 2): agents may not use runs, so a report of agent_session.report finds the active
// run of its thread through it, and internal/wire wires it into the agents use cases. It is safe
// for concurrent use.
type RunSource struct {
	pool *pg.Pool
}

// Compile-time check: RunSource is the source agents declares.
var _ agents.RunSource = (*RunSource)(nil)

// NewRunSource returns the source over pool, a pool of bothy_app; ActiveRun works only in the
// ambient transaction of its caller, because it locks the row.
//
// Example:
//
//	src := postgres.NewRunSource(pool)
//	run, found, err := src.ActiveRun(ctx, threadID) // inside a unit of work
func NewRunSource(pool *pg.Pool) *RunSource { return &RunSource{pool: pool} }

// ActiveRun implements agents.RunSource: the run of threadID in queued, running or waiting_input
// (R-DM-14) with its row locked FOR UPDATE for the rest of the unit of work, so the report locks
// the run before its session row (03 6.3). found is false for none. Errors: internal outside a
// unit of work and for a database failure.
func (s *RunSource) ActiveRun(ctx context.Context, threadID ids.ID) (agents.ActiveRun, bool, error) {
	if !txn.InUnit(ctx) {
		// A lock outside a unit of work would end with its statement.
		return agents.ActiveRun{}, false, errs.New(errs.CodeInternalError, "runs: ActiveRun runs only inside a unit of work")
	}
	locked, err := sqlcgen.New(pg.Querier(ctx, s.pool)).LockActiveRunOfThread(ctx, uuid(threadID))
	row, found, err := one(locked, err, "locking the active run of a thread")
	if err != nil || !found {
		return agents.ActiveRun{}, false, err
	}
	return agents.ActiveRun{RunID: row.ID, AgentID: row.AgentID, SessionID: row.AgentSessionID, State: string(row.State)}, true, nil
}
