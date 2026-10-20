// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package session

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/clock/clocktest"
	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/session/sessiontest"
)

// advance waits until the session watch has armed its timer, then moves the fake clock.
func advance(t *testing.T, clk *clocktest.Fake, d time.Duration) {
	t.Helper()
	waitFor(t, "a pending timer", func() bool { return clk.Pending() > 0 })
	clk.Advance(d)
}

func TestIdleTimeout(t *testing.T) {
	clk := clocktest.New(time.Now())
	v := sessiontest.DefaultValues
	v.IdleTimeoutMs, v.DetachGraceMs = (10 * time.Minute).Milliseconds(), (60 * time.Minute).Milliseconds()
	r := newRig(t, withClock(clk), withValues(v))
	dev, dc := r.open(sessA, "shell")
	r.note.wait(t, "started", func() bool { return r.note.startedCount(sessA) == 1 })
	advance(t, clk, 9*time.Minute)
	// Only data and resize reset the timer; output of the host does not.
	r.send(dc, dev, frame.Resize(100, 30))
	r.send(dc, dev, frame.Data([]byte("echo idle-probe\n")))
	r.readUntil(dc, dev, "idle-probe\r\n")
	advance(t, clk, 9*time.Minute)
	if r.mgr.Active() != 1 {
		t.Fatal("the input reset the idle timer")
	}
	// Detached time counts.
	dc.Close(CodeNormal, "")
	waitFor(t, "detached", func() bool {
		s, ok := r.mgr.liveOf(sessA)
		if !ok {
			return false
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.dc == nil
	})
	advance(t, clk, 2*time.Minute)
	if e := r.note.waitEnded(t, sessA); e.CloseReason != ReasonIdleTimeout {
		t.Errorf("ended %+v", e)
	}
}

func TestDetachGrace(t *testing.T) {
	clk := clocktest.New(time.Now())
	v := sessiontest.DefaultValues
	v.IdleTimeoutMs, v.DetachGraceMs = (60 * time.Minute).Milliseconds(), (10 * time.Minute).Milliseconds()
	r := newRig(t, withClock(clk), withValues(v))
	_, dc := r.open(sessA, "shell")
	r.note.wait(t, "started", func() bool { return r.note.startedCount(sessA) == 1 })
	detached := func() bool {
		s, ok := r.mgr.liveOf(sessA)
		if !ok {
			return false
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.dc == nil
	}
	dc.Close(CodeNormal, "")
	waitFor(t, "detached", detached)
	advance(t, clk, 9*time.Minute)
	// An attach in time stops the detach timer.
	r.sessionOpen(sessA, "shell", 20*time.Second)
	dc2 := r.srv.Data(r.ctx, sessA, 2)
	dev2 := r.device(sessA)
	if err := r.handshake(dc2, dev2, r.signer.Sign(r.claims(sessA, "shell", AttAttach, dev2))); err != nil {
		t.Fatal(err)
	}
	r.send(dc2, dev2, frame.Start(80, 24))
	waitFor(t, "attached", func() bool { return !detached() })
	advance(t, clk, 9*time.Minute)
	if r.mgr.Active() != 1 {
		t.Fatal("the attach did not stop the detach timer")
	}
	dc2.Close(CodeNormal, "")
	waitFor(t, "detached again", detached)
	advance(t, clk, 10*time.Minute)
	if e := r.note.waitEnded(t, sessA); e.CloseReason != ReasonDetachExpired {
		t.Errorf("ended %+v", e)
	}
}
