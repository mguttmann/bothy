// SPDX-License-Identifier: Apache-2.0

package multinode

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	gosync "sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// The node names of the edge.
const (
	nodeA = "a"
	nodeB = "b"
)

// nodeHeader names, on a request or a gateway upgrade to the edge, the node it goes to; without it
// the edge takes node a.
const nodeHeader = "X-Test-Node"

// The short windows of the cluster: the tests wait for conditions, bounded by testkit.Patience;
// the exact boundaries are proven by the unit tests of gateway.
var clusterConfig = map[string]string{
	"cluster.mode": "multi", "cluster.presence_flush_interval": "100ms", "cluster.presence_ttl": "1s",
	"signing_key.cache_ttl": "1s", "database.listen_reconnect_backoff_cap": "200ms", "audit.deny_aggregate_window": "50ms",
	"sync.run_patch_coalesce_interval": "100ms",
}

// clusterSlots bounds the clusters that run at once: each runs three servers with their own pools
// on the shared pgtest server.
var clusterSlots = make(chan struct{}, 2)

// edge is the in-process reverse proxy in front of the api nodes (the pattern of the multi-node
// tests of M2-47): both nodes share server.public_url, so one client reaches either; the header
// nodeHeader picks the node of a request or of a gateway connection.
type edge struct {
	URL     string
	mu      gosync.Mutex
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
		r.SetURL(e.target(r.In.Header.Get(nodeHeader)))
		r.Out.Header.Del(nodeHeader)
		r.Out.Host = r.In.Host
		r.SetXForwarded()
	}}
	srv := &http.Server{Handler: proxy, ReadHeaderTimeout: testkit.Patience}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return e
}

// set returns the OnListen hook that names the origin of node.
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

// target returns the origin of node, node a when it is empty.
func (e *edge) target(node string) *url.URL {
	e.mu.Lock()
	defer e.mu.Unlock()
	if node == "" {
		node = nodeA
	}
	return e.targets[node]
}

// via returns the header that sends a request or a gateway connection to node.
func via(node string) http.Header { return http.Header{nodeHeader: []string{node}} }

// cluster is the world of the tests: api nodes a and b behind the edge and a worker, all with
// cluster.mode multi on one database, and the matrix world of the harness (built through node a).
type cluster struct {
	t      *testing.T
	nodes  map[string]*harness.Server
	worker *harness.Server
	w      *harness.World
}

// newCluster starts the cluster.
func newCluster(t *testing.T) *cluster {
	t.Helper()
	clusterSlots <- struct{}{}
	t.Cleanup(func() { <-clusterSlots })
	conf := harness.RaisedRateLimits()
	for k, v := range clusterConfig {
		conf[k] = v
	}
	e := newEdge(t)
	opts := func(node string, role wire.Role) harness.Options {
		o := harness.Options{Config: conf, PublicURL: e.URL, Role: role, EgressDenyAll: true}
		if node != "" {
			o.OnListen = e.set(node)
		}
		return o
	}
	a := harness.Start(t, opts(nodeA, wire.RoleAPI))
	c := &cluster{t: t, nodes: map[string]*harness.Server{nodeA: a}}
	c.nodes[nodeB] = harness.StartNode(t, a, opts(nodeB, wire.RoleAPI))
	c.worker = harness.StartNode(t, a, opts("", wire.RoleWorker))
	c.w = harness.BuildMatrixWorld(t, a)
	return c
}

// count returns the integer result of query.
func (c *cluster) count(query string, args ...any) int {
	c.t.Helper()
	var n int
	if err := c.nodes[nodeA].AdminDB().QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		c.t.Fatalf("%s: %v", query, err)
	}
	return n
}

// do sends a request as cl through the edge to node.
func (c *cluster) do(node string, cl *harness.Client, method, path string, body any) harness.Response {
	c.t.Helper()
	return cl.Do(c.t.Context(), harness.Request{Method: method, Path: path, Body: body, Header: via(node)})
}

// must is do that fails unless the status is want.
func (c *cluster) must(node string, cl *harness.Client, method, path string, body any, want int) harness.Response {
	c.t.Helper()
	return harness.MustDo(c.t, cl, harness.Request{Method: method, Path: path, Body: body, Header: via(node)}, want)
}

// connect opens a gateway connection of cl on node; it closes in t.Cleanup.
func (c *cluster) connect(node string, cl *harness.Client) *harness.GatewayConn {
	c.t.Helper()
	g, err := cl.GatewayWith(c.t.Context(), via(node))
	if err != nil {
		c.t.Fatalf("gateway of %s on node %s: %v", cl.PrincipalID, node, err)
	}
	c.t.Cleanup(g.Close)
	return g
}

// dispatch is the part of a dispatch frame the tests read.
type dispatch struct {
	Type     string `json:"type"`
	Resource struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
	} `json:"resource"`
	Actor *struct {
		ID string `json:"id"`
	} `json:"actor"`
	Scope           map[string]string `json:"scope"`
	Payload         json.RawMessage   `json:"payload"`
	InvocationToken string            `json:"invocation_token"`
}

// waitDispatch reads frames of g until a dispatch of typ for which match holds (nil: any)
// arrives, bounded by testkit.Patience, and returns it with its frame.
func (c *cluster) waitDispatch(g *harness.GatewayConn, typ string, match func(dispatch) bool) (dispatch, harness.Frame) {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	for {
		f, err := g.Next(ctx)
		if err != nil {
			c.t.Fatalf("waiting for %s: %v", typ, err)
		}
		if f.Op != "dispatch" {
			continue
		}
		var d dispatch
		if json.Unmarshal(f.D, &d) != nil || d.Type != typ {
			continue
		}
		if match == nil || match(d) {
			return d, f
		}
	}
}

// waitClose reads frames of g until it ends and returns the close code of the server (0 for
// another end), bounded by testkit.Patience.
func (c *cluster) waitClose(g *harness.GatewayConn) int {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	for {
		if _, err := g.Next(ctx); err != nil {
			var ce *wstest.CloseError
			if errors.As(err, &ce) {
				return ce.Code
			}
			if ctx.Err() != nil {
				c.t.Fatal("the connection stayed open")
			}
			return 0
		}
	}
}

// focus sends focus with threads and idle on g and waits until gateway_presence holds the row of
// person with exactly that focus and idle flag (the flush of its node).
func (c *cluster) focus(g *harness.GatewayConn, person string, idle bool, threads ...string) {
	c.t.Helper()
	if threads == nil {
		threads = []string{}
	}
	if err := g.Send("focus", map[string]any{"thread_ids": threads, "idle": idle}); err != nil {
		c.t.Fatal(err)
	}
	want := make([]ids.ID, 0, len(threads))
	for _, th := range threads {
		want = append(want, ids.MustParse(th))
	}
	testkit.Until(c.t, "the presence row of "+person, func() bool {
		return c.count(`SELECT count(*) FROM gateway_presence WHERE principal_id = $1 AND idle = $2
			AND focus_thread_ids @> $3::uuid[] AND focus_thread_ids <@ $3::uuid[]`, person, idle, want) == 1
	})
}

// newThread opens a thread in the text channel key ("Space/Channel") as cl through node a: a
// message in the main thread and a thread on it with a first message; it returns the thread.
func (c *cluster) newThread(cl *harness.Client, key, text string) string {
	c.t.Helper()
	var ch struct {
		Channel struct {
			MainThreadID string `json:"main_thread_id"`
		} `json:"channel"`
	}
	c.must(nodeA, c.w.Owner, http.MethodGet, "/channels/"+c.w.Channels[key], nil, http.StatusOK).JSON(c.t, &ch)
	var parent struct {
		ID string `json:"id"`
	}
	c.must(nodeA, cl, http.MethodPost, "/threads/"+ch.Channel.MainThreadID+"/messages",
		map[string]any{"client_msg_id": ids.New().String(), "content_md": "Question: " + text}, http.StatusCreated).JSON(c.t, &parent)
	var body struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	c.must(nodeA, cl, http.MethodPost, "/channels/"+c.w.Channels[key]+"/threads", map[string]any{"title": text, "parent_message_id": parent.ID,
		"first_message": map[string]any{"client_msg_id": ids.New().String(), "content_md": text}}, http.StatusCreated).JSON(c.t, &body)
	return body.Thread.ID
}

// post writes text as cl into thread through node.
func (c *cluster) post(node string, cl *harness.Client, thread, text string) {
	c.t.Helper()
	c.must(node, cl, http.MethodPost, "/threads/"+thread+"/messages", map[string]any{"client_msg_id": ids.New().String(), "content_md": text},
		http.StatusCreated)
}
