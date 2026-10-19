// SPDX-License-Identifier: Apache-2.0

// Package hosttest is the Go test client of the control connection of a host service (M2-43,
// asyncapi channel hostControl): Dial connects and reads hello; Identify signs the proof over the
// challenge with the Ed25519 key of a host and sends identify; Send, Next and ExpectClose drive the
// connection frame by frame; Pause freezes the bytes the client reads, so the server's ping of the
// duplicate check goes unanswered; StartHeartbeat sends heartbeats. Every frame in both directions
// is validated with api/specschema (gate G9) unless WithRawClientFrames is given for the frames the
// client sends; a violation fails the test. M2-44's fake host builds on it. Only tests import it.
//
// Example:
//
//	c := hosttest.Dial(t, ctx, srv.URL)
//	err := c.Identify(ctx, hosttest.IdentifyParams{HostID: h1, Key: key})
//	op, d, err := c.Next(ctx) // ready
package hosttest

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/specschema"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// Path is the path of the control connection below the base URL.
const Path = "/api/v1/hosts/connect"

// channel is the asyncapi channel of the control connection.
const channel = "hostControl"

// CloseError is the close of the connection as the client saw it.
type CloseError struct {
	Code   int
	Reason string
}

// Error returns code and reason.
func (e *CloseError) Error() string {
	return fmt.Sprintf("hosttest: closed with %d %q", e.Code, e.Reason)
}

// Option configures Dial.
type Option func(*options)

type options struct {
	rawClient bool
	maxFrame  int
}

// WithRawClientFrames turns the validation of the frames the client sends off (negative tests).
func WithRawClientFrames() Option { return func(o *options) { o.rawClient = true } }

// WithMaxFrameBytes sets sync.ws_max_frame_bytes of the server under test.
func WithMaxFrameBytes(n int) Option { return func(o *options) { o.maxFrame = n } }

// message is one frame read from the server, or the end of the connection.
type message struct {
	op  string
	d   json.RawMessage
	raw []byte
	err error
}

// Client is one control connection of a test.
type Client struct {
	t        testing.TB
	ws       *websocket.Conn
	gate     *gate
	opts     options
	contract *specschema.Contract
	hello    rest.HostChannelHello
	readDone chan struct{}
	hbStop   chan struct{}
	hbOnce   gosync.Once

	mu    gosync.Mutex
	cond  *gosync.Cond
	queue []message
	done  bool
}

// Dial connects to the control connection at baseURL (http or https) and reads hello. The client
// ends with the test. It fails the test when the connection or hello fails.
func Dial(t testing.TB, ctx context.Context, baseURL string, opts ...Option) *Client {
	t.Helper()
	c, err := DialErr(t, ctx, baseURL, opts...)
	if err != nil {
		t.Fatalf("hosttest: %v", err)
	}
	return c
}

// DialErr is Dial returning the error instead of failing the test.
func DialErr(t testing.TB, ctx context.Context, baseURL string, opts ...Option) (*Client, error) {
	o := options{maxFrame: specschema.DefaultMaxFrameBytes}
	for _, f := range opts {
		f(&o)
	}
	contract, err := specschema.Default()
	if err != nil {
		return nil, err
	}
	g := &gate{}
	g.cond = gosync.NewCond(&g.mu)
	tr := http.DefaultTransport.(*http.Transport).Clone() // nosemgrep: bothy-go-egress-only -- test client of the control connection; it dials only the httptest server of the test
	dialer := &net.Dialer{}                               // nosemgrep: bothy-go-egress-only -- the same test client; the gate of Pause needs the connection
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		nc, err := dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		return &gatedConn{Conn: nc, g: g}, nil
	}
	url := "ws" + strings.TrimPrefix(baseURL, "http") + Path
	ws, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPClient: &http.Client{Transport: tr}, CompressionMode: websocket.CompressionDisabled}) // nosemgrep: bothy-go-egress-only -- the same test client
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}
	ws.SetReadLimit(-1)
	c := &Client{t: t, ws: ws, gate: g, opts: o, contract: contract.WithMaxFrameBytes(o.maxFrame), readDone: make(chan struct{}), hbStop: make(chan struct{})}
	c.cond = gosync.NewCond(&c.mu)
	// The reader lives as long as the connection, not as the dial context.
	go c.readLoop(context.WithoutCancel(ctx))
	t.Cleanup(c.finish)
	op, d, err := c.Next(ctx)
	if err != nil {
		return nil, err
	}
	if op != "hello" {
		return nil, fmt.Errorf("hosttest: first frame %q, want hello", op)
	}
	if err := strict(d, &c.hello); err != nil {
		return nil, err
	}
	return c, nil
}

// Hello returns the hello of the server.
func (c *Client) Hello() rest.HostChannelHello { return c.hello }

// readLoop reads and validates the frames of the server until the connection ends.
func (c *Client) readLoop(ctx context.Context) {
	defer close(c.readDone)
	for {
		typ, b, err := c.ws.Read(ctx)
		if err != nil {
			c.push(message{err: closeOf(err)})
			return
		}
		if typ != websocket.MessageText {
			c.t.Errorf("hosttest: binary frame from the server")
			continue
		}
		if err := c.contract.ValidateChannelFrame(channel, true, b); err != nil {
			c.t.Errorf("hosttest: server frame violates the contract (G9): %v", err)
		}
		var env struct {
			Op string          `json:"op"`
			D  json.RawMessage `json:"d"`
		}
		if err := json.Unmarshal(b, &env); err != nil {
			c.t.Errorf("hosttest: server frame is no JSON object")
			continue
		}
		c.push(message{op: env.Op, d: env.D, raw: b})
	}
}

// closeOf turns the end of the read into a CloseError when the server sent a close frame.
func closeOf(err error) error {
	var ce websocket.CloseError
	if errors.As(err, &ce) {
		return &CloseError{Code: int(ce.Code), Reason: ce.Reason}
	}
	return err
}

// push queues m.
func (c *Client) push(m message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queue = append(c.queue, m)
	if m.err != nil {
		c.done = true
	}
	c.cond.Broadcast()
}

// Next returns the next frame of the server: its op and body. After the end of the connection it
// returns the *CloseError of the server's close frame, or the read error. Errors: ctx ending.
func (c *Client) Next(ctx context.Context) (string, json.RawMessage, error) {
	stop := context.AfterFunc(ctx, func() {
		c.mu.Lock()
		c.cond.Broadcast()
		c.mu.Unlock()
	})
	defer stop()
	c.mu.Lock()
	defer c.mu.Unlock()
	for len(c.queue) == 0 {
		if ctx.Err() != nil {
			return "", nil, ctx.Err()
		}
		c.cond.Wait()
	}
	m := c.queue[0]
	if m.err == nil {
		c.queue = c.queue[1:]
	}
	return m.op, m.d, m.err
}

// NextOp returns the next frame and fails the test unless its op is op; it decodes the body into
// v when v is not nil.
func (c *Client) NextOp(ctx context.Context, op string, v any) {
	c.t.Helper()
	got, d, err := c.Next(ctx)
	if err != nil {
		c.t.Fatalf("hosttest: waiting for %s: %v", op, err)
	}
	if got != op {
		c.t.Fatalf("hosttest: got %s %s, want %s", got, d, op)
	}
	if v != nil {
		if err := strict(d, v); err != nil {
			c.t.Fatalf("hosttest: %s: %v", op, err)
		}
	}
}

// ExpectClose reads until the connection ends and returns the close code of the server, or 0 and
// the error when the connection ended without a close frame.
func (c *Client) ExpectClose(ctx context.Context) (int, error) {
	for {
		_, _, err := c.Next(ctx)
		if err == nil {
			continue
		}
		var ce *CloseError
		if errors.As(err, &ce) {
			return ce.Code, nil
		}
		return 0, err
	}
}

// Send sends the frame {op, d}; the frame is validated against the contract unless
// WithRawClientFrames was given.
func (c *Client) Send(ctx context.Context, op string, d any) error {
	b, err := json.Marshal(struct {
		Op string `json:"op"`
		D  any    `json:"d"`
	}{op, d})
	if err != nil {
		return err
	}
	return c.SendRaw(ctx, b)
}

// SendRaw sends b as a text frame, validated like Send.
func (c *Client) SendRaw(ctx context.Context, b []byte) error {
	if !c.opts.rawClient {
		if err := c.contract.ValidateChannelFrame(channel, false, b); err != nil {
			c.t.Errorf("hosttest: client frame violates the contract (G9): %v", err)
		}
	}
	return c.ws.Write(ctx, websocket.MessageText, b)
}

// SendBinary sends b as a binary frame (a negative test: the control connection takes text only).
func (c *Client) SendBinary(ctx context.Context, b []byte) error {
	return c.ws.Write(ctx, websocket.MessageBinary, b)
}

// Key is the login key of a host: the private key and its kid (the RFC 7638 thumbprint).
type Key struct {
	Priv ed25519.PrivateKey
	KID  string
}

// NewKey returns the key of priv with its thumbprint.
func NewKey(priv ed25519.PrivateKey) Key {
	k, err := jose.JWKFromPublicKey(priv.Public())
	if err != nil {
		panic(err)
	}
	return Key{Priv: priv, KID: k.Thumbprint()}
}

// JWK returns the public JWK of k as hosts.auth_public_jwk stores it.
func (k Key) JWK() []byte {
	j, err := jose.JWKFromPublicKey(k.Priv.Public())
	if err != nil {
		panic(err)
	}
	return j.Canonical()
}

// Proof returns the proof of identify (24 section 3.5): a compact JWS with alg EdDSA, typ
// host-auth+jwt and kid, over exactly {challenge, host_id, iat}.
func Proof(k Key, hostID ids.ID, challenge string, iat time.Time) string {
	claims := `{"challenge":` + strconv.Quote(challenge) + `,"host_id":"` + hostID.String() + `","iat":` + strconv.FormatInt(iat.Unix(), 10) + `}`
	tok, err := jose.SignCompact(jose.EdDSASigner{Key: k.Priv}, jose.Header{Typ: "host-auth+jwt", Kid: k.KID}, []byte(claims))
	if err != nil {
		panic(err)
	}
	return tok
}

// IdentifyParams are the members of identify. Zero values take: protocol version 1, host version
// 1.0.0, an empty live_sessions, the proof over the challenge of hello at IAT (zero: the
// server_time of hello).
type IdentifyParams struct {
	HostID           ids.ID
	Key              Key
	ProtocolVersion  int
	HostVersion      string
	LiveSessions     []ids.ID
	OmitLiveSessions bool
	SessionID        ids.ID
	IAT              time.Time
	// Proof replaces the signed proof when set.
	Proof string
}

// Identify sends identify.
func (c *Client) Identify(ctx context.Context, p IdentifyParams) error {
	if p.ProtocolVersion == 0 {
		p.ProtocolVersion = 1
	}
	if p.HostVersion == "" {
		p.HostVersion = "1.0.0"
	}
	if p.IAT.IsZero() {
		p.IAT = c.hello.ServerTime
	}
	if p.Proof == "" {
		p.Proof = Proof(p.Key, p.HostID, c.hello.Challenge, p.IAT)
	}
	d := map[string]any{"protocol_version": p.ProtocolVersion, "host_id": p.HostID.String(), "proof": p.Proof, "host_version": p.HostVersion}
	if !p.OmitLiveSessions {
		live := make([]string, 0, len(p.LiveSessions))
		for _, id := range p.LiveSessions {
			live = append(live, id.String())
		}
		d["live_sessions"] = live
	}
	if !p.SessionID.IsZero() {
		d["session_id"] = p.SessionID.String()
	}
	return c.Send(ctx, "identify", d)
}

// Login sends identify and returns ready; it fails the test when the server answers otherwise.
func (c *Client) Login(ctx context.Context, p IdentifyParams) rest.HostReady {
	c.t.Helper()
	if err := c.Identify(ctx, p); err != nil {
		c.t.Fatalf("hosttest: identify: %v", err)
	}
	var r rest.HostReady
	c.NextOp(ctx, "ready", &r)
	return r
}

// Heartbeat sends heartbeat with active_sessions n.
func (c *Client) Heartbeat(ctx context.Context, n int) error {
	return c.Send(ctx, "heartbeat", map[string]any{"active_sessions": n})
}

// StartHeartbeat sends heartbeat every interval until the client ends or StopHeartbeat.
func (c *Client) StartHeartbeat(interval time.Duration) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-c.hbStop:
				return
			case <-c.readDone:
				return
			case <-t.C:
				_ = c.Heartbeat(context.Background(), 0)
			}
		}
	}()
}

// StopHeartbeat stops the loop of StartHeartbeat.
func (c *Client) StopHeartbeat() { c.hbOnce.Do(func() { close(c.hbStop) }) }

// Pause freezes the bytes the client reads: control frames such as the ping of the duplicate
// check are not answered until Unpause.
func (c *Client) Pause() { c.gate.set(true) }

// Unpause lets the bytes through again.
func (c *Client) Unpause() { c.gate.set(false) }

// Close closes the connection with code 1000.
func (c *Client) Close() error {
	c.StopHeartbeat()
	return c.ws.Close(websocket.StatusNormalClosure, "")
}

// finish is the cleanup: the connection is dropped.
func (c *Client) finish() {
	c.StopHeartbeat()
	c.Unpause()
	_ = c.ws.CloseNow()
	<-c.readDone
}

// strict decodes b into v and refuses unknown members.
func strict(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// Strict decodes the body of a frame strictly into v (unknown members fail).
func Strict(b []byte, v any) error { return strict(b, v) }

// gate blocks the reads of a connection while closed.
type gate struct {
	mu     gosync.Mutex
	cond   *gosync.Cond
	closed bool
}

func (g *gate) set(closed bool) {
	g.mu.Lock()
	g.closed = closed
	g.cond.Broadcast()
	g.mu.Unlock()
}

func (g *gate) wait() {
	g.mu.Lock()
	for g.closed {
		g.cond.Wait()
	}
	g.mu.Unlock()
}

// gatedConn is a net.Conn whose reads wait at the gate.
type gatedConn struct {
	net.Conn
	g *gate
}

// Read implements net.Conn: bytes that arrive while the gate is closed are held until it opens,
// also those of a read that was already waiting.
func (c *gatedConn) Read(b []byte) (int, error) {
	c.g.wait()
	n, err := c.Conn.Read(b)
	c.g.wait()
	return n, err
}

// B64 encodes base64url without padding (challenges and keys of the tests).
func B64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
