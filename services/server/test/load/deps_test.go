// SPDX-License-Identifier: Apache-2.0

package load

import (
	"os/exec"
	"strings"
	"testing"
)

// module is the Go module of the server.
const module = "github.com/OWNER/bothy/services/server/"

// TestBlackBoxImports: the load test and its command import, besides the standard library, only the
// generated api/rest, github.com/coder/websocket and the DPoP helpers of test/smoke, and reach no
// other package of the module, so the load is a client of the public API like any app.
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
	pkgs := []string{module + "test/load", module + "test/load/cmd/bothy-load"}
	allowed := map[string]bool{
		module + "api/rest": true, "github.com/coder/websocket": true, module + "test/smoke": true, module + "test/load": true,
	}
	for _, imp := range run(append([]string{"list", "-f", `{{join .Imports "\n"}}`}, pkgs...)...) {
		if strings.Contains(strings.SplitN(imp, "/", 2)[0], ".") && !allowed[imp] {
			t.Errorf("import %s: the load test imports only api/rest, websocket and test/smoke", imp)
		}
	}
	// test/smoke is itself a black box (its TestBlackBoxImports); its dependencies and those of
	// api/rest are allowed, every other package of the module is not.
	viaAllowed := map[string]bool{}
	for _, dep := range run("list", "-deps", module+"api/rest", module+"test/smoke") {
		viaAllowed[dep] = true
	}
	for _, dep := range run(append([]string{"list", "-deps"}, pkgs...)...) {
		if strings.HasPrefix(dep, module) && !allowed[dep] && !viaAllowed[dep] && !strings.HasPrefix(dep, module+"test/load") {
			t.Errorf("dependency %s: the load test is a black box of the server", dep)
		}
	}
}
