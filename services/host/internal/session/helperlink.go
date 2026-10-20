// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/OWNER/bothy/services/host/internal/localchan"
)

// Dialer opens the verified session channel to the helper of a session user: the peer check of
// M2-42 (startup.ChannelHelpers.Open). An error wrapping doctor.ErrPeerMismatch is a failed peer
// check in operation (HD10).
type Dialer func(ctx context.Context, sessionUser string) (*localchan.Conn, error)

// startAnswer is the answer of the helper to one pty.start: "" for pty.started, else the detail
// of pty.failed.
type startAnswer struct{ detail string }

// helperLink is the session channel to the helper of one session user: it carries the session
// ops and the frames of every PTY of that helper. When it breaks, the helper ends every PTY it
// started on it, and the manager ends those sessions.
type helperLink struct {
	user string
	c    *localchan.Conn
	m    *Manager
	next atomic.Uint64

	mu      sync.Mutex
	pending map[string]chan startAnswer
	broken  bool
	done    chan struct{}
}

func newHelperLink(m *Manager, user string, c *localchan.Conn) *helperLink {
	l := &helperLink{user: user, c: c, m: m, pending: map[string]chan startAnswer{}, done: make(chan struct{})}
	go l.read()
	return l
}

// read dispatches the packets of the helper until the channel ends or carries something a
// helper never sends.
func (l *helperLink) read() {
	defer l.fail()
	for {
		p, err := l.c.ReadPacket()
		if err != nil {
			return
		}
		if p.IsFrame {
			l.m.fromHelper(p.SessionID, p.Frame)
			continue
		}
		if !l.handle(p.Msg) {
			return
		}
	}
}

// handle reacts to one control message of the helper; false closes the channel.
func (l *helperLink) handle(m localchan.Message) bool {
	switch m.Op {
	case localchan.OpPTYStarted:
		var b localchan.PTYStarted
		if json.Unmarshal(m.D, &b) != nil {
			return false
		}
		l.answer(b.SessionID, startAnswer{})
	case localchan.OpPTYFailed:
		var b localchan.PTYFailed
		if json.Unmarshal(m.D, &b) != nil {
			return false
		}
		if b.CloseDetail != DetailSharedLocation {
			b.CloseDetail = DetailPTYFailed
		}
		l.answer(b.SessionID, startAnswer{detail: b.CloseDetail})
	case localchan.OpPTYExited:
		var b localchan.PTYExited
		if json.Unmarshal(m.D, &b) != nil {
			return false
		}
		l.m.exited(b.SessionID, b.ExitCode)
	default:
		// Any other op on a session channel is a broken helper: close the channel.
		return false
	}
	return true
}

func (l *helperLink) answer(id string, a startAnswer) {
	l.mu.Lock()
	ch := l.pending[id]
	delete(l.pending, id)
	l.mu.Unlock()
	if ch != nil {
		ch <- a
	}
}

// fail marks the link broken, closes it and tells the manager.
func (l *helperLink) fail() {
	l.mu.Lock()
	if l.broken {
		l.mu.Unlock()
		return
	}
	l.broken = true
	pend := l.pending
	l.pending = map[string]chan startAnswer{}
	close(l.done)
	l.mu.Unlock()
	_ = l.c.Close()
	for _, ch := range pend {
		ch <- startAnswer{detail: DetailPTYFailed}
	}
	l.m.linkBroken(l)
}

// isBroken reports a link that ended.
func (l *helperLink) isBroken() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.broken
}

// start sends pty.start and waits for pty.started or pty.failed; it returns "" when the PTY runs,
// else the close_detail. A broken channel gives pty_failed.
func (l *helperLink) start(ctx context.Context, req localchan.PTYStart) string {
	ch := make(chan startAnswer, 1)
	l.mu.Lock()
	if l.broken {
		l.mu.Unlock()
		return DetailPTYFailed
	}
	l.pending[req.SessionID] = ch
	l.mu.Unlock()
	err := l.c.Send(localchan.OpPTYStart, l.id(), req)
	if err != nil {
		l.fail()
	}
	select {
	case a := <-ch:
		return a.detail
	case <-ctx.Done():
		l.mu.Lock()
		delete(l.pending, req.SessionID)
		l.mu.Unlock()
		return DetailPTYFailed
	}
}

func (l *helperLink) id() string { return strconv.FormatUint(l.next.Add(1), 10) }

// data sends the frame bytes f of a session to the helper.
func (l *helperLink) data(sid [localchan.SessionIDLen]byte, f []byte) {
	if err := l.c.SendFrame(sid, f); err != nil {
		l.fail()
	}
}

// resize sends pty.resize.
func (l *helperLink) resize(id string, cols, rows uint16) {
	if err := l.c.Send(localchan.OpPTYResize, l.id(), localchan.PTYResize{SessionID: id, Cols: cols, Rows: rows}); err != nil {
		l.fail()
	}
}

// stop sends pty.stop: the helper ends the PTY with its whole tree at once.
func (l *helperLink) stop(id string) {
	if err := l.c.Send(localchan.OpPTYStop, l.id(), localchan.PTYStop{SessionID: id}); err != nil {
		l.fail()
	}
}
