// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// eventLogPartitions counts the partitions of event_log and returns the highest upper bound.
func eventLogPartitions(t *testing.T, db *pgtest.DB) (int, int64) {
	t.Helper()
	var n int
	var upper int64
	err := db.SuperConn(t).QueryRow(context.Background(), `SELECT count(*), COALESCE(max(
		(regexp_match(pg_get_expr(c.relpartbound, c.oid), 'TO \(''?([0-9]+)''?\)'))[1]::bigint), 0)
		FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid WHERE i.inhparent = 'public.event_log'::regclass`).Scan(&n, &upper)
	if err != nil {
		t.Fatal(err)
	}
	return n, upper
}

// TestMigrateUp_CreatesInitialPartitions: bothy migrate up creates the partitions of event_log
// with the configured size and number ahead, so serve finds partitions on a fresh database
// (upgrade.md rule M10), and a second run creates none.
func TestMigrateUp_CreatesInitialPartitions(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t, pgtest.WithoutMigrations())
	env := migrateEnvFor(db, pg.RoleOwner,
		"BOTHY_SYNC__EVENT_LOG_PARTITION_SIZE=100", "BOTHY_SYNC__EVENT_LOG_PARTITIONS_AHEAD=4")
	if code, _, stderr := runBothy(t, env, "migrate", "up"); code != 0 {
		t.Fatalf("migrate up: exit %d, stderr %q", code, stderr)
	}
	n, upper := eventLogPartitions(t, db)
	if n != 4 || upper != 401 {
		t.Fatalf("%d partitions up to %d, want 4 partitions of 100 up to 401", n, upper)
	}
	if code, _, stderr := runBothy(t, env, "migrate", "up"); code != 0 {
		t.Fatalf("second migrate up: exit %d, stderr %q", code, stderr)
	}
	if again, _ := eventLogPartitions(t, db); again != n {
		t.Fatalf("second run left %d partitions, want %d", again, n)
	}
}
