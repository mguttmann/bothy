// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// write creates the file rel below dir with content.
func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestCopyReplacesTheOutputWithTheTemplateFolders: every template folder is copied byte for byte,
// files at the root of the source (the README) stay out, and a stale file of an earlier run goes.
func TestCopyReplacesTheOutputWithTheTemplateFolders(t *testing.T) {
	t.Parallel()
	src, out := t.TempDir(), filepath.Join(t.TempDir(), "agents")
	write(t, src, "README.md", "# Templates\n")
	write(t, src, "notes/template.yaml", "name: Notes\n")
	write(t, src, "notes/skills/short/SKILL.md", "---\nname: short\n---\n")
	write(t, out, "gone/template.yaml", "old\n")
	var stderr bytes.Buffer
	if code := run([]string{"-src", src, "-out", out}, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	got, err := os.ReadFile(filepath.Join(out, "notes", "skills", "short", "SKILL.md")) //nolint:gosec // a file the test wrote
	if err != nil || string(got) != "---\nname: short\n---\n" {
		t.Fatalf("copy: %q, %v", got, err)
	}
	for _, rel := range []string{"README.md", "gone"} {
		if _, err := os.Stat(filepath.Join(out, rel)); !os.IsNotExist(err) {
			t.Errorf("%s still exists: %v", rel, err)
		}
	}
}

// TestCopyRefusesALinkAndBadUsage: a symbolic link is no file of the layout (26 7.1); the gate
// reports it, and the copy refuses it rather than following it.
func TestCopyRefusesALinkAndBadUsage(t *testing.T) {
	t.Parallel()
	src, out := t.TempDir(), filepath.Join(t.TempDir(), "agents")
	write(t, src, "notes/CLAUDE.md", "x\n")
	if err := os.Symlink(filepath.Join(src, "notes", "CLAUDE.md"), filepath.Join(src, "notes", "link.md")); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if code := run([]string{"-src", src, "-out", out}, &stderr); code != 1 || !strings.Contains(stderr.String(), "link.md") {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if code := run([]string{"-src", src}, &stderr); code != 2 {
		t.Fatalf("usage: exit %d", code)
	}
}
