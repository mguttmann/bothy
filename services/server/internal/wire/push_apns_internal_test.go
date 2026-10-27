// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"crypto/x509"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// TestProductionWiringHasNoTestOverrides (brief M3-24 design note 3 b): the APNs adapter of a
// process built the way cmd/bothy builds it gets no endpoint override (the adapter then targets
// the two Apple hosts) and no extra TLS roots (the egress client then uses the system pool); the
// overrides of a test server reach the adapter unchanged; and cmd/bothy names neither field.
func TestProductionWiringHasNoTestOverrides(t *testing.T) {
	t.Parallel()
	prod := (&server{role: RoleAll, deps: Deps{Env: []string{}, Stdout: io.Discard, Version: "test"}}).apnsDeps(egress.Config{})
	if prod.Endpoints != nil || prod.EgressDeps.RootCAs != nil {
		t.Fatalf("production APNs deps carry test overrides: endpoints %v, root CAs set %v", prod.Endpoints, prod.EgressDeps.RootCAs != nil)
	}
	endpoints := map[string]string{"production": "https://127.0.0.1:1", "sandbox": "https://127.0.0.1:2"}
	pool := x509.NewCertPool()
	test := (&server{role: RoleAll, deps: Deps{APNsEndpoints: endpoints, APNsRootCAs: pool}}).apnsDeps(egress.Config{})
	if !maps.Equal(test.Endpoints, endpoints) || test.EgressDeps.RootCAs != pool {
		t.Fatalf("test APNs deps = endpoints %v, root CAs %p; want %v and %p", test.Endpoints, test.EgressDeps.RootCAs, endpoints, pool)
	}
	files, err := filepath.Glob(filepath.Join("..", "..", "cmd", "bothy", "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("cmd/bothy sources: %v, %v", files, err)
	}
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f) // #nosec G304 -- a source file of this module
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, f, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && (id.Name == "APNsEndpoints" || id.Name == "APNsRootCAs" || id.Name == "FCMEndpoints") {
				t.Errorf("%s names the test override %s", fset.Position(id.Pos()), id.Name)
			}
			return true
		})
	}
}
