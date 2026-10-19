// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

var linux = Platform{GOOS: "linux"}

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadDefaultsPerPlatform(t *testing.T) {
	for _, tc := range []struct {
		p           Platform
		state, path string
	}{
		{linux, "/var/lib/bothy-host", "/etc/bothy-host/config.yaml"},
		{Platform{GOOS: "darwin"}, "/var/lib/bothy-host", "/Library/Application Support/bothy-host/config.yaml"},
		{Platform{GOOS: "windows", ProgramData: `C:\ProgramData`, ProgramFiles: `C:\Program Files`}, `C:\ProgramData\bothy-host`, `C:\Program Files\bothy-host\config.yaml`},
	} {
		c, err := Load(write(t, "host_service: {}\n"), tc.p)
		if err != nil {
			t.Fatal(err)
		}
		if c.StateDir != tc.state || DefaultPath(tc.p) != tc.path {
			t.Errorf("%s: state %q path %q", tc.p.GOOS, c.StateDir, DefaultPath(tc.p))
		}
		if c.CredentialCheckInterval != time.Minute || c.EntryScanTimeout != 10*time.Minute || c.AuthStatusTimeout != 30*time.Second ||
			c.ScrollbackBytes != 1048576 || len(c.Profiles) != 1 || c.Profiles[0].Kind != KindShell || c.RequireLocalConfirm {
			t.Errorf("%s: defaults %+v", tc.p.GOOS, c)
		}
	}
	if c, err := Parse([]byte(""), linux); err != nil || c.StateDir == "" {
		t.Errorf("empty document: %v", err)
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml"), linux); err == nil || errors.Is(err, ErrInvalid) {
		t.Errorf("missing file: %v", err)
	}
}

func keyOf(t *testing.T, err error) string {
	t.Helper()
	var ke *KeyError
	if !errors.Is(err, ErrInvalid) || !errors.As(err, &ke) {
		t.Fatalf("error %v is no KeyError", err)
	}
	return ke.Key
}

func TestLoadRejectsUnknownAndDuplicateKeys(t *testing.T) {
	for body, key := range map[string]string{
		"other: 1\n":                    "other",
		"host_service: {colour: red}\n": "host_service.colour",
		"host_service:\n  claude_bin: /a\n  claude_bin: /b\n": "host_service.claude_bin",
		"host_service: []\n":                               "host_service",
		"- a\n":                                            "host_service",
		"host_service: {session_users: alex}\n":            "host_service.session_users",
		"host_service: {session_users: [1]}\n":             "host_service.session_users",
		"host_service: {scrollback_bytes: many}\n":         "host_service.scrollback_bytes",
		"host_service: {scrollback_bytes: -1}\n":           "host_service.scrollback_bytes",
		"host_service: {require_local_confirm: yes}\n":     "host_service.require_local_confirm",
		"host_service: {state_dir: [a]}\n":                 "host_service.state_dir",
		"host_service: {1: a}\n":                           "host_service.<key>",
		"host_service: {claude_workdirs: [a]}\n":           "host_service.claude_workdirs",
		"host_service: {claude_workdirs: {a: b}}\n":        "host_service.claude_workdirs.a",
		"host_service: {claude_workdirs: {a: [b]}}\n":      "host_service.claude_workdirs.a",
		"host_service: {claude_workdirs: {a: [{x: 1}]}}\n": "host_service.claude_workdirs.a.x",
	} {
		_, err := Parse([]byte(body), linux)
		if got := keyOf(t, err); got != key {
			t.Errorf("%q: key %q, want %q", body, got, key)
		}
		if strings.Contains(err.Error(), "red") {
			t.Errorf("error quotes a value: %v", err)
		}
	}
	if _, err := Parse([]byte("host_service: [\n"), linux); !errors.Is(err, ErrInvalid) {
		t.Errorf("broken YAML: %v", err)
	}
}

func TestProfilesForms(t *testing.T) {
	c, err := Parse([]byte(`host_service:
  profiles:
    - shell
    - claude
    - {name: htop, display_name: Top, kind: command, program: /usr/bin/htop, args: [-d, "10"]}
    - {name: rel, display_name: Relative, kind: command, program: bin/tool}
`), linux)
	if err != nil {
		t.Fatal(err)
	}
	want := []Profile{
		{Name: "shell", DisplayName: "Shell", Kind: KindShell},
		{Name: "claude", DisplayName: "Claude Code", Kind: KindClaude},
		{Name: "htop", DisplayName: "Top", Kind: KindCommand, Program: "/usr/bin/htop", Args: []string{"-d", "10"}},
		{Name: "rel", DisplayName: "Relative", Kind: KindCommand, Program: "bin/tool"},
	}
	if !reflect.DeepEqual(c.Profiles, want) {
		t.Errorf("profiles = %+v", c.Profiles)
	}
	for body, key := range map[string]string{
		"[fish]": "host_service.profiles",
		"[[a]]":  "host_service.profiles",
		"a":      "host_service.profiles",
		"[{name: Bad, display_name: X, kind: command, program: /x}]":        "host_service.profiles.name",
		"[{name: shell, display_name: X, kind: command, program: /x}]":      "host_service.profiles.name",
		"[{name: ok, kind: command, program: /x}]":                          "host_service.profiles.display_name",
		"[{name: ok, display_name: X, kind: shell, program: /x}]":           "host_service.profiles.kind",
		"[{name: ok, display_name: X, kind: command}]":                      "host_service.profiles.program",
		"[{name: ok, display_name: X, kind: command, program: /x, env: a}]": "host_service.profiles.env",
	} {
		_, err := Parse([]byte("host_service: {profiles: "+body+"}\n"), linux)
		if got := keyOf(t, err); got != key {
			t.Errorf("%s: key %q, want %q", body, got, key)
		}
	}
}

func TestClaudeWorkdirsForm(t *testing.T) {
	c, err := Parse([]byte(`host_service:
  session_users: [bothy-s-alex, bothy-s-sam]
  claude_workdirs:
    bothy-s-alex:
      - {display_name: Project A, path: /home/bothy-s-alex/a}
      - {display_name: "", path: /home/bothy-s-alex/b}
    bothy-s-sam: []
`), linux)
	if err != nil {
		t.Fatal(err)
	}
	want := []WorkdirList{
		{SessionUser: "bothy-s-alex", Dirs: []Workdir{{"Project A", "/home/bothy-s-alex/a"}, {"", "/home/bothy-s-alex/b"}}},
		{SessionUser: "bothy-s-sam", Dirs: []Workdir{}},
	}
	if !reflect.DeepEqual(c.ClaudeWorkdirs, want) {
		t.Errorf("workdirs = %+v", c.ClaudeWorkdirs)
	}
	if len(c.Workdirs("bothy-s-alex")) != 2 || c.Workdirs("bothy-s-robin") != nil {
		t.Error("Workdirs lookup")
	}
}

func TestDurationKeys(t *testing.T) {
	c, err := Parse([]byte(`host_service:
  credential_check_interval: 90s
  entry_scan_timeout: 1h
  auth_status_timeout: 1500ms
  local_confirm_timeout: 2m
`), linux)
	if err != nil {
		t.Fatal(err)
	}
	if c.CredentialCheckInterval != 90*time.Second || c.EntryScanTimeout != time.Hour || c.AuthStatusTimeout != 1500*time.Millisecond || c.LocalConfirmTimeout != 2*time.Minute {
		t.Errorf("durations %+v", c)
	}
	for _, bad := range []string{"30", "1.5s", "01s", "-1s", "1w", "[1s]", "9999999999999999999999999d", "9999999999999d"} {
		_, err := Parse([]byte("host_service: {auth_status_timeout: "+bad+"}\n"), linux)
		if keyOf(t, err) != "host_service.auth_status_timeout" {
			t.Errorf("%s accepted", bad)
		}
	}
	if d, err := ParseDuration("2d"); err != nil || d != 48*time.Hour {
		t.Errorf("2d = %v %v", d, err)
	}
}

func TestAddRemoveSessionUserKeepsComments(t *testing.T) {
	body := `# Host service of the lab machine (EXAMPLE).
host_service:
  # people with a terminal
  session_users: [bothy-s-alex, bothy-s-sam]
  claude_bin: /opt/claude/bin/claude # pinned
  claude_workdirs:
    bothy-s-alex:
      - {display_name: Project A, path: /home/bothy-s-alex/a}
    bothy-s-sam:
      - {display_name: Project S, path: /home/bothy-s-sam/s}
`
	p := write(t, body)
	before, err := Load(p, linux)
	if err != nil {
		t.Fatal(err)
	}
	if err := AddSessionUser(p, "bothy-s-robin"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveSessionUser(p, "bothy-s-alex"); err != nil {
		t.Fatal(err)
	}
	after, err := Load(p, linux)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after.SessionUsers, []string{"bothy-s-sam", "bothy-s-robin"}) {
		t.Errorf("session users %v", after.SessionUsers)
	}
	if len(after.ClaudeWorkdirs) != 1 || after.ClaudeWorkdirs[0].SessionUser != "bothy-s-sam" {
		t.Errorf("workdirs %+v", after.ClaudeWorkdirs)
	}
	before.SessionUsers, after.SessionUsers = nil, nil
	before.ClaudeWorkdirs, after.ClaudeWorkdirs = nil, nil
	if !reflect.DeepEqual(before, after) {
		t.Errorf("other keys changed:\n%+v\n%+v", before, after)
	}
	b, _ := os.ReadFile(p) //nolint:gosec // the file of this test
	for _, c := range []string{"# Host service of the lab machine (EXAMPLE).", "# people with a terminal", "# pinned"} {
		if !strings.Contains(string(b), c) {
			t.Errorf("comment %q lost:\n%s", c, b)
		}
	}
	fi, _ := os.Stat(p)
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", fi.Mode().Perm())
	}
	// A file without the keys gains them.
	q := write(t, "")
	if err := AddSessionUser(q, "bothy-s-alex"); err != nil {
		t.Fatal(err)
	}
	if c, err := Load(q, linux); err != nil || !reflect.DeepEqual(c.SessionUsers, []string{"bothy-s-alex"}) {
		t.Errorf("new file: %v %+v", err, c)
	}
	if err := AddSessionUser(write(t, "- a\n"), "x"); !errors.Is(err, ErrInvalid) {
		t.Errorf("list file: %v", err)
	}
	if err := AddSessionUser(write(t, "host_service: [a]\n"), "x"); !errors.Is(err, ErrInvalid) {
		t.Errorf("list host_service: %v", err)
	}
	if err := RemoveSessionUser(filepath.Join(t.TempDir(), "none"), "x"); err == nil {
		t.Error("missing file accepted")
	}
}

// TestReconnectBackoffKeys (M2-43, 24 section 17): the defaults 1s and 5m, both keys read, zero
// and a cap below the base refused with the key named.
func TestReconnectBackoffKeys(t *testing.T) {
	c, err := Parse([]byte("host_service: {}\n"), linux)
	if err != nil || c.ReconnectBackoffBase != time.Second || c.ReconnectBackoffCap != 5*time.Minute {
		t.Fatalf("defaults %v %v %v", c, err, err)
	}
	c, err = Parse([]byte("host_service:\n  reconnect_backoff_base: 200ms\n  reconnect_backoff_cap: 30s\n"), linux)
	if err != nil || c.ReconnectBackoffBase != 200*time.Millisecond || c.ReconnectBackoffCap != 30*time.Second {
		t.Fatalf("set %v %v", c, err)
	}
	for doc, key := range map[string]string{
		"host_service:\n  reconnect_backoff_base: 0s\n":                              "host_service.reconnect_backoff_base",
		"host_service:\n  reconnect_backoff_base: 2m\n  reconnect_backoff_cap: 1m\n": "host_service.reconnect_backoff_cap",
	} {
		_, err := Parse([]byte(doc), linux)
		var ke *KeyError
		if !errors.As(err, &ke) || ke.Key != key {
			t.Errorf("%q: %v, want a KeyError of %s", doc, err, key)
		}
	}
}
