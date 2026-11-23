// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
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

// maxFileBytes is repo.max_file_kb (docs/spec/23-quality.md) in bytes: the hook
// check-added-large-files refuses larger files.
const maxFileBytes = 1024 * 1024

// splitAll is splitRest with objects.gen.go cut into parts below repo.max_file_kb (chunkObjects)
// and rest.gen.go cut into rest.gen.go and rest2.gen.go (halveServer).
func splitAll(src []byte) (models, server, server2 []byte, objects [][]byte, err error) {
	models, server, whole, err := splitRest(src)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if objects, err = chunkObjects(whole); err != nil {
		return nil, nil, nil, nil, err
	}
	server, server2, err = halveServer(server)
	return models, server, server2, objects, err
}

// halveServer cuts rest.gen.go (the server part of splitRest) at the type StrictServerInterface:
// the std-http server stays in rest.gen.go, the strict interface and its handler go to
// rest2.gen.go, so neither reaches repo.max_file_kb as the contract grows (AR-04: the server part
// passed it with the operations of stage AR). Each file keeps the header and only the imports it
// uses. Without StrictServerInterface the first file keeps everything and the second holds only
// the package clause. Errors: server is not valid Go.
func halveServer(server []byte) (head, tail []byte, err error) {
	f, err := gosplit.Parse("rest.gen.go", server)
	if err != nil {
		return nil, nil, err
	}
	cut := len(server)
	if d := typeDecl(f.AST, func(n string) bool { return n == "StrictServerInterface" }); d != nil {
		cut = f.DeclStart(d)
	}
	if head, err = f.Assemble(server[f.BodyStart:cut]); err != nil {
		return nil, nil, err
	}
	if tail, err = f.Assemble(server[cut:]); err != nil {
		return nil, nil, err
	}
	return head, tail, nil
}

// chunkObjects cuts objects.gen.go (the output of splitRest) into parts of at most
// repo.max_file_kb, only between top-level declarations (api/internal/gosplit, as the API client
// is cut), so the request and response objects never reach the limit as the contract grows
// (M6-01, M2-07; audit finding api-contract-codegen-3: two fixed halves left no room). Each part
// keeps the header and only the imports it uses; the first part always exists. Errors: objects
// is not valid Go, or one declaration alone is larger than the limit.
func chunkObjects(objects []byte) ([][]byte, error) {
	return gosplit.Chunks(objects, maxFileBytes)
}

// objectsName returns the file name of part i (0-based) of chunkObjects: objects.gen.go,
// objects2.gen.go, objects3.gen.go and so on.
func objectsName(i int) string {
	if i == 0 {
		return "objects.gen.go"
	}
	return fmt.Sprintf("objects%d.gen.go", i+1)
}
