// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
)

// TestJobsWaitBudgetLeavesTheLaterSteps: the wait for cancelled jobs gets only what the fixed
// budgets of the role leave of shutdown.total_timeout, so a handler that ignores its cancelled
// context cannot take the budget of the telemetry flush (02 "Shutdown").
func TestJobsWaitBudgetLeavesTheLaterSteps(t *testing.T) {
	t.Parallel()
	cfg := ServeConfig{
		DrainDelay: 5 * time.Second, HTTPGrace: 10 * time.Second,
		TelemetryFlushTimeout: 5 * time.Second, TotalTimeout: 60 * time.Second,
	}
	soft := 20 * time.Second
	cases := []struct {
		name string
		role Role
		want time.Duration
	}{
		{"worker", RoleWorker, 35 * time.Second},
		{"all", RoleAll, 20 * time.Second},
	}
	for _, c := range cases {
		if got := jobsWaitBudget(cfg, c.role, soft); got != c.want {
			t.Errorf("%s: jobsWaitBudget = %s, want %s", c.name, got, c.want)
		}
	}
	cfg.TotalTimeout = 40 * time.Second
	if got := jobsWaitBudget(cfg, RoleAll, soft); got != 0 {
		t.Errorf("all without slack: jobsWaitBudget = %s, want 0 (no wait step)", got)
	}
}

// TestJobsStopStepsCarryTheBudgets: jobsStopSteps, whose steps register adds as they are,
// returns in registration order jobs_wait with the slack as its budget before jobs with
// shutdown.jobs_soft_timeout (steps of one phase run in reverse order), and no jobs_wait without
// slack, so no step of the phase has budget 0. That register adds them at all is covered by the
// serve tests: TestServeWorkerBudgetIncludesJobsSoftTimeout fails on the soft stop's budget.
func TestJobsStopStepsCarryTheBudgets(t *testing.T) {
	t.Parallel()
	cfg := ServeConfig{TelemetryFlushTimeout: 5 * time.Second, TotalTimeout: 60 * time.Second}
	soft := 20 * time.Second
	steps := (&server{cfg: cfg, role: RoleWorker, jobsCfg: jobs.Config{SoftStopTimeout: soft}}).jobsStopSteps()
	if len(steps) != 2 || steps[0].Name != "jobs_wait" || steps[0].Budget != 35*time.Second ||
		steps[1].Name != "jobs" || steps[1].Budget != soft {
		t.Fatalf("steps = %+v, want jobs_wait 35s then jobs 20s", steps)
	}
	for _, s := range steps {
		if s.Phase != lifecycle.PhaseJobs || s.Run == nil {
			t.Errorf("step %s: phase %v, run set %v", s.Name, s.Phase, s.Run != nil)
		}
	}
	cfg.TotalTimeout = 25 * time.Second
	steps = (&server{cfg: cfg, role: RoleWorker, jobsCfg: jobs.Config{SoftStopTimeout: soft}}).jobsStopSteps()
	if len(steps) != 1 || steps[0].Name != "jobs" {
		t.Fatalf("without slack: steps = %+v, want only jobs", steps)
	}
	if steps := (&server{cfg: cfg, role: RoleAPI}).jobsStopSteps(); len(steps) != 0 {
		t.Fatalf("api: steps = %+v, want none", steps)
	}
}
