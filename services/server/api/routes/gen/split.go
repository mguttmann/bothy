// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path"
	"regexp"
	"strconv"
	"strings"
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
// exist. The declarations are moved byte for byte; the files are gofmt-formatted. Errors: src is
// not valid Go or has an import whose name cannot be derived from its path.
func splitRest(src []byte) (models, server, objects []byte, err error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "rest.gen.go", src, parser.ParseComments)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("split rest.gen.go: %w", err)
	}
	header := src[:fset.Position(f.Package).Offset]
	bodyStart := fset.Position(f.Name.End()).Offset
	for _, d := range f.Decls {
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
			bodyStart = fset.Position(g.End()).Offset
		}
	}
	offset := func(d *ast.GenDecl) int {
		start := d.Pos()
		if d.Doc != nil {
			start = d.Doc.Pos()
		}
		return fset.Position(start).Offset
	}
	cut := len(src)
	if d := typeDecl(f, func(n string) bool { return n == "ServerInterface" }); d != nil {
		cut = offset(d)
	}
	objStart, objEnd := len(src), len(src)
	first := typeDecl(f, func(n string) bool { return strings.HasSuffix(n, "RequestObject") })
	strict := typeDecl(f, func(n string) bool { return n == "StrictServerInterface" })
	if first != nil && strict != nil && cut <= offset(first) && offset(first) < offset(strict) {
		objStart, objEnd = offset(first), offset(strict)
	}
	if models, err = assemble(header, f, src[bodyStart:cut]); err != nil {
		return nil, nil, nil, err
	}
	rest := append(append([]byte{}, src[cut:objStart]...), src[objEnd:]...)
	if server, err = assemble(header, f, rest); err != nil {
		return nil, nil, nil, err
	}
	if objects, err = assemble(header, f, src[objStart:objEnd]); err != nil {
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

// importName matches an import path element that is its package name.
var importName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// assemble writes header, the package clause of f, the imports of f that body uses and body, and
// formats the result.
func assemble(header []byte, f *ast.File, body []byte) ([]byte, error) {
	used, err := usedPackages(f.Name.Name, body)
	if err != nil {
		return nil, err
	}
	std, other, err := importLines(f.Imports, used)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.Write(header)
	fmt.Fprintf(&buf, "package %s\n\n", f.Name.Name)
	writeImports(&buf, std, other)
	buf.Write(body)
	out, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, errors.Join(errors.New("split rest.gen.go: a part is not valid Go"), err)
	}
	return out, nil
}

// importLines returns the import specs of imports whose package name is in used, as source lines,
// split into the standard library (no dot in the path) and the others.
func importLines(imports []*ast.ImportSpec, used map[string]bool) (std, other []string, err error) {
	for _, s := range imports {
		p, err := strconv.Unquote(s.Path.Value)
		if err != nil {
			return nil, nil, err
		}
		name := path.Base(p)
		if s.Name != nil {
			name = s.Name.Name
		}
		if !importName.MatchString(name) || name == "_" {
			return nil, nil, fmt.Errorf("split rest.gen.go: cannot tell which half uses the import %s", s.Path.Value)
		}
		if !used[name] {
			continue
		}
		line := s.Path.Value
		if s.Name != nil {
			line = s.Name.Name + " " + line
		}
		if strings.Contains(p, ".") {
			other = append(other, line)
		} else {
			std = append(std, line)
		}
	}
	return std, other, nil
}

// writeImports writes the import block of std and other (one group each) to buf; nothing when
// both are empty.
func writeImports(buf *bytes.Buffer, std, other []string) {
	if len(std)+len(other) == 0 {
		return
	}
	buf.WriteString("import (\n")
	for _, l := range std {
		buf.WriteString("\t" + l + "\n")
	}
	if len(std) > 0 && len(other) > 0 {
		buf.WriteString("\n")
	}
	for _, l := range other {
		buf.WriteString("\t" + l + "\n")
	}
	buf.WriteString(")\n")
}

// usedPackages returns the names body uses as the left side of a selector without declaring
// them, which in generated code are the imported packages.
func usedPackages(pkg string, body []byte) (map[string]bool, error) {
	src := append([]byte("package "+pkg+"\n"), body...)
	f, err := parser.ParseFile(token.NewFileSet(), "half.go", src, 0)
	if err != nil {
		return nil, fmt.Errorf("split rest.gen.go: %w", err)
	}
	used := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			// An unresolved name (Obj nil) on the left of a selector is a package name.
			if id, ok := sel.X.(*ast.Ident); ok && id.Obj == nil {
				used[id.Name] = true
			}
		}
		return true
	})
	return used, nil
}
