// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/localchan"
	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
)

// quietManager returns a manager without a server, gate or helper: the tests of the session
// channel need only its bookkeeping.
func quietManager(t *testing.T) *Manager {
	t.Helper()
	m := New(Deps{Log: slog.New(slog.DiscardHandler)})
	t.Cleanup(m.Shutdown)
	return m
}

// pipeLink returns a link over an in-memory channel and the helper end of that channel.
func pipeLink(t *testing.T, m *Manager) (*helperLink, *localchan.Conn) {
	t.Helper()
	a, b := net.Pipe()
	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })
	return newHelperLink(m, alexOSU, localchan.NewConn(a, localchan.Peer{})), localchan.NewConn(b, localchan.Peer{})
}

// nextStart reads packets of the helper end until pty.start and returns its session ID.
func nextStart(t *testing.T, h *localchan.Conn) string {
	t.Helper()
	for {
		p, err := h.ReadPacket()
		if err != nil {
			t.Errorf("helper end: %v", err)
			return ""
		}
		if !p.IsFrame && p.Msg.Op == localchan.OpPTYStart {
			var req localchan.PTYStart
			if err := json.Unmarshal(p.Msg.D, &req); err != nil {
				t.Errorf("pty.start: %v", err)
			}
			return req.SessionID
		}
	}
}

// startAsync runs l.start for id and returns the channel of its answer.
func startAsync(ctx context.Context, l *helperLink, id string) <-chan string {
	out := make(chan string, 1)
	go func() { out <- l.start(ctx, localchan.PTYStart{SessionID: id, Profile: "shell", Kind: "shell"}) }()
	return out
}

func answerOf(t *testing.T, ctx context.Context, ch <-chan string) string {
	t.Helper()
	select {
	case d := <-ch:
		return d
	case <-ctx.Done():
		t.Fatal("pty.start got no answer")
		return ""
	}
}

func waitDone(t *testing.T, ctx context.Context, l *helperLink) {
	t.Helper()
	select {
	case <-l.done:
	case <-ctx.Done():
		t.Fatal("the link did not break")
	}
}

// TestHelperLinkAnswers: pty.started starts the PTY; pty.failed keeps shared_location and turns
// every other detail into pty_failed.
func TestHelperLinkAnswers(t *testing.T) {
	ctx := testkit.Context(t)
	l, h := pipeLink(t, quietManager(t))
	rows := []struct {
		op   string
		body func(id string) any
		want string
	}{
		{localchan.OpPTYStarted, func(id string) any { return localchan.PTYStarted{SessionID: id} }, ""},
		{localchan.OpPTYFailed, func(id string) any { return localchan.PTYFailed{SessionID: id, CloseDetail: DetailSharedLocation} }, DetailSharedLocation},
		{localchan.OpPTYFailed, func(id string) any { return localchan.PTYFailed{SessionID: id, CloseDetail: "spawn_error"} }, DetailPTYFailed},
	}
	for i, r := range rows {
		id := []string{sessID, otherID, hostID}[i]
		ch := startAsync(ctx, l, id)
		if got := nextStart(t, h); got != id {
			t.Fatalf("pty.start for %q, want %q", got, id)
		}
		if err := h.Send(r.op, "1", r.body(id)); err != nil {
			t.Fatal(err)
		}
		if d := answerOf(t, ctx, ch); d != r.want {
			t.Errorf("%s: answer %q, want %q", r.op, d, r.want)
		}
	}
	if l.isBroken() {
		t.Error("valid answers broke the link")
	}
}

// TestHelperLinkBrokenHelper: a body that does not decode or an op a helper never sends closes
// the channel; a waiting start gets pty_failed, a later start too, and the manager forgets the link.
func TestHelperLinkBrokenHelper(t *testing.T) {
	for _, op := range []string{localchan.OpPTYStarted, localchan.OpPTYFailed, localchan.OpPTYExited, localchan.OpCheckRun} {
		t.Run(op, func(t *testing.T) {
			ctx := testkit.Context(t)
			m := quietManager(t)
			l, h := pipeLink(t, m)
			m.mu.Lock()
			m.links[alexOSU] = l
			m.mu.Unlock()
			ch := startAsync(ctx, l, sessID)
			nextStart(t, h)
			if err := h.Send(op, "1", "not an object"); err != nil {
				t.Fatal(err)
			}
			if d := answerOf(t, ctx, ch); d != DetailPTYFailed {
				t.Errorf("waiting start: %q", d)
			}
			waitDone(t, ctx, l)
			if d := l.start(ctx, localchan.PTYStart{SessionID: otherID}); d != DetailPTYFailed {
				t.Errorf("start on a broken link: %q", d)
			}
			until(t, "the broken link forgotten", func() bool {
				m.mu.Lock()
				defer m.mu.Unlock()
				_, kept := m.links[alexOSU]
				return !kept
			})
		})
	}
}

// TestHelperLinkStartGivesUp: a start whose context ends gets pty_failed; the late pty.started
// finds no waiter and the link stays usable.
func TestHelperLinkStartGivesUp(t *testing.T) {
	ctx := testkit.Context(t)
	l, h := pipeLink(t, quietManager(t))
	sctx, cancel := context.WithCancel(ctx)
	ch := startAsync(sctx, l, sessID)
	nextStart(t, h)
	cancel()
	if d := answerOf(t, ctx, ch); d != DetailPTYFailed {
		t.Errorf("abandoned start: %q", d)
	}
	if err := h.Send(localchan.OpPTYStarted, "1", localchan.PTYStarted{SessionID: sessID}); err != nil {
		t.Fatal(err)
	}
	ch = startAsync(ctx, l, otherID)
	nextStart(t, h)
	if err := h.Send(localchan.OpPTYStarted, "2", localchan.PTYStarted{SessionID: otherID}); err != nil {
		t.Fatal(err)
	}
	if d := answerOf(t, ctx, ch); d != "" || l.isBroken() {
		t.Errorf("start after a late answer: %q, broken %v", d, l.isBroken())
	}
}

// failingWriter is a channel end whose reads wait until it is closed and whose writes fail.
type failingWriter struct {
	r *io.PipeReader
	w *io.PipeWriter
}

var _ io.ReadWriteCloser = (*failingWriter)(nil)

func newFailingWriter() *failingWriter {
	r, w := io.Pipe()
	return &failingWriter{r: r, w: w}
}

func (f *failingWriter) Read(b []byte) (int, error) { return f.r.Read(b) }
func (f *failingWriter) Write([]byte) (int, error)  { return 0, errors.New("channel write failed") }
func (f *failingWriter) Close() error               { return f.w.Close() }
func (f *failingWriter) conn() *localchan.Conn      { return localchan.NewConn(f, localchan.Peer{}) }

// TestHelperLinkWriteFailureBreaks: a failed write of pty.start, a frame, pty.resize or pty.stop
// breaks the link; a start gets pty_failed.
func TestHelperLinkWriteFailureBreaks(t *testing.T) {
	var sid [localchan.SessionIDLen]byte
	rows := map[string]func(ctx context.Context, l *helperLink) string{
		"pty.start": func(ctx context.Context, l *helperLink) string {
			return l.start(ctx, localchan.PTYStart{SessionID: sessID})
		},
		"frame": func(_ context.Context, l *helperLink) string {
			l.data(sid, []byte{1})
			return ""
		},
		"pty.resize": func(_ context.Context, l *helperLink) string {
			l.resize(sessID, 80, 24)
			return ""
		},
		"pty.stop": func(_ context.Context, l *helperLink) string {
			l.stop(sessID)
			return ""
		},
	}
	for name, do := range rows {
		t.Run(name, func(t *testing.T) {
			ctx := testkit.Context(t)
			fw := newFailingWriter()
			t.Cleanup(func() { _ = fw.Close() })
			l := newHelperLink(quietManager(t), alexOSU, fw.conn())
			d := do(ctx, l)
			waitDone(t, ctx, l)
			if name == "pty.start" && d != DetailPTYFailed {
				t.Errorf("start over a failing channel: %q", d)
			}
		})
	}
}
