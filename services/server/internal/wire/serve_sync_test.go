// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"net/http"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// countRows runs a count query as the superuser of db.
func countRows(t *testing.T, db *pgtest.DB, query string) int {
	t.Helper()
	var n int
	if err := db.SuperConn(t).QueryRow(context.Background(), query).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// tailBackends is the number of LISTEN connections of a log tail on db.
const tailBackends = `SELECT count(*) FROM pg_stat_activity
	WHERE application_name = 'bothy-event-tail' AND datname = current_database()`

// TestServeLogTailInAPIRoles: api and all start the log tail with its own LISTEN connection
// before they are ready (02 "Start and graceful shutdown" step 8), the worker does not; the stop step event_tail closes
// it. worker and all run sync.partition_maintenance at their start (RunOnStart), so a database
// without partitions gets them.
func TestServeLogTailInAPIRoles(t *testing.T) {
	t.Parallel()
	for _, role := range []wire.Role{wire.RoleAPI, wire.RoleWorker, wire.RoleAll} {
		t.Run(string(role), func(t *testing.T) {
			t.Parallel()
			db := newServeDB(t)
			p := start(t, role, serveEnv(t, db), wire.Deps{})
			waitReadyz(t, p.waitInternal(t), http.StatusOK)
			want := 1
			if role == wire.RoleWorker {
				want = 0
			}
			// LISTEN is set up by the tail's own goroutine right after the head is read, so the
			// connection may appear just after readiness.
			deadline := testkit.Deadline()
			for countRows(t, db, tailBackends) != want {
				if time.Now().After(deadline) {
					t.Fatalf("%d tail connections after readiness, want %d", countRows(t, db, tailBackends), want)
				}
				time.Sleep(50 * time.Millisecond)
			}
			if role != wire.RoleAPI {
				deadline := testkit.Deadline()
				for countRows(t, db, "SELECT count(*) FROM pg_inherits WHERE inhparent = 'public.event_log'::regclass") == 0 {
					if time.Now().After(deadline) {
						t.Fatalf("the worker did not create partitions; logs:\n%s", p.logs)
					}
					time.Sleep(100 * time.Millisecond)
				}
			}
			p.sigs <- syscall.SIGTERM
			if code := p.exit(t); code != lifecycle.ExitOK {
				t.Fatalf("exit = %d; logs:\n%s", code, p.logs)
			}
			deadline = testkit.Deadline()
			for countRows(t, db, tailBackends) != 0 {
				if time.Now().After(deadline) {
					t.Fatal("the LISTEN connection is still open after the shutdown")
				}
				time.Sleep(50 * time.Millisecond)
			}
		})
	}
}
