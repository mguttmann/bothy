// SPDX-License-Identifier: Apache-2.0

package main

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/OWNER/bothy/services/server/api/internal/gosplit"
)

// splitRest splits the package oapi-codegen wrote into models.gen.go (every declaration before
// the type ServerInterface: the types of the schemas, parameters and request bodies),
// objects.gen.go (the request and response objects of the strict server: from the first type
// named *RequestObject up to the type StrictServerInterface) and rest.gen.go (the rest from
// ServerInterface on: std-http server, strict interface and handler), so no file reaches
// repo.max_file_kb (docs/DECISIONS.md M1-17/16, INT-W14/4, INT-W17/4). Every file keeps the
// header up to the package clause (build constraint, generated-code marker) and imports only what
// it uses. Without ServerInterface every declaration is a model; without the objects or
// StrictServerInterface objects.gen.go holds only the package clause, so all three files always
// exist. The declarations are moved byte for byte; the files are gofmt-formatted (api/internal/gosplit).
// Errors: src is not valid Go or has an import whose name cannot be derived from its path.
func splitRest(src []byte) (models, server, objects []byte, err error) {
	f, err := gosplit.Parse("rest.gen.go", src)
	if err != nil {
		return nil, nil, nil, err
	}
	offset := func(d *ast.GenDecl) int { return f.DeclStart(d) }
	cut := len(src)
	if d := typeDecl(f.AST, func(n string) bool { return n == "ServerInterface" }); d != nil {
		cut = offset(d)
	}
	objStart, objEnd := len(src), len(src)
	first := typeDecl(f.AST, func(n string) bool { return strings.HasSuffix(n, "RequestObject") })
	strict := typeDecl(f.AST, func(n string) bool { return n == "StrictServerInterface" })
	if first != nil && strict != nil && cut <= offset(first) && offset(first) < offset(strict) {
		objStart, objEnd = offset(first), offset(strict)
	}
	if models, err = f.Assemble(src[f.BodyStart:cut]); err != nil {
		return nil, nil, nil, err
	}
	rest := append(append([]byte{}, src[cut:objStart]...), src[objEnd:]...)
	if server, err = f.Assemble(rest); err != nil {
		return nil, nil, nil, err
	}
	if objects, err = f.Assemble(src[objStart:objEnd]); err != nil {
		return nil, nil, nil, err
	}
	return models, server, objects, nil
}

// typeDecl returns the first type declaration of f with a type whose name matches, nil when
// there is none.
func typeDecl(f *ast.File, match func(name string) bool) *ast.GenDecl {
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.TYPE {
			continue
		}
		for _, s := range g.Specs {
			if match(s.(*ast.TypeSpec).Name.Name) {
				return g
			}
		}
	}
	return nil
}
