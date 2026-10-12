// SPDX-License-Identifier: Apache-2.0

package authorize_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// catalogueTypes are the identifier types of catalog_ids_gen.go.
var catalogueTypes = map[string]bool{"Action": true, "EventType": true, "Op": true, "ResourceKind": true, "ChannelOp": true}

// TestNoConversionInsideThePackage: the Semgrep rule bothy-go-catalog-id-conversion sees only
// qualified conversions (authorize.Action(s)); inside this package a conversion is unqualified,
// so this test holds the hand-written files to the same rule.
func TestNoConversionInsideThePackage(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		n := e.Name()
		if !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") || strings.HasSuffix(n, "_gen.go") {
			continue
		}
		f, err := parser.ParseFile(fset, n, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range conversions(f) {
			t.Errorf("%s: converts to %s; use a constant or a Parse function", fset.Position(bad.Pos()), bad.(*ast.Ident).Name)
		}
	}
}

// TestNoConversionInsideThePackage_FindsFixture proves the scan on a source that converts.
func TestNoConversionInsideThePackage_FindsFixture(t *testing.T) {
	src := "package authorize\nfunc f(s string) { _ = Action(s); _ = Op(\"x\"); _ = (EventType)(s); _ = len(s) }\nvar v ResourceKind = \"k\"\n"
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(conversions(f)); got != 4 {
		t.Fatalf("found %d conversions, want 4", got)
	}
}

// conversions returns the type identifiers of every conversion to a catalogue type in f and of
// every typed declaration from a string literal.
func conversions(f *ast.File) []ast.Node {
	var out []ast.Node
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			fun := n.Fun
			for {
				p, ok := fun.(*ast.ParenExpr)
				if !ok {
					break
				}
				fun = p.X
			}
			if id, ok := fun.(*ast.Ident); ok && catalogueTypes[id.Name] {
				out = append(out, id)
			}
		case *ast.ValueSpec:
			if id, ok := n.Type.(*ast.Ident); ok && catalogueTypes[id.Name] {
				for _, v := range n.Values {
					if lit, ok := v.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						out = append(out, id)
					}
				}
			}
		}
		return true
	})
	return out
}
