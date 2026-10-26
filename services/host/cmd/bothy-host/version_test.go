// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestVersionEqualsVERSION (brief M7-23 requirement 13): a binary built with the linker flag of
// make release-build (tools/distribution/hostdist.py) prints exactly the version of services/host/VERSION, so a renamed variable
// cannot leave the release binaries at the development default.
func TestVersionEqualsVERSION(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(string(raw))
	// make release-build runs tools/distribution/hostdist.py, which sets the version with this flag.
	tool, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "tools", "distribution", "hostdist.py"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`MODULE = "github.com/OWNER/bothy/services/host"`, "-X {MODULE}/internal/buildinfo.Version={version} "} {
		if !strings.Contains(string(tool), want) {
			t.Fatalf("hostdist.py sets the version with another flag than %q", want)
		}
	}
	bin := filepath.Join(t.TempDir(), "bothy-host")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.CommandContext(context.Background(), "go", "build", "-trimpath", //nolint:gosec // the Go tool with fixed arguments
		"-ldflags", "-X github.com/OWNER/bothy/services/host/internal/buildinfo.Version="+want, "-o", bin, ".")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	out, err := exec.CommandContext(context.Background(), bin, "version").Output() //nolint:gosec // the binary this test built
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if string(out) != want+"\n" {
		t.Errorf("version printed %q, want %q", out, want+"\n")
	}
}

// The version command prints the version of the build and nothing else; arguments are a usage
// error.
func TestVersionCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"version"}, &stdout, &stderr); code != exitOK || stdout.String() != "0.0.0-dev\n" {
		t.Errorf("code %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	if code := run(context.Background(), []string{"version", "extra"}, &stdout, &stderr); code != exitUsage || stdout.Len() != 0 {
		t.Errorf("version with an argument: code %d stdout %q", code, stdout.String())
	}
}
