// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestDemoOnlyUseCases (brief M7-27 requirement 13): the demo package reaches the instance only
// through the public API. Neither it nor anything it imports may be server code of a module, a
// kernel port, the composition root, a platform package (the database among them) or a database
// driver; of the server it uses only the generated contract api/rest and, through it, the kernel
// core that every package may use (architecture/modules.yaml, kernel.core).
func TestDemoOnlyUseCases(t *testing.T) {
	t.Parallel()
	out, err := exec.CommandContext(t.Context(), "go", "list", "-deps", "-f", "{{.ImportPath}}", ".").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	const server = "github.com/OWNER/bothy/services/server/"
	allowed := map[string]bool{server + "api/rest": true, server + "cmd/bothy/demo": true}
	for _, core := range []string{"ids", "clock", "errs", "textcheck", "reqmeta", "jose", "designkeys", "searchlang", "searchkit", "statskit"} {
		allowed[server+"internal/kernel/"+core] = true
	}
	var bad []string
	for _, pkg := range strings.Fields(string(out)) {
		if (strings.HasPrefix(pkg, server) && !allowed[pkg]) || strings.Contains(pkg, "jackc/pgx") || strings.Contains(pkg, "riverqueue") || pkg == "database/sql" {
			bad = append(bad, pkg)
		}
	}
	if len(bad) > 0 {
		t.Fatalf("the demo package depends on server internals: %v", bad)
	}
}
