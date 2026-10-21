// SPDX-License-Identifier: Apache-2.0

// Package agentstest holds test helpers of the agents module. Only _test.go files import it;
// TestAgentstest_NotInBinary fails when the server binary depends on it.
//
// PersistHeartbeat writes the persisted heartbeat of an agent directly, so integration tests of
// runs.stall_check (M2-09) can place a heartbeat at any time of a fake clock without a running
// presence tracker and its persist interval.
//
// Example:
//
//	agentstest.PersistHeartbeat(t, pool, lotse, clk.Now(), []ids.ID{runID})
package agentstest

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Execer runs one statement: *pg.Pool, pgx.Tx and *pgx.Conn satisfy it.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// PersistHeartbeat stores a heartbeat of agentID at time at in its agents row, as the presence
// tracker's flush would: reported_runtime_state online, last_heartbeat_at at and
// reported_details with active_runs in the stored shape of the agents domain. activeRuns nil
// stores a heartbeat without active_runs (not reported); an empty non-nil slice stores a
// reported empty list. The row is overwritten as given, also with an older time (the tracker's
// flush keeps the newest), so a test can move the heartbeat back. Other reported fields are
// cleared. It fails tb when the statement fails or the agent has no row.
//
// Example:
//
//	agentstest.PersistHeartbeat(t, pool, lotse, started.Add(presenceTimeout+time.Second), []ids.ID{})
func PersistHeartbeat(tb testing.TB, db Execer, agentID ids.ID, at time.Time, activeRuns []ids.ID) {
	tb.Helper()
	details := domain.Report{Details: domain.Details{ActiveRuns: activeRuns}}.StoredDetails()
	tag, err := db.Exec(context.Background(),
		`UPDATE agents SET reported_runtime_state = 'online', reported_details = $1, last_heartbeat_at = $2 WHERE principal_id = $3`,
		details, at, agentID.String())
	if err != nil {
		tb.Fatalf("agentstest: storing a heartbeat: %v", err)
		return
	}
	if tag.RowsAffected() != 1 {
		tb.Fatalf("agentstest: no agents row for the heartbeat")
	}
}
