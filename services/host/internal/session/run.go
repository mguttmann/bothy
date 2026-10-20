// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/localchan"
	"github.com/OWNER/bothy/services/host/internal/noise"
)

// session is one session with a running PTY.
type session struct {
	m             *Manager
	id            string
	idb           [localchan.SessionIDLen]byte
	sub, osu, prf string
	link          *helperLink

	mu sync.Mutex
	// dc and tr are the attached data connection and its transport; nil while detached.
	dc     *dataConn
	tr     *noise.Transport
	window time.Duration
	// attaching collects the output that arrives between start and the replay of an attach.
	attaching bool
	attachBuf []byte
	ring      *Ring
	// lastInput is the time of the last input frame of the device (idle, also across a detach);
	// detachedAt the start of the current detach.
	lastInput, detachedAt time.Time
	idle, detach          time.Duration
	ended                 bool
	kick                  chan struct{}
	done                  chan struct{}
}

// endSpec describes an end. silent ends without session.ended (the server closed the session);
// dc is a data connection whose close is left to the server after the report.
type endSpec struct {
	reason, detail string
	exitCode       *int
	silent         bool
	dc             *dataConn
	window         time.Duration
}

// claimSession registers the session of cl before pty.start (so output and exit find it), but only
// while dc is still the data connection of the session, the session was neither closed nor ended
// and no other session holds its ID: the opening is atomic against session.close, the end of all
// sessions and a newer session.open of the same session (which replaced dc). false: the opening is
// abandoned without a PTY.
func (m *Manager) claimSession(cl Claims, l *helperLink, idb [noise.IDLen]byte, dc *dataConn, tr *noise.Transport, window time.Duration) (*session, bool) {
	now := m.d.Clock.Now()
	s := &session{m: m, id: cl.Sid, idb: idb, sub: cl.Sub, osu: cl.Osu, prf: cl.Prf, link: l, dc: dc, tr: tr, window: window,
		ring: NewRing(m.d.Config.ScrollbackBytes), lastInput: now, idle: dc.idle(), detach: dc.detach(), kick: make(chan struct{}, 1),
		done: make(chan struct{})}
	m.mu.Lock()
	_, exists := m.sessions[cl.Sid]
	if exists || !m.currentLocked(cl.Sid, dc) {
		m.mu.Unlock()
		return nil, false
	}
	m.sessions[cl.Sid] = s
	m.mu.Unlock()
	go s.watch()
	return s, true
}

// current reports whether dc is still the data connection of the session id and the session was
// neither closed nor ended.
func (m *Manager) current(id string, dc *dataConn) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentLocked(id, dc)
}

func (m *Manager) currentLocked(id string, dc *dataConn) bool {
	_, reported := m.reported[id]
	return !m.closed && !reported && m.conns[id] == dc
}

// forget removes a session whose PTY did not start; a pty.stop covers a start the helper still
// completes after the service gave up waiting.
func (m *Manager) forget(s *session) {
	s.link.stop(s.id)
	m.mu.Lock()
	if m.sessions[s.id] == s {
		delete(m.sessions, s.id)
	}
	m.mu.Unlock()
	s.mu.Lock()
	if !s.ended {
		s.ended = true
		close(s.done)
	}
	s.mu.Unlock()
}

// wasReported reports a session whose end this process reported.
func (m *Manager) wasReported(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.reported[id]
	return ok
}

// markReported records that the end of id was reported or the server closed it.
func (m *Manager) markReported(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reported[id] = struct{}{}
}

// closeAfterReport leaves the close of dc to the server after session.ended; only when the server
// has not closed it within window does the host service close it with 1000 (Design note 6).
func (m *Manager) closeAfterReport(id string, dc *dataConn, window time.Duration) {
	m.dropConn(id, dc)
	if window <= 0 {
		window = time.Millisecond
	}
	go func() {
		t := m.d.Clock.NewTimer(window)
		defer t.Stop()
		select {
		case <-dc.done:
		case <-t.C():
			dc.close(CodeNormal, "")
		}
	}()
}

// run pumps the frames of the device on dc to the helper until dc ends.
func (s *session) run(dc *dataConn, tr *noise.Transport) {
	for msg := range dc.in {
		pt, err := tr.Decrypt(msg)
		var f frame.Frame
		if err == nil {
			f, err = frame.Decode(pt, frame.FromDevice)
		}
		if err != nil || f.Type == frame.TypeStart {
			// A fault of the encrypted stream ends the session (Design note 7).
			s.end(endSpec{reason: ReasonOpenFailed, detail: DetailHandshakeFailed, dc: dc, window: s.window})
			s.drain(dc)
			return
		}
		if s.isEnded() {
			continue
		}
		if f.IsInput() {
			s.input()
		}
		switch f.Type {
		case frame.TypeData:
			s.link.data(s.idb, pt)
		case frame.TypeResize:
			s.link.resize(s.id, f.Cols, f.Rows)
		case frame.TypeClose:
			s.end(endSpec{reason: ReasonClientClosed, dc: dc, window: s.window})
		case frame.TypeStart, frame.TypeExit, frame.TypeReplayBegin, frame.TypeReplayEnd:
			// start was handled above; the others never come from the device (frame.Decode).
		}
	}
	<-dc.done
	s.m.dropConn(s.id, dc)
	if dc.closeCode() == CodeClosed {
		s.end(endSpec{reason: ReasonClientClosed, silent: true})
		return
	}
	s.detached(dc)
}

// drain discards the rest of dc until it ends.
func (s *session) drain(dc *dataConn) {
	for range dc.in {
	}
}

func (s *session) isEnded() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ended
}

func (s *session) input() {
	s.mu.Lock()
	s.lastInput = s.m.d.Clock.Now()
	s.mu.Unlock()
}

// detached makes the session detached when dc was its data connection: the detach timer starts.
func (s *session) detached(dc *dataConn) {
	s.mu.Lock()
	if s.ended || s.dc != dc {
		s.mu.Unlock()
		return
	}
	s.dc, s.tr = nil, nil
	s.detachedAt = s.m.d.Clock.Now()
	s.mu.Unlock()
	s.m.d.Log.LogAttrs(context.Background(), slog.LevelInfo, msgDetached, slog.String(attrSession, s.id))
	s.poke()
}

func (s *session) poke() {
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

// beginAttach returns the scrollback as it is when start arrived and collects the output from now
// on until the replay.
func (s *session) beginAttach() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attaching, s.attachBuf = true, nil
	return s.ring.Snapshot()
}

// attach makes dc the data connection: replay_begin, the snapshot, replay_end, the output since
// start, then live output. The values of the hello of dc apply from now on.
func (s *session) attach(dc *dataConn, tr *noise.Transport, snap []byte, window time.Duration) bool {
	s.mu.Lock()
	if s.ended {
		s.mu.Unlock()
		dc.close(CodeNormal, "")
		return false
	}
	old := s.dc
	s.dc, s.tr, s.window = dc, tr, window
	s.idle, s.detach = dc.idle(), dc.detach()
	s.sendLocked(frame.ReplayBegin())
	for len(snap) > 0 {
		n := min(len(snap), localchan.MaxData)
		s.sendLocked(frame.Data(snap[:n]))
		snap = snap[n:]
	}
	s.sendLocked(frame.ReplayEnd())
	buf := s.attachBuf
	s.attaching, s.attachBuf = false, nil
	for len(buf) > 0 {
		n := min(len(buf), localchan.MaxData)
		s.sendLocked(frame.Data(buf[:n]))
		buf = buf[n:]
	}
	s.mu.Unlock()
	if old != nil && old != dc {
		old.close(CodeNormal, "")
	}
	s.poke()
	return true
}

// sendLocked encrypts f and queues it on the attached data connection; s.mu is held.
func (s *session) sendLocked(f frame.Frame) {
	if s.dc == nil || s.tr == nil {
		return
	}
	b, err := frame.Encode(f)
	if err != nil {
		return
	}
	c, err := s.tr.Encrypt(b)
	if err != nil {
		return
	}
	s.dc.send(c)
}

// output takes one data frame of the helper: into the scrollback, and to the device when attached.
func (s *session) output(f []byte) {
	fr, err := frame.Decode(f, frame.FromHost)
	if err != nil || fr.Type != frame.TypeData {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	s.ring.Write(fr.Data)
	switch {
	case s.attaching:
		if len(s.attachBuf)+len(fr.Data) <= s.m.d.Config.ScrollbackBytes {
			s.attachBuf = append(s.attachBuf, fr.Data...)
		}
	case s.dc != nil:
		s.sendLocked(fr)
	}
}

// exit sends the exit frame and ends the session with process_exited.
func (s *session) exit(code int) {
	s.mu.Lock()
	s.sendLocked(frame.Exit(int32(code))) //nolint:gosec // exit codes fit 32 bits
	dc, window := s.dc, s.window
	s.mu.Unlock()
	s.end(endSpec{reason: ReasonProcessExited, exitCode: &code, dc: dc, window: window})
}

// end ends the session once: the PTY with its tree (pty.stop), the timers, the scrollback; then
// session.ended unless silent.
func (s *session) end(e endSpec) {
	s.mu.Lock()
	if s.ended {
		s.mu.Unlock()
		return
	}
	s.ended = true
	close(s.done)
	s.ring.Reset()
	s.attachBuf = nil
	dc := s.dc
	s.dc, s.tr = nil, nil
	s.mu.Unlock()
	m := s.m
	m.mu.Lock()
	if m.sessions[s.id] == s {
		delete(m.sessions, s.id)
	}
	m.mu.Unlock()
	s.link.stop(s.id)
	attrs := []slog.Attr{slog.String(attrSession, s.id), slog.String(attrReason, e.reason), slog.String(attrProfile, s.prf), slog.String(attrUser, s.osu)}
	if e.detail != "" {
		attrs = append(attrs, slog.String(attrDetail, e.detail))
	}
	if e.exitCode != nil {
		attrs = append(attrs, slog.Int(attrExit, *e.exitCode))
	}
	m.d.Log.LogAttrs(context.Background(), slog.LevelInfo, msgEnded, attrs...)
	m.markReported(s.id)
	if e.silent {
		if dc != nil {
			dc.close(CodeNormal, "")
		}
		return
	}
	m.report(context.Background(), control.SessionEnded{SessionID: s.id, CloseReason: e.reason, CloseDetail: e.detail, ExitCode: e.exitCode})
	switch {
	case e.dc != nil:
		m.closeAfterReport(s.id, e.dc, e.window)
	case dc != nil:
		m.closeAfterReport(s.id, dc, s.window)
	}
}

// watch ends the session after the idle value without input, or the detach value while detached.
func (s *session) watch() {
	for {
		s.mu.Lock()
		now := s.m.d.Clock.Now()
		next := s.lastInput.Add(s.idle)
		detached := s.dc == nil
		if detached {
			if d := s.detachedAt.Add(s.detach); d.Before(next) {
				next = d
			}
		}
		idleAt, detachAt := s.lastInput.Add(s.idle), s.detachedAt.Add(s.detach)
		s.mu.Unlock()
		switch {
		case !now.Before(idleAt):
			s.end(endSpec{reason: ReasonIdleTimeout})
			return
		case detached && !now.Before(detachAt):
			s.end(endSpec{reason: ReasonDetachExpired})
			return
		}
		t := s.m.d.Clock.NewTimer(next.Sub(now))
		select {
		case <-t.C():
		case <-s.kick:
			t.Stop()
		case <-s.done:
			t.Stop()
			return
		}
	}
}
