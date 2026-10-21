// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRun_RealCodegenOnTheFixture runs the pinned oapi-codegen on the small contract fixture of
// the route generator and checks that the client package is written.
func TestRun_RealCodegenOnTheFixture(t *testing.T) {
	out := t.TempDir()
	var stderr bytes.Buffer
	code := run(context.Background(), []string{
		"-spec", filepath.Join("..", "..", "routes", "gen", "testdata", "fixture", "openapi.yaml"),
		"-config", filepath.Join("..", "oapi-codegen.yaml"),
		"-out", out,
	}, &stderr, runOapiCodegen)
	if code != 0 {
		t.Fatalf("run = %d, stderr: %s", code, stderr.String())
	}
	src, err := os.ReadFile(filepath.Clean(filepath.Join(out, output)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(src, []byte("package restclient")) {
		t.Fatalf("%s is not the client package", output)
	}
}

// TestRun_CodegenFailureIsExitCodeOne checks the exit code and message of a failed generation.
func TestRun_CodegenFailureIsExitCodeOne(t *testing.T) {
	failed := errors.New("codegen failed")
	var stderr bytes.Buffer
	code := run(context.Background(), []string{"-spec", "a.yaml", "-config", "c.yaml", "-out", t.TempDir()}, &stderr,
		func(context.Context, string, string, string) error { return failed })
	if code != 1 || !strings.Contains(stderr.String(), "codegen failed") {
		t.Fatalf("run = %d, stderr %q", code, stderr.String())
	}
}

// TestGenerate_NoOutputIsAnError checks a codegen run that writes no file.
func TestGenerate_NoOutputIsAnError(t *testing.T) {
	err := generate(context.Background(), "a.yaml", "c.yaml", t.TempDir(),
		func(context.Context, string, string, string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "wrote no") {
		t.Fatalf("generate = %v", err)
	}
}

// TestRunOapiCodegen_ReportsItsOutput checks that a failing oapi-codegen run returns its message.
func TestRunOapiCodegen_ReportsItsOutput(t *testing.T) {
	err := runOapiCodegen(context.Background(), filepath.Join(t.TempDir(), "missing.yaml"), "missing.yaml", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "oapi-codegen") {
		t.Fatalf("runOapiCodegen = %v", err)
	}
}
