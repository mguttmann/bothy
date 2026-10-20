// SPDX-License-Identifier: Apache-2.0

package pty

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// testOptions needs nothing on Windows: the Job Objects belong to the supervisor.
func testOptions(t *testing.T) Options {
	t.Helper()
	return Options{}
}

// systemRoot returns the Windows folder from the system folder, without reading the environment.
func systemRoot(t *testing.T) string {
	t.Helper()
	dir, err := windows.GetSystemDirectory()
	if err != nil {
		t.Fatalf("system directory: %v", err)
	}
	return filepath.Dir(dir)
}

// shellSpec returns the spec of a cmd.exe session that runs script.
func shellSpec(t *testing.T, id, script string) StartSpec {
	t.Helper()
	root := systemRoot(t)
	return StartSpec{
		SessionID: id,
		Path:      filepath.Join(root, "System32", "cmd.exe"),
		Args:      []string{"/d", "/c", script},
		Env: []string{
			"SystemRoot=" + root,
			"PATH=" + filepath.Join(root, "System32") + ";" + root,
			"TERM=xterm-256color",
		},
		Dir:  t.TempDir(),
		Cols: 80,
		Rows: 24,
	}
}

// runTestHelper runs one helper mode of the re-executed test binary and returns its exit code.
//
// Mode "sleep" sleeps. Mode "children" tries a child with CREATE_BREAKAWAY_FROM_JOB, which the
// job refuses, starts a detached child, prints both results and sleeps. Mode "supervisor" holds
// a supervisor with one session running mode "children" and prints the PID of the detached child.
func runTestHelper(mode string, _ []string) int {
	switch mode {
	case "sleep":
		time.Sleep(time.Hour)
		return 0
	case "children":
		return helperChildren()
	case "supervisor":
		return helperSupervisor()
	default:
		return 2
	}
}

// startChild starts the test binary in mode "sleep" with flags and returns its PID.
func startChild(flags uint32) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, err
	}
	app, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return 0, err
	}
	line, err := windows.UTF16PtrFromString(windows.ComposeCommandLine([]string{exe, helperMarker, "sleep"}))
	if err != nil {
		return 0, err
	}
	var si windows.StartupInfo
	si.Cb = uint32(unsafe.Sizeof(si))
	var pi windows.ProcessInformation
	if err := windows.CreateProcess(app, line, nil, nil, false, flags, nil, nil, &si, &pi); err != nil {
		return 0, err
	}
	_ = windows.CloseHandle(pi.Thread)
	_ = windows.CloseHandle(pi.Process)
	return int(pi.ProcessId), nil
}

func helperChildren() int {
	if _, err := startChild(windows.CREATE_BREAKAWAY_FROM_JOB); err != nil {
		fmt.Println("BREAKAWAY:REFUSED")
	} else {
		fmt.Println("BREAKAWAY:ALLOWED")
	}
	pid, err := startChild(windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP)
	if err != nil {
		fmt.Println("DETACHED:FAILED")
		return 1
	}
	fmt.Printf("DETACHED:%d\n", pid)
	time.Sleep(time.Hour)
	return 0
}

func helperSupervisor() int {
	s, err := New(Options{})
	if err != nil {
		return 1
	}
	exe, err := os.Executable()
	if err != nil {
		return 1
	}
	dir, err := windows.GetSystemDirectory()
	if err != nil {
		return 1
	}
	home, err := os.MkdirTemp("", "bothy-pty")
	if err != nil {
		return 1
	}
	p, err := s.Start(StartSpec{
		SessionID: testSessionID(90),
		Path:      exe,
		Args:      []string{helperMarker, "children"},
		Env:       []string{"SystemRoot=" + filepath.Dir(dir), "TERM=xterm-256color"},
		Dir:       home,
		Cols:      80,
		Rows:      24,
	})
	if err != nil {
		return 1
	}
	out := collect(p)
	re := regexp.MustCompile(`DETACHED:(\d+)`)
	for {
		if m := re.FindStringSubmatch(out.text()); m != nil {
			fmt.Printf("TREE %d %s\n", p.PID(), m[1])
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	select {}
}

// alive reports whether a process with pid still runs.
func alive(pid int) bool {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid)) //nolint:gosec // a PID from the process table
	if err != nil {
		return false
	}
	defer func() { _ = windows.CloseHandle(h) }()
	ev, err := windows.WaitForSingleObject(h, 0)
	return err == nil && ev == uint32(windows.WAIT_TIMEOUT)
}

// waitGone fails the test when one of pids still runs after the wait limit.
func waitGone(t *testing.T, pids ...int) {
	t.Helper()
	deadline := time.Now().Add(waitLimit)
	for _, pid := range pids {
		for alive(pid) {
			if time.Now().After(deadline) {
				t.Fatalf("process %d of the tree still runs", pid)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func TestPTYStartEchoResizeExit(t *testing.T) {
	s, err := New(testOptions(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	p, err := s.Start(shellSpec(t, testSessionID(10), "mode con & pause >nul & mode con & pause >nul & exit 7"))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = p.Kill() })
	out := collect(p)
	out.waitMatch(t, regexp.MustCompile(`(?s)Lines:\s+24.*Columns:\s+80`))
	if err := p.Resize(100, 40); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if _, err := p.Write([]byte("\r")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out.waitMatch(t, regexp.MustCompile(`(?s)Lines:\s+40.*Columns:\s+100`))
	if _, err := p.Write([]byte("\r")); err != nil {
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
	out.waitEnd(t)
}

// startChildren starts a session running mode "children" and returns the detached child's PID.
func startChildren(t *testing.T, s *Supervisor, id string) (Process, int) {
	t.Helper()
	sp := shellSpec(t, id, "")
	sp.Path = testBinary(t)
	sp.Args = []string{helperMarker, "children"}
	p, err := s.Start(sp)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = p.Kill() })
	out := collect(p)
	out.waitMatch(t, regexp.MustCompile(`BREAKAWAY:REFUSED`))
	m := out.waitMatch(t, regexp.MustCompile(`DETACHED:(\d+)`))
	pid, err := strconv.Atoi(m[1])
	if err != nil || !alive(pid) {
		t.Fatalf("detached child %q is not running (%v)", m[1], err)
	}
	return p, pid
}

func TestKillTreeEndsBackground(t *testing.T) {
	s, err := New(testOptions(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	p, child := startChildren(t, s, testSessionID(20))
	if err := p.Kill(); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	waitGone(t, p.PID(), child)
	if code := waitCode(t, p); code != killedExitCode {
		t.Fatalf("exit code after Kill = %d, want %d", code, killedExitCode)
	}
}

func TestHelperChannelBreakEndsPTYs(t *testing.T) {
	s, err := New(testOptions(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	p1, c1 := startChildren(t, s, testSessionID(30))
	p2, c2 := startChildren(t, s, testSessionID(31))
	s.KillAll()
	waitGone(t, p1.PID(), c1, p2.PID(), c2)
	p3, err := s.Start(shellSpec(t, testSessionID(32), "exit 3"))
	if err != nil {
		t.Fatalf("Start after KillAll: %v", err)
	}
	t.Cleanup(func() { _ = p3.Kill() })
	if code := waitCode(t, p3); code != 3 {
		t.Fatalf("exit code = %d, want 3", code)
	}
}

func TestJobObjectKillOnClose(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), testBinary(t), helperMarker, "supervisor") //nolint:gosec // the test binary itself
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start of the helper process: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || !strings.HasPrefix(line, "TREE ") {
		t.Fatalf("helper process said %q (%v)", line, err)
	}
	var leader, child int
	if _, err := fmt.Sscanf(line, "TREE %d %d", &leader, &child); err != nil {
		t.Fatalf("parse %q: %v", line, err)
	}
	// The helper dies without a chance to end its tree; the job closes with its handle.
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill of the helper process: %v", err)
	}
	_ = cmd.Wait()
	waitGone(t, leader, child)
}
