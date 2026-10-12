// SPDX-License-Identifier: Apache-2.0

package txn_test

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// inUnit returns a context inside a new unit in the work phase.
func inUnit(reg *txn.Registry) (context.Context, *txn.Unit) {
	u, setPhase := txn.NewUnit(reg, txn.Options{})
	setPhase(txn.PhaseWork)
	return txn.WithUnit(context.Background(), u), u
}

// otherKey is a context key that is not the key of kernel/txn.
type otherKey struct{}

// testEvent is an event type of this test package; its publisher is kernel.
type testEvent struct{}

func mustOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestPhase_String(t *testing.T) {
	t.Parallel()
	want := map[txn.Phase]string{
		txn.PhaseNone: "none", txn.PhaseRightsBegin: "rights_begin", txn.PhaseWork: "work",
		txn.PhaseRightsCheck: "rights_check", txn.PhaseSync: "sync", txn.PhaseAudit: "audit",
	}
	for p, s := range want {
		if p.String() != s {
			t.Errorf("Phase(%d).String() = %q, want %q", p, p.String(), s)
		}
	}
	if got := txn.Phase(42).String(); got != "phase(42)" {
		t.Errorf("unknown phase = %q", got)
	}
}

func TestPublish_OutsideUnit(t *testing.T) {
	t.Parallel()
	err := txn.Publish(context.Background(), testEvent{})
	if errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Publish outside a unit = %v, want kind internal", err)
	}
}

func TestPublish_UnsealedRegistry(t *testing.T) {
	t.Parallel()
	reg := txn.NewRegistry()
	ctx, _ := inUnit(reg)
	if err := txn.Publish(ctx, testEvent{}); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Publish with an unsealed registry = %v, want kind internal", err)
	}
}

func TestRegistry_SetHookPhaseNotAllowed(t *testing.T) {
	t.Parallel()
	reg := txn.NewRegistry()
	noop := txn.Hook(func(context.Context) error { return nil })
	for _, p := range []txn.Phase{txn.PhaseNone, txn.PhaseWork, txn.Phase(42)} {
		if err := reg.SetHook(p, noop); err == nil {
			t.Errorf("SetHook(%s) was accepted", p)
		}
	}
	if err := reg.SetHook(txn.PhaseAudit, nil); err == nil {
		t.Error("a nil hook was accepted")
	}
	if err := txn.Subscribe[testEvent](reg, nil); err == nil {
		t.Error("a nil handler was accepted")
	}
}

func TestRegistry_RejectsUnderivableHandler(t *testing.T) {
	t.Parallel()
	reg := txn.NewRegistry()
	// A closure of this test package belongs to kernel, which owns no hooks and no subscribers;
	// a closure in internal/wire is rejected by the same rule (see TestModuleOfSymbol).
	if err := reg.SetHook(txn.PhaseSync, func(context.Context) error { return nil }); err == nil {
		t.Error("a hook without an owning module was accepted")
	}
	if err := txn.Subscribe(reg, func(context.Context, testEvent) error { return nil }); err == nil {
		t.Error("a handler without an owning module was accepted")
	}
	// Events need a named type in a module or in kernel.
	if err := txn.Subscribe(reg, func(context.Context, *testEvent) error { return nil }); err == nil {
		t.Error("an unnamed event type was accepted")
	}
	if err := txn.Subscribe(reg, func(context.Context, string) error { return nil }); err == nil {
		t.Error("an event type without package was accepted")
	}
	if len(reg.Registrations()) != 0 {
		t.Fatalf("rejected registrations were recorded: %v", reg.Registrations())
	}
}

func TestModuleOfSymbol(t *testing.T) {
	t.Parallel()
	const root = "example.com/bothy/services/server"
	cases := map[string]string{
		root + "/internal/chat/app.(*Service).OnAccessChanged-fm": "chat",
		root + "/internal/runs/app.Handler[...].func1":            "runs",
		root + "/internal/sync.(*Hooks).Sync-fm":                  "sync",
		root + "/internal/authz/adapters/postgres.Begin":          "authz",
		root + "/internal/wire.registerTxn.func1":                 "",
		root + "/internal/platform/pg.(*UnitOfWork).Do":           "",
		root + "/internal/kernel/txn_test.TestX.func1":            "kernel",
		root + "/cmd/bothy.main.func1":                            "",
		"main.handler":                                            "",
		"":                                                        "",
	}
	for sym, want := range cases {
		if got := txn.ModuleOfSymbol(sym); got != want {
			t.Errorf("ModuleOfSymbol(%q) = %q, want %q", sym, got, want)
		}
	}
}

func TestKey_OutsideUnit(t *testing.T) {
	t.Parallel()
	k := txn.NewKey[int]("count")
	if err := txn.Store(context.Background(), k, 1); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Store outside a unit = %v, want kind internal", err)
	}
	if v, ok := txn.Load(context.Background(), k); ok || v != 0 {
		t.Fatalf("Load outside a unit = %v, %v", v, ok)
	}
}

func TestKey_StoreLoadPerUnit(t *testing.T) {
	t.Parallel()
	reg := txn.NewRegistry()
	reg.Seal()
	a, b := txn.NewKey[[]string]("buffer"), txn.NewKey[[]string]("buffer")
	ctx, _ := inUnit(reg)
	mustOK(t, txn.Store(ctx, a, []string{"x"}))
	if v, ok := txn.Load(ctx, a); !ok || !slices.Equal(v, []string{"x"}) {
		t.Fatalf("Load = %v, %v", v, ok)
	}
	if _, ok := txn.Load(ctx, b); ok {
		t.Fatal("two keys with the same name share a value")
	}
	other, _ := inUnit(reg)
	if _, ok := txn.Load(other, a); ok {
		t.Fatal("a value leaked into another unit")
	}
	if a.Name() != "buffer" {
		t.Fatalf("Name = %q", a.Name())
	}
}

func TestContextHelpers(t *testing.T) {
	t.Parallel()
	bg := context.Background()
	if txn.InUnit(bg) || txn.CurrentPhase(bg) != txn.PhaseNone {
		t.Fatal("a plain context is inside a unit")
	}
	if _, ok := txn.AffectedPrincipals(bg); ok {
		t.Fatal("AffectedPrincipals outside a unit reports a rights change")
	}
	reg := txn.NewRegistry()
	ctx, u := inUnit(reg)
	if !txn.InUnit(ctx) || txn.CurrentPhase(ctx) != txn.PhaseWork {
		t.Fatal("the unit is not visible")
	}
	if got, ok := txn.UnitFrom(ctx); !ok || got != u {
		t.Fatal("UnitFrom does not return the unit")
	}
	if _, ok := txn.AffectedPrincipals(ctx); ok {
		t.Fatal("a unit without RightsChange reports one")
	}
	p1, p2 := ids.New(), ids.New()
	u, setPhase := txn.NewUnit(reg, txn.ApplyOptions(txn.RightsChange(p1), txn.RightsChange(p2, p1)))
	ctx = txn.WithUnit(context.Background(), u)
	if txn.CurrentPhase(ctx) != txn.PhaseNone {
		t.Fatal("a new unit is not in phase none")
	}
	got, ok := txn.AffectedPrincipals(ctx)
	if !ok || !slices.Equal(got, []ids.ID{p1, p2}) {
		t.Fatalf("AffectedPrincipals = %v, %v; want [p1 p2] without duplicates", got, ok)
	}
	got[0] = ids.ID{}
	if again, _ := txn.AffectedPrincipals(ctx); again[0] != p1 {
		t.Fatal("AffectedPrincipals returns the internal slice")
	}
	if o := u.Options(); !o.RightsChange || !slices.Equal(o.Affected, []ids.ID{p1, p2}) || u.Registry() != reg {
		t.Fatalf("Options = %+v", o)
	}
	u.Options().Affected[0] = ids.ID{}
	if u.Options().Affected[0] != p1 {
		t.Fatal("Options returns the internal slice")
	}
	setPhase(txn.PhaseSync)
	if txn.CurrentPhase(ctx) != txn.PhaseSync {
		t.Fatal("the phase is not read from the unit")
	}
	if _, ok := txn.UnitFrom(context.WithValue(context.Background(), otherKey{}, u)); ok {
		t.Fatal("UnitFrom found a unit under a foreign key")
	}
}

func TestOptions(t *testing.T) {
	t.Parallel()
	o := txn.ApplyOptions()
	if o.RightsChange || len(o.Affected) != 0 {
		t.Fatalf("no options = %+v", o)
	}
	o = txn.ApplyOptions(txn.RightsChange())
	if !o.RightsChange {
		t.Fatal("RightsChange without principals is not a rights change")
	}
	p := ids.New()
	outer := txn.ApplyOptions(txn.RightsChange(p))
	if !outer.Covers(txn.ApplyOptions(txn.RightsChange(p))) || !outer.Covers(txn.ApplyOptions()) ||
		!outer.Covers(txn.ApplyOptions(txn.RightsChange())) {
		t.Error("an outer RightsChange does not cover its subset")
	}
	if outer.Covers(txn.ApplyOptions(txn.RightsChange(ids.New()))) {
		t.Error("an outer RightsChange covers a principal it does not name")
	}
	if txn.ApplyOptions().Covers(txn.ApplyOptions(txn.RightsChange())) {
		t.Error("a unit without RightsChange covers a nested one")
	}
	if !reflect.DeepEqual(txn.ApplyOptions(nil), txn.Options{}) {
		t.Error("a nil option is not ignored")
	}
}
