// SPDX-License-Identifier: Apache-2.0

//go:build perf

package chaos_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/test/terminal/suite"
)

// chaosSessions is the number of sessions TestChaosTMT16 drives at once (a test constant).
const chaosSessions = 6

// TestChaosTMT16 is TM-T16 under load in make perf (brief M2-48 Design note 10, PLAN 5 row "Load,
// endurance, chaos", P-07): the scenario file tm-t16.yaml of the package restart runs on its own
// stack with several parallel sessions that print continuously; every session survives the
// restart of the server, the cut of its device and the loss of the host network with its
// scrollback, and all end host_restarted at bothy-host restart with no process left. The
// database is the PostgreSQL of the stack with the image defaults (fsync and synchronous_commit
// on, ADR 0063 point 2). It runs without -race, as the perf recipe of M1-72 runs it.
func TestChaosTMT16(t *testing.T) {
	env := suite.StartEnv(t, "chaos", "default")
	sc, err := suite.LoadScenario(env.Repo.ScenarioPath("tm-t16"))
	if err != nil {
		t.Fatal(err)
	}
	suite.RunScenario(t, env, suite.Multiply(sc, chaosSessions))
}
