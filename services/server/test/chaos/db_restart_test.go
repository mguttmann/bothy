// SPDX-License-Identifier: Apache-2.0

//go:build perf

package chaos_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/test/chaos"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// The parameters of TestChaos_DatabaseRestartDuringTail (README.md).
const (
	// beforeRestart is the load before the restart; after it the load runs for the delivery
	// bound (log poll plus reconnect backoff plus listenSlack) plus afterFault.
	beforeRestart = 3 * time.Second
	// restartSettle bounds the wait for the receivers after the writers stopped.
	restartSettle = 2 * time.Minute
)

// TestChaos_DatabaseRestartDuringTail restarts the PostgreSQL container (same host port) while
// writers and the log tail run. Writers repeat failed sends with the same client_msg_id until they
// succeed; every message acknowledged after the restart reaches every receiver within the poll and
// backoff bounds, and the invariants hold.
func TestChaos_DatabaseRestartDuringTail(t *testing.T) {
	p := harness.StartProcess(t, harness.ProcessOptions{Config: harness.RaisedRateLimits()})
	w := chaos.NewWorld(t, p)
	w.StartWriters()
	time.Sleep(beforeRestart)
	if err := p.DB.Restart(t.Context()); err != nil {
		t.Fatal(err)
	}
	restarted := time.Now()
	bound := defaultDuration(t, "sync.log_poll_interval") + defaultDuration(t, "database.listen_reconnect_backoff_cap") + listenSlack
	time.Sleep(bound + afterFault)
	w.StopWriters(t, restartSettle)
	t.Log(w.Report())
	// The tail resumes after the restart: messages acknowledged afterwards arrive within the bound.
	w.CheckDeliveredWithin(t, restarted, bound)
	chaos.CheckInvariants(t, w)
}
