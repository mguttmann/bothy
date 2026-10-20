// SPDX-License-Identifier: Apache-2.0

package session

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoInboundNetworkListener: no package of the terminal path opens a network listener (R-TM-01,
// requirement 29); only internal/confirm may listen, and only on a unix socket or a named pipe.
func TestNoInboundNetworkListener(t *testing.T) {
	root := filepath.Join("..", "..")
	dirs := []string{"internal/session", "internal/session/sessiontest", "internal/pty", "internal/profiles", "internal/confirm", "internal/helper", "cmd/bothy-host"}
	banned := map[string]bool{"Listen": true, "ListenTCP": true, "ListenUDP": true, "ListenPacket": true, "ListenAndServe": true, "ListenAndServeTLS": true}
	for _, d := range dirs {
		entries, err := os.ReadDir(filepath.Join(root, d))
		if err != nil {
			t.Fatalf("%s: %v", d, err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || strings.HasPrefix(d, "internal/session/sessiontest") {
				continue
			}
			f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, d, name), nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !banned[sel.Sel.Name] {
					return true
				}
				if d == "internal/confirm" && len(call.Args) >= 2 {
					if lit, ok := call.Args[len(call.Args)-2].(*ast.BasicLit); ok && lit.Value == `"unix"` {
						return true
					}
				}
				t.Errorf("%s/%s calls %s", d, name, sel.Sel.Name)
				return true
			})
		}
	}
}
