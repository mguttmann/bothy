// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// goldenPath is the expected output for testdata/tokens.json.
const goldenPath = "testdata/golden.go.txt"

func TestKeys_Fixture(t *testing.T) {
	t.Parallel()
	k, err := ReadKeys("testdata/tokens.json")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"amber", "moss"}; !slices.Equal(k.CategoryColors, want) {
		t.Errorf("category colours %v, want %v", k.CategoryColors, want)
	}
	if want := []string{"code", "terminal-window"}; !slices.Equal(k.SpaceIcons, want) {
		t.Errorf("space icons %v, want %v", k.SpaceIcons, want)
	}
}

func TestRender_Golden(t *testing.T) {
	t.Parallel()
	k, err := ReadKeys("testdata/tokens.json")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Render(k)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output differs from %s:\n%s", goldenPath, got)
	}
}

func TestKeys_ThemesDiffer(t *testing.T) {
	t.Parallel()
	_, err := ReadKeys("testdata/tokens-mismatch.json")
	if err == nil {
		t.Fatal("themes with different category colours were accepted")
	}
	for _, want := range []string{"loch", "moss"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %s", err, want)
		}
	}
}

func TestKeys_Invalid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"not-json":      `{`,
		"no-light":      `{"contexts":{"theme":{"dark":{}}},"icons":{"space-icons":["code"]}}`,
		"no-icons":      `{"contexts":{"theme":{"light":{"color.category.moss.fg":{}},"dark":{"color.category.moss.fg":{}}}}}`,
		"no-categories": `{"contexts":{"theme":{"light":{},"dark":{}}},"icons":{"space-icons":["code"]}}`,
		"bad-key":       `{"contexts":{"theme":{"light":{"color.category.Moss.fg":{}},"dark":{"color.category.Moss.fg":{}}}},"icons":{"space-icons":["code"]}}`,
		"bad-icon":      `{"contexts":{"theme":{"light":{"color.category.moss.fg":{}},"dark":{"color.category.moss.fg":{}}}},"icons":{"space-icons":["Code Icon"]}}`,
	} {
		path := filepath.Join(dir, name+".json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadKeys(path); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestRun_WritesFileAndRefusesUsage(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "designkeys_gen.go")
	var stderr bytes.Buffer
	if code := run([]string{"-tokens", "testdata/tokens.json", "-out", out}, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	got, err := os.ReadFile(out) //nolint:gosec // G304: a file in the test's own temporary directory.
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Error("run wrote something else than the golden file")
	}
	if code := run([]string{"-tokens", "testdata/tokens.json"}, &stderr); code != 2 {
		t.Errorf("missing -out: exit %d, want 2", code)
	}
	if code := run([]string{"-tokens", "testdata/tokens-mismatch.json", "-out", out}, &stderr); code != 1 {
		t.Errorf("mismatch: exit %d, want 1", code)
	}
}

// TestCommittedFileIsCurrent fails when internal/kernel/designkeys/designkeys_gen.go differs from
// a fresh generation from design/build/tokens.resolved.json; make codegen-check holds the same
// on every run of go generate, this test makes the proof part of go test.
func TestCommittedFileIsCurrent(t *testing.T) {
	t.Parallel()
	k, err := ReadKeys("../../../../design/build/tokens.resolved.json")
	if err != nil {
		t.Fatal(err)
	}
	want, err := Render(k)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("../../internal/kernel/designkeys/designkeys_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Error("designkeys_gen.go is not current; run go generate ./internal/kernel/designkeys")
	}
}
