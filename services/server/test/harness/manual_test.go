// SPDX-License-Identifier: Apache-2.0

package harness

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// switchProxy is a reverse proxy on 127.0.0.1 port 0 whose upstream a test switches: the stable
// address a server keeps across Restart.
type switchProxy struct {
	ln  net.Listener
	srv *http.Server
	mu  sync.Mutex
	up  *url.URL
}

// newSwitchProxy starts the proxy; OnListen of the server sets its upstream.
func newSwitchProxy(t *testing.T) *switchProxy {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &switchProxy{ln: ln}
	rp := &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
		p.mu.Lock()
		up := p.up
		p.mu.Unlock()
		r.SetURL(up)
		r.Out.Host = r.In.Host
	}}
	p.srv = &http.Server{Handler: rp, ReadHeaderTimeout: time.Minute}
	go func() { _ = p.srv.Serve(ln) }()
	t.Cleanup(func() { _ = p.srv.Close() })
	return p
}

func (p *switchProxy) origin() string { return "http://" + p.ln.Addr().String() }

func (p *switchProxy) set(upstream string) {
	u, _ := url.Parse(upstream)
	p.mu.Lock()
	p.up = u
	p.mu.Unlock()
}

// startBehindProxy starts a server on a fake clock behind a switch proxy with ManualJobs.
func startBehindProxy(t *testing.T, cfg map[string]string) (*Server, *clock.Fake) {
	t.Helper()
	p := newSwitchProxy(t)
	clk := clock.NewFake(time.Now().UTC().Truncate(time.Millisecond))
	conf := RaisedRateLimits()
	for k, v := range cfg {
		conf[k] = v
	}
	s := Start(t, Options{Config: conf, Clock: clk, ManualJobs: true, ProofsOnClock: true, PublicURL: p.origin(), OnListen: p.set})
	return s, clk
}

// count reads one number.
func count(t *testing.T, s *Server, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := s.AdminDB().QueryRow(t.Context(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// TestRunDueJobs_RunsOnlyDueJobsOfKind: queued_expiry runs nothing before its time, then every due
// job once through the real handler (the runs fail with agent_unavailable), then nothing again;
// River's own scheduling runs no real handler; other kinds stay untouched.
func TestRunDueJobs_RunsOnlyDueJobsOfKind(t *testing.T) {
	t.Parallel()
	s, clk := startBehindProxy(t, map[string]string{"run.queued_max_age": "1h"})
	BuildMatrixWorld(t, s)
	queued := count(t, s, `SELECT count(*) FROM runs WHERE state = 'queued'`)
	if queued == 0 {
		t.Fatal("the world opened no run")
	}
	if n := s.RunDueJobs(t, "runs.queued_expiry"); n != 0 {
		t.Fatalf("%d jobs ran before their time", n)
	}
	if n := s.RunDueJobs(t, "runs.cancel_timeout"); n != 0 {
		t.Fatalf("%d cancel jobs ran", n)
	}
	clk.Advance(time.Hour + time.Millisecond)
	if n := s.RunDueJobs(t, "runs.queued_expiry"); n != queued {
		t.Fatalf("%d of %d due jobs ran", n, queued)
	}
	if n := count(t, s, `SELECT count(*) FROM runs WHERE state = 'failed' AND state_reason = 'agent_unavailable'`); n != queued {
		t.Fatalf("%d runs expired", n)
	}
	if n := s.RunDueJobs(t, "runs.queued_expiry"); n != 0 {
		t.Fatalf("%d jobs ran twice", n)
	}
}

// TestRunDueJobs_PeriodicKindRunsOnce: a periodic kind runs its handler once per call.
func TestRunDueJobs_PeriodicKindRunsOnce(t *testing.T) {
	t.Parallel()
	s, _ := startBehindProxy(t, nil)
	for _, kind := range []string{"runs.stall_check", "instance.key_rotation"} {
		if n := s.RunDueJobs(t, kind); n != 1 {
			t.Fatalf("%s ran %d times", kind, n)
		}
	}
	if got := ManualJobKinds(); len(got) != 4 {
		t.Fatalf("manual kinds %v", got)
	}
}

// TestRestart_SameDatabaseNewListeners: after Restart the listener changes, the public URL stays,
// the data and the owner's session stay.
func TestRestart_SameDatabaseNewListeners(t *testing.T) {
	t.Parallel()
	s, _ := startBehindProxy(t, nil)
	space := MustDo(t, s.Owner, Request{Method: http.MethodPost, Path: "/spaces", Body: map[string]any{"name": "Support"}}, http.StatusCreated)
	before, public := s.ListenURL, s.PublicURL
	s.Restart(t, RestartOptions{})
	if s.ListenURL == before || s.PublicURL != public {
		t.Fatalf("listen %s -> %s, public %s -> %s", before, s.ListenURL, public, s.PublicURL)
	}
	var created struct {
		ID string `json:"id"`
	}
	space.JSON(t, &created)
	MustDo(t, s.Owner, Request{Method: http.MethodGet, Path: "/spaces/" + created.ID}, http.StatusOK)
}

// TestRestart_DPoPAgentRequestThroughProxy: an agent request with a DPoP proof for the proxy's
// URL is answered before and after Restart.
func TestRestart_DPoPAgentRequestThroughProxy(t *testing.T) {
	t.Parallel()
	s, _ := startBehindProxy(t, nil)
	agent := s.NewAgent(t, AgentSpec{Handle: "lotse", DisplayName: "Lotse"})
	MustDo(t, agent, Request{Method: http.MethodGet, Path: "/agents/me/consumers/default/events"}, http.StatusOK)
	s.Restart(t, RestartOptions{})
	MustDo(t, agent, Request{Method: http.MethodGet, Path: "/agents/me/consumers/default/events"}, http.StatusOK)
}

// TestRestart_RestoreCursors: an acknowledgement stored after the snapshot is lost by a Restart
// that restores the snapshot; one stored before it stays.
func TestRestart_RestoreCursors(t *testing.T) {
	t.Parallel()
	s, _ := startBehindProxy(t, nil)
	agent := s.NewAgent(t, AgentSpec{Handle: "lotse", DisplayName: "Lotse"})
	poll := Request{Method: http.MethodGet, Path: "/agents/me/consumers/default/events"}
	MustDo(t, agent, poll, http.StatusOK)
	acked := func() int64 {
		var n int64
		if err := s.AdminDB().QueryRow(t.Context(), `SELECT acked_seq FROM agent_consumer_cursors WHERE agent_id = $1`, agent.PrincipalID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	head := func() int64 {
		return int64(count(t, s, `SELECT coalesce(max(seq), 0)::int FROM event_log`))
	}
	// Events to acknowledge: any write raises head_seq; an acknowledgement may name any seq up to it.
	MustDo(t, s.Owner, Request{Method: http.MethodPost, Path: "/spaces", Body: map[string]any{"name": "One"}}, http.StatusCreated)
	first := head()
	ack := func(seq int64) {
		MustDo(t, agent, Request{Method: http.MethodPost, Path: "/agents/me/consumers/default/ack", Body: map[string]any{"seq": seq}}, http.StatusOK)
	}
	waitHead(t, agent, first)
	ack(first)
	snap := s.SnapshotCursors(t)
	if snap.Len() != 1 {
		t.Fatalf("snapshot of %d cursors", snap.Len())
	}
	MustDo(t, s.Owner, Request{Method: http.MethodPost, Path: "/spaces", Body: map[string]any{"name": "Two"}}, http.StatusCreated)
	second := head()
	waitHead(t, agent, second)
	ack(second)
	if acked() != second {
		t.Fatalf("acked %d before the restart, want %d", acked(), second)
	}
	s.Restart(t, RestartOptions{RestoreCursors: &snap})
	if got := acked(); got != first {
		t.Fatalf("acked %d after the restart, want %d (the snapshot)", got, first)
	}
}

// waitHead waits until the consumer state of agent names head at least seq (the head of the
// serving process follows the log asynchronously).
func waitHead(t *testing.T, agent *Client, seq int64) {
	t.Helper()
	testkit.Until(t, fmt.Sprintf("head_seq %d", seq), func() bool {
		var st struct {
			HeadSeq int64 `json:"head_seq"`
		}
		MustDo(t, agent, Request{Method: http.MethodGet, Path: "/agents/me/consumers/default"}, http.StatusOK).JSON(t, &st)
		return st.HeadSeq >= seq
	})
}

// fatalTB records Fatal instead of ending the test.
type fatalTB struct {
	testing.TB
	mu  sync.Mutex
	msg string
}

func (f *fatalTB) Helper() {}

func (f *fatalTB) Fatal(args ...any) {
	f.mu.Lock()
	f.msg = fmt.Sprint(args...)
	f.mu.Unlock()
	panic(f)
}

func (f *fatalTB) Fatalf(format string, args ...any) { f.Fatal(fmt.Sprintf(format, args...)) }

// failsWith runs fn with a recording TB and returns its fatal message.
func failsWith(t *testing.T, fn func(tb testing.TB)) string {
	t.Helper()
	rec := &fatalTB{TB: t}
	func() {
		defer func() {
			if r := recover(); r != nil && r != any(rec) {
				panic(r)
			}
		}()
		fn(rec)
	}()
	return rec.msg
}

// TestAgentCredentials_OnlyForAgents: an agent client hands out its token and key, never in its
// text form; a person's client has none and fails the test.
func TestAgentCredentials_OnlyForAgents(t *testing.T) {
	t.Parallel()
	s := Start(t, Options{EgressDenyAll: true, Config: RaisedRateLimits()})
	agent := s.NewAgent(t, AgentSpec{Handle: "lotse", DisplayName: "Lotse"})
	creds := agent.AgentCredentials()
	if creds.Token == "" || !strings.Contains(string(creds.DPoPPrivateJWK), `"d"`) {
		t.Fatal("the agent client has no credentials")
	}
	for _, text := range []string{fmt.Sprint(creds), fmt.Sprintf("%v %+v %#v %s", creds, creds, creds, creds)} {
		if strings.Contains(text, creds.Token) || strings.Contains(text, `"d"`) {
			t.Fatalf("the text form shows a credential: %s", text)
		}
	}
	person := s.NewUser(t, UserSpec{Handle: "alex", DisplayName: "Alex", Email: "alex@example.com"})
	msg := failsWith(t, func(tb testing.TB) {
		person.tb = tb
		person.AgentCredentials()
	})
	if !strings.Contains(msg, "without an API token") {
		t.Fatalf("a person's credentials: %q", msg)
	}
}

// TestProofsOnClock: with ProofsOnClock the proofs of every client take their iat from the
// harness clock, so a server whose fake clock moved far ahead accepts them.
func TestProofsOnClock(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var last string
	clk := clock.NewFake(time.Now().UTC())
	s := Start(t, Options{EgressDenyAll: true, Config: RaisedRateLimits(), Clock: clk, ProofsOnClock: true,
		ProofObserver: func(p string) { mu.Lock(); last = p; mu.Unlock() }})
	agent := s.NewAgent(t, AgentSpec{Handle: "lotse", DisplayName: "Lotse"})
	clk.Advance(10 * time.Minute)
	MustDo(t, agent, Request{Method: http.MethodGet, Path: "/me"}, http.StatusOK)
	mu.Lock()
	proof := last
	mu.Unlock()
	parts := strings.Split(proof, ".")
	if len(parts) != 3 {
		t.Fatal("no proof observed")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var claims struct {
		IAT int64 `json:"iat"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.IAT != clk.Now().Unix() {
		t.Fatalf("iat %d, harness clock %d", claims.IAT, clk.Now().Unix())
	}
}
