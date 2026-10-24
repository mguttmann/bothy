// SPDX-License-Identifier: Apache-2.0

package agents_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// faultProxy is the reverse proxy of one case between every client and the server (brief Design
// note 9): it keeps the address of the clients stable across a restart of the server (only its
// upstream changes; open WebSockets are closed, so the SDK reconnects) and drops the answer of
// the next request of an operation on request, after the server answered (the "fake proxy" of
// AG-32). It listens on 127.0.0.1 port 0.
type faultProxy struct {
	ln     net.Listener
	srv    *http.Server
	client *http.Client

	mu       sync.Mutex
	upstream *url.URL
	sockets  map[net.Conn]struct{}
	drop     string
	dropped  []droppedAnswer
}

// droppedAnswer is the answer of a request whose connection the proxy closed without it.
type droppedAnswer struct {
	operation string
	status    int
	body      []byte
}

// newFaultProxy opens the listener; Start sets the upstream and serves. timeout bounds one
// upstream request (http.request_timeout of the case).
func newFaultProxy(timeout time.Duration) (*faultProxy, error) {
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	p := &faultProxy{ln: ln, client: egress.NewLoopback(timeout), sockets: map[net.Conn]struct{}{}}
	p.srv = &http.Server{Handler: p, ReadHeaderTimeout: timeout}
	return p, nil
}

// Origin is the origin the clients use: http://127.0.0.1:<port>.
func (p *faultProxy) Origin() string { return "http://" + p.ln.Addr().String() }

// Start serves with upstream as the server origin.
func (p *faultProxy) Start(upstream string) error {
	if err := p.SetUpstream(upstream); err != nil {
		return err
	}
	go func() { _ = p.srv.Serve(p.ln) }()
	return nil
}

// SetUpstream switches the upstream and closes every open WebSocket tunnel.
func (p *faultProxy) SetUpstream(upstream string) error {
	u, err := url.Parse(upstream)
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.upstream = u
	socks := p.sockets
	p.sockets = map[net.Conn]struct{}{}
	p.mu.Unlock()
	for c := range socks {
		_ = c.Close()
	}
	return nil
}

// DropNext makes the proxy drop the answer of the next request of operationID.
func (p *faultProxy) DropNext(operationID string) {
	p.mu.Lock()
	p.drop = operationID
	p.mu.Unlock()
}

// Dropped returns the answers dropped so far, oldest first.
func (p *faultProxy) Dropped() []droppedAnswer {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]droppedAnswer(nil), p.dropped...)
}

// Close stops the proxy and closes every tunnel.
func (p *faultProxy) Close() {
	_ = p.srv.Close()
	_ = p.SetUpstream("http://127.0.0.1:0")
}

// ServeHTTP forwards one request.
func (p *faultProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		p.tunnel(w, r)
		return
	}
	op := operationOf(r.Method, r.URL.Path)
	p.mu.Lock()
	up := *p.upstream
	// The request that matches claims the drop at once, so a second one in flight passes.
	dropping := p.drop != "" && op == p.drop
	if dropping {
		p.drop = ""
	}
	p.mu.Unlock()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "proxy: read body", http.StatusBadGateway)
		return
	}
	// The upstream is the server of the case on 127.0.0.1, set by the conductor.
	out, err := http.NewRequestWithContext(r.Context(), r.Method, up.Scheme+"://"+up.Host+r.URL.RequestURI(), bytes.NewReader(body)) //nolint:gosec // G704: loopback upstream of the test
	if err != nil {
		http.Error(w, "proxy: request", http.StatusBadGateway)
		return
	}
	out.Header = r.Header.Clone()
	out.Host = r.Host
	out.ContentLength = int64(len(body))
	resp, err := p.client.Do(out) //nolint:gosec // G704: the loopback client of egress, upstream of the test
	if err != nil {
		http.Error(w, "proxy: upstream", http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	answer, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "proxy: upstream body", http.StatusBadGateway)
		return
	}
	if dropping && resp.StatusCode == http.StatusUnauthorized {
		// A DPoP nonce challenge is no answer of the operation: keep the drop for the retry.
		p.mu.Lock()
		p.drop = op
		p.mu.Unlock()
	} else if dropping {
		p.mu.Lock()
		p.dropped = append(p.dropped, droppedAnswer{operation: op, status: resp.StatusCode, body: answer})
		p.mu.Unlock()
		// Close the client connection without an answer: the server stored its effect.
		if hj, ok := w.(http.Hijacker); ok {
			if c, _, err := hj.Hijack(); err == nil {
				_ = c.Close()
				return
			}
		}
		panic(http.ErrAbortHandler)
	}
	for k, vs := range resp.Header {
		w.Header()[k] = vs
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(answer)
}

// tunnel passes a WebSocket upgrade through as raw bytes.
func (p *faultProxy) tunnel(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	up := *p.upstream
	p.mu.Unlock()
	upConn, err := (&net.Dialer{}).DialContext(r.Context(), "tcp", up.Host)
	if err != nil {
		http.Error(w, "proxy: upstream", http.StatusBadGateway)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		_ = upConn.Close()
		http.Error(w, "proxy: no hijack", http.StatusInternalServerError)
		return
	}
	client, buf, err := hj.Hijack()
	if err != nil {
		_ = upConn.Close()
		return
	}
	if err := r.Write(upConn); err != nil {
		_ = upConn.Close()
		_ = client.Close()
		return
	}
	p.mu.Lock()
	p.sockets[client] = struct{}{}
	p.sockets[upConn] = struct{}{}
	p.mu.Unlock()
	go func() {
		_, _ = io.Copy(upConn, io.MultiReader(bufferedOf(buf.Reader), client))
		_ = upConn.Close()
		_ = client.Close()
	}()
	go func() {
		_, _ = io.Copy(client, upConn)
		_ = upConn.Close()
		_ = client.Close()
	}()
}

// bufferedOf returns the bytes the hijacked reader already buffered.
func bufferedOf(r *bufio.Reader) io.Reader {
	n := r.Buffered()
	if n == 0 {
		return bytes.NewReader(nil)
	}
	b, _ := r.Peek(n)
	return bytes.NewReader(append([]byte(nil), b...))
}

// operationOf returns the operationId of a request path, or "".
func operationOf(method, path string) string {
	ops, err := operations()
	if err != nil {
		return ""
	}
	rest, ok := strings.CutPrefix(path, "/api/v1")
	if !ok {
		return ""
	}
	for id, op := range ops {
		if op.method == method && op.pattern.MatchString(rest) {
			return id
		}
	}
	return ""
}

// errNoDropped reports a keep of a dropped answer that does not exist.
var errNoDropped = errors.New("no dropped answer")

// lastDropped returns the newest dropped answer of operation.
func (p *faultProxy) lastDropped(operation string) (droppedAnswer, error) {
	d := p.Dropped()
	for i := len(d) - 1; i >= 0; i-- {
		if d[i].operation == operation {
			return d[i], nil
		}
	}
	return droppedAnswer{}, fmt.Errorf("%w of %s", errNoDropped, operation)
}
