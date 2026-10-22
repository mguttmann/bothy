// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildTarget is the binary of the standard build whose dependencies must be pure Go.
const buildTarget = "./cmd/bothy"

// listedPackage is the part of a `go list -json` record the test reads.
type listedPackage struct {
	ImportPath string
	Standard   bool
	CgoFiles   []string
}

// TestStandardBuildWithoutCgo lists every dependency of the server binary with cgo enabled and
// fails for any package outside the standard library that has cgo files (design note 15: the
// image processing stays pure Go, so the standard build needs no C toolchain).
func TestStandardBuildWithoutCgo(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("no go binary in PATH: %v", err)
	}
	t.Setenv("CGO_ENABLED", "1")                                                          // the command inherits the environment of the test
	cmd := exec.CommandContext(t.Context(), goBin, "list", "-deps", "-json", buildTarget) //nolint:gosec // G204: the go binary from PATH with fixed arguments
	cmd.Dir = moduleRoot(t)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, stderr.String())
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	count := 0
	for {
		var p listedPackage
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatalf("decode go list output: %v", err)
		}
		count++
		if !p.Standard && len(p.CgoFiles) > 0 {
			t.Errorf("%s has cgo files %v", p.ImportPath, p.CgoFiles)
		}
	}
	if count == 0 {
		t.Fatalf("go list listed no package")
	}
}

// moduleRoot walks up from the package directory to the directory of go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod above the package")
		}
		dir = parent
	}
}
