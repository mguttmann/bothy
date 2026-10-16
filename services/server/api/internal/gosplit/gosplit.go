// SPDX-License-Identifier: Apache-2.0

// Package gosplit cuts a Go file that oapi-codegen wrote into parts that each stay below
// repo.max_file_kb (docs/spec/23-qualitaet.md; the hook check-added-large-files holds the limit).
// The generators of api (api/routes/gen for the server, api/restclient/gen for the client) use
// it. Declarations move byte for byte; every part keeps the header up to the package clause (build
// constraint, generated-code marker), imports only the packages it uses and is gofmt-formatted.
//
// Example:
//
//	parts, err := gosplit.Chunks(src, 1000*1024)
//	// parts[0] holds the first declarations, every part is valid Go of the same package
package gosplit

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

// File is a parsed Go file with the offsets the split needs.
type File struct {
	// AST is the parsed file.
	AST *ast.File
	// Fset holds the positions of AST.
	Fset *token.FileSet
	// Src is the source.
	Src []byte
	// Header is the source before the package clause.
	Header []byte
	// BodyStart is the offset after the package clause and the import declarations.
	BodyStart int
}

// Parse parses src (name is used in errors). Errors: src is not valid Go.
func Parse(name string, src []byte) (*File, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("split %s: %w", name, err)
	}
	bodyStart := fset.Position(f.Name.End()).Offset
	for _, d := range f.Decls {
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
			bodyStart = fset.Position(g.End()).Offset
		}
	}
	return &File{AST: f, Fset: fset, Src: src, Header: src[:fset.Position(f.Package).Offset], BodyStart: bodyStart}, nil
}

// DeclStart returns the offset where d starts, including its doc comment.
func (f *File) DeclStart(d ast.Decl) int {
	start := d.Pos()
	switch x := d.(type) {
	case *ast.GenDecl:
		if x.Doc != nil {
			start = x.Doc.Pos()
		}
	case *ast.FuncDecl:
		if x.Doc != nil {
			start = x.Doc.Pos()
		}
	}
	return f.Fset.Position(start).Offset
}

// Assemble writes the header, the package clause, the imports of f that body uses and body, and
// formats the result. Errors: body is not valid Go, or an import has a name that cannot be derived
// from its path.
func (f *File) Assemble(body []byte) ([]byte, error) {
	used, err := usedPackages(f.AST.Name.Name, body)
	if err != nil {
		return nil, err
	}
	std, other, err := importLines(f.AST.Imports, used)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.Write(f.Header)
	fmt.Fprintf(&buf, "package %s\n\n", f.AST.Name.Name)
	writeImports(&buf, std, other)
	buf.Write(body)
	out, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, errors.Join(errors.New("split: a part is not valid Go"), err)
	}
	return out, nil
}

// Chunks splits src into parts of at most maxBytes each, cutting only between top-level
// declarations, in source order. A declaration larger than maxBytes on its own is an error, as is
// src that is not valid Go. The first part always exists, also for a file without declarations.
func Chunks(src []byte, maxBytes int) ([][]byte, error) {
	f, err := Parse("source", src)
	if err != nil {
		return nil, err
	}
	var starts []int
	for _, d := range f.AST.Decls {
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
			continue
		}
		starts = append(starts, f.DeclStart(d))
	}
	starts = append(starts, len(src))
	// overhead is what every part carries besides its declarations: header, package clause and
	// at most every import of the file.
	overhead := f.BodyStart
	var parts [][]byte
	from, inPart := f.BodyStart, 0
	for i := 0; i+1 < len(starts); i++ {
		if starts[i+1]-from+overhead > maxBytes && inPart > 0 {
			part, err := f.Assemble(src[from:starts[i]])
			if err != nil {
				return nil, err
			}
			parts = append(parts, part)
			from, inPart = starts[i], 0
		}
		if starts[i+1]-from+overhead > maxBytes {
			return nil, fmt.Errorf("split: a declaration is larger than %d bytes", maxBytes)
		}
		inPart++
	}
	part, err := f.Assemble(src[from:])
	if err != nil {
		return nil, err
	}
	return append(parts, part), nil
}

// importName matches an import path element that is its package name.
var importName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

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
			return nil, nil, fmt.Errorf("split: cannot tell which part uses the import %s", s.Path.Value)
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
	f, err := parser.ParseFile(token.NewFileSet(), "part.go", src, 0)
	if err != nil {
		return nil, fmt.Errorf("split: %w", err)
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
