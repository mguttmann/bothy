// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/profiles"
	"github.com/OWNER/bothy/services/host/internal/session/sessiontest"
	"github.com/OWNER/bothy/services/host/internal/state"
)

const (
	sessA = "0192f4a0-7b1c-7b22-9d4f-00000000b001"
	sessB = "0192f4a0-7b1c-7b22-9d4f-00000000b002"
)

func TestOpenShellHappyPath(t *testing.T) {
	r := newRig(t)
	dev, dc := r.open(sessA, "shell")
	if dc.Identify["session_id"] != sessA || dc.Identify["host_id"] != hostID || dc.Identify["proof"] == "" {
		t.Errorf("identify %v", dc.Identify["session_id"])
	}
	var p2 payload2
	if err := json.Unmarshal(dev.Payload2, &p2); err != nil || p2.V != 1 || p2.HostVersion != "1.2.3" || p2.Profile != "shell" {
		t.Errorf("payload of message 2: %+v", p2)
	}
	r.note.wait(t, "session.started", func() bool { return r.note.startedCount(sessA) == 1 })
	r.send(dc, dev, frame.Data([]byte("stty size; echo o''k\n")))
	r.readUntil(dc, dev, "40 120")
	r.readUntil(dc, dev, "ok")
	r.send(dc, dev, frame.Resize(90, 30))
	r.send(dc, dev, frame.Data([]byte("stty size; echo $$\n")))
	r.readUntil(dc, dev, "30 90")
	r.send(dc, dev, frame.Close())
	e := r.note.waitEnded(t, sessA)
	if e.CloseReason != ReasonClientClosed || r.note.startedCount(sessA) != 1 {
		t.Errorf("ended %+v, started %d", e, r.note.startedCount(sessA))
	}
	if r.mgr.Active() != 0 || len(r.mgr.Live()) != 0 {
		t.Errorf("active %d live %v", r.mgr.Active(), r.mgr.Live())
	}
	if r.startCount() != 1 {
		t.Errorf("PTYs started: %d", r.startCount())
	}
}

func TestOpenCommandProfile(t *testing.T) {
	r := newRig(t)
	dev, dc := r.open(sessA, "echo")
	r.readUntil(dc, dev, "command-ran")
	var exit *frame.Frame
	for exit == nil {
		m, err := dc.NextBinary(r.ctx)
		if err != nil {
			t.Fatal(err)
		}
		f, err := dev.Open(m)
		if err != nil {
			t.Fatal(err)
		}
		if f.Type == frame.TypeExit {
			exit = &f
		}
	}
	e := r.note.waitEnded(t, sessA)
	if e.CloseReason != ReasonProcessExited || e.ExitCode == nil || *e.ExitCode != 0 || exit.ExitCode != 0 {
		t.Errorf("ended %+v exit %d", e, exit.ExitCode)
	}
}

func TestTMT03Replay(t *testing.T) {
	r := newRig(t)
	r.sessionOpen(sessA, "shell", 20*time.Second)
	dc := r.srv.Data(r.ctx, sessA, 1)
	dev := r.device(sessA)
	tok := r.signer.Sign(r.claims(sessA, "shell", AttOpen, dev))
	m1, _ := dev.Message1(tok)
	if err := dc.SendBinary(r.ctx, m1); err != nil {
		t.Fatal(err)
	}
	m2, _ := dc.NextBinary(r.ctx)
	if err := dev.ReadMessage2(m2); err != nil {
		t.Fatal(err)
	}
	r.send(dc, dev, frame.Start(80, 24))
	r.note.wait(t, "session.started", func() bool { return r.note.startedCount(sessA) == 1 })
	// A second session.open for the same session and the recorded message 1 on the new connection.
	r.sessionOpen(sessA, "shell", 20*time.Second)
	dc2 := r.srv.Data(r.ctx, sessA, 2)
	if err := dc2.SendBinary(r.ctx, m1); err != nil {
		t.Fatal(err)
	}
	if code, reason := r.closedWith(dc2); code != CodeTokenStale || reason != DetailTokenReplayed {
		t.Errorf("replay closed with %d %q", code, reason)
	}
	if r.mgr.Active() != 1 || r.startCount() != 1 {
		t.Errorf("active %d, PTYs %d", r.mgr.Active(), r.startCount())
	}
	// The recorded message 1 for another session ID fails the handshake (the prologue binds it).
	r.sessionOpen(sessB, "shell", 20*time.Second)
	dc3 := r.srv.Data(r.ctx, sessB, 1)
	_ = dc3.SendBinary(r.ctx, m1)
	if code, reason := r.closedWith(dc3); code != CodeHandshakeFail || reason != DetailHandshakeFailed {
		t.Errorf("other session closed with %d %q", code, reason)
	}
	// A fresh message 1 for another session that carries the used token: sid is checked first.
	r.sessionOpen(sessB, "shell", 20*time.Second)
	dc4 := r.srv.Data(r.ctx, sessB, 2)
	devB := r.device(sessB)
	m1b, _ := devB.Message1(tok)
	_ = dc4.SendBinary(r.ctx, m1b)
	if code, reason := r.closedWith(dc4); code != CodeHandshakeFail || reason != DetailTokenInvalid {
		t.Errorf("used token for another session closed with %d %q", code, reason)
	}
	if r.note.endedCount() != 0 {
		t.Error("a refusal sends no session.ended")
	}
}

func TestTMT04TokenFailures(t *testing.T) {
	r := newRig(t)
	cases := []struct {
		name   string
		mutate func(c map[string]any)
		header map[string]any
		code   int
		detail string
	}{
		{"aud", func(c map[string]any) { c["aud"] = otherID }, nil, CodeHandshakeFail, DetailTokenInvalid},
		{"sid", func(c map[string]any) { c["sid"] = otherID }, nil, CodeHandshakeFail, DetailTokenInvalid},
		{"cek", func(c map[string]any) { c["cek"] = "b3RoZXI" }, nil, CodeHandshakeFail, DetailTokenInvalid},
		{"foreign kid", nil, map[string]any{"alg": "EdDSA", "kid": "terminal-key-9", "typ": TokenType}, CodeHandshakeFail, DetailTokenInvalid},
		{"typ of the invocation token", nil, map[string]any{"alg": "EdDSA", "kid": "terminal-key-1", "typ": "agent-invocation+jwt"}, CodeHandshakeFail, DetailTokenInvalid},
		{"expired", func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() }, nil, CodeTokenStale, DetailTokenExpired},
	}
	for i, tc := range cases {
		id := sessiontest.NewID()
		r.sessionOpen(id, "shell", 20*time.Second)
		dc := r.srv.Data(r.ctx, id, 1)
		dev := r.device(id)
		c := r.claims(id, "shell", AttOpen, dev)
		if tc.mutate != nil {
			tc.mutate(c)
		}
		tok := r.signer.Sign(c)
		if tc.header != nil {
			tok = r.signer.SignRaw(tc.header, c)
		}
		m1, _ := dev.Message1(tok)
		_ = dc.SendBinary(r.ctx, m1)
		if code, reason := r.closedWith(dc); code != tc.code || reason != tc.detail {
			t.Errorf("%d %s: closed with %d %q", i, tc.name, code, reason)
		}
	}
	if r.startCount() != 0 || r.note.endedCount() != 0 {
		t.Errorf("PTYs %d, ended %d", r.startCount(), r.note.endedCount())
	}
}

func TestTMT05NoStart(t *testing.T) {
	r := newRig(t)
	// Message 1 and 2, then nothing until expires_at.
	r.sessionOpen(sessA, "shell", 1500*time.Millisecond)
	dc := r.srv.Data(r.ctx, sessA, 1)
	dev := r.device(sessA)
	if err := r.handshake(dc, dev, r.signer.Sign(r.claims(sessA, "shell", AttOpen, dev))); err != nil {
		t.Fatal(err)
	}
	e := r.note.waitEnded(t, sessA)
	if e.CloseReason != ReasonOpenFailed || e.CloseDetail != DetailOpenTimeout {
		t.Errorf("no start: %+v", e)
	}
	if !stillOpen(dc, 200*time.Millisecond) {
		t.Error("the host service closed the data connection after session.ended")
	}
	dc.Close(CodeClosed, "")
	// A data frame as the first transport message.
	r.sessionOpen(sessB, "shell", 20*time.Second)
	dc2 := r.srv.Data(r.ctx, sessB, 1)
	dev2 := r.device(sessB)
	if err := r.handshake(dc2, dev2, r.signer.Sign(r.claims(sessB, "shell", AttOpen, dev2))); err != nil {
		t.Fatal(err)
	}
	r.send(dc2, dev2, frame.Data([]byte("x")))
	e = r.note.waitEnded(t, sessB)
	if e.CloseReason != ReasonOpenFailed || e.CloseDetail != DetailHandshakeFailed {
		t.Errorf("data first: %+v", e)
	}
	if !stillOpen(dc2, 200*time.Millisecond) {
		t.Error("the host service closed the data connection after session.ended")
	}
	if r.startCount() != 0 {
		t.Errorf("PTYs %d", r.startCount())
	}
}

func TestSessionEndedLeavesCloseToServer(t *testing.T) {
	r := newRig(t)
	// process_exited: the server closes with 4100, the host reports nothing more.
	dev, dc := r.open(sessA, "shell")
	r.note.wait(t, "started", func() bool { return r.note.startedCount(sessA) == 1 })
	r.send(dc, dev, frame.Data([]byte("exit 3\n")))
	e := r.note.waitEnded(t, sessA)
	if e.CloseReason != ReasonProcessExited || e.ExitCode == nil || *e.ExitCode != 3 {
		t.Errorf("exit: %+v", e)
	}
	if !stillOpen(dc, 200*time.Millisecond) {
		t.Error("closed by the host after process_exited")
	}
	dc.Close(CodeClosed, "")
	time.Sleep(100 * time.Millisecond)
	if r.note.endedCount() != 1 {
		t.Errorf("reports after the close of the server: %d", r.note.endedCount())
	}
	// Without the close of the server the host closes with 1000 after the opening window.
	r.sessionOpen(sessB, "shell", 1200*time.Millisecond)
	dc2 := r.srv.Data(r.ctx, sessB, 1)
	dev2 := r.device(sessB)
	if err := r.handshake(dc2, dev2, r.signer.Sign(r.claims(sessB, "shell", AttOpen, dev2))); err != nil {
		t.Fatal(err)
	}
	r.send(dc2, dev2, frame.Data([]byte("x")))
	r.note.waitEnded(t, sessB)
	if code, _ := r.closedWith(dc2); code != CodeNormal {
		t.Errorf("after the window: %d", code)
	}
}

func TestLocalConfirmAfterOpenWindow(t *testing.T) {
	fc := &fakeConfirm{allow: true, delay: 1500 * time.Millisecond}
	r := newRig(t, withConfirm(fc))
	r.cfg.LocalConfirmTimeout = 3 * time.Second
	// start before expires_at; consent after expires_at but within the confirmation window.
	r.sessionOpen(sessA, "shell", 1*time.Second)
	dc := r.srv.Data(r.ctx, sessA, 1)
	dev := r.device(sessA)
	if err := r.handshake(dc, dev, r.signer.Sign(r.claims(sessA, "shell", AttOpen, dev))); err != nil {
		t.Fatal(err)
	}
	r.send(dc, dev, frame.Start(80, 24))
	r.note.wait(t, "session.started", func() bool { return r.note.startedCount(sessA) == 1 })
	// Without an answer in time: no PTY.
	fc.mu.Lock()
	fc.delay = 10 * time.Second
	fc.mu.Unlock()
	r.cfg.LocalConfirmTimeout = 500 * time.Millisecond
	r.sessionOpen(sessB, "shell", 20*time.Second)
	dc2 := r.srv.Data(r.ctx, sessB, 1)
	dev2 := r.device(sessB)
	if err := r.handshake(dc2, dev2, r.signer.Sign(r.claims(sessB, "shell", AttOpen, dev2))); err != nil {
		t.Fatal(err)
	}
	r.send(dc2, dev2, frame.Start(80, 24))
	e := r.note.waitEnded(t, sessB)
	if e.CloseReason != ReasonOpenFailed || e.CloseDetail != DetailLocalConfirmDenied || r.startCount() != 1 {
		t.Errorf("denied: %+v, PTYs %d", e, r.startCount())
	}
}

func TestConfirmDisabled(t *testing.T) {
	r := newRig(t)
	if r.mgr.d.Confirm != nil {
		t.Fatal("confirmation wired although host_service.require_local_confirm is false")
	}
	r.open(sessA, "shell")
	r.note.wait(t, "started", func() bool { return r.note.startedCount(sessA) == 1 })
}

func TestStreamFaultEndsSession(t *testing.T) {
	r := newRig(t)
	faults := []func(dc *sessiontest.Conn, dev *sessiontest.Device){
		func(dc *sessiontest.Conn, dev *sessiontest.Device) {
			m, _ := dev.Seal(frame.Data([]byte("x")))
			m[len(m)-1] ^= 1
			_ = dc.SendBinary(r.ctx, m)
		},
		func(dc *sessiontest.Conn, dev *sessiontest.Device) {
			m, _ := dev.SealRaw([]byte{0x7f})
			_ = dc.SendBinary(r.ctx, m)
		},
		func(dc *sessiontest.Conn, dev *sessiontest.Device) { r.send(dc, dev, frame.Start(10, 10)) },
	}
	for i, fault := range faults {
		id := sessiontest.NewID()
		dev, dc := r.open(id, "shell")
		r.note.wait(t, "started", func() bool { return r.note.startedCount(id) == 1 })
		r.send(dc, dev, frame.Data([]byte("echo $$\n")))
		fault(dc, dev)
		e := r.note.waitEnded(t, id)
		if e.CloseReason != ReasonOpenFailed || e.CloseDetail != DetailHandshakeFailed {
			t.Errorf("fault %d: %+v", i, e)
		}
		dc.Close(CodeClosed, "")
	}
	waitFor(t, "no active session", func() bool { return r.mgr.Active() == 0 })
}

func TestAttachReplay(t *testing.T) {
	r := newRig(t)
	dev, dc := r.open(sessA, "shell")
	r.note.wait(t, "started", func() bool { return r.note.startedCount(sessA) == 1 })
	r.send(dc, dev, frame.Data([]byte("echo before-detach\n")))
	r.readUntil(dc, dev, "before-detach\r\n")
	dc.Close(CodeNormal, "")
	waitFor(t, "detached", func() bool {
		s, _ := r.mgr.liveOf(sessA)
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.dc == nil
	})
	// An expired attach token: 4101, the PTY stays.
	r.sessionOpen(sessA, "shell", 20*time.Second)
	dcX := r.srv.Data(r.ctx, sessA, 2)
	devX := r.device(sessA)
	c := r.claims(sessA, "shell", AttAttach, devX)
	c["exp"] = time.Now().Add(-time.Hour).Unix()
	m1, _ := devX.Message1(r.signer.Sign(c))
	_ = dcX.SendBinary(r.ctx, m1)
	if code, reason := r.closedWith(dcX); code != CodeTokenStale || reason != DetailTokenExpired || r.mgr.Active() != 1 {
		t.Fatalf("expired attach %d %q active %d", code, reason, r.mgr.Active())
	}
	// A valid attach: replay_begin, the buffered bytes, replay_end, then live output.
	r.sessionOpen(sessA, "shell", 20*time.Second)
	dc2 := r.srv.Data(r.ctx, sessA, 3)
	dev2 := r.device(sessA)
	if err := r.handshake(dc2, dev2, r.signer.Sign(r.claims(sessA, "shell", AttAttach, dev2))); err != nil {
		t.Fatal(err)
	}
	r.send(dc2, dev2, frame.Start(100, 30))
	frames := r.readUntil(dc2, dev2, "before-detach")
	if frames[0].Type != frame.TypeReplayBegin {
		t.Fatalf("first frame type %d", frames[0].Type)
	}
	for {
		m, err := dc2.NextBinary(r.ctx)
		if err != nil {
			t.Fatal(err)
		}
		f, _ := dev2.Open(m)
		if f.Type == frame.TypeReplayEnd {
			break
		}
	}
	r.send(dc2, dev2, frame.Data([]byte("echo after-attach\n")))
	r.readUntil(dc2, dev2, "after-attach\r\n")
	if r.note.startedCount(sessA) != 1 {
		t.Error("an attach sends no session.started")
	}
	// An attach token bound to another key: 4104 token_invalid and the PTY ends.
	r.sessionOpen(sessA, "shell", 20*time.Second)
	dc3 := r.srv.Data(r.ctx, sessA, 4)
	dev3 := r.device(sessA)
	c3 := r.claims(sessA, "shell", AttAttach, dev3)
	c3["cek"] = r.device(sessA).Cek()
	m13, _ := dev3.Message1(r.signer.Sign(c3))
	_ = dc3.SendBinary(r.ctx, m13)
	if code, reason := r.closedWith(dc3); code != CodeHandshakeFail || reason != DetailTokenInvalid {
		t.Errorf("attach with another cek: %d %q", code, reason)
	}
	waitFor(t, "the PTY ended", func() bool { return r.mgr.Active() == 0 })
	if r.note.endedCount() != 0 {
		t.Error("a refusal sends no session.ended")
	}
}

func TestSessionCloseAndDataClose(t *testing.T) {
	r := newRig(t)
	for _, viaClose := range []bool{true, false} {
		id := sessiontest.NewID()
		dev, dc := r.open(id, "shell")
		r.note.wait(t, "started", func() bool { return r.note.startedCount(id) == 1 })
		r.send(dc, dev, frame.Data([]byte("set +H 2>/dev/null; sleep 1000 & echo b''g=$! ; echo ST''ARTED\n")))
		out := r.readUntil(dc, dev, "STARTED")
		var all strings.Builder
		for _, f := range out {
			all.Write(f.Data)
		}
		m := regexp.MustCompile(`bg=(\d+)`).FindStringSubmatch(all.String())
		if m == nil {
			t.Fatal("no PID of the background process")
		}
		pid := atoi(m[1])
		if !pidAlive(pid) {
			t.Fatal("background process missing")
		}
		if viaClose {
			r.mgr.Close(r.ctx, controlClose(id, "access_revoked"))
		} else {
			dc.Close(CodeClosed, "")
		}
		waitFor(t, "background process gone", func() bool { return !pidAlive(pid) })
		waitFor(t, "session gone", func() bool { return r.mgr.Active() == 0 })
		for _, l := range r.mgr.Live() {
			if l == id {
				t.Error("the session is still live")
			}
		}
	}
	if r.note.endedCount() != 0 {
		t.Error("session.close and 4100 send no session.ended")
	}
	// 1000, 4013, 4014 and a dropped connection keep the PTY detached.
	for i, end := range []func(dc *sessiontest.Conn){
		func(dc *sessiontest.Conn) { dc.Close(CodeNormal, "") },
		func(dc *sessiontest.Conn) { dc.Close(4013, "") },
		func(dc *sessiontest.Conn) { dc.Close(4014, "") },
		func(dc *sessiontest.Conn) { dc.Drop() },
	} {
		id := sessiontest.NewID()
		_, dc := r.open(id, "shell")
		r.note.wait(t, "started", func() bool { return r.note.startedCount(id) == 1 })
		end(dc)
		waitFor(t, "detached", func() bool {
			s, ok := r.mgr.liveOf(id)
			if !ok {
				return false
			}
			s.mu.Lock()
			defer s.mu.Unlock()
			return s.dc == nil
		})
		if _, ok := r.mgr.liveOf(id); !ok {
			t.Errorf("end %d: the PTY ended", i)
		}
	}
	// A session.close for an unknown session changes nothing.
	before := r.mgr.Active()
	r.mgr.Close(r.ctx, controlClose(sessiontest.NewID(), "access_revoked"))
	if r.mgr.Active() != before {
		t.Error("a close of an unknown session changed something")
	}
}

func TestHelperUntrustedEndsSessions(t *testing.T) {
	r := newRig(t)
	r.open(sessA, "shell")
	r.note.wait(t, "started", func() bool { return r.note.startedCount(sessA) == 1 })
	// The local channel reports peer_mismatch in operation (HD10): the runs see it too.
	res := shellResult(r.osu)
	for i := range res.Checks {
		if res.Checks[i].Check == doctor.HD10 {
			res.Checks[i].Entries = []doctor.Entry{{Status: doctor.Fail, Code: doctor.CodePeerMismatch, SessionUser: r.osu}}
		}
	}
	r.runner.set(res)
	r.gate.PeerMismatch(r.ctx)
	e := r.note.waitEnded(t, sessA)
	if e.CloseReason != ReasonHostPolicyViolation || e.CloseDetail != DetailHelperUntrusted {
		t.Errorf("running session: %+v", e)
	}
	r.open(sessB, "shell")
	e = r.note.waitEnded(t, sessB)
	if e.CloseReason != ReasonOpenFailed || e.CloseDetail != DetailHelperUntrusted {
		t.Errorf("opening: %+v", e)
	}
	if r.startCount() != 1 {
		t.Errorf("PTYs %d", r.startCount())
	}
}

// TestPeerMismatchOnDial: a session channel whose peer check fails when it is dialled refuses the
// opening with helper_untrusted and records the HD10 event in the gate.
func TestPeerMismatchOnDial(t *testing.T) {
	r := newRig(t)
	r.peerBad.Store(true)
	r.open(sessA, "shell")
	e := r.note.waitEnded(t, sessA)
	if e.CloseReason != ReasonOpenFailed || e.CloseDetail != DetailHelperUntrusted || r.startCount() != 0 {
		t.Errorf("opening: %+v, PTYs %d", e, r.startCount())
	}
}

// TestSessionCloseDuringOpening: a session.close that arrives while the opening waits for the
// local confirmation starts no PTY afterwards.
func TestSessionCloseDuringOpening(t *testing.T) {
	fc := &fakeConfirm{allow: true, delay: 700 * time.Millisecond}
	r := newRig(t, withConfirm(fc))
	r.cfg.LocalConfirmTimeout = 3 * time.Second
	_, dc := r.open(sessA, "shell")
	waitFor(t, "the confirmation asked", func() bool {
		fc.mu.Lock()
		defer fc.mu.Unlock()
		return len(fc.asked) == 1
	})
	r.mgr.Close(r.ctx, controlClose(sessA, "access_revoked"))
	if code, _ := r.closedWith(dc); code != CodeNormal {
		t.Errorf("data connection closed with %d", code)
	}
	time.Sleep(200 * time.Millisecond)
	if r.startCount() != 0 || r.mgr.Active() != 0 || r.note.startedCount(sessA) != 0 {
		t.Errorf("PTYs %d active %d started %d", r.startCount(), r.mgr.Active(), r.note.startedCount(sessA))
	}
}

// fakeClaudeScript stands in for host_service.claude_bin: auth status reports no login; any other
// start prints its working directory, the names of its environment, its CLAUDE_CONFIG_DIR and the
// length of ANTHROPIC_API_KEY (never the key), then waits for input.
const fakeClaudeScript = `#!/bin/sh
if [ "$1" = auth ]; then echo '{"loggedIn":false,"authMethod":"none"}'; exit 1; fi
echo "cwd=$(pwd)"
echo "names=$(env | cut -d= -f1 | sort | tr '\n' ' ')"
echo "config=$CLAUDE_CONFIG_DIR"
echo "keylen=${#ANTHROPIC_API_KEY}"
echo "lc=$LC_CANARY"
echo "DO""NE"
read line
echo "got=$line"
`

// TestClaudeOrganizationAndNoContent starts the profile claude with organization in the working
// directory of wdi and proves that no content, token, key, environment value, path or command line
// reaches a log line, a session.ended or a close reason (TestNoContentInLogsAndReports).
func TestClaudeOrganizationAndNoContent(t *testing.T) {
	r := newRig(t)
	bin := filepath.Join(filepath.Dir(r.home), "bin")
	wd := filepath.Join(r.home, "project")
	for _, d := range []string{bin, wd} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	claude := filepath.Join(bin, "claude")
	if err := os.WriteFile(claude, []byte(fakeClaudeScript), 0o700); err != nil { //nolint:gosec // the fake program must be executable
		t.Fatal(err)
	}
	wdReal, _ := filepath.EvalSymlinks(wd)
	claudeReal, _ := filepath.EvalSymlinks(claude)
	res := shellResult(r.osu)
	res.Profiles = append(res.Profiles, doctor.Profile{Name: "claude", DisplayName: "Claude", Kind: config.KindClaude})
	res.Workdirs[r.osu] = []doctor.Workdir{{DisplayName: "Project", Path: wdReal}}
	res.ClaudeBin, res.SearchList = claudeReal, []string{filepath.Dir(claudeReal), "/usr/bin", "/bin"}
	r.runner.set(res)
	r.checkRun(doctor.Request{Mode: doctor.ModeLight, SessionUser: r.osu, ClaudeBin: claudeReal, SearchList: res.SearchList,
		Declaration: state.DeclOrganization, Workdirs: []string{wdReal}, AuthStatusTimeout: 10 * time.Second})
	dev, dc := r.openWdi(sessA, "claude", 0)
	out := r.readUntil(dc, dev, "DONE")
	var text strings.Builder
	for _, f := range out {
		text.Write(f.Data)
	}
	got := text.String()
	for _, want := range []string{"cwd=" + wdReal, "config=" + filepath.Join(r.home, ".config", "bothy-host", "claude"), "keylen=22", "lc=" + envCanary} {
		if !strings.Contains(got, want) {
			t.Errorf("claude start lacks %d bytes of expected output", len(want))
		}
	}
	m := regexp.MustCompile(`names=([^\r\n]*)`).FindStringSubmatch(got)
	if m == nil {
		t.Fatal("no environment names")
	}
	for _, name := range []string{"ANTHROPIC_API_KEY", "CLAUDE_CONFIG_DIR", "TERM", "HOME", "PATH"} {
		if !strings.Contains(" "+m[1], " "+name+" ") {
			t.Errorf("environment lacks %s", name)
		}
	}
	for _, name := range []string{"CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_AUTH_TOKEN", "AWS_SECRET_ACCESS_KEY"} {
		if strings.Contains(m[1], name) {
			t.Errorf("environment holds %s", name)
		}
	}
	r.send(dc, dev, frame.Data([]byte("CANARY-5c1e\n")))
	r.readUntil(dc, dev, "got=CANARY-5c1e")
	r.send(dc, dev, frame.Close())
	e := r.note.waitEnded(t, sessA)
	if e.CloseReason != ReasonClientClosed {
		t.Errorf("ended %+v", e)
	}
	// A refused handshake with a token in its close reason would leak; check a refusal too.
	r.sessionOpen(sessB, "shell", 20*time.Second)
	dcB := r.srv.Data(r.ctx, sessB, 1)
	devB := r.device(sessB)
	c := r.claims(sessB, "shell", AttOpen, devB)
	c["aud"] = otherID
	tok := r.signer.Sign(c)
	m1, _ := devB.Message1(tok)
	_ = dcB.SendBinary(r.ctx, m1)
	_, reason := r.closedWith(dcB)
	ended, _ := json.Marshal(r.note.ended)
	all := r.logs.String() + string(ended) + reason
	for _, canary := range []string{"CANARY-5c1e", "EXAMPLE-API-KEY-CANARY", envCanary, tok, c["jti"].(string), wdReal, r.home, claudeReal, "names="} {
		if strings.Contains(all, canary) {
			t.Errorf("a canary of %d bytes reached a log, report or close reason", len(canary))
		}
	}
}

// TestDeclarationAndValues: the values of the hello of a data connection set the leeway of its
// token check, and a new declaration of a later ready changes the offer for new openings while a
// running session continues.
func TestDeclarationAndValues(t *testing.T) {
	v := sessiontest.DefaultValues
	v.ClockLeewayMs = 0
	r := newRig(t, withValues(v))
	expiredBy := func(id string, d time.Duration) (int, string) {
		r.sessionOpen(id, "shell", 20*time.Second)
		dc := r.srv.Data(r.ctx, id, 1)
		dev := r.device(id)
		c := r.claims(id, "shell", AttOpen, dev)
		c["exp"] = time.Now().Add(-d).Unix()
		m1, _ := dev.Message1(r.signer.Sign(c))
		_ = dc.SendBinary(r.ctx, m1)
		if !stillOpen(dc, 300*time.Millisecond) {
			return r.closedWith(dc)
		}
		dc.Close(CodeClosed, "")
		return 0, ""
	}
	if code, reason := expiredBy(sessiontest.NewID(), 2*time.Second); code != CodeTokenStale || reason != DetailTokenExpired {
		t.Errorf("leeway 0: %d %q", code, reason)
	}
	r.values.ClockLeewayMs = (time.Minute).Milliseconds()
	if code, _ := expiredBy(sessiontest.NewID(), 2*time.Second); code != 0 {
		t.Errorf("leeway of a minute refused with %d", code)
	}
	// A running session continues when the declaration changes; new openings use the new offer.
	res := shellResult(r.osu)
	res.Profiles = append(res.Profiles, doctor.Profile{Name: "claude", DisplayName: "Claude", Kind: config.KindClaude})
	res.Workdirs[r.osu] = []doctor.Workdir{{DisplayName: "Project", Path: r.home}}
	res.ClaudeBin = "/bin/sh"
	r.runner.set(res)
	_, dc := r.open(sessA, "shell")
	r.note.wait(t, "started", func() bool { return r.note.startedCount(sessA) == 1 })
	if !hasProfile(profiles.Offered(res), "claude") {
		t.Fatal("organization with claude_bin offers claude")
	}
	none := res.WithDeclaration(state.DeclNone)
	r.runner.set(none)
	if hasProfile(profiles.Offered(none), "claude") {
		t.Error("none offers claude")
	}
	if !stillOpen(dc, 300*time.Millisecond) || r.mgr.Active() != 1 {
		t.Error("the running session ended with the new declaration")
	}
	r.sessionOpen(sessB, "claude", 20*time.Second)
	dcB := r.srv.Data(r.ctx, sessB, 1)
	devB := r.device(sessB)
	c := r.claims(sessB, "claude", AttOpen, devB)
	c["wdi"] = 0
	m1, _ := devB.Message1(r.signer.Sign(c))
	_ = dcB.SendBinary(r.ctx, m1)
	if code, reason := r.closedWith(dcB); code != CodeHandshakeFail || reason != DetailProfileUnknown {
		t.Errorf("claude after none: %d %q", code, reason)
	}
}

func hasProfile(ps []doctor.Profile, name string) bool {
	for _, p := range ps {
		if p.Name == name {
			return true
		}
	}
	return false
}

// TestOpeningAtomicAgainstEnd: a session.close or the end of all sessions while the helper starts
// the PTY leaves no PTY behind (review M1).
func TestOpeningAtomicAgainstEnd(t *testing.T) {
	for _, end := range []string{"close", "endall"} {
		t.Run(end, func(t *testing.T) {
			r := newRig(t, withStartDelay(800*time.Millisecond))
			id := sessiontest.NewID()
			r.open(id, "shell")
			waitFor(t, "the opening registered", func() bool {
				_, ok := r.mgr.liveOf(id)
				return ok
			})
			if end == "close" {
				r.mgr.Close(r.ctx, controlClose(id, "access_revoked"))
			} else {
				r.mgr.EndAll(r.ctx)
			}
			waitFor(t, "the PTY started", func() bool { return r.startCount() == 1 })
			pid := r.startedPIDs()[0]
			waitFor(t, "the late PTY ended", func() bool { return !pidAlive(pid) })
			if r.mgr.Active() != 0 {
				t.Errorf("active %d", r.mgr.Active())
			}
		})
	}
}

// TestRepeatedOpenStartsOnePTY: a repeated session.open of an opening while the first waits for the
// local confirmation starts exactly one PTY, on the newer connection (review M2).
func TestRepeatedOpenStartsOnePTY(t *testing.T) {
	fc := &fakeConfirm{allow: true, delay: 600 * time.Millisecond}
	r := newRig(t, withConfirm(fc))
	r.cfg.LocalConfirmTimeout = 3 * time.Second
	r.open(sessA, "shell")
	waitFor(t, "the first confirmation asked", func() bool {
		fc.mu.Lock()
		defer fc.mu.Unlock()
		return len(fc.asked) == 1
	})
	dev2, dc2 := r.open(sessA, "shell")
	r.note.wait(t, "started", func() bool { return r.note.startedCount(sessA) == 1 })
	time.Sleep(800 * time.Millisecond)
	if r.startCount() != 1 || r.note.startedCount(sessA) != 1 || r.mgr.Active() != 1 {
		t.Fatalf("PTYs %d, started %d, active %d", r.startCount(), r.note.startedCount(sessA), r.mgr.Active())
	}
	r.send(dc2, dev2, frame.Data([]byte("echo SEC''OND\n")))
	r.readUntil(dc2, dev2, "SECOND")
	if r.note.endedCount() != 0 {
		t.Error("the stale opening reported an end")
	}
}
