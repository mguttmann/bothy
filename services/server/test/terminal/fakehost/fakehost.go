// SPDX-License-Identifier: Apache-2.0

// Package fakehost is the Fake-Host of the terminal tests (M2-44, PLAN row "Test doubles",
// component testsupport): Host holds the keys of an active host, runs its control connection over
// hosttest with a report and a heartbeat loop, and answers every session.open with one data
// connection run by a script per session (Accept, Pair, Refuse, End, EndAfterHandshake, Ignore,
// Stall); Device opens the
// device stream with an access token and a DPoP proof of a harness client. Both exchange opaque
// bytes: message 1 of the device is the compact session token (a test framing only), which the
// relay cannot tell from Noise. Every text frame passes api/specschema.ValidateChannelFrame (G9).
// Timers is the fake of terminal/app.Timers the tests fire. Neither fake replaces the Go test
// device of M2-48 or the real host service (README.md).
package fakehost

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/specschema"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/hosttest"
)

// The channels of asyncapi.yaml the fakes speak.
const (
	channelData   = "hostData"
	channelStream = "terminalStream"
	tokenTyp      = "terminal-session+jwt"
)

// Timers is a fake of terminal/app.Timers: AfterFunc records the function, Fire runs every pending
// one. It is safe for concurrent use.
type Timers struct {
	mu   gosync.Mutex
	list []*timer
}

type timer struct {
	f       func()
	stopped bool
	fired   bool
}

// AfterFunc implements terminal/app.Timers; d is ignored, the test decides when time has passed.
func (t *Timers) AfterFunc(_ time.Duration, f func()) func() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	tm := &timer{f: f}
	t.list = append(t.list, tm)
	return func() bool {
		t.mu.Lock()
		defer t.mu.Unlock()
		was := !tm.stopped && !tm.fired
		tm.stopped = true
		return was
	}
}

// Fire runs every pending timer once, as if its deadline passed, and returns how many ran.
func (t *Timers) Fire() int {
	t.mu.Lock()
	var due []*timer
	for _, tm := range t.list {
		if !tm.stopped && !tm.fired {
			tm.fired = true
			due = append(due, tm)
		}
	}
	t.list = nil
	t.mu.Unlock()
	for _, tm := range due {
		tm.f()
	}
	return len(due)
}

// Pending returns the number of timers not yet fired or stopped.
func (t *Timers) Pending() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for _, tm := range t.list {
		if !tm.stopped && !tm.fired {
			n++
		}
	}
	return n
}

// errNotText is a first frame that is no text frame.
var errNotText = errors.New("fakehost: hello is no text frame")

// ScriptKind names the behaviour of the host service for one session.
type ScriptKind int

// The scripts of Design note 13.
const (
	// Accept reads message 1, verifies the token, answers message 2, sends session.started on the
	// next device frame and then echoes.
	Accept ScriptKind = iota
	// Refuse reads message 1 and closes the data connection with Code and Detail.
	Refuse
	// End sends session.ended with Reason and Detail instead of a data connection.
	End
	// Ignore never opens the data connection.
	Ignore
	// Stall opens the data connection but never reads.
	Stall
	// Pair reads and forwards like Accept but never sends session.started.
	Pair
	// EndAfterHandshake runs the close order of the host service after message 2 (M2-45 Design
	// note 6): it verifies message 1, answers message 2, and on the first device frame sends
	// session.ended with Reason and Detail on the control connection; it keeps the data connection
	// open and forwards nothing, so the server closes both sides through its end path.
	EndAfterHandshake
)

// Script is the behaviour for one session.
type Script struct {
	Kind   ScriptKind
	Code   int
	Detail string
	Reason string
}

// Host is the fake host service of one active host.
type Host struct {
	t        testing.TB
	base     string
	ID       ids.ID
	Key      hosttest.Key
	Control  *hosttest.Client
	contract *specschema.Contract

	mu      gosync.Mutex
	scripts map[ids.ID]Script
	def     Script
	opens   []rest.HostSessionOpen
	closes  []rest.HostSessionClose
	errs    []rest.GatewayError
	jwks    []map[string]any
	seenJTI map[string]bool
	data    map[ids.ID]*DataConn
	tokens  []string
	cond    *gosync.Cond
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewKey returns a fresh login key and Noise key pair of a host: the Ed25519 login key and 32
// random bytes as the static Noise public key (the server never uses it).
func NewKey() (hosttest.Key, []byte) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	noise := make([]byte, 32)
	_, _ = rand.Read(noise)
	return hosttest.NewKey(priv), noise
}

// Connect opens the control connection of the host id with key at baseURL, logs in with
// live_sessions, sends report (a map of HostReport) and starts the heartbeat loop and the reader
// that answers session.open. It fails the test when the login fails.
func Connect(t testing.TB, baseURL string, id ids.ID, key hosttest.Key, report map[string]any, live ...ids.ID) *Host {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	contract, err := specschema.Default()
	if err != nil {
		t.Fatal(err)
	}
	h := &Host{t: t, base: baseURL, ID: id, Key: key, contract: contract, scripts: map[ids.ID]Script{}, seenJTI: map[string]bool{},
		data: map[ids.ID]*DataConn{}, ctx: ctx, cancel: cancel}
	h.cond = gosync.NewCond(&h.mu)
	h.Control = hosttest.Dial(t, ctx, baseURL)
	h.addJWKS(h.Control.Hello().Jwks)
	h.Control.Login(ctx, hosttest.IdentifyParams{HostID: id, Key: key, LiveSessions: live})
	if err := h.Control.Send(ctx, "report", report); err != nil {
		t.Fatal(err)
	}
	if err := h.Control.Heartbeat(ctx, 0); err != nil {
		t.Fatal(err)
	}
	// The heartbeat_ack shows the report was applied before it.
	for {
		op, d, err := h.Control.Next(ctx)
		if err != nil {
			t.Fatalf("fakehost: waiting for heartbeat_ack: %v", err)
		}
		if op == "heartbeat_ack" {
			h.ack(d)
			break
		}
		if op == "error" {
			t.Fatalf("fakehost: the report was refused: %s", d)
		}
	}
	h.Control.StartHeartbeat(time.Second)
	go h.read()
	t.Cleanup(h.Close)
	return h
}

// Close ends the control connection and every data connection.
func (h *Host) Close() {
	h.cancel()
	_ = h.Control.Close()
	h.mu.Lock()
	list := make([]*DataConn, 0, len(h.data))
	for _, d := range h.data {
		list = append(list, d)
	}
	h.mu.Unlock()
	for _, d := range list {
		_ = d.ws.CloseNow()
	}
}

// Drop closes the control connection without a close frame (the link is lost).
func (h *Host) Drop() {
	h.cancel()
	h.Control.StopHeartbeat()
	_ = h.Control.Close()
}

// SetScript sets the script of session; Default sets the one of every other session.
func (h *Host) SetScript(session ids.ID, s Script) {
	h.mu.Lock()
	h.scripts[session] = s
	h.mu.Unlock()
}

// Default sets the script of every session without its own.
func (h *Host) Default(s Script) {
	h.mu.Lock()
	h.def = s
	h.mu.Unlock()
}

// Send sends an op on the control connection (report, session.ended, session.started).
func (h *Host) Send(op string, d any) error { return h.Control.Send(h.ctx, op, d) }

// read handles the frames of the server until the connection ends.
func (h *Host) read() {
	for {
		op, d, err := h.Control.Next(h.ctx)
		if err != nil {
			h.mu.Lock()
			h.cond.Broadcast()
			h.mu.Unlock()
			return
		}
		switch op {
		case "session.open":
			var o rest.HostSessionOpen
			if json.Unmarshal(d, &o) == nil {
				h.mu.Lock()
				h.opens = append(h.opens, o)
				s, ok := h.scripts[ids.ID(o.SessionId)]
				if !ok {
					s = h.def
				}
				h.cond.Broadcast()
				h.mu.Unlock()
				go h.run(ids.ID(o.SessionId), s)
			}
		case "session.close":
			var c rest.HostSessionClose
			if json.Unmarshal(d, &c) == nil {
				h.mu.Lock()
				h.closes = append(h.closes, c)
				h.cond.Broadcast()
				h.mu.Unlock()
			}
		case "heartbeat_ack":
			h.ack(d)
		case "error":
			var e rest.GatewayError
			if json.Unmarshal(d, &e) == nil {
				h.mu.Lock()
				h.errs = append(h.errs, e)
				h.mu.Unlock()
			}
		}
	}
}

// ack takes the key set of a heartbeat_ack.
func (h *Host) ack(d json.RawMessage) {
	var a rest.HostHeartbeatAck
	if json.Unmarshal(d, &a) == nil && a.Jwks != nil {
		h.addJWKS(*a.Jwks)
	}
}

// addJWKS replaces the key set.
func (h *Host) addJWKS(set rest.Jwks) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.jwks = nil
	for _, k := range set.Keys {
		h.jwks = append(h.jwks, map[string]any(k))
	}
}

// Opens returns the session.open frames received so far.
func (h *Host) Opens() []rest.HostSessionOpen {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]rest.HostSessionOpen(nil), h.opens...)
}

// OpensOf returns the number of session.open of session.
func (h *Host) OpensOf(session ids.ID) int {
	n := 0
	for _, o := range h.Opens() {
		if ids.ID(o.SessionId) == session {
			n++
		}
	}
	return n
}

// Closes returns the session.close frames received so far.
func (h *Host) Closes() []rest.HostSessionClose {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]rest.HostSessionClose(nil), h.closes...)
}

// WaitClose waits until a session.close of session arrived and returns its reason.
func (h *Host) WaitClose(ctx context.Context, session ids.ID) (string, error) {
	return waitFor(ctx, h, func() (string, bool) {
		for _, c := range h.closes {
			if ids.ID(c.SessionId) == session {
				return string(c.CloseReason), true
			}
		}
		return "", false
	})
}

// WaitOpens waits until n session.open of session arrived.
func (h *Host) WaitOpens(ctx context.Context, session ids.ID, n int) error {
	_, err := waitFor(ctx, h, func() (string, bool) {
		c := 0
		for _, o := range h.opens {
			if ids.ID(o.SessionId) == session {
				c++
			}
		}
		return "", c >= n
	})
	return err
}

// Data returns the current data connection of session; nil without one.
func (h *Host) Data(session ids.ID) *DataConn {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.data[session]
}

// WaitData waits until a data connection of session exists and returns it.
func (h *Host) WaitData(ctx context.Context, session ids.ID) (*DataConn, error) {
	var out *DataConn
	_, err := waitFor(ctx, h, func() (string, bool) {
		out = h.data[session]
		return "", out != nil
	})
	return out, err
}

// Tokens returns the session tokens the data connections received, in order.
func (h *Host) Tokens() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.tokens...)
}

// waitFor waits under h.mu until cond holds or ctx ends.
func waitFor(ctx context.Context, h *Host, cond func() (string, bool)) (string, error) {
	stop := context.AfterFunc(ctx, func() {
		h.mu.Lock()
		h.cond.Broadcast()
		h.mu.Unlock()
	})
	defer stop()
	h.mu.Lock()
	defer h.mu.Unlock()
	for {
		if v, ok := cond(); ok {
			return v, nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		h.cond.Wait()
	}
}

// run executes the script s for session after its session.open.
func (h *Host) run(session ids.ID, s Script) {
	switch s.Kind {
	case Ignore:
		return
	case End:
		d := map[string]any{"session_id": session.String(), "close_reason": s.Reason}
		if s.Detail != "" {
			d["close_detail"] = s.Detail
		}
		_ = h.Send("session.ended", d)
		return
	case Accept, Refuse, Stall, Pair, EndAfterHandshake:
	}
	dc, err := h.DialData(h.ctx, session)
	if err != nil {
		return
	}
	switch s.Kind {
	case Stall:
		return
	case Refuse:
		if _, err := dc.Recv(h.ctx); err != nil {
			return
		}
		_ = dc.ws.Close(websocket.StatusCode(s.Code), s.Detail)
	case Accept, Pair:
		h.accept(dc, s.Kind == Accept)
	case EndAfterHandshake:
		h.endAfterHandshake(dc, s)
	case End, Ignore:
	}
}

// accept runs Accept: message 1 is the token, verified against the key set; message 2 answers;
// the first transport message of the device triggers session.started (with start); then every
// message is echoed.
func (h *Host) accept(dc *DataConn, start bool) {
	tok, err := dc.Recv(h.ctx)
	if err != nil {
		return
	}
	h.mu.Lock()
	h.tokens = append(h.tokens, string(tok))
	h.mu.Unlock()
	if err := h.VerifyToken(string(tok), dc.Session); err != nil {
		_ = dc.ws.Close(4104, "token_invalid")
		return
	}
	if err := dc.Send(h.ctx, []byte("message-2")); err != nil {
		return
	}
	first := true
	for {
		m, err := dc.Recv(h.ctx)
		if err != nil {
			return
		}
		if first && start {
			first = false
			_ = h.Send("session.started", map[string]any{"session_id": dc.Session.String()})
		}
		if err := dc.Send(h.ctx, m); err != nil {
			return
		}
	}
}

// endAfterHandshake runs EndAfterHandshake: message 1 verified, message 2 answered, then the
// first device frame triggers session.ended; the data connection stays open until the server
// closes it, which CloseCode reports.
func (h *Host) endAfterHandshake(dc *DataConn, s Script) {
	tok, err := dc.Recv(h.ctx)
	if err != nil {
		return
	}
	h.mu.Lock()
	h.tokens = append(h.tokens, string(tok))
	h.mu.Unlock()
	if err := h.VerifyToken(string(tok), dc.Session); err != nil {
		_ = dc.ws.Close(4104, "token_invalid")
		return
	}
	if err := dc.Send(h.ctx, []byte("message-2")); err != nil {
		return
	}
	if _, err := dc.Recv(h.ctx); err != nil {
		return
	}
	d := map[string]any{"session_id": dc.Session.String(), "close_reason": s.Reason}
	if s.Detail != "" {
		d["close_detail"] = s.Detail
	}
	_ = h.Send("session.ended", d)
}

// VerifyToken checks a session token like the host service (24 section 5.3): the signature with a
// key of the set from hello and heartbeat_ack, typ, aud, sid, exp and a jti never seen.
func (h *Host) VerifyToken(tok string, session ids.ID) error {
	j, err := jose.ParseCompactJWS(tok, []string{jose.AlgEdDSA}, tokenTyp)
	if err != nil {
		return err
	}
	h.mu.Lock()
	var key map[string]any
	for _, k := range h.jwks {
		if k["kid"] == j.Header.Kid {
			key = k
		}
	}
	h.mu.Unlock()
	if key == nil {
		return errors.New("fakehost: unknown kid")
	}
	raw, _ := json.Marshal(map[string]any{"kty": key["kty"], "crv": key["crv"], "x": key["x"]})
	k, err := jose.ParseJWK(raw)
	if err != nil {
		return err
	}
	if err := j.Verify(k); err != nil {
		return err
	}
	var c struct {
		Aud, Sid, Jti string
		Exp           int64
	}
	if err := json.Unmarshal(j.Payload, &c); err != nil {
		return err
	}
	if c.Aud != h.ID.String() || c.Sid != session.String() || time.Now().Unix() > c.Exp+30 {
		return errors.New("fakehost: aud, sid or exp")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.seenJTI[c.Jti] {
		return errors.New("fakehost: jti replayed")
	}
	h.seenJTI[c.Jti] = true
	return nil
}

// DataConn is one data connection of the fake host.
type DataConn struct {
	Session ids.ID
	// Hello is the hello of the server on this connection (schema HostChannelHello).
	Hello rest.HostChannelHello
	ws    *websocket.Conn
	h     *Host
	done  chan struct{}
	mu    gosync.Mutex
	queue [][]byte
	cerr  error
	cond  *gosync.Cond
}

// DialData opens the data connection of session: hello, identify with session_id; the frames
// after it are binary. Errors: the dial or the login.
func (h *Host) DialData(ctx context.Context, session ids.ID) (*DataConn, error) {
	url := "ws" + strings.TrimPrefix(h.base, "http") + "/api/v1/hosts/sessions/" + session.String() + "/stream"
	ws, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{CompressionMode: websocket.CompressionDisabled}) // nosemgrep: bothy-go-egress-only -- test client of the data connection; it dials only the server of the test
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}
	ws.SetReadLimit(-1)
	typ, b, err := ws.Read(ctx)
	if err != nil {
		_ = ws.CloseNow()
		return nil, fmt.Errorf("fakehost: hello: %w", err)
	}
	if typ != websocket.MessageText {
		_ = ws.CloseNow()
		return nil, errNotText
	}
	if err := h.contract.ValidateChannelFrame(channelData, true, b); err != nil {
		h.t.Errorf("fakehost: server frame violates the contract (G9): %v", err)
	}
	var env struct {
		D rest.HostChannelHello `json:"d"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, err
	}
	d := map[string]any{"protocol_version": 1, "host_id": h.ID.String(), "host_version": "1.0.0", "session_id": session.String(),
		"proof": hosttest.Proof(h.Key, h.ID, env.D.Challenge, env.D.ServerTime)}
	frame, _ := json.Marshal(map[string]any{"op": "identify", "d": d})
	if err := h.contract.ValidateChannelFrame(channelData, false, frame); err != nil {
		h.t.Errorf("fakehost: client frame violates the contract (G9): %v", err)
	}
	if err := ws.Write(ctx, websocket.MessageText, frame); err != nil {
		return nil, err
	}
	dc := &DataConn{Session: session, Hello: env.D, ws: ws, h: h, done: make(chan struct{})}
	dc.cond = gosync.NewCond(&dc.mu)
	go dc.read() //nolint:gosec // G118: the reader lives as long as the connection, not as ctx
	h.mu.Lock()
	h.data[session] = dc
	h.cond.Broadcast()
	h.mu.Unlock()
	return dc, nil
}

// read queues the binary messages until the connection ends.
func (dc *DataConn) read() {
	defer close(dc.done)
	for {
		typ, b, err := dc.ws.Read(context.Background())
		dc.mu.Lock()
		if err != nil {
			dc.cerr = err
			dc.cond.Broadcast()
			dc.mu.Unlock()
			return
		}
		if typ == websocket.MessageBinary {
			dc.queue = append(dc.queue, b)
		}
		dc.cond.Broadcast()
		dc.mu.Unlock()
	}
}

// Recv returns the next binary message; after the end the close error.
func (dc *DataConn) Recv(ctx context.Context) ([]byte, error) {
	return recv(ctx, &dc.mu, dc.cond, &dc.queue, &dc.cerr)
}

// Send sends m as one binary message.
func (dc *DataConn) Send(ctx context.Context, m []byte) error {
	return dc.ws.Write(ctx, websocket.MessageBinary, m)
}

// SendText sends a text frame (a negative test).
func (dc *DataConn) SendText(ctx context.Context, s string) error {
	return dc.ws.Write(ctx, websocket.MessageText, []byte(s))
}

// CloseCode waits for the end of the connection and returns the close code and reason of the
// server; 0 without a close frame.
func (dc *DataConn) CloseCode(ctx context.Context) (int, string) {
	return closeOf(ctx, dc.done, &dc.mu, &dc.cerr)
}

// Sync pings the server and waits for the pong: the data connection is in the relay (or ended)
// when it returns.
func (dc *DataConn) Sync(ctx context.Context) error { return dc.ws.Ping(ctx) }

// Close closes the connection with code and reason (the host service ending its side).
func (dc *DataConn) Close(code int, reason string) error {
	return dc.ws.Close(websocket.StatusCode(code), reason)
}

// recv is the shared receive of both fakes.
func recv(ctx context.Context, mu *gosync.Mutex, cond *gosync.Cond, queue *[][]byte, cerr *error) ([]byte, error) {
	stop := context.AfterFunc(ctx, func() {
		mu.Lock()
		cond.Broadcast()
		mu.Unlock()
	})
	defer stop()
	mu.Lock()
	defer mu.Unlock()
	for {
		if len(*queue) > 0 {
			m := (*queue)[0]
			*queue = (*queue)[1:]
			return m, nil
		}
		if *cerr != nil {
			return nil, *cerr
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		cond.Wait()
	}
}

// closeOf waits for done and returns the close code and reason of the server.
func closeOf(ctx context.Context, done chan struct{}, mu *gosync.Mutex, cerr *error) (int, string) {
	select {
	case <-done:
	case <-ctx.Done():
		return -1, "timeout"
	}
	mu.Lock()
	defer mu.Unlock()
	var ce websocket.CloseError
	if errors.As(*cerr, &ce) {
		return int(ce.Code), ce.Reason
	}
	return 0, ""
}

// Prover signs the DPoP proof of the device stream (harness.Client.Proof).
type Prover interface {
	Proof(htm, htu, nonce string) string
	Tokens() (access, refresh string)
}

// DeviceParams steer the identify of a Device; zero values send a correct identify.
type DeviceParams struct {
	// DeviceID is the device_id member; required.
	DeviceID string
	// HTU replaces the htu of the proof; Nonce the nonce; Origin sets the Origin header.
	HTU, Nonce, Origin string
	// NoToken leaves access_token and dpop out; BadProof sends a broken proof.
	NoToken, BadProof bool
	// ProtocolVersion replaces 1; Extra adds members to d; Raw replaces the whole frame.
	ProtocolVersion int
	Extra           map[string]any
	Raw             []byte
	// BinaryFirst sends a binary frame instead of identify; Silent sends nothing.
	BinaryFirst, Silent bool
}

// Device is one device stream.
type Device struct {
	Session ids.ID
	ws      *websocket.Conn
	Hello   rest.TerminalStreamHello
	done    chan struct{}
	mu      gosync.Mutex
	queue   [][]byte
	cerr    error
	cond    *gosync.Cond
	// paused holds the reader before its next read (Pause, Resume): a slow device.
	paused bool
}

// Pause stops reading from the server before the next message, so the server's writes back up.
func (dv *Device) Pause() {
	dv.mu.Lock()
	dv.paused = true
	dv.mu.Unlock()
}

// Resume reads again.
func (dv *Device) Resume() {
	dv.mu.Lock()
	dv.paused = false
	dv.cond.Broadcast()
	dv.mu.Unlock()
}

// DialDevice opens the device stream of session at baseURL as the client p and sends identify as
// p says; publicURL is server.public_url (the htu is its HTTPS form). It returns the device, or
// the HTTP status of a refused upgrade.
func DialDevice(t testing.TB, ctx context.Context, baseURL, publicURL string, session ids.ID, p Prover, dp DeviceParams) (*Device, int, error) {
	contract, err := specschema.Default()
	if err != nil {
		return nil, 0, err
	}
	url := "ws" + strings.TrimPrefix(baseURL, "http") + "/api/v1/terminal/sessions/" + session.String() + "/stream"
	opts := &websocket.DialOptions{CompressionMode: websocket.CompressionDisabled}
	if dp.Origin != "" {
		opts.HTTPHeader = http.Header{"Origin": []string{dp.Origin}}
	}
	ws, resp, err := websocket.Dial(ctx, url, opts) // nosemgrep: bothy-go-egress-only -- test client of the device stream; it dials only the server of the test
	status := 0
	if resp != nil {
		status = resp.StatusCode
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
	}
	if err != nil {
		return nil, status, err
	}
	ws.SetReadLimit(-1)
	typ, b, err := ws.Read(ctx)
	if err != nil {
		return nil, status, fmt.Errorf("fakehost: hello: %w", err)
	}
	if typ != websocket.MessageText {
		return nil, status, errNotText
	}
	if err := contract.ValidateChannelFrame(channelStream, true, b); err != nil {
		t.Errorf("fakehost: server frame violates the contract (G9): %v", err)
	}
	dev := &Device{Session: session, ws: ws, done: make(chan struct{})}
	dev.cond = gosync.NewCond(&dev.mu)
	var env struct {
		D rest.TerminalStreamHello `json:"d"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, status, err
	}
	dev.Hello = env.D
	go dev.read() //nolint:gosec // G118: the reader lives as long as the connection, not as ctx
	t.Cleanup(func() { _ = ws.CloseNow() })
	switch {
	case dp.Silent:
		return dev, status, nil
	case dp.BinaryFirst:
		return dev, status, ws.Write(ctx, websocket.MessageBinary, []byte{1})
	}
	frame := dp.Raw
	if frame == nil {
		frame = identifyFrame(publicURL, session, env.D.DpopNonce, p, dp)
		if dp.Extra == nil && dp.ProtocolVersion == 0 {
			if err := contract.ValidateChannelFrame(channelStream, false, frame); err != nil {
				t.Errorf("fakehost: client frame violates the contract (G9): %v", err)
			}
		}
	}
	return dev, status, ws.Write(ctx, websocket.MessageText, frame)
}

// identifyFrame builds identify of dp.
func identifyFrame(publicURL string, session ids.ID, nonce string, p Prover, dp DeviceParams) []byte {
	v := dp.ProtocolVersion
	if v == 0 {
		v = 1
	}
	d := map[string]any{"protocol_version": v, "device_id": dp.DeviceID}
	if !dp.NoToken {
		htu := dp.HTU
		if htu == "" {
			htu = "https://" + strings.TrimPrefix(strings.TrimPrefix(publicURL, "https://"), "http://") +
				"/api/v1/terminal/sessions/" + session.String() + "/stream"
		}
		if dp.Nonce != "" {
			nonce = dp.Nonce
		}
		access, _ := p.Tokens()
		proof := p.Proof(http.MethodGet, htu, nonce)
		if dp.BadProof {
			proof = proof[:len(proof)-4] + "AAAA"
		}
		d["access_token"], d["dpop"] = access, proof
	}
	for k, x := range dp.Extra {
		d[k] = x
	}
	b, _ := json.Marshal(map[string]any{"op": "identify", "d": d})
	return b
}

// read queues the binary messages until the connection ends; it waits while paused.
func (dv *Device) read() {
	defer close(dv.done)
	for {
		dv.mu.Lock()
		for dv.paused {
			dv.cond.Wait()
		}
		dv.mu.Unlock()
		typ, b, err := dv.ws.Read(context.Background())
		dv.mu.Lock()
		if err != nil {
			dv.cerr = err
			dv.cond.Broadcast()
			dv.mu.Unlock()
			return
		}
		if typ == websocket.MessageBinary {
			dv.queue = append(dv.queue, b)
		}
		dv.cond.Broadcast()
		dv.mu.Unlock()
	}
}

// Send sends m as one binary message.
func (dv *Device) Send(ctx context.Context, m []byte) error {
	return dv.ws.Write(ctx, websocket.MessageBinary, m)
}

// SendText sends a text frame (a negative test).
func (dv *Device) SendText(ctx context.Context, s string) error {
	return dv.ws.Write(ctx, websocket.MessageText, []byte(s))
}

// Recv returns the next binary message; after the end the close error.
func (dv *Device) Recv(ctx context.Context) ([]byte, error) {
	return recv(ctx, &dv.mu, dv.cond, &dv.queue, &dv.cerr)
}

// CloseCode waits for the end of the stream and returns the close code and reason of the server.
func (dv *Device) CloseCode(ctx context.Context) (int, string) {
	return closeOf(ctx, dv.done, &dv.mu, &dv.cerr)
}

// Sync pings the server and waits for the pong: the server answers only once it reads the stream
// after identify, so the device side is in the relay (or the stream ended) when it returns.
func (dv *Device) Sync(ctx context.Context) error { return dv.ws.Ping(ctx) }

// Close closes the stream with 1000 (the device leaves).
func (dv *Device) Close() error { return dv.ws.Close(websocket.StatusNormalClosure, "") }

// Ended reports whether the stream has ended.
func (dv *Device) Ended() bool {
	select {
	case <-dv.done:
		return true
	default:
		return false
	}
}

// Dial opens a plain WebSocket client of url for the tests of the relay (compression off, no read
// limit); the tests of services/server dial only through the test clients (TestHarness_OnlyEntryPoint).
// It returns the connection and the HTTP status of a refused upgrade.
func Dial(ctx context.Context, url string) (*websocket.Conn, int, error) {
	ws, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{CompressionMode: websocket.CompressionDisabled}) // nosemgrep: bothy-go-egress-only -- test client of the relay tests; it dials only the server of the test
	status := 0
	if resp != nil {
		status = resp.StatusCode
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
	}
	if err != nil {
		return nil, status, err
	}
	ws.SetReadLimit(-1)
	return ws, status, nil
}

// B64 encodes base64url without padding.
func B64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// Equal reports whether a and b hold the same bytes.
func Equal(a, b []byte) bool { return bytes.Equal(a, b) }
