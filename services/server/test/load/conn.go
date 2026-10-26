// SPDX-License-Identifier: Apache-2.0

package load

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/test/smoke"
)

// Bounds of a gateway connection: the largest frame (ready with its snapshot) and one write.
const (
	maxFrameBytes = 16 << 20
	writeTimeout  = 10 * time.Second
)

// Dispatch is what an observer keeps of one message.created dispatch: the client_msg_id of the
// message, occurred_at of its event and when the frame arrived.
type Dispatch struct {
	ClientMsgID string
	OccurredAt  time.Time
	Received    time.Time
}

// Conn is one gateway connection of a person, identified with its access token. An observer
// decodes its dispatches and keeps the message.created ones; every other connection reads and
// drops its frames. A heartbeat loop answers the server's interval. It is safe for concurrent use.
type Conn struct {
	ws       *websocket.Conn
	observer bool
	lastSeq  atomic.Int64
	stop     chan struct{}
	done     chan struct{}
	once     sync.Once

	mu         sync.Mutex
	dispatches []Dispatch
	err        error
	closed     bool
}

// Connect opens a gateway connection for p from its client address and identifies with p's
// access token and a DPoP proof for the nonce of hello; observer decides whether the connection
// keeps its message.created dispatches. The error names the step that failed, never a token.
func (d *Driver) Connect(ctx context.Context, p *Person, observer bool) (*Conn, error) {
	u := strings.Replace(strings.Replace(d.base, "https://", "wss://", 1), "http://", "ws://", 1) + apiPrefix + "/gateway"
	// The client of the requests has a total timeout, which would end a long-lived connection;
	// the upgrade shares only its transport.
	h := http.Header{}
	h.Set("X-Forwarded-For", p.Address)
	ws, resp, err := websocket.Dial(ctx, u, &websocket.DialOptions{HTTPClient: &http.Client{Transport: d.hc.Transport}, HTTPHeader: h, CompressionMode: websocket.CompressionDisabled})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("load: gateway upgrade: %w", err)
	}
	ws.SetReadLimit(maxFrameBytes)
	c := &Conn{ws: ws, observer: observer, stop: make(chan struct{}), done: make(chan struct{})}
	if err := c.identify(ctx, d, p); err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, "")
		return nil, err
	}
	return c, nil
}

// frame is the envelope of a gateway frame.
type frame struct {
	Op string          `json:"op"`
	D  json.RawMessage `json:"d"`
}

// readFrame reads one text frame.
func (c *Conn) readFrame(ctx context.Context) (frame, error) {
	typ, b, err := c.ws.Read(ctx)
	if err != nil {
		return frame{}, err
	}
	var f frame
	if typ != websocket.MessageText || json.Unmarshal(b, &f) != nil {
		return frame{}, errors.New("load: the gateway sent a frame that is no JSON text")
	}
	return f, nil
}

// identify reads hello, sends identify and waits for ready, then starts the reader and the
// heartbeat loop.
func (c *Conn) identify(ctx context.Context, d *Driver, p *Person) error {
	f, err := c.readFrame(ctx)
	if err != nil {
		return fmt.Errorf("load: gateway hello: %w", err)
	}
	var hello rest.GatewayHello
	if f.Op != "hello" || json.Unmarshal(f.D, &hello) != nil || len(hello.ProtocolVersions) == 0 {
		return fmt.Errorf("load: first gateway frame %q is no usable hello", f.Op)
	}
	htu, err := smoke.GatewayHTU(d.public)
	if err != nil {
		return err
	}
	access, _ := p.tokens()
	proof, err := smoke.NewProof(p.key, http.MethodGet, htu, hello.DpopNonce, access, time.Now())
	if err != nil {
		return err
	}
	device := p.DeviceID
	if err := c.send("identify", rest.GatewayIdentify{ProtocolVersion: slices.Max(hello.ProtocolVersions), AccessToken: &access, Dpop: &proof,
		DeviceId: &device, Client: &rest.GatewayClientInfo{Platform: rest.DevicePlatformMacos, Version: "1.0.0"}}); err != nil {
		return err
	}
	for {
		f, err := c.readFrame(ctx)
		if err != nil {
			return fmt.Errorf("load: gateway identify: %w", err)
		}
		if f.Op == "heartbeat_ack" {
			continue
		}
		if f.Op != "ready" {
			return fmt.Errorf("load: answer to identify %q, want ready", f.Op)
		}
		var ready rest.GatewayReady
		if err := json.Unmarshal(f.D, &ready); err != nil {
			return fmt.Errorf("load: decode ready: %w", err)
		}
		c.lastSeq.Store(ready.HeadSeq)
		break
	}
	go c.read() //nolint:gosec // G118: the reader lives as long as the connection, not as ctx
	go c.heartbeat(time.Duration(hello.HeartbeatIntervalMs) * time.Millisecond)
	return nil
}

// send writes the frame {op, d}.
func (c *Conn) send(op string, d any) error {
	b, err := json.Marshal(map[string]any{"op": op, "d": d})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	return c.ws.Write(ctx, websocket.MessageText, b)
}

// envelope is the part of a dispatch an observer reads.
type envelope struct {
	Seq         int64     `json:"seq"`
	Type        string    `json:"type"`
	ClientMsgID string    `json:"client_msg_id"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// read consumes frames until the connection ends; an end the driver did not ask for is kept as
// the connection's error.
func (c *Conn) read() {
	defer close(c.done)
	for {
		_, b, err := c.ws.Read(context.Background())
		if err != nil {
			c.mu.Lock()
			if !c.closed {
				c.err = fmt.Errorf("load: the gateway connection ended with %d: %w", websocket.CloseStatus(err), err)
			}
			c.mu.Unlock()
			return
		}
		if !c.observer {
			continue
		}
		now := time.Now()
		var f frame
		if json.Unmarshal(b, &f) != nil || f.Op != "dispatch" {
			continue
		}
		var env envelope
		if json.Unmarshal(f.D, &env) != nil {
			continue
		}
		if env.Seq > c.lastSeq.Load() {
			c.lastSeq.Store(env.Seq)
		}
		if env.Type == "message.created" && env.ClientMsgID != "" {
			c.mu.Lock()
			c.dispatches = append(c.dispatches, Dispatch{ClientMsgID: env.ClientMsgID, OccurredAt: env.OccurredAt, Received: now})
			c.mu.Unlock()
		}
	}
}

// heartbeat sends heartbeat {last_seq} every interval, the first after a random share of it (04
// 6.3 step 5), until Close or the end of the connection.
func (c *Conn) heartbeat(interval time.Duration) {
	if interval <= 0 {
		return
	}
	t := time.NewTimer(time.Duration(rand.Float64() * float64(interval))) // #nosec G404 -- jitter, not a secret
	defer t.Stop()
	for {
		select {
		case <-c.stop:
			return
		case <-c.done:
			return
		case <-t.C:
		}
		if c.send("heartbeat", map[string]int64{"last_seq": c.lastSeq.Load()}) != nil {
			return
		}
		t.Reset(interval)
	}
}

// Dispatches returns the message.created dispatches an observer received so far.
func (c *Conn) Dispatches() []Dispatch {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.dispatches)
}

// Err returns why the connection ended without Close, nil while it is open or after Close.
func (c *Conn) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Close ends the connection with 1000 and waits for its reader.
func (c *Conn) Close() {
	c.once.Do(func() {
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()
		close(c.stop)
		_ = c.ws.Close(websocket.StatusNormalClosure, "")
		<-c.done
	})
}
