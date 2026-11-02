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

// TestOwnerAccountDetachedSessionLivesOn (FIX-terminal-session-lifecycle, D-TSL-1 and D-TSL-2):
// in the owner account mode a detached session keeps running past terminal.idle_timeout and the
// old terminal.detach_grace, because idle pauses while no device is attached and the hello of its
// data connection carries terminal.owner_account_detach_grace; another device then reattaches and
// gets the scrollback. After the reattach the idle timeout of the attached session holds again,
// and a detach beyond the owner detach lifetime ends the session with detach_expired.
func TestOwnerAccountDetachedSessionLivesOn(t *testing.T) {
	clk := clocktest.New(time.Now())
	v := sessiontest.DefaultValues
	v.IdleTimeoutMs, v.DetachGraceMs = (30 * time.Minute).Milliseconds(), (24 * time.Hour).Milliseconds()
	r := newRig(t, withClock(clk), withValues(v), withOwnerAccount())
	dev, dc := r.open(sessA, "shell")
	r.note.wait(t, "started", func() bool { return r.note.startedCount(sessA) == 1 })
	r.send(dc, dev, frame.Data([]byte("echo kept-for-later\n")))
	r.readUntil(dc, dev, "kept-for-later\r\n")
	detached := func() bool {
		s, ok := r.mgr.liveOf(sessA)
		if !ok {
			return false
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.dc == nil
	}
	// The owner detaches on purpose and comes back two hours later: past the idle timeout and the
	// old detach grace of 15 minutes.
	dc.Close(CodeNormal, "")
	waitFor(t, "detached", detached)
	advance(t, clk, 2*time.Hour)
	if r.mgr.Active() != 1 || r.note.endedCount() != 0 {
		t.Fatal("the detached owner session ended")
	}
	// Another device of the owner attaches and gets the scrollback.
	r.sessionOpen(sessA, "shell", 20*time.Second)
	dc2 := r.srv.Data(r.ctx, sessA, 2)
	dev2 := r.device(sessA)
	c := r.claims(sessA, "shell", AttAttach, dev2)
	c["dev"] = "5f0c6a52-0000-7000-8000-0000000000d2"
	if err := r.handshake(dc2, dev2, r.signer.Sign(c)); err != nil {
		t.Fatal(err)
	}
	r.send(dc2, dev2, frame.Start(100, 30))
	frames := r.readUntil(dc2, dev2, "kept-for-later")
	if frames[0].Type != frame.TypeReplayBegin {
		t.Fatalf("first frame type %d", frames[0].Type)
	}
	waitFor(t, "attached", func() bool { return !detached() })
	// Idle counts from the reattach: 29 minutes later the session lives, at 30 it ends.
	advance(t, clk, 29*time.Minute)
	if r.mgr.Active() != 1 {
		t.Fatal("the reattached owner session ended before its idle timeout")
	}
	advance(t, clk, time.Minute)
	if e := r.note.waitEnded(t, sessA); e.CloseReason != ReasonIdleTimeout {
		t.Errorf("ended %+v", e)
	}
}

// TestOwnerAccountDetachExpires: a detached owner account session ends with detach_expired at the
// detach value of its hello, not earlier through idle.
func TestOwnerAccountDetachExpires(t *testing.T) {
	clk := clocktest.New(time.Now())
	v := sessiontest.DefaultValues
	v.IdleTimeoutMs, v.DetachGraceMs = (30 * time.Minute).Milliseconds(), (24 * time.Hour).Milliseconds()
	r := newRig(t, withClock(clk), withValues(v), withOwnerAccount())
	_, dc := r.open(sessA, "shell")
	r.note.wait(t, "started", func() bool { return r.note.startedCount(sessA) == 1 })
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
	advance(t, clk, 24*time.Hour-time.Minute)
	if r.mgr.Active() != 1 {
		t.Fatal("ended before the detach value")
	}
	advance(t, clk, time.Minute)
	if e := r.note.waitEnded(t, sessA); e.CloseReason != ReasonDetachExpired {
		t.Errorf("ended %+v", e)
	}
}
