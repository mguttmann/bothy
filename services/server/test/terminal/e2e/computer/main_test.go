// SPDX-License-Identifier: Apache-2.0

//go:build computere2e

// Package computer is the end-to-end test of computer use CU-T24 (docs/spec/27-computer-use.md
// section 15, brief CU-06 requirement 19): the terminal stack of M2-48 (the server image of this
// tree with PostgreSQL in Compose, the TLS front and real host services in Linux containers with
// systemd) with two machines on its host network: the desktop, a host service with
// host_service.computer_use and the Linux test backend of the helper in Xvfb with the fixture app,
// and the runner machine, the Claude Code runner of this tree with runner.computer_use and the
// bothy-computer-mcp of this tree, whose Fake-Claude calls the MCP tools of a script. No model runs
// anywhere.
//
// The build tag computere2e keeps it out of go test ./... and make check: it needs a Docker engine
// (as every e2e package of the suite) and uv for the runner's Python dependencies, and runs with
// make computer-e2e (CI job test-adapters-e2e). Without either it fails, it never skips.
package computer

import (
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/test/terminal/suite"
)

// hostPackages are the Debian packages of the computer variant of the host image: Xvfb, Python
// with python-xlib, Pillow and Tk for the test backend and the fixture app
// (services/host/test/computer_backend), and the full Python for the runner.
var hostPackages = []string{"xvfb", "python3", "python3-xlib", "python3-pil", "python3-tk"}

// TestMain starts the one stack of the package and removes it after the test.
func TestMain(m *testing.M) {
	os.Exit(suite.MainWith(m, "computer", "default", suite.Extra{HostPackages: hostPackages}))
}
