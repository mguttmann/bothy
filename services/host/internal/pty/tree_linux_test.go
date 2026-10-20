// SPDX-License-Identifier: Apache-2.0

package pty

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"golang.org/x/sys/unix"
)

const isLinux = true

// inScopeMarker is the positional argument of a test binary that runs inside the systemd scope
// TestSessionCgroup asked for; inside, a missing delegation fails instead of re-executing again.
const inScopeMarker = "bothy-pty-in-delegated-scope"

var (
	delegationOnce sync.Once
	delegatedRoot  string
)

// ownCgroupDir returns the cgroup v2 directory of the test process.
func ownCgroupDir(t *testing.T) string {
	t.Helper()
	dir, err := ownCgroup()
	if err != nil {
		t.Fatalf("own cgroup: %v", err)
	}
	return dir
}

// delegated returns the own cgroup directory when the test process may create cgroups below it,
// or "" when the runner gives no delegated cgroup.
func delegated() string {
	delegationOnce.Do(func() {
		dir, err := ownCgroup()
		if err != nil {
			return
		}
		if unix.Access(dir, unix.W_OK) == nil && unix.Access(filepath.Join(dir, "cgroup.procs"), unix.W_OK) == nil {
			delegatedRoot = dir
		}
	})
	return delegatedRoot
}

// testOptions uses the delegated cgroup where the runner has one and process groups otherwise;
// only TestSessionCgroup insists on a cgroup.
func testOptions(t *testing.T) Options {
	t.Helper()
	if delegated() != "" {
		return Options{}
	}
	return Options{ProcessGroupOnly: true}
}

// catchesOrphans reports whether a tree end finds a setsid child whose parent ended: only with a
// session cgroup.
func catchesOrphans(t *testing.T) bool { return !testOptions(t).ProcessGroupOnly }

// isZombie reports whether pid is a zombie according to /proc.
func isZombie(pid int) bool {
	e, err := readProc(pid)
	return err == nil && e.zombie
}

// runPlatformTestHelper has no Linux-only modes.
func runPlatformTestHelper(string, []string) int { return 2 }

func TestSessionCgroup(t *testing.T) {
	if delegated() == "" {
		if slices.Contains(flag.Args(), inScopeMarker) {
			t.Fatal("inside the systemd scope with Delegate=yes the own cgroup is still not writable; " +
				"the runner must run the tests as a user with a systemd user manager (loginctl enable-linger) " +
				"or inside a cgroup delegated to that user")
		}
		runInDelegatedScope(t)
		return
	}
	root := ownCgroupDir(t)
	if filepath.Base(root) == helperLeaf {
		root = filepath.Dir(root)
	}
	s, err := New(Options{})
	if err != nil {
		t.Fatalf("New in the delegated cgroup: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if got := ownCgroupDir(t); got != filepath.Join(root, helperLeaf) {
		t.Fatalf("own cgroup after New = %q, want the leaf %q", got, filepath.Join(root, helperLeaf))
	}

	p1, _, pids1 := startTree(t, s, testSessionID(50))
	p2, _, pids2 := startTree(t, s, testSessionID(51))
	for i, p := range []Process{p1, p2} {
		dir := filepath.Join(root, "s-"+testSessionID(50+i))
		procs := cgroupPIDs(t, dir)
		if !slices.Contains(procs, p.PID()) {
			t.Fatalf("leader %d is not in its session cgroup (%v)", p.PID(), procs)
		}
	}
	for _, pid := range pids1[1:] {
		if !slices.Contains(cgroupPIDs(t, filepath.Join(root, "s-"+testSessionID(50))), pid) {
			t.Fatalf("background process %d is not in the session cgroup", pid)
		}
	}

	if err := p1.Kill(); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	waitGone(t, pids1...)
	if _, err := os.Stat(filepath.Join(root, "s-"+testSessionID(50))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the cgroup of the ended session still exists (%v)", err)
	}
	if !alive(p2.PID()) {
		t.Fatal("the end of one session ended another one")
	}
	s.KillAll()
	waitGone(t, pids2...)
	if _, err := os.Stat(filepath.Join(root, "s-"+testSessionID(51))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the cgroup of the second session still exists (%v)", err)
	}
}

// cgroupPIDs reads cgroup.procs of dir.
func cgroupPIDs(t *testing.T, dir string) []int {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "cgroup.procs")) //nolint:gosec // a cgroup of this test
	if err != nil {
		t.Fatalf("read cgroup.procs: %v", err)
	}
	var pids []int
	for _, f := range strings.Fields(string(raw)) {
		n, err := strconv.Atoi(f)
		if err == nil {
			pids = append(pids, n)
		}
	}
	return pids
}

// runInDelegatedScope re-executes TestSessionCgroup inside a transient systemd user scope with
// Delegate=yes, the same delegation the helper unit gets, and fails when that is not possible.
func runInDelegatedScope(t *testing.T) {
	t.Helper()
	run, err := exec.LookPath("systemd-run")
	if err != nil {
		t.Fatal("no delegated cgroup: the own cgroup is not writable and systemd-run is missing; " +
			"run the tests inside a cgroup delegated to the test user or on a host with a systemd user manager")
	}
	//nolint:gosec // systemd-run with the test binary itself
	cmd := exec.CommandContext(t.Context(), run, "--user", "--scope", "-p", "Delegate=yes", "--quiet",
		testBinary(t), "-test.run=^TestSessionCgroup$", "-test.count=1", "-test.v", inScopeMarker)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("TestSessionCgroup in a systemd user scope with Delegate=yes failed (%v); "+
			"the runner needs a systemd user manager for the test user (loginctl enable-linger) "+
			"or a cgroup delegated to it; output:\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "--- PASS: TestSessionCgroup") {
		t.Fatalf("TestSessionCgroup did not pass in the scope; output:\n%s", out.String())
	}
}

func TestNoDelegation(t *testing.T) {
	// A folder that is no cgroup v2 directory gives no delegation, whatever its permissions.
	if _, err := New(Options{CgroupRoot: t.TempDir()}); !errors.Is(err, ErrNoDelegation) {
		t.Fatalf("New with a plain folder = %v, want ErrNoDelegation", err)
	}
}
