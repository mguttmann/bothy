// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"bytes"
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
)

const (
	sidA = "0192f3a4-5b6c-7d8e-9f01-23456789ab01"
	sidB = "0192f3a4-5b6c-7d8e-9f01-23456789ab02"
)

// TestSessionActions: open, a screenshot with its image chunks, a click mapped back with the
// Retina factor and an automatic image, a key and a text passed unchanged, refused actions that
// inject nothing, and the end by done with release, hide, cu_state stopped and one computer.ended.
func TestSessionActions(t *testing.T) {
	r := newRig(t)
	fh := r.connect(hello())
	r.waitState(func(s HelperState) bool { return s.Connected })
	d, st := r.open(sidA)
	if st.State != frame.StateActive {
		t.Fatalf("first cu_state %+v", st)
	}
	fh.waitOp(OpShow)
	if s := fh.opsOf(OpShow)[0].(*Show); s.AgentName != "Builder" || s.SessionID != sidA {
		t.Errorf("show %+v", s)
	}
	r.n.mu.Lock()
	started := r.n.started
	r.n.mu.Unlock()
	if len(started) != 1 || started[0].State != "active" {
		t.Errorf("computer.started %+v", started)
	}

	res, img, begin := d.action(actScreenshot, `{}`)
	if res.Code != CodeOK || !res.Image || res.ScaleID < 1 || res.Width != 1280 || res.Height != 831 || res.App.ID != "com.example.editor" {
		t.Fatalf("screenshot result %+v", res)
	}
	if !bytes.Equal(img, fh.img) || begin.Format != frame.FormatJPEG || begin.ScaleID != res.ScaleID {
		t.Errorf("image differs or begin %+v", begin)
	}
	if c := fh.opsOf(OpCapture)[0].(*Capture); c.MaxEdge != 1280 || c.Quality != 80 || c.Format != "jpeg" || c.Region != nil {
		t.Errorf("capture %+v", c)
	}

	res, img, _ = d.action(actLeftClick, `{"coordinate":[640,400],"scale_id":`+itoa(res.ScaleID)+`,"modifiers":["cmd"]}`)
	if res.Code != CodeOK || len(img) == 0 {
		t.Errorf("click result %+v with %d image bytes", res, len(img))
	}
	do := fh.opsOf(OpDo)[0].(*Do)
	if do.Action != actLeftClick || *do.X != 756 || *do.Y != 472.5 || do.Modifiers[0] != "cmd" {
		t.Errorf("do %+v at (%v, %v)", do, *do.X, *do.Y)
	}

	// Every key passes unchanged (CU-T15).
	for _, k := range []string{"cmd+q", "ctrl+alt+Delete", "cmd+space", "Escape"} {
		if res, _, _ := d.action(actKey, `{"keys":"`+k+`"}`); res.Code != CodeOK {
			t.Errorf("key %s: %+v", k, res)
		}
	}
	keys := fh.opsOf(OpDo)
	if got := keys[len(keys)-1].(*Do).Keys; got != "Escape" {
		t.Errorf("last key %q", got)
	}
	if res, _, _ := d.action(actType, `{"text":"naïve café, 世界"}`); res.Code != CodeOK {
		t.Errorf("type: %+v", res)
	}

	before := len(fh.opsOf(OpDo))
	for _, c := range []struct{ name, args string }{
		{actLeftClick, `{"coordinate":[1280,0],"scale_id":1}`},
		{actLeftClick, `{"coordinate":[1,1],"scale_id":99}`},
		{actLeftClick, `{"coordinate":[1,1]}`},
		{actKey, `{"keys":"a","repeat":51}`},
		{actScroll, `{"coordinate":[1,1],"scale_id":1,"direction":"down","amount":51}`},
		{"drag", `{}`},
		{actType, `{"text":"x","extra":1}`},
		{actWait, `{"duration_ms":60001}`},
	} {
		if res, _, _ := d.action(c.name, c.args); res.Code != CodeInvalidAction || res.Image {
			t.Errorf("%s %s: %+v", c.name, c.args, res)
		}
	}
	if len(fh.opsOf(OpDo)) != before {
		t.Error("a refused action injected input")
	}

	// cursor_position is answered from the pointer in pixels of the screenshot.
	if res, _, _ := d.action(actCursorPosition, `{}`); res.Code != CodeOK || res.Cursor == nil || res.Cursor.X != 640 || res.Cursor.Y != 400 {
		t.Errorf("cursor_position %+v", res)
	}

	// A zoom keeps the scale ID of its screenshot and comes as PNG.
	res, _, begin = d.action(actZoom, `{"region":[0,0,640,400],"scale_id":1}`)
	if res.Code != CodeOK || res.ScaleID != 1 || begin.Format != frame.FormatPNG {
		t.Errorf("zoom %+v %+v", res, begin)
	}
	zooms := fh.opsOf(OpCapture)
	if z := zooms[len(zooms)-1].(*Capture); z.Region == nil || z.Region.Width != 756 || z.Format != "png" {
		t.Errorf("zoom capture %+v", z)
	}

	// A changed geometry makes the old scale ID stale, nothing is injected.
	fh.mu.Lock()
	fh.scr = Screen{Width: 1920, Height: 1080, Scale: 1}
	fh.mu.Unlock()
	if res, _, _ := d.action(actScreenshot, `{}`); res.ScaleID == 1 {
		t.Errorf("new geometry kept scale ID 1")
	}
	before = len(fh.opsOf(OpDo))
	if res, _, _ := d.action(actLeftClick, `{"coordinate":[1,1],"scale_id":1}`); res.Code != CodeStaleScreenshot {
		t.Errorf("stale click %+v", res)
	}
	if len(fh.opsOf(OpDo)) != before {
		t.Error("a stale click injected input")
	}

	d.sendFrame(frame.Close())
	if st := d.state(); st.State != frame.StateStopped || st.Reason != ReasonDone {
		t.Errorf("last cu_state %+v", st)
	}
	e := r.n.waitEnded(t, sidA)
	if e.CloseReason != ReasonDone || e.CloseDetail != "" || e.ActionCounts[actKey] != 5 || e.ActionCounts[actLeftClick] != 5 ||
		len(e.Apps) != 1 || e.Apps[0].Signer != "EXAMPLE123" {
		t.Errorf("computer.ended %+v", e)
	}
	fh.waitOp(OpHide)
	if fh.pressedAfterRelease() {
		t.Error("input after the last release")
	}
	if code, _, ok := r.stream(sidA).closed(); !ok || code != 1000 {
		t.Errorf("data connection close %d %v", code, ok)
	}
	if log := r.logText(); strings.Contains(log, "naïve") || strings.Contains(log, "cmd+q") || strings.Contains(log, "Editor") {
		t.Error("the log names typed text, a key or an app")
	}
	time.Sleep(20 * time.Millisecond)
	if len(r.n.endedOf(sidA)) != 1 {
		t.Errorf("computer.ended %d times", len(r.n.endedOf(sidA)))
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// pipePair returns both ends of an in-memory connection.
func pipePair() (net.Conn, net.Conn) { return net.Pipe() }

// TestOpenChecks: the checks of 27 5.3 steps 9 to 11 at computer.open, with the close detail in
// computer.ended open_failed and nothing shown (CU-T07).
func TestOpenChecks(t *testing.T) {
	t.Run("helper absent", func(t *testing.T) {
		r := newRig(t)
		r.m.Open(context.Background(), openOp(sidA), r.n)
		if e := r.n.waitEnded(t, sidA); e.CloseReason != ReasonOpenFailed || e.CloseDetail != DetailHelperAbsent {
			t.Errorf("%+v", e)
		}
	})
	t.Run("helper untrusted", func(t *testing.T) {
		r := newRig(t)
		r.mu.Lock()
		r.peerUID = uidRobin
		r.mu.Unlock()
		a, b := pipePair()
		go func() { r.l.ch <- a }()
		defer func() { _ = b.Close() }()
		r.waitState(func(s HelperState) bool { return s.Refused })
		r.m.Open(context.Background(), openOp(sidA), r.n)
		if e := r.n.waitEnded(t, sidA); e.CloseDetail != DetailHelperUntrusted {
			t.Errorf("%+v", e)
		}
		if !strings.Contains(r.logText(), LogPeerRefused) {
			t.Error("no computer_peer_refused in the log (CU-T23)")
		}
		if rep := r.m.Report(); rep.Helper != control.HelperRefused {
			t.Errorf("report %+v", rep)
		}
	})
	t.Run("not enabled", func(t *testing.T) {
		r := newRig(t)
		h := hello()
		h.Enabled = false
		fh := r.connect(h)
		r.waitState(func(s HelperState) bool { return s.Connected })
		r.m.Open(context.Background(), openOp(sidA), r.n)
		if e := r.n.waitEnded(t, sidA); e.CloseDetail != DetailNotEnabled {
			t.Errorf("%+v", e)
		}
		if len(fh.opsOf(OpShow)) != 0 {
			t.Error("indicator shown for a refused open")
		}
	})
	t.Run("busy", func(t *testing.T) {
		r := newRig(t)
		fh := r.connect(hello())
		r.waitState(func(s HelperState) bool { return s.Connected })
		d, _ := r.open(sidA)
		r.m.Open(context.Background(), openOp(sidB), r.n)
		if e := r.n.waitEnded(t, sidB); e.CloseReason != ReasonOpenFailed || e.CloseDetail != DetailBusy {
			t.Errorf("%+v", e)
		}
		if len(fh.opsOf(OpShow)) != 1 {
			t.Error("the busy open showed the indicator")
		}
		// A terminal session is no conflict and the first session still works.
		if res, _, _ := d.action(actScreenshot, `{}`); res.Code != CodeOK {
			t.Errorf("first session after the busy open: %+v", res)
		}
	})
}

// TestMessage1Checks: the steps of 27 5.3 on message 1 through the whole path, each with its close
// code and detail; the indicator shown at the open is hidden and nothing is pressed (CU-T07).
func TestMessage1Checks(t *testing.T) {
	cases := []struct {
		name   string
		edit   func(map[string]any)
		before func(*rig, *fakeHelper)
		detail string
		code   int
	}{
		{name: "aud", edit: func(c map[string]any) { c["aud"] = sidB }, detail: DetailTokenInvalid, code: 4104},
		{name: "expired", edit: func(c map[string]any) { c["exp"] = time.Now().Add(-time.Minute).Unix() }, detail: DetailTokenExpired, code: 4101},
		{name: "other run", edit: func(c map[string]any) { c["run"] = sidB }, detail: DetailTokenInvalid, code: 4104},
		{name: "own false", edit: func(c map[string]any) { c["own"] = false }, detail: DetailTokenInvalid, code: 4104},
		{name: "generation changed", edit: func(c map[string]any) { c["gen"] = 2 }, detail: DetailNotEnabled, code: 4101},
		{name: "helper switched off", before: func(r *rig, fh *fakeHelper) {
			off := false
			fh.send(StateMsg{Op: OpState, Enabled: &off})
			r.waitState(func(s HelperState) bool { return !s.Enabled })
		}, detail: "", code: 1000},
		{name: "helper gone", before: func(r *rig, fh *fakeHelper) {
			_ = fh.c.Close()
			r.waitState(func(s HelperState) bool { return !s.Connected })
		}, detail: "", code: 1000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t)
			fh := r.connect(hello())
			r.waitState(func(s HelperState) bool { return s.Connected })
			r.m.Open(context.Background(), openOp(sidA), r.n)
			fh.waitOp(OpShow)
			if c.before != nil {
				c.before(r, fh)
			}
			if c.detail == "" {
				// The session ended before message 1 (switch-off or crash during the opening).
				e := r.n.waitEnded(t, sidA)
				if e.CloseReason != ReasonDeviceDisabled && e.CloseDetail != DetailHelperCrashed {
					t.Errorf("%+v", e)
				}
				return
			}
			if _, ok := r.handshake(sidA, c.edit); ok {
				t.Fatal("message 2 despite the failed check")
			}
			code, reason, _ := r.stream(sidA).closed()
			if code != c.code || reason != c.detail {
				t.Errorf("close %d %q, want %d %q", code, reason, c.code, c.detail)
			}
			if e := r.n.waitEnded(t, sidA); e.CloseReason != ReasonOpenFailed || e.CloseDetail != c.detail {
				t.Errorf("%+v", e)
			}
			fh.waitOp(OpHide)
			if len(fh.opsOf(OpDo)) != 0 {
				t.Error("input injected")
			}
		})
	}
}

// TestHandshakeFailed: a message 1 that is no Noise message of the computer prologue closes with
// 4104 handshake_failed and reports nothing (27 5.3 step 0).
func TestHandshakeFailed(t *testing.T) {
	r := newRig(t)
	fh := r.connect(hello())
	r.waitState(func(s HelperState) bool { return s.Connected })
	r.m.Open(context.Background(), openOp(sidA), r.n)
	r.stream(sidA).in <- []byte("not a noise message, long enough to look like one ......")
	deadline := testkit.Deadline()
	for time.Now().Before(deadline) {
		if _, _, ok := r.stream(sidA).closed(); ok {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if code, reason, _ := r.stream(sidA).closed(); code != 4104 || reason != DetailHandshakeFailed {
		t.Errorf("close %d %q", code, reason)
	}
	fh.waitOp(OpHide)
	time.Sleep(20 * time.Millisecond)
	if len(r.n.endedOf(sidA)) != 0 {
		t.Error("computer.ended after a failure of step 0")
	}
}

// TestReplayedToken: the token of an ended session in a new opening of the same session ID is
// token_replayed (27 5.3 step 5).
func TestReplayedToken(t *testing.T) {
	r := newRig(t)
	_ = r.connect(hello())
	r.waitState(func(s HelperState) bool { return s.Connected })
	var jti string
	r.m.Open(context.Background(), openOp(sidA), r.n)
	d, ok := r.handshake(sidA, func(c map[string]any) { jti = c["jti"].(string) })
	if !ok {
		t.Fatal("first open refused")
	}
	d.state()
	d.sendFrame(frame.Close())
	r.n.waitEnded(t, sidA)
	r.mu.Lock()
	delete(r.streams, sidA)
	r.mu.Unlock()
	r.m.Open(context.Background(), openOp(sidA), r.n)
	if _, ok := r.handshake(sidA, func(c map[string]any) { c["jti"] = jti }); ok {
		t.Fatal("replayed token accepted")
	}
	if code, reason, _ := r.stream(sidA).closed(); code != 4101 || reason != DetailTokenReplayed {
		t.Errorf("close %d %q", code, reason)
	}
}

// TestPauseResume: a lock pauses with locked, releases every key, answers actions with
// screen_locked without injecting; the unlock resumes (CU-T13).
func TestPauseResume(t *testing.T) {
	r := newRig(t)
	fh := r.connect(hello())
	r.waitState(func(s HelperState) bool { return s.Connected })
	d, _ := r.open(sidA)
	releases := len(fh.opsOf(OpRelease))
	on, off := true, false
	fh.send(StateMsg{Op: OpState, Locked: &on})
	if st := d.state(); st.State != frame.StatePaused || st.Reason != "locked" {
		t.Fatalf("cu_state %+v", st)
	}
	if len(fh.opsOf(OpRelease)) <= releases {
		fh.waitOp(OpRelease)
	}
	before := len(fh.opsOf(OpDo)) + len(fh.opsOf(OpCapture))
	for range 3 {
		if res, _, _ := d.action(actKey, `{"keys":"a"}`); res.Code != CodeScreenLocked || res.Image {
			t.Errorf("action while locked: %+v", res)
		}
	}
	if len(fh.opsOf(OpDo))+len(fh.opsOf(OpCapture)) != before {
		t.Error("the helper was asked while locked")
	}
	time.Sleep(10 * time.Millisecond)
	fh.send(StateMsg{Op: OpState, Locked: &off})
	if st := d.state(); st.State != frame.StateActive {
		t.Fatalf("cu_state after unlock %+v", st)
	}
	if res, _, _ := d.action(actKey, `{"keys":"a"}`); res.Code != CodeOK {
		t.Errorf("action after unlock: %+v", res)
	}
	r.n.mu.Lock()
	var states []string
	for _, s := range r.n.states {
		states = append(states, s.State+"/"+s.Reason)
	}
	r.n.mu.Unlock()
	if len(states) < 2 || states[0] != "paused/locked" || states[1] != "active/" {
		t.Errorf("computer.state %v", states)
	}
	d.sendFrame(frame.Close())
	if e := r.n.waitEnded(t, sidA); e.PausedMs < 10 {
		t.Errorf("paused_ms %d", e.PausedMs)
	}
}

// TestOpenWhileLocked: a locked screen opens the session paused (27 5.1 step 7).
func TestOpenWhileLocked(t *testing.T) {
	r := newRig(t)
	h := hello()
	h.Locked = true
	_ = r.connect(h)
	r.waitState(func(s HelperState) bool { return s.Connected })
	_, st := r.open(sidA)
	if st.State != frame.StatePaused || st.Reason != "locked" {
		t.Errorf("first cu_state %+v", st)
	}
	r.n.mu.Lock()
	defer r.n.mu.Unlock()
	if len(r.n.started) != 1 || r.n.started[0].State != "paused" {
		t.Errorf("computer.started %+v", r.n.started)
	}
}

// TestEndPaths: every end path the host sees releases every key, hides the indicator, sends
// cu_state stopped and exactly one computer.ended with the reason of 27 8.3; computer.close sends
// none, because the server ended the session (CU-T16 host part).
func TestEndPaths(t *testing.T) {
	cases := []struct {
		name           string
		do             func(r *rig, fh *fakeHelper, d *device)
		reason, detail string
		state          bool // a cu_state stopped reaches the device
		release        bool // release and hide reach the helper
		reported       bool
	}{
		{"done", func(_ *rig, _ *fakeHelper, d *device) { d.sendFrame(frame.Close()) }, ReasonDone, "", true, true, true},
		{"Esc", func(_ *rig, fh *fakeHelper, _ *device) {
			fh.send(Stop{Op: OpStop, SessionID: sidA, Detail: DetailLocalEscape})
		}, ReasonStoppedByUser, DetailLocalEscape, true, true, true},
		{"indicator", func(_ *rig, fh *fakeHelper, _ *device) {
			fh.send(Stop{Op: OpStop, SessionID: sidA, Detail: DetailIndicatorStop})
		}, ReasonStoppedByUser, DetailIndicatorStop, true, true, true},
		{"computer.close", func(r *rig, _ *fakeHelper, _ *device) {
			r.m.Close(context.Background(), control.ComputerClose{SessionID: sidA, Reason: "run_ended"})
		}, "run_ended", "", true, true, false},
		{"computer.disable", func(r *rig, _ *fakeHelper, _ *device) {
			r.m.Disable(context.Background(), control.ComputerDisable{Generation: 3})
		}, ReasonDeviceDisabled, "", true, true, true},
		{"local switch-off", func(_ *rig, fh *fakeHelper, _ *device) {
			off := false
			fh.send(StateMsg{Op: OpState, Enabled: &off})
		}, ReasonDeviceDisabled, "", true, true, true},
		{"lost connection", func(_ *rig, _ *fakeHelper, d *device) { d.s.dropByServer() }, ReasonConnectionLost, "", true, true, true},
		// A failed decryption breaks the transport (D-M2-38-8): no frame can follow it.
		{"stream fault", func(_ *rig, _ *fakeHelper, d *device) { d.s.in <- []byte("garbage that does not decrypt") }, ReasonError, DetailStreamFault, false, true, true},
		{"terminal frame", func(_ *rig, _ *fakeHelper, d *device) { d.sendFrame(frame.Resize(80, 24)) }, ReasonError, DetailStreamFault, true, true, true},
		{"action out of order", func(_ *rig, _ *fakeHelper, d *device) {
			f, _ := frame.NewAction(frame.Action{Seq: 5, Action: actScreenshot, Args: []byte(`{}`)})
			d.sendFrame(f)
		}, ReasonError, DetailStreamFault, true, true, true},
		{"helper crash", func(_ *rig, fh *fakeHelper, _ *device) { _ = fh.c.Close() }, ReasonError, DetailHelperCrashed, true, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t)
			fh := r.connect(hello())
			r.waitState(func(s HelperState) bool { return s.Connected })
			d, _ := r.open(sidA)
			if res, _, _ := d.action(actLeftClick, `{"coordinate":[1,1],"scale_id":0}`); res.Code != CodeInvalidAction {
				t.Fatalf("click without a screenshot %+v", res)
			}
			c.do(r, fh, d)
			if c.state {
				if st := d.state(); st.State != frame.StateStopped || st.Reason != c.reason {
					t.Errorf("cu_state %+v", st)
				}
			}
			if c.release {
				fh.waitOp(OpHide)
				if len(fh.opsOf(OpRelease)) == 0 || fh.pressedAfterRelease() {
					t.Error("not every key released")
				}
			}
			if c.reported {
				e := r.n.waitEnded(t, sidA)
				if e.CloseReason != c.reason || e.CloseDetail != c.detail || e.ActionCounts[actLeftClick] != 1 {
					t.Errorf("computer.ended %+v", e)
				}
			}
			time.Sleep(30 * time.Millisecond)
			want := 0
			if c.reported {
				want = 1
			}
			if n := len(r.n.endedOf(sidA)); n != want {
				t.Errorf("computer.ended %d times, want %d", n, want)
			}
			if len(r.m.Live()) != 0 {
				t.Errorf("live after the end: %v", r.m.Live())
			}
			// The controller lock is free again.
			if c.name != "helper crash" && c.name != "computer.disable" && c.name != "local switch-off" {
				_, st := r.open(sidB)
				if st.State != frame.StateActive {
					t.Errorf("second session %+v", st)
				}
			}
		})
	}
}

// TestEndedPendingWhileOffline: a computer.ended the control connection could not take is kept,
// named in Live, and sent with the next Resend.
func TestEndedPendingWhileOffline(t *testing.T) {
	r := newRig(t)
	_ = r.connect(hello())
	r.waitState(func(s HelperState) bool { return s.Connected })
	d, _ := r.open(sidA)
	r.n.mu.Lock()
	r.n.fail = true
	r.n.mu.Unlock()
	d.sendFrame(frame.Close())
	deadline := testkit.Deadline()
	for time.Now().Before(deadline) && len(r.m.Live()) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if live := r.m.Live(); len(live) != 1 || live[0] != sidA {
		t.Fatalf("live %v", live)
	}
	r.n.mu.Lock()
	r.n.fail = false
	r.n.mu.Unlock()
	r.m.Resend(context.Background(), r.n)
	if e := r.n.waitEnded(t, sidA); e.CloseReason != ReasonDone {
		t.Errorf("%+v", e)
	}
	if len(r.m.Live()) != 0 {
		t.Error("still live after the resend")
	}
}

// TestReportObject: the object computer of the report follows the helper (27 12.3).
func TestReportObject(t *testing.T) {
	r := newRig(t)
	if rep := r.m.Report(); rep.Helper != control.HelperAbsent || rep.Permissions.Capture != "unknown" || rep.Signature != "development" {
		t.Errorf("absent %+v", rep)
	}
	h := hello()
	h.Permissions.Input = "denied"
	fh := r.connect(h)
	r.waitState(func(s HelperState) bool { return s.Connected })
	rep := r.m.Report()
	want := control.ComputerReport{Helper: "connected", Enabled: true, Generation: 3, OSAccount: "alex", HelperVersion: "1.0.0", Signature: "development",
		Permissions: control.ComputerPermissions{Capture: "granted", Input: "denied"}}
	if rep != want {
		t.Errorf("connected %+v", rep)
	}
	on := true
	fh.send(StateMsg{Op: OpState, Locked: &on})
	r.waitState(func(s HelperState) bool { return s.Locked })
	if !r.m.Report().Locked {
		t.Error("locked not reported")
	}
	_ = fh.c.Close()
	r.waitState(func(s HelperState) bool { return !s.Connected })
	if r.m.Report().Helper != control.HelperAbsent {
		t.Error("a gone helper is not absent")
	}
}

// TestDisableReachesHelper: computer.disable goes to the helper as disable with the generation.
func TestDisableReachesHelper(t *testing.T) {
	r := newRig(t)
	fh := r.connect(hello())
	r.waitState(func(s HelperState) bool { return s.Connected })
	r.m.Disable(context.Background(), control.ComputerDisable{Generation: 3})
	fh.waitOp(OpDisable)
	if d := fh.opsOf(OpDisable)[0].(*Disable); d.Generation != 3 {
		t.Errorf("disable %+v", d)
	}
}
