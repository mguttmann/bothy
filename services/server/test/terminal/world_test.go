// SPDX-License-Identifier: Apache-2.0

// The integration tests of M2-46 (terminal revocation, limits, audit and metrics) in package
// terminal_test: the full server of test/harness (the real internal/wire graph, PostgreSQL 18
// through pgtest, HTTP and WebSockets on 127.0.0.1) with the Fake-Host and the device client of
// test/terminal/fakehost, a frozen fake clock and the deadline scheduler fakehost.ClockTimers, so no
// test sleeps for a window. M2-48 turns them into its data-driven suite.
package terminal_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/hosttest"
	terminalapp "github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// waitLong bounds every wait of a test (a test constant, not a product limit).
const waitLong = 30 * time.Second

// The windows of the test configuration (test values, not the product defaults of 24 section 17).
const (
	idleTimeout = 10 * time.Minute
	idleGrace   = time.Minute
	detachGrace = 5 * time.Minute
	maxDuration = 2 * time.Hour
	openTimeout = 2 * time.Minute
	tick        = time.Millisecond
)

// cek is a well-formed client_ephemeral_key_hash (43 base64url characters of 32 bytes).
var cek = base64.RawURLEncoding.EncodeToString(make([]byte, 32))

// worldSlots bounds the servers that run at once: each has its own database and pool on the shared
// pgtest server.
var worldSlots = make(chan struct{}, 3)

// syncBuffer is a bytes.Buffer safe for the concurrent writes of the server's log.
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

// sessionMetrics counts the session instruments of M2-46 of one server.
type sessionMetrics struct {
	mu     gosync.Mutex
	closed map[domain.CloseReason]int
	active map[string]int64
	hosts  int64
}

// SessionClosed implements terminalapp.SessionMetrics.
func (m *sessionMetrics) SessionClosed(_ context.Context, r domain.CloseReason) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed[r]++
}

// SessionsActive implements terminalapp.SessionMetrics.
func (m *sessionMetrics) SessionsActive(_ context.Context, state string, delta int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active[state] += delta
}

// HostsConnected implements terminalapp.SessionMetrics.
func (m *sessionMetrics) HostsConnected(_ context.Context, delta int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hosts += delta
}

// closedOf returns the count of bothy.terminal.sessions.closed for r.
func (m *sessionMetrics) closedOf(r domain.CloseReason) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed[r]
}

// activeOf returns bothy.terminal.sessions.active of state.
func (m *sessionMetrics) activeOf(state string) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active[state]
}

// connected returns bothy.terminal.hosts.connected.
func (m *sessionMetrics) connected() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.hosts
}

// newSessionMetrics returns empty counters.
func newSessionMetrics() *sessionMetrics {
	return &sessionMetrics{closed: map[domain.CloseReason]int{}, active: map[string]int64{}}
}

// world is the world of the brief (M2-46 Tests, Integration): Robin is the owner, Alex and Sam are
// people; host r1 is organization, assigned to Alex, with a grant terminal to Alex and the reported
// and assigned session users term-alex, term-sam and term-robin; host r2 is organization, assigned to
// Sam, with the group Engineering (member Sam) holding a grant terminal on r2 and the session users
// term-sam and term-robin. h1 and h2 are the Fake-Hosts of r1 and r2. A row that needs a second owner
// or another host sets it up itself.
type world struct {
	t       *testing.T
	s       *harness.Server
	clk     *clock.Fake
	timers  *fakehost.ClockTimers
	metrics *sessionMetrics
	logs    *syncBuffer

	robin, alex, sam *harness.Client
	// alex2 is a second device and login session of Alex.
	alex2 *harness.Client

	r1, r2 ids.ID
	k1, k2 hosttest.Key
	h1, h2 *fakehost.Host

	// alexGrant is Alex's grant terminal on r1; engineering the group, engGrant its grant on r2.
	alexGrant, engineering, engGrant string

	svcMu gosync.Mutex
	svc   *terminalapp.Service
	jobs  *wire.JobModules
}

// worldOptions change a world.
type worldOptions struct {
	// cfg overrides configuration keys.
	cfg map[string]string
	// role is the role of the server (wire.RoleAll when empty).
	role wire.Role
}

// testConfig is the configuration of every world: raised rate limits, the windows of the tests, a
// wide auth.clock_leeway (the fake clock jumps ahead of the clients' real clock) and periods of the
// jobs long enough that only the tests run them.
func testConfig(over map[string]string) map[string]string {
	conf := harness.RaisedRateLimits()
	for k, v := range map[string]string{
		"audit.deny_aggregate_window":   "50ms",
		"auth.clock_leeway":             "48h",
		"terminal.sweep_interval":       "24h",
		"authz.expiry_sweep_interval":   "24h",
		"terminal.open_timeout":         seconds(openTimeout),
		"terminal.idle_timeout":         seconds(idleTimeout),
		"terminal.idle_grace":           seconds(idleGrace),
		"terminal.detach_grace":         seconds(detachGrace),
		"terminal.max_session_duration": seconds(maxDuration),
	} {
		conf[k] = v
	}
	for k, v := range over {
		conf[k] = v
	}
	return conf
}

// seconds formats d as a configuration value in whole seconds.
func seconds(d time.Duration) string { return fmt.Sprintf("%ds", int64(d/time.Second)) }

// newWorld builds the world with the options o.
func newWorld(t *testing.T, o worldOptions) *world {
	t.Helper()
	worldSlots <- struct{}{}
	t.Cleanup(func() { <-worldSlots })
	clk := clock.NewFake(time.Now().UTC().Truncate(time.Second))
	w := &world{t: t, clk: clk, timers: fakehost.NewClockTimers(clk), metrics: newSessionMetrics(), logs: &syncBuffer{}}
	w.s = harness.Start(t, harness.Options{Config: testConfig(o.cfg), Clock: clk, TerminalTimers: w.timers, EgressDenyAll: true,
		LogWriter: w.logs, Role: o.role, TerminalSessionMetrics: w.metrics,
		TerminalModule: func(s *terminalapp.Service) { w.svcMu.Lock(); w.svc = s; w.svcMu.Unlock() },
		JobModules:     func(m *wire.JobModules) { w.svcMu.Lock(); w.jobs = m; w.svcMu.Unlock() }})
	w.robin = w.s.Owner
	w.alex = w.s.NewUser(t, harness.UserSpec{Handle: "alex", DisplayName: "Alex"})
	w.sam = w.s.NewUser(t, harness.UserSpec{Handle: "sam", DisplayName: "Sam"})
	var err error
	if w.alex2, err = w.s.UserSession(t.Context(), w.alex.PrincipalID); err != nil {
		t.Fatal(err)
	}
	w.k1, w.r1 = w.pairedHost("r1", w.alex.PrincipalID, r1Users(), map[string]string{
		w.alex.PrincipalID: "term-alex", w.sam.PrincipalID: "term-sam", w.robin.PrincipalID: "term-robin"})
	w.k2, w.r2 = w.pairedHost("r2", w.sam.PrincipalID, r2Users(), map[string]string{
		w.sam.PrincipalID: "term-sam", w.robin.PrincipalID: "term-robin"})
	w.alexGrant = w.grant("user", w.alex.PrincipalID, w.r1.String(), "")
	w.engineering = w.group("Engineering", w.sam.PrincipalID, "")
	w.engGrant = w.grant("group", w.engineering, w.r2.String(), "")
	w.h1 = w.connect(w.r1, w.k1, r1Users(), false)
	w.h2 = w.connect(w.r2, w.k2, r2Users(), false)
	return w
}

// service returns the use cases of the terminal module of the server.
func (w *world) service() *terminalapp.Service {
	w.svcMu.Lock()
	defer w.svcMu.Unlock()
	return w.svc
}

// jobModules returns the use cases of the jobs of the server.
func (w *world) jobModules() *wire.JobModules {
	w.svcMu.Lock()
	defer w.svcMu.Unlock()
	if w.jobs == nil {
		w.t.Fatal("the jobs of the server are not built")
	}
	return w.jobs
}

// sweep runs the handler of terminal.sweep once.
func (w *world) sweep() {
	w.t.Helper()
	if _, err := w.jobModules().TerminalSweep.Sweep(w.ctx()); err != nil {
		w.t.Fatalf("terminal.sweep: %v", err)
	}
}

// expirySweep runs the handler of authz.expiry_sweep once.
func (w *world) expirySweep() {
	w.t.Helper()
	if _, err := w.jobModules().Expiry.Sweep(w.ctx()); err != nil {
		w.t.Fatalf("authz.expiry_sweep: %v", err)
	}
}

// syncClock moves the frozen fake clock forward to the real time when it lags behind (before a
// phase of requests: the clients sign with the real clock), never backwards, and runs what became
// due.
func (w *world) syncClock() {
	if d := time.Now().UTC().Truncate(time.Millisecond).Sub(w.clk.Now()); d > 0 {
		w.timers.Advance(d)
	}
}

// advance moves the fake clock by d and runs every deadline it reaches.
func (w *world) advance(d time.Duration) { w.timers.Advance(d) }

// users is a report of session users.
func users(names ...string) []any {
	var out []any
	for _, n := range names {
		out = append(out, map[string]any{"os_user": n, "workdirs": []string{}})
	}
	return out
}

// r1Users and r2Users are the reported session users of the hosts.
func r1Users() []any { return users("term-alex", "term-sam", "term-robin") }
func r2Users() []any { return users("term-sam", "term-robin") }

// profiles are the profiles of every host.
func profiles() []any {
	return []any{map[string]any{"name": "shell", "display_name": "Shell", "kind": "shell"}}
}

// report is a report that passes, with the session users u and the server host flag.
func report(u []any, serverHost bool) map[string]any {
	var doctor []any
	for _, c := range []string{"hd1_user", "hd2_groups", "hd3_sudo", "hd4_state_dir", "hd5_server_host", "hd6_llm_login",
		"hd7_profiles", "hd8_separation", "hd9_foreign_homes", "hd10_helper_integrity", "hd11_shared_locations"} {
		doctor = append(doctor, map[string]any{"check": c, "ok": true})
	}
	return map[string]any{"profiles": profiles(), "session_users": u, "doctor": doctor, "is_server_host": serverHost,
		"require_local_confirm": false, "os_version": "Debian 13"}
}

// connect connects the Fake-Host of host with its report; every session is accepted (echo).
func (w *world) connect(host ids.ID, key hosttest.Key, u []any, serverHost bool, live ...ids.ID) *fakehost.Host {
	w.syncClock()
	h := fakehost.Connect(w.t, w.s.PublicURL, host, key, report(u, serverHost), live...)
	h.Default(fakehost.Script{Kind: fakehost.Accept})
	return h
}

// must sends r as c (with a step-up when asked) and fails unless the status is want.
func (w *world) must(c *harness.Client, method, path string, body any, want int) harness.Response {
	w.t.Helper()
	w.syncClock()
	return harness.MustDo(w.t, c, harness.Request{Method: method, Path: path, Body: body}, want)
}

// pairedHost creates, pairs and confirms a host named name with the holder and the assignments
// (person to session user); it returns the login key and the host id.
func (w *world) pairedHost(name, holder string, reported []any, assign map[string]string) (hosttest.Key, ids.ID) {
	w.t.Helper()
	body := map[string]any{"name": name, "reason": "terminal test"}
	if holder != "" {
		body["assigned_user_id"] = holder
	}
	var created struct {
		HostID      string `json:"host_id"`
		PairingCode string `json:"pairing_code"`
	}
	w.must(w.robin, http.MethodPost, "/hosts", body, http.StatusCreated).JSON(w.t, &created)
	key, noise := fakehost.NewKey()
	var jwk map[string]any
	if err := json.Unmarshal(key.JWK(), &jwk); err != nil {
		w.t.Fatal(err)
	}
	var paired struct {
		KeyFingerprint string `json:"key_fingerprint"`
	}
	w.must(w.robin, http.MethodPost, "/hosts/pair", map[string]any{"pairing_code": created.PairingCode, "noise_public_key": fakehost.B64(noise),
		"auth_public_jwk": jwk, "platform": "linux", "host_version": "1.0.0", "profiles": profiles(), "session_users": reported},
		http.StatusCreated).JSON(w.t, &paired)
	people := make([]string, 0, len(assign))
	for u := range assign {
		people = append(people, u)
	}
	sort.Strings(people)
	var su []any
	for _, u := range people {
		su = append(su, map[string]any{"user_id": u, "os_user": assign[u]})
	}
	prefix := strings.ReplaceAll(paired.KeyFingerprint, "-", "")[:8]
	get := w.must(w.robin, http.MethodGet, "/hosts/"+created.HostID, nil, http.StatusOK)
	resp, err := harness.DoWithStepUp(w.t.Context(), w.robin, harness.Request{Method: http.MethodPost, Path: "/hosts/" + created.HostID + "/confirm",
		Header: http.Header{"If-Match": []string{get.Header.Get("ETag")}},
		Body: map[string]any{"fingerprint_prefix": prefix, "llm_credentials": "organization", "declared_server_host": false,
			"session_users": su, "reason": "terminal test"}})
	if err != nil || resp.Status != http.StatusOK {
		w.t.Fatalf("confirmHost: %v %d %s", err, resp.Status, resp.Body)
	}
	return key, ids.MustParse(created.HostID)
}

// grant creates a grant terminal of the subject (subjectKind user or group) on the host and returns
// its id; expires is "" for none.
func (w *world) grant(subjectKind, subject, host, expires string) string {
	w.t.Helper()
	body := map[string]any{"subject_kind": subjectKind, "subject_id": subject, "target_kind": "host", "target_id": host,
		"capabilities": []string{"terminal"}, "reason": "terminal test"}
	if expires != "" {
		body["expires_at"] = expires
	}
	var g struct {
		ID string `json:"id"`
	}
	w.must(w.robin, http.MethodPost, "/grants", body, http.StatusCreated).JSON(w.t, &g)
	return g.ID
}

// group creates a group with one member (membership expiring at expires, "" for never) and returns
// its id.
func (w *world) group(name, member, expires string) string {
	w.t.Helper()
	var g struct {
		ID string `json:"id"`
	}
	w.must(w.robin, http.MethodPost, "/groups", map[string]any{"name": name}, http.StatusCreated).JSON(w.t, &g)
	m := map[string]any{"user_id": member}
	if expires != "" {
		m["expires_at"] = expires
	}
	w.must(w.robin, http.MethodPost, "/groups/"+g.ID+"/members", m, http.StatusCreated)
	return g.ID
}

// stamp formats t like the API.
func stamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

// ctx is a context bounded by waitLong.
func (w *world) ctx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), waitLong)
	w.t.Cleanup(cancel)
	return ctx
}

// grantBody is the decoded TerminalSessionGrant.
type grantBody struct {
	Session struct {
		ID, State, DeviceID string
		Version             int64
	}
	SessionToken   string `json:"session_token"`
	StreamPath     string `json:"stream_path"`
	NoisePublicKey string `json:"noise_public_key"`
}

// openBody is the body of openTerminalSession of a shell.
func openBody() map[string]any {
	return map[string]any{"profile": "shell", "cols": 120, "rows": 40, "client_ephemeral_key_hash": cek}
}

// openRaw sends openTerminalSession as c on host after a fresh step-up.
func (w *world) openRaw(c *harness.Client, host ids.ID) harness.Response {
	w.t.Helper()
	w.syncClock()
	if err := c.StepUp(w.t.Context()); err != nil {
		w.t.Fatal(err)
	}
	return c.Do(w.t.Context(), harness.Request{Method: http.MethodPost, Path: "/hosts/" + host.String() + "/terminal/sessions", Body: openBody()})
}

// open opens a session of c on host and decodes the grant; it fails unless the answer is 201.
func (w *world) open(c *harness.Client, host ids.ID) grantBody {
	w.t.Helper()
	r := w.openRaw(c, host)
	if r.Status != http.StatusCreated {
		w.t.Fatalf("openTerminalSession: %d %s", r.Status, r.Body)
	}
	var g grantBody
	r.JSON(w.t, &g)
	return g
}

// attach sends attachTerminalSession as c, with a fresh step-up when stepUp.
func (w *world) attach(c *harness.Client, session string, stepUp bool) harness.Response {
	w.t.Helper()
	w.syncClock()
	if stepUp {
		if err := c.StepUp(w.t.Context()); err != nil {
			w.t.Fatal(err)
		}
	}
	return c.Do(w.t.Context(), harness.Request{Method: http.MethodPost, Path: "/terminal/sessions/" + session + "/attach",
		Body: map[string]any{"cols": 120, "rows": 40, "client_ephemeral_key_hash": cek}})
}

// device opens the device stream of session as c.
func (w *world) device(c *harness.Client, session string) *fakehost.Device {
	w.t.Helper()
	w.syncClock()
	d, status, err := fakehost.DialDevice(w.t, w.ctx(), w.s.PublicURL, w.s.PublicURL, ids.MustParse(session), c,
		fakehost.DeviceParams{DeviceID: c.DeviceID})
	if err != nil {
		w.t.Fatalf("device stream: %d %v", status, err)
	}
	return d
}

// handshake runs message 1 and 2 and the first transport message between d and the host side,
// which makes an Accept host send session.started.
func (w *world) handshake(d *fakehost.Device, token string) {
	w.t.Helper()
	ctx := w.ctx()
	if err := d.Send(ctx, []byte(token)); err != nil {
		w.t.Fatal(err)
	}
	if m, err := d.Recv(ctx); err != nil || string(m) != "message-2" {
		w.t.Fatalf("message 2: %q %v", m, err)
	}
	if err := d.Send(ctx, []byte("start")); err != nil {
		w.t.Fatal(err)
	}
	if m, err := d.Recv(ctx); err != nil || string(m) != "start" {
		w.t.Fatalf("echo of start: %q %v", m, err)
	}
}

// live is a started session: the grant, the device stream and the data connection of the host.
type live struct {
	g    grantBody
	id   ids.ID
	dev  *fakehost.Device
	data *fakehost.DataConn
	host *fakehost.Host
}

// start opens a shell of c on host (Fake-Host h), connects the device and runs the handshake; the
// session is active when it returns.
func (w *world) start(c *harness.Client, host ids.ID, h *fakehost.Host) live {
	w.t.Helper()
	g := w.open(c, host)
	id := ids.MustParse(g.Session.ID)
	d := w.device(c, g.Session.ID)
	w.handshake(d, g.SessionToken)
	w.waitState(g.Session.ID, "active")
	data, err := h.WaitData(w.ctx(), id)
	if err != nil {
		w.t.Fatalf("data connection: %v", err)
	}
	return live{g: g, id: id, dev: d, data: data, host: h}
}

// text returns a single text value of query on the database.
func (w *world) text(query string, args ...any) string {
	w.t.Helper()
	var s string
	if err := w.s.AdminDB().QueryRow(context.Background(), query, args...).Scan(&s); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return s
}

// count returns an integer result of query.
func (w *world) count(query string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.s.AdminDB().QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return n
}

// exec runs stmt on the database (fixtures behind the module's back).
func (w *world) exec(stmt string, args ...any) {
	w.t.Helper()
	if _, err := w.s.AdminDB().Exec(context.Background(), stmt, args...); err != nil {
		w.t.Fatalf("%s: %v", stmt, err)
	}
}

// state returns state, close_reason and close_detail of a session ("closed:open_failed:open_timeout").
func (w *world) state(session string) string {
	return w.text(`SELECT state || coalesce(':' || close_reason, '') || coalesce(':' || close_detail, '') FROM terminal_sessions WHERE id = $1`, session)
}

// eventually waits until cond holds or waitLong passes.
func (w *world) eventually(what string, cond func() bool) {
	w.t.Helper()
	deadline := time.Now().Add(waitLong)
	for !cond() {
		if time.Now().After(deadline) {
			w.t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// waitState waits until the session has the state string want.
func (w *world) waitState(session, want string) {
	w.t.Helper()
	w.eventually(fmt.Sprintf("session %s %s", session, want), func() bool { return w.state(session) == want })
}

// closeCode waits for the close of a connection and checks code and reason ("" for any reason).
func (w *world) closeCode(what string, got func(context.Context) (int, string), code int, reason string) {
	w.t.Helper()
	c, r := got(w.ctx())
	if c != code || reason != "" && r != reason {
		w.t.Fatalf("%s: close %d %q, want %d %q", what, c, r, code, reason)
	}
}

// assertEnded checks the end of l with reason (no detail): the row closed in one commit with its
// cause, one terminal_session.changed with state closed, one terminal.terminate, both connections
// closed with 4100 and the reason, one session.close at the Fake-Host and the metric raised.
func (w *world) assertEnded(l live, reason domain.CloseReason) {
	w.t.Helper()
	w.waitState(l.id.String(), "closed:"+string(reason))
	w.closeCode("device stream", l.dev.CloseCode, 4100, string(reason))
	w.closeCode("data connection", l.data.CloseCode, 4100, string(reason))
	got, err := l.host.WaitClose(w.ctx(), l.id)
	if err != nil || got != string(reason) {
		w.t.Fatalf("session.close at the host: %q %v, want %s", got, err, reason)
	}
	if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'terminal.terminate' AND resource_id = $1`, l.id); n != 1 {
		w.t.Fatalf("%d entries terminal.terminate, want 1", n)
	}
	if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'terminal_session.changed' AND resource_id = $1
		AND payload->>'state' = 'closed'`, l.id); n != 1 {
		w.t.Fatalf("%d events closed, want 1", n)
	}
	w.eventually("bothy.terminal.sessions.closed "+string(reason), func() bool { return w.metrics.closedOf(reason) > 0 })
}

// sameCommit reports whether the terminal.terminate entry of the session carries the correlation
// of an audit entry of action (the cause), which proves the end committed with its cause.
func (w *world) sameCommit(session ids.ID, action string) bool {
	w.t.Helper()
	return w.count(`SELECT count(*) FROM audit_events t JOIN audit_events c ON c.correlation_id = t.correlation_id
		WHERE t.action = 'terminal.terminate' AND t.resource_id = $1 AND c.action = $2`, session, action) > 0
}
