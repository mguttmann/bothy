// SPDX-License-Identifier: Apache-2.0

package discord

import (
	"os/exec"
	"strings"
	"testing"
)

// serverModule is the module path of services/server.
const serverModule = "github.com/OWNER/bothy/services/server"

// goListDeps runs a go list -deps command and returns the listed packages.
func goListDeps(t *testing.T, cmd *exec.Cmd) []string {
	t.Helper()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", cmd, err, out)
	}
	return strings.Fields(string(out))
}

// TestDiscordClientImportsNoServerModule (Design note 1): the client reaches no server module;
// of internal/ only platform/egress and the kernel. The tool talks to the server only through
// the public API (22 2, R-IM-01).
func TestDiscordClientImportsNoServerModule(t *testing.T) {
	deps := goListDeps(t, exec.CommandContext(t.Context(), "go", "list", "-deps", "."))
	if len(deps) == 0 {
		t.Fatal("no dependencies listed")
	}
	for _, d := range deps {
		rest, ok := strings.CutPrefix(d, serverModule+"/internal/")
		if !ok {
			continue
		}
		if rest != "platform/egress" && !strings.HasPrefix(rest, "kernel/") {
			t.Errorf("the Discord client depends on internal/%s", rest)
		}
	}
}

// TestNoTestPackageInBinary: the fake Discord never reaches the binary bothy.
func TestNoTestPackageInBinary(t *testing.T) {
	deps := goListDeps(t, exec.CommandContext(t.Context(), "go", "list", "-deps", "../.."))
	if len(deps) == 0 {
		t.Fatal("no dependencies listed")
	}
	for _, d := range deps {
		if strings.Contains(d, "discordtest") {
			t.Errorf("the binary bothy depends on %s", d)
		}
	}
}
