// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// On macOS the run folder of the session trees lives in the home of the session user; when it
// cannot be created, no PTY may start.
func TestPtyPartsRunFolderFails(t *testing.T) {
	newFake(t)
	sys.goos = hostos.Darwin
	home := filepath.Join(t.TempDir(), "home")
	if err := os.WriteFile(home, []byte("not a folder"), 0o600); err != nil {
		t.Fatal(err)
	}
	sup, _, err := ptyParts(hostos.Account{Name: "bothy-s-alex", ID: "1001", Home: home}, doctor.HelperEnv{})
	if err == nil || sup != nil {
		t.Fatalf("ptyParts = %v, %v; want an error without the run folder", sup, err)
	}
}

// On macOS ptyParts creates the run folder with mode 0700 and passes the login shells of
// /etc/shells to the profiles.
func TestPtyPartsDarwin(t *testing.T) {
	if runtime.GOOS != hostos.Darwin {
		t.Skip("the session trees of macOS")
	}
	newFake(t)
	sys.goos = hostos.Darwin
	home := t.TempDir()
	self := hostos.Account{Name: "bothy-s-alex", ID: "1001", Home: home}
	sup, penv, err := ptyParts(self, doctor.HelperEnv{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sup.Close() })
	fi, err := os.Stat(filepath.Join(home, ".config", "bothy-host", "run"))
	if err != nil || !fi.IsDir() || fi.Mode().Perm() != 0o700 {
		t.Fatalf("run folder %v %v, want a folder with 0700", fi, err)
	}
	want, _, err := profileExtras()
	if err != nil {
		t.Fatal(err)
	}
	if penv.GOOS != hostos.Darwin || penv.Self != self || !slices.Equal(penv.Shells, want) || penv.EnvBlock != nil {
		t.Fatalf("profiles environment %+v", penv)
	}
}

// profileExtras lists the login shells of /etc/shells without comments and empty lines, and no
// environment block.
func TestProfileExtras(t *testing.T) {
	b, err := os.ReadFile("/etc/shells")
	if err != nil {
		t.Skip("no /etc/shells on this machine")
	}
	shells, block, err := profileExtras()
	if err != nil || block != nil {
		t.Fatalf("profileExtras = %v, %v", block, err)
	}
	lines := strings.Split(string(b), "\n")
	for _, s := range shells {
		if s == "" || strings.HasPrefix(s, "#") || s != strings.TrimSpace(s) || !slices.ContainsFunc(lines, func(l string) bool { return strings.TrimSpace(l) == s }) {
			t.Errorf("shell %q is not a listed line of /etc/shells", s)
		}
	}
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") && !slices.Contains(shells, l) {
			t.Errorf("listed shell %q is missing", l)
		}
	}
}
