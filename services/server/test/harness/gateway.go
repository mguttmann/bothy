// SPDX-License-Identifier: Apache-2.0

package harness

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	gatewayws "github.com/OWNER/bothy/services/server/internal/gateway/adapters/ws"
	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
)

// frameBuffer is how many frames a GatewayConn holds for Next before its reader waits; a test
// that does not read stalls the connection like a slow client.
const frameBuffer = 1 << 14

// Frame is one frame of the server as a GatewayConn received it, with the host time of receipt.
type Frame struct {
	Op       string
	D        json.RawMessage
	Received time.Time
}

// GatewayConn is one gateway connection of a Client after identify and ready: it sends heartbeats
// every heartbeat_interval_ms of hello (the first after a random share of it, 04 section 6.3),
// answers reauth_required with a refreshed token and a reauth, counts the heartbeat_ack frames,
// and hands every other frame to Next. It is safe for concurrent use.
type GatewayConn struct {
	// Hello and Ready are the first two frames of the server.
	Hello rest.GatewayHello
	Ready rest.GatewayReady

	c      *Client
	ws     *wstest.Client
	frames chan Frame
	done   chan struct{}
	stop   chan struct{}
	once   sync.Once

	lastSeq atomic.Int64
	sent    atomic.Int64
	acks    atomic.Int64
	reauths atomic.Int64

	mu  sync.Mutex
	err error // the end of the connection: a *wstest.CloseError or a read error
}

// Gateway opens a gateway connection as c: upgrade, hello, identify with the access token and
// device of a person or the API token of an agent and a DPoP proof for the nonce of hello, then
// ready. An agent's connection answers reauth_required with nothing (INT-W40/1). The error reports
// a failed upgrade, a close of the server or another frame than ready.
func (c *Client) Gateway(ctx context.Context) (*GatewayConn, error) { return c.gateway(ctx, nil, nil) }

// GatewayWith is Gateway with the extra headers header on the upgrade request: a test edge in front
// of several api nodes routes the connection by them (M7-17).
func (c *Client) GatewayWith(ctx context.Context, header http.Header) (*GatewayConn, error) {
	return c.gateway(ctx, nil, header)
}

// GatewayFrom is Gateway with the cursor of identify set to after: the server catches the
// connection up from there before it goes live (04 "Catch-up"). Chaos tests reconnect with the
// last seq a receiver saw.
func (c *Client) GatewayFrom(ctx context.Context, after int64) (*GatewayConn, error) {
	return c.gateway(ctx, &after, nil)
}

// GatewayResume opens a new connection that resumes the gateway session sessionID of an earlier
// connection of c from cursor (04 section 6.3, op resume) instead of identify. The error reports
// a failed upgrade, a close of the server or another frame than resumed.
func (c *Client) GatewayResume(ctx context.Context, sessionID string, cursor int64) (*GatewayConn, error) {
	opts := []wstest.Option{wstest.WithoutHeartbeat()}
	if c.ClientIP != "" {
		opts = append(opts, wstest.WithHeader("X-Forwarded-For", c.ClientIP))
	}
	ws, err := wstest.Dial(c.tb, ctx, c.publicURL, opts...)
	if err != nil {
		return nil, err
	}
	access, _ := c.Tokens()
	p := wstest.ResumeParams{SessionID: sessionID, Cursor: cursor, Proof: func(nonce string) string { return c.gatewayProof(nonce, access) }}
	if c.apiToken {
		p.APIToken = access
	} else {
		p.AccessToken = access
	}
	resumed, err := ws.Resume(ctx, p)
	if err != nil {
		ws.Abort()
		return nil, err
	}
	g := &GatewayConn{Hello: ws.Hello(), c: c, ws: ws, frames: make(chan Frame, frameBuffer), done: make(chan struct{}), stop: make(chan struct{})}
	g.Ready.GatewaySessionId = resumed.GatewaySessionId
	g.lastSeq.Store(resumed.HeadSeq)
	go g.read() //nolint:gosec // G118: the reader lives as long as the connection, not as ctx
	go g.heartbeat(time.Duration(ws.Hello().HeartbeatIntervalMs) * time.Millisecond)
	return g, nil
}

// gateway opens the connection with an optional cursor and extra upgrade headers.
func (c *Client) gateway(ctx context.Context, cursor *int64, header http.Header) (*GatewayConn, error) {
	opts := []wstest.Option{wstest.WithoutHeartbeat()}
	if c.ClientIP != "" {
		opts = append(opts, wstest.WithHeader("X-Forwarded-For", c.ClientIP))
	}
	for name, values := range header {
		for _, v := range values {
			opts = append(opts, wstest.WithHeader(name, v))
		}
	}
	ws, err := wstest.Dial(c.tb, ctx, c.publicURL, opts...)
	if err != nil {
		return nil, err
	}
	access, _ := c.Tokens()
	p := wstest.IdentifyParams{Cursor: cursor, Proof: func(nonce string) string { return c.gatewayProof(nonce, access) }}
	if c.apiToken {
		p.APIToken = access
	} else {
		p.AccessToken, p.DeviceID = access, c.DeviceID
	}
	ready, err := ws.Identify(ctx, p)
	if err != nil {
		ws.Abort()
		return nil, err
	}
	g := &GatewayConn{Hello: ws.Hello(), Ready: ready, c: c, ws: ws, frames: make(chan Frame, frameBuffer), done: make(chan struct{}), stop: make(chan struct{})}
	g.lastSeq.Store(ready.HeadSeq)
	go g.read() //nolint:gosec // G118: the reader lives as long as the connection, not as ctx
	go g.heartbeat(time.Duration(ws.Hello().HeartbeatIntervalMs) * time.Millisecond)
	return g, nil
}

// gatewayProof returns a DPoP proof for the gateway with nonce and access; its htu is the HTTPS
// form of the public origin (04 section 6.3 step 3, gatewayws.HTU).
func (c *Client) gatewayProof(nonce, access string) string {
	htu, err := gatewayws.HTU(c.publicURL)
	if err != nil {
		htu = c.publicURL + wstest.Path
	}
	proof := c.key.proof(http.MethodGet, htu, nonce, access, c.now())
	if c.proofs != nil {
		c.proofs(proof)
	}
	return proof
}

// read hands frames to Next until the connection ends; it counts heartbeat_ack and answers
// reauth_required.
func (g *GatewayConn) read() {
	defer close(g.done)
	defer close(g.frames)
	for {
		op, d, err := g.ws.Next(context.Background())
		if err != nil {
			g.mu.Lock()
			g.err = err
			g.mu.Unlock()
			return
		}
		switch op {
		case "heartbeat_ack":
			g.acks.Add(1)
			continue
		case "reauth_required":
			go g.reauth(d)
			continue
		case "dispatch":
			var env struct {
				Seq int64 `json:"seq"`
			}
			if json.Unmarshal(d, &env) == nil && env.Seq > g.lastSeq.Load() {
				g.lastSeq.Store(env.Seq)
			}
		}
		g.frames <- Frame{Op: op, D: d, Received: time.Now()}
	}
}

// reauth refreshes the token pair of the client and sends reauth with a proof for the nonce of
// reauth_required.
func (g *GatewayConn) reauth(d json.RawMessage) {
	var req struct {
		Nonce string `json:"dpop_nonce"`
	}
	if json.Unmarshal(d, &req) != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if !g.c.renew(ctx) {
		return
	}
	access, _ := g.c.Tokens()
	if g.ws.Send("reauth", map[string]any{"access_token": access, "dpop": g.c.gatewayProof(req.Nonce, access)}) == nil {
		g.reauths.Add(1)
	}
}

// heartbeat sends a heartbeat with the last seen seq after a random share of interval and then
// every interval, until Close or the end of the connection.
func (g *GatewayConn) heartbeat(interval time.Duration) {
	if interval <= 0 {
		return
	}
	t := time.NewTimer(wstest.FirstHeartbeatDelay(interval, rand.Float64())) // #nosec G404 -- jitter, not a secret
	defer t.Stop()
	for {
		select {
		case <-g.stop:
			return
		case <-g.done:
			return
		case <-t.C:
		}
		if g.ws.Send("heartbeat", map[string]any{"last_seq": g.lastSeq.Load()}) != nil {
			return
		}
		g.sent.Add(1)
		t.Reset(interval)
	}
}

// Next returns the next frame other than heartbeat_ack and reauth_required. At the end of the
// connection it returns its error (a *wstest.CloseError for a close of the server).
func (g *GatewayConn) Next(ctx context.Context) (Frame, error) {
	select {
	case <-ctx.Done():
		return Frame{}, ctx.Err()
	case f, ok := <-g.frames:
		if !ok {
			return Frame{}, g.Err()
		}
		return f, nil
	}
}

// Send sends the frame {op, d}.
func (g *GatewayConn) Send(op string, d any) error { return g.ws.Send(op, d) }

// Err returns how the connection ended, or nil while it is open.
func (g *GatewayConn) Err() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.err == nil {
		select {
		case <-g.done:
			return errors.New("harness: gateway connection ended")
		default:
		}
	}
	return g.err
}

// Reauth sends reauth with the current access token of the client and a proof for the newest
// dpop_nonce the server announced (hello), without waiting for reauth_required.
func (g *GatewayConn) Reauth() error {
	access, _ := g.c.Tokens()
	return g.ws.Send("reauth", map[string]any{"access_token": access, "dpop": g.c.gatewayProof(g.Hello.DpopNonce, access)})
}

// Heartbeats returns how many heartbeats were sent and how many heartbeat_ack frames came back.
func (g *GatewayConn) Heartbeats() (sent, acked int64) { return g.sent.Load(), g.acks.Load() }

// Reauths returns how many reauth frames were sent after reauth_required.
func (g *GatewayConn) Reauths() int64 { return g.reauths.Load() }

// Close ends the heartbeats and closes the connection normally (1000).
func (g *GatewayConn) Close() {
	g.once.Do(func() { close(g.stop) })
	g.ws.Close()
}
