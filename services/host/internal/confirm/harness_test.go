// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"
)

// memListener is an in-memory endpoint: every dial creates a net.Pipe and hands one end to Accept,
// so the tests run the server and the agent on every build host without a socket.
type memListener struct {
	conns chan net.Conn
	done  chan struct{}
	once  sync.Once
	peers sync.Map // server end -> Peer
}

func newMemListener() *memListener {
	return &memListener{conns: make(chan net.Conn), done: make(chan struct{})}
}

func (l *memListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *memListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return nil
}

func (l *memListener) Addr() net.Addr { return memAddr{} }

type memAddr struct{}

func (memAddr) Network() string { return "mem" }
func (memAddr) String() string  { return "mem" }

// dialAs returns a Dial function whose connections the server sees as peer p.
func (l *memListener) dialAs(p Peer) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, _ string) (net.Conn, error) {
		server, client := net.Pipe()
		l.peers.Store(server, p)
		select {
		case l.conns <- server:
			return client, nil
		case <-l.done:
			return nil, net.ErrClosed
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (l *memListener) peerOf(c net.Conn) (Peer, error) {
	p, ok := l.peers.Load(c)
	if !ok {
		return Peer{}, errors.New("no peer")
	}
	return p.(Peer), nil
}

// syncBuffer is a log sink the server goroutines write while the test reads it.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// The people of the tests: the service user, the session users of Alex and Sam, a former session
// user, Robin at the console and another person without the console.
const (
	uidService = "210"
	uidAlex    = "1001"
	uidSam     = "1002"
	uidFormer  = "1003"
	uidRobin   = "1004"
	uidOther   = "1005"
)

func testRule() Rule {
	return Rule{
		ServiceUID: uidService,
		Excluded:   func() []string { return []string{uidAlex, uidSam, uidFormer} },
		ConsoleOwner: func(context.Context) (string, error) {
			return uidRobin, nil
		},
	}
}

type harness struct {
	t   *testing.T
	l   *memListener
	s   *Server
	log *syncBuffer
	ctx context.Context
}

func newHarness(t *testing.T, rule Rule) *harness {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	l := newMemListener()
	buf := &syncBuffer{}
	s := NewServer(ServerOptions{Listener: l, Rule: rule, PeerOf: l.peerOf, Log: slog.New(slog.NewTextHandler(buf, nil))})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = s.Serve(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return &harness{t: t, l: l, s: s, log: buf, ctx: ctx}
}

// agent starts RunAgent as peer p with dialog d and waits until the server counts want agents.
func (h *harness) agent(p Peer, d Dialog, want int) {
	h.t.Helper()
	ctx, cancel := context.WithCancel(h.ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = RunAgent(ctx, AgentOptions{Endpoint: "mem", Dialog: d, Log: slog.New(slog.DiscardHandler), Dial: h.l.dialAs(p)})
	}()
	h.t.Cleanup(func() {
		cancel()
		<-done
	})
	h.waitAgents(want)
}

func (h *harness) waitAgents(want int) {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for h.s.Agents() != want {
		if time.Now().After(deadline) {
			h.t.Fatalf("agents = %d, want %d", h.s.Agents(), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// fixedDialog answers allow at once and records the requests it saw.
type fixedDialog struct {
	allow bool
	mu    sync.Mutex
	seen  []Request
}

func (f *fixedDialog) dialog(_ context.Context, r Request, _ time.Duration) bool {
	f.mu.Lock()
	f.seen = append(f.seen, r)
	f.mu.Unlock()
	return f.allow
}

func (f *fixedDialog) requests() []Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Request(nil), f.seen...)
}

// blockingDialog never answers; it reports on cancelled when its context ends (a cancel of the
// service or the end of the connection).
type blockingDialog struct {
	started   chan Request
	cancelled chan struct{}
}

func newBlockingDialog() *blockingDialog {
	return &blockingDialog{started: make(chan Request, 8), cancelled: make(chan struct{}, 8)}
}

func (b *blockingDialog) dialog(ctx context.Context, r Request, _ time.Duration) bool {
	b.started <- r
	<-ctx.Done()
	b.cancelled <- struct{}{}
	return true // the service must not count an answer after its cancel
}

func waitFor[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
	var zero T
	return zero
}
