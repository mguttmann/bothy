// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestMain builds the binary once into a temporary directory for the tests that run bothy as a
// child process, then runs the tests with a PostgreSQL container for bothy migrate and bothy
// serve (plan P-09: without a Docker-compatible runtime the binary fails, it never skips). The
// templates run the steps of bothy migrate up, so every database has its instance row.
func TestMain(m *testing.M) { os.Exit(testMain(m)) }

func testMain(m *testing.M) int {
	dir, err := os.MkdirTemp("", "bothy-bin-")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "build bothy: %v\n", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(dir) }()
	bothyBinary = filepath.Join(dir, "bothy")
	build := exec.CommandContext(context.Background(), "go", "build", "-o", bothyBinary, ".") // #nosec G204 -- fixed arguments
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	if err := build.Run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "build bothy: %v\n", err)
		return 1
	}
	return pgtest.Main(m, pgtest.WithSteps(wire.MigrationSteps))
}
