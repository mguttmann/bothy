// SPDX-License-Identifier: Apache-2.0

package pty

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"
)

const isLinux = false

// testOptions gives every test supervisor its own run folder, as the helper does with its home.
func testOptions(t *testing.T) Options {
	t.Helper()
	return Options{RunDir: t.TempDir()}
}

// catchesOrphans reports whether a tree end finds a setsid child whose parent ended: never on
// macOS (docs/spec/24-terminal.md 6.5, "Limit").
func catchesOrphans(*testing.T) bool { return false }

// isZombie reports whether pid is a zombie in a fresh process table snapshot.
func isZombie(pid int) bool {
	procs, err := snapshot()
	if err != nil {
		return false
	}
	for _, p := range procs {
		if p.pid == pid {
			return p.zombie
		}
	}
	return false
}

// runPlatformTestHelper runs the macOS helper modes of the re-executed test binary.
//
// Mode "supervisor <run dir> <trap>": a helper process with a supervisor and one session whose
// shell starts a nohup background process; it prints the PIDs and then blocks until it is killed.
// With trap "trap" the leader ignores the hangup of its terminal, so it outlives the helper.
func runPlatformTestHelper(mode string, args []string) int {
	if mode != "supervisor" || len(args) != 2 {
		return 2
	}
	s, err := New(Options{RunDir: args[0]})
	if err != nil {
		fmt.Println("NEW-FAILED")
		return 1
	}
	script := `nohup sleep 1000 >/dev/null 2>&1 &
echo "BG:$!"
while :; do sleep 1; done`
	if args[1] == "trap" {
		script = `trap "" HUP
` + script
	}
	home, err := os.MkdirTemp(args[0], "home")
	if err != nil {
		return 1
	}
	p, err := s.Start(StartSpec{
		SessionID: testSessionID(90),
		Path:      "/bin/sh",
		Args:      []string{"-c", script},
		Env:       []string{testPath, "TERM=xterm-256color", "HOME=" + home},
		Dir:       home,
		Cols:      80,
		Rows:      24,
	})
	if err != nil {
		fmt.Println("START-FAILED")
		return 1
	}
	out := collect(p)
	re := regexp.MustCompile(`BG:(\d+)`)
	for {
		if m := re.FindStringSubmatch(out.text()); m != nil {
			fmt.Printf("TREE %d %s\n", p.PID(), m[1])
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	select {}
}

// runFileEntries reads the run file of dir.
func runFileEntries(t *testing.T, dir string) []runEntry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, runFileName)) //nolint:gosec // the run file of this test
	if err != nil {
		t.Fatalf("read run file: %v", err)
	}
	var entries []runEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("run file is not a JSON list: %v", err)
	}
	return entries
}

func TestRunFileCleanup(t *testing.T) {
	for _, mode := range []string{"trap", "hangup"} {
		t.Run(mode, func(t *testing.T) {
			runDir := t.TempDir()
			cmd := exec.CommandContext(t.Context(), testBinary(t), helperMarker, "supervisor", runDir, mode) //nolint:gosec // the test binary itself
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
			var leader, bg int
			if _, err := fmt.Sscanf(line, "TREE %d %d", &leader, &bg); err != nil {
				t.Fatalf("parse %q: %v", line, err)
			}
			entries := runFileEntries(t, runDir)
			if len(entries) != 1 || entries[0].PID != leader || entries[0].SID != leader || entries[0].PGID != leader {
				t.Fatalf("run file = %+v, want one entry for leader %d", entries, leader)
			}
			info, err := os.Stat(filepath.Join(runDir, runFileName))
			if err != nil || info.Mode().Perm() != 0o600 {
				t.Fatalf("run file mode = %v (%v), want 0600", info.Mode().Perm(), err)
			}

			// The helper dies without a chance to end its tree.
			if err := cmd.Process.Kill(); err != nil {
				t.Fatalf("kill of the helper process: %v", err)
			}
			_ = cmd.Wait()
			if !alive(bg) {
				t.Fatalf("the background process %d died with the helper; the test proves nothing", bg)
			}
			if mode == "trap" && !alive(leader) {
				t.Fatalf("the leader %d died with the helper although it ignores the hangup", leader)
			}

			s, err := New(Options{RunDir: runDir})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			t.Cleanup(func() { _ = s.Close() })
			waitGone(t, leader, bg)
			if entries := runFileEntries(t, runDir); len(entries) != 0 {
				t.Fatalf("run file after New = %+v, want empty", entries)
			}
		})
	}
}

func TestRunFileSkipsReusedLeader(t *testing.T) {
	// A recorded leader PID whose start time differs belongs to another process now; New must
	// leave it alone. A child in its own session plays that process, so a wrong kill stays inside
	// the test.
	other := exec.CommandContext(t.Context(), testBinary(t), helperMarker, "sleep") //nolint:gosec // the test binary itself
	other.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := other.Start(); err != nil {
		t.Fatalf("start of the other process: %v", err)
	}
	t.Cleanup(func() { _ = other.Process.Kill(); _ = other.Wait() })
	pid := other.Process.Pid
	runDir := t.TempDir()
	entries := []runEntry{{PID: pid, StartSec: 1, StartUsec: 2, PGID: pid, SID: pid}}
	raw, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, runFileName), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := New(Options{RunDir: runDir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if got := runFileEntries(t, runDir); len(got) != 0 {
		t.Fatalf("run file after New = %+v, want empty", got)
	}
	if !alive(pid) {
		t.Fatalf("New killed process %d whose start time differs from the record", pid)
	}
}

func TestRunFileUnreadable(t *testing.T) {
	runDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(runDir, runFileName), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := New(Options{RunDir: runDir})
	if err != nil {
		t.Fatalf("New with a damaged run file: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if got := runFileEntries(t, runDir); len(got) != 0 {
		t.Fatalf("run file after New = %+v, want empty", got)
	}
}
