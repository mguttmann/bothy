// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// tricky holds spaces, both quote kinds, a backslash and shell syntax that must stay literal.
const tricky = `/home/bothy-s-alex/my "cfg" it's \ $(echo pwned) ` + "`id`" + ` $HOME; x`

func TestPersonalStartCommand(t *testing.T) {
	tests := []struct {
		kind string
		set  []string
		del  []string
	}{
		{"sh",
			[]string{"-c", `export CLAUDE_CONFIG_DIR='/home/bothy-s-alex/my "cfg" it'\''s \ $(echo pwned) ` + "`id`" + ` $HOME; x'; exec claude`},
			[]string{"-c", "unset CLAUDE_CONFIG_DIR; exec claude"}},
		{"zsh",
			[]string{"-c", `export CLAUDE_CONFIG_DIR='/home/bothy-s-alex/my "cfg" it'\''s \ $(echo pwned) ` + "`id`" + ` $HOME; x'; exec claude`},
			[]string{"-c", "unset CLAUDE_CONFIG_DIR; exec claude"}},
		{"bash", nil, []string{"-c", "unset CLAUDE_CONFIG_DIR; exec claude"}},
		{"dash", nil, []string{"-c", "unset CLAUDE_CONFIG_DIR; exec claude"}},
		{"ksh", nil, []string{"-c", "unset CLAUDE_CONFIG_DIR; exec claude"}},
		{"fish",
			[]string{"-c", `set -gx CLAUDE_CONFIG_DIR '/home/bothy-s-alex/my "cfg" it\'s \\ $(echo pwned) ` + "`id`" + ` $HOME; x'; exec claude`},
			[]string{"-c", "set -e CLAUDE_CONFIG_DIR; exec claude"}},
		{"tcsh",
			[]string{"-c", `setenv CLAUDE_CONFIG_DIR '/home/bothy-s-alex/my "cfg" it'\''s \ $(echo pwned) ` + "`id`" + ` $HOME; x'; exec claude`},
			[]string{"-c", "unsetenv CLAUDE_CONFIG_DIR; exec claude"}},
		{"csh", nil, []string{"-c", "unsetenv CLAUDE_CONFIG_DIR; exec claude"}},
		{"powershell",
			[]string{"-NoLogo", "-Command", `$env:CLAUDE_CONFIG_DIR = '/home/bothy-s-alex/my "cfg" it''s \ $(echo pwned) ` + "`id`" + ` $HOME; x'; & claude`},
			[]string{"-NoLogo", "-Command", `Remove-Item Env:\CLAUDE_CONFIG_DIR -ErrorAction SilentlyContinue; & claude`}},
	}
	for _, tc := range tests {
		if tc.set != nil {
			if got := PersonalCommand(tc.kind, tricky, true); !slices.Equal(got, tc.set) {
				t.Errorf("%s set:\n got %q\nwant %q", tc.kind, got, tc.set)
			}
		}
		if got := PersonalCommand(tc.kind, tricky, false); !slices.Equal(got, tc.del) {
			t.Errorf("%s unset:\n got %q\nwant %q", tc.kind, got, tc.del)
		}
	}
	// PowerShell also reads the typographic single quotes as quotes; each is doubled.
	if got := PersonalCommand("powershell", "C:\\a\u2019b\u2018c", true)[2]; got != "$env:CLAUDE_CONFIG_DIR = 'C:\\a\u2019\u2019b\u2018\u2018c'; & claude" {
		t.Errorf("powershell typographic quotes %q", got)
	}
	// An unknown shell kind gets the POSIX form, which every listed sh-like shell reads.
	if got := PersonalCommand("mksh", "/x", true); !slices.Equal(got, []string{"-c", "export CLAUDE_CONFIG_DIR='/x'; exec claude"}) {
		t.Errorf("unknown kind %q", got)
	}

	// The real shells prove the quoting: exec is replaced by printing the variable.
	for _, sh := range []string{"/bin/sh", "/bin/zsh", "/bin/bash"} {
		if _, err := os.Stat(sh); err != nil {
			if sh == "/bin/sh" {
				t.Fatal("no /bin/sh")
			}
			continue
		}
		kind := filepath.Base(sh)
		set := strings.Replace(PersonalCommand(kind, tricky, true)[1], "exec claude", `printf '%s' "$CLAUDE_CONFIG_DIR"`, 1)
		if out := runShell(t, sh, set); out != tricky {
			t.Errorf("%s set printed %q, want %q", sh, out, tricky)
		}
		del := strings.Replace(PersonalCommand(kind, tricky, false)[1], "exec claude", `printf '%s' "${CLAUDE_CONFIG_DIR-unset}"`, 1)
		if out := runShell(t, sh, del); out != "unset" {
			t.Errorf("%s unset printed %q", sh, out)
		}
	}
}

func runShell(t *testing.T, sh, script string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), sh, "-c", script) //nolint:gosec // a fixed system shell of the test
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + t.TempDir(), "CLAUDE_CONFIG_DIR=/stale"}
	cmd.Dir = t.TempDir()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s: %v", sh, err)
	}
	return string(out)
}

// osSecurity is the Security of the build host from os.Lstat (mode and type only).
type osSecurity struct{}

func (osSecurity) Lstat(p string) (hostos.FileSec, error) {
	fi, err := os.Lstat(p)
	if err != nil {
		return hostos.FileSec{}, err
	}
	return hostos.FileSec{IsDir: fi.IsDir(), IsLink: fi.Mode()&fs.ModeSymlink != 0, Mode: fi.Mode()}, nil
}

func TestPersonalConfigDir(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{home, outside, filepath.Join(home, "repo"), filepath.Join(home, "cfg"), filepath.Join(home, "sub", "deep")} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(home, "out")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(base, "nowhere"), filepath.Join(home, "dangling")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, "cfg"), filepath.Join(home, "inner")); err != nil {
		t.Fatal(err)
	}
	env := func(dir string, withDefault bool) Env {
		_ = os.RemoveAll(filepath.Join(home, ".claude"))
		if withDefault {
			if err := os.Mkdir(filepath.Join(home, ".claude"), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		return Env{GOOS: "linux", Self: hostos.Account{Name: "bothy-s-alex", Home: home, Shell: "/bin/sh"}, Environ: []string{"PATH=/usr/bin:/bin"},
			Resolver: hostos.OSResolver{}, Security: osSecurity{}, Shells: []string{"/bin/sh"}, LoginEnv: probe(dir)}
	}
	repo := filepath.Join(home, "repo")
	pass := []struct {
		name, dir   string
		withDefault bool
	}{
		{"unset with the default folder", "", true},
		{"inside", filepath.Join(home, "cfg"), false},
		{"deeper inside", filepath.Join(home, "sub", "deep"), false},
		{"the home itself", home, false},
		{"a link inside the home", filepath.Join(home, "inner"), false},
	}
	for _, tc := range pass {
		s, err := ClaudePersonal(context.Background(), env(tc.dir, tc.withDefault), repo, time.Second)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if !slices.Equal(s.Args, PersonalCommand("sh", tc.dir, tc.dir != "")) || s.Path != "/bin/sh" || s.Argv0 != "-sh" || s.Dir != repo {
			t.Errorf("%s: spec %+v", tc.name, s)
		}
	}
	refuse := []struct{ name, dir string }{
		{"unset without the default folder", ""},
		{"outside", outside},
		{"a link leading outside", filepath.Join(home, "out")},
		{"below a link leading outside", filepath.Join(home, "out", "x")},
		{"a dangling link", filepath.Join(home, "dangling")},
		{"a missing folder", filepath.Join(home, "missing")},
		{"dot dot out of the home", filepath.Join(home, "..", "outside")},
		{"a sibling with the home as prefix", home + "x"},
		{"relative", "cfg"},
	}
	if err := os.MkdirAll(home+"x", 0o700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range refuse {
		if _, err := ClaudePersonal(context.Background(), env(tc.dir, false), repo, time.Second); detailOf(err) != DetailSharedLocation {
			t.Errorf("%s: %v", tc.name, err)
		}
	}

	// The probe is bounded by host_service.auth_status_timeout; a timeout or a failure is pty_failed.
	e := env("", true)
	e.LoginEnv = func(context.Context, time.Duration) (bool, string, error) {
		time.Sleep(2 * time.Second) // ignores its context on purpose
		return false, "", nil
	}
	start := time.Now()
	if _, err := ClaudePersonal(context.Background(), e, repo, 50*time.Millisecond); detailOf(err) != DetailPTYFailed {
		t.Errorf("slow probe: %v", err)
	}
	if time.Since(start) > time.Second {
		t.Error("the probe timeout was not applied")
	}
	gotTimeout := make(chan time.Duration, 1)
	e.LoginEnv = func(ctx context.Context, timeout time.Duration) (bool, string, error) {
		gotTimeout <- timeout
		<-ctx.Done()
		return false, "", ctx.Err()
	}
	if _, err := ClaudePersonal(context.Background(), e, repo, 30*time.Millisecond); detailOf(err) != DetailPTYFailed {
		t.Errorf("probe honouring its context: %v", err)
	}
	if got := <-gotTimeout; got != 30*time.Millisecond {
		t.Errorf("probe timeout %v", got)
	}
	e.LoginEnv = func(context.Context, time.Duration) (bool, string, error) {
		return false, "", errors.New("login shell failed")
	}
	if _, err := ClaudePersonal(context.Background(), e, repo, time.Second); detailOf(err) != DetailPTYFailed {
		t.Errorf("failed probe: %v", err)
	}
	e.LoginEnv = nil
	if _, err := ClaudePersonal(context.Background(), e, repo, time.Second); detailOf(err) != DetailPTYFailed {
		t.Errorf("no probe: %v", err)
	}
	// A login shell that is not listed refuses even with a good folder.
	e = env("", true)
	e.Shells = nil
	if _, err := ClaudePersonal(context.Background(), e, repo, time.Second); detailOf(err) != DetailPTYFailed {
		t.Errorf("unlisted shell: %v", err)
	}
}
