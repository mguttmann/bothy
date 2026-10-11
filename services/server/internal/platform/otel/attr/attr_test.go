// SPDX-License-Identifier: Apache-2.0

package attr

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/genai"
)

func TestAllowed(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		HTTPRoute:                    true,
		ErrorType:                    true,
		PrincipalPseudoID:            true,
		CorrelationID:                true,
		DBQueryText:                  true,
		genai.ProviderName:           true,
		Signal:                       false, // metric-only
		ServerRole:                   false, // resource only
		"url.query":                  false,
		"url.path":                   false,
		"client.address":             false,
		"http.request.header.cookie": false,
		"exception.message":          false,
		"gen_ai.input.messages":      false,
		"":                           false,
	}
	for key, want := range cases {
		if got := Allowed(key); got != want {
			t.Errorf("Allowed(%q) = %v, want %v", key, got, want)
		}
	}
}

// TestEveryConstantRegistered parses attr.go: every exported string constant has a scope, and
// every scope entry is a constant, so a new key cannot bypass Allowed or the collector check.
func TestEveryConstantRegistered(t *testing.T) {
	t.Parallel()
	f, err := parser.ParseFile(token.NewFileSet(), "attr.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	consts := map[string]bool{}
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				if i >= len(vs.Values) { // iota constants repeat the previous expression
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !name.IsExported() || !ok || lit.Kind != token.STRING {
					continue
				}
				key, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatal(err)
				}
				consts[key] = true
				if _, ok := scopes[key]; !ok {
					t.Errorf("constant %s = %q has no scope", name.Name, key)
				}
			}
		}
	}
	for key := range scopes {
		if !consts[key] {
			t.Errorf("scope entry %q is not an exported constant of attr.go", key)
		}
	}
}
