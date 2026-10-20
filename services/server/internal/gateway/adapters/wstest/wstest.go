// SPDX-License-Identifier: Apache-2.0

// Package wstest is a Go client of the WebSocket gateway for tests of this and later tickets
// (M1-65 to M1-67): Dial connects and reads hello; Identify and Resume open a session and start
// the heartbeat loop of 04 section 6.3 step 5, whose first heartbeat waits a uniformly random
// share of heartbeat_interval_ms; Send, Next and ExpectClose drive the connection frame by frame.
// It decodes the server frames into the generated Gateway* types of api/rest and refuses unknown
// members. Only tests import it.
//
// Example:
//
//	c, err := wstest.Dial(ctx, "http://127.0.0.1:41234", wstest.WithOrigin("https://chat.example.com"))
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
	gosync "sync"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// Path is the path of the gateway below the base URL.
const Path = "/api/v1/gateway"

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

// message is one frame read from the server, or the end of the connection.
type message struct {
	op  string
	d   json.RawMessage
	err error
}

// Client is one gateway connection of a test.
type Client struct {
	ws      *websocket.Conn
	opts    options
	hello   rest.GatewayHello
	in      chan message
	cancel  context.CancelFunc
	mu      gosync.Mutex
	lastSeq int64
	stop    chan struct{}
	once    gosync.Once
}

// Dial connects to the gateway at baseURL (http or https, for example the URL of a test server)
// and reads hello. Errors: the upgrade fails (a 403 for a foreign Origin, for example), or the
// first frame is no valid hello.
func Dial(ctx context.Context, baseURL string, opts ...Option) (*Client, error) {
	o := options{header: http.Header{}, heartbeat: true, random: rand.Float64}
	for _, f := range opts {
		f(&o)
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
	c := &Client{ws: ws, opts: o, in: make(chan message, 64), cancel: cancel, stop: make(chan struct{})}
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
// Errors: no answer at all.
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

// wsURL returns the URL of the gateway below baseURL. The scheme stays http or https:
// websocket.Dial takes both and maps ws and wss to them itself, so the client never spells out a
// cleartext WebSocket URL.
func wsURL(baseURL string) string {
	return baseURL + Path
}

// readLoop reads every frame into in until the connection ends.
func (c *Client) readLoop(ctx context.Context) {
	defer close(c.in)
	for {
		typ, b, err := c.ws.Read(ctx)
		if err != nil {
			c.in <- message{err: err}
			return
		}
		if typ != websocket.MessageText {
			c.in <- message{err: errors.New("wstest: binary frame from the server")}
			return
		}
		var env struct {
			Op string          `json:"op"`
			D  json.RawMessage `json:"d"`
		}
		if err := Strict(b, &env); err != nil {
			c.in <- message{err: err}
			return
		}
		c.in <- message{op: env.Op, d: env.D}
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

// SendRaw sends b as one text frame.
func (c *Client) SendRaw(b []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return c.ws.Write(ctx, websocket.MessageText, b)
}

// SendBinary sends b as one binary frame.
func (c *Client) SendBinary(b []byte) error {
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
	for {
		select {
		case <-ctx.Done():
			return "", nil, ctx.Err()
		case m, ok := <-c.in:
			if !ok {
				return "", nil, errors.New("wstest: connection ended")
			}
			if m.err != nil {
				return "", nil, closeOf(m.err)
			}
			if skipAcks && m.op == "heartbeat_ack" {
				continue
			}
			return m.op, m.d, nil
		}
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
	select {
	case <-c.stop:
	default:
		close(c.stop)
	}
}

// Close closes the connection normally (1000).
func (c *Client) Close() {
	c.StopHeartbeat()
	_ = c.ws.Close(websocket.StatusNormalClosure, "")
	c.cancel()
}

// Abort drops the connection without a close frame, like a lost network.
func (c *Client) Abort() {
	c.StopHeartbeat()
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
