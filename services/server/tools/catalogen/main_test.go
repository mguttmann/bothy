// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_WritesBothFiles(t *testing.T) {
	dir := t.TempDir()
	ids, domain := filepath.Join(dir, "ids.go"), filepath.Join(dir, "domain.go")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-catalog", "internal/gen/testdata/valid.yaml", "-ids", ids, "-domain", domain}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	for path, golden := range map[string]string{ids: "valid.ids.golden", domain: "valid.domain.golden"} {
		got, err := os.ReadFile(path) //nolint:gosec // G304: a file this test wrote
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join("internal", "gen", "testdata", golden)) //nolint:gosec // G304: golden file
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from %s", path, golden)
		}
	}
}

func TestRun_InvalidCatalogueWritesNothing(t *testing.T) {
	dir := t.TempDir()
	ids := filepath.Join(dir, "ids.go")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-catalog", "internal/gen/testdata/invalid/owner-value.yaml", "-ids", ids, "-domain", filepath.Join(dir, "d.go")}, &stdout, &stderr)
	if code != exitFailure || !strings.Contains(stderr.String(), "owner") {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if _, err := os.Stat(ids); !os.IsNotExist(err) {
		t.Fatalf("output written despite the error: %v", err)
	}
}

func TestRun_Usage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-nope"}, &stdout, &stderr); code != exitUsage {
		t.Fatalf("unknown flag: exit %d", code)
	}
	if code := run([]string{"extra"}, &stdout, &stderr); code != exitUsage {
		t.Fatalf("argument: exit %d", code)
	}
	if code := run([]string{"-h"}, &stdout, &stderr); code != exitOK || !strings.Contains(stdout.String(), "-catalog") {
		t.Fatalf("-h: exit %d, stdout %q", code, stdout.String())
	}
	if code := run([]string{"-catalog", filepath.Join(t.TempDir(), "missing.yaml")}, &stdout, &stderr); code != exitFailure {
		t.Fatalf("missing file: exit %d", code)
	}
	if code := run([]string{"-catalog", "internal/gen/testdata/valid.yaml", "-ids", filepath.Join(t.TempDir(), "no", "dir", "x.go")}, &stdout, &stderr); code != exitFailure {
		t.Fatalf("unwritable output: exit %d", code)
	}
}

// TestRun_NoPartialWrite: when the second file cannot be written, the first is not changed.
func TestRun_NoPartialWrite(t *testing.T) {
	dir := t.TempDir()
	ids := filepath.Join(dir, "ids.go")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-catalog", "internal/gen/testdata/valid.yaml", "-ids", ids, "-domain", filepath.Join(dir, "missing", "d.go")}, &stdout, &stderr)
	if code != exitFailure {
		t.Fatalf("exit %d", code)
	}
	if _, err := os.Stat(ids); !os.IsNotExist(err) {
		t.Fatalf("the ids file was written although the domain file failed: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("left files behind: %v", entries)
	}
}

// TestRun_ReservedNames: a hand-written name of the target package that a generated constant
// would redeclare fails the run with entry and field.
func TestRun_ReservedNames(t *testing.T) {
	dir := t.TempDir()
	src := "// SPDX-License-Identifier: Apache-2.0\n\npackage authorize\n\n// OpHeartbeat is hand-written.\nfunc OpHeartbeat() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "own.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"-catalog", "internal/gen/testdata/valid.yaml", "-ids", filepath.Join(dir, "ids.go"), "-domain", filepath.Join(dir, "d.go")}, &stdout, &stderr)
	if code != exitFailure || !strings.Contains(stderr.String(), "ws_ops (heartbeat)") || !strings.Contains(stderr.String(), "OpHeartbeat") {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
}

// TestRun_SecondRenameFailsRestoresFirst: when the domain file cannot be replaced (a directory is
// in its place), the ids file keeps its previous content.
func TestRun_SecondRenameFailsRestoresFirst(t *testing.T) {
	dir := t.TempDir()
	ids, domain := filepath.Join(dir, "ids.go"), filepath.Join(dir, "domain.go")
	if err := os.WriteFile(ids, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(domain, "child"), 0o700); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-catalog", "internal/gen/testdata/valid.yaml", "-ids", ids, "-domain", domain}, &stdout, &stderr); code != exitFailure {
		t.Fatalf("exit %d", code)
	}
	got, err := os.ReadFile(ids) //nolint:gosec // G304: a file this test wrote
	if err != nil || string(got) != "old" {
		t.Fatalf("ids file = %q, %v; want the previous content", got, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatalf("left temporary files: %v", entries)
	}
}
