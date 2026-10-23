// SPDX-License-Identifier: Apache-2.0

//go:build sdkserver

package sdkserver_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// dialWait bounds the connection of the forwarder to its target.
const dialWait = 5 * time.Second

// forwarder is a TCP forwarder on 127.0.0.1 port 0 in front of the public listener of the
// current server generation. Its origin is server.public_url of every generation, so the URLs of
// discovery, the DPoP htu of the SDK and its reconnects stay valid across a restart (brief M2-29
// Design note 16). While it has no target it closes every new connection at once, which the SDK
// sees as a connection loss. It is safe for concurrent use.
type forwarder struct {
	ln net.Listener

	mu     sync.Mutex
	target string
	conns  map[net.Conn]struct{}
	closed bool
	wg     sync.WaitGroup
}

// newForwarder listens on 127.0.0.1 port 0 and accepts in the background; it has no target yet.
func newForwarder() (*forwarder, error) {
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	f := &forwarder{ln: ln, conns: map[net.Conn]struct{}{}}
	f.wg.Add(1)
	go f.accept()
	return f, nil
}

// Origin returns http://127.0.0.1:<port> of the listener.
func (f *forwarder) Origin() string { return "http://" + f.ln.Addr().String() }

// SetTarget routes new connections to addr (host:port of a public listener).
func (f *forwarder) SetTarget(addr string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.target = addr
}

// Pause closes every new connection at once until the next SetTarget; open ones stay.
func (f *forwarder) Pause() { f.SetTarget("") }

// CloseConns ends every open connection on both sides.
func (f *forwarder) CloseConns() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for c := range f.conns {
		_ = c.Close()
	}
}

// Close stops the listener, ends every open connection and waits for the copy loops.
func (f *forwarder) Close() error {
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()
	err := f.ln.Close()
	f.CloseConns()
	f.wg.Wait()
	return err
}

// accept serves every connection of the listener until it closes.
func (f *forwarder) accept() {
	defer f.wg.Done()
	for {
		c, err := f.ln.Accept()
		if err != nil {
			return
		}
		f.wg.Add(1)
		go f.serve(c)
	}
}

// serve connects c to the current target and copies both directions until one side ends;
// then both connections close.
func (f *forwarder) serve(c net.Conn) {
	defer f.wg.Done()
	f.mu.Lock()
	target := f.target
	f.mu.Unlock()
	if target == "" {
		_ = c.Close()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), dialWait)
	up, err := (&net.Dialer{}).DialContext(ctx, "tcp", target)
	cancel()
	if err != nil {
		_ = c.Close()
		return
	}
	if !f.track(c, up) {
		_ = c.Close()
		_ = up.Close()
		return
	}
	done := make(chan struct{}, 2)
	pipe := func(dst, src net.Conn) {
		_, _ = io.Copy(dst, src)
		done <- struct{}{}
	}
	go pipe(up, c)
	go pipe(c, up)
	<-done
	_ = c.Close()
	_ = up.Close()
	<-done
	f.untrack(c, up)
}

// track registers the pair and reports false when the forwarder is closed already.
func (f *forwarder) track(c, up net.Conn) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return false
	}
	f.conns[c], f.conns[up] = struct{}{}, struct{}{}
	return true
}

// untrack removes the pair.
func (f *forwarder) untrack(c, up net.Conn) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.conns, c)
	delete(f.conns, up)
}

// TestForwarder: the forwarder passes connections to its target, closes new connections while it
// has none, ends its open connections on CloseConns and reaches a new target under the same
// origin.
func TestForwarder(t *testing.T) {
	t.Parallel()
	a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "a") }))
	defer a.Close()
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "b") }))
	defer b.Close()
	f, err := newForwarder()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	f.SetTarget(a.Listener.Addr().String())
	if got := fetch(t, f.Origin()); got != "a" {
		t.Fatalf("through the forwarder %q, want a", got)
	}
	open, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", f.Origin()[len("http://"):])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = open.Close() }()
	f.Pause()
	f.CloseConns()
	if err := open.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := open.Read(make([]byte, 1)); err == nil {
		t.Fatal("an open connection survived CloseConns")
	}
	if got := fetch(t, f.Origin()); got != "" {
		t.Fatalf("a paused forwarder answered %q", got)
	}
	f.SetTarget(b.Listener.Addr().String())
	if got := fetch(t, f.Origin()); got != "b" {
		t.Fatalf("after the new target %q, want b", got)
	}
}

// fetch returns the body of GET origin on a fresh connection, or "" when the request fails.
func fetch(t *testing.T, origin string) string {
	t.Helper()
	c := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, origin, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return string(b)
}
