// SPDX-License-Identifier: Apache-2.0

// Package wstest is the gateway test harness: the only Go client of the WebSocket gateway in
// tests (TestHarness_OnlyEntryPoint holds that) and the client side of gate G9 (M1-67). Dial
// connects and reads hello; Identify and Resume open a session and start the heartbeat loop of
// 04 section 6.3 step 5, whose first heartbeat waits a uniformly random share of
// heartbeat_interval_ms; Send, Next and ExpectClose drive the connection frame by frame.
//
// Every client reads frames in a background goroutine and records them. Each server frame is
// validated on receipt against docs/api/asyncapi.yaml and openapi.yaml (api/specschema: strict
// schema, no null, integer bound, frame size, durable/ephemeral envelope, resource.version), and
// each client frame on send unless WithRawClientFrames is given (negative tests). Per connection
// the harness checks the order: the first server frame is hello, the seq of durable dispatch
// frames strictly increases, cursor and catchup.complete never fall below the last delivered seq;
// and a close of the server carries a code of x-close-codes. Violations fail the test in
// t.Cleanup with test name, frame index, op and JSON pointer. Only tests import it.
//
// Example:
//
//	c, err := wstest.Dial(t, ctx, "http://127.0.0.1:41234", wstest.WithOrigin("https://chat.example.com"))
//	ready, err := c.Identify(ctx, wstest.IdentifyParams{AccessToken: tok, DeviceID: dev, Proof: proofFor})
package wstest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/specschema"
)

// Path is the path of the gateway below the base URL.
const Path = "/api/v1/gateway"

// drainTimeout bounds how long the cleanup of a paused client reads the rest.
const drainTimeout = 5 * time.Second

// CloseError is the close of the server as the client saw it.
type CloseError struct {
	Code   int
	Reason string
}

// Error returns code and reason.
func (e *CloseError) Error() string {
	return fmt.Sprintf("wstest: closed with %d %q", e.Code, e.Reason)
}

// Option configures Dial.
type Option func(*options)

type options struct {
	header       http.Header
	heartbeat    bool
	random       func() float64
	onFirstDelay func(time.Duration)
	rawClient    bool
	maxFrame     int
}

// WithOrigin sends the Origin header origin with the upgrade.
func WithOrigin(origin string) Option {
	return func(o *options) { o.header.Set("Origin", origin) }
}

// WithHeader adds the header name with value to the upgrade.
func WithHeader(name, value string) Option {
	return func(o *options) { o.header.Add(name, value) }
}

// WithoutHeartbeat turns the automatic heartbeat loop off.
func WithoutHeartbeat() Option {
	return func(o *options) { o.heartbeat = false }
}

// WithRandom replaces the source of the random share of the first heartbeat delay (a value in
// [0, 1)).
func WithRandom(f func() float64) Option {
	return func(o *options) { o.random = f }
}

// OnFirstHeartbeatDelay calls f with the delay of the first heartbeat when the loop starts.
func OnFirstHeartbeatDelay(f func(time.Duration)) Option {
	return func(o *options) { o.onFirstDelay = f }
}

// WithRawClientFrames turns the validation of client frames off, for negative tests that send
// invalid frames on purpose. Server frames are always validated.
func WithRawClientFrames() Option {
	return func(o *options) { o.rawClient = true }
}

// WithMaxFrameBytes sets the sync.ws_max_frame_bytes of the server under test (default
// specschema.DefaultMaxFrameBytes); frames above it are violations.
func WithMaxFrameBytes(n int) Option {
	return func(o *options) { o.maxFrame = n }
}

// message is one frame read from the server, or the end of the connection.
type message struct {
	op  string
	d   json.RawMessage
	err error
}

// Client is one gateway connection of a test.
type Client struct {
	t        testing.TB
	ws       *websocket.Conn
	opts     options
	contract *specschema.Contract
	hello    rest.GatewayHello
	cancel   context.CancelFunc
	stop     chan struct{}
	once     gosync.Once
	readDone chan struct{}

	mu         gosync.Mutex
	cond       *gosync.Cond // signals queue, paused and closed changes
	queue      []message    // frames read and not yet taken by Next
	closed     bool         // the read loop ended
	paused     bool
	selfClosed bool // Close or Abort ended the connection, so its close is not the server's
	lastSeq    int64
	// Order and conformance state of the connection.
	serverFrames, clientFrames int
	delivered                  int64 // seq of the last durable dispatch, 0 before the first
	violations                 []string
}

// Dial connects to the gateway at baseURL (http or https, for example the URL of a test server)
// and reads hello. The client is closed and its violations reported in t.Cleanup. Errors: the
// contract does not load, the upgrade fails (a 403 for a foreign Origin, for example), or the
// first frame is no valid hello.
func Dial(t testing.TB, ctx context.Context, baseURL string, opts ...Option) (*Client, error) {
	t.Helper()
	o := options{header: http.Header{}, heartbeat: true, random: rand.Float64, maxFrame: specschema.DefaultMaxFrameBytes}
	for _, f := range opts {
		f(&o)
	}
	contract, err := specschema.Default()
	if err != nil {
		return nil, err
	}
	ws, resp, err := websocket.Dial(ctx, wsURL(baseURL), &websocket.DialOptions{HTTPHeader: o.header, CompressionMode: websocket.CompressionDisabled})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}
	ws.SetReadLimit(-1)
	rctx, cancel := context.WithCancel(context.Background())
	c := &Client{t: t, ws: ws, opts: o, contract: contract.WithMaxFrameBytes(o.maxFrame), cancel: cancel, stop: make(chan struct{}),
		readDone: make(chan struct{})}
	c.cond = gosync.NewCond(&c.mu)
	t.Cleanup(c.finish)
	go c.readLoop(rctx)
	op, d, err := c.next(ctx, false)
	if err != nil {
		c.Abort()
		return nil, err
	}
	if op != "hello" {
		c.Abort()
		return nil, fmt.Errorf("wstest: first frame %q, want hello", op)
	}
	if err := Strict(d, &c.hello); err != nil {
		c.Abort()
		return nil, err
	}
	return c, nil
}

// DialStatus attempts the upgrade with the options and returns the HTTP status of the answer:
// 101 after a successful upgrade (which it closes again), the status of a refused one otherwise.
// No frame is exchanged, so nothing is validated. Errors: no answer at all.
func DialStatus(ctx context.Context, baseURL string, opts ...Option) (int, error) {
	o := options{header: http.Header{}}
	for _, f := range opts {
		f(&o)
	}
	ws, resp, err := websocket.Dial(ctx, wsURL(baseURL), &websocket.DialOptions{HTTPHeader: o.header, CompressionMode: websocket.CompressionDisabled})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		_ = ws.CloseNow()
		return http.StatusSwitchingProtocols, nil
	}
	if resp == nil {
		return 0, err
	}
	return resp.StatusCode, nil
}

// wsURL returns the WebSocket URL of the gateway below baseURL.
func wsURL(baseURL string) string {
	return strings.Replace(strings.Replace(baseURL, "https://", "wss://", 1), "http://", "ws://", 1) + Path
}

// readLoop reads every frame, validates and records it, until the connection ends. While the
// client is paused it does not read, so the server's queue fills like a slow consumer's.
func (c *Client) readLoop(ctx context.Context) {
	defer close(c.readDone)
	for {
		c.mu.Lock()
		for c.paused && !c.selfClosed {
			c.cond.Wait()
		}
		c.mu.Unlock()
		typ, b, err := c.ws.Read(ctx)
		if err != nil {
			c.ended(err)
			return
		}
		if typ != websocket.MessageText {
			c.violate(c.serverIndex(), "", "", "binary frame from the server")
			c.ended(errors.New("wstest: binary frame from the server"))
			return
		}
		m := c.receive(b)
		c.mu.Lock()
		c.queue = append(c.queue, m)
		c.cond.Broadcast()
		c.mu.Unlock()
		if m.err != nil {
			c.ended(nil)
			return
		}
	}
}

// receive validates one server frame and checks the order of the connection.
func (c *Client) receive(b []byte) message {
	c.mu.Lock()
	idx := c.serverFrames
	c.serverFrames++
	c.mu.Unlock()
	var env struct {
		Op string          `json:"op"`
		D  json.RawMessage `json:"d"`
	}
	if err := Strict(b, &env); err != nil {
		c.violate(idx, "", "", "not a frame {op, d}: "+err.Error())
		return message{err: err}
	}
	if err := c.contract.ValidateServerFrame(b); err != nil {
		var fe *specschema.FrameError
		if errors.As(err, &fe) {
			c.violate(idx, env.Op, fe.Pointer, fe.Reason)
		} else {
			c.violate(idx, env.Op, "", err.Error())
		}
	}
	c.order(idx, env.Op, env.D)
	return message{op: env.Op, d: env.D}
}

// order checks the per-connection order of 04 "Delivery guarantee" item 2 and section 6.3.
func (c *Client) order(idx int, op string, d json.RawMessage) {
	if idx == 0 && op != "hello" {
		c.violate(idx, op, "/op", "first server frame is not hello")
	}
	var v struct {
		Seq     *int64 `json:"seq"`
		HeadSeq *int64 `json:"head_seq"`
	}
	_ = json.Unmarshal(d, &v)
	c.mu.Lock()
	last := c.delivered
	c.mu.Unlock()
	switch op {
	case "dispatch":
		if v.Seq == nil {
			return // ephemeral
		}
		if *v.Seq <= last {
			c.violate(idx, op, "/d/seq", fmt.Sprintf("seq %d after delivered seq %d", *v.Seq, last))
			return
		}
		c.mu.Lock()
		c.delivered = *v.Seq
		c.mu.Unlock()
	case "cursor":
		if v.Seq != nil && *v.Seq < last {
			c.violate(idx, op, "/d/seq", fmt.Sprintf("cursor %d below delivered seq %d", *v.Seq, last))
		}
	case "catchup.complete":
		if v.HeadSeq != nil && *v.HeadSeq < last {
			c.violate(idx, op, "/d/head_seq", fmt.Sprintf("head_seq %d below delivered seq %d", *v.HeadSeq, last))
		}
	}
}

// ended records the end of the read loop; a close of the server must carry a code of
// x-close-codes.
func (c *Client) ended(err error) {
	c.mu.Lock()
	self := c.selfClosed
	c.mu.Unlock()
	if err != nil {
		var ce websocket.CloseError
		if errors.As(err, &ce) && !self {
			if _, ok := c.contract.CloseCodes()[int(ce.Code)]; !ok {
				c.violate(c.serverIndex(), "", "", fmt.Sprintf("close code %d is not in x-close-codes", ce.Code))
			}
		}
	}
	c.mu.Lock()
	if err != nil {
		c.queue = append(c.queue, message{err: err})
	}
	c.closed = true
	c.cond.Broadcast()
	c.mu.Unlock()
}

// serverIndex returns the index the next server event would get.
func (c *Client) serverIndex() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.serverFrames
}

// violate records one violation; the cleanup fails the test with it.
func (c *Client) violate(idx int, op, ptr, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.violations = append(c.violations, fmt.Sprintf("frame %d op %q at %q: %s", idx, op, ptr, reason))
}

// Violations returns the violations recorded so far, for tests of the harness itself.
func (c *Client) Violations() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.violations...)
}

// Frames returns how many server frames the client received and validated and how many client
// frames it sent.
func (c *Client) Frames() (server, client int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.serverFrames, c.clientFrames
}

// Pause stops reading from the connection, as a slow consumer does; frames already read stay
// queued. The cleanup drains and validates the rest.
func (c *Client) Pause() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paused = true
}

// Unpause starts reading again after Pause.
func (c *Client) Unpause() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paused = false
	c.cond.Broadcast()
}

// finish is the cleanup: a paused client reads the rest until the server ends the connection or
// drainTimeout passes; then the connection is dropped and every violation fails the test.
func (c *Client) finish() {
	c.mu.Lock()
	wasPaused := c.paused
	c.mu.Unlock()
	if wasPaused {
		c.Unpause()
		select {
		case <-c.readDone:
		case <-time.After(drainTimeout):
		}
	}
	c.Abort()
	<-c.readDone
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t.Logf("wstest: %d server frames and %d client frames validated", c.serverFrames, c.clientFrames)
	for _, v := range c.violations {
		c.t.Errorf("G9 %s: %s", c.t.Name(), v)
	}
}

// Hello returns the hello of the connection.
func (c *Client) Hello() rest.GatewayHello { return c.hello }

// Send sends the frame {op, d}; d is encoded with encoding/json (json.RawMessage is sent as is).
func (c *Client) Send(op string, d any) error {
	b, err := json.Marshal(map[string]any{"op": op, "d": d})
	if err != nil {
		return err
	}
	return c.SendRaw(b)
}

// SendRaw sends b as one text frame; unless WithRawClientFrames was given, b must be a valid
// client frame of the contract (a violation fails the test in the cleanup, the frame is sent).
func (c *Client) SendRaw(b []byte) error {
	c.mu.Lock()
	idx := c.clientFrames
	c.clientFrames++
	c.mu.Unlock()
	if !c.opts.rawClient {
		if err := c.contract.ValidateClientFrame(b); err != nil {
			c.violate(idx, "", "", "client frame: "+err.Error())
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return c.ws.Write(ctx, websocket.MessageText, b)
}

// SendBinary sends b as one binary frame; it needs WithRawClientFrames, the contract has none.
func (c *Client) SendBinary(b []byte) error {
	if !c.opts.rawClient {
		c.violate(0, "", "", "binary client frame without WithRawClientFrames")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return c.ws.Write(ctx, websocket.MessageBinary, b)
}

// Next returns the next frame of the server; while the heartbeat loop is on (the default), its
// heartbeat_ack frames are skipped. A close of the server is a *CloseError.
func (c *Client) Next(ctx context.Context) (op string, d json.RawMessage, err error) {
	return c.next(ctx, c.opts.heartbeat)
}

// next returns the next frame, skipping heartbeat_ack when skipAcks.
func (c *Client) next(ctx context.Context, skipAcks bool) (string, json.RawMessage, error) {
	stop := context.AfterFunc(ctx, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.cond.Broadcast()
	})
	defer stop()
	c.mu.Lock()
	defer c.mu.Unlock()
	for {
		for len(c.queue) == 0 && !c.closed && ctx.Err() == nil {
			c.cond.Wait()
		}
		if len(c.queue) == 0 {
			if err := ctx.Err(); err != nil {
				return "", nil, err
			}
			return "", nil, errors.New("wstest: connection ended")
		}
		m := c.queue[0]
		if m.err != nil {
			// The end stays queued, so every later call sees it too.
			return "", nil, closeOf(m.err)
		}
		c.queue = c.queue[1:]
		if skipAcks && m.op == "heartbeat_ack" {
			continue
		}
		return m.op, m.d, nil
	}
}

// closeOf turns the read error of a close into a *CloseError.
func closeOf(err error) error {
	var ce websocket.CloseError
	if errors.As(err, &ce) {
		return &CloseError{Code: int(ce.Code), Reason: ce.Reason}
	}
	return err
}

// ExpectClose reads until the server closes and returns code and reason; frames before the close
// are discarded. Errors: ctx ends first, or the connection ends without a close frame.
func (c *Client) ExpectClose(ctx context.Context) (code int, reason string, err error) {
	for {
		_, _, err := c.next(ctx, true)
		if err == nil {
			continue
		}
		var ce *CloseError
		if errors.As(err, &ce) {
			return ce.Code, ce.Reason, nil
		}
		return 0, "", err
	}
}

// IdentifyParams steer one identify. Exactly what is set is sent; Extra adds members as they
// are (also unknown ones, for negative tests).
type IdentifyParams struct {
	AccessToken, APIToken string
	// Proof returns the DPoP proof for the nonce of hello; nil sends none.
	Proof func(nonce string) string
	// DeviceID is the device of a person.
	DeviceID string
	// ProtocolVersion is sent as is; 0 means 1.
	ProtocolVersion int
	Cursor          *int64
	SyncEpoch       string
	Consumer        *string
	Extra           map[string]any
}

// body returns the identify body of p for the nonce.
func (p IdentifyParams) body(nonce string) map[string]any {
	v := p.ProtocolVersion
	if v == 0 {
		v = 1
	}
	d := map[string]any{"protocol_version": v}
	set := func(k, s string) {
		if s != "" {
			d[k] = s
		}
	}
	set("access_token", p.AccessToken)
	set("api_token", p.APIToken)
	set("device_id", p.DeviceID)
	set("sync_epoch", p.SyncEpoch)
	if p.Proof != nil {
		d["dpop"] = p.Proof(nonce)
	}
	if p.Cursor != nil {
		d["cursor"] = *p.Cursor
	}
	if p.Consumer != nil {
		d["consumer"] = *p.Consumer
	}
	for k, x := range p.Extra {
		d[k] = x
	}
	return d
}

// Identify sends identify and waits for ready, which it decodes strictly; then it starts the
// heartbeat loop unless WithoutHeartbeat was given. A close of the server is a *CloseError.
func (c *Client) Identify(ctx context.Context, p IdentifyParams) (rest.GatewayReady, error) {
	if err := c.Send("identify", p.body(c.hello.DpopNonce)); err != nil {
		return rest.GatewayReady{}, err
	}
	op, d, err := c.next(ctx, true)
	if err != nil {
		return rest.GatewayReady{}, err
	}
	if op != "ready" {
		return rest.GatewayReady{}, fmt.Errorf("wstest: got %q, want ready", op)
	}
	var r rest.GatewayReady
	if err := Strict(d, &r); err != nil {
		return rest.GatewayReady{}, err
	}
	c.setSeq(r.HeadSeq)
	c.startHeartbeat()
	return r, nil
}

// ResumeParams steer one resume.
type ResumeParams struct {
	SessionID             string
	AccessToken, APIToken string
	Proof                 func(nonce string) string
	Cursor                int64
	SyncEpoch             string
}

// Resume sends resume and waits for resumed, which it decodes strictly; then it starts the
// heartbeat loop unless WithoutHeartbeat was given. A close of the server is a *CloseError.
func (c *Client) Resume(ctx context.Context, p ResumeParams) (rest.GatewayResumed, error) {
	d := map[string]any{"gateway_session_id": p.SessionID, "cursor": p.Cursor}
	if p.AccessToken != "" {
		d["access_token"] = p.AccessToken
	}
	if p.APIToken != "" {
		d["api_token"] = p.APIToken
	}
	if p.Proof != nil {
		d["dpop"] = p.Proof(c.hello.DpopNonce)
	}
	if p.SyncEpoch != "" {
		d["sync_epoch"] = p.SyncEpoch
	}
	if err := c.Send("resume", d); err != nil {
		return rest.GatewayResumed{}, err
	}
	op, raw, err := c.next(ctx, true)
	if err != nil {
		return rest.GatewayResumed{}, err
	}
	if op != "resumed" {
		return rest.GatewayResumed{}, fmt.Errorf("wstest: got %q, want resumed", op)
	}
	var r rest.GatewayResumed
	if err := Strict(raw, &r); err != nil {
		return rest.GatewayResumed{}, err
	}
	c.setSeq(r.HeadSeq)
	c.startHeartbeat()
	return r, nil
}

// setSeq records the last seq the heartbeats report.
func (c *Client) setSeq(seq int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastSeq = seq
}

// FirstHeartbeatDelay returns the delay of the first heartbeat for the interval and the random
// share r in [0, 1): r times the interval (04 section 6.3 step 5).
func FirstHeartbeatDelay(interval time.Duration, r float64) time.Duration {
	return time.Duration(r * float64(interval))
}

// startHeartbeat starts the heartbeat loop once: the first heartbeat after a random share of
// heartbeat_interval_ms, then one per interval.
func (c *Client) startHeartbeat() {
	if !c.opts.heartbeat {
		return
	}
	c.once.Do(func() {
		interval := time.Duration(c.hello.HeartbeatIntervalMs) * time.Millisecond
		first := FirstHeartbeatDelay(interval, c.opts.random())
		if c.opts.onFirstDelay != nil {
			c.opts.onFirstDelay(first)
		}
		go func() {
			t := time.NewTimer(first)
			defer t.Stop()
			for {
				select {
				case <-c.stop:
					return
				case <-t.C:
				}
				c.mu.Lock()
				seq := c.lastSeq
				c.mu.Unlock()
				if err := c.Send("heartbeat", map[string]any{"last_seq": seq}); err != nil {
					return
				}
				t.Reset(interval)
			}
		}()
	})
}

// StopHeartbeat ends the heartbeat loop.
func (c *Client) StopHeartbeat() {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.stop:
	default:
		close(c.stop)
	}
}

// markSelfClosed records that the client ends the connection itself.
func (c *Client) markSelfClosed() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.selfClosed = true
	c.cond.Broadcast()
}

// Close closes the connection normally (1000).
func (c *Client) Close() {
	c.StopHeartbeat()
	c.markSelfClosed()
	_ = c.ws.Close(websocket.StatusNormalClosure, "")
	c.cancel()
}

// Abort drops the connection without a close frame, like a lost network.
func (c *Client) Abort() {
	c.StopHeartbeat()
	c.markSelfClosed()
	_ = c.ws.CloseNow()
	c.cancel()
}

// Strict decodes b into v and refuses unknown members and trailing data.
func Strict(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("wstest: trailing data")
	}
	return nil
}
