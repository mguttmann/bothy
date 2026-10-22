// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// fakeLoginctlScript writes an executable loginctl that reports user uid for the session c2.
func fakeLoginctlScript(t *testing.T, dir, name, uid string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\ncase \"$1\" in\nshow-seat) echo ActiveSession=c2 ;;\nshow-session) echo User=" + uid + " ;;\nesac\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil { //nolint:gosec // an executable test double
		t.Fatal(err)
	}
	return path
}

// ConsoleOwner runs the first loginctl of loginctlPaths that exists and refuses without one.
func TestConsoleOwnerUsesFirstLoginctl(t *testing.T) {
	old := loginctlPaths
	t.Cleanup(func() { loginctlPaths = old })
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing")
	first := fakeLoginctlScript(t, dir, "first", uidRobin)
	second := fakeLoginctlScript(t, dir, "second", uidOther)

	loginctlPaths = []string{missing, first, second}
	if uid, err := ConsoleOwner(context.Background()); err != nil || uid != uidRobin {
		t.Fatalf("ConsoleOwner = %q, %v; want %s from the first loginctl", uid, err, uidRobin)
	}
	loginctlPaths = []string{missing}
	if uid, err := ConsoleOwner(context.Background()); !errors.Is(err, errNoConsole) {
		t.Fatalf("ConsoleOwner without loginctl = %q, %v; want errNoConsole", uid, err)
	}
}
