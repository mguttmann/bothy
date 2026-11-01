// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/install"
	"github.com/OWNER/bothy/services/host/internal/startup"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// ownerFake is newFake with the caller as the account "alex" (uid 1000) of the owner account
// mode (ADR 0073) and its home in a temporary folder.
func ownerFake(t *testing.T) (*fakeSys, string) {
	t.Helper()
	f := newFake(t)
	home := filepath.Join(t.TempDir(), "alex")
	f.w.AddUser("alex", "1000", home, true)
	sys.account = func() (hostos.Account, error) { return hostos.Account{Name: "alex", ID: "1000", Home: home}, nil }
	return f, home
}

// TestInstallOwnerAccountWithoutRoot: install --owner-account runs as the account without
// administrator rights and applies the per-user plan (D-OA-2).
func TestInstallOwnerAccountWithoutRoot(t *testing.T) {
	f, home := ownerFake(t)
	code, out, stderr := runCmd("install", "--owner-account")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	want, err := install.OwnerAccount(install.OwnerTarget{GOOS: "linux", Account: "alex", UID: "1000", Home: home, Running: "/tmp/download/bothy-host"})
	if err != nil || len(f.applied) != 1 || len(f.applied[0]) != len(want) {
		t.Fatalf("applied %v, want %v (%v)", f.applied, want, err)
	}
	for i, s := range f.applied[0] {
		if s.String() != want[i].String() {
			t.Errorf("step %d: %s, want %s", i, s, want[i])
		}
		if s.Owner == "root" || strings.Contains(s.String(), "useradd") {
			t.Errorf("step needs root: %s", s)
		}
	}
	if !strings.Contains(out, "pair") {
		t.Errorf("no next step: %q", out)
	}
}

func TestInstallOwnerAccountRefused(t *testing.T) {
	_, _ = ownerFake(t)
	sys.isAdmin = func() bool { return true }
	if code, _, stderr := runCmd("install", "--owner-account"); code != 2 || !strings.Contains(stderr, "not as root") {
		t.Errorf("root: %d %q", code, stderr)
	}
	sys.isAdmin = func() bool { return false }
	sys.goos = "windows"
	if code, _, stderr := runCmd("install", "--owner-account"); code != 2 || !strings.Contains(stderr, "not supported") {
		t.Errorf("windows: %d %q", code, stderr)
	}
	sys.goos = "linux"
	if code, _, _ := runCmd("install", "--owner-account", "--session-user", "bothy-s-robin"); code != 2 {
		t.Errorf("a session user next to the owner account: %d", code)
	}
}

// writeOwnerConfig writes the configuration install --owner-account would write.
func writeOwnerConfig(t *testing.T, home string) string {
	t.Helper()
	files, _ := install.OwnerPaths(install.OwnerTarget{GOOS: "linux", Account: "alex", UID: "1000", Home: home})
	if err := os.MkdirAll(files.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := "host_service:\n  owner_account: alex\n  state_dir: " + files.StateDir + "\n"
	if err := os.WriteFile(files.Config, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return files.Config
}

func TestRestartOwnerAccount(t *testing.T) {
	f, home := ownerFake(t)
	writeOwnerConfig(t, home)
	code, _, stderr := runCmd("restart")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	want, _ := install.OwnerRestart(install.OwnerTarget{GOOS: "linux", UID: "1000"})
	if len(f.applied) != 1 || f.applied[0][1].String() != want[1].String() {
		t.Fatalf("applied %v", f.applied)
	}
	// Another account than the owner account needs administrator rights as before.
	sys.callerID = func() string { return "1001" }
	if code, _, _ := runCmd("restart"); code != 2 {
		t.Errorf("another account: %d", code)
	}
}

func TestSessionUserRefusedForOwnerAccount(t *testing.T) {
	_, home := ownerFake(t)
	cfg := writeOwnerConfig(t, home)
	sys.isAdmin = func() bool { return true }
	if code, _, stderr := runCmd("session-user", "--config", cfg, "add", "bothy-s-robin"); code != 2 || !strings.Contains(stderr, "owner_account") {
		t.Errorf("%d %q", code, stderr)
	}
}

func TestDoctorEnvOwnerAccount(t *testing.T) {
	_, home := ownerFake(t)
	c, err := sys.loadConfig("")
	if err == nil {
		t.Fatalf("a missing configuration loaded: %s", c.Path)
	}
	cfg := writeOwnerConfig(t, home)
	c, err = sys.loadConfig("") // falls back to the owner account path
	if err != nil || c.Path != cfg || c.OwnerAccount != "alex" {
		t.Fatalf("fallback %v %+v", err, c)
	}
	files, def := install.OwnerPaths(install.OwnerTarget{GOOS: "linux", Account: "alex", UID: "1000", Home: home})
	env := sys.doctorEnv(c, state.Open(c.StateDir, sys.protector))
	kids, ok := env.Helpers.(startup.ChannelHelpers).Services.(*startup.ChildHelpers)
	if !ok || kids.Dir != files.RunDir {
		t.Fatalf("helpers %+v", env.Helpers)
	}
	if env.ServiceUser != "alex" || env.OwnerAccount != "alex" || env.CallerID != "1000" || env.HelperBinary != files.Binary ||
		len(env.ServiceDefinitions) != 1 || env.ServiceDefinitions[0] != def.Path || len(env.EndpointDirs) != 1 || env.EndpointDirs[0] != files.RunDir {
		t.Errorf("env %+v", env)
	}
}
