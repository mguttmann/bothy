// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// serverModule is the module path of services/server.
const serverModule = "github.com/OWNER/bothy/services/server"

// instancePkg is the port package that declares Counters.
const instancePkg = serverModule + "/internal/instance"

// moduleDir returns services/server.
func moduleDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// typedPackage is one package of the module, type-checked from its source.
type typedPackage struct {
	PkgPath   string
	Fset      *token.FileSet
	TypesInfo *types.Info
}

// typeCheck loads patterns with their tests and type-checks every matched package of the module
// (and test variant) from source, one after the other in import order, in one goroutine. Packages
// of the module are never compiled for the scan: under go test -race -coverpkg the build cache
// holds only their instrumented objects, so the former go list -export compiled the whole module
// a second time, and with the type check under the race detector the test ran up to 850 s and
// pushed the package past its timeout (PL/30). Packages outside the module (standard library and
// dependencies) come from their export data, built with the race and tag settings of the running
// test binary, so the build of the test has already cached them. Checking one package at a time
// keeps the go/types race of a parallel check away (Named.unpack against Checker.isComplete when
// two packages expand the same imported generic instance, docs/DECISIONS.md M1-18/17); the
// checked packages and the imported ones are shared, so every package is checked once.
func typeCheck(t *testing.T, patterns ...string) []typedPackage {
	t.Helper()
	flags := testBuildFlags()
	cfg := &packages.Config{
		Mode:       packages.NeedName | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedDeps | packages.NeedModule,
		Dir:        moduleDir(t),
		Tests:      true,
		BuildFlags: flags,
	}
	roots, err := packages.Load(cfg, patterns...)
	if err != nil {
		t.Fatal(err)
	}
	order := moduleOrder(roots)
	for _, p := range order {
		for _, e := range p.Errors {
			t.Fatalf("load %s: %v", p.PkgPath, e)
		}
	}
	fset := token.NewFileSet()
	ext := exportImporter(t, fset, flags, externalImports(order))
	matched := map[string]bool{}
	for _, p := range roots {
		matched[p.ID] = true
	}
	checked := map[string]*types.Package{}
	out := make([]typedPackage, 0, len(roots))
	for _, p := range order {
		files := make([]*ast.File, 0, len(p.CompiledGoFiles))
		for _, name := range p.CompiledGoFiles {
			f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}
			files = append(files, f)
		}
		// Language version and sizes as packages.Load sets them for its own type check.
		conf := types.Config{Importer: graphImporter{p: p, checked: checked, ext: ext}, Sizes: types.SizesFor("gc", runtime.GOARCH)}
		if p.Module != nil && p.Module.GoVersion != "" {
			conf.GoVersion = "go" + p.Module.GoVersion
		}
		info := &types.Info{
			Types:     map[ast.Expr]types.TypeAndValue{},
			Uses:      map[*ast.Ident]types.Object{},
			Instances: map[*ast.Ident]types.Instance{},
		}
		tp, err := conf.Check(p.PkgPath, fset, files, info)
		if err != nil {
			t.Fatalf("type-check %s: %v", p.ID, err)
		}
		checked[p.ID] = tp
		if matched[p.ID] && len(files) > 0 {
			out = append(out, typedPackage{PkgPath: p.PkgPath, Fset: fset, TypesInfo: info})
		}
	}
	return out
}

// testBuildFlags returns the -race and -tags settings the running test binary was built with, so
// the export data of the packages outside the module is the one the build of the test cached.
func testBuildFlags() []string {
	var flags []string
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	for _, s := range bi.Settings {
		switch {
		case s.Key == "-race" && s.Value == "true":
			flags = append(flags, "-race")
		case s.Key == "-tags" && s.Value != "":
			flags = append(flags, "-tags="+s.Value)
		}
	}
	return flags
}

// inModule reports whether p belongs to services/server.
func inModule(p *packages.Package) bool {
	return p.Module != nil && p.Module.Path == serverModule
}

// moduleOrder returns the packages of the module reachable from roots (test variants and the
// generated test mains excluded) with every package after the packages it imports.
func moduleOrder(roots []*packages.Package) []*packages.Package {
	var order []*packages.Package
	seen := map[string]bool{}
	var visit func(p *packages.Package)
	visit = func(p *packages.Package) {
		if seen[p.ID] || !inModule(p) || strings.HasSuffix(p.ID, ".test") {
			return
		}
		seen[p.ID] = true
		paths := make([]string, 0, len(p.Imports))
		for path := range p.Imports {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			visit(p.Imports[path])
		}
		order = append(order, p)
	}
	for _, p := range roots {
		visit(p)
	}
	return order
}

// externalImports returns the sorted import paths outside the module that the packages of order
// import directly.
func externalImports(order []*packages.Package) []string {
	set := map[string]bool{}
	for _, p := range order {
		for path, dep := range p.Imports {
			if !inModule(dep) && path != "unsafe" {
				set[path] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(set))
}

// exportImporter imports the packages outside the module from their export data (go list -export
// of exactly those packages and their dependencies, never a package of the module); one importer
// serves the whole check, so each of them is read once.
func exportImporter(t *testing.T, fset *token.FileSet, flags, paths []string) types.Importer {
	t.Helper()
	exports := map[string]string{}
	if len(paths) > 0 {
		cfg := &packages.Config{
			Mode:       packages.NeedName | packages.NeedExportFile | packages.NeedDeps | packages.NeedImports | packages.NeedModule,
			Dir:        moduleDir(t),
			BuildFlags: flags,
		}
		pkgs, err := packages.Load(cfg, paths...)
		if err != nil {
			t.Fatal(err)
		}
		packages.Visit(pkgs, nil, func(p *packages.Package) {
			if inModule(p) {
				t.Errorf("export data requested for %s of the module", p.ID)
			}
			for _, e := range p.Errors {
				t.Errorf("load %s: %v", p.PkgPath, e)
			}
			exports[p.PkgPath] = p.ExportFile
		})
	}
	return importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		file := exports[path]
		if file == "" {
			return nil, fmt.Errorf("no export data for import %q", path)
		}
		return os.Open(file) //nolint:gosec // G304: the path is an export file go list reported for this build
	})
}

// graphImporter resolves the imports of p: a package of the module (or its test variant) to the
// package checked before it, every other one through the shared export data importer.
type graphImporter struct {
	p       *packages.Package
	checked map[string]*types.Package
	ext     types.Importer
}

func (g graphImporter) Import(path string) (*types.Package, error) {
	if path == "unsafe" {
		return types.Unsafe, nil
	}
	dep, ok := g.p.Imports[path]
	if !ok {
		return nil, fmt.Errorf("%s: unknown import %q", g.p.ID, path)
	}
	if !inModule(dep) {
		return g.ext.Import(path)
	}
	if tp := g.checked[dep.ID]; tp != nil {
		return tp, nil
	}
	return nil, fmt.Errorf("%s: import %s not checked before it", g.p.ID, dep.ID)
}

// countersRefs type-checks patterns with their tests (typeCheck) and returns, per package path
// (an external test package counts as its package), the positions where the package refers to
// instance.Counters: the type name or instance.CounterName with its constants, a method of the
// interface, an expression whose type is or contains Counters (a value passed on without naming
// the type), or a generic instantiation whose type arguments contain it.
func countersRefs(t *testing.T, patterns ...string) map[string][]string {
	t.Helper()
	pkgs := typeCheck(t, patterns...)
	refs := map[string][]string{}
	for _, p := range pkgs {
		owner := strings.TrimSuffix(p.PkgPath, "_test")
		add := func(pos token.Pos) {
			if at := p.Fset.Position(pos).String(); !slices.Contains(refs[owner], at) {
				refs[owner] = append(refs[owner], at)
			}
		}
		for id, obj := range p.TypesInfo.Uses {
			if isCountersObject(obj) {
				add(id.Pos())
			}
		}
		for expr, tv := range p.TypesInfo.Types {
			if mentionsCounters(tv.Type, nil) {
				add(expr.Pos())
			}
		}
		// A generic function or type instantiated with Counters (or a type built from it) uses
		// the port without naming it.
		for id, inst := range p.TypesInfo.Instances {
			for arg := range inst.TypeArgs.Types() {
				if mentionsCounters(arg, nil) {
					add(id.Pos())
				}
			}
		}
	}
	return refs
}

// isCountersType reports whether ty is the named interface instance.Counters.
func isCountersType(ty types.Type) bool {
	n, ok := types.Unalias(ty).(*types.Named)
	if !ok {
		return false
	}
	o := n.Obj()
	return o.Pkg() != nil && o.Pkg().Path() == instancePkg && o.Name() == "Counters"
}

// mentionsCounters reports whether ty is instance.Counters or is built from it: a pointer,
// slice, array, map, channel, function signature, struct field or type argument that contains
// it. seen stops the walk at recursive named types.
func mentionsCounters(ty types.Type, seen map[types.Type]bool) bool {
	if ty == nil || isCountersType(ty) {
		return ty != nil
	}
	if seen == nil {
		seen = map[types.Type]bool{}
	}
	if seen[ty] {
		return false
	}
	seen[ty] = true
	switch t := types.Unalias(ty).(type) {
	case *types.Pointer:
		return mentionsCounters(t.Elem(), seen)
	case *types.Slice:
		return mentionsCounters(t.Elem(), seen)
	case *types.Array:
		return mentionsCounters(t.Elem(), seen)
	case *types.Chan:
		return mentionsCounters(t.Elem(), seen)
	case *types.Map:
		return mentionsCounters(t.Key(), seen) || mentionsCounters(t.Elem(), seen)
	case *types.Signature:
		return mentionsCounters(t.Params(), seen) || mentionsCounters(t.Results(), seen)
	case *types.Tuple:
		for v := range t.Variables() {
			if mentionsCounters(v.Type(), seen) {
				return true
			}
		}
	case *types.Struct:
		for f := range t.Fields() {
			if mentionsCounters(f.Type(), seen) {
				return true
			}
		}
	case *types.Named:
		for arg := range t.TypeArgs().Types() {
			if mentionsCounters(arg, seen) {
				return true
			}
		}
	}
	return false
}

// isCounterName reports whether ty is instance.CounterName.
func isCounterName(ty types.Type) bool {
	n, ok := types.Unalias(ty).(*types.Named)
	if !ok {
		return false
	}
	o := n.Obj()
	return o.Pkg() != nil && o.Pkg().Path() == instancePkg && o.Name() == "CounterName"
}

// isCountersObject reports the type name instance.Counters, its interface methods, and the type
// instance.CounterName with its constants: an interface of the same shape declared elsewhere
// must name CounterName in its method signatures, so naming it marks a counters user as well.
func isCountersObject(obj types.Object) bool {
	switch o := obj.(type) {
	case *types.Const:
		return isCounterName(o.Type())
	case *types.TypeName:
		return isCountersType(o.Type()) || isCounterName(o.Type())
	case *types.Func:
		sig, ok := o.Type().(*types.Signature)
		return ok && sig.Recv() != nil && isCountersType(sig.Recv().Type())
	default:
		return false
	}
}

// handlerPackage returns the Go package of a runtime function name, for example
// ".../internal/sync/app" for ".../internal/sync/app.(*Hooks).Sync-fm".
func handlerPackage(name string) string {
	slash := strings.LastIndex(name, "/")
	if slash < 0 {
		return ""
	}
	dot := strings.Index(name[slash:], ".")
	if dot < 0 {
		return ""
	}
	return name[:slash+dot]
}

// allowedCountersPackages are internal/instance with its subpackages, the package internal/wire
// itself (not its subpackages) and the packages of the hook handlers of the real registry
// (02 "Other checks in the gate", "Counter access").
func allowedCountersPackages(t *testing.T) func(string) bool {
	t.Helper()
	reg, err := wire.NewTxnRegistry(wire.NewModules(nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	hooks := map[string]bool{}
	for _, r := range reg.Registrations() {
		if r.Kind == txn.KindHook {
			hooks[handlerPackage(r.Handler)] = true
		}
	}
	return func(pkg string) bool {
		return pkg == instancePkg || strings.HasPrefix(pkg, instancePkg+"/") ||
			pkg == serverModule+"/internal/wire" || hooks[pkg]
	}
}

// violations returns the packages of refs that allowed rejects, sorted.
func violations(refs map[string][]string, allowed func(string) bool) []string {
	var out []string
	for pkg := range refs {
		if !allowed(pkg) {
			out = append(out, pkg)
		}
	}
	sort.Strings(out)
	return out
}

// TestCountersOnlyUsedByPhaseHooks holds 02 "Lock order" statically: only instance, wire
// and the packages of the registered phase hooks of authz, sync and audit refer to
// instance.Counters. With the runtime phase guard of the implementation this keeps every use case
// away from the counter rows.
func TestCountersOnlyUsedByPhaseHooks(t *testing.T) {
	t.Parallel()
	refs := countersRefs(t, "./...")
	if len(refs[instancePkg+"/app"]) == 0 || len(refs[serverModule+"/internal/wire"]) == 0 {
		t.Fatalf("the scan did not see the implementation in instance/app and the constructor in wire: %v", refs)
	}
	if bad := violations(refs, allowedCountersPackages(t)); len(bad) > 0 {
		for _, pkg := range bad {
			t.Errorf("%s refers to instance.Counters (%s); only instance, wire and the registered phase hooks may", pkg, strings.Join(refs[pkg], ", "))
		}
	}
}

// TestCountersOnlyUsedByPhaseHooks_ReportsFixtures: the check reports a package that names
// Counters, one that only passes a Counters value on, and one with an interface of the same
// shape.
func TestCountersOnlyUsedByPhaseHooks_ReportsFixtures(t *testing.T) {
	t.Parallel()
	refs := countersRefs(t, "./internal/wire/testdata/countersuser", "./internal/wire/testdata/countersflow",
		"./internal/wire/testdata/countersshape", "./internal/wire/testdata/countersgeneric")
	want := []string{
		serverModule + "/internal/wire/testdata/countersflow",
		serverModule + "/internal/wire/testdata/countersgeneric",
		serverModule + "/internal/wire/testdata/countersshape",
		serverModule + "/internal/wire/testdata/countersuser",
	}
	if got := violations(refs, allowedCountersPackages(t)); !slices.Equal(got, want) {
		t.Fatalf("violations = %v, want %v (refs %v)", got, want, refs)
	}
}

func TestHandlerPackage(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]string{
		serverModule + "/internal/sync/app.(*Hooks).Sync-fm": serverModule + "/internal/sync/app",
		serverModule + "/internal/audit.Hook":                serverModule + "/internal/audit",
		"main.f":                                             "", "": "",
	} {
		if got := handlerPackage(name); got != want {
			t.Errorf("handlerPackage(%q) = %q, want %q", name, got, want)
		}
	}
}

// TestCountersScanBuildsNothingOfTheModule holds the cost of the scan (PL/30): the packages of the
// module, test variants included, are checked from source in import order and only packages
// outside the module come from export data, built with the race setting of the test binary, so
// the scan never compiles the module a second time.
func TestCountersScanBuildsNothingOfTheModule(t *testing.T) {
	t.Parallel()
	if got := slices.Contains(testBuildFlags(), "-race"); got != testkit.RaceEnabled {
		t.Fatalf("testBuildFlags() = %v, race detector %v", testBuildFlags(), testkit.RaceEnabled)
	}
	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedDeps | packages.NeedModule,
		Dir:   moduleDir(t),
		Tests: true,
	}
	roots, err := packages.Load(cfg, "./internal/instance")
	if err != nil {
		t.Fatal(err)
	}
	order := moduleOrder(roots)
	pos := map[string]int{}
	for i, p := range order {
		pos[p.ID] = i
	}
	for _, id := range []string{instancePkg, instancePkg + "_test [" + instancePkg + ".test]"} {
		if _, ok := pos[id]; !ok {
			t.Errorf("order misses %s", id)
		}
	}
	for _, p := range order {
		for _, dep := range p.Imports {
			if j, ok := pos[dep.ID]; ok && j > pos[p.ID] {
				t.Errorf("%s comes before its import %s", p.ID, dep.ID)
			}
		}
	}
	ext := externalImports(order)
	for _, path := range ext {
		if path == serverModule || strings.HasPrefix(path, serverModule+"/") {
			t.Errorf("export data requested for %s of the module", path)
		}
	}
	if !slices.Contains(ext, "context") {
		t.Errorf("externalImports = %v, want the standard library imports of instance", ext)
	}
}
