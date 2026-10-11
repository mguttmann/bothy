// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runArchgen(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRunWritesConfig(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "go-arch-lint.yml")
	code, _, stderr := runArchgen("-register", goldenRegister, "-out", out)
	if code != 0 {
		t.Fatalf("exit code %d, stderr:\n%s", code, stderr)
	}
	got, err := os.ReadFile(filepath.Clean(out))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(goldenConfig)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("written file differs from %s", goldenConfig)
	}
}

func TestRunUsageErrorExits2(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"-unknown"}, {"extra"}, {"-register"}} {
		code, stdout, stderr := runArchgen(args...)
		if code != 2 {
			t.Errorf("run(%q) = %d, want 2", args, code)
		}
		if stdout != "" || !strings.Contains(stderr, "Usage: archgen") {
			t.Errorf("run(%q): want usage on stderr only, stdout %q, stderr %q", args, stdout, stderr)
		}
	}
}

func TestRunHelpExits0(t *testing.T) {
	t.Parallel()
	for _, arg := range []string{"-h", "-help", "--help"} {
		code, stdout, stderr := runArchgen(arg)
		if code != 0 {
			t.Errorf("run(%s) = %d, want 0", arg, code)
		}
		if !strings.Contains(stdout, "Usage: archgen") || !strings.Contains(stdout, "-register") || stderr != "" {
			t.Errorf("run(%s): want usage with flags on stdout, stdout %q, stderr %q", arg, stdout, stderr)
		}
	}
}

func TestRunMissingRegisterExits1(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	code, _, stderr := runArchgen("-register", filepath.Join(dir, "missing.yaml"), "-out", filepath.Join(dir, "out.yml"))
	if code != 1 {
		t.Fatalf("exit code %d, want 1", code)
	}
	if !strings.Contains(stderr, "missing.yaml") {
		t.Errorf("stderr does not name the register: %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "out.yml")); !os.IsNotExist(err) {
		t.Errorf("output file written despite the error")
	}
}

func TestRunInvalidRegisterExits1(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := filepath.Join(dir, "modules.yaml")
	text := edit(t, "    depends_on: [chat]", "    depends_on: [chat, search]", "tables: [agents]", "tables: [channels]")
	if err := os.WriteFile(reg, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runArchgen("-register", reg, "-out", filepath.Join(dir, "out.yml"))
	if code != 1 {
		t.Fatalf("exit code %d, want 1", code)
	}
	wantLines := []string{
		reg + `:24:14: modules.agents.tables[0]: table "channels" is already owned by module chat (modules.chat.tables[0])`,
		reg + `:29:24: modules.agents.depends_on[1]: unknown module "search"`,
	}
	for _, want := range wantLines {
		if !strings.Contains(stderr, want+"\n") {
			t.Errorf("stderr lacks line %q:\n%s", want, stderr)
		}
	}
}

func TestRunUnwritableOutputExits1(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "no-such-dir", "out.yml")
	code, _, stderr := runArchgen("-register", goldenRegister, "-out", out)
	if code != 1 || !strings.Contains(stderr, "no-such-dir") {
		t.Errorf("exit code %d, stderr %q; want 1 and the output path", code, stderr)
	}
}

// moduleDir writes the register text into a fresh module directory with the given directories
// and returns the register path and the output path next to it.
func moduleDir(t *testing.T, text string, dirs ...string) (reg, out string) {
	t.Helper()
	root := t.TempDir()
	for _, d := range append([]string{"architecture"}, dirs...) {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	reg = filepath.Join(root, "architecture", "modules.yaml")
	if err := os.WriteFile(reg, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return reg, filepath.Join(root, ".go-arch-lint.yml")
}

// TestRunRejectsExcludePrefixOfOtherDirectories checks exclude the way go-arch-lint applies it:
// every path that starts with an entry as a string is excluded, so next to tools/ a directory
// toolsx/ would leave the gate without a word.
func TestRunRejectsExcludePrefixOfOtherDirectories(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, exclude string
		dirs          []string
		want          string
	}{
		{"in next to internal", "exclude: [tools, in]", []string{"in", "internal/chat", "tools"},
			`exclude[1]: "in" takes product code under internal/ out of the architecture gate`},
		{"tools next to toolsx", "exclude: [tools]", []string{"internal/chat", "tools/archgen", "toolsx/gen"},
			`exclude[0]: "tools" also takes toolsx/ out of the architecture gate`},
		{"nested entry next to a sibling", "exclude: [tools/gen]", []string{"internal", "tools/gen", "tools/generator"},
			`exclude[0]: "tools/gen" also takes tools/generator/ out of the architecture gate`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reg, out := moduleDir(t, edit(t, "exclude: [tools]", tt.exclude), tt.dirs...)
			code, _, stderr := runArchgen("-register", reg, "-out", out)
			if code != 1 || !strings.Contains(stderr, tt.want) {
				t.Fatalf("exit code %d, stderr:\n%s\nwant 1 and a line containing %q", code, stderr, tt.want)
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Errorf("output file written despite the problem")
			}
		})
	}
}

func TestRunAcceptsExcludeWithoutOtherMatches(t *testing.T) {
	t.Parallel()
	reg, out := moduleDir(t, validRegister, "internal/chat/domain", "tools/archgen/testdata", "test", "testkit")
	if code, _, stderr := runArchgen("-register", reg, "-out", out); code != 0 {
		t.Fatalf("exit code %d, stderr:\n%s", code, stderr)
	}
}

// TestWriteFileReplacesTheFile checks that writeFile never rewrites the old file in place: a hard
// link to it keeps the old content, so a run that stops halfway cannot leave a truncated config.
func TestWriteFileReplacesTheFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(target, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "old.yml")
	if err := os.Link(target, old); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(target, []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{target: "new\n", old: "old\n"} {
		got, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", filepath.Base(name), got, want)
		}
	}
	assertEntries(t, dir, "config.yml", "old.yml")
}

func TestWriteFileLeavesNoTemporaryFileOnError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "config.yml")
	if err := os.MkdirAll(filepath.Join(target, "occupied"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(target, []byte("new\n")); err == nil {
		t.Fatal("writeFile over a non-empty directory succeeded")
	}
	assertEntries(t, dir, "config.yml")
}

// assertEntries fails unless dir holds exactly the named entries.
func assertEntries(t *testing.T, dir string, want ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("%s holds %v, want %v", dir, got, want)
	}
}
