// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OWNER/bothy/services/host/internal/clock"
	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/noise"
)

// actionQueue bounds the actions of one session that wait for the worker; bothy-computer-mcp sends
// one action at a time and waits for its result, so a fuller queue is a stream fault. A protocol
// constant, not a product limit.
const actionQueue = 64

// drainWait bounds the wait for the last frames (cu_state stopped) before the data connection
// closes; a protocol constant of the close, not a product window.
const drainWait = 2 * time.Second

// Session states (docs/spec/27-computer-use.md 8.1).
const (
	stateOpening = "opening"
	stateActive  = frame.StateActive
	statePaused  = frame.StatePaused
	stateClosed  = "closed"
)

// endSpec is one end of a session: the close reason and detail of 27 8.3; silent ends send no
// computer.ended (the server ended the session itself).
type endSpec struct {
	reason, detail string
	silent         bool
	// code is the close code of the data connection; 0 means 1000.
	code int
}

// csession is one computer session from computer.open to its end.
type csession struct {
	m       *Manager
	o       control.ComputerOpen
	n       control.ComputerNotifier
	expires time.Time
	sidb    [noise.IDLen]byte
	hostb   [noise.IDLen]byte
	ctx     context.Context
	cancel  context.CancelFunc
	scale   *ScaleMap
	queue   chan frame.Action
	// sendMu orders the frames to bothy-computer-mcp: a result and its image frames stay together,
	// and the transport encrypts one message at a time.
	sendMu sync.Mutex
	st     Stream
	tr     *noise.Transport

	mu          sync.Mutex
	state       string
	shown       bool
	ended       bool
	pausedAt    time.Time
	pausedTotal time.Duration
	counts      map[string]int64
	total       int64
	apps        []control.AppIdentity
	front       control.AppIdentity
	lastSeq     int64
	stateSent   time.Time
	stateDirty  bool
	stateTimer  clock.Timer
	// captureCost is the duration of the last screenshot in nanoseconds, the expected cost of a
	// look of the adaptive wait (quiet.go).
	captureCost atomic.Int64
}

func newSession(m *Manager, o control.ComputerOpen, n control.ComputerNotifier, expires time.Time, sidb, hostb [noise.IDLen]byte) *csession {
	ctx, cancel := context.WithCancel(m.ctx)
	return &csession{m: m, o: o, n: n, expires: expires, sidb: sidb, hostb: hostb, ctx: ctx, cancel: cancel, scale: NewScaleMap(),
		queue: make(chan frame.Action, actionQueue), state: stateOpening, counts: map[string]int64{}}
}

// show tells the helper to show the indicator (27 5.1 step 7).
func (s *csession) show() {
	if err := s.m.c.Helper.Send(Show{Op: OpShow, SessionID: s.o.SessionID, AgentName: s.o.AgentName}); err == nil {
		s.mu.Lock()
		s.shown = true
		s.mu.Unlock()
	}
}

// serve dials the data connection, runs the handshake of 27 5.3 and then the session.
func (s *csession) serve() {
	m := s.m
	dctx, cancel := context.WithDeadline(s.ctx, s.expires.Add(time.Second))
	st, err := m.c.Dial(dctx, s.o.DataPath, s.o.SessionID)
	cancel()
	if err != nil {
		m.c.Log.LogAttrs(s.ctx, slog.LevelWarn, msgNotOpened, slog.String(attrSession, s.o.SessionID))
		s.end(endSpec{reason: ReasonOpenFailed, detail: DetailOpenTimeout})
		return
	}
	s.mu.Lock()
	s.st = st
	ended := s.ended
	s.mu.Unlock()
	if ended {
		st.Close(codeNormal, "")
		return
	}
	msg1, ok := s.message1(st)
	if !ok {
		return
	}
	cl, tr, detail := s.handshake(st, msg1)
	if detail != "" {
		s.refuse(detail)
		return
	}
	s.mu.Lock()
	if s.ended {
		s.mu.Unlock()
		return
	}
	s.tr = tr
	state := stateActive
	if m.c.Helper.State().Locked {
		state = statePaused
		s.pausedAt = m.c.Clock.Now()
	}
	s.state = state
	s.mu.Unlock()
	reason := ""
	if state == statePaused {
		reason = reasonLocked
	}
	s.sendState(state, reason)
	if n := s.n; n != nil {
		_ = n.ComputerStarted(s.ctx, control.ComputerStarted{SessionID: s.o.SessionID, State: state})
	}
	m.c.Log.LogAttrs(s.ctx, slog.LevelInfo, msgStarted, slog.String(attrSession, cl.Sid), slog.String(attrState, state))
	go s.worker()
	s.run()
}

// message1 waits for message 1 until expires_at; false when the session ended meanwhile.
func (s *csession) message1(st Stream) ([]byte, bool) {
	t := s.m.c.Clock.NewTimer(s.expires.Sub(s.m.c.Clock.Now()))
	defer t.Stop()
	select {
	case msg, ok := <-st.In():
		if !ok {
			s.end(endSpec{reason: ReasonConnectionLost})
			return nil, false
		}
		return msg, true
	case <-t.C():
		s.end(endSpec{reason: ReasonOpenFailed, detail: DetailOpenTimeout})
		return nil, false
	case <-s.ctx.Done():
		return nil, false
	}
}

// payload2 is the payload of message 2 (27 5.1 step 9). Features names what this host understands
// beyond version 1: quick, the member of cu_action for a batch item before the last (27 6.4).
type payload2 struct {
	V           int      `json:"v"`
	HostVersion string   `json:"host_version"`
	Platform    string   `json:"platform"`
	Features    []string `json:"features"`
}

// hostFeatures are the features of payload2.
var hostFeatures = []string{"quick"}

// handshake runs the steps 0 to 11 of 27 5.3 and writes message 2; it returns the claims and the
// transport, or the close detail of the first failing step.
func (s *csession) handshake(st Stream, msg1 []byte) (Claims, *noise.Transport, string) {
	m := s.m
	resp, err := noise.NewResponder(noise.ResponderConfig{Static: m.c.NoiseKey, SessionID: s.sidb, HostID: s.hostb, Random: m.c.Random,
		Label: noise.ComputerPrologueLabel})
	if err != nil {
		return Claims{}, nil, DetailHandshakeFailed
	}
	tok, eph, err := resp.ReadMessage1(msg1)
	if err != nil {
		return Claims{}, nil, DetailHandshakeFailed
	}
	cek, _ := noise.EphemeralKeyHash(eph[:])
	cl, detail := m.verifier.Verify(Check{Token: string(tok), Keys: st.Keys(), HostID: m.c.HostID, SessionID: s.o.SessionID,
		Now: m.c.Clock.Now(), Leeway: st.Leeway(), Cek: cek,
		Open: OpenBinding{SessionID: s.o.SessionID, RunID: s.o.RunID, AgentID: s.o.AgentID, UserID: s.o.UserID}})
	clear(tok)
	if detail != "" {
		return Claims{}, nil, detail
	}
	if d := m.admit(s.ctx, s.o.SessionID, cl.Gen); d != "" {
		return Claims{}, nil, d
	}
	if s.isEnded() {
		// The session ended while the checks ran; its end freed the lock already, so the lock
		// admit took again is freed here (an end never leaves the controller lock held).
		m.lock.release(s.o.SessionID)
		return Claims{}, nil, DetailHandshakeFailed
	}
	b, _ := json.Marshal(payload2{V: 1, HostVersion: m.c.HostVersion, Platform: m.c.Platform, Features: hostFeatures})
	msg2, tr, err := resp.WriteMessage2(b)
	if err != nil {
		return Claims{}, nil, DetailHandshakeFailed
	}
	if !st.Send(s.ctx, msg2) {
		return Claims{}, nil, DetailHandshakeFailed
	}
	return cl, tr, ""
}

// refuse closes the data connection with the close code and detail of a failed step and ends the
// session; a failure of step 0 reports nothing (27 5.3), every later one computer.ended
// open_failed with the detail.
func (s *csession) refuse(detail string) {
	s.m.c.Log.LogAttrs(s.ctx, slog.LevelWarn, msgRefused, slog.String(attrSession, s.o.SessionID), slog.String(attrDetail, detail),
		slog.Int(attrCode, CloseCodeOf(detail)))
	s.end(endSpec{reason: ReasonOpenFailed, detail: detail, silent: detail == DetailHandshakeFailed, code: CloseCodeOf(detail)})
}

// run reads the frames of bothy-computer-mcp until the session ends.
func (s *csession) run() {
	for {
		select {
		case msg, ok := <-s.st.In():
			if !ok {
				s.end(endSpec{reason: ReasonConnectionLost})
				return
			}
			if !s.frame(msg) {
				return
			}
		case <-s.ctx.Done():
			return
		}
	}
}

// frame handles one message; false ends the loop.
func (s *csession) frame(msg []byte) bool {
	pt, err := s.tr.Decrypt(msg)
	if err != nil {
		s.end(endSpec{reason: ReasonError, detail: DetailStreamFault})
		return false
	}
	f, err := frame.Decode(pt, frame.KindComputer, frame.FromDevice)
	if err != nil {
		s.end(endSpec{reason: ReasonError, detail: DetailStreamFault})
		return false
	}
	if f.Type == frame.TypeClose {
		s.end(endSpec{reason: ReasonDone})
		return false
	}
	a, err := f.ActionBody()
	s.mu.Lock()
	inOrder := err == nil && a.Seq == s.lastSeq+1
	if inOrder {
		s.lastSeq = a.Seq
	}
	s.mu.Unlock()
	if !inOrder {
		s.end(endSpec{reason: ReasonError, detail: DetailStreamFault})
		return false
	}
	select {
	case s.queue <- a:
		return true
	default:
		s.end(endSpec{reason: ReasonError, detail: DetailStreamFault})
		return false
	}
}

// worker runs the queued actions in order until the session ends.
func (s *csession) worker() {
	for {
		select {
		case a := <-s.queue:
			s.act(a)
		case <-s.ctx.Done():
			return
		}
	}
}

// pause handles a lock (27 8.2 item 1): paused with locked, pending actions dropped with
// screen_locked, every key and button released, cu_state and computer.state sent.
func (s *csession) pause() {
	s.mu.Lock()
	if s.state != stateActive {
		// An opening reads the lock from the helper state after message 1.
		s.mu.Unlock()
		return
	}
	s.state = statePaused
	s.pausedAt = s.m.c.Clock.Now()
	s.mu.Unlock()
	s.dropQueued()
	_ = s.m.c.Helper.Send(Release{Op: OpRelease})
	s.sendState(statePaused, reasonLocked)
	s.reportState(true)
}

// resume handles the unlock (27 8.2 item 3).
func (s *csession) resume() {
	s.mu.Lock()
	if s.state != statePaused || s.tr == nil {
		s.mu.Unlock()
		return
	}
	s.state = stateActive
	s.pausedTotal += s.m.c.Clock.Now().Sub(s.pausedAt)
	s.mu.Unlock()
	s.sendState(stateActive, "")
	s.reportState(true)
}

// dropQueued answers every queued action with screen_locked.
func (s *csession) dropQueued() {
	for {
		select {
		case a := <-s.queue:
			s.count(a.Action)
			s.sendResult(frame.Result{Seq: a.Seq, Code: CodeScreenLocked, App: s.frontApp()}, nil, 0)
		default:
			return
		}
	}
}

func (s *csession) isEnded() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ended
}

func (s *csession) paused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == statePaused
}

// end ends the session once (27 8.3, 8.4): pending input dropped, every key and button released,
// the indicator hidden, cu_state stopped, the data connection closed, the controller lock freed,
// and computer.ended reported unless the server ended the session itself.
func (s *csession) end(e endSpec) {
	s.mu.Lock()
	if s.ended {
		s.mu.Unlock()
		return
	}
	s.ended = true
	shown := s.shown
	st, tr := s.st, s.tr
	if s.state == statePaused {
		s.pausedTotal += s.m.c.Clock.Now().Sub(s.pausedAt)
	}
	s.state = stateClosed
	if s.stateTimer != nil {
		s.stateTimer.Stop()
	}
	s.mu.Unlock()
	s.cancel()
	h := s.m.c.Helper
	_ = h.Send(Release{Op: OpRelease})
	if shown {
		_ = h.Send(Hide{Op: OpHide, SessionID: s.o.SessionID})
	}
	if st != nil {
		if tr != nil {
			dctx, cancel := context.WithTimeout(context.Background(), drainWait)
			s.sendMu.Lock()
			s.writeFrame(dctx, stateFrame(frame.StateStopped, e.reason), tr, st)
			s.sendMu.Unlock()
			st.Drain(dctx)
			cancel()
		}
		code, reason := codeNormal, ""
		if e.code != 0 {
			code, reason = e.code, e.detail
		}
		st.Close(code, reason)
	}
	s.m.forget(s)
	s.m.c.Log.LogAttrs(context.Background(), slog.LevelInfo, msgEnded, slog.String(attrSession, s.o.SessionID),
		slog.String(attrReason, e.reason), slog.String(attrDetail, e.detail))
	if !e.silent {
		s.m.report(context.Background(), endedOf(s.o.SessionID, e.reason, e.detail, s))
	}
}

// stateFrame returns the cu_state frame.
func stateFrame(state, reason string) frame.Frame {
	f, _ := frame.NewState(frame.State{State: state, Reason: reason})
	return f
}

// sendState sends cu_state to bothy-computer-mcp.
func (s *csession) sendState(state, reason string) {
	s.mu.Lock()
	st, tr := s.st, s.tr
	s.mu.Unlock()
	if st == nil || tr == nil {
		return
	}
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	s.writeFrame(s.ctx, stateFrame(state, reason), tr, st)
}

// writeFrame encrypts and queues one frame; the caller holds sendMu. False when it was not queued.
func (s *csession) writeFrame(ctx context.Context, f frame.Frame, tr *noise.Transport, st Stream) bool {
	b, err := frame.Encode(f)
	if err != nil {
		return false
	}
	msg, err := tr.Encrypt(b)
	if err != nil {
		return false
	}
	return st.Send(ctx, msg)
}

// sendResult sends cu_result and, with img, the image frames: cu_image_begin, the data frames and
// cu_image_end (27 6.3). The image stays in memory and is cleared after sending.
func (s *csession) sendResult(r frame.Result, img []byte, format int) {
	s.mu.Lock()
	st, tr := s.st, s.tr
	s.mu.Unlock()
	defer clear(img)
	if st == nil || tr == nil {
		return
	}
	r.Image = len(img) > 0
	if !r.Image {
		r.Width, r.Height = 0, 0
		if r.Cursor == nil {
			r.ScaleID = 0
		}
	}
	rf, err := frame.NewResult(r)
	if err != nil {
		return
	}
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	if !s.writeFrame(s.ctx, rf, tr, st) || !r.Image {
		return
	}
	bf, err := frame.NewImageBegin(frame.ImageBegin{Seq: r.Seq, Format: formatName(format), Width: r.Width, Height: r.Height,
		ScaleID: r.ScaleID, Bytes: int64(len(img))})
	if err != nil || !s.writeFrame(s.ctx, bf, tr, st) {
		return
	}
	for _, c := range Chunks(img) {
		if !s.writeFrame(s.ctx, frame.ImageData(c), tr, st) {
			return
		}
	}
	s.writeFrame(s.ctx, frame.ImageEnd(), tr, st)
}

// Image formats of sendResult.
const (
	formatJPEG = iota
	formatPNG
)

func formatName(f int) string {
	if f == formatPNG {
		return frame.FormatPNG
	}
	return frame.FormatJPEG
}

// count records one action of kind for action_counts and action_count.
func (s *csession) count(kind string) {
	s.mu.Lock()
	s.counts[kind]++
	s.total++
	s.mu.Unlock()
}

// seeApp records the frontmost app of a done: the current app of the status chip and, up to
// computer.audit_apps_max distinct identities, the apps of the audit entry.
func (s *csession) seeApp(a AppID) {
	if a.ID == "" {
		return
	}
	id := control.AppIdentity(a)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.front = id
	for _, x := range s.apps {
		if x.ID == id.ID && x.Signer == id.Signer {
			return
		}
	}
	if int64(len(s.apps)) < s.o.Settings.AuditAppsMax {
		s.apps = append(s.apps, id)
	}
}

// frontApp returns the frontmost app as cu_result names it.
func (s *csession) frontApp() frame.App {
	s.mu.Lock()
	defer s.mu.Unlock()
	return frame.App{ID: s.front.ID, Name: s.front.Name}
}

// reportState sends computer.state: at once with force (a state change), else at most every
// computer.status_interval while actions run (27 12.3).
func (s *csession) reportState(force bool) {
	clk := s.m.c.Clock
	interval := time.Duration(s.o.Settings.StatusIntervalMs) * time.Millisecond
	s.mu.Lock()
	if s.ended || s.state == stateOpening {
		s.mu.Unlock()
		return
	}
	now := clk.Now()
	if !force && now.Sub(s.stateSent) < interval {
		if !s.stateDirty {
			s.stateDirty = true
			s.stateTimer = clk.NewTimer(interval - now.Sub(s.stateSent))
			go s.awaitState(s.stateTimer)
		}
		s.mu.Unlock()
		return
	}
	s.stateDirty = false
	s.stateSent = now
	msg := control.ComputerState{SessionID: s.o.SessionID, State: s.state, ActionCount: s.total, FrontApp: s.front}
	if s.state == statePaused {
		msg.Reason = reasonLocked
	}
	s.mu.Unlock()
	if n := s.n; n != nil {
		_ = n.ComputerState(s.ctx, msg)
	}
}

// awaitState sends the delayed computer.state when its timer fires.
func (s *csession) awaitState(t clock.Timer) {
	select {
	case <-t.C():
		s.mu.Lock()
		dirty := s.stateDirty
		s.stateDirty = false
		s.stateSent = time.Time{}
		s.mu.Unlock()
		if dirty {
			s.reportState(true)
		}
	case <-s.ctx.Done():
	}
}
