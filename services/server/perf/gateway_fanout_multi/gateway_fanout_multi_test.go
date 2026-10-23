// SPDX-License-Identifier: Apache-2.0

//go:build perf

package gatewayfanoutmulti_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/perf/internal/perfkit"
	"github.com/OWNER/bothy/services/server/perf/internal/report"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// The parameters of the measurement (perf/README.md), those of gateway_fanout.
const (
	warmup    = 5 * time.Second
	window    = 30 * time.Second
	receivers = 100
	threads   = 10
	headroom  = 0.8
	// settle is how long the test waits after the last send for every receiver to get every event.
	settle = 30 * time.Second
)

// The node names of the edge and its header.
const (
	nodeA      = "a"
	nodeB      = "b"
	nodeHeader = "X-Test-Node"
)

// edge is the in-process reverse proxy in front of the two processes: both share
// server.public_url; the header nodeHeader picks the node of a request or a gateway connection,
// node a without it.
type edge struct {
	URL     string
	mu      sync.Mutex
	targets map[string]*url.URL
}

// newEdge starts the edge on 127.0.0.1; it stops in t.Cleanup.
func newEdge(t *testing.T) *edge {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	e := &edge{URL: "http://" + ln.Addr().String(), targets: map[string]*url.URL{}}
	proxy := &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
		node := r.In.Header.Get(nodeHeader)
		if node == "" {
			node = nodeA
		}
		e.mu.Lock()
		target := e.targets[node]
		e.mu.Unlock()
		r.SetURL(target)
		r.Out.Header.Del(nodeHeader)
		r.Out.Host = r.In.Host
		// Keep the documentation address of each simulated client: the server trusts the
		// loopback proxies and counts the client by the address before them.
		r.Out.Header["X-Forwarded-For"] = r.In.Header["X-Forwarded-For"]
		r.SetXForwarded()
	}}
	srv := &http.Server{Handler: proxy, ReadHeaderTimeout: time.Minute}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return e
}

// set returns the OnListen hook of node.
func (e *edge) set(node string) func(string) {
	return func(origin string) {
		u, err := url.Parse(origin)
		if err != nil {
			panic(err)
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		e.targets[node] = u
	}
}

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

// presenceWrites returns the rows inserted, updated and deleted in gateway_presence so far
// (pg_stat_user_tables).
func presenceWrites(t *testing.T, p *harness.Process) int64 {
	t.Helper()
	var n int64
	err := p.AdminDB().QueryRow(context.Background(), `SELECT coalesce(sum(n_tup_ins + n_tup_upd + n_tup_del), 0)::bigint
		FROM pg_stat_user_tables WHERE relname = 'gateway_presence'`).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// TestPerf_FanoutCrossNodeP99 measures commit to dispatch across two api nodes (02 "Performance
// targets", perf.fanout_cross_node_p99): process a (role all, cluster.mode multi) takes the
// messages of the senders at perf.sustained_messages_per_s, process b (role api, cluster.mode
// multi) on the same database holds the gateway connections of the receivers, so every event
// reaches them through the log tail of node b. Latency per event and receiver is the receive time
// minus occurred_at, corrected by the clock offset between the database and the host. Every
// receiver must get every event exactly once and in seq order. extra records the write rate of
// gateway_presence under this load (the flush of node b).
func TestPerf_FanoutCrossNodeP99(t *testing.T) {
	env := perfkit.Setup(t)
	th := env.Threshold(t, "perf.fanout_cross_node_p99")
	rate := float64(env.Count(t, "perf.sustained_messages_per_s"))
	n := perfkit.MessagesPerSender(rate, perfkit.DefaultInt(t, "ratelimit.rl.message.per_minute"), headroom)
	e := newEdge(t)
	multi := map[string]string{"cluster.mode": "multi"}

	seed := harness.RaisedRateLimits()
	for k, v := range multi {
		seed[k] = v
	}
	p := harness.StartProcess(t, harness.ProcessOptions{Config: seed, GOMAXPROCS: env.GOMAXPROCS, DBCPUs: env.DBCPUs, DBMemoryBytes: env.DBMemoryBytes,
		PublicURL: e.URL, OnListen: e.set(nodeA)})
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

	env.Measure(t, p, multi)
	b := harness.StartProcessNode(t, p, harness.ProcessOptions{Config: multi, GOMAXPROCS: env.GOMAXPROCS, Role: wire.RoleAPI,
		PublicURL: e.URL, OnListen: e.set(nodeB)})
	offset := perfkit.ClockOffset(t, p)
	recv := make([]*receiver, receivers)
	perfkit.Parallel(t, receivers, func(ctx context.Context, i int) error {
		g, err := readers[i].GatewayWith(ctx, http.Header{nodeHeader: []string{nodeB}})
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
	writesBefore, writesFrom := presenceWrites(t, b), time.Now()
	start, sent := perfkit.OpenLoop(t.Context(), senders, threadIDs, rate, warmup, window)
	writes, writesFor := presenceWrites(t, b)-writesBefore, time.Since(writesFrom)
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
	deadline := time.Now().Add(settle) // nosemgrep: bothy-go-test-wall-clock-wait -- the settle time is a latency bound under test
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
	res.Pass = res.Pass && sendErr == nil && delivered
	env.Record(t, []report.Result{res}, map[string]any{
		"gateway_fanout_multi.db_clock_offset_ms":    float64(offset) / float64(time.Millisecond),
		"gateway_fanout_multi.failed_sends":          len(sent) - len(want),
		"gateway_fanout_multi.p50_ms":                p50 / float64(time.Millisecond),
		"gateway_fanout_multi.presence_writes_per_s": float64(writes) / writesFor.Seconds(),
		"gateway_fanout_multi.params": map[string]any{"rate_per_s": rate, "senders": n, "receivers": receivers, "threads": threads,
			"warmup_s": warmup.Seconds(), "window_s": window.Seconds(), "headroom": headroom,
			"presence_flush_interval": perfkit.DefaultDuration(t, "cluster.presence_flush_interval").String()},
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
