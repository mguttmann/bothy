// SPDX-License-Identifier: Apache-2.0

// Package controltest is a fake server of the control connection for the tests of the host
// service (M2-43): TLS from net/http/httptest on loopback, whose certificate pool the client
// trusts through RootCAs; every connection is handed to the test, which scripts hello, the
// answers and the close codes and reads the frames of the client. Only tests import it.
//
// Example:
//
//	srv := controltest.New(t)
//	c := srv.Accept(ctx)
//	c.Send("hello", controltest.Hello(20000, 3))
//	op, d := c.Next(ctx) // identify
package controltest

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	gosync "sync"
	"sync/atomic"
	"testing"

	"github.com/coder/websocket"
)

// Path is the path of the control connection.
const Path = "/api/v1/hosts/connect"

// Server is the fake server.
type Server struct {
	t     testing.TB
	srv   *httptest.Server
	conns chan *Conn
	count atomic.Int32
	// URL is the https URL of the server; Pool trusts its certificate.
	URL  string
	Pool *x509.CertPool
}

// New starts the fake server on 127.0.0.1 with TLS; it stops with the test.
func New(t testing.TB) *Server {
	s := &Server{t: t, conns: make(chan *Conn, 16)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+Path, s.serve)
	s.srv = httptest.NewTLSServer(mux)
	s.URL = s.srv.URL
	s.Pool = x509.NewCertPool()
	s.Pool.AddCert(s.srv.Certificate())
	t.Cleanup(s.srv.Close)
	return s
}

// Close stops the server: every later dial fails (the server is down).
func (s *Server) Close() { s.srv.Close() }

// Count returns the number of accepted connections.
func (s *Server) Count() int { return int(s.count.Load()) }

// serve upgrades one connection and hands it to the test until it ends.
func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return
	}
	s.count.Add(1)
	c := &Conn{ws: ws, frames: make(chan Frame, 64), done: make(chan struct{}), Remote: r.RemoteAddr}
	go c.read()
	s.conns <- c
	<-c.done
}

// Accept returns the next connection of the client; it fails the test when ctx ends first.
func (s *Server) Accept(ctx context.Context) *Conn {
	s.t.Helper()
	select {
	case c := <-s.conns:
		return c
	case <-ctx.Done():
		s.t.Fatalf("controltest: no connection: %v", ctx.Err())
		return nil
	}
}

// Frame is one frame of the client, or the end of the connection (Err).
type Frame struct {
	Op  string
	D   json.RawMessage
	Raw []byte
	// Binary marks a binary frame.
	Binary bool
	// Err is the end of the connection; Code is the close code of the client, -1 without one.
	Err  error
	Code int
}

// Conn is one connection of the client.
type Conn struct {
	ws     *websocket.Conn
	frames chan Frame
	done   chan struct{}
	once   gosync.Once
	// Remote is the address of the client.
	Remote string
}

// read reads the frames of the client until the connection ends.
func (c *Conn) read() {
	defer c.once.Do(func() { close(c.done) })
	for {
		typ, b, err := c.ws.Read(context.Background())
		if err != nil {
			code := -1
			var ce websocket.CloseError
			if errors.As(err, &ce) {
				code = int(ce.Code)
			}
			c.frames <- Frame{Err: err, Code: code}
			return
		}
		f := Frame{Raw: b, Binary: typ != websocket.MessageText}
		var env struct {
			Op string          `json:"op"`
			D  json.RawMessage `json:"d"`
		}
		if json.Unmarshal(b, &env) == nil {
			f.Op, f.D = env.Op, env.D
		}
		c.frames <- f
	}
}

// Next returns the next frame of the client (an ended connection is a frame with Err).
func (c *Conn) Next(ctx context.Context) (Frame, error) {
	select {
	case f := <-c.frames:
		return f, nil
	case <-ctx.Done():
		return Frame{}, ctx.Err()
	}
}

// Send sends the frame {op, d}.
func (c *Conn) Send(ctx context.Context, op string, d any) error {
	b, err := json.Marshal(struct {
		Op string `json:"op"`
		D  any    `json:"d"`
	}{op, d})
	if err != nil {
		return err
	}
	return c.SendRaw(ctx, b)
}

// SendRaw sends b as a text frame.
func (c *Conn) SendRaw(ctx context.Context, b []byte) error {
	return c.ws.Write(ctx, websocket.MessageText, b)
}

// Close closes the connection with code and reason.
func (c *Conn) Close(code int, reason string) {
	_ = c.ws.Close(websocket.StatusCode(code), reason)
}

// Done is closed when the connection ended.
func (c *Conn) Done() <-chan struct{} { return c.done }

// The server values Hello sends: the defaults of auth.clock_leeway, terminal.idle_timeout and
// terminal.detach_grace in milliseconds, as the server's recording carries them.
const (
	HelloLeewayMs int64 = 30000
	HelloIdleMs   int64 = 1800000
	HelloDetachMs int64 = 900000
)

// Hello returns the body of hello with the challenge of the recording, heartbeat_interval_ms
// intervalMs, heartbeat_miss_limit missLimit, the server values HelloLeewayMs, HelloIdleMs and
// HelloDetachMs and one key of purpose terminal.
func Hello(intervalMs int64, missLimit int) map[string]any {
	return HelloWith(intervalMs, missLimit, HelloLeewayMs, HelloIdleMs, HelloDetachMs)
}

// HelloWith is Hello with the server values clock_leeway_ms leewayMs, idle_timeout_ms idleMs and
// detach_grace_ms detachMs (M2-45 Design note 2a), for tests that set the windows of a session
// through hello.
func HelloWith(intervalMs int64, missLimit int, leewayMs, idleMs, detachMs int64) map[string]any {
	return map[string]any{
		"protocol_versions": []int{1}, "challenge": "SGVsbG8gZnJvbSB0aGUgcmVjb3JkaW5nIG9mIE0yLTQzIQ",
		"server_time": "2026-10-14T09:00:00.000Z", "heartbeat_interval_ms": intervalMs, "heartbeat_miss_limit": missLimit,
		"clock_leeway_ms": leewayMs, "idle_timeout_ms": idleMs, "detach_grace_ms": detachMs,
		"jwks": map[string]any{"keys": []any{Key("terminal-key-1")}},
	}
}

// Key returns a JWK of purpose terminal with kid.
func Key(kid string) map[string]any {
	return map[string]any{"kty": "OKP", "crv": "Ed25519", "x": "11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo", "kid": kid, "alg": "EdDSA", "use": "sig"}
}

// Ready returns the body of ready for host with llm credentials decl.
func Ready(host, decl string) map[string]any {
	return map[string]any{"host_id": host, "state": "active", "llm_credentials": decl}
}

// Expect reads the next frame and fails t unless its op is op; it returns the body.
func (c *Conn) Expect(t testing.TB, ctx context.Context, op string) json.RawMessage {
	t.Helper()
	f, err := c.Next(ctx)
	if err != nil {
		t.Fatalf("controltest: waiting for %s: %v", op, err)
	}
	if f.Err != nil || f.Op != op {
		t.Fatalf("controltest: got %s (%v, code %d), want %s", f.Op, f.Err, f.Code, op)
	}
	return f.D
}

// Strict decodes b into v and refuses unknown members.
func Strict(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("controltest: %w", err)
	}
	return nil
}
