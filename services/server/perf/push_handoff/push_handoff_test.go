// SPDX-License-Identifier: Apache-2.0

//go:build perf

package pushhandoff_test

import (
	"context"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/perf/internal/perfkit"
	"github.com/OWNER/bothy/services/server/perf/internal/pushhandoff"
	"github.com/OWNER/bothy/services/server/perf/internal/pushworld"
	"github.com/OWNER/bothy/services/server/perf/internal/report"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// note is the caveat of design note 3 c that results.json carries under extra.
const note = "the server, the load generator and Fake-APNs share the CPUs of one process; the value is an upper bound of the server's own time"

// TestPerf_PushHandoffP95 measures perf.push_handoff_p95 (brief M3-24 design notes 6 to 9): the
// time from the commit of a message that mentions a person (occurred_at of its transaction) to
// the 200 of Fake-APNs for that person's registration, under perf.sustained_messages_per_s human
// messages with the default rl.message, on a durable database, with the server in the test
// process. The value counts only when every mention of the window gave exactly one correct push
// per registration and no request carried text (pushhandoff.Check).
func TestPerf_PushHandoffP95(t *testing.T) {
	env := perfkit.Setup(t)
	th := env.Threshold(t, "perf.push_handoff_p95")
	in, err := pushhandoff.Defaults()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := pushhandoff.NewPlan(in)
	if err != nil {
		t.Fatal(err)
	}
	if env.GOMAXPROCS > 0 {
		defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(env.GOMAXPROCS))
	}
	srv, fake := pushworld.StartServer(t, harness.Options{DurableDB: true, DBCPUs: env.DBCPUs, DBMemoryBytes: env.DBMemoryBytes})
	db := srv.AdminDB()
	env.ReadDurability(t, db)
	seedStart := time.Now()
	w, err := pushworld.Seed(t.Context(), srv, plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("seeded %d senders, %d recipients, %d channels in %s", plan.Senders, plan.Recipients, plan.Channels, time.Since(seedStart).Round(time.Second))

	msgs, err := plan.Schedule(plan.Total())
	if err != nil {
		t.Fatal(err)
	}
	canary := "canary-" + ids.New().String()
	sends := make([]pushhandoff.Send, len(msgs))
	lags := sampleLag(t.Context(), db)
	start, n := perfkit.Schedule(t.Context(), plan.Rate, plan.Warmup, plan.Window, func(i int, due time.Time) {
		sends[i] = w.Send(t.Context(), msgs[i], canary, due)
	})
	sends = sends[:n]
	select {
	case <-t.Context().Done():
		t.Fatal("cancelled during Drain")
	case <-time.After(plan.Drain):
	}
	lag := lags.stop()

	var mentionIDs []string
	for _, s := range sends {
		if len(s.Recipients) > 0 && s.MessageID != "" {
			mentionIDs = append(mentionIDs, s.MessageID)
		}
	}
	occurred, err := pushworld.Occurred(t.Context(), db, mentionIDs)
	if err != nil {
		t.Fatal(err)
	}
	unhealthy, err := pushworld.Unhealthy(t.Context(), db)
	if err != nil {
		t.Fatal(err)
	}
	offset := perfkit.ClockOffsetOf(t, db)
	from, to := start.Add(plan.Warmup), start.Add(plan.Warmup+plan.Window)
	res := pushhandoff.Check(pushhandoff.Run{Sends: sends, Registrations: w.Registrations(), Requests: fake.Requests(), Occurred: occurred,
		Offset: offset, From: from, To: to, Canary: canary, Unhealthy: unhealthy, MinSamples: plan.MinSamples})
	for _, p := range res.Problems {
		t.Errorf("push path: %s", strings.ReplaceAll(p, canary, "<canary>"))
	}

	extra := map[string]any{"push_handoff.params": plan.Params(), "push_handoff.gomaxprocs": env.ServerProcs(), "push_handoff.note": note,
		"push_handoff.clock_offset_ms": float64(offset) / float64(time.Millisecond)}
	inWindow, mentionsInWindow := 0, 0
	for _, s := range sends {
		if !s.Scheduled.Before(from) && s.Scheduled.Before(to) {
			inWindow++
			if len(s.Recipients) > 0 {
				mentionsInWindow++
			}
		}
	}
	extra["push_handoff.achieved_messages_per_s"] = float64(inWindow) / plan.Window.Seconds()
	extra["push_handoff.achieved_mentions_per_s"] = float64(mentionsInWindow) / plan.Window.Seconds()
	extra["push_handoff.expected_pushes"], extra["push_handoff.received_pushes"], extra["push_handoff.matched_pushes"] = res.Expected, res.Received, res.Matched
	extra["push_handoff.consumer_lag_max"], extra["push_handoff.consumer_lag_mean"], extra["push_handoff.consumer_lag_samples"] = lag.max, lag.mean, lag.n
	value := 0.0
	if len(res.Samples) > 0 {
		for name, p := range map[string]float64{"p50": 50, "p99": 99, "max": 100} {
			v, _ := report.DurationsP(res.Samples, p)
			extra["push_handoff."+name+"_ms"] = v / float64(time.Millisecond)
		}
		p95, err := pushhandoff.P95(res.Samples)
		if err != nil {
			t.Fatal(err)
		}
		value = float64(p95)
	}
	result := th.Result(t.Name(), value, len(res.Samples))
	// A measurement counts only when the path behaved correctly while it was taken (D-M3-24-5).
	result.Pass = result.Pass && res.OK() && len(res.Samples) > 0
	env.Record(t, []report.Result{result}, extra)
}

// lagStats are the consumer lag samples of a run.
type lagStats struct {
	max  int64
	mean float64
	n    int
}

// lagSampler reads the lag of the consumer push every LagSampleEvery until stop.
type lagSampler struct {
	done   chan struct{}
	exited chan struct{}
	mu     sync.Mutex
	xs     []int64
}

// sampleLag starts the lag samples of design note 8 d (read-only).
func sampleLag(ctx context.Context, q pushworld.Querier) *lagSampler {
	s := &lagSampler{done: make(chan struct{}), exited: make(chan struct{})}
	go func() {
		defer close(s.exited)
		tick := time.NewTicker(pushhandoff.LagSampleEvery)
		defer tick.Stop()
		for {
			select {
			case <-s.done:
				return
			case <-ctx.Done():
				return
			case <-tick.C:
			}
			if v, err := pushworld.ConsumerLag(ctx, q); err == nil {
				s.mu.Lock()
				s.xs = append(s.xs, v)
				s.mu.Unlock()
			}
		}
	}()
	return s
}

// stop ends the samples and returns their maximum and mean.
func (s *lagSampler) stop() lagStats {
	close(s.done)
	<-s.exited
	s.mu.Lock()
	defer s.mu.Unlock()
	st := lagStats{n: len(s.xs)}
	var sum int64
	for _, v := range s.xs {
		st.max = max(st.max, v)
		sum += v
	}
	if st.n > 0 {
		st.mean = float64(sum) / float64(st.n)
	}
	return st
}
