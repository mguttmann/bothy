// SPDX-License-Identifier: Apache-2.0

package fakeadapter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	gatewayws "github.com/OWNER/bothy/services/server/internal/gateway/adapters/ws"
	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/runstest"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// Frame is one frame of the server with the host time it was read from the connection.
type Frame struct {
	Op       string
	D        json.RawMessage
	Received time.Time
}

// Conn is the gateway connection of one consumer. Next returns the frames in order and, at the
// end of the connection, its error; Send writes one frame; Close ends the connection.
type Conn interface {
	Next(ctx context.Context) (Frame, error)
	Send(op string, d any) error
	Close()
}

// API is the REST client of the agent: Do sends one request with a JSON body (nil for none) and
// returns the status and body of the answer; a transport error is status 0.
type API interface {
	Do(ctx context.Context, method, path string, body any) (int, []byte)
}

// Verifier is the adapter check of an invocation token (09 section 4.5), implemented by
// runstest.ReferenceVerifier.
type Verifier interface {
	Check(ctx context.Context, token string, e runstest.ReferenceExpectation) (runstest.ReferenceClaims, error)
}

// HarnessAPI binds API to the client of an agent of a harness server (NewAgent): every request
// carries the agent's DPoP-bound API token.
type HarnessAPI struct{ C *harness.Client }

// Do sends the request through the harness client.
func (h HarnessAPI) Do(ctx context.Context, method, path string, body any) (int, []byte) {
	r := h.C.Do(ctx, harness.Request{Method: method, Path: path, Body: body})
	return r.Status, r.Body
}

// frameBuffer is how many read frames a connection holds before its reader waits.
const frameBuffer = 1 << 14

// wsConn is a Conn over the gateway test client wstest: a reader goroutine takes every frame as
// soon as wstest has read and validated it, stamps it with the host time and queues it, so a busy
// adapter never moves the receive time.
type wsConn struct {
	ws     *wstest.Client
	frames chan Frame
	mu     sync.Mutex
	err    error
}

// DialGateway connects the agent client c to the gateway below publicURL as the consumer
// consumer: upgrade, hello, identify with the API token, the consumer name and a DPoP proof of
// the client's key for the nonce of hello, then ready. wstest sends the gateway heartbeats and
// checks every frame against the contract (a violation fails tb in its cleanup). The error
// reports a failed upgrade or identify; it never contains the token.
func DialGateway(ctx context.Context, tb testing.TB, c *harness.Client, publicURL, consumer string) (Conn, error) {
	var opts []wstest.Option
	if c.ClientIP != "" {
		opts = append(opts, wstest.WithHeader("X-Forwarded-For", c.ClientIP))
	}
	ws, err := wstest.Dial(tb, ctx, publicURL, opts...)
	if err != nil {
		return nil, err
	}
	htu, err := gatewayws.HTU(publicURL)
	if err != nil {
		ws.Abort()
		return nil, err
	}
	token, _ := c.Tokens()
	if _, err := ws.Identify(ctx, wstest.IdentifyParams{APIToken: token, Consumer: &consumer,
		Proof: func(nonce string) string { return c.Proof(http.MethodGet, htu, nonce) }}); err != nil {
		ws.Abort()
		return nil, err
	}
	w := &wsConn{ws: ws, frames: make(chan Frame, frameBuffer)}
	go w.read() //nolint:gosec // G118: the reader lives as long as the connection, not as ctx
	return w, nil
}

// read queues every frame with its receive time until the connection ends.
func (w *wsConn) read() {
	defer close(w.frames)
	for {
		op, d, err := w.ws.Next(context.Background())
		if err != nil {
			w.mu.Lock()
			w.err = err
			w.mu.Unlock()
			return
		}
		w.frames <- Frame{Op: op, D: d, Received: time.Now()}
	}
}

// Next returns the next frame or the end of the connection.
func (w *wsConn) Next(ctx context.Context) (Frame, error) {
	select {
	case <-ctx.Done():
		return Frame{}, ctx.Err()
	case f, ok := <-w.frames:
		if !ok {
			w.mu.Lock()
			defer w.mu.Unlock()
			if w.err == nil {
				return Frame{}, errors.New("fakeadapter: connection ended")
			}
			return Frame{}, w.err
		}
		return f, nil
	}
}

// Send writes one frame.
func (w *wsConn) Send(op string, d any) error { return w.ws.Send(op, d) }

// Close closes the connection normally.
func (w *wsConn) Close() { w.ws.Close() }
