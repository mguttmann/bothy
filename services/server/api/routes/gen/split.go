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

// splitAll splits the package oapi-codegen wrote into models, server and the two halves of the
// strict objects (splitRest, then halveObjects). Errors: those of both.
func splitAll(src []byte) (models, server, objects, objects2 []byte, err error) {
	models, server, objects, err = splitRest(src)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	objects, objects2, err = halveObjects(objects)
	return models, server, objects, objects2, err
}

// halveObjects cuts objects.gen.go (the output of splitRest) into two files at the start of the
// operation group nearest its middle: the type declaration named *RequestObject, other than the
// first, closest to half of its body. Each file keeps the header and only the imports it uses, so neither reaches
// repo.max_file_kb as the contract grows (M6-01). With one group only the first file keeps
// everything and the second holds only the package clause. Errors: objects is
// not valid Go.
func halveObjects(objects []byte) (head, tail []byte, err error) {
	f, err := gosplit.Parse("objects.gen.go", objects)
	if err != nil {
		return nil, nil, err
	}
	cut := groupNearest(f, f.BodyStart+(len(objects)-f.BodyStart)/2, len(objects))
	if head, err = f.Assemble(objects[f.BodyStart:cut]); err != nil {
		return nil, nil, err
	}
	if tail, err = f.Assemble(objects[cut:]); err != nil {
		return nil, nil, err
	}
	return head, tail, nil
}

// groupNearest returns the offset of the operation group of f (a type declaration named
// *RequestObject with its doc comment) nearest to middle, never the first group; end when f has
// only one group.
func groupNearest(f *gosplit.File, middle, end int) int {
	cut, first := end, true
	for _, d := range f.AST.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.TYPE || !strings.HasSuffix(g.Specs[0].(*ast.TypeSpec).Name.Name, "RequestObject") {
			continue
		}
		if first {
			// The first group starts the file; cutting there would leave the first file empty.
			first = false
			continue
		}
		if off := f.DeclStart(g); cut == end || abs(off-middle) < abs(cut-middle) {
			cut = off
		}
	}
	return cut
}

// abs returns the distance of n from zero.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
