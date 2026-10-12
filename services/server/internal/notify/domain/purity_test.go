// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

const modulePath = "github.com/OWNER/bothy/services/server"

// allowedImports are the standard library packages without I/O and kernel-core packages the
// production code of this package may import (02 5 "Layers within a module", PLAN row M3-17).
var allowedImports = map[string]bool{
	"time": true, "slices": true, "cmp": true, "math": true, "strconv": true,
	"strings": true, "fmt": true, "errors": true,
	modulePath + "/internal/kernel/ids":  true,
	modulePath + "/internal/kernel/errs": true,
}

// forbiddenTime are the names of package time that read the clock, wait, load a zone or use the
// zone of the host.
var forbiddenTime = map[string]bool{
	"Now": true, "Since": true, "Until": true, "Sleep": true, "After": true, "Tick": true,
	"NewTimer": true, "NewTicker": true, "AfterFunc": true, "LoadLocation": true, "LoadLocationFromTZData": true,
	"Local": true,
}

// allowedFmt are the functions of package fmt that format into a value and do no I/O.
var allowedFmt = map[string]bool{"Sprintf": true, "Sprint": true, "Sprintln": true, "Errorf": true, "Append": true, "Appendf": true, "Appendln": true}

// TestDomainPurity parses the production files of the package and rejects any import outside the
// list and any use of the clock or the zone loader.
func TestDomainPurity(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	files := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files++
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		checkFile(t, fset, f)
	}
	if files < 7 {
		t.Fatalf("found %d production files, want at least 7 (doc, kinds, level, quiet, decide, flush, delay)", files)
	}
}

func checkFile(t *testing.T, fset *token.FileSet, f *ast.File) {
	t.Helper()
	timeNames, fmtNames := map[string]bool{}, map[string]bool{}
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if !allowedImports[p] {
			t.Errorf("%s imports %s", fset.Position(imp.Pos()), p)
		}
		// A dot or blank import would hide the package name the checks below look for.
		if imp.Name != nil && (imp.Name.Name == "." || imp.Name.Name == "_") {
			t.Errorf("%s imports %s as %s", fset.Position(imp.Pos()), p, imp.Name.Name)
		}
		local := path.Base(p)
		if imp.Name != nil {
			local = imp.Name.Name
		}
		switch p {
		case "time":
			timeNames[local] = true
		case "fmt":
			fmtNames[local] = true
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		// The builtins print and println write to stderr.
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := ast.Unparen(call.Fun).(*ast.Ident); ok && (id.Name == "print" || id.Name == "println") {
				t.Errorf("%s calls the builtin %s, which does I/O", fset.Position(call.Pos()), id.Name)
			}
		}
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		x, isIdent := sel.X.(*ast.Ident)
		switch {
		case isIdent && timeNames[x.Name] && forbiddenTime[sel.Sel.Name]:
			t.Errorf("%s uses time.%s", fset.Position(sel.Pos()), sel.Sel.Name)
		case isIdent && fmtNames[x.Name] && !allowedFmt[sel.Sel.Name]:
			t.Errorf("%s uses fmt.%s, which does I/O", fset.Position(sel.Pos()), sel.Sel.Name)
		case sel.Sel.Name == "Local":
			// t.Local() converts into the zone of the host.
			t.Errorf("%s uses Local, the zone of the host", fset.Position(sel.Pos()))
		}
		return true
	})
}

// TestDecisionCarriesNoText holds ADR 0031 and T-PU-01 (design note 11): every field of the
// decision is one of an allow list of types, the enum types of this package, ids.ID, bool, int64
// and time.Time, or a slice or struct of this package built from them. An allow list instead of
// a deny list, because any other array, slice or number type ([]byte, []rune, [64]uint16) could
// carry text.
func TestDecisionCarriesNoText(t *testing.T) {
	t.Parallel()
	ownPkg := reflect.TypeFor[domain.Kind]().PkgPath()
	leaves := map[reflect.Type]bool{
		reflect.TypeFor[ids.ID]():    true,
		reflect.TypeFor[bool]():      true,
		reflect.TypeFor[int64]():     true,
		reflect.TypeFor[time.Time](): true,
		// The enum types of the package; a new string type must be added here on purpose.
		reflect.TypeFor[domain.Kind]():        true,
		reflect.TypeFor[domain.Level]():       true,
		reflect.TypeFor[domain.Platform]():    true,
		reflect.TypeFor[domain.LevelSource](): true,
		reflect.TypeFor[domain.Outcome]():     true,
		reflect.TypeFor[domain.SkipReason]():  true,
		reflect.TypeFor[domain.Via]():         true,
	}
	var walk func(where string, typ reflect.Type)
	walk = func(where string, typ reflect.Type) {
		switch {
		case leaves[typ]:
		case typ.Kind() == reflect.Slice:
			walk(where+"[]", typ.Elem())
		case typ.Kind() == reflect.Struct && typ.PkgPath() == ownPkg:
			for i := range typ.NumField() {
				fld := typ.Field(i)
				walk(where+"."+fld.Name, fld.Type)
			}
		default:
			t.Errorf("%s is a %s, which is not on the allow list of design note 11", where, typ)
		}
	}
	for _, typ := range []reflect.Type{
		reflect.TypeFor[domain.Decision](),
		reflect.TypeFor[domain.Route](),
		reflect.TypeFor[domain.EffectiveLevel](),
	} {
		walk(typ.Name(), typ)
	}
}
