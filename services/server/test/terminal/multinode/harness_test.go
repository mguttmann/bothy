// SPDX-License-Identifier: Apache-2.0

package multinode

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/hosttest"
	terminalapp "github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// waitLong bounds every wait of a test (testkit.Patience, docs/DECISIONS.md PL/8).
const waitLong = testkit.Patience

// The short windows of the cluster: the tests wait at most a small multiple of them; exact
// boundaries are proven by the unit tests of domain (Design note 13 of M2-47).
const (
	heartbeatInterval = time.Second
	heartbeatMisses   = 3
	probeTimeout      = time.Second
	presenceWindow    = heartbeatInterval * heartbeatMisses
)

// cek is a well-formed client_ephemeral_key_hash.
var cek = base64.RawURLEncoding.EncodeToString(make([]byte, 32))

// clusterSlots bounds the clusters that run at once: each runs two servers with their own pools
// on the shared pgtest server.
var clusterSlots = make(chan struct{}, 2)

// syncBuffer is a bytes.Buffer safe for the concurrent writes of a server's log.
type syncBuffer struct {
	mu gosync.Mutex
	b  bytes.Buffer
}

// Write implements io.Writer.
func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

// String returns the text written so far.
func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// records returns the log records with message msg.
func (s *syncBuffer) records(msg string) []map[string]any {
	var out []map[string]any
	for _, line := range strings.Split(s.String(), "\n") {
		var m map[string]any
		if json.Unmarshal([]byte(line), &m) == nil && m["msg"] == msg {
			out = append(out, m)
		}
	}
	return out
}

// countingMetrics counts bothy.terminal.relay.unpaired of one node by side.
type countingMetrics struct {
	mu gosync.Mutex
	n  map[domain.Side]int
}

// RelayUnpaired implements terminalapp.Metrics.
func (m *countingMetrics) RelayUnpaired(_ context.Context, side domain.Side) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.n == nil {
		m.n = map[domain.Side]int{}
	}
	m.n[side]++
}

// of returns the count of side.
func (m *countingMetrics) of(side domain.Side) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.n[side]
}

// switchSignals wraps the signals of a node: mute drops every signal of the node (a crashed node
// sends nothing); failAfterStage fails the unit of work after the signal was staged.
type switchSignals struct {
	inner          terminalapp.Signals
	mu             gosync.Mutex
	mute           bool
	failAfterStage bool
}

// Stage implements terminalapp.Signals.
func (s *switchSignals) Stage(ctx context.Context, sig domain.Signal) error {
	s.mu.Lock()
	mute, fail := s.mute, s.failAfterStage
	s.mu.Unlock()
	if mute {
		return nil
	}
	if err := s.inner.Stage(ctx, sig); err != nil {
		return err
	}
	if fail {
		return errors.New("multinode: the unit fails after the signal was staged")
	}
	return nil
}

// Committed implements terminalapp.Signals.
func (s *switchSignals) Committed(ctx context.Context, sig domain.Signal) {
	s.inner.Committed(ctx, sig)
}

// Send implements terminalapp.Signals.
func (s *switchSignals) Send(ctx context.Context, sig domain.Signal) error {
	s.mu.Lock()
	mute := s.mute
	s.mu.Unlock()
	if mute {
		return nil
	}
	return s.inner.Send(ctx, sig)
}

// set changes the switches.
func (s *switchSignals) set(mute, failAfterStage bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mute, s.failAfterStage = mute, failAfterStage
}

// listenGate blocks the dial of a node's LISTEN connection while it is closed.
type listenGate struct {
	mu   gosync.Mutex
	open chan struct{}
}

// newListenGate returns an open gate.
func newListenGate() *listenGate {
	g := &listenGate{open: make(chan struct{})}
	close(g.open)
	return g
}

// wait implements the gate of wire.Deps.TerminalListenerGate.
func (g *listenGate) wait(ctx context.Context) error {
	g.mu.Lock()
	ch := g.open
	g.mu.Unlock()
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// close blocks the next dials; release lets them pass.
func (g *listenGate) close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.open = make(chan struct{})
}

func (g *listenGate) release() {
	g.mu.Lock()
	defer g.mu.Unlock()
	select {
	case <-g.open:
	default:
		close(g.open)
	}
}

// node is one server process of the cluster with its seams.
type node struct {
	name    string
	srv     *harness.Server
	timers  *fakehost.Timers
	logs    *syncBuffer
	metrics *countingMetrics
	signals *switchSignals
	gate    *listenGate
}

// options returns the harness options of the node behind edge with conf.
func (n *node) options(edge *testEdge, conf map[string]string, role wire.Role) harness.Options {
	return harness.Options{Config: conf, PublicURL: edge.URL, Role: role, EgressDenyAll: true, LogWriter: n.logs, TerminalTimers: n.timers,
		TerminalMetrics: n.metrics, TerminalListenerGate: n.gate.wait,
		TerminalSignals: func(s terminalapp.Signals) terminalapp.Signals {
			n.signals.inner = s
			return n.signals
		},
		OnListen: func(u string) {
			if n.name != "" {
				edge.setTarget(n.name, u)
			}
		}}
}

// newNode returns a node named name with fresh seams.
func newNode(name string) *node {
	return &node{name: name, timers: &fakehost.Timers{}, logs: &syncBuffer{}, metrics: &countingMetrics{}, signals: &switchSignals{},
		gate: newListenGate()}
}

// cluster is the world of the tests (Design note 13): nodes A and B behind the test edge in role
// api with cluster.mode multi; Robin is the owner; Alex is the assignee of the host lab-1 (active,
// profile shell) with a grant terminal and the session user bothy-s-alex, which the Fake-Host
// reports.
type cluster struct {
	t      *testing.T
	edge   *testEdge
	nodes  map[string]*node
	conf   map[string]string
	robin  *harness.Client
	alex   *harness.Client
	alex2  *harness.Client
	host   ids.ID
	key    hosttest.Key
	listen *ephemeralRecorder
}

// newCluster starts the cluster with cluster.mode mode ("multi" or "single"; single starts only
// node A).
func newCluster(t *testing.T, mode string) *cluster {
	t.Helper()
	return newClusterWith(t, mode, nil)
}

// newClusterWith is newCluster with the configuration overrides extra.
func newClusterWith(t *testing.T, mode string, extra map[string]string) *cluster {
	t.Helper()
	clusterSlots <- struct{}{}
	t.Cleanup(func() { <-clusterSlots })
	conf := harness.RaisedRateLimits()
	for k, v := range map[string]string{
		"audit.deny_aggregate_window": "50ms", "cluster.mode": mode,
		"terminal.host_heartbeat_interval": heartbeatInterval.String(), "terminal.host_heartbeat_miss_limit": fmt.Sprint(heartbeatMisses),
		"terminal.host_duplicate_probe_timeout": probeTimeout.String(), "database.listen_reconnect_backoff_cap": "200ms",
		// The tests open sessions in loops while they wait for presence; the session limits of
		// M2-46 are not what they prove.
		"terminal.max_sessions_per_person": "1000", "terminal.max_sessions_per_host": "1000",
	} {
		conf[k] = v
	}
	for k, v := range extra {
		conf[k] = v
	}
	c := &cluster{t: t, edge: newTestEdge(t), nodes: map[string]*node{nodeA: newNode(nodeA)}, conf: conf}
	a := c.nodes[nodeA]
	a.srv = harness.Start(t, a.options(c.edge, conf, wire.RoleAPI))
	if mode == "multi" {
		b := newNode(nodeB)
		c.nodes[nodeB] = b
		b.srv = harness.StartNode(t, a.srv, b.options(c.edge, conf, wire.RoleAPI))
	}
	c.listen = newEphemeralRecorder(t, a.srv)
	c.robin = a.srv.Owner
	c.alex = a.srv.NewUser(t, harness.UserSpec{Handle: "alex", DisplayName: "Alex"})
	var err error
	if c.alex2, err = a.srv.UserSession(t.Context(), c.alex.PrincipalID); err != nil {
		t.Fatal(err)
	}
	c.key, c.host = c.pairedHost("lab-1", c.alex.PrincipalID)
	c.grant(c.alex.PrincipalID, c.host)
	return c
}

// worker starts instance C in role worker on the database of A (the worker case) and returns it
// with its terminal use cases, through which a test commits an end in the worker.
func (c *cluster) worker() (*node, *terminalapp.Service) {
	c.t.Helper()
	w := newNode("")
	opts := w.options(c.edge, c.conf, wire.RoleWorker)
	var svc *terminalapp.Service
	opts.TerminalModule = func(s *terminalapp.Service) { svc = s }
	w.srv = harness.StartNode(c.t, c.nodes[nodeA].srv, opts)
	return w, svc
}

// n returns the node named name.
func (c *cluster) n(name string) *node { return c.nodes[name] }

// ctx is a context bounded by waitLong.
func (c *cluster) ctx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), waitLong)
	c.t.Cleanup(cancel)
	return ctx
}

// req is a REST request to node through the edge.
func req(node, method, path string, body any) harness.Request {
	return harness.Request{Method: method, Path: path, Body: body, Header: http.Header{nodeHeader: []string{node}}}
}

// must sends a request as cl through the edge to node and fails unless the status is want.
func (c *cluster) must(node string, cl *harness.Client, method, path string, body any, want int) harness.Response {
	c.t.Helper()
	return harness.MustDo(c.t, cl, req(node, method, path, body), want)
}

// profiles are the profiles of lab-1.
func profiles() []any {
	return []any{map[string]any{"name": "shell", "display_name": "Shell", "kind": "shell"}}
}

// reportedUsers are the session users lab-1 reports.
func reportedUsers() []any {
	return []any{map[string]any{"os_user": "bothy-s-alex", "workdirs": []string{}}}
}

// report is a report of lab-1 that passes, without local confirmation.
func report() map[string]any {
	var doctor []any
	for _, ch := range []string{"hd1_user", "hd2_groups", "hd3_sudo", "hd4_state_dir", "hd5_server_host", "hd6_llm_login",
		"hd7_profiles", "hd8_separation", "hd9_foreign_homes", "hd10_helper_integrity", "hd11_shared_locations"} {
		doctor = append(doctor, map[string]any{"check": ch, "ok": true})
	}
	return map[string]any{"profiles": profiles(), "session_users": reportedUsers(), "doctor": doctor, "is_server_host": false,
		"require_local_confirm": false, "os_version": "Debian 13"}
}

// pairedHost creates, pairs and confirms a host named name with the holder and Alex's session
// user; it returns the login key and the host id.
func (c *cluster) pairedHost(name, holder string) (hosttest.Key, ids.ID) {
	c.t.Helper()
	var created struct {
		HostID      string `json:"host_id"`
		PairingCode string `json:"pairing_code"`
	}
	c.must(nodeA, c.robin, http.MethodPost, "/hosts", map[string]any{"name": name, "reason": "terminal test", "assigned_user_id": holder},
		http.StatusCreated).JSON(c.t, &created)
	key, noise := fakehost.NewKey()
	var jwk map[string]any
	if err := json.Unmarshal(key.JWK(), &jwk); err != nil {
		c.t.Fatal(err)
	}
	var paired struct {
		KeyFingerprint string `json:"key_fingerprint"`
	}
	c.must(nodeA, c.robin, http.MethodPost, "/hosts/pair", map[string]any{"pairing_code": created.PairingCode, "noise_public_key": fakehost.B64(noise),
		"auth_public_jwk": jwk, "platform": "linux", "host_version": "1.0.0", "profiles": profiles(), "session_users": reportedUsers()},
		http.StatusCreated).JSON(c.t, &paired)
	get := c.must(nodeA, c.robin, http.MethodGet, "/hosts/"+created.HostID, nil, http.StatusOK)
	resp, err := harness.DoWithStepUp(c.t.Context(), c.robin, harness.Request{Method: http.MethodPost, Path: "/hosts/" + created.HostID + "/confirm",
		Header: http.Header{"If-Match": []string{get.Header.Get("ETag")}},
		Body: map[string]any{"fingerprint_prefix": strings.ReplaceAll(paired.KeyFingerprint, "-", "")[:8], "llm_credentials": "organization",
			"declared_server_host": false, "session_users": []any{map[string]any{"user_id": holder, "os_user": "bothy-s-alex"}},
			"reason": "terminal test"}})
	if err != nil || resp.Status != http.StatusOK {
		c.t.Fatalf("confirmHost: %v %d %s", err, resp.Status, resp.Body)
	}
	return key, ids.MustParse(created.HostID)
}

// grant gives subject the capability terminal on host.
func (c *cluster) grant(subject string, host ids.ID) {
	c.t.Helper()
	c.must(nodeA, c.robin, http.MethodPost, "/grants", map[string]any{"subject_kind": "user", "subject_id": subject, "target_kind": "host",
		"target_id": host.String(), "capabilities": []string{"terminal"}, "reason": "terminal test"}, http.StatusCreated)
}

// connect connects the Fake-Host of lab-1 through the edge, its control connection on node on.
func (c *cluster) connect(on string) *fakehost.Host {
	c.t.Helper()
	return c.connectHost(on, c.host, c.key)
}

// connectHost is connect for host. Connections are not REST, so the edge takes its default node:
// the call holds the edge lock of the default while it connects.
func (c *cluster) connectHost(on string, host ids.ID, key hosttest.Key) *fakehost.Host {
	c.t.Helper()
	connectMu.Lock()
	defer connectMu.Unlock()
	c.edge.setRest(on)
	defer c.edge.setRest(nodeA)
	h := fakehost.Connect(c.t, c.edge.URL, host, key, report())
	h.Default(fakehost.Script{Kind: fakehost.Accept})
	return h
}

// connectMu serializes the connects of one test process (they switch the default of an edge).
var connectMu gosync.Mutex

// grantBody is the decoded TerminalSessionGrant.
type grantBody struct {
	Session struct {
		ID, State, DeviceID string
	}
	SessionToken string `json:"session_token"`
	StreamPath   string `json:"stream_path"`
}

// openBody is the body of openTerminalSession of a shell.
func openBody() map[string]any {
	return map[string]any{"profile": "shell", "cols": 120, "rows": 40, "client_ephemeral_key_hash": cek}
}

// open sends openTerminalSession of the host as cl through node on with a fresh step-up first.
func (c *cluster) open(on string, cl *harness.Client) harness.Response {
	c.t.Helper()
	return c.openOn(on, cl, c.host)
}

// openOn is open on host.
func (c *cluster) openOn(on string, cl *harness.Client, host ids.ID) harness.Response {
	c.t.Helper()
	if err := cl.StepUp(c.t.Context()); err != nil {
		c.t.Fatal(err)
	}
	return cl.Do(c.t.Context(), req(on, http.MethodPost, "/hosts/"+host.String()+"/terminal/sessions", openBody()))
}

// opened opens a session through node on and decodes the grant; it fails unless 201.
func (c *cluster) opened(on string, cl *harness.Client) grantBody {
	c.t.Helper()
	r := c.open(on, cl)
	if r.Status != http.StatusCreated {
		c.t.Fatalf("openTerminalSession through %s: %d %s", on, r.Status, r.Body)
	}
	var g grantBody
	r.JSON(c.t, &g)
	return g
}

// attach sends attachTerminalSession as cl through node on with a fresh step-up.
func (c *cluster) attach(on string, cl *harness.Client, session string) grantBody {
	c.t.Helper()
	if err := cl.StepUp(c.t.Context()); err != nil {
		c.t.Fatal(err)
	}
	r := cl.Do(c.t.Context(), req(on, http.MethodPost, "/terminal/sessions/"+session+"/attach",
		map[string]any{"cols": 120, "rows": 40, "client_ephemeral_key_hash": cek}))
	if r.Status != http.StatusOK && r.Status != http.StatusCreated {
		c.t.Fatalf("attachTerminalSession through %s: %d %s", on, r.Status, r.Body)
	}
	var g grantBody
	r.JSON(c.t, &g)
	return g
}

// device opens the device stream of session as cl through the edge.
func (c *cluster) device(cl *harness.Client, session string) *fakehost.Device {
	c.t.Helper()
	d, status, err := fakehost.DialDevice(c.t, c.ctx(), c.edge.URL, c.edge.URL, ids.MustParse(session), cl, fakehost.DeviceParams{DeviceID: cl.DeviceID})
	if err != nil {
		c.t.Fatalf("device stream: %d %v", status, err)
	}
	return d
}

// handshake runs message 1 and 2 and the first transport frame between d and the host side.
func (c *cluster) handshake(d *fakehost.Device, token string) {
	c.t.Helper()
	ctx := c.ctx()
	if err := d.Send(ctx, []byte(token)); err != nil {
		c.t.Fatal(err)
	}
	if m, err := d.Recv(ctx); err != nil || string(m) != "message-2" {
		c.t.Fatalf("message 2: %q %v", m, err)
	}
	if err := d.Send(ctx, []byte("start")); err != nil {
		c.t.Fatal(err)
	}
	if m, err := d.Recv(ctx); err != nil || string(m) != "start" {
		c.t.Fatalf("echo of start: %q %v", m, err)
	}
}

// count returns an integer result of query.
func (c *cluster) count(query string, args ...any) int {
	c.t.Helper()
	var n int
	if err := c.n(nodeA).srv.AdminDB().QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		c.t.Fatalf("%s: %v", query, err)
	}
	return n
}

// state returns state, close_reason and close_detail of a session.
func (c *cluster) state(session string) string {
	c.t.Helper()
	var s string
	err := c.n(nodeA).srv.AdminDB().QueryRow(context.Background(),
		`SELECT state || coalesce(':' || close_reason, '') || coalesce(':' || close_detail, '') FROM terminal_sessions WHERE id = $1`, session).Scan(&s)
	if err != nil {
		c.t.Fatal(err)
	}
	return s
}

// eventually waits until cond holds or waitLong passes.
func (c *cluster) eventually(what string, cond func() bool) {
	c.t.Helper()
	testkit.Until(c.t, what, cond)
}

// waitState waits until the session has the state string want.
func (c *cluster) waitState(session, want string) {
	c.t.Helper()
	c.eventually(fmt.Sprintf("session %s %s", session, want), func() bool { return c.state(session) == want })
}

// ready reports whether /readyz of every node answers 200.
func (c *cluster) ready() bool {
	for _, n := range c.nodes {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, n.srv.InternalURL+"/readyz", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return false
		}
	}
	return true
}

// closeCode checks a close code and reason ("" for any reason).
func (c *cluster) closeCode(what string, got func() (int, string), code int, reason string) {
	c.t.Helper()
	gc, gr := got()
	if gc != code || reason != "" && gr != reason {
		c.t.Fatalf("%s: close %d %q, want %d %q", what, gc, gr, code, reason)
	}
}

// ephemeralRecorder is a test connection with its own LISTEN ephemeral: it records every payload.
type ephemeralRecorder struct {
	mu       gosync.Mutex
	payloads []string
}

// newEphemeralRecorder listens on the database of srv until the end of the test.
func newEphemeralRecorder(t *testing.T, srv *harness.Server) *ephemeralRecorder {
	t.Helper()
	r := &ephemeralRecorder{}
	conn, err := srv.AdminDB().Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(context.Background(), "LISTEN ephemeral"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.run(ctx, conn.Conn())
	}()
	t.Cleanup(func() {
		cancel()
		<-done
		conn.Release()
	})
	return r
}

// run records the notifications until ctx ends.
func (r *ephemeralRecorder) run(ctx context.Context, conn *pgx.Conn) {
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return
		}
		r.mu.Lock()
		r.payloads = append(r.payloads, n.Payload)
		r.mu.Unlock()
	}
}

// all returns the payloads so far.
func (r *ephemeralRecorder) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.payloads...)
}

// kinds returns the kinds of the recorded terminal signals.
func (r *ephemeralRecorder) kinds() []domain.SignalKind {
	var out []domain.SignalKind
	for _, p := range r.all() {
		if s, ours, err := domain.DecodeSignal([]byte(p)); err == nil && ours {
			out = append(out, s.Kind)
		}
	}
	return out
}
