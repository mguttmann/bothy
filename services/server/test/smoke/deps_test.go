// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"os/exec"
	"strings"
	"testing"
)

// module is the Go module of the server.
const module = "github.com/OWNER/bothy/services/server/"

// TestBlackBoxImports: the probe and its command import, besides the standard library, only the
// generated api/rest and github.com/coder/websocket, and no package of the module outside api/rest,
// its own dependencies and test/smoke is among their dependencies, so the probe reaches no
// internal/... package except through the generated client.
func TestBlackBoxImports(t *testing.T) {
	t.Parallel()
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not on PATH")
	}
	run := func(args ...string) []string {
		out, err := exec.CommandContext(t.Context(), goBin, args...).Output() // #nosec G204 -- fixed arguments
		if err != nil {
			t.Fatalf("go %v: %v", args, err)
		}
		return strings.Fields(string(out))
	}
	pkgs := []string{module + "test/smoke", module + "test/smoke/cmd/bothy-smoke"}
	allowed := map[string]bool{module + "api/rest": true, "github.com/coder/websocket": true, module + "test/smoke": true}
	for _, imp := range run(append([]string{"list", "-f", `{{join .Imports "\n"}}`}, pkgs...)...) {
		if strings.Contains(strings.SplitN(imp, "/", 2)[0], ".") && !allowed[imp] {
			t.Errorf("import %s: the probe imports only api/rest and github.com/coder/websocket", imp)
		}
	}
	// The generated client api/rest may use value types of the module (internal/kernel/ids); its
	// own dependencies are allowed, every other package of the module is not.
	viaREST := map[string]bool{}
	for _, dep := range run("list", "-deps", module+"api/rest") {
		viaREST[dep] = true
	}
	for _, dep := range run(append([]string{"list", "-deps"}, pkgs...)...) {
		if strings.HasPrefix(dep, module) && !allowed[dep] && !viaREST[dep] && !strings.HasPrefix(dep, module+"test/smoke") {
			t.Errorf("dependency %s: the probe is a black box of the server", dep)
		}
	}
}
