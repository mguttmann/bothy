// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/hostos/hostostest"
	"github.com/OWNER/bothy/services/host/internal/install"
	"github.com/OWNER/bothy/services/host/internal/localchan"
	"github.com/OWNER/bothy/services/host/internal/state"
)

type fakeSys struct {
	t        *testing.T
	w        *hostostest.World
	cfg      string
	state    string
	applied  [][]install.Step
	applyErr error
	child    [][]string
	childIn  []byte
}

// newFake installs a fake operating system: Linux, caller uid 1000 (not an administrator), a
// configuration file with two session users and a state directory owned by the service user.
func newFake(t *testing.T) *fakeSys {
	t.Helper()
	dir := t.TempDir()
	f := &fakeSys{t: t, w: hostostest.New("linux"), cfg: filepath.Join(dir, "config.yaml"), state: filepath.Join(dir, "state")}
	f.w.AddUser(serviceUser, "999", f.state, false)
	f.w.AddUser("bothy-s-alex", "1001", "/home/bothy-s-alex", true)
	f.w.AddUser("bothy-s-sam", "1002", "/home/bothy-s-sam", true)
	f.w.Dir(f.state, "999", 0o700)
	for _, u := range []string{"bothy-s-alex", "bothy-s-sam"} {
		f.w.Dir("/home/"+u, f.w.UID(u), 0o700)
	}
	if err := os.WriteFile(f.cfg, []byte("host_service:\n  state_dir: "+f.state+"\n  session_users: [bothy-s-alex, bothy-s-sam]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(f.state, 0o700); err != nil {
		t.Fatal(err)
	}
	old := sys
	t.Cleanup(func() { sys = old })
	sys = &app{
		goos: "linux", getenv: func(string) string { return "" }, isAdmin: func() bool { return false }, callerID: func() string { return "1000" },
		executable: func() (string, error) { return "/tmp/download/bothy-host", nil }, stdin: strings.NewReader(""), interactive: func() bool { return false },
		runner: &hostostest.Runner{}, services: hostos.Systemd{Runner: &hostostest.Runner{}},
		platform: func(hostos.Runner) (hostos.Users, hostos.Security, hostos.Probe) {
			return f.w.Users(), f.w.Security(), f.w.As(serviceUser).Probe()
		},
		resolver: f.w.As(serviceUser), files: f.w.As(serviceUser),
		apply: func(_ context.Context, s []install.Step) error {
			f.applied = append(f.applied, s)
			return f.applyErr
		},
		dialAdmin: func(context.Context, string, string) (*localchan.Conn, error) { return nil, errors.New("not running") },
		asServiceUser: func(_ context.Context, uid, gid int, args []string, stdin []byte, stdout, _ io.Writer) (int, error) {
			f.child = append(f.child, args)
			f.childIn = stdin
			_, _ = io.WriteString(stdout, "child\n")
			return 0, nil
		},
		listenAdmin: localchan.ListenAdmin, random: strings.NewReader(strings.Repeat("r", 256)), protector: state.Plain{},
		osVersion: func() string { return "Linux EXAMPLE" }, log: func(w io.Writer) *slog.Logger { return slog.New(slog.NewTextHandler(w, nil)) },
	}
	return f
}

func runCmd(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestCommandsRegistered(t *testing.T) {
	var names []string
	for _, c := range commands {
		names = append(names, c.name)
	}
	slices.Sort(names)
	if strings.Join(names, " ") != "doctor install pair pty-helper restart run session-user" {
		t.Errorf("commands %v", names)
	}
	_, out, _ := runCmd("--help")
	for _, n := range names {
		if !strings.Contains(out, "  "+n+" ") {
			t.Errorf("usage lacks %s", n)
		}
	}
}

func TestAdminRightsRequired(t *testing.T) {
	f := newFake(t)
	for _, args := range [][]string{
		{"install", "--config", f.cfg},
		{"session-user", "add", "bothy-s-robin", "--config", f.cfg},
		{"restart"},
		{"pair", "--server", "https://chat.example.com", "--code", "c", "--config", f.cfg},
		{"doctor", "--config", f.cfg},
	} {
		// Flags before positional arguments for session-user.
		if args[0] == "session-user" {
			args = []string{"session-user", "--config", f.cfg, "add", "bothy-s-robin"}
		}
		code, _, stderr := runCmd(args...)
		if code != 2 || stderr != "bothy-host "+args[0]+": administrator rights required\n" {
			t.Errorf("%v: %d %q", args, code, stderr)
		}
	}
	if len(f.applied) != 0 {
		t.Error("a plan ran without rights")
	}
}

func TestExitCodes(t *testing.T) {
	f := newFake(t)
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"install", "--bogus"}, 2},
		{[]string{"install", "--drive-root-confirm", "maybe"}, 2},
		{[]string{"session-user", "rename", "x"}, 2},
		{[]string{"restart", "x"}, 2},
		{[]string{"pair"}, 2},
		{[]string{"doctor", "x"}, 2},
		{[]string{"doctor", "--config", filepath.Join(f.state, "missing.yaml")}, 2},
		{[]string{"run", "x"}, 2},
		{[]string{"pty-helper"}, 2},
	} {
		if code, _, _ := runCmd(tc.args...); code != tc.code {
			t.Errorf("%v: %d, want %d", tc.args, code, tc.code)
		}
	}
	// An invalid configuration makes run exit 78 with config_invalid and the key.
	bad := filepath.Join(t.TempDir(), "bad.yaml")
	_ = os.WriteFile(bad, []byte("host_service: {colour: red}\n"), 0o600)
	code, _, stderr := runCmd("run", "--config", bad)
	if code != 78 || !strings.Contains(stderr, "config_invalid") || !strings.Contains(stderr, "host_service.colour") || strings.Contains(stderr, "red") {
		t.Errorf("run with a bad configuration: %d %q", code, stderr)
	}
	if code, _, _ := runCmd("doctor", "--config", bad); code != 2 {
		t.Errorf("doctor with a bad configuration: %d", code)
	}
}

func TestDoctorDirect(t *testing.T) {
	f := newFake(t)
	sys.callerID = func() string { return "999" }
	code, out, _ := runCmd("doctor", "--config", f.cfg)
	if !strings.HasPrefix(out, "HD1 ok\n") || !strings.Contains(out, "declaration: unknown") || code != 1 {
		// HD10 fails: no helper runs in the fake (helper_unreachable), and the peer check is unproven.
		t.Errorf("%d\n%s", code, out)
	}
	if !strings.Contains(out, "HD10 FAIL helper_unreachable session_user=bothy-s-alex") {
		t.Errorf("output:\n%s", out)
	}
}

func TestPairChildAndDirect(t *testing.T) {
	f := newFake(t)
	sys.isAdmin = func() bool { return true }
	code, out, _ := runCmd("pair", "--server", "https://chat.example.com", "--code", "c", "--config", f.cfg, "--ca-file", f.cfg)
	if code != 0 || out != "child\n" || len(f.applied) != 1 || !slices.Contains(f.child[0], "-") || len(f.childIn) == 0 {
		t.Errorf("child: %d %q %v", code, out, f.child)
	}
	// The child reads the certificates from standard input.
	sys.stdin = strings.NewReader("")
	if code, _, stderr := runCmd("pair", "--server", "https://x.example.com", "--code", "c", "--ca-file", "-", "--config", f.cfg); code != 2 || !strings.Contains(stderr, "no CA certificates") {
		t.Errorf("empty stdin: %d %q", code, stderr)
	}
	if code, _, _ := runCmd("pair", "--server", "https://x.example.com", "--code", "c", "--ca-file", filepath.Join(f.state, "none")); code != 2 {
		t.Errorf("missing CA file: %d", code)
	}
	sys.isAdmin = func() bool { return false }
	sys.callerID = func() string { return "999" }
	code, _, stderr := runCmd("pair", "--server", "http://chat.example.com", "--code", "c", "--config", f.cfg)
	if code != 1 || !strings.Contains(stderr, "pair: the server URL must use https") || !strings.Contains(stderr, "working directories follow with the first report") {
		t.Errorf("direct: %d %q", code, stderr)
	}
}

func TestInstallCommand(t *testing.T) {
	f := newFake(t)
	sys.isAdmin = func() bool { return true }
	var atApply string
	sys.apply = func(_ context.Context, s []install.Step) error {
		b, _ := os.ReadFile(f.cfg)
		atApply = string(b)
		f.applied = append(f.applied, s)
		return f.applyErr
	}
	code, out, stderr := runCmd("install", "--config", f.cfg, "--session-user", "bothy-s-robin")
	if code != 0 || !strings.Contains(out, "install: done") || len(f.applied) != 1 {
		t.Fatalf("%d %q %q", code, out, stderr)
	}
	if !strings.Contains(atApply, "bothy-s-robin") {
		t.Error("the restart of the plan ran before the new name was in the configuration")
	}
	b, _ := os.ReadFile(f.cfg)
	if !strings.Contains(string(b), "bothy-s-robin") {
		t.Error("--session-user not added to the configuration")
	}
	if code, _, _ := runCmd("install", "--drive-root", "C:"); code != 2 {
		t.Error("--drive-root on Linux")
	}
	f.applyErr = errors.New("boom")
	if code, _, stderr := runCmd("install", "--config", f.cfg); code != 1 || !strings.Contains(stderr, "boom") {
		t.Errorf("failure %d %q", code, stderr)
	}
	f.applyErr = nil
	// A fresh host without a configuration file: the defaults and a written file.
	fresh := filepath.Join(t.TempDir(), "new.yaml")
	if code, _, stderr := runCmd("install", "--config", fresh, "--session-user", "bothy-s-alex"); code != 0 {
		t.Errorf("fresh %d %q", code, stderr)
	}
	// Windows: the drive root prompt with the flag, the terminal and without a terminal.
	sys.goos = "windows"
	sys.getenv = func(k string) string {
		return map[string]string{"SystemDrive": "C:", "ProgramFiles": `C:\Program Files`, "ProgramData": `C:\ProgramData`}[k]
	}
	f.applied = nil
	if code, _, _ := runCmd("install", "--drive-root", "d:", "--drive-root-confirm", "yes"); code != 0 || len(f.applied) != 1 || len(f.applied[0]) != 1 {
		t.Errorf("drive root yes: %d %v", code, f.applied)
	}
	f.applied = nil
	sys.interactive = func() bool { return true }
	sys.stdin = strings.NewReader("y\n")
	if code, out, _ := runCmd("install", "--drive-root", "d:"); code != 0 || len(f.applied[0]) != 1 || !strings.Contains(out, "[y/N]") {
		t.Errorf("prompt yes: %d %v", code, f.applied)
	}
	f.applied = nil
	sys.interactive = func() bool { return false }
	if code, out, _ := runCmd("install", "--drive-root", "d:"); code != 0 || len(f.applied[0]) != 0 || !strings.Contains(out, "no terminal") {
		t.Errorf("no terminal: %d %v", code, f.applied)
	}
	f.applied = nil
	if code, _, stderr := runCmd("install", "--config", f.cfg, "--drive-root-confirm", "no"); code != 0 || len(f.applied) != 2 {
		t.Errorf("windows install %d %q %d", code, stderr, len(f.applied))
	}
}

func TestSessionUserCommand(t *testing.T) {
	f := newFake(t)
	sys.isAdmin = func() bool { return true }
	st := state.Open(f.state, state.Plain{})
	if err := st.WriteBindings(map[string]string{"bothy-s-old": "p"}); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runCmd("session-user", "--config", f.cfg, "add", "bothy-s-old"); code != 1 || !strings.Contains(stderr, "is bound to a person on this host") {
		t.Errorf("bound: %d %q", code, stderr)
	}
	if code, _, _ := runCmd("session-user", "--config", f.cfg, "add", "bothy-s-sam"); code != 2 {
		t.Error("configured name")
	}
	if code, out, _ := runCmd("session-user", "--config", f.cfg, "add", "bothy-s-robin"); code != 0 || !strings.Contains(out, "created") {
		t.Errorf("add %d", code)
	}
	if code, out, _ := runCmd("session-user", "--config", f.cfg, "remove", "bothy-s-sam"); code != 0 || !strings.Contains(out, "delete the account bothy-s-sam") {
		t.Errorf("remove %d %q", code, out)
	}
	if code, _, _ := runCmd("session-user", "--config", f.cfg, "remove", "bothy-s-ghost"); code != 2 {
		t.Error("remove unknown")
	}
	if code, _, _ := runCmd("session-user", "--config", filepath.Join(f.state, "none.yaml"), "add", "x"); code != 2 {
		t.Error("missing configuration")
	}
	if code, out, _ := runCmd("restart"); code != 0 || out != "restart: done\n" {
		t.Errorf("restart %d", code)
	}
	f.applyErr = errors.New("x")
	if code, _, _ := runCmd("restart"); code != 1 {
		t.Error("restart failure")
	}
	sys.goos = "plan9"
	if code, _, _ := runCmd("restart"); code != 2 {
		t.Error("restart plan9")
	}
}

func TestRunCommand(t *testing.T) {
	f := newFake(t)
	d, err := os.MkdirTemp("", "rn")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(d) }()
	cfg := filepath.Join(d, "c.yaml")
	_ = os.WriteFile(cfg, []byte("host_service: {state_dir: "+d+", session_users: []}\n"), 0o600)
	code, _, stderr := runCmd("run", "--config", cfg)
	if code != 78 || !strings.Contains(stderr, "session_users_empty") {
		t.Errorf("refused start: %d %q", code, stderr)
	}
	sys.listenAdmin = func(string) (net.Listener, error) { return nil, errors.New("x") }
	if code, _, _ := runCmd("run", "--config", f.cfg); code != 1 {
		t.Errorf("no admin endpoint: %d", code)
	}
}

func TestPtyHelperCommand(t *testing.T) {
	newFake(t)
	if code, _, stderr := runCmd("pty-helper", "--session-user", "bothy-s-alex"); code != 2 || !strings.Contains(stderr, "must run as its session user") {
		t.Errorf("%d %q", code, stderr)
	}
	sys.callerID = func() string { return "1001" }
	if code, _, stderr := runCmd("pty-helper", "--session-user", "bothy-s-alex"); code != 1 || !strings.Contains(stderr, "no channel endpoint") {
		t.Errorf("%d %q", code, stderr)
	}
}
