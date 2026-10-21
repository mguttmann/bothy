// SPDX-License-Identifier: Apache-2.0

package actingfor_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// offsetClock is the system clock moved forward by an offset the test sets: requests keep real
// DPoP proofs, jobs and timers keep running, and the server sees an instant after a token's exp.
type offsetClock struct {
	mu  sync.Mutex
	off time.Duration
}

// Now returns the system time plus the offset in UTC.
func (c *offsetClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Now().UTC().Add(c.off)
}

// Advance moves the clock forward by d.
func (c *offsetClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.off += d
}

// nodeHeader names, on a request to the edge, the node it goes to; without it the edge takes
// node a.
const nodeHeader = "X-Test-Node"

// edge is a reverse proxy in front of two api nodes of one installation (the pattern of the
// multi-node tests of M2-47): both nodes share server.public_url, so one client reaches either.
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
		r.SetURL(e.target(r.In.Header.Get(nodeHeader)))
		r.Out.Header.Del(nodeHeader)
		r.Out.Host = r.In.Host
		r.SetXForwarded()
	}}
	srv := &http.Server{Handler: proxy, ReadHeaderTimeout: waitLong}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return e
}

// set names the origin of node.
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
		node = "a"
	}
	return e.targets[node]
}

// on sends one request as c to node through the edge with the Invocation-Token values tokens.
func (e *env) on(node string, c *harness.Client, method, path string, tokens ...string) harness.Response {
	e.t.Helper()
	h := invocation(tokens...)
	h.Set(nodeHeader, node)
	return c.Do(e.t.Context(), harness.Request{Method: method, Path: path, Header: h})
}

// TestSR9_ExpiredTokenAndRevocation runs the cases of brief M2-14 that need the clock or a second
// node on one installation: node a (role all) with the offset clock and node b (role api) on the
// same database behind an edge, cluster.mode multi. A short agent.invocation_token_ttl and
// auth.clock_leeway let the clock pass exp plus leeway with a small offset; a long
// auth.dpop_proof_max_age keeps the real proofs of the clients valid on the moved clock.
func TestSR9_ExpiredTokenAndRevocation(t *testing.T) {
	t.Parallel()
	clk := &offsetClock{}
	ed := newEdge(t)
	cfg := envConfig(map[string]string{"cluster.mode": "multi", "agent.invocation_token_ttl": "5s", "auth.clock_leeway": "1s",
		"auth.dpop_proof_max_age": "10m", "agent.cancel_grace": "30m"})
	logs := &syncBuffer{}
	s := harness.Start(t, harness.Options{Config: cfg, EgressDenyAll: true, LogWriter: logs, Clock: clk, PublicURL: ed.URL, OnListen: ed.set("a")})
	harness.StartNode(t, s, harness.Options{Config: cfg, EgressDenyAll: true, LogWriter: logs, Role: wire.RoleAPI, PublicURL: ed.URL,
		OnListen: ed.set("b")})
	e := attachEnv(t, s, harness.BuildMatrixWorld(t, s), logs)
	main := e.startedRun(e.alex, e.lotse, "Support/Allgemein", "Lauf E")
	post := e.w.Threads["Engineering/Projekt-A"]
	postMsg := e.text(`SELECT id::text FROM messages WHERE thread_id = $1 AND kind <> 'run' ORDER BY created_at, id LIMIT 1`, post)

	t.Run("TestSR9_ExpiredTokenStillBinds", func(t *testing.T) {
		e := e.in(t)
		// A second, queued run whose token expires last: its refused start proves that the server
		// reads the moved clock for the expiry of invocation tokens.
		probe := e.newRun(e.alex, e.lotse, "Support/Allgemein", "Lauf X")
		leeway, err := time.ParseDuration(e.s.ConfigValue("auth.clock_leeway"))
		if err != nil {
			t.Fatal(err)
		}
		exp := func(tok string) time.Time {
			f, _ := claims(t, tok)["exp"].(float64)
			return time.Unix(int64(f), 0)
		}
		latest := exp(main.token)
		if p := exp(probe.token); p.After(latest) {
			latest = p
		}
		if d := latest.Add(leeway).Add(2 * time.Second).Sub(clk.Now()); d > 0 {
			clk.Advance(d)
		}
		e.wantProblem(e.do(e.lotse, http.MethodPost, "/runs/"+probe.id+"/start", startBody(probe.token)), http.StatusForbidden,
			"invocation_token_invalid", "expired", "startRun with an expired token")
		e.wantProblem(e.do(e.lotse, http.MethodGet, "/messages/"+postMsg, nil, main.token), http.StatusNotFound, "not_found", "",
			"getMessage of the post with the expired T")
		e.ok(e.do(e.lotse, http.MethodGet, "/messages/"+main.first, nil, main.token), http.StatusOK, "getMessage in the own thread with the expired T")
	})

	t.Run("TestSR9_RevocationImmediate", func(t *testing.T) {
		e := e.in(t)
		msg := e.w.Messages["Support/Allgemein"][0]
		for _, node := range []string{"a", "b"} {
			e.ok(e.on(node, e.lotse, http.MethodGet, "/messages/"+msg, main.token), http.StatusOK, "warm getMessage on node "+node)
		}
		e.revoke(e.w.Grants["G-1"])
		for _, node := range []string{"a", "b"} {
			got := e.on(node, e.lotse, http.MethodGet, "/messages/"+msg, main.token)
			e.wantProblem(got, http.StatusNotFound, "not_found", "", "getMessage on node "+node+" right after the revocation")
			e.sameAnswer(got, e.on(node, e.lotse, http.MethodGet, "/messages/"+ids.New().String(), main.token), "node "+node+" after the revocation")
		}
	})

	t.Run("TestSR9_TokenNeverLogged", func(t *testing.T) { e.in(t).noTokenLeak(e.secrets()) })
	t.Run("Contract_NoNullInSuccessBodies", func(t *testing.T) { e.in(t).noNullInSuccess() })
}
