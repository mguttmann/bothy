// SPDX-License-Identifier: Apache-2.0

//go:build perf

package chaos_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/test/chaos"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// The parameters of TestChaos_ListenConnectionLoss (README.md).
const (
	// listenKills is how often the LISTEN backend of the tail is terminated, killEvery apart.
	listenKills = 3
	killEvery   = 3 * time.Second
	// listenSlack is added to the bound of log poll and reconnect backoff for scheduling.
	listenSlack = 2 * time.Second
	// afterFault is how long the load runs after the last fault beyond the delivery bound, so a
	// tail that never resumes leaves messages that miss the bound.
	afterFault = 5 * time.Second
)

// TestChaos_ListenConnectionLoss terminates the LISTEN backend of the log tail (application_name
// bothy-event-tail) with pg_terminate_backend under load. Receivers still get every event in
// order, every acknowledged message reaches them within sync.log_poll_interval plus
// database.listen_reconnect_backoff_cap, and the invariants hold.
func TestChaos_ListenConnectionLoss(t *testing.T) {
	p := harness.StartProcess(t, harness.ProcessOptions{Config: harness.RaisedRateLimits()})
	w := chaos.NewWorld(t, p)
	w.StartWriters()
	time.Sleep(killEvery)
	var lastKill time.Time
	for range listenKills {
		var killed int
		if err := p.AdminDB().QueryRow(t.Context(), `SELECT count(*) FROM (SELECT pg_terminate_backend(pid) FROM pg_stat_activity
			WHERE application_name = 'bothy-event-tail' AND datname = current_database()) k`).Scan(&killed); err != nil {
			t.Fatal(err)
		}
		if killed == 0 {
			t.Fatal("no LISTEN backend of the tail (application_name bothy-event-tail) to terminate")
		}
		lastKill = time.Now()
		time.Sleep(killEvery)
	}
	bound := defaultDuration(t, "sync.log_poll_interval") + defaultDuration(t, "database.listen_reconnect_backoff_cap") + listenSlack
	time.Sleep(bound + afterFault - killEvery)
	w.StopWriters(t, bound+time.Minute)
	t.Log(w.Report())
	// Losing LISTEN must not touch the gateway connections: every event came live.
	for i, n := range w.Reconnects() {
		if n != 0 {
			t.Errorf("receiver %d reconnected %d times; the LISTEN loss must not end gateway connections", i, n)
		}
	}
	w.CheckDeliveredWithin(t, lastKill, bound)
	chaos.CheckInvariants(t, w)
}

// defaultDuration returns the default of a duration key of the server's configuration schema.
func defaultDuration(t *testing.T, key string) time.Duration {
	t.Helper()
	d, err := chaos.DefaultDuration(key)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
