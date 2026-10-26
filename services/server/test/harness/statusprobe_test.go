// SPDX-License-Identifier: Apache-2.0

package harness

import (
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	statusjobs "github.com/OWNER/bothy/services/server/internal/status/adapters/jobs"
)

// waitProbeRound waits until River completed at least one job status.probe (the round at the
// start of the worker).
func waitProbeRound(t *testing.T, s *Server) {
	t.Helper()
	schema := pgx.Identifier{s.ConfigValue("jobs.schema"), "river_job"}.Sanitize()
	q := `SELECT count(*) FROM ` + schema + ` WHERE kind = $1 AND state = 'completed'`
	testkit.Until(t, "a completed round of status.probe", func() bool { return count(t, s, q, statusjobs.KindProbe) > 0 })
}

// TestStatusProbe_OffByDefault (PL/46): River runs a no-op in place of status.probe, so no round
// writes status_components or appends status.updated behind a test that reads the event log,
// also with an active agent, which a real round turns into a component.
func TestStatusProbe_OffByDefault(t *testing.T) {
	t.Parallel()
	s := Start(t, Options{Config: RaisedRateLimits()})
	s.NewAgent(t, AgentSpec{Handle: "lotse", DisplayName: "Lotse"})
	MustDo(t, s.Owner, Request{Method: http.MethodPost, Path: "/spaces", Body: map[string]any{"name": "Support"}}, http.StatusCreated)
	waitProbeRound(t, s)
	if n := count(t, s, `SELECT count(*) FROM status_components`); n != 0 {
		t.Fatalf("%d status components written by a round", n)
	}
	if n := count(t, s, `SELECT count(*) FROM event_log WHERE type = 'status.updated'`); n != 0 {
		t.Fatalf("%d status.updated events appended by a round", n)
	}
}

// TestStatusProbe_OptIn: with Options.StatusProbe River runs the real use case; its round at the
// start writes the standard components.
func TestStatusProbe_OptIn(t *testing.T) {
	t.Parallel()
	s := Start(t, Options{Config: RaisedRateLimits(), StatusProbe: true})
	testkit.Until(t, "the standard components", func() bool {
		return count(t, s, `SELECT count(*) FROM status_components WHERE agent_id IS NULL`) >= 4
	})
}
