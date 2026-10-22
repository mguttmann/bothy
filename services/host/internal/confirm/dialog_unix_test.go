// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package confirm

import (
	"os"
	"path/filepath"
	"testing"
)

// toolExists accepts only an executable regular file as a dialog or loginctl program.
func TestToolExists(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "exe")
	plain := filepath.Join(dir, "plain")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o700); err != nil { //nolint:gosec // an executable test double
		t.Fatal(err)
	}
	if err := os.WriteFile(plain, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]bool{exe: true, plain: false, dir: false, filepath.Join(dir, "missing"): false} {
		if got := toolExists(path); got != want {
			t.Errorf("toolExists(%s) = %v, want %v", filepath.Base(path), got, want)
		}
	}
}
