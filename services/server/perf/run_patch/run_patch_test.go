// SPDX-License-Identifier: Apache-2.0

//go:build perf

package runpatch_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/perf/internal/agentpath"
	"github.com/OWNER/bothy/services/server/perf/internal/agentworld"
	"github.com/OWNER/bothy/services/server/perf/internal/fakeadapter"
	"github.com/OWNER/bothy/services/server/perf/internal/perfkit"
	"github.com/OWNER/bothy/services/server/perf/internal/report"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// patchRun is one run of the test: its adapter, the run and the viewers that focus its thread.
type patchRun struct {
	adapter int
	run     fakeadapter.RunStart
	viewers []int
}

// TestPerf_RunPatchFanoutP99 measures perf.run_patch_fanout_p99 (brief M2-36 design notes 7 and
// 8): A * R long runs, each patched by its fake adapter through run.update every
// agent.run_update_min_interval, each focused by ViewersPerRun viewers, under human background
// messages at perf.sustained_messages_per_s, against the measured server in the default
// configuration on a durable database. A sample is, per version and viewer, the time from the
// scheduled send of the patch that produced the version to the first run.updated that lifts the
// viewer's version to it or above (both on the test process clock). The value counts only when no
// viewer saw a gap or a late snapshot, every viewer ended at the version getRun reports, every
// version of the window reached every viewer, no error or close frame came and every run ended
// completed.
func TestPerf_RunPatchFanoutP99(t *testing.T) {
	env := perfkit.Setup(t)
	th := env.Threshold(t, "perf.run_patch_fanout_p99")
	in, err := agentpath.Defaults()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := agentpath.NewPlan(in)
	if err != nil {
		t.Fatal(err)
	}
	p := env.StartSeeding(t, nil)
	seedStart := time.Now()
	w, err := agentworld.Seed(t, p.Server, plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("seeded %d agents, %d channels, %d threads, %d trigger persons, %d senders, %d viewers in %s", plan.Agents, plan.Channels,
		plan.Threads, plan.TriggerPersons, plan.Senders, plan.Viewers, time.Since(seedStart).Round(time.Second))

	env.Measure(t, p, nil)
	ctx := t.Context()
	adapters, err := w.StartAdapters(t, p.Server, plan, agentpath.ModePatch)
	if err != nil {
		t.Fatal(err)
	}
	setup := setupTriggers(ctx, w, plan)
	var problems []string
	problems = append(problems, agentworld.SendProblems(setup)...)
	if len(problems) > 0 {
		t.Fatalf("setup triggers: %v", problems)
	}
	// Every run must be running before a viewer focuses it: the version step of startRun after a
	// focus would reach the viewer as a snapshot (M2-10 note 9).
	waitCtx, cancel := context.WithTimeout(ctx, testkit.Patience)
	defer cancel()
	for i, a := range adapters {
		if err := a.Wait(waitCtx, func(r fakeadapter.Report) bool { return r.Running == plan.RunsPerAgent || r.Err() != nil }); err != nil {
			t.Fatalf("adapter of %s: runs not running: %v (%+v)", w.Agents[i].Handle, err, a.Report().Errors)
		}
		if err := a.Report().Err(); err != nil {
			t.Fatalf("adapter of %s: %v", w.Agents[i].Handle, err)
		}
	}
	var runs []patchRun
	for i, a := range adapters {
		for _, r := range a.Running() {
			runs = append(runs, patchRun{adapter: i, run: r})
		}
	}
	viewers, err := w.ConnectViewers(t)
	if err != nil {
		t.Fatal(err)
	}
	for v := range viewers {
		r := &runs[v/agentpath.ViewersPerRun]
		r.viewers = append(r.viewers, v)
		if err := viewers[v].Focus(r.run.Thread); err != nil {
			t.Fatal(err)
		}
	}
	for v, vw := range viewers {
		run := runs[v/agentpath.ViewersPerRun].run.ID
		if err := vw.WaitFor(waitCtx, func(fs []agentworld.RunFrame) bool { return hasSnapshot(fs, run) }); err != nil {
			t.Fatalf("viewer %d: no snapshot of run %s after focus: %v (%v)", v+1, run, err, vw.Errors())
		}
	}

	// The patches and the background load start together; the window is the same for both.
	patchStart := time.Now().Add(time.Second)
	stop := patchStart.Add(plan.Warmup + plan.Window)
	var writers []<-chan struct{}
	for _, a := range adapters {
		done, err := a.StartPatches(ctx, patchStart, stop)
		if err != nil {
			t.Fatal(err)
		}
		writers = append(writers, done)
	}
	sends := make([]agentworld.Sent, plan.Total())
	bgStart, n := perfkit.ScheduleAt(ctx, patchStart, plan.Rate, plan.Warmup, plan.Window, func(i int, due time.Time) {
		sends[i] = w.Background(ctx, i, due)
	})
	sends = sends[:n]
	sentAt := map[string]map[int64]time.Time{}
	last := map[string]int64{}
	collectSent := func() {
		for _, a := range adapters {
			for _, s := range a.Report().Sent {
				if sentAt[s.RunID] == nil {
					sentAt[s.RunID] = map[int64]time.Time{}
				}
				sentAt[s.RunID][s.Version] = s.Scheduled
				last[s.RunID] = max(last[s.RunID], s.Version)
			}
		}
	}
	// The writers end at stop (a lagging one later, with every patch it sent recorded); then the
	// viewers get up to Drain to reach the last versions.
	for _, done := range writers {
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	collectSent()
	drainCtx, drainCancel := context.WithTimeout(ctx, plan.Drain)
	defer drainCancel()
	for _, r := range runs {
		for _, v := range r.viewers {
			_ = viewers[v].WaitFor(drainCtx, func(fs []agentworld.RunFrame) bool { return reached(fs, r.run.ID, last[r.run.ID]) })
		}
	}

	from, to := patchStart.Add(plan.Warmup), patchStart.Add(plan.Warmup+plan.Window)
	var samples []time.Duration
	frames, versions, late, gaps := 0, 0, 0, 0
	for _, r := range runs {
		final, err := runVersion(ctx, w.Agents[r.adapter].Client, r.run.ID)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		versions += len(sentAt[r.run.ID])
		for _, v := range r.viewers {
			m := agentpath.NewViewer(sentAt[r.run.ID], from, to)
			for _, f := range viewers[v].RunFrames() {
				if f.RunID == r.run.ID {
					m.Apply(f.ViewerFrame)
				}
			}
			samples = append(samples, m.Samples...)
			frames += m.Frames - 1
			late += m.Late
			gaps += m.Gaps
			switch {
			case m.Late > 0 || m.Gaps > 0:
				problems = append(problems, fmt.Sprintf("viewer %d of run %s: %d late snapshots, %d gaps", v+1, r.run.ID, m.Late, m.Gaps))
			case m.Version() != final:
				problems = append(problems, fmt.Sprintf("viewer %d of run %s ends at version %d, getRun says %d", v+1, r.run.ID, m.Version(), final))
			}
			if miss := m.Missing(); len(miss) > 0 {
				problems = append(problems, fmt.Sprintf("viewer %d of run %s: %d versions of the window never arrived, first %d", v+1, r.run.ID, len(miss), miss[0]))
			}
		}
	}
	for _, a := range adapters {
		a.FinishRuns(ctx)
	}
	maxCalls, sentInWindow := 0, 0
	for i, a := range adapters {
		rep := a.Report()
		maxCalls = max(maxCalls, rep.MaxCallsPerSecond)
		if err := rep.Err(); err != nil {
			problems = append(problems, fmt.Sprintf("adapter of %s: %v", w.Agents[i].Handle, err))
		}
		for _, s := range rep.Sent {
			if !s.Scheduled.Before(from) && s.Scheduled.Before(to) {
				sentInWindow++
			}
		}
	}
	problems = append(problems, agentworld.SendProblems(sends)...)
	for i, v := range viewers {
		if errs := v.Errors(); len(errs) > 0 {
			problems = append(problems, fmt.Sprintf("viewer %d: %d findings, first: %s", i+1, len(errs), errs[0]))
		}
	}
	states, err := agentworld.RunStates(ctx, p.AdminDB())
	if err != nil {
		t.Fatal(err)
	}
	if states["completed"] != len(runs) || len(states) != 1 {
		problems = append(problems, fmt.Sprintf("runs by state %v, want all %d completed", states, len(runs)))
	}
	if err := agentpath.CheckMinSamples(th.Name, len(samples), plan.MinSamples); err != nil {
		problems = append(problems, err.Error())
	}
	for _, pr := range problems {
		t.Errorf("agent path: %s", pr)
	}

	bgIn := 0
	for _, s := range sends {
		if !s.Scheduled.Before(bgStart.Add(plan.Warmup)) {
			bgIn++
		}
	}
	extra := map[string]any{"run_patch.params": plan.Params(), "run_patch.gomaxprocs": env.ServerProcs(),
		"run_patch.achieved_patches_per_s":    float64(sentInWindow) / plan.Window.Seconds(),
		"run_patch.achieved_background_per_s": float64(bgIn) / plan.Window.Seconds(),
		"run_patch.viewer_connections":        len(viewers), "run_patch.late_snapshots": late, "run_patch.gaps": gaps,
		"run_patch.planned_calls_per_agent_per_s": plan.AgentCallsPerSecond(agentpath.ModePatch), "run_patch.max_calls_per_agent_per_s": maxCalls,
	}
	if versions > 0 {
		// Frames per version and viewer: below one when coalescing joins versions.
		extra["run_patch.frames_per_version"] = float64(frames) / float64(versions*agentpath.ViewersPerRun)
	}
	ms := func(xs []time.Duration, p float64) float64 {
		v, err := report.DurationsP(xs, p)
		if err != nil {
			return 0
		}
		return v / float64(time.Millisecond)
	}
	extra["run_patch.p50_ms"], extra["run_patch.max_ms"] = ms(samples, 50), ms(samples, 100)
	value := 0.0
	if len(samples) > 0 {
		value, _ = report.DurationsP(samples, 99)
	}
	result := th.Result(t.Name(), value, len(samples))
	// A measurement counts only when the path behaved correctly while it was taken (D-M2-36-5).
	result.Pass = result.Pass && len(problems) == 0
	env.Record(t, []report.Result{result}, extra)
}

// setupTriggers posts one trigger per patch run, evenly spaced at the trigger rate of the plan:
// trigger j goes to agent j mod A, alternating over its channels, so every agent gets R runs.
func setupTriggers(ctx context.Context, w *agentworld.World, plan agentpath.Plan) []agentworld.Sent {
	runs := plan.PatchRuns()
	out := make([]agentworld.Sent, runs)
	interval := time.Duration(float64(time.Second) / plan.TriggerRate)
	start := time.Now()
	var wg sync.WaitGroup
	for j := range runs {
		due := start.Add(time.Duration(j) * interval)
		if waitUntil(ctx, due) != nil {
			break
		}
		agent := j % plan.Agents
		channel := agent*agentpath.ChannelsPerAgent + (j/plan.Agents)%agentpath.ChannelsPerAgent
		wg.Go(func() { out[j] = w.TriggerIn(ctx, channel, j, due) })
	}
	wg.Wait()
	return out
}

// hasSnapshot reports whether fs hold a snapshot of run.
func hasSnapshot(fs []agentworld.RunFrame, run string) bool {
	for _, f := range fs {
		if f.RunID == run && f.Snapshot {
			return true
		}
	}
	return false
}

// reached reports whether fs carry run at version v or above.
func reached(fs []agentworld.RunFrame, run string, v int64) bool {
	for _, f := range fs {
		if f.RunID == run && f.Version >= v {
			return true
		}
	}
	return false
}

// runVersion reads the version of run through getRun as its agent.
func runVersion(ctx context.Context, c *harness.Client, run string) (int64, error) {
	r := c.Do(ctx, harness.Request{Method: http.MethodGet, Path: "/runs/" + run})
	if r.Status != http.StatusOK {
		return 0, fmt.Errorf("getRun of run %s = %d", run, r.Status)
	}
	var doc struct {
		Version int64 `json:"version"`
	}
	if err := json.Unmarshal(r.Body, &doc); err != nil {
		return 0, err
	}
	return doc.Version, nil
}

// waitUntil waits until t or the end of ctx.
func waitUntil(ctx context.Context, t time.Time) error {
	timer := time.NewTimer(time.Until(t))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
