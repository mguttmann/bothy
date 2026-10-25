// SPDX-License-Identifier: Apache-2.0

//go:build perf

package invocation_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/perf/internal/agentpath"
	"github.com/OWNER/bothy/services/server/perf/internal/agentworld"
	"github.com/OWNER/bothy/services/server/perf/internal/fakeadapter"
	"github.com/OWNER/bothy/services/server/perf/internal/perfkit"
	"github.com/OWNER/bothy/services/server/perf/internal/report"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// pollEvery is how often the drain looks at the adapters.
const pollEvery = 100 * time.Millisecond

// TestPerf_InvocationDeliveryP99 measures perf.invocation_delivery_p99 (brief M2-36 design notes
// 6 and 8): human messages at perf.sustained_messages_per_s into the channels of the agents, every
// TriggerEvery-th one mentioning the channel's operator agent, against the measured server in the
// default configuration on a durable database. The sample of a trigger is the receipt of its
// agent.invocation at the fake adapter minus occurred_at of the envelope (the transaction of the
// triggering message, an upper bound of the commit), corrected by the clock offset between
// database and host. The value counts only when every trigger got exactly one invocation with a
// token the reference verifier accepted, nothing was redelivered or rejected, every run ended
// completed and no request or connection failed.
func TestPerf_InvocationDeliveryP99(t *testing.T) {
	env := perfkit.Setup(t)
	th := env.Threshold(t, "perf.invocation_delivery_p99")
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
	offset := perfkit.ClockOffset(t, p)
	adapters, err := w.StartAdapters(t, p.Server, plan, agentpath.ModeInvocation)
	if err != nil {
		t.Fatal(err)
	}
	viewers, err := w.ConnectViewers(t)
	if err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	sends := make([]agentworld.Sent, plan.Total())
	start, n := perfkit.Schedule(ctx, plan.Rate, plan.Warmup, plan.Window, func(i int, due time.Time) {
		if agentpath.IsTrigger(i) {
			sends[i] = w.Trigger(ctx, i/agentpath.TriggerEvery, due)
			return
		}
		sends[i] = w.Background(ctx, i-i/agentpath.TriggerEvery-1, due)
	})
	sends = sends[:n]
	var triggers []agentpath.Trigger
	for _, s := range sends {
		if s.Trigger {
			triggers = append(triggers, agentpath.Trigger{MessageID: s.MessageID, Scheduled: s.Scheduled})
		}
	}
	drain(t, adapters, len(triggers), plan.Drain)

	from, to := start.Add(plan.Warmup), start.Add(plan.Warmup+plan.Window)
	var invs []agentpath.Invocation
	var startRTT, finishRTT []time.Duration
	maxCalls := 0
	var problems []string
	for i, a := range adapters {
		rep := a.Report()
		invs = append(invs, rep.Invocations...)
		startRTT, finishRTT = append(startRTT, rep.StartRTT...), append(finishRTT, rep.FinishRTT...)
		maxCalls = max(maxCalls, rep.MaxCallsPerSecond)
		if err := rep.Err(); err != nil {
			problems = append(problems, fmt.Sprintf("adapter of %s: %v", w.Agents[i].Handle, err))
		}
	}
	res := agentpath.InvocationSamples(triggers, invs, offset, from, to)
	problems = append(problems, res.Problems()...)
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
	created := 0
	for _, tr := range triggers {
		if tr.MessageID != "" {
			created++
		}
	}
	if states["completed"] != created || len(states) != 1 {
		problems = append(problems, fmt.Sprintf("runs by state %v, want all %d completed", states, created))
	}
	if err := agentpath.CheckMinSamples(th.Name, len(res.Samples), plan.MinSamples); err != nil {
		problems = append(problems, err.Error())
	}
	for _, pr := range problems {
		t.Errorf("agent path: %s", pr)
	}

	extra := map[string]any{"invocation.params": plan.Params(), "invocation.gomaxprocs": env.ServerProcs(),
		"invocation.db_clock_offset_ms": float64(offset) / float64(time.Millisecond)}
	trigIn, bgIn := 0, 0
	for _, s := range sends {
		if !s.Scheduled.Before(from) && s.Scheduled.Before(to) {
			if s.Trigger {
				trigIn++
			} else {
				bgIn++
			}
		}
	}
	extra["invocation.achieved_trigger_per_s"] = float64(trigIn) / plan.Window.Seconds()
	extra["invocation.achieved_background_per_s"] = float64(bgIn) / plan.Window.Seconds()
	extra["invocation.invocations_received"] = len(invs)
	extra["invocation.planned_calls_per_agent_per_s"] = plan.AgentCallsPerSecond(agentpath.ModeInvocation)
	extra["invocation.max_calls_per_agent_per_s"] = maxCalls
	ms := func(xs []time.Duration, p float64) float64 {
		v, err := report.DurationsP(xs, p)
		if err != nil {
			return 0
		}
		return v / float64(time.Millisecond)
	}
	extra["invocation.p50_ms"], extra["invocation.max_ms"] = ms(res.Samples, 50), ms(res.Samples, 100)
	extra["invocation.start_run_p99_ms"], extra["invocation.finish_run_p99_ms"] = ms(startRTT, 99), ms(finishRTT, 99)
	value := 0.0
	if len(res.Samples) > 0 {
		value, _ = report.DurationsP(res.Samples, 99)
	}
	result := th.Result(t.Name(), value, len(res.Samples))
	// A measurement counts only when the path behaved correctly while it was taken (D-M2-36-5).
	result.Pass = result.Pass && len(problems) == 0
	env.Record(t, []report.Result{result}, extra)
}

// drain waits up to limit after the last send until the adapters have finished (or failed) as
// many runs as there were triggers.
func drain(t *testing.T, adapters []*fakeadapter.Adapter, triggers int, limit time.Duration) {
	t.Helper()
	deadline := time.Now().Add(limit) // nosemgrep: bothy-go-test-wall-clock-wait -- Drain is a latency bound under test
	for time.Now().Before(deadline) {
		done := 0
		for _, a := range adapters {
			r := a.Report()
			done += r.Finished + r.Failed
		}
		if done >= triggers {
			return
		}
		time.Sleep(pollEvery)
	}
}
