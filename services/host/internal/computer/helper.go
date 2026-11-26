// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OWNER/bothy/services/host/internal/confirm"
)

// Endpoints of the helper channel (docs/spec/27-computer-use.md 9.4). The Linux folder comes from
// RuntimeDirectory=bothy-host-computer of the service unit, the macOS folder from install.
const (
	linuxEndpoint   = "/run/bothy-host-computer/computer.sock"
	darwinEndpoint  = "/Library/Application Support/bothy-host/computer/computer.sock"
	windowsEndpoint = `\\.\pipe\bothy-host-computer`
)

// Endpoint returns the endpoint of goos: a Unix socket path on Linux and macOS, the name of the
// named pipe on Windows. Any other goos gets the Linux path.
func Endpoint(goos string) string {
	switch goos {
	case "windows":
		return windowsEndpoint
	case "darwin":
		return darwinEndpoint
	}
	return linuxEndpoint
}

// HelperState is the mirror of the helper's hello and state frames.
type HelperState struct {
	// Connected is a trusted helper with a hello; Refused is set when the last connection failed
	// the peer rule and no trusted helper connected after it.
	Connected, Refused bool
	Enabled            bool
	Generation         int64
	OSAccount          string
	Permissions        Permissions
	HelperVersion      string
	Locked             bool
	SecureDesktop      bool
}

// EventKind names a change of the helper that the manager acts on.
type EventKind int

// Kinds of Event.
const (
	// EventState: the state changed (also a connect, a refusal, a lock or a switch-off).
	EventState EventKind = iota + 1
	// EventStop: Esc or the indicator stopped a session.
	EventStop
	// EventGone: the trusted helper connection broke.
	EventGone
)

// Event is one change of the helper; Before and After are the states around it.
type Event struct {
	Kind          EventKind
	Before, After HelperState
	Stop          Stop
}

// HelperOptions configures NewHelper.
type HelperOptions struct {
	// Listener is the endpoint (confirm.Listen of Endpoint); tests pass their own.
	Listener net.Listener
	// PeerOf returns the peer of an accepted connection (confirm.PeerOf in production).
	PeerOf func(net.Conn) (confirm.Peer, error)
	// Bind captures the peer process of an accepted connection (DefaultBind in production); nil
	// binds none, which only the Linux rule accepts.
	Bind func(c net.Conn, p confirm.Peer, accepted time.Time) (Process, error)
	// Rule is the peer rule of 27 9.4.
	Rule PeerRule
	// MaxBytes is host_service.computer_frame_max_bytes.
	MaxBytes int
	// OnEvent receives every change; it must not block.
	OnEvent func(Event)
	// Log receives codes only; nil discards.
	Log *slog.Logger
}

// Errors of the helper channel.
var (
	// ErrHelperAbsent is a request without a trusted helper connection.
	ErrHelperAbsent = errors.New("computer: no helper connected")
	// ErrHelperGone is a request whose connection broke before the answer.
	ErrHelperGone = errors.New("computer: helper connection broke")
)

// Reply is the answer of the helper to capture, do or position; Image holds the image bytes of a
// capture, only in memory.
type Reply struct {
	Done  Done
	Image []byte
}

// Helper is the host side of the local channel to the computer helper: it accepts one trusted
// helper at a time, keeps the mirror of its state, and sends requests. It is safe for concurrent
// use.
type Helper struct {
	o   HelperOptions
	log *slog.Logger

	mu      sync.Mutex
	conn    *helperConn
	peer    confirm.Peer
	state   HelperState
	seq     int64
	pending map[int64]chan Reply
}

// helperConn is one trusted connection.
type helperConn struct {
	c    net.Conn
	proc Process
	wmu  sync.Mutex
	// refused is set when a later check of the peer rule closed the connection; the report then
	// says refused, not absent.
	refused atomic.Bool
}

// closeProc releases the binding of a peer process, if any.
func closeProc(p Process) {
	if p != nil {
		_ = p.Close()
	}
}

// NewHelper returns a Helper for o; Serve starts it.
func NewHelper(o HelperOptions) *Helper {
	log := o.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Helper{o: o, log: log, pending: map[int64]chan Reply{}}
}

// Serve accepts helper connections until ctx ends; then it closes the listener and the connection.
// Every connection is checked with the peer rule before it is read; a refused one is closed and
// logged with computer_peer_refused.
func (h *Helper) Serve(ctx context.Context) error {
	if h.o.Listener == nil {
		return errors.New("computer: no listener")
	}
	stop := context.AfterFunc(ctx, func() {
		_ = h.o.Listener.Close()
		h.mu.Lock()
		c := h.conn
		h.mu.Unlock()
		if c != nil {
			_ = c.c.Close()
		}
	})
	defer stop()
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		c, err := h.o.Listener.Accept()
		accepted := time.Now()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.accept(ctx, c, accepted)
		}()
	}
}

// accept checks the peer of c (accepted at accepted) with the process the connection captured,
// waits for hello, checks the peer again and serves the connection.
func (h *Helper) accept(ctx context.Context, c net.Conn, accepted time.Time) {
	var peer confirm.Peer
	var proc Process
	var err error
	if h.o.PeerOf == nil {
		err = refuse(ReasonNoPeer)
	} else if peer, err = h.o.PeerOf(c); err != nil {
		err = refuse(ReasonNoPeer)
	} else if h.o.Bind != nil {
		if proc, err = h.o.Bind(c, peer, accepted); err != nil {
			err = refuse(ReasonNoBinding)
		}
	}
	if err == nil {
		err = h.o.Rule.Check(ctx, peer, proc)
	}
	if err != nil {
		closeProc(proc)
		h.refused(ctx, c, RefusalReason(err))
		return
	}
	// A peer that never sends hello must not hold Serve past its end.
	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	kind, body, err := ReadFrame(c, h.o.MaxBytes)
	stop()
	op, msg, derr := DecodeMsg(body)
	hello, ok := msg.(*Hello)
	if err != nil || kind != KindJSON || derr != nil || op != OpHello || !ok {
		closeProc(proc)
		h.log.LogAttrs(ctx, slog.LevelWarn, LogPeerRefused, slog.String(attrCode, LogPeerRefused), slog.String(attrRefusal, ReasonNoHello))
		_ = c.Close()
		return
	}
	// The process behind the connection must still be the one admitted before hello (27 9.4).
	if err := h.o.Rule.Check(ctx, peer, proc); err != nil {
		closeProc(proc)
		h.refused(ctx, c, RefusalReason(err))
		return
	}
	h.log.LogAttrs(ctx, slog.LevelInfo, LogHelperConnected, slog.String(attrCode, LogHelperConnected),
		slog.Bool(attrEnabled, hello.Enabled), slog.String(attrHelperVersion, hello.HelperVersion))
	hc := &helperConn{c: c, proc: proc}
	h.mu.Lock()
	old := h.conn
	h.conn, h.peer = hc, peer
	var pend map[int64]chan Reply
	if old != nil {
		pend = h.pending
		h.pending = map[int64]chan Reply{}
	}
	h.mu.Unlock()
	if old != nil {
		// A newer trusted helper replaces the older one: the older one's session cannot go on with
		// a helper that never showed its indicator, so it ends like a crash of the helper.
		_ = old.c.Close()
		for _, ch := range pend {
			close(ch)
		}
		h.emit(Event{Kind: EventGone})
	}
	h.update(func(s *HelperState) {
		*s = HelperState{Connected: true, Enabled: hello.Enabled, Generation: hello.Generation, OSAccount: hello.OSAccount,
			Permissions: hello.Permissions, HelperVersion: hello.HelperVersion, Locked: hello.Locked}
	})
	h.read(hc)
}

// refused closes a connection whose peer the rule refused, logs computer_peer_refused with the
// reason and reports the helper as refused while no trusted helper is connected.
func (h *Helper) refused(ctx context.Context, c net.Conn, reason string) {
	h.log.LogAttrs(ctx, slog.LevelWarn, LogPeerRefused, slog.String(attrCode, LogPeerRefused), slog.String(attrRefusal, reason))
	_ = c.Close()
	h.update(func(s *HelperState) {
		if !s.Connected {
			s.Refused = true
		}
	})
}

// read serves one trusted connection until it breaks or sends a frame against the protocol.
func (h *Helper) read(hc *helperConn) {
	defer h.gone(hc)
	for {
		kind, body, err := ReadFrame(hc.c, h.o.MaxBytes)
		if err != nil || kind != KindJSON {
			return
		}
		op, msg, err := DecodeMsg(body)
		if err != nil {
			return
		}
		switch m := msg.(type) {
		case *StateMsg:
			h.update(func(s *HelperState) { merge(s, m) })
		case *Stop:
			h.emit(Event{Kind: EventStop, Stop: *m})
		case *Done:
			if !h.done(hc, m) {
				return
			}
		default:
			// Only hello, state, stop and done come from the helper; a second hello or a host op
			// breaks the protocol.
			_ = op
			return
		}
	}
}

// doneCodes are the codes of done (27 9.4); another one is answered as os_refused.
var doneCodes = []string{CodeOK, CodeScreenLocked, CodeSecureDesktop, CodeOSRefused, CodeInvalidAction}

// done reads the image of a capture answer and hands the reply to its request.
func (h *Helper) done(hc *helperConn, d *Done) bool {
	if !slices.Contains(doneCodes, d.Code) {
		d.Code = CodeOSRefused
	}
	r := Reply{Done: *d}
	if d.Image != nil {
		kind, img, err := ReadFrame(hc.c, h.o.MaxBytes)
		if err != nil || kind != KindImage || int64(len(img)) != d.Image.Bytes {
			return false
		}
		r.Image = img
	}
	h.mu.Lock()
	ch := h.pending[d.Seq]
	delete(h.pending, d.Seq)
	h.mu.Unlock()
	if ch != nil {
		ch <- r
	}
	return true
}

// merge applies the members of a state frame.
func merge(s *HelperState, m *StateMsg) {
	if m.Enabled != nil {
		s.Enabled = *m.Enabled
	}
	if m.Generation != nil {
		s.Generation = *m.Generation
	}
	if m.OSAccount != nil {
		s.OSAccount = *m.OSAccount
	}
	if m.Permissions != nil {
		s.Permissions = *m.Permissions
	}
	if m.HelperVersion != nil {
		s.HelperVersion = *m.HelperVersion
	}
	if m.Locked != nil {
		s.Locked = *m.Locked
	}
	if m.SecureDesktop != nil {
		s.SecureDesktop = *m.SecureDesktop
	}
}

// gone forgets hc; when it was the current connection the helper counts as absent, every pending
// request fails, and EventGone goes out.
func (h *Helper) gone(hc *helperConn) {
	_ = hc.c.Close()
	closeProc(hc.proc)
	h.mu.Lock()
	if h.conn != hc {
		h.mu.Unlock()
		return
	}
	h.conn = nil
	h.log.LogAttrs(context.Background(), slog.LevelInfo, LogHelperGone, slog.String(attrCode, LogHelperGone))
	before := h.state
	h.state = HelperState{Refused: hc.refused.Load()}
	pend := h.pending
	h.pending = map[int64]chan Reply{}
	after := h.state
	h.mu.Unlock()
	for _, ch := range pend {
		close(ch)
	}
	h.emit(Event{Kind: EventGone, Before: before, After: after})
}

// update changes the state with f and emits EventState when it changed.
func (h *Helper) update(f func(*HelperState)) {
	h.mu.Lock()
	before := h.state
	f(&h.state)
	after := h.state
	h.mu.Unlock()
	if before != after {
		h.emit(Event{Kind: EventState, Before: before, After: after})
	}
}

func (h *Helper) emit(e Event) {
	if h.o.OnEvent != nil {
		h.o.OnEvent(e)
	}
}

// State returns the current mirror.
func (h *Helper) State() HelperState {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state
}

// Recheck runs the peer rule again on the current connection, with the process it captured (27
// 9.4: again before every session);
// a failure closes the connection and returns ErrPeerRefused, no connection ErrHelperAbsent.
func (h *Helper) Recheck(ctx context.Context) error {
	h.mu.Lock()
	hc, peer := h.conn, h.peer
	h.mu.Unlock()
	if hc == nil {
		return ErrHelperAbsent
	}
	if err := h.o.Rule.Check(ctx, peer, hc.proc); err != nil {
		h.log.LogAttrs(ctx, slog.LevelWarn, LogPeerRefused, slog.String(attrCode, LogPeerRefused), slog.String(attrRefusal, RefusalReason(err)))
		hc.refused.Store(true)
		_ = hc.c.Close()
		return ErrPeerRefused
	}
	return nil
}

// NextSeq returns a new sequence number of a request.
func (h *Helper) NextSeq() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seq++
	return h.seq
}

// Send writes one message without an answer (show, hide, release, disable). Without a connection
// it returns ErrHelperAbsent.
func (h *Helper) Send(m any) error {
	h.mu.Lock()
	hc := h.conn
	h.mu.Unlock()
	if hc == nil {
		return ErrHelperAbsent
	}
	return h.write(hc, m)
}

func (h *Helper) write(hc *helperConn, m any) error {
	b, err := EncodeMsg(m, h.o.MaxBytes)
	if err != nil {
		return err
	}
	hc.wmu.Lock()
	defer hc.wmu.Unlock()
	_, err = hc.c.Write(b)
	return err
}

// Request writes m (capture, do or position with seq) and waits for the done of seq, until ctx
// ends or the connection breaks.
func (h *Helper) Request(ctx context.Context, seq int64, m any) (Reply, error) {
	ch := make(chan Reply, 1)
	h.mu.Lock()
	hc := h.conn
	if hc == nil {
		h.mu.Unlock()
		return Reply{}, ErrHelperAbsent
	}
	h.pending[seq] = ch
	h.mu.Unlock()
	if ctx.Err() != nil {
		// The session ended (its release and hide may be out already): nothing more is sent.
		h.forget(seq)
		return Reply{}, ctx.Err()
	}
	if err := h.write(hc, m); err != nil {
		h.forget(seq)
		return Reply{}, ErrHelperGone
	}
	select {
	case r, ok := <-ch:
		if !ok {
			return Reply{}, ErrHelperGone
		}
		return r, nil
	case <-ctx.Done():
		h.forget(seq)
		return Reply{}, ctx.Err()
	}
}

func (h *Helper) forget(seq int64) {
	h.mu.Lock()
	delete(h.pending, seq)
	h.mu.Unlock()
}

// Installed paths of the helper executable (docs/spec/27-computer-use.md 9.3; Ruling D-CU-03-7):
// macOS inside the app bundle in the program folder of the host package, Windows next to the host
// service under Program Files (a condition of uiAccess). Linux has no helper.
const (
	darwinHelperPath  = "/Library/Application Support/bothy-host/bin/bothy-computer-helper.app/Contents/MacOS/bothy-computer-helper"
	windowsHelperName = `\bothy-host\bothy-computer-helper.exe`
)

// HelperPath returns the installed helper path of goos; programFiles is %ProgramFiles% on Windows.
// Linux gives "".
func HelperPath(goos, programFiles string) string {
	switch goos {
	case "darwin":
		return darwinHelperPath
	case "windows":
		return programFiles + windowsHelperName
	}
	return ""
}
