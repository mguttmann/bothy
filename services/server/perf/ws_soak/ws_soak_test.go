// SPDX-License-Identifier: Apache-2.0

//go:build perf

package wssoak_test

import (
	"context"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/perf/internal/perfkit"
	"github.com/OWNER/bothy/services/server/perf/internal/report"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// The parameters of the soak (perf/README.md).
const (
	// settle is the wait before each RSS reading: after ready, and after the last connection.
	settle = 10 * time.Second
	// intervals is how many sync.heartbeat_interval the connections stay idle with heartbeats.
	intervals = 2
	// ackGrace is how long the test waits for the acknowledgement of the last heartbeats.
	ackGrace = 15 * time.Second
	// fdReserve is the descriptors the test process and the server need besides the connections.
	fdReserve = 1024
)

// TestPerf_WSConnectionsPerNode opens perf.ws_connections_per_node gateway connections to a server
// process, each a distinct seeded device through identify to ready, and keeps them idle with
// heartbeats for at least two sync.heartbeat_interval. It passes when all stay connected without a
// close frame and every heartbeat is acknowledged; memory per connection is the RSS with all
// connections after settling minus the RSS after ready, divided by the count
// (perf.ws_idle_conn_mem_max).
func TestPerf_WSConnectionsPerNode(t *testing.T) {
	env := perfkit.Setup(t)
	thConns := env.Threshold(t, "perf.ws_connections_per_node")
	thMem := env.Threshold(t, "perf.ws_idle_conn_mem_max")
	n := int(thConns.Base)
	raiseFileLimit(t, uint64(2*n+fdReserve)) //nolint:gosec // G115: n is a positive count of the 02 table

	p := env.StartSeeding(t, nil)
	people := perfkit.NewUsers(t, p, "idle", n)
	env.Measure(t, p, nil)
	interval := perfkit.DefaultDuration(t, "sync.heartbeat_interval")
	// A long seeding can outlive auth.access_token_ttl; one read renews an expired token pair.
	perfkit.Parallel(t, n, func(ctx context.Context, i int) error {
		_, err := perfkit.Do(ctx, people[i], harness.Request{Method: http.MethodGet, Path: "/me"}, http.StatusOK)
		return err
	})
	time.Sleep(settle)
	rssReady, err := p.RSSBytes()
	if err != nil {
		t.Fatal(err)
	}
	conns := make([]*harness.GatewayConn, n)
	perfkit.Parallel(t, n, func(ctx context.Context, i int) error {
		g, err := people[i].Gateway(ctx)
		conns[i] = g
		return err
	})
	for _, g := range conns {
		defer g.Close()
	}
	time.Sleep(settle)
	rssAll, err := p.RSSBytes()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(intervals*interval + interval/2)
	alive, unacked, fewBeats := 0, 0, 0
	deadline := time.Now().Add(ackGrace) // nosemgrep: bothy-go-test-wall-clock-wait -- the ack grace is a latency bound under test
	for _, g := range conns {
		for {
			sent, acked := g.Heartbeats()
			if acked >= sent || time.Now().After(deadline) {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		sent, acked := g.Heartbeats()
		switch {
		case g.Err() != nil:
			t.Errorf("a connection ended: %v", g.Err())
		case acked < sent:
			unacked++
		case sent < intervals:
			fewBeats++
		default:
			alive++
		}
	}
	if unacked > 0 || fewBeats > 0 {
		t.Errorf("%d connections with unacknowledged heartbeats, %d with fewer than %d heartbeats", unacked, fewBeats, intervals)
	}
	perConn := float64(rssAll-rssReady) / float64(n)
	env.Record(t, []report.Result{
		thConns.Result(t.Name(), float64(alive), n),
		thMem.Result(t.Name(), perConn, n),
	}, map[string]any{
		"ws_soak.rss_ready_bytes": rssReady, "ws_soak.rss_connected_bytes": rssAll,
		"ws_soak.params": map[string]any{"connections": n, "heartbeat_interval_s": interval.Seconds(), "intervals": intervals,
			"settle_s": settle.Seconds()},
	})
}

// raiseFileLimit fails t with a clear message when the hard RLIMIT_NOFILE of the test process is
// below need, and raises the soft limit to need; the server process inherits it.
func raiseFileLimit(t *testing.T, need uint64) {
	t.Helper()
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		t.Fatal(err)
	}
	if lim.Max < need {
		t.Fatalf("RLIMIT_NOFILE hard limit %d is below the %d descriptors of the soak; raise it (ulimit -Hn) before make perf", lim.Max, need)
	}
	if lim.Cur < need {
		lim.Cur = need
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
			t.Fatalf("raise RLIMIT_NOFILE to %d: %v", need, err)
		}
	}
}
