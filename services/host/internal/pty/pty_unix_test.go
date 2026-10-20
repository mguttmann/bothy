// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package pty

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// testPath is the PATH of the test sessions; it holds only system folders, never the runner's own.
const testPath = "PATH=/usr/bin:/bin:/usr/sbin:/sbin"

// shellSpec returns the spec of a /bin/sh session that runs script.
func shellSpec(t *testing.T, id, script string) StartSpec {
	t.Helper()
	return StartSpec{
		SessionID: id,
		Path:      "/bin/sh",
		Args:      []string{"-c", script},
		Env:       []string{testPath, "TERM=xterm-256color", "HOME=" + t.TempDir()},
		Dir:       t.TempDir(),
		Cols:      80,
		Rows:      24,
	}
}

// runTestHelper runs one helper mode of the re-executed test binary and returns its exit code.
func runTestHelper(mode string, args []string) int {
	switch mode {
	case "sleep":
		time.Sleep(time.Hour)
		return 0
	case "setsid-sleep":
		// The process leaves the session of the PTY like `setsid sleep` would.
		if _, err := unix.Setsid(); err != nil {
			fmt.Println("SETSID-FAILED")
			return 1
		}
		fmt.Println("SETSID-OK")
		time.Sleep(time.Hour)
		return 0
	default:
		return runPlatformTestHelper(mode, args)
	}
}

// alive reports whether a process with pid exists and is not a zombie.
func alive(pid int) bool {
	if err := unix.Kill(pid, 0); errors.Is(err, unix.ESRCH) {
		return false
	}
	return !isZombie(pid)
}

// waitGone fails the test when one of pids still exists after the wait limit.
func waitGone(t *testing.T, pids ...int) {
	t.Helper()
	deadline := time.Now().Add(waitLimit)
	for _, pid := range pids {
		for alive(pid) {
			if time.Now().After(deadline) {
				t.Fatalf("process %d of the tree still exists", pid)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

// setsidCommand returns the shell command that starts a child leaving the session of the PTY:
// the setsid program on Linux where installed, otherwise the test binary calling setsid itself
// (macOS has no setsid program).
func setsidCommand(t *testing.T) string {
	t.Helper()
	if p, err := exec.LookPath("setsid"); err == nil && isLinux {
		return p + " sleep 1000"
	}
	return testBinary(t) + " " + helperMarker + " setsid-sleep"
}

func TestPTYStartEchoResizeExit(t *testing.T) {
	s, err := New(testOptions(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	t.Run("size, resize and exit code", func(t *testing.T) {
		sp := shellSpec(t, testSessionID(10),
			`echo "S1:$(stty size)"; read a; echo "S2:$(stty size)"; echo "ENV:[${BOTHY_PTY_CANARY-unset}]"; read b; exit 7`)
		p, err := s.Start(sp)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		t.Cleanup(func() { _ = p.Kill() })
		if p.PID() <= 0 {
			t.Fatalf("PID = %d", p.PID())
		}
		out := collect(p)
		out.waitMatch(t, regexp.MustCompile(`S1:24 80`))
		if err := p.Resize(100, 40); err != nil {
			t.Fatalf("Resize: %v", err)
		}
		if _, err := p.Write([]byte("\n")); err != nil {
			t.Fatalf("Write: %v", err)
		}
		out.waitMatch(t, regexp.MustCompile(`S2:40 100`))
		out.waitMatch(t, regexp.MustCompile(`ENV:\[unset\]`))
		if _, err := p.Write([]byte("\n")); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if code := waitCode(t, p); code != 7 {
			t.Fatalf("exit code = %d, want 7", code)
		}
		if code := p.Wait(); code != 7 {
			t.Fatalf("second Wait = %d, want 7", code)
		}
		if err := p.Kill(); err != nil {
			t.Fatalf("Kill after Wait: %v", err)
		}
		if err := p.Kill(); err != nil {
			t.Fatalf("second Kill: %v", err)
		}
		out.waitEnd(t)
	})

	t.Run("signal of the leader", func(t *testing.T) {
		p, err := s.Start(shellSpec(t, testSessionID(11), `echo READY; while :; do sleep 1; done`))
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		t.Cleanup(func() { _ = p.Kill() })
		out := collect(p)
		out.waitMatch(t, regexp.MustCompile(`READY`))
		if err := unix.Kill(p.PID(), unix.SIGKILL); err != nil {
			t.Fatalf("kill of the leader: %v", err)
		}
		if code := waitCode(t, p); code != 128+int(syscall.SIGKILL) {
			t.Fatalf("exit code = %d, want %d", code, 128+int(syscall.SIGKILL))
		}
	})

	t.Run("environment is never inherited", func(t *testing.T) {
		// Every test of this package may set a variable the session must not see; the canary is
		// set on the test process and the spec leaves it out.
		t.Setenv("BOTHY_PTY_CANARY", "canary-value")
		p, err := s.Start(shellSpec(t, testSessionID(12), `echo "ENV:[${BOTHY_PTY_CANARY-unset}]"`))
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		t.Cleanup(func() { _ = p.Kill() })
		out := collect(p)
		out.waitMatch(t, regexp.MustCompile(`ENV:\[unset\]`))
		if code := waitCode(t, p); code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
	})
}

// treeScript starts a nohup background process and a child that leaves the session, prints their
// PIDs and keeps the leader alive.
func treeScript(t *testing.T) string {
	t.Helper()
	return `nohup sleep 1000 >/dev/null 2>&1 &
echo "BG:$!"
` + setsidCommand(t) + ` &
echo "SS:$!"
while :; do sleep 1; done`
}

// startTree starts a session with treeScript and returns the PIDs of its leader, its nohup
// background process and its setsid child once the setsid child left the session.
func startTree(t *testing.T, s *Supervisor, id string) (p Process, out *collector, pids []int) {
	t.Helper()
	p, err := s.Start(shellSpec(t, id, treeScript(t)))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = p.Kill() })
	out = collect(p)
	bg := atoi(t, out.waitMatch(t, regexp.MustCompile(`BG:(\d+)`))[1])
	ss := atoi(t, out.waitMatch(t, regexp.MustCompile(`SS:(\d+)`))[1])
	// The setsid child is in its own session only once setsid returned.
	deadline := time.Now().Add(waitLimit)
	for {
		sid, err := unix.Getsid(ss)
		if err == nil && sid == ss {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the setsid child %d never left the session (sid %d, %v)", ss, sid, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, pid := range []int{p.PID(), bg, ss} {
		if !alive(pid) {
			t.Fatalf("process %d of the tree is not running before the end", pid)
		}
	}
	return p, out, []int{p.PID(), bg, ss}
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("not a number: %q", s)
	}
	return n
}

func TestKillTreeEndsBackground(t *testing.T) {
	s, err := New(testOptions(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	t.Run("kill", func(t *testing.T) {
		p, out, pids := startTree(t, s, testSessionID(20))
		if err := p.Kill(); err != nil {
			t.Fatalf("Kill: %v", err)
		}
		waitGone(t, pids...)
		if code := waitCode(t, p); code != 128+int(syscall.SIGKILL) {
			t.Fatalf("exit code after Kill = %d, want %d", code, 128+int(syscall.SIGKILL))
		}
		out.waitEnd(t)
	})

	t.Run("leader exit ends the rest of the tree", func(t *testing.T) {
		p, out, pids := startTree(t, s, testSessionID(21))
		if err := unix.Kill(p.PID(), unix.SIGTERM); err != nil {
			t.Fatalf("SIGTERM of the leader: %v", err)
		}
		if code := waitCode(t, p); code != 128+int(syscall.SIGTERM) {
			t.Fatalf("exit code = %d, want %d", code, 128+int(syscall.SIGTERM))
		}
		// The setsid child was a child of the leader; the leader's exit hands it to init before
		// the tree end can see the parent chain. Only a session cgroup still holds it; without
		// one this is the limit of docs/spec/24-terminal.md 6.5 and the test ends it itself.
		if catchesOrphans(t) {
			waitGone(t, pids...)
		} else {
			waitGone(t, pids[:2]...)
			_ = unix.Kill(pids[2], unix.SIGKILL)
		}
		out.waitEnd(t)
		if err := p.Kill(); err != nil {
			t.Fatalf("Kill after the end: %v", err)
		}
	})
}

func TestHelperChannelBreakEndsPTYs(t *testing.T) {
	s, err := New(testOptions(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	p1, _, pids1 := startTree(t, s, testSessionID(30))
	p2, _, pids2 := startTree(t, s, testSessionID(31))

	s.KillAll()

	waitGone(t, append(pids1, pids2...)...)
	for _, p := range []Process{p1, p2} {
		if code := waitCode(t, p); code != 128+int(syscall.SIGKILL) {
			t.Fatalf("exit code = %d, want %d", code, 128+int(syscall.SIGKILL))
		}
	}
	// The helper waits for the next connection with the same supervisor.
	p3, err := s.Start(shellSpec(t, testSessionID(32), "exit 3"))
	if err != nil {
		t.Fatalf("Start after KillAll: %v", err)
	}
	t.Cleanup(func() { _ = p3.Kill() })
	if code := waitCode(t, p3); code != 3 {
		t.Fatalf("exit code = %d, want 3", code)
	}
}

func TestCloseEndsPTYs(t *testing.T) {
	s, err := New(testOptions(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p, _, pids := startTree(t, s, testSessionID(40))
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	waitGone(t, pids...)
	if code := waitCode(t, p); code != 128+int(syscall.SIGKILL) {
		t.Fatalf("exit code = %d, want %d", code, 128+int(syscall.SIGKILL))
	}
}
