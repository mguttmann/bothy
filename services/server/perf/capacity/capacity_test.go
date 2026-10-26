// SPDX-License-Identifier: Apache-2.0

//go:build perf

package capacity_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/perf/internal/perfkit"
	"github.com/OWNER/bothy/services/server/perf/internal/report"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/load"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// TestPerf_CapacitySteadyThroughput is the capacity gate (brief M7-18 Design note 2): the scenario
// of the capacity load test (one space with one text channel per load.GroupSize people, every
// sender within load.Headroom of ratelimit.rl.message.per_minute and connected to the gateway)
// sends open loop at perf.capacity_steady_messages_per_s for load.Warmup plus load.SteadyWindow.
// The goodput is the share of the window's sends answered 201 within perf.message_create_p99, per
// second (load.SummarizeSends); it must reach perf.capacity_min_messages_per_s. A connection that
// ends during the run fails the gate, whatever the goodput.
func TestPerf_CapacitySteadyThroughput(t *testing.T) {
	env := perfkit.Setup(t)
	th := env.Threshold(t, "perf.capacity_min_messages_per_s")
	target := env.Threshold(t, "perf.message_create_p99")
	rate := float64(env.Count(t, "perf.capacity_steady_messages_per_s"))
	plan, err := load.PlanStep(0, rate, perfkit.DefaultInt(t, "ratelimit.rl.message.per_minute"))
	if err != nil {
		t.Fatal(err)
	}

	p := env.StartSeeding(t, nil)
	senders := perfkit.NewUsers(t, p, "sender", plan.Senders)
	spaces := make([]string, plan.Groups)
	for g := range spaces {
		spaces[g] = perfkit.Space(t, p, fmt.Sprintf("Load %d", g+1))
	}
	threads := make([]string, plan.Groups)
	perfkit.Parallel(t, plan.Groups, func(ctx context.Context, g int) error {
		ch, err := perfkit.CreateChannel(ctx, p, spaces[g], "general")
		threads[g] = ch.Thread
		return err
	})
	// One grant after the other: in parallel they exhausted the pool of the seeding process on a
	// loaded host (503 unavailable), and the seeding is not measured.
	for i, s := range senders {
		if err := perfkit.Grant(t.Context(), p, s.PrincipalID, "space", spaces[load.GroupOf(i)], "write"); err != nil {
			t.Fatal(err)
		}
	}

	env.Measure(t, p, nil)
	conns := make([]*harness.GatewayConn, plan.Senders)
	perfkit.Parallel(t, plan.Senders, func(ctx context.Context, i int) error {
		g, err := senders[i].Gateway(ctx)
		conns[i] = g
		return err
	})
	for _, g := range conns {
		go drain(g)
		defer g.Close()
	}
	sent := make([]load.Send, int(rate*(load.Warmup+load.SteadyWindow).Seconds()))
	start, n := perfkit.Schedule(t.Context(), rate, load.Warmup, load.SteadyWindow, func(i int, due time.Time) {
		s := i % plan.Senders
		r := perfkit.Send(t.Context(), senders[s], threads[load.GroupOf(s)], due)
		sent[i] = load.Send{Scheduled: r.Scheduled, Done: r.Done, Status: r.Status}
	})
	from := start.Add(load.Warmup)
	sum := load.SummarizeSends(sent[:n], from, from.Add(load.SteadyWindow), time.Duration(target.Base))
	lost := 0
	for _, g := range conns {
		if g.Err() != nil {
			lost++
		}
	}
	res := th.Result(t.Name(), sum.GoodputPerS, sum.OK)
	// A connection the server dropped under the load is no pass, whatever the goodput.
	res.Pass = res.Pass && lost == 0
	if lost > 0 {
		t.Errorf("%d gateway connections ended during the steady run", lost)
	}
	env.Record(t, []report.Result{res}, map[string]any{
		"capacity.achieved_per_s": sum.AchievedPerS,
		"capacity.failed_sends":   sum.Failed,
		"capacity.p99_ms":         float64(sum.P99) / float64(time.Millisecond),
		"capacity.params": map[string]any{"rate_per_s": rate, "senders": plan.Senders, "groups": plan.Groups, "connections": len(conns),
			"group_size": load.GroupSize, "headroom": load.Headroom, "warmup_s": load.Warmup.Seconds(), "window_s": load.SteadyWindow.Seconds(),
			"scheduled_in_window": sum.Scheduled},
	})
}

// drain reads the frames of a connection until it ends.
func drain(g *harness.GatewayConn) {
	for {
		if _, err := g.Next(context.Background()); err != nil {
			return
		}
	}
}
