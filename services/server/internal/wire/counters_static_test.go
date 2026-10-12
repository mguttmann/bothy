// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"go/token"
	"go/types"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
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

// countersRefs loads patterns with their tests and returns, per package path (an external test
// package counts as its package), the positions where the package refers to instance.Counters:
// the type name or instance.CounterName with its constants, a method of the interface, an
// expression whose type is or contains Counters (a value passed on without naming the type), or
// a generic instantiation whose type arguments contain it.
func countersRefs(t *testing.T, patterns ...string) map[string][]string {
	t.Helper()
	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
		Dir:   moduleDir(t),
		Tests: true,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		t.Fatal(err)
	}
	refs := map[string][]string{}
	for _, p := range pkgs {
		for _, e := range p.Errors {
			t.Fatalf("load %s: %v", p.PkgPath, e)
		}
		if p.TypesInfo == nil {
			continue
		}
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
// (02 "Weitere Prüfungen im Gate", "Zähler-Zugriff").
func allowedCountersPackages(t *testing.T) func(string) bool {
	t.Helper()
	reg, err := wire.NewTxnRegistry(&wire.Modules{})
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

// TestCountersOnlyUsedByPhaseHooks holds 02 "Sperrreihenfolge" statically: only instance, wire
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
