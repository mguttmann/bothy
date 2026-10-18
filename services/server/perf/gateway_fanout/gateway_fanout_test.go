// SPDX-License-Identifier: Apache-2.0

//go:build perf

package gatewayfanout_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/perf/internal/perfkit"
	"github.com/OWNER/bothy/services/server/perf/internal/report"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// The parameters of the measurement (perf/README.md).
const (
	warmup = 5 * time.Second
	window = 30 * time.Second
	// receivers is how many people with read on the space hold a gateway connection.
	receivers = 100
	// threads is how many channels of the space (their main threads) take the messages, as in
	// load_messages: one thread serializes its messages (thread_seq).
	threads = 10
	// headroom is the share of ratelimit.rl.message.per_minute each sender uses.
	headroom = 0.8
	// settle is how long the test waits after the last send for every receiver to get every event.
	settle = 30 * time.Second
)

// got is one dispatch a receiver saw.
type got struct {
	seq         int64
	typ         string
	clientMsgID string
	occurred    time.Time
	received    time.Time
}

// receiver collects the dispatches of one connection.
type receiver struct {
	g   *harness.GatewayConn
	mu  sync.Mutex
	got []got
	err error
}

// collect reads dispatches until the connection ends.
func (r *receiver) collect() {
	for {
		f, err := r.g.Next(context.Background())
		if err != nil {
			r.mu.Lock()
			r.err = err
			r.mu.Unlock()
			return
		}
		if f.Op != "dispatch" {
			continue
		}
		var env struct {
			Seq         int64     `json:"seq"`
			Type        string    `json:"type"`
			ClientMsgID string    `json:"client_msg_id"`
			OccurredAt  time.Time `json:"occurred_at"`
		}
		if err := json.Unmarshal(f.D, &env); err != nil {
			continue
		}
		r.mu.Lock()
		r.got = append(r.got, got{seq: env.Seq, typ: env.Type, clientMsgID: env.ClientMsgID, occurred: env.OccurredAt, received: f.Received})
		r.mu.Unlock()
	}
}

// snapshot returns the dispatches so far.
func (r *receiver) snapshot() []got {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]got(nil), r.got...)
}

// TestPerf_FanoutLocalP99 measures commit to dispatch on the same node: receivers with read on a
// space are connected while senders write into its channels at perf.sustained_messages_per_s. Latency per
// event and receiver is the receive time minus occurred_at of the envelope (transaction time, at
// or before the commit, so an upper bound), corrected by the clock offset between the database
// and the host. Every receiver must get every event exactly once and in seq order.
func TestPerf_FanoutLocalP99(t *testing.T) {
	env := perfkit.Setup(t)
	th := env.Threshold(t, "perf.fanout_local_p99")
	rate := float64(env.Count(t, "perf.sustained_messages_per_s"))
	n := perfkit.MessagesPerSender(rate, perfkit.DefaultInt(t, "ratelimit.rl.message.per_minute"), headroom)

	p := env.StartSeeding(t, nil)
	space := perfkit.Space(t, p, "Fanout")
	chans := perfkit.Channels(t, p, space, "fanout", threads)
	threadIDs := make([]string, len(chans))
	for i, c := range chans {
		threadIDs[i] = c.Thread
	}
	senders := perfkit.NewUsers(t, p, "sender", n)
	readers := perfkit.NewUsers(t, p, "receiver", receivers)
	perfkit.Parallel(t, n+receivers, func(ctx context.Context, i int) error {
		if i < n {
			return perfkit.Grant(ctx, p, senders[i].PrincipalID, "space", space, "write")
		}
		return perfkit.Grant(ctx, p, readers[i-n].PrincipalID, "space", space, "read")
	})

	env.Measure(t, p, nil)
	offset := perfkit.ClockOffset(t, p)
	recv := make([]*receiver, receivers)
	perfkit.Parallel(t, receivers, func(ctx context.Context, i int) error {
		g, err := readers[i].Gateway(ctx)
		if err != nil {
			return err
		}
		recv[i] = &receiver{g: g}
		go recv[i].collect() //nolint:gosec // G118: a receiver reads until its connection ends, not until a request ends
		return nil
	})
	for _, r := range recv {
		defer r.g.Close()
	}
	start, sent := perfkit.OpenLoop(t.Context(), senders, threadIDs, rate, warmup, window)
	_, _, sendErr := perfkit.Window(sent, start)
	if sendErr != nil {
		t.Error(sendErr)
	}
	want := map[string]perfkit.Sent{}
	for _, s := range sent {
		if s.Status >= 200 && s.Status <= 299 {
			want[s.ClientMsgID] = s
		}
	}
	deadline := time.Now().Add(settle)
	for _, r := range recv {
		for countSent(r.snapshot(), want) < len(want) && time.Now().Before(deadline) {
			time.Sleep(100 * time.Millisecond)
		}
	}
	measureFrom := start.Add(warmup)
	var lat []time.Duration
	delivered := true
	for i, r := range recv {
		if err := check(r.snapshot(), want); err != nil {
			delivered = false
			r.mu.Lock()
			end := r.err
			r.mu.Unlock()
			t.Errorf("receiver %d (connection end: %v): %v", i, end, err)
		}
		for _, d := range r.snapshot() {
			if s, ok := want[d.clientMsgID]; ok && d.typ == "message.created" && !s.Scheduled.Before(measureFrom) {
				lat = append(lat, report.CorrectedLatency(d.received, d.occurred, offset))
			}
		}
	}
	p99, err := report.DurationsP(lat, 99)
	if err != nil {
		t.Fatal(err)
	}
	p50, _ := report.DurationsP(lat, 50)
	res := th.Result(t.Name(), p99, len(lat))
	// A load the server did not take or did not deliver is no pass, whatever the latency.
	res.Pass = res.Pass && sendErr == nil && delivered
	env.Record(t, []report.Result{res}, map[string]any{
		"gateway_fanout.db_clock_offset_ms": float64(offset) / float64(time.Millisecond),
		"gateway_fanout.failed_sends":       len(sent) - len(want),
		"gateway_fanout.p50_ms":             p50 / float64(time.Millisecond),
		"gateway_fanout.params": map[string]any{"rate_per_s": rate, "senders": n, "receivers": receivers, "threads": threads,
			"warmup_s": warmup.Seconds(), "window_s": window.Seconds(), "headroom": headroom},
	})
}

// countSent returns how many of the sent messages a receiver has seen.
func countSent(gs []got, want map[string]perfkit.Sent) int {
	n := 0
	for _, g := range gs {
		if _, ok := want[g.clientMsgID]; ok && g.typ == "message.created" {
			n++
		}
	}
	return n
}

// check holds 04 "Delivery guarantee" and "Ordering" for one receiver: every sent message exactly
// once, and the seq of every dispatch above the one before.
func check(gs []got, want map[string]perfkit.Sent) error {
	seen := map[string]int{}
	var last int64
	for _, g := range gs {
		if g.seq <= last {
			return fmt.Errorf("seq %d after %d", g.seq, last)
		}
		last = g.seq
		if _, ok := want[g.clientMsgID]; ok && g.typ == "message.created" {
			seen[g.clientMsgID]++
		}
	}
	var wrong []string
	for cm := range want {
		if seen[cm] != 1 {
			wrong = append(wrong, fmt.Sprintf("%s seen %d times", cm, seen[cm]))
		}
	}
	if len(wrong) > 0 {
		return fmt.Errorf("%d of %d messages not seen exactly once (%d dispatches), first: %s", len(wrong), len(want), len(gs), wrong[0])
	}
	return nil
}
