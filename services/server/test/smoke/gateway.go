// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// Gateway frame names the scenario reads (docs/api/asyncapi.yaml).
const (
	opHello           = "hello"
	opReady           = "ready"
	opDispatch        = "dispatch"
	opCursor          = "cursor"
	opCatchupComplete = "catchup.complete"
	opResyncRequired  = "resync_required"
	opHeartbeatAck    = "heartbeat_ack"
)

// frameBuffer is how many frames a WSGateway holds before its reader waits.
const frameBuffer = 4096

// maxFrameBytes bounds one frame of the server; ready with its snapshot is the largest.
const maxFrameBytes = 16 << 20

// writeTimeout bounds one frame write of the probe.
const writeTimeout = 10 * time.Second

// Frame is one frame of the server {op, d} with the time the reader received it.
type Frame struct {
	Op       string
	D        json.RawMessage
	Received time.Time
}

// Gateway is one connection to the WebSocket gateway (docs/spec/04-sync-protocol.md 6).
type Gateway interface {
	// Dial opens the connection without an Origin header (SEC-CORS-3) and returns hello.
	Dial(ctx context.Context) (rest.GatewayHello, error)
	// Identify sends identify and returns ready; afterwards the heartbeat loop runs.
	Identify(ctx context.Context, id rest.GatewayIdentify) (rest.GatewayReady, error)
	// Next returns the next frame other than heartbeat_ack; at the end of the connection or of
	// ctx it returns an error.
	Next(ctx context.Context) (Frame, error)
	// Close closes the connection with code.
	Close(code int) error
	// CloseCode returns the close code the server sent, 0 while the connection is open or when the
	// probe closed it.
	CloseCode() int
}

// WSGateway implements Gateway with github.com/coder/websocket. A reader goroutine queues the
// frames; after Identify a heartbeat loop sends heartbeat {last_seq} every heartbeat_interval_ms of
// hello, the first after a random share of it (04 6.3 step 5).
type WSGateway struct {
	url string

	conn     *websocket.Conn
	frames   chan Frame
	done     chan struct{}
	stop     chan struct{}
	stopOnce sync.Once
	hello    rest.GatewayHello
	lastSeq  atomic.Int64

	mu         sync.Mutex
	closeCode  int
	selfClosed bool
}

// Compile-time check: WSGateway is the Gateway of the command.
var _ Gateway = (*WSGateway)(nil)

// NewWSGateway returns an unconnected gateway client for baseURL (http://127.0.0.1:8080 becomes
// ws://127.0.0.1:8080/api/v1/gateway).
func NewWSGateway(baseURL string) *WSGateway {
	u := strings.Replace(strings.Replace(baseURL, "https://", "wss://", 1), "http://", "ws://", 1)
	return &WSGateway{url: strings.TrimSuffix(u, "/") + apiPrefix + "/gateway", stop: make(chan struct{})}
}

// Dial implements Gateway. The error reports a failed upgrade or a first frame other than hello.
func (g *WSGateway) Dial(ctx context.Context) (rest.GatewayHello, error) {
	hc := &http.Client{Transport: &http.Transport{Proxy: nil}}
	conn, resp, err := websocket.Dial(ctx, g.url, &websocket.DialOptions{HTTPClient: hc, CompressionMode: websocket.CompressionDisabled})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return rest.GatewayHello{}, fmt.Errorf("smoke: gateway upgrade: %w", err)
	}
	conn.SetReadLimit(maxFrameBytes)
	g.conn, g.frames, g.done = conn, make(chan Frame, frameBuffer), make(chan struct{})
	go g.read()
	f, err := g.Next(ctx)
	if err != nil {
		return rest.GatewayHello{}, err
	}
	if f.Op != opHello {
		return rest.GatewayHello{}, fmt.Errorf("smoke: first gateway frame %q, want hello", f.Op)
	}
	if err := json.Unmarshal(f.D, &g.hello); err != nil {
		return rest.GatewayHello{}, fmt.Errorf("smoke: decode hello: %w", err)
	}
	return g.hello, nil
}

// Identify implements Gateway. The error reports a write failure, a close of the server (its code
// in the message) or another frame than ready.
func (g *WSGateway) Identify(ctx context.Context, id rest.GatewayIdentify) (rest.GatewayReady, error) {
	if err := g.send("identify", id); err != nil {
		return rest.GatewayReady{}, err
	}
	f, err := g.Next(ctx)
	if err != nil {
		return rest.GatewayReady{}, err
	}
	if f.Op != opReady {
		return rest.GatewayReady{}, fmt.Errorf("smoke: answer to identify %q, want ready", f.Op)
	}
	var ready rest.GatewayReady
	if err := json.Unmarshal(f.D, &ready); err != nil {
		return rest.GatewayReady{}, fmt.Errorf("smoke: decode ready: %w", err)
	}
	g.lastSeq.Store(ready.HeadSeq)
	go g.heartbeat(time.Duration(g.hello.HeartbeatIntervalMs) * time.Millisecond)
	return ready, nil
}

// send writes the frame {op, d}.
func (g *WSGateway) send(op string, d any) error {
	b, err := json.Marshal(map[string]any{"op": op, "d": d})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	return g.conn.Write(ctx, websocket.MessageText, b)
}

// read queues every frame of the server except heartbeat_ack until the connection ends, and keeps
// the close code of the server.
func (g *WSGateway) read() {
	defer close(g.done)
	defer close(g.frames)
	for {
		typ, b, err := g.conn.Read(context.Background())
		if err != nil {
			g.mu.Lock()
			if !g.selfClosed {
				g.closeCode = int(websocket.CloseStatus(err))
			}
			g.mu.Unlock()
			return
		}
		var f struct {
			Op string          `json:"op"`
			D  json.RawMessage `json:"d"`
		}
		if typ != websocket.MessageText || json.Unmarshal(b, &f) != nil {
			f.Op = "invalid"
		}
		if f.Op == opHeartbeatAck {
			continue
		}
		if f.Op == opDispatch {
			var env struct {
				Seq int64 `json:"seq"`
			}
			if json.Unmarshal(f.D, &env) == nil && env.Seq > g.lastSeq.Load() {
				g.lastSeq.Store(env.Seq)
			}
		}
		g.frames <- Frame{Op: f.Op, D: f.D, Received: time.Now()}
	}
}

// heartbeat sends heartbeats until Close or the end of the connection.
func (g *WSGateway) heartbeat(interval time.Duration) {
	if interval <= 0 {
		return
	}
	t := time.NewTimer(time.Duration(rand.Float64() * float64(interval))) // #nosec G404 -- jitter, not a secret
	defer t.Stop()
	for {
		select {
		case <-g.stop:
			return
		case <-g.done:
			return
		case <-t.C:
		}
		if g.send("heartbeat", map[string]int64{"last_seq": g.lastSeq.Load()}) != nil {
			return
		}
		t.Reset(interval)
	}
}

// Next implements Gateway.
func (g *WSGateway) Next(ctx context.Context) (Frame, error) {
	select {
	case <-ctx.Done():
		return Frame{}, ctx.Err()
	case f, ok := <-g.frames:
		if !ok {
			return Frame{}, fmt.Errorf("smoke: gateway connection closed with %d", g.CloseCode())
		}
		return f, nil
	}
}

// Close implements Gateway.
func (g *WSGateway) Close(code int) error {
	g.stopOnce.Do(func() { close(g.stop) })
	if g.conn == nil {
		return errors.New("smoke: gateway not dialled")
	}
	g.mu.Lock()
	g.selfClosed = true
	g.mu.Unlock()
	return g.conn.Close(websocket.StatusCode(code), "")
}

// CloseCode implements Gateway.
func (g *WSGateway) CloseCode() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.closeCode
}
