// SPDX-License-Identifier: Apache-2.0

package agentpath_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/perf/internal/agentpath"
)

// defaults returns the inputs of the measured configuration or fails t.
func defaults(t *testing.T) agentpath.Inputs {
	t.Helper()
	in, err := agentpath.Defaults()
	if err != nil {
		t.Fatal(err)
	}
	return in
}

// plan returns the plan of in or fails t.
func plan(t *testing.T, in agentpath.Inputs) agentpath.Plan {
	t.Helper()
	p, err := agentpath.NewPlan(in)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestPlan_DefaultsWithinRateLimits: with the default configuration every planned principal stays
// at or below BudgetShare of its class; A is minimal, R is maximal; viewers equal A * R *
// ViewersPerRun.
func TestPlan_DefaultsWithinRateLimits(t *testing.T) {
	t.Parallel()
	in := defaults(t)
	p := plan(t, in)
	t.Logf("A %d, R %d, channels %d, threads %d, trigger persons %d, senders %d, viewers %d", p.Agents, p.RunsPerAgent, p.Channels, p.Threads,
		p.TriggerPersons, p.Senders, p.Viewers)
	share := func(perMinute int) float64 { return agentpath.BudgetShare * float64(perMinute) / 60 }
	heartbeat := 1 / in.HeartbeatInterval.Seconds()
	if got := p.AgentCallsPerSecond(agentpath.ModeInvocation); got > share(in.AgentPerMinute) {
		t.Errorf("agent in perf/invocation plans %.3f calls/s, above the share %.3f", got, share(in.AgentPerMinute))
	}
	if got := p.AgentCallsPerSecond(agentpath.ModePatch); got > share(in.AgentPerMinute) {
		t.Errorf("agent in perf/run_patch plans %.3f calls/s, above the share %.3f", got, share(in.AgentPerMinute))
	}
	if p.Agents > 1 {
		if fewer := p.TriggerRate/float64(p.Agents-1)*agentpath.CallsPerRun + heartbeat; fewer <= share(in.AgentPerMinute) {
			t.Errorf("A = %d is not minimal: %d agents plan %.3f calls/s", p.Agents, p.Agents-1, fewer)
		}
	}
	if more := float64(p.RunsPerAgent+1)/in.RunUpdateMinInterval.Seconds() + heartbeat; more <= share(in.AgentPerMinute) {
		t.Errorf("R = %d is not maximal: %d runs plan %.3f calls/s", p.RunsPerAgent, p.RunsPerAgent+1, more)
	}
	if got := p.TriggerPersonRate(); got > share(in.MessagePerMinute) || got > share(in.TriggerPerMinute) {
		t.Errorf("trigger person sends %.3f/s, above the share of rl.message %.3f or rl.trigger %.3f", got, share(in.MessagePerMinute), share(in.TriggerPerMinute))
	}
	for _, m := range []agentpath.Mode{agentpath.ModeInvocation, agentpath.ModePatch} {
		if got := p.SenderRate(m); got > share(in.MessagePerMinute) {
			t.Errorf("sender in %s sends %.3f/s, above the share %.3f", m, got, share(in.MessagePerMinute))
		}
	}
	if p.Viewers != p.Agents*p.RunsPerAgent*agentpath.ViewersPerRun {
		t.Errorf("viewers = %d, want A * R * ViewersPerRun = %d", p.Viewers, p.Agents*p.RunsPerAgent*agentpath.ViewersPerRun)
	}
	if p.Agents < 1 || p.RunsPerAgent < 1 || p.TriggerPersons < 1 || p.Senders < 1 {
		t.Errorf("plan without a principal: %+v", p)
	}
	if p.BackgroundRate(agentpath.ModeInvocation)+p.TriggerRate != p.Rate || p.BackgroundRate(agentpath.ModePatch) != p.Rate {
		t.Errorf("background rates %v and %v do not add up to %v", p.BackgroundRate(agentpath.ModeInvocation), p.BackgroundRate(agentpath.ModePatch), p.Rate)
	}
}

// TestPlan_FollowsKeys: halving ratelimit.rl.agent_runtime.per_minute gives at least as many
// agents and at most as many runs per agent; doubling perf.sustained_messages_per_s doubles the
// trigger rate; an agent.run_update_min_interval short enough that R < 1 gives an error naming the
// keys.
func TestPlan_FollowsKeys(t *testing.T) {
	t.Parallel()
	in := defaults(t)
	base := plan(t, in)
	half := in
	half.AgentPerMinute /= 2
	p := plan(t, half)
	if p.Agents < base.Agents || p.RunsPerAgent > base.RunsPerAgent {
		t.Errorf("halved rl.agent_runtime: A %d -> %d, R %d -> %d", base.Agents, p.Agents, base.RunsPerAgent, p.RunsPerAgent)
	}
	double := in
	double.Rate *= 2
	if p := plan(t, double); p.TriggerRate != 2*base.TriggerRate {
		t.Errorf("doubled rate: trigger rate %v, want %v", p.TriggerRate, 2*base.TriggerRate)
	}
	short := in
	short.RunUpdateMinInterval = time.Duration(float64(time.Second) / (agentpath.BudgetShare * float64(in.AgentPerMinute) / 60))
	_, err := agentpath.NewPlan(short)
	if err == nil || !strings.Contains(err.Error(), "agent.run_update_min_interval") || !strings.Contains(err.Error(), "ratelimit.rl.agent_runtime.per_minute") {
		t.Errorf("R < 1: %v, want an error naming the keys", err)
	}
	none := in
	none.Rate = 0
	if _, err := agentpath.NewPlan(none); err == nil {
		t.Error("a zero rate was accepted")
	}
}

// TestMinSamples: fewer than MinSamples samples make the result fail, as does a Window too short
// for MinSamples in the plan.
func TestMinSamples(t *testing.T) {
	t.Parallel()
	if err := agentpath.CheckMinSamples("perf.x", agentpath.MinSamples-1, agentpath.MinSamples); err == nil || !strings.Contains(err.Error(), "perf.x") {
		t.Errorf("one sample short: %v, want an error naming the target", err)
	}
	if err := agentpath.CheckMinSamples("perf.x", agentpath.MinSamples, agentpath.MinSamples); err != nil {
		t.Errorf("exactly MinSamples: %v", err)
	}
	in := defaults(t)
	in.Window = time.Second
	if _, err := agentpath.NewPlan(in); err == nil || !strings.Contains(err.Error(), "MinSamples") {
		t.Errorf("a window too short for MinSamples: %v", err)
	}
}

// TestChapterDefault: the default comes from the column "Default" of the section's table; a row
// outside the section and a missing row are errors.
func TestChapterDefault(t *testing.T) {
	t.Parallel()
	got, err := agentpath.ChapterDefault(agentpath.SyncChapter(), "agent.run_update_min_interval")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := time.ParseDuration(got); err != nil {
		t.Errorf("agent.run_update_min_interval = %q: %v", got, err)
	}
	p := filepath.Join(t.TempDir(), "c.md")
	body := "# C\n\n| `a.out` | `1s` | x |\n\n## 9. Configuration values of this chapter\n\n| Name | Place | Default | Meaning |\n|---|---|---|---|\n" +
		"| `a.in` | Server | `2s` | x |\n\n## Sources\n\n| Name | Place | Default | Meaning |\n|---|---|---|---|\n| `a.after` | Server | `3s` | x |\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if v, err := agentpath.ChapterDefault(p, "a.in"); err != nil || v != "2s" {
		t.Errorf("a.in = %q, %v", v, err)
	}
	for _, name := range []string{"a.out", "a.after", "a.missing"} {
		if _, err := agentpath.ChapterDefault(p, name); err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("%s: %v, want an error naming it", name, err)
		}
	}
}
