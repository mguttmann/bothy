// SPDX-License-Identifier: Apache-2.0

package bothyapi

import (
	"os/exec"
	"strings"
	"testing"
)

// serverModule is the module path of services/server.
const serverModule = "github.com/OWNER/bothy/services/server"

// goListDeps runs go list -deps for pkg and returns the listed packages.
func goListDeps(t *testing.T, pkg string) []string {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "go", "list", "-deps", pkg).CombinedOutput() // #nosec G204 -- pkg is a fixed relative package path of the test
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", pkg, err, out)
	}
	return strings.Fields(string(out))
}

// TestClientImportsNoServerModule (22 2, R-IM-01): the client reaches the server only through
// the public API; of internal/ it uses only platform/egress and the kernel.
func TestClientImportsNoServerModule(t *testing.T) {
	for _, d := range goListDeps(t, ".") {
		rest, ok := strings.CutPrefix(d, serverModule+"/internal/")
		if ok && rest != "platform/egress" && !strings.HasPrefix(rest, "kernel/") {
			t.Errorf("the client depends on internal/%s", rest)
		}
	}
}

// TestNoTestPackageInBinary: the fake server never reaches the binary bothy.
func TestNoTestPackageInBinary(t *testing.T) {
	for _, d := range goListDeps(t, "../..") {
		if strings.Contains(d, "bothytest") {
			t.Errorf("the binary bothy depends on %s", d)
		}
	}
}
