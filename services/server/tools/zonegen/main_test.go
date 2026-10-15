// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNamesAndRender(t *testing.T) {
	dir := t.TempDir()
	zp := filepath.Join(dir, "z.zip")
	f, err := os.Create(zp) //nolint:gosec // G304: a file in the test's own temporary directory
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for _, n := range []string{"UTC", "Europe/", "Europe/Berlin", "Factory"} {
		if _, err := w.Create(n); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	names, err := Names(zp)
	if err != nil || strings.Join(names, ",") != "Europe/Berlin,UTC" {
		t.Fatalf("Names = %v, %v", names, err)
	}
	if src := string(Render(names)); !strings.Contains(src, `"Europe/Berlin": true`) || !strings.Contains(src, "DO NOT EDIT") {
		t.Fatalf("Render:\n%s", src)
	}
	out := filepath.Join(dir, "out.go")
	var stderr bytes.Buffer
	if code := run([]string{"-zip", zp, "-out", out}, &stderr); code != 0 {
		t.Fatalf("run = %d: %s", code, stderr.String())
	}
	if code := run(nil, &stderr); code != 2 {
		t.Fatalf("run without -out = %d", code)
	}
	if code := run([]string{"-zip", filepath.Join(dir, "missing.zip"), "-out", out}, &stderr); code != 1 {
		t.Fatalf("run with a missing zip = %d", code)
	}
}

// TestDefaultZipFromGOROOT: without -zip the zip comes from $GOROOT, which go generate sets for
// the generator; without GOROOT the run is a usage error and starts no command.
func TestDefaultZipFromGOROOT(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "lib", "time"), 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(root, "lib", "time", "zoneinfo.zip")) //nolint:gosec // G304: the test's own temporary directory
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	if _, err := w.Create("UTC"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	out := filepath.Join(t.TempDir(), "out.go")
	var stderr bytes.Buffer
	t.Setenv("GOROOT", root)
	if code := run([]string{"-out", out}, &stderr); code != 0 {
		t.Fatalf("run with GOROOT = %d: %s", code, stderr.String())
	}
	t.Setenv("GOROOT", "")
	if code := run([]string{"-out", out}, &stderr); code != 2 {
		t.Fatalf("run without GOROOT and -zip = %d, want 2", code)
	}
}
