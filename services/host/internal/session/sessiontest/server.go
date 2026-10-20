// SPDX-License-Identifier: Apache-2.0

package sessiontest

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/coder/websocket"
)

// Paths of the fake server (asyncapi channels hostControl and hostData).
const (
	ControlPath = "/api/v1/hosts/connect"
	DataPrefix  = "/api/v1/hosts/sessions/"
)

// Values are the server values of hello (24 17): heartbeat and the three values of M2-45.
type Values struct {
	HeartbeatIntervalMs, ClockLeewayMs, IdleTimeoutMs, DetachGraceMs int64
	HeartbeatMissLimit                                               int
}

// DefaultValues are the defaults of 24 17 in milliseconds.
var DefaultValues = Values{HeartbeatIntervalMs: 20000, HeartbeatMissLimit: 3, ClockLeewayMs: 30000, IdleTimeoutMs: 1800000, DetachGraceMs: 900000}

// Server is the fake server on 127.0.0.1 port 0 with TLS: the control endpoint and the data
// endpoint. It signs nothing itself; the test signs tokens with Signer and puts its JWK into
// hello.
type Server struct {
	t     testing.TB
	srv   *httptest.Server
	ctrl  chan *Conn
	mu    sync.Mutex
	data  map[string][]*Conn
	cond  *sync.Cond
	hello func() map[string]any
	// URL is the https URL of the server; Pool trusts its certificate.
	URL  string
	Pool *x509.CertPool
}

// NewServer starts the fake server; hello returns the body of every hello (control and data).
func NewServer(t testing.TB, hello func() map[string]any) *Server {
	s := &Server{t: t, ctrl: make(chan *Conn, 16), data: map[string][]*Conn{}, hello: hello}
	s.cond = sync.NewCond(&s.mu)
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+ControlPath, s.serveControl)
	mux.HandleFunc("GET "+DataPrefix+"{id}/stream", s.serveData)
	s.srv = httptest.NewTLSServer(mux)
	s.URL = s.srv.URL
	s.Pool = x509.NewCertPool()
	s.Pool.AddCert(s.srv.Certificate())
	t.Cleanup(s.srv.Close)
	return s
}

// Hello returns a hello body with keys (JWK objects) and v.
func Hello(keys []map[string]any, v Values) map[string]any {
	ks := make([]any, len(keys))
	for i, k := range keys {
		ks[i] = k
	}
	return map[string]any{
		"protocol_versions": []int{1}, "challenge": "SGVsbG8gZnJvbSB0aGUgZmFrZSBzZXJ2ZXIgb2YgTTItNDU", "server_time": "2026-10-20T09:00:00.000Z",
		"heartbeat_interval_ms": v.HeartbeatIntervalMs, "heartbeat_miss_limit": v.HeartbeatMissLimit, "jwks": map[string]any{"keys": ks},
		"clock_leeway_ms": v.ClockLeewayMs, "idle_timeout_ms": v.IdleTimeoutMs, "detach_grace_ms": v.DetachGraceMs,
	}
}

// Conn is one connection of the host service (control or data).
type Conn struct {
	ws     *websocket.Conn
	frames chan Frame
	done   chan struct{}
	once   sync.Once
	// Identify is the body of identify (data connections).
	Identify map[string]any
	mu       sync.Mutex
	code     int
	reason   string
}

// Frame is one frame of the host service, or the end (Err with the close Code and Reason, -1 for
// a lost connection).
type Frame struct {
	Op     string
	D      json.RawMessage
	Binary bool
	Raw    []byte
	Err    error
	Code   int
	Reason string
}

func newConn(ws *websocket.Conn) *Conn {
	c := &Conn{ws: ws, frames: make(chan Frame, 256), done: make(chan struct{}), code: -1}
	return c
}

func (c *Conn) read() {
	defer c.once.Do(func() { close(c.done) })
	for {
		typ, b, err := c.ws.Read(context.Background())
		if err != nil {
			code, reason := -1, ""
			var ce websocket.CloseError
			if errors.As(err, &ce) {
				code, reason = int(ce.Code), ce.Reason
			}
			c.mu.Lock()
			c.code, c.reason = code, reason
			c.mu.Unlock()
			c.frames <- Frame{Err: err, Code: code, Reason: reason}
			return
		}
		f := Frame{Raw: b, Binary: typ == websocket.MessageBinary}
		if !f.Binary {
			var env struct {
				Op string          `json:"op"`
				D  json.RawMessage `json:"d"`
			}
			if json.Unmarshal(b, &env) == nil {
				f.Op, f.D = env.Op, env.D
			}
		}
		c.frames <- f
	}
}

func (s *Server) serveControl(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return
	}
	ws.SetReadLimit(-1)
	c := newConn(ws)
	go c.read()
	s.ctrl <- c
	<-c.done
}

func (s *Server) serveData(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return
	}
	ws.SetReadLimit(-1)
	c := newConn(ws)
	b, _ := json.Marshal(map[string]any{"op": "hello", "d": s.hello()})
	if err := ws.Write(r.Context(), websocket.MessageText, b); err != nil {
		return
	}
	typ, msg, err := ws.Read(r.Context())
	if err != nil || typ != websocket.MessageText {
		_ = ws.CloseNow()
		return
	}
	var env struct {
		Op string         `json:"op"`
		D  map[string]any `json:"d"`
	}
	if json.Unmarshal(msg, &env) != nil || env.Op != "identify" || env.D["session_id"] != id {
		_ = ws.Close(4002, "")
		return
	}
	c.Identify = env.D
	go c.read() //nolint:gosec // G118: the reader lives as long as the connection, not as the request
	s.mu.Lock()
	s.data[id] = append(s.data[id], c)
	s.cond.Broadcast()
	s.mu.Unlock()
	<-c.done
}

// AcceptControl returns the next control connection; it fails the test when ctx ends first.
func (s *Server) AcceptControl(ctx context.Context) *Conn {
	s.t.Helper()
	select {
	case c := <-s.ctrl:
		return c
	case <-ctx.Done():
		s.t.Fatalf("sessiontest: no control connection: %v", ctx.Err())
		return nil
	}
}

// Data waits for the n-th (from 1) data connection of session id.
func (s *Server) Data(ctx context.Context, id string, n int) *Conn {
	s.t.Helper()
	stop := context.AfterFunc(ctx, func() {
		s.mu.Lock()
		s.cond.Broadcast()
		s.mu.Unlock()
	})
	defer stop()
	s.mu.Lock()
	defer s.mu.Unlock()
	for len(s.data[id]) < n {
		if ctx.Err() != nil {
			s.t.Fatalf("sessiontest: no data connection %d of the session", n)
			return nil
		}
		s.cond.Wait()
	}
	return s.data[id][n-1]
}

// DataCount returns the number of data connections of session id so far.
func (s *Server) DataCount(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.data[id])
}

// Next returns the next frame of the connection.
func (c *Conn) Next(ctx context.Context) (Frame, error) {
	select {
	case f := <-c.frames:
		return f, nil
	case <-ctx.Done():
		return Frame{}, ctx.Err()
	}
}

// Send sends the text frame {op, d}.
func (c *Conn) Send(ctx context.Context, op string, d any) error {
	b, err := json.Marshal(map[string]any{"op": op, "d": d})
	if err != nil {
		return err
	}
	return c.ws.Write(ctx, websocket.MessageText, b)
}

// SendBinary sends one binary message.
func (c *Conn) SendBinary(ctx context.Context, b []byte) error {
	return c.ws.Write(ctx, websocket.MessageBinary, b)
}

// Close closes the connection with code and reason (the server ends it).
func (c *Conn) Close(code int, reason string) { _ = c.ws.Close(websocket.StatusCode(code), reason) }

// Drop ends the connection without a close frame (a lost connection).
func (c *Conn) Drop() { _ = c.ws.CloseNow() }

// Done is closed when the connection ended.
func (c *Conn) Done() <-chan struct{} { return c.done }

// Closed returns the close code and reason of the host service once the connection ended
// (-1 for a lost connection) and whether it ended.
func (c *Conn) Closed() (int, string, bool) {
	select {
	case <-c.done:
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.code, c.reason, true
	default:
		return 0, "", false
	}
}

// NextBinary returns the next binary message, skipping nothing: a text frame or the end is an
// error.
func (c *Conn) NextBinary(ctx context.Context) ([]byte, error) {
	f, err := c.Next(ctx)
	if err != nil {
		return nil, err
	}
	if f.Err != nil {
		return nil, f.Err
	}
	if !f.Binary {
		return nil, errors.New("sessiontest: text frame on a data connection: " + strings.TrimSpace(f.Op))
	}
	return f.Raw, nil
}
