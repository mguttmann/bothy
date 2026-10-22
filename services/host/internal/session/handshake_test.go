// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/clock/clocktest"
	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/localchan"
	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
)

// wantEnded fails the test unless e is the end of sessID with reason and detail.
func wantEnded(t *testing.T, e control.SessionEnded, reason, detail string) {
	t.Helper()
	if e.SessionID != sessID || e.CloseReason != reason || e.CloseDetail != detail {
		t.Errorf("session.ended %+v, want %s/%s", e, reason, detail)
	}
}

// TestOpeningHelperRefusesThePTY: pty.failed of the helper ends the opening with open_failed and
// its detail; the service forgets the session and stops a PTY the helper might still start.
func TestOpeningHelperRefusesThePTY(t *testing.T) {
	r := newPipeRig(t)
	r.helper.answer = func(req localchan.PTYStart) (string, any) {
		return localchan.OpPTYFailed, localchan.PTYFailed{SessionID: req.SessionID, CloseDetail: DetailSharedLocation}
	}
	dev, dc := r.connect("shell", AttOpen)
	r.send(dc, dev, frame.Start(80, 24))
	wantEnded(t, recv(t, r.ctx, "session.ended", r.note.ended), ReasonOpenFailed, DetailSharedLocation)
	if id := recv(t, r.ctx, "pty.stop", r.helper.stops); id != sessID {
		t.Errorf("pty.stop for %q", id)
	}
	if r.mgr.Active() != 0 || len(r.note.started) != 0 {
		t.Errorf("a refused PTY counts as running: active %d", r.mgr.Active())
	}
}

// TestOpeningCommandProfileWithoutConfiguration: a command profile the result offers but the
// configuration does not name ends the opening with profile_unknown before any helper is asked.
func TestOpeningCommandProfileWithoutConfiguration(t *testing.T) {
	res := pipeResult()
	res.Profiles = append(res.Profiles, doctor.Profile{Name: "ghost", DisplayName: "Ghost", Kind: config.KindCommand})
	r := newPipeRig(t, pipeDoctor(res))
	dev, dc := r.connect("ghost", AttOpen)
	r.send(dc, dev, frame.Start(80, 24))
	wantEnded(t, recv(t, r.ctx, "session.ended", r.note.ended), ReasonOpenFailed, DetailProfileUnknown)
	if n := r.helper.dialCount(); n != 0 {
		t.Errorf("the helper was dialled %d times", n)
	}
}

// TestOpeningHelperUnreachable: a session channel that cannot be opened for another reason than a
// failed peer check ends the opening with pty_failed and records no peer mismatch.
func TestOpeningHelperUnreachable(t *testing.T) {
	r := newPipeRig(t)
	r.helper.dialErr = errors.New("helper not running")
	dev, dc := r.connect("shell", AttOpen)
	r.send(dc, dev, frame.Start(80, 24))
	wantEnded(t, recv(t, r.ctx, "session.ended", r.note.ended), ReasonOpenFailed, DetailPTYFailed)
	if p := r.gate.Current(); p.Host != "" {
		t.Errorf("an unreachable helper changed the policy: %+v", p)
	}
}

// TestOpeningConnectionLostBeforeMessage1: a data connection that ends before message 1 is
// forgotten without a report.
func TestOpeningConnectionLostBeforeMessage1(t *testing.T) {
	r := newPipeRig(t)
	r.sessionOpen(20 * time.Second)
	dc := r.srv.Data(r.ctx, sessID, 1)
	until(t, "the connection recorded", func() bool {
		r.mgr.mu.Lock()
		defer r.mgr.mu.Unlock()
		return r.mgr.conns[sessID] != nil
	})
	dc.Close(CodeNormal, "")
	until(t, "the connection forgotten", func() bool {
		r.mgr.mu.Lock()
		defer r.mgr.mu.Unlock()
		return len(r.mgr.conns) == 0
	})
	if len(r.mgr.Live()) != 0 || r.note.endedCount() != 0 {
		t.Errorf("a lost opening left %v, %d reports", r.mgr.Live(), r.note.endedCount())
	}
}

// TestFirstFrameNotDecryptable: a first transport message that does not decrypt ends an opening
// with open_failed/handshake_failed and a running session the same way when it comes on an attach.
func TestFirstFrameNotDecryptable(t *testing.T) {
	for _, att := range []string{AttOpen, AttAttach} {
		t.Run(att, func(t *testing.T) {
			r := newPipeRig(t)
			if att == AttAttach {
				_, dc := r.open()
				r.detach(dc)
			}
			_, dc := r.connect("shell", att)
			if err := dc.SendBinary(r.ctx, []byte("not a transport message")); err != nil {
				t.Fatal(err)
			}
			wantEnded(t, recv(t, r.ctx, "session.ended", r.note.ended), ReasonOpenFailed, DetailHandshakeFailed)
			if att == AttAttach {
				if id := recv(t, r.ctx, "pty.stop", r.helper.stops); id != sessID || r.mgr.Active() != 0 {
					t.Errorf("pty.stop for %q, active %d", id, r.mgr.Active())
				}
			}
		})
	}
}

// TestAttachLostWith4100EndsThePTY: an attach whose data connection the server closes with 4100
// before start ends the PTY without a report (the server closed the session).
func TestAttachLostWith4100EndsThePTY(t *testing.T) {
	r := newPipeRig(t)
	_, dc := r.open()
	r.detach(dc)
	_, dc2 := r.connect("shell", AttAttach)
	dc2.Close(CodeClosed, "")
	if id := recv(t, r.ctx, "pty.stop", r.helper.stops); id != sessID {
		t.Errorf("pty.stop for %q", id)
	}
	until(t, "the end recorded", func() bool { return r.mgr.wasReported(sessID) })
	if r.mgr.Active() != 0 || r.note.endedCount() != 0 {
		t.Errorf("active %d, reports %d", r.mgr.Active(), r.note.endedCount())
	}
}

// TestAttachWithoutStartStaysDetached: an attach that sends no start until expires_at is closed
// with 1000; the PTY stays detached for the next session.open.
func TestAttachWithoutStartStaysDetached(t *testing.T) {
	clk := clocktest.New(time.Now())
	r := newPipeRig(t, pipeClock(clk))
	_, dc := r.open()
	r.detach(dc)
	_, dc2 := r.connect("shell", AttAttach)
	// The watch of the session and the wait for start.
	until(t, "the wait for start", func() bool { return clk.Pending() >= 2 })
	clk.Advance(20 * time.Second)
	if code := r.closedWith(dc2); code != CodeNormal {
		t.Errorf("close code %d", code)
	}
	if r.mgr.Active() != 1 || r.note.endedCount() != 0 {
		t.Errorf("active %d, reports %d", r.mgr.Active(), r.note.endedCount())
	}
	until(t, "detached", func() bool { return r.detachedNow(sessID) })
}

// TestAttachConfirmDenied: the person at the machine refuses the attach; the session ends with
// open_failed/local_confirm_denied.
func TestAttachConfirmDenied(t *testing.T) {
	r := newPipeRig(t, pipeConfirm(func(_ context.Context, kind, _, _ string, _ time.Duration) bool { return kind == confirmOpen }))
	_, dc := r.open()
	r.detach(dc)
	dev2, dc2 := r.connect("shell", AttAttach)
	r.send(dc2, dev2, frame.Start(80, 24))
	wantEnded(t, recv(t, r.ctx, "session.ended", r.note.ended), ReasonOpenFailed, DetailLocalConfirmDenied)
	if id := recv(t, r.ctx, "pty.stop", r.helper.stops); id != sessID {
		t.Errorf("pty.stop for %q", id)
	}
}

// TestAttachAfterSessionClose: a session.close while the attach waits in the confirmation ends the
// session; the attach then replays nothing and closes its data connection with 1000.
func TestAttachAfterSessionClose(t *testing.T) {
	var r *pipeRig
	r = newPipeRig(t, pipeConfirm(func(ctx context.Context, kind, _, _ string, _ time.Duration) bool {
		if kind == confirmAttach {
			r.mgr.Close(ctx, control.SessionClose{SessionID: sessID, CloseReason: "access_revoked"})
		}
		return true
	}))
	_, dc := r.open()
	r.detach(dc)
	dev2, dc2 := r.connect("shell", AttAttach)
	r.send(dc2, dev2, frame.Start(80, 24))
	if code := r.closedWith(dc2); code != CodeNormal {
		t.Errorf("close code %d", code)
	}
	if id := recv(t, r.ctx, "pty.stop", r.helper.stops); id != sessID {
		t.Errorf("pty.stop for %q", id)
	}
	if r.mgr.Active() != 0 || r.note.endedCount() != 0 {
		t.Errorf("active %d, reports %d", r.mgr.Active(), r.note.endedCount())
	}
}

// TestAttachReplaysOutputOfTheConfirmation: output that arrives while the attach waits in the
// confirmation is not part of the snapshot; it follows replay_end.
func TestAttachReplaysOutputOfTheConfirmation(t *testing.T) {
	var r *pipeRig
	r = newPipeRig(t, pipeConfirm(func(_ context.Context, kind, _, _ string, _ time.Duration) bool {
		if kind == confirmAttach {
			if err := r.helper.sendOutput(sessID, frame.Data([]byte("during-confirm"))); err != nil {
				t.Error(err)
			}
		}
		return true
	}))
	dev, dc := r.open()
	r.helper.output(t, sessID, frame.Data([]byte("before")))
	if f := r.next(dc, dev); f.Type != frame.TypeData || string(f.Data) != "before" {
		t.Fatalf("live output %+v", f)
	}
	r.detach(dc)
	dev2, dc2 := r.connect("shell", AttAttach)
	r.send(dc2, dev2, frame.Start(100, 30))
	want := []struct {
		typ  frame.Type
		data string
	}{{frame.TypeReplayBegin, ""}, {frame.TypeData, "before"}, {frame.TypeReplayEnd, ""}, {frame.TypeData, "during-confirm"}}
	for i, w := range want {
		f := r.next(dc2, dev2)
		if f.Type != w.typ || string(f.Data) != w.data {
			t.Fatalf("frame %d: %v %q, want %v %q", i, f.Type, f.Data, w.typ, w.data)
		}
	}
	if rs := recv(t, r.ctx, "pty.resize", r.helper.resizes); rs.Cols != 100 || rs.Rows != 30 {
		t.Errorf("resize of the attach %+v", rs)
	}
}

// TestAttachOfAReportedSession: a session.open for a session whose end was reported closes its
// data connection with 1000 and reports nothing new.
func TestAttachOfAReportedSession(t *testing.T) {
	r := newPipeRig(t)
	_, dc := r.open()
	r.detach(dc)
	r.helper.exited(t, sessID, 0)
	recv(t, r.ctx, "session.ended", r.note.ended)
	r.sessionOpen(20 * time.Second)
	if code := r.closedWith(r.srv.Data(r.ctx, sessID, 2)); code != CodeNormal {
		t.Errorf("close code %d", code)
	}
	if n := r.note.endedCount(); n != 1 {
		t.Errorf("%d reports", n)
	}
}

// TestOpeningWindowAlreadyOver: an opening whose window is over when session.open arrives times
// out at once; the host service closes the data connection itself shortly after the report.
func TestOpeningWindowAlreadyOver(t *testing.T) {
	clk := clocktest.New(testkit.Deadline())
	r := newPipeRig(t, pipeClock(clk))
	r.sessionOpen(0)
	dc := r.srv.Data(r.ctx, sessID, 1)
	dev := r.device()
	if err := r.handshakeOn(dc, dev, "shell", AttOpen); err != nil {
		t.Fatal(err)
	}
	wantEnded(t, recv(t, r.ctx, "session.ended", r.note.ended), ReasonOpenFailed, DetailOpenTimeout)
	until(t, "the close timer", func() bool { return clk.Pending() > 0 })
	clk.Advance(time.Millisecond)
	if code := r.closedWith(dc); code != CodeNormal {
		t.Errorf("close code %d", code)
	}
}
