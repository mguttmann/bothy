// SPDX-License-Identifier: Apache-2.0

//go:build perf

package chaos_test

import (
	"math/rand/v2"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/test/chaos"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// The parameters of TestChaos_SigkillDuringCommit (README.md).
const (
	// kills is how often the server process is killed; each kill comes after a random load time
	// between killAfterMin and killAfterMax.
	kills        = 3
	killAfterMin = time.Second
	killAfterMax = 3 * time.Second
	// killSettle bounds the wait for the receivers after the writers stopped.
	killSettle = 2 * time.Minute
)

// TestChaos_SigkillDuringCommit kills the server process with SIGKILL at random moments under
// write load and starts it again; writers repeat unacknowledged sends with the same client_msg_id.
// Afterwards the invariants hold: no gap, no partial change, nothing lost or doubled.
func TestChaos_SigkillDuringCommit(t *testing.T) {
	p := harness.StartProcess(t, harness.ProcessOptions{Config: harness.RaisedRateLimits()})
	w := chaos.NewWorld(t, p)
	w.StartWriters()
	for range kills {
		time.Sleep(killAfterMin + rand.N(killAfterMax-killAfterMin)) // #nosec G404 -- a random moment, not a secret
		if err := p.Signal(syscall.SIGKILL); err != nil {
			t.Fatal(err)
		}
		<-p.Exited()
		if err := p.Restart(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(killAfterMax)
	w.StopWriters(t, killSettle)
	t.Log(w.Report())
	chaos.CheckInvariants(t, w)
}
