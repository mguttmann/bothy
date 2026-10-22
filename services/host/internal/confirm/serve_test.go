// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
)

// failingListener fails every Accept with err.
type failingListener struct{ err error }

func (l failingListener) Accept() (net.Conn, error) { return nil, l.err }
func (failingListener) Close() error                { return nil }
func (failingListener) Addr() net.Addr              { return memAddr{} }

// debugLog is a logger into buf that keeps the debug codes as well.
func debugLog(buf *syncBuffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestServeNeedsListener(t *testing.T) {
	if err := NewServer(ServerOptions{Rule: testRule()}).Serve(testkit.Context(t)); err == nil {
		t.Fatal("Serve without a listener returned nil")
	}
}

func TestServeReturnsListenerFailure(t *testing.T) {
	boom := errors.New("accept failed")
	s := NewServer(ServerOptions{Listener: failingListener{err: boom}, Rule: testRule()})
	if err := s.Serve(testkit.Context(t)); !errors.Is(err, boom) {
		t.Fatalf("Serve = %v, want the listener error", err)
	}
	if s.Ask(context.Background(), reqOpen, time.Second) {
		t.Fatal("Ask = true after Serve failed")
	}
}

// startServer runs a Server over o until the test ends.
func startServer(t *testing.T, o ServerOptions) *Server {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	s := NewServer(o)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = s.Serve(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return s
}

// expectClosedWithoutData reads c until it closes and fails when it carried any byte.
func expectClosedWithoutData(t *testing.T, c net.Conn) {
	t.Helper()
	_ = c.SetReadDeadline(testkit.Deadline())
	n, err := io.Copy(io.Discard, c)
	if n != 0 || err != nil {
		t.Fatalf("read %d bytes, err %v; want a close without data", n, err)
	}
}

// Without a PeerOf function every connection is refused, and a server without a log does so
// without failing.
func TestServerWithoutPeerOfRefuses(t *testing.T) {
	l := newMemListener()
	s := startServer(t, ServerOptions{Listener: l, Rule: testRule()})
	c, err := l.dialAs(robin)(testkit.Context(t), "mem")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	expectClosedWithoutData(t, c)
	if s.Agents() != 0 {
		t.Fatalf("agents = %d without a peer lookup", s.Agents())
	}
}

// A connection whose peer check ends after Serve returned is closed and not counted.
func TestServerClosesAgentsAfterServe(t *testing.T) {
	l := newMemListener()
	inPeerOf := make(chan struct{})
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := NewServer(ServerOptions{Listener: l, Rule: testRule(), PeerOf: func(c net.Conn) (Peer, error) {
		close(inPeerOf)
		<-release
		return l.peerOf(c)
	}})
	served := make(chan error, 1)
	go func() { served <- s.Serve(ctx) }()
	c, err := l.dialAs(robin)(testkit.Context(t), "mem")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	waitFor(t, inPeerOf, "the peer check")
	cancel()
	// Serve marks itself closed before it waits for the peer check of c.
	deadline := testkit.Deadline()
	for {
		s.mu.Lock()
		closed := s.closed
		s.mu.Unlock()
		if closed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Serve did not close")
		}
		time.Sleep(5 * time.Millisecond)
	}
	close(release)
	if err := waitFor(t, served, "Serve to return"); err != nil {
		t.Fatalf("Serve = %v after the end of ctx", err)
	}
	expectClosedWithoutData(t, c)
	if s.Agents() != 0 {
		t.Fatalf("agents = %d after Serve returned", s.Agents())
	}
}

func TestDefaultRule(t *testing.T) {
	r := DefaultRule(uidService, func() []string { return []string{uidAlex} })
	if r.ServiceUID != uidService || r.ConsoleOwner == nil || !slices.Equal(r.Excluded(), []string{uidAlex}) {
		t.Fatalf("DefaultRule = %+v", r)
	}
	// Only Windows checks the session of the peer process; Linux and macOS go by the console owner.
	if (r.ConsoleSession != nil) != (runtime.GOOS == "windows") {
		t.Fatalf("ConsoleSession set = %v on %s", r.ConsoleSession != nil, runtime.GOOS)
	}
	if r.Allow(context.Background(), Peer{UID: uidService, PID: 1}) {
		t.Fatal("DefaultRule allows the service user")
	}
	if r.Allow(context.Background(), Peer{UID: uidAlex, PID: 1}) {
		t.Fatal("DefaultRule allows an excluded session user")
	}
}

// An invalid request gives false at once and reaches no agent.
func TestAskInvalidRequest(t *testing.T) {
	h := newHarness(t, testRule())
	c, r := rawAgent(t, h)
	for name, req := range map[string]Request{
		"unknown kind":       {Kind: "delete", Profile: "shell", SessionUser: "bothy-s-alex"},
		"empty kind":         {Profile: "shell", SessionUser: "bothy-s-alex"},
		"empty profile":      {Kind: KindOpen, SessionUser: "bothy-s-alex"},
		"empty session user": {Kind: KindOpen, Profile: "shell"},
	} {
		if h.s.Ask(context.Background(), req, testkit.Patience) {
			t.Errorf("%s: Ask = true", name)
		}
	}
	for _, timeout := range []time.Duration{0, -time.Second} {
		if h.s.Ask(context.Background(), reqOpen, timeout) {
			t.Errorf("timeout %v: Ask = true", timeout)
		}
	}
	// The first message the agent gets is the valid request that follows.
	res := make(chan bool, 1)
	go func() { res <- h.s.Ask(context.Background(), reqAttach, testkit.Patience) }()
	m := readMsg(t, c, r)
	if m["op"] != "confirm" || m["kind"] != KindAttach || m["profile"] != "claude" {
		t.Fatalf("first message %v, want the attach request", m)
	}
	writeLine(t, c, `{"op":"answer","id":"`+m["id"].(string)+`","allow":true}`)
	if !waitFor(t, res, "the answer") {
		t.Fatal("Ask = false after allow")
	}
}

// rawAgentAs connects as p and waits until the server counts want agents.
func rawAgentAs(t *testing.T, h *harness, p Peer, want int) (net.Conn, func() map[string]any) {
	t.Helper()
	c, err := h.l.dialAs(p)(context.Background(), "mem")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	h.waitAgents(want)
	r := bufio.NewReader(c)
	return c, func() map[string]any { return readMsg(t, c, r) }
}

// The loss of every asked agent ends the request as a denial before its timeout; the loss of one
// of two leaves the request to the other.
func TestAskAgentLost(t *testing.T) {
	t.Run("last agent", func(t *testing.T) {
		h := newHarness(t, testRule())
		c, read := rawAgentAs(t, h, robin, 1)
		res := make(chan bool, 1)
		go func() { res <- h.s.Ask(context.Background(), reqOpen, 2*testkit.Patience) }()
		read()
		_ = c.Close()
		if waitFor(t, res, "the denial after the loss of the agent") {
			t.Fatal("Ask = true after the agent was lost")
		}
		h.waitAgents(0)
	})

	t.Run("one of two", func(t *testing.T) {
		h := newHarness(t, testRule())
		c1, read1 := rawAgentAs(t, h, robin, 1)
		c2, read2 := rawAgentAs(t, h, Peer{UID: uidRobin, PID: 4243}, 2)
		res := make(chan bool, 1)
		go func() { res <- h.s.Ask(context.Background(), reqOpen, 2*testkit.Patience) }()
		read1()
		m := read2()
		_ = c1.Close()
		h.waitAgents(1)
		select {
		case v := <-res:
			t.Fatalf("Ask = %v after the loss of one of two agents", v)
		default:
		}
		writeLine(t, c2, `{"op":"answer","id":"`+m["id"].(string)+`","allow":true}`)
		if !waitFor(t, res, "the answer of the other agent") {
			t.Fatal("Ask = false although the remaining agent allowed")
		}
	})
}

// An agent that connected after a request was sent was not asked: its answer does not count.
func TestAnswerFromAgentNotAsked(t *testing.T) {
	h := newHarness(t, testRule())
	c1, read1 := rawAgentAs(t, h, robin, 1)
	res := make(chan bool, 1)
	go func() { res <- h.s.Ask(context.Background(), reqOpen, 2*testkit.Patience) }()
	id := read1()["id"].(string)
	c2, _ := rawAgentAs(t, h, Peer{UID: uidRobin, PID: 4243}, 2)
	writeLine(t, c2, `{"op":"answer","id":"`+id+`","allow":true}`)
	// The server reads the next line of c2 only after it handled the answer above.
	writeLine(t, c2, `{"op":"answer","id":"00000000-0000-4000-8000-000000000000","allow":true}`)
	select {
	case v := <-res:
		t.Fatalf("Ask = %v after the answer of an agent that was not asked", v)
	default:
	}
	writeLine(t, c1, `{"op":"answer","id":"`+id+`","allow":false}`)
	if waitFor(t, res, "the answer") {
		t.Fatal("Ask = true from the allow of an agent that was not asked")
	}
	if h.s.Agents() != 2 {
		t.Fatalf("agents = %d, want both to stay", h.s.Agents())
	}
}

// An agent that does not read is closed once its queue is full, and its requests end as denials
// instead of waiting for their timeouts.
func TestAgentThatDoesNotReadIsClosed(t *testing.T) {
	h := newHarness(t, testRule())
	rawAgentAs(t, h, robin, 1)
	// One line blocks in the write to the agent, agentQueue lines wait, the next one overflows.
	const asks = agentQueue + 2
	res := make(chan bool, asks)
	// The timeout stays below testkit.Patience: an Ask that took the agent from the list just
	// before its loss and registered its request just after it waits for its timeout (the gap
	// between allowed and the pending entry in Ask), and must still end within the wait below.
	for range asks {
		go func() { res <- h.s.Ask(context.Background(), reqOpen, testkit.Patience/4) }()
	}
	for range asks {
		if waitFor(t, res, "a denial") {
			t.Fatal("Ask = true from an agent that never read")
		}
	}
	h.waitAgents(0)
}

func TestRunAgentNeedsEndpointAndDialog(t *testing.T) {
	d := (&fixedDialog{allow: true}).dialog
	for name, o := range map[string]AgentOptions{
		"no endpoint": {Dialog: d},
		"no dialog":   {Endpoint: "mem"},
	} {
		if err := RunAgent(testkit.Context(t), o); err == nil {
			t.Errorf("%s: RunAgent = nil", name)
		}
	}
}

// RunAgent without a log works, and a failed dial is logged and tried again.
func TestRunAgentRetriesFailedDial(t *testing.T) {
	h := newHarness(t, testRule())
	buf := &syncBuffer{}
	dial := h.l.dialAs(robin)
	failed := make(chan struct{})
	first := true
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = RunAgent(ctx, AgentOptions{Endpoint: "mem", Dialog: (&fixedDialog{allow: true}).dialog, Log: debugLog(buf),
			Dial: func(ctx context.Context, ep string) (net.Conn, error) {
				if first {
					first = false
					close(failed)
					return nil, errors.New("service down")
				}
				return dial(ctx, ep)
			}})
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	waitFor(t, failed, "the failed dial")
	h.waitAgents(1)
	if !strings.Contains(buf.String(), "confirm_agent_dial_failed") {
		t.Fatalf("log %q lacks confirm_agent_dial_failed", buf.String())
	}
	if !h.s.Ask(context.Background(), reqOpen, testkit.Patience) {
		t.Fatal("Ask = false after the agent reconnected")
	}

	// The same agent without a log.
	h2 := newHarness(t, testRule())
	ctx2, cancel2 := context.WithCancel(context.Background())
	done2 := make(chan struct{})
	go func() {
		defer close(done2)
		_ = RunAgent(ctx2, AgentOptions{Endpoint: "mem", Dialog: (&fixedDialog{allow: true}).dialog, Dial: h2.l.dialAs(robin)})
	}()
	t.Cleanup(func() {
		cancel2()
		<-done2
	})
	h2.waitAgents(1)
	if !h2.s.Ask(context.Background(), reqOpen, testkit.Patience) {
		t.Fatal("Ask = false from an agent without a log")
	}
}

// A second request with the id of an open one is ignored and logged; the open dialog stays the
// only one for that id.
func TestAgentIgnoresDuplicateID(t *testing.T) {
	l := newMemListener()
	t.Cleanup(func() { _ = l.Close() })
	buf := &syncBuffer{}
	b := newBlockingDialog()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = RunAgent(ctx, AgentOptions{Endpoint: "mem", Dialog: b.dialog, Log: debugLog(buf), Dial: l.dialAs(robin)})
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	c := accept(t, l)
	writeLine(t, c, `{"op":"confirm","id":"a","kind":"open","profile":"shell","session_user":"bothy-s-alex","timeout_ms":600000}`)
	waitFor(t, b.started, "the dialog of a")
	writeLine(t, c, `{"op":"confirm","id":"a","kind":"attach","profile":"claude","session_user":"bothy-s-sam","timeout_ms":600000}`)
	writeLine(t, c, `{"op":"confirm","id":"b","kind":"open","profile":"other","session_user":"bothy-s-alex","timeout_ms":600000}`)
	if got := waitFor(t, b.started, "the dialog of b"); got.Profile != "other" {
		t.Fatalf("second dialog %+v, want the request b", got)
	}
	if !strings.Contains(buf.String(), "confirm_message_ignored") {
		t.Fatalf("log %q lacks confirm_message_ignored", buf.String())
	}
}
