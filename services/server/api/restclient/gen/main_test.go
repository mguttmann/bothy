// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRealClient_BelowMaxFileKB: every part of the checked-in client stays below repo.max_file_kb
// (docs/spec/23-qualitaet.md), the limit the hook check-added-large-files holds.
func TestRealClient_BelowMaxFileKB(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "*.gen.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no generated client files: %v", err)
	}
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() > maxFileBytes {
			t.Errorf("%s has %d bytes, above repo.max_file_kb", f, info.Size())
		}
	}
}

// TestGenerate_WritesPartsAndRemovesStale: the generator writes the parts of the codegen output
// and removes a part of an earlier run that it does not write again.
func TestGenerate_WritesPartsAndRemovesStale(t *testing.T) {
	out := t.TempDir()
	stale := filepath.Join(out, "restclient_09.gen.go")
	if err := os.WriteFile(stale, []byte("package restclient\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := func(_ context.Context, _, _, dir string) error {
		return os.WriteFile(filepath.Join(dir, output), []byte("// Code generated. DO NOT EDIT.\npackage restclient\n\n// A is a type.\ntype A int\n"), 0o600)
	}
	var stderr bytes.Buffer
	if code := run(t.Context(), []string{"-spec", "s.yaml", "-config", "c.yaml", "-out", out}, &stderr, fake); code != 0 {
		t.Fatalf("run = %d: %s", code, stderr.String())
	}
	got, err := os.ReadFile(filepath.Join(out, output)) // #nosec G304 -- a path inside t.TempDir()
	if err != nil || !strings.Contains(string(got), "type A int") {
		t.Fatalf("%s = %q, %v", output, got, err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale part still exists: %v", err)
	}
}

// TestRun_Usage: missing flags are exit 2, a failing codegen exit 1.
func TestRun_Usage(t *testing.T) {
	var stderr bytes.Buffer
	if code := run(t.Context(), nil, &stderr, nil); code != 2 {
		t.Errorf("run without flags = %d, want 2", code)
	}
	fail := func(context.Context, string, string, string) error { return os.ErrPermission }
	if code := run(t.Context(), []string{"-spec", "s", "-config", "c", "-out", t.TempDir()}, &stderr, fail); code != 1 {
		t.Errorf("run with failing codegen = %d, want 1", code)
	}
}
