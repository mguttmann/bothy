// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package main

import (
	"testing"

	"golang.org/x/sys/unix"
)

// keepStdin restores standard input of the test process after the test: on macOS the helper takes
// its activated socket from fd 0 and closes it, and a later descriptor of the runtime that lands on
// fd 0 can then be guarded by the system, which ends the test process at the next run.
func keepStdin(t *testing.T) {
	t.Helper()
	saved, err := unix.Dup(0)
	if err != nil {
		t.Fatalf("dup of standard input: %v", err)
	}
	t.Cleanup(func() {
		_ = unix.Dup2(saved, 0)
		_ = unix.Close(saved)
	})
}
