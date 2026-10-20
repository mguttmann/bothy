// SPDX-License-Identifier: Apache-2.0

package pty

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"golang.org/x/sys/unix"
)

const isLinux = true

var (
	delegationOnce sync.Once
	delegatedRoot  string
)

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

func TestNoDelegation(t *testing.T) {
	// A folder that is no cgroup v2 directory gives no delegation, whatever its permissions.
	if _, err := New(Options{CgroupRoot: t.TempDir()}); !errors.Is(err, ErrNoDelegation) {
		t.Fatalf("New with a plain folder = %v, want ErrNoDelegation", err)
	}
}
