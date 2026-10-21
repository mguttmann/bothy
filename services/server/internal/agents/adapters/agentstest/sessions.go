// SPDX-License-Identifier: Apache-2.0

package agentstest

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ImportedSession is one earlier session reference of an import (22 section 9 step 1): the
// thread, its operator, the reference and the cost baseline. Currency "" means the column
// default of agent_sessions.currency.
type ImportedSession struct {
	ThreadID, AgentID  ids.ID
	ExternalSessionRef string
	CostBaselineMicros int64
	Currency           string
}

// InsertImportedSession writes the agent_sessions row the import creates for s, as 22 section 9
// step 1 states it: is_current false, started true, end_reason import, created_at now and
// ended_at equal to it, the baseline and the currency of s. Integration tests of the import
// binding (M2-12 design note 9) place such rows without running an import. It returns the new
// row ID and fails tb when the statement fails, for example for an unknown thread or agent.
//
// Example:
//
//	old := agentstest.InsertImportedSession(t, pool, agentstest.ImportedSession{ThreadID: thread, AgentID: lotse, ExternalSessionRef: "sess-1"})
func InsertImportedSession(tb testing.TB, db Execer, s ImportedSession) ids.ID {
	tb.Helper()
	id := ids.New()
	// now() is the start of the statement's transaction, so ended_at equals created_at.
	sql := `INSERT INTO agent_sessions (id, thread_id, agent_id, external_session_ref, is_current, started,
		cost_baseline_micros, created_at, ended_at, end_reason) VALUES ($1, $2, $3, $4, false, true, $5, now(), now(), 'import')`
	args := []any{id.String(), s.ThreadID.String(), s.AgentID.String(), s.ExternalSessionRef, s.CostBaselineMicros}
	if s.Currency != "" {
		sql = `INSERT INTO agent_sessions (id, thread_id, agent_id, external_session_ref, is_current, started,
			cost_baseline_micros, created_at, ended_at, end_reason, currency) VALUES ($1, $2, $3, $4, false, true, $5, now(), now(), 'import', $6)`
		args = append(args, s.Currency)
	}
	if _, err := db.Exec(context.Background(), sql, args...); err != nil {
		tb.Fatalf("agentstest: storing an imported session: %v", err)
	}
	return id
}
