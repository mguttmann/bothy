// SPDX-License-Identifier: Apache-2.0

package agents_test

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// newTestProxy starts a fault proxy in front of upstream.
func newTestProxy(t *testing.T, upstream string) *faultProxy {
	t.Helper()
	p, err := newFaultProxy(30 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	if err := p.Start(upstream); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestFaultProxy_DropsAnswerAfterUpstream: the upstream handler runs and commits, the client gets
// a closed connection without an answer, and the proxy keeps the upstream answer.
func TestFaultProxy_DropsAnswerAfterUpstream(t *testing.T) {
	t.Parallel()
	var ran atomic.Int32
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		ran.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"invocation_token":"t2","expires_at":"2030-01-01T00:00:00.000Z"}`)
	}))
	t.Cleanup(up.Close)
	p := newTestProxy(t, up.URL)
	p.DropNext("renewInvocationToken")
	url := p.Origin() + "/api/v1/runs/0192f4a0-7b1c-7000-8000-000000000001/invocation-token"
	req, err := http.NewRequestWithContext(testkit.Context(t), http.MethodPost, url, strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp, err := http.DefaultClient.Do(req); err == nil {
		_ = resp.Body.Close()
		t.Fatalf("the client got an answer %d", resp.StatusCode)
	}
	if ran.Load() != 1 {
		t.Fatalf("the upstream ran %d times", ran.Load())
	}
	d, err := p.lastDropped("renewInvocationToken")
	if err != nil || d.status != http.StatusOK || !strings.Contains(string(d.body), `"invocation_token":"t2"`) {
		t.Fatalf("kept answer %+v, %v", d, err)
	}
	// Only the next request is dropped.
	req2, _ := http.NewRequestWithContext(testkit.Context(t), http.MethodPost, url, strings.NewReader(`{}`))
	resp, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || ran.Load() != 2 {
		t.Fatalf("second request %d, upstream ran %d", resp.StatusCode, ran.Load())
	}
}

// TestFaultProxy_SwitchesUpstreamAndClosesSockets: after SetUpstream a request reaches the new
// upstream and an open WebSocket tunnel is closed.
func TestFaultProxy_SwitchesUpstreamAndClosesSockets(t *testing.T) {
	t.Parallel()
	answer := func(text string) *httptest.Server {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Upgrade") != "" {
				hj, _ := w.(http.Hijacker)
				c, buf, err := hj.Hijack()
				if err != nil {
					return
				}
				_, _ = buf.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
				_ = buf.Flush()
				// Hold the tunnel open until the proxy closes it.
				_, _ = io.Copy(io.Discard, c)
				_ = c.Close()
				return
			}
			_, _ = io.WriteString(w, text)
		}))
		t.Cleanup(s.Close)
		return s
	}
	first, second := answer("first"), answer("second")
	p := newTestProxy(t, first.URL)
	get := func() string {
		req, _ := http.NewRequestWithContext(testkit.Context(t), http.MethodGet, p.Origin()+"/api/v1/instance/discovery", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	if got := get(); got != "first" {
		t.Fatalf("before the switch %q", got)
	}
	conn := openTunnel(t, p)
	if err := p.SetUpstream(second.URL); err != nil {
		t.Fatal(err)
	}
	if got := get(); got != "second" {
		t.Fatalf("after the switch %q", got)
	}
	_ = conn.SetReadDeadline(time.Now().Add(testkit.Patience)) // nosemgrep: bothy-go-test-wall-clock-wait -- a socket deadline of Patience, no wait for a window
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("the tunnel stayed open after the switch")
	}
}

// TestFaultProxy_PassesWebSocketUpgrade: an upgrade reaches the upstream and bytes flow both ways.
func TestFaultProxy_PassesWebSocketUpgrade(t *testing.T) {
	t.Parallel()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "websocket" {
			http.Error(w, "no upgrade", http.StatusBadRequest)
			return
		}
		hj, _ := w.(http.Hijacker)
		c, buf, err := hj.Hijack()
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		_, _ = buf.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
		_ = buf.Flush()
		line, _ := buf.ReadString('\n')
		_, _ = c.Write([]byte("echo " + line))
	}))
	t.Cleanup(up.Close)
	p := newTestProxy(t, up.URL)
	conn := openTunnel(t, p)
	if _, err := conn.Write([]byte("ping\n")); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(testkit.Patience)) // nosemgrep: bothy-go-test-wall-clock-wait -- a socket deadline of Patience, no wait for a window
	got, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil || got != "echo ping\n" {
		t.Fatalf("through the tunnel %q, %v", got, err)
	}
}

// openTunnel sends a WebSocket upgrade through the proxy and returns the connection after the
// 101 answer.
func openTunnel(t *testing.T, p *faultProxy) net.Conn {
	t.Helper()
	conn, err := (&net.Dialer{}).DialContext(context.Background(), "tcp", strings.TrimPrefix(p.Origin(), "http://"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_, err = io.WriteString(conn, "GET /api/v1/gateway HTTP/1.1\r\nHost: example.com\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(testkit.Patience)) // nosemgrep: bothy-go-test-wall-clock-wait -- a socket deadline of Patience, no wait for a window
	r := bufio.NewReader(conn)
	status, err := r.ReadString('\n')
	if err != nil || !strings.Contains(status, "101") {
		t.Fatalf("upgrade answer %q, %v", status, err)
	}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}
	_ = conn.SetReadDeadline(time.Time{})
	return &bufferedConn{Conn: conn, r: r}
}

// bufferedConn reads through the reader that consumed the upgrade answer.
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (b *bufferedConn) Read(p []byte) (int, error) { return b.r.Read(p) }
