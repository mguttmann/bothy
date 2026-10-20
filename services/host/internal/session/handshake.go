// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/noise"
)

// handshake runs one data connection from message 1 to the PTY or the attach.
type handshake struct {
	m       *Manager
	o       control.SessionOpen
	n       control.Notifier
	dc      *dataConn
	sidb    [noise.IDLen]byte
	hostb   [noise.IDLen]byte
	expires time.Time
	// window is the length of the opening window: expires_at minus the arrival of session.open;
	// the host service waits this long for the server to close after a report (Design note 6).
	window time.Duration
}

// confirm kinds (internal/confirm).
const (
	confirmOpen   = "open"
	confirmAttach = "attach"
)

func (h *handshake) run(ctx context.Context) {
	m := h.m
	if m.wasReported(h.o.SessionID) {
		// An attach for a session whose end was reported: the server closed it already.
		h.dc.close(CodeNormal, "")
		m.dropConn(h.o.SessionID, h.dc)
		return
	}
	msg1, ok := h.next(ctx, nil)
	if !ok {
		h.lost()
		return
	}
	resp, err := noise.NewResponder(noise.ResponderConfig{Static: m.d.NoiseKey, SessionID: h.sidb, HostID: h.hostb, Random: m.d.Random})
	if err != nil {
		h.refuse(DetailHandshakeFailed)
		return
	}
	tok, eph, err := resp.ReadMessage1(msg1)
	if err != nil {
		h.refuse(DetailHandshakeFailed)
		return
	}
	cek, _ := noise.EphemeralKeyHash(eph[:])
	offered, res := m.offered()
	kinds := make([]OfferedProfile, 0, len(offered))
	for _, p := range offered {
		kinds = append(kinds, OfferedProfile{Name: p.Name, Kind: p.Kind})
	}
	cl, detail := m.verifier.Verify(Check{Token: string(tok), Keys: h.dc.hello.JWKS.Keys, HostID: m.d.HostID, SessionID: h.o.SessionID,
		Now: m.d.Clock.Now(), Leeway: h.dc.leeway(), Cek: cek, Live: func(sid string) (Live, bool) {
			s, ok := m.liveOf(sid)
			if !ok {
				return Live{}, false
			}
			return Live{Sub: s.sub, Osu: s.osu, Prf: s.prf}, true
		}, Offered: kinds, SessionUsers: m.d.Config.SessionUsers, Workdirs: func(osu string) int {
			if res == nil {
				return 0
			}
			return len(res.Workdirs[osu])
		}, ServerHost: m.d.ServerHost != nil && m.d.ServerHost()})
	clear(tok)
	if detail != "" {
		h.refuse(detail)
		return
	}
	msg2, tr, err := resp.WriteMessage2(encodeJSON(payload2{V: 1, HostVersion: m.d.HostVersion, Profile: cl.Prf}))
	if err != nil {
		h.refuse(DetailHandshakeFailed)
		return
	}
	h.dc.send(msg2)
	if cl.Att == AttAttach {
		h.attach(ctx, cl, tr)
		return
	}
	h.open(ctx, cl, tr, kinds, res)
}

// next waits for the next binary message; deadline nil waits until the connection ends.
func (h *handshake) next(ctx context.Context, deadline <-chan time.Time) ([]byte, bool) {
	select {
	case m, ok := <-h.dc.in:
		return m, ok
	case <-deadline:
		return nil, false
	case <-ctx.Done():
		return nil, false
	}
}

// lost handles a data connection that ended during the handshake: a close with 4100 ends a live
// PTY without a report; any other end leaves a live PTY detached.
func (h *handshake) lost() {
	h.m.dropConn(h.o.SessionID, h.dc)
	<-h.dc.done
	if h.dc.closeCode() == CodeClosed {
		if s, ok := h.m.liveOf(h.o.SessionID); ok {
			s.end(endSpec{reason: ReasonClientClosed, silent: true})
		}
	}
}

// refuse closes the data connection with the close code of detail and the detail as the reason
// (Design note 6); no session.ended. A 4104 refusal of an attach also ends the live PTY.
func (h *handshake) refuse(detail string) {
	m := h.m
	code := CloseCodeOf(detail)
	m.d.Log.LogAttrs(context.Background(), slog.LevelWarn, msgRefused, slog.String(attrSession, h.o.SessionID),
		slog.String(attrDetail, detail), slog.Int(attrCode, code))
	h.dc.close(code, detail)
	m.dropConn(h.o.SessionID, h.dc)
	if code == CodeHandshakeFail {
		if s, ok := m.liveOf(h.o.SessionID); ok {
			s.end(endSpec{reason: ReasonOpenFailed, detail: detail, silent: true})
		}
	}
}

// failOpening reports an opening that ends after message 2 without a PTY and leaves the close of
// the data connection to the server (Design note 6).
func (h *handshake) failOpening(ctx context.Context, cl Claims, detail string) {
	m := h.m
	m.d.Log.LogAttrs(ctx, slog.LevelInfo, msgEnded, slog.String(attrSession, h.o.SessionID), slog.String(attrReason, ReasonOpenFailed),
		slog.String(attrDetail, detail), slog.String(attrProfile, cl.Prf), slog.String(attrUser, cl.Osu))
	m.markReported(h.o.SessionID)
	m.report(ctx, control.SessionEnded{SessionID: h.o.SessionID, CloseReason: ReasonOpenFailed, CloseDetail: detail})
	m.closeAfterReport(h.o.SessionID, h.dc, h.window)
}

// firstFrame waits for the first transport message until expires_at and decodes it; it returns the
// start frame, or the detail of the failure ("" with ok false: the connection ended).
func (h *handshake) firstFrame(ctx context.Context, tr *noise.Transport) (frame.Frame, string, bool) {
	t := h.m.d.Clock.NewTimer(h.expires.Sub(h.m.d.Clock.Now()))
	defer t.Stop()
	select {
	case msg, ok := <-h.dc.in:
		if !ok {
			return frame.Frame{}, "", false
		}
		pt, err := tr.Decrypt(msg)
		if err != nil {
			return frame.Frame{}, DetailHandshakeFailed, true
		}
		f, err := frame.Decode(pt, frame.FromDevice)
		if err != nil || f.Type != frame.TypeStart {
			return frame.Frame{}, DetailHandshakeFailed, true
		}
		return f, "", true
	case <-t.C():
		return frame.Frame{}, DetailOpenTimeout, true
	case <-ctx.Done():
		return frame.Frame{}, "", false
	}
}

// open runs an opening after message 2: start, the gate, the local confirmation, the PTY.
func (h *handshake) open(ctx context.Context, cl Claims, tr *noise.Transport, kinds []OfferedProfile, res *doctor.Result) {
	m := h.m
	f, detail, ok := h.firstFrame(ctx, tr)
	if !ok {
		h.lost()
		return
	}
	if detail != "" {
		h.failOpening(ctx, cl, detail)
		return
	}
	if m.d.Gate != nil {
		if d := m.d.Gate.Admit(ctx, cl.Osu); d != "" {
			h.failOpening(ctx, cl, d)
			return
		}
		res = m.d.Result()
	}
	if m.d.Confirm != nil && !m.d.Confirm.Ask(ctx, confirmOpen, cl.Prf, cl.Osu, m.d.Config.LocalConfirmTimeout) {
		h.failOpening(ctx, cl, DetailLocalConfirmDenied)
		return
	}
	kind := kinds[slices.IndexFunc(kinds, func(p OfferedProfile) bool { return p.Name == cl.Prf })].Kind
	req, d := m.startRequest(cl, kind, res, f.Cols, f.Rows)
	if d != "" {
		h.failOpening(ctx, cl, d)
		return
	}
	l, d := m.link(ctx, cl.Osu)
	if d != "" {
		h.failOpening(ctx, cl, d)
		return
	}
	s := m.newSession(cl, l, h.sidb, h.dc, tr, h.window)
	if d := l.start(ctx, req); d != "" {
		m.forget(s)
		h.failOpening(ctx, cl, d)
		return
	}
	req.APIKey = ""
	m.d.Log.LogAttrs(ctx, slog.LevelInfo, msgStarted, slog.String(attrSession, cl.Sid), slog.String(attrProfile, cl.Prf), slog.String(attrUser, cl.Osu))
	m.started(ctx, cl.Sid)
	s.run(h.dc, tr)
}

// attach runs an attach after message 2: start, the local confirmation, the replay.
func (h *handshake) attach(ctx context.Context, cl Claims, tr *noise.Transport) {
	m := h.m
	s, live := m.liveOf(cl.Sid)
	if !live {
		h.dc.close(CodeNormal, "")
		m.dropConn(cl.Sid, h.dc)
		return
	}
	f, detail, ok := h.firstFrame(ctx, tr)
	if !ok {
		h.lost()
		return
	}
	switch detail {
	case "":
	case DetailOpenTimeout:
		// No start on this connection: the PTY stays detached for the next session.open.
		h.dc.close(CodeNormal, "")
		m.dropConn(cl.Sid, h.dc)
		return
	default:
		s.end(endSpec{reason: ReasonOpenFailed, detail: detail, dc: h.dc, window: h.window})
		return
	}
	snap := s.beginAttach()
	if m.d.Confirm != nil && !m.d.Confirm.Ask(ctx, confirmAttach, cl.Prf, cl.Osu, m.d.Config.LocalConfirmTimeout) {
		// The person at the machine refused access to this session: it ends.
		s.end(endSpec{reason: ReasonOpenFailed, detail: DetailLocalConfirmDenied, dc: h.dc, window: h.window})
		return
	}
	s.link.resize(cl.Sid, f.Cols, f.Rows)
	if !s.attach(h.dc, tr, snap, h.window) {
		return
	}
	s.run(h.dc, tr)
}
