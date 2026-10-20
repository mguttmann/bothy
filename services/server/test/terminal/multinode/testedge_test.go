// SPDX-License-Identifier: Apache-2.0

package multinode

import (
	"context"
	"hash/fnv"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	gosync "sync"
	"testing"
)

// edgeMode is how the test edge routes the two stream paths of a session.
type edgeMode int

// The modes of the test edge (Design note 13 of M2-47).
const (
	// modeHash routes both stream paths of a session by rendezvous hashing over its ID to one
	// node, like the example edge (16 4.4).
	modeHash edgeMode = iota
	// modeSplit routes device streams to node A and host data connections to node B: an edge
	// without the hash rule.
	modeSplit
)

// The node names of the edge.
const (
	nodeA = "A"
	nodeB = "B"
)

// streamPath is the single regular expression of the example edge for both stream paths.
var streamPath = regexp.MustCompile(`^/api/v1/(terminal|hosts)/sessions/([^/]+)/stream$`)

// testEdge is the in-process edge in front of the nodes: an httputil.ReverseProxy on 127.0.0.1:0
// that passes WebSocket upgrades. Every path besides the stream paths goes to the node the test
// chooses (rest). It is safe for concurrent use.
type testEdge struct {
	URL string

	mu      gosync.Mutex
	targets map[string]*url.URL
	mode    edgeMode
	rest    string
	srv     *http.Server
}

// newTestEdge starts the edge; it stops in t.Cleanup.
func newTestEdge(t *testing.T) *testEdge {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	e := &testEdge{URL: "http://" + ln.Addr().String(), targets: map[string]*url.URL{}, rest: nodeA}
	proxy := &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
		r.SetURL(e.target(r.In.URL.Path, r.In.Header.Get(nodeHeader)))
		r.Out.Header.Del(nodeHeader)
		r.Out.Host = r.In.Host
		r.SetXForwarded()
	}}
	e.srv = &http.Server{Handler: proxy, ReadHeaderTimeout: waitLong}
	go func() { _ = e.srv.Serve(ln) }()
	t.Cleanup(func() { _ = e.srv.Close() })
	return e
}

// setTarget names the origin of a node.
func (e *testEdge) setTarget(node, origin string) {
	u, err := url.Parse(origin)
	if err != nil {
		panic(err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.targets[node] = u
}

// setMode switches the routing of the stream paths.
func (e *testEdge) setMode(m edgeMode) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.mode = m
}

// setRest chooses the node of every other path.
func (e *testEdge) setRest(node string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rest = node
}

// nodeHeader names, on a REST request, the node it goes to; without it the edge takes rest.
const nodeHeader = "X-Test-Node"

// target returns the origin of the node path goes to; pick is the node a REST request names.
func (e *testEdge) target(path, pick string) *url.URL {
	e.mu.Lock()
	defer e.mu.Unlock()
	node := e.rest
	if pick != "" {
		node = pick
	}
	if m := streamPath.FindStringSubmatch(path); m != nil {
		switch {
		case e.mode == modeSplit && m[1] == "terminal":
			node = nodeA
		case e.mode == modeSplit:
			node = nodeB
		default:
			node = hashNode(m[2])
		}
	}
	if t := e.targets[node]; t != nil {
		return t
	}
	// One node (cluster.mode single): everything goes to A.
	return e.targets[nodeA]
}

// hashNode is the rendezvous hash of the session ID over both nodes.
func hashNode(session string) string {
	best, bestScore := "", uint64(0)
	for _, n := range []string{nodeA, nodeB} {
		h := fnv.New64a()
		_, _ = h.Write([]byte(n + "/" + session))
		if s := h.Sum64(); best == "" || s > bestScore {
			best, bestScore = n, s
		}
	}
	return best
}
