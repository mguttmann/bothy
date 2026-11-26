// SPDX-License-Identifier: Apache-2.0

// Package terminal_test holds the integration tests of computer sessions (CU-04,
// docs/spec/27-computer-use.md 15: CU-T01 to CU-T06, CU-T16 to CU-T18 server parts, CU-T25): the
// real internal/wire graph on the shared harness with PostgreSQL 18 through pgtest, an agent with a
// DPoP-bound API token and the invocation token of its run from the REST poll, a host paired over
// the API whose control connection, computer data connection and helper report are played by the
// test (the host side of CU-03 is not needed), and the attach connection of bothy-computer-mcp
// played by the test. A harness server takes long to start under load, so one server runs the cases
// as ordered subtests.
package terminal_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/hosttest"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// consumer is the name of the agents' event consumer.
const consumer = "default"

// cek is a well-formed client_ephemeral_key_hash (43 base64url characters of 32 bytes).
var cek = base64.RawURLEncoding.EncodeToString(make([]byte, 32))

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

// world is the world of the computer tests: Robin is the owner (the harness owner), Alex a person;
// the agent Runner (tool class host) operates the channel desk, the agent Texter (tool class
// scoped) the channel notes; desk-a is a host without assignee, desk-b a host assigned to Alex.
type world struct {
	t      *testing.T
	s      *harness.Server
	clk    *clock.Fake
	logs   *syncBuffer
	db     *pgxpool.Pool
	owner  *harness.Client
	alex   *harness.Client
	runner *harness.Client
	texter *harness.Client
	space  string
	desk   string
	notes  string
	hostA  *host
	hostB  *host

	jobsMu gosync.Mutex
	jobs   *wire.JobModules
}

// newWorld starts the server and builds the world.
func newWorld(t *testing.T) *world {
	t.Helper()
	conf := harness.RaisedRateLimits()
	for k, v := range map[string]string{
		"audit.deny_aggregate_window": "50ms",
		"auth.clock_leeway":           "48h",
		"auth.dpop_proof_max_age":     "48h",
		"terminal.sweep_interval":     "24h",
		"computer.open_timeout":       "120s",
		"computer.attach_token_ttl":   "30s",
	} {
		conf[k] = v
	}
	clk := clock.NewFake(time.Now().UTC().Truncate(time.Second))
	w := &world{t: t, clk: clk, logs: &syncBuffer{}}
	w.s = harness.Start(t, harness.Options{Config: conf, Clock: clk, EgressDenyAll: true, LogWriter: w.logs,
		JobModules: func(m *wire.JobModules) { w.jobsMu.Lock(); w.jobs = m; w.jobsMu.Unlock() }})
	w.db = w.s.AdminDB()
	w.owner = w.s.Owner
	w.alex = w.s.NewUser(t, harness.UserSpec{Handle: "alex", DisplayName: "Alex", Email: "alex@example.com"})
	w.runner = w.s.NewAgent(t, harness.AgentSpec{Handle: "runner", DisplayName: "Runner", BillingMode: "organization", AdapterKind: "code_runner"})
	w.texter = w.s.NewAgent(t, harness.AgentSpec{Handle: "texter", DisplayName: "Texter", BillingMode: "organization"})
	w.toolClass(w.runner, "host")
	w.toolClass(w.texter, "scoped")
	w.space = w.created(w.must(w.owner, http.MethodPost, "/spaces", map[string]any{"name": "Ops"}, http.StatusCreated))
	w.desk = w.channel("desk", w.runner)
	w.notes = w.channel("notes", w.texter)
	w.must(w.owner, http.MethodPost, "/grants", map[string]any{"subject_kind": "user", "subject_id": w.alex.PrincipalID, "target_kind": "space",
		"target_id": w.space, "level": "trigger", "reason": "computer test"}, http.StatusCreated)
	for _, a := range []*harness.Client{w.runner, w.texter} {
		w.page(a)
	}
	w.hostA = w.pairHost("desk-a", "")
	w.hostB = w.pairHost("desk-b", w.alex.PrincipalID)
	w.hostA.connect(enabled(1))
	w.hostB.connect(enabled(1))
	return w
}

// ctx returns the context of the test.
func (w *world) ctx() context.Context { return w.t.Context() }

// syncClock moves the fake clock forward to the real time when it lags behind (the clients sign
// with the real clock); never backwards.
func (w *world) syncClock() {
	if d := time.Now().UTC().Truncate(time.Millisecond).Sub(w.clk.Now()); d > 0 {
		w.clk.Advance(d)
	}
}

// do sends one request as c with the Invocation-Token values tokens.
func (w *world) do(c *harness.Client, method, path string, body any, tokens ...string) harness.Response {
	w.t.Helper()
	w.syncClock()
	h := http.Header{}
	for _, tok := range tokens {
		h.Add("Invocation-Token", tok)
	}
	return c.Do(w.ctx(), harness.Request{Method: method, Path: path, Body: body, Header: h})
}

// must sends r as c with the step-up retry of the harness and fails unless the status is want.
func (w *world) must(c *harness.Client, method, path string, body any, want int, tokens ...string) harness.Response {
	w.t.Helper()
	w.syncClock()
	h := http.Header{}
	for _, tok := range tokens {
		h.Add("Invocation-Token", tok)
	}
	r, err := harness.DoWithStepUp(w.ctx(), c, harness.Request{Method: method, Path: path, Body: body, Header: h})
	if err != nil {
		w.t.Fatalf("%s %s: %v", method, path, err)
	}
	if r.Status != want {
		w.t.Fatalf("%s %s = %d %s, want %d", method, path, r.Status, redact(r.Body), want)
	}
	return r
}

// redact cuts a body that may carry a token to its first bytes for a failure message.
func redact(b []byte) string {
	if bytes.Contains(b, []byte(`"token"`)) {
		return "<body with a token>"
	}
	return string(b)
}

// created returns the member id of a JSON answer.
func (w *world) created(r harness.Response) string {
	w.t.Helper()
	var b struct {
		ID string `json:"id"`
	}
	r.JSON(w.t, &b)
	return b.ID
}

// toolClass sets the tool class of agent with the If-Match of its current version.
func (w *world) toolClass(agent *harness.Client, class string) {
	w.t.Helper()
	cur := w.must(w.owner, http.MethodGet, "/agents/"+agent.PrincipalID, nil, http.StatusOK)
	w.syncClock()
	r, err := harness.DoWithStepUp(w.ctx(), w.owner, harness.Request{Method: http.MethodPut, Path: "/agents/" + agent.PrincipalID + "/tool-class",
		Header: http.Header{"If-Match": []string{cur.Header.Get("ETag")}}, Body: map[string]any{"tool_class": class, "reason": "computer test"}})
	if err != nil || r.Status != http.StatusOK {
		w.t.Fatalf("tool class %s: %v %d %s", class, err, r.Status, r.Body)
	}
}

// channel creates a text channel of the space with agent as its operator.
func (w *world) channel(name string, agent *harness.Client) string {
	w.t.Helper()
	id := w.created(w.must(w.owner, http.MethodPost, "/spaces/"+w.space+"/channels", map[string]any{"kind": "text", "name": name}, http.StatusCreated))
	cur := w.must(w.owner, http.MethodGet, "/channels/"+id, nil, http.StatusOK)
	h := http.Header{}
	h.Set("If-Match", cur.Header.Get("ETag"))
	w.syncClock()
	if _, err := harness.DoWithStepUp(w.ctx(), w.owner, harness.Request{Method: http.MethodPut, Path: "/channels/" + id + "/operator", Header: h,
		Body: map[string]any{"mode": "agent", "agent_id": agent.PrincipalID, "reason": "computer test"}}); err != nil {
		w.t.Fatal(err)
	}
	return id
}

// text runs a query that returns one text value.
func (w *world) text(q string, args ...any) string {
	w.t.Helper()
	var s *string
	if err := w.db.QueryRow(w.ctx(), q, args...).Scan(&s); err != nil {
		w.t.Fatalf("%s: %v", q, err)
	}
	if s == nil {
		return ""
	}
	return *s
}

// count runs a query that returns one count.
func (w *world) count(q string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.db.QueryRow(w.ctx(), q, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", q, err)
	}
	return n
}

// eventually polls cond until it holds, bounded by testkit.Patience.
func (w *world) eventually(what string, cond func() bool) {
	w.t.Helper()
	testkit.Until(w.t, what, cond)
}

// eventPage is the body of pollAgentEvents.
type eventPage struct {
	Events []struct {
		Type            string `json:"type"`
		InvocationToken string `json:"invocation_token"`
		Resource        struct {
			ID string `json:"id"`
		} `json:"resource"`
		Payload json.RawMessage `json:"payload"`
	} `json:"events"`
	NextCursor int64 `json:"next_cursor"`
}

// page polls and acknowledges one page of the consumer of agent.
func (w *world) page(agent *harness.Client) eventPage {
	w.t.Helper()
	r := w.do(agent, http.MethodGet, "/agents/me/consumers/"+consumer+"/events?limit=100", nil)
	if r.Status != http.StatusOK {
		w.t.Fatalf("pollAgentEvents = %d %s", r.Status, r.Body)
	}
	var p eventPage
	r.JSON(w.t, &p)
	if len(p.Events) > 0 {
		w.must(agent, http.MethodPost, "/agents/me/consumers/"+consumer+"/ack", map[string]any{"seq": p.NextCursor}, http.StatusOK)
	}
	return p
}

// run is a run with the invocation token its agent got through the REST poll.
type run struct {
	id, thread, token string
	agent             *harness.Client
}

// newRun opens a thread in channel as author and returns the run of agent there with its token.
func (w *world) newRun(author, agent *harness.Client, channel, text string) run {
	w.t.Helper()
	ch := w.must(w.owner, http.MethodGet, "/channels/"+channel, nil, http.StatusOK)
	var c struct {
		Channel struct {
			MainThreadID string `json:"main_thread_id"`
		} `json:"channel"`
	}
	ch.JSON(w.t, &c)
	parent := w.created(w.must(author, http.MethodPost, "/threads/"+c.Channel.MainThreadID+"/messages",
		map[string]any{"client_msg_id": ids.New().String(), "content_md": "Question: " + text}, http.StatusCreated))
	var th struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	w.must(author, http.MethodPost, "/channels/"+channel+"/threads", map[string]any{"title": text, "parent_message_id": parent,
		"first_message": map[string]any{"client_msg_id": ids.New().String(), "content_md": text}}, http.StatusCreated).JSON(w.t, &th)
	var id string
	w.eventually("the run of "+text, func() bool {
		id = w.text(`SELECT max(id::text) FROM runs WHERE thread_id = $1 AND triggered_by = $2`, th.Thread.ID, author.PrincipalID)
		return id != ""
	})
	var tok string
	w.eventually("the agent.invocation of "+text, func() bool {
		for _, ev := range w.page(agent).Events {
			if ev.Type == "agent.invocation" && ev.Resource.ID == id && ev.InvocationToken != "" {
				tok = ev.InvocationToken
			}
		}
		return tok != ""
	})
	return run{id: id, thread: th.Thread.ID, token: tok, agent: agent}
}

// grant is the answer of openComputerSession.
type grant struct {
	SessionID      string `json:"session_id"`
	Token          string `json:"token"`
	AttachPath     string `json:"attach_path"`
	NoisePublicKey string `json:"noise_public_key"`
	KeyGeneration  int    `json:"key_generation"`
}

// open opens a computer session of r on h and returns the answer.
func (w *world) open(r run, h *host) grant {
	w.t.Helper()
	var g grant
	w.must(r.agent, http.MethodPost, "/computer/sessions", map[string]any{"host_id": h.id.String(), "client_ephemeral_key_hash": cek},
		http.StatusCreated, r.token).JSON(w.t, &g)
	return g
}

// problem is the part of a problem body the tests read.
type problem struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
}

// refused asserts a refusal with status, code and reason ("" for none).
func (w *world) refused(resp harness.Response, status int, code, reason string) {
	w.t.Helper()
	var p problem
	_ = json.Unmarshal(resp.Body, &p)
	if resp.Status != status || p.Code != code || p.Reason != reason {
		w.t.Fatalf("got %d %s/%s (%s), want %d %s/%s", resp.Status, p.Code, p.Reason, redact(resp.Body), status, code, reason)
	}
}

// state returns state, close_reason and close_detail of a computer session.
func (w *world) state(session string) (string, string, string) {
	w.t.Helper()
	var st string
	var reason, detail *string
	if err := w.db.QueryRow(w.ctx(), `SELECT state, close_reason, close_detail FROM computer_sessions WHERE id = $1`, session).
		Scan(&st, &reason, &detail); err != nil {
		w.t.Fatal(err)
	}
	deref := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	return st, deref(reason), deref(detail)
}

// waitClosed waits until session is closed with reason and detail.
func (w *world) waitClosed(session, reason, detail string) {
	w.t.Helper()
	w.eventually("the end of "+session, func() bool {
		st, r, d := w.state(session)
		return st == "closed" && r == reason && d == detail
	})
}

// terminateAudit returns the details of the computer.terminate entry of session.
func (w *world) terminateAudit(session string) map[string]any {
	w.t.Helper()
	var raw []byte
	w.eventually("computer.terminate of "+session, func() bool {
		err := w.db.QueryRow(w.ctx(), `SELECT details FROM audit_events WHERE action = 'computer.terminate' AND resource_id = $1`, session).Scan(&raw)
		return err == nil
	})
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		w.t.Fatal(err)
	}
	return m
}

// sweep runs the handler of terminal.sweep once.
func (w *world) sweep() {
	w.t.Helper()
	w.jobsMu.Lock()
	jobs := w.jobs
	w.jobsMu.Unlock()
	if jobs == nil {
		w.t.Fatal("the jobs of the server are not built")
	}
	if _, err := jobs.TerminalSweep.Sweep(w.ctx()); err != nil {
		w.t.Fatalf("terminal.sweep: %v", err)
	}
}

// enabled is report.computer of a connected helper, enabled with generation gen and both
// permissions granted.
func enabled(gen int) map[string]any {
	return map[string]any{"helper": "connected", "enabled": true, "generation": gen, "os_account": "alex",
		"permissions": map[string]any{"capture": "granted", "input": "granted"}, "helper_version": "1.0.0", "signature": "verified", "locked": false}
}

// report is a report that passes, with the computer object c (nil for none).
func report(c map[string]any) map[string]any {
	var doctor []any
	for _, n := range []string{"hd1_user", "hd2_groups", "hd3_sudo", "hd4_state_dir", "hd5_server_host", "hd6_llm_login",
		"hd7_profiles", "hd8_separation", "hd9_foreign_homes", "hd10_helper_integrity", "hd11_shared_locations"} {
		doctor = append(doctor, map[string]any{"check": n, "ok": true})
	}
	r := map[string]any{"profiles": []any{map[string]any{"name": "shell", "display_name": "Shell", "kind": "shell"}},
		"session_users": sessionUsers(), "doctor": doctor, "is_server_host": false, "require_local_confirm": false, "os_version": "Debian 13"}
	if c != nil {
		r["computer"] = c
	}
	return r
}

// sessionUsers is the reported session user of every host (no assignment is needed for computer
// use).
func sessionUsers() []any {
	return []any{map[string]any{"os_user": "term-robin", "workdirs": []string{}}}
}

// host is a paired host whose host service the test plays: the control connection and the
// computer data connections.
type host struct {
	w     *world
	id    ids.ID
	key   hosttest.Key
	ctl   *hosttest.Client
	stop  context.CancelFunc
	mu    gosync.Mutex
	ops   []hostOp
	ended bool
}

// hostOp is one frame the server sent on the control connection.
type hostOp struct {
	op string
	d  json.RawMessage
}

// pairHost creates, pairs and confirms a host with the assignee holder ("" for none).
func (w *world) pairHost(name, holder string) *host {
	w.t.Helper()
	body := map[string]any{"name": name, "reason": "computer test"}
	if holder != "" {
		body["assigned_user_id"] = holder
	}
	var created struct {
		HostID      string `json:"host_id"`
		PairingCode string `json:"pairing_code"`
	}
	w.must(w.owner, http.MethodPost, "/hosts", body, http.StatusCreated).JSON(w.t, &created)
	key, noise := fakehost.NewKey()
	var jwk map[string]any
	if err := json.Unmarshal(key.JWK(), &jwk); err != nil {
		w.t.Fatal(err)
	}
	var paired struct {
		KeyFingerprint string `json:"key_fingerprint"`
	}
	w.must(w.owner, http.MethodPost, "/hosts/pair", map[string]any{"pairing_code": created.PairingCode, "noise_public_key": hosttest.B64(noise),
		"auth_public_jwk": jwk, "platform": "macos", "host_version": "1.0.0", "profiles": []any{map[string]any{"name": "shell",
			"display_name": "Shell", "kind": "shell"}}, "session_users": sessionUsers()}, http.StatusCreated).JSON(w.t, &paired)
	prefix := strings.ReplaceAll(paired.KeyFingerprint, "-", "")[:8]
	get := w.must(w.owner, http.MethodGet, "/hosts/"+created.HostID, nil, http.StatusOK)
	w.syncClock()
	resp, err := harness.DoWithStepUp(w.ctx(), w.owner, harness.Request{Method: http.MethodPost, Path: "/hosts/" + created.HostID + "/confirm",
		Header: http.Header{"If-Match": []string{get.Header.Get("ETag")}},
		Body: map[string]any{"fingerprint_prefix": prefix, "llm_credentials": "organization", "declared_server_host": false,
			"session_users": []any{}, "reason": "computer test"}})
	if err != nil || resp.Status != http.StatusOK {
		w.t.Fatalf("confirmHost: %v %d %s", err, resp.Status, resp.Body)
	}
	return &host{w: w, id: ids.MustParse(created.HostID), key: key}
}

// connect opens the control connection with a report of computer c and reads the server's frames.
func (h *host) connect(c map[string]any) {
	w := h.w
	w.t.Helper()
	w.syncClock()
	ctx, cancel := context.WithCancel(context.Background())
	h.stop = cancel
	h.ctl = hosttest.Dial(w.t, ctx, w.s.PublicURL)
	h.ctl.Login(ctx, hosttest.IdentifyParams{HostID: h.id, Key: h.key})
	if err := h.ctl.Send(ctx, "report", report(c)); err != nil {
		w.t.Fatal(err)
	}
	if err := h.ctl.Heartbeat(ctx, 0); err != nil {
		w.t.Fatal(err)
	}
	for {
		op, d, err := h.ctl.Next(ctx)
		if err != nil {
			w.t.Fatalf("waiting for heartbeat_ack: %v", err)
		}
		if op == "heartbeat_ack" {
			break
		}
		if op == "error" {
			w.t.Fatalf("the report was refused: %s", d)
		}
	}
	h.ctl.StartHeartbeat(time.Second)
	h.mu.Lock()
	h.ended = false
	h.mu.Unlock()
	go func() {
		for {
			op, d, err := h.ctl.Next(ctx)
			if err != nil {
				h.mu.Lock()
				h.ended = true
				h.mu.Unlock()
				return
			}
			h.mu.Lock()
			h.ops = append(h.ops, hostOp{op: op, d: d})
			h.mu.Unlock()
		}
	}()
	w.t.Cleanup(func() { cancel(); _ = h.ctl.Close() })
}

// send sends op with body d on the control connection.
func (h *host) send(op string, d any) {
	h.w.t.Helper()
	if err := h.ctl.Send(h.w.ctx(), op, d); err != nil {
		h.w.t.Fatal(err)
	}
}

// report sends a new report with computer c and waits until the server stored it.
func (h *host) report(c map[string]any) {
	h.w.t.Helper()
	h.send("report", report(c))
	want, _ := json.Marshal(c["generation"])
	h.w.eventually("the stored report of "+h.id.String(), func() bool {
		got := h.w.text(`SELECT (reported_computer->'generation')::text || ':' || (reported_computer->'enabled')::text FROM hosts WHERE id = $1`, h.id)
		return got == string(want)+":"+fmt.Sprint(c["enabled"])
	})
}

// waitOp waits for a frame op whose body names session (or any body for an empty session) and
// returns its body.
func (h *host) waitOp(op, session string) json.RawMessage {
	h.w.t.Helper()
	var out json.RawMessage
	h.w.eventually(op+" "+session, func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		for _, f := range h.ops {
			if f.op == op && (session == "" || bytes.Contains(f.d, []byte(session))) {
				out = f.d
				return true
			}
		}
		return false
	})
	return out
}

// drop ends the control connection without a close frame.
func (h *host) drop() {
	h.ctl.StopHeartbeat()
	h.stop()
	_ = h.ctl.Close()
}

// conn is one WebSocket connection of the test (a computer data connection or an attach).
type conn struct {
	ws *websocket.Conn
}

// dialData opens the computer data connection of session as the host service: hello, identify.
func (h *host) dialData(session string) *conn {
	w := h.w
	w.t.Helper()
	url := "ws" + strings.TrimPrefix(w.s.PublicURL, "http") + "/api/v1/hosts/computer-sessions/" + session + "/stream"
	ws, resp, err := websocket.Dial(w.ctx(), url, &websocket.DialOptions{CompressionMode: websocket.CompressionDisabled})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		w.t.Fatal(err)
	}
	ws.SetReadLimit(-1)
	_, b, err := ws.Read(w.ctx())
	if err != nil {
		w.t.Fatal(err)
	}
	var hello struct {
		D struct {
			Challenge  string    `json:"challenge"`
			ServerTime time.Time `json:"server_time"`
		} `json:"d"`
	}
	if err := json.Unmarshal(b, &hello); err != nil {
		w.t.Fatal(err)
	}
	frame, _ := json.Marshal(map[string]any{"op": "identify", "d": map[string]any{"protocol_version": 1, "host_id": h.id.String(),
		"host_version": "1.0.0", "session_id": session, "proof": hosttest.Proof(h.key, h.id, hello.D.Challenge, hello.D.ServerTime)}})
	if err := ws.Write(w.ctx(), websocket.MessageText, frame); err != nil {
		w.t.Fatal(err)
	}
	return &conn{ws: ws}
}

// attach opens the attach connection of g as bothy-computer-mcp: hello, identify with token.
func (w *world) attach(path, token string) *conn {
	w.t.Helper()
	url := "ws" + strings.TrimPrefix(w.s.PublicURL, "http") + path
	ws, resp, err := websocket.Dial(w.ctx(), url, &websocket.DialOptions{CompressionMode: websocket.CompressionDisabled})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		w.t.Fatal(err)
	}
	ws.SetReadLimit(-1)
	if typ, _, err := ws.Read(w.ctx()); err != nil || typ != websocket.MessageText {
		w.t.Fatalf("hello of the attach: %v", err)
	}
	frame, _ := json.Marshal(map[string]any{"op": "identify", "d": map[string]any{"protocol_version": 1, "token": token}})
	if err := ws.Write(w.ctx(), websocket.MessageText, frame); err != nil {
		w.t.Fatal(err)
	}
	return &conn{ws: ws}
}

// send writes m as one binary message.
func (c *conn) send(t *testing.T, m []byte) {
	t.Helper()
	if err := c.ws.Write(t.Context(), websocket.MessageBinary, m); err != nil {
		t.Fatal(err)
	}
}

// recv reads the next binary message.
func (c *conn) recv(t *testing.T) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), testkit.Patience)
	defer cancel()
	typ, b, err := c.ws.Read(ctx)
	if err != nil || typ != websocket.MessageBinary {
		t.Fatalf("recv: %v %v", typ, err)
	}
	return b
}

// closed waits for the close of the server and returns its code and reason.
func (c *conn) closed(t *testing.T) (int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), testkit.Patience)
	defer cancel()
	for {
		_, _, err := c.ws.Read(ctx)
		if err == nil {
			continue
		}
		var ce websocket.CloseError
		if ok := asClose(err, &ce); ok {
			return int(ce.Code), ce.Reason
		}
		return 0, err.Error()
	}
}

// asClose reports a close error.
func asClose(err error, ce *websocket.CloseError) bool {
	code := websocket.CloseStatus(err)
	if code == -1 {
		return false
	}
	ce.Code = code
	var full websocket.CloseError
	if errors.As(err, &full) {
		ce.Reason = full.Reason
	}
	return true
}

// sentinel is the known byte pattern of CU-T18: it passes the relay in both directions and must be
// found in no table, log line, audit entry or event of the server.
var sentinel = []byte("CU-T18-SENTINEL-\xde\xad\xbe\xef-SCREEN-BYTES-7f3c9a51")

// containsAny reports whether s holds the sentinel in raw, hex or base64 form.
func containsAny(s string) bool {
	forms := []string{string(sentinel), fmt.Sprintf("%x", sentinel), base64.StdEncoding.EncodeToString(sentinel),
		base64.RawURLEncoding.EncodeToString(sentinel), "SCREEN-BYTES-7f3c9a51"}
	return slices.ContainsFunc(forms, func(f string) bool { return strings.Contains(s, f) })
}

// base64RawURL decodes s as base64url without padding.
func base64RawURL(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }
