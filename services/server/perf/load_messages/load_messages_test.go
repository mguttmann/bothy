// SPDX-License-Identifier: Apache-2.0

//go:build perf

package loadmessages_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/perf/internal/perfkit"
	"github.com/OWNER/bothy/services/server/perf/internal/report"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// The parameters of the measurement (perf/README.md).
const (
	// warmup is excluded from the percentile; window is the measured time after it.
	warmup = 10 * time.Second
	window = 60 * time.Second
	// threads is how many shared threads (main threads of as many channels) take the messages.
	threads = 10
	// receivers is how many of the senders hold a gateway connection during the load.
	receivers = 50
	// headroom is the share of ratelimit.rl.message.per_minute each sender uses.
	headroom = 0.8
)

// TestPerf_MessageCreateP99 measures createMessage under open-loop arrivals at
// perf.sustained_messages_per_s, spread over enough senders that each stays within the default
// ratelimit.rl.message.*, into shared threads while gateway receivers are connected. Latency runs
// from the scheduled send time to the answer on loopback. Any non-2xx answer fails the test. It
// also records the pool occupancy and lock waiters of the database and the database size per
// thousand messages (extra of results.json).
func TestPerf_MessageCreateP99(t *testing.T) {
	env := perfkit.Setup(t)
	th := env.Threshold(t, "perf.message_create_p99")
	rate := float64(env.Count(t, "perf.sustained_messages_per_s"))
	n := perfkit.MessagesPerSender(rate, perfkit.DefaultInt(t, "ratelimit.rl.message.per_minute"), headroom)

	p := env.StartSeeding(t, nil)
	space := perfkit.Space(t, p, "Load")
	chans := perfkit.Channels(t, p, space, "load", threads)
	senders := perfkit.NewUsers(t, p, "sender", n)
	perfkit.Parallel(t, n, func(ctx context.Context, i int) error {
		return perfkit.Grant(ctx, p, senders[i].PrincipalID, "space", space, "write")
	})
	threadIDs := make([]string, len(chans))
	for i, c := range chans {
		threadIDs[i] = c.Thread
	}

	env.Measure(t, p, nil)
	conns := make([]*harness.GatewayConn, min(receivers, n))
	perfkit.Parallel(t, len(conns), func(ctx context.Context, i int) error {
		g, err := senders[i].Gateway(ctx)
		conns[i] = g
		return err
	})
	for _, g := range conns {
		go drain(g)
		defer g.Close()
	}
	before := sizes(t, p)
	sampler := perfkit.SampleDB(p)
	start, sent := perfkit.OpenLoop(t.Context(), senders, threadIDs, rate, warmup, window)
	extra := sampler.Stop("load_messages.", perfkit.DefaultInt(t, "database.max_conns"))
	in, lat, sendErr := perfkit.Window(sent, start.Add(warmup))
	if sendErr != nil {
		t.Error(sendErr)
	}
	for _, g := range conns {
		if err := g.Err(); err != nil {
			t.Errorf("a receiver lost its connection during the load: %v", err)
		}
	}
	p99, err := report.DurationsP(lat, 99)
	if err != nil {
		t.Fatal(err)
	}
	after := sizes(t, p)
	thousands := float64(len(sent)) / 1000
	for k, v := range after {
		extra["load_messages."+k+"_bytes_per_1000_messages"] = (v - before[k]) / thousands
	}
	extra["load_messages.params"] = map[string]any{
		"rate_per_s": rate, "senders": n, "threads": threads, "receivers": len(conns), "warmup_s": warmup.Seconds(),
		"window_s": window.Seconds(), "headroom": headroom, "sent_in_window": len(in),
	}
	res := th.Result(t.Name(), p99, len(lat))
	// Any non-2xx answer fails the target, whatever the latency of the other sends.
	res.Pass = res.Pass && sendErr == nil
	extra["load_messages.failed_sends"] = len(in) - len(lat)
	env.Record(t, []report.Result{res}, extra)
}

// drain reads the frames of a receiver until its connection ends.
func drain(g *harness.GatewayConn) {
	for {
		if _, err := g.Next(context.Background()); err != nil {
			return
		}
	}
}

// sizes returns the size of the database, of messages and event_log with their indexes, and of
// their indexes alone.
func sizes(t *testing.T, p *harness.Process) map[string]float64 {
	t.Helper()
	var db, msgs, msgIdx, events, eventIdx float64
	err := p.AdminDB().QueryRow(t.Context(), `SELECT pg_database_size(current_database()),
		pg_total_relation_size('messages'), pg_indexes_size('messages'),
		(SELECT coalesce(sum(pg_total_relation_size(c.oid)), 0) FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid WHERE i.inhparent = 'event_log'::regclass),
		(SELECT coalesce(sum(pg_indexes_size(c.oid)), 0) FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid WHERE i.inhparent = 'event_log'::regclass)`).
		Scan(&db, &msgs, &msgIdx, &events, &eventIdx)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]float64{"database": db, "messages": msgs, "messages_indexes": msgIdx, "event_log": events, "event_log_indexes": eventIdx}
}
