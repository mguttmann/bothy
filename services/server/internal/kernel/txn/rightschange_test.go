// SPDX-License-Identifier: Apache-2.0

package txn_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// TestDeclareRightsChange_CollectsTheDeclaration: the option carries action, resource, principals
// and targets of a rights change (02 section 8.1, brief M1-29 scope 1); RightsChange adds
// principals to it.
func TestDeclareRightsChange_CollectsTheDeclaration(t *testing.T) {
	t.Parallel()
	p, q := ids.New(), ids.New()
	ch := txn.Ref{Kind: "channel", ID: ids.New()}
	res := txn.Ref{Kind: "grant", ID: ids.New()}
	o := txn.ApplyOptions(
		txn.DeclareRightsChange(txn.RightsChangeSpec{Action: "grant.create", Resource: res, Principals: []ids.ID{p}, Targets: []txn.Ref{ch}}),
		txn.RightsChange(q, p),
	)
	if !o.RightsChange || o.Action != "grant.create" || o.Resource != res {
		t.Fatalf("options %+v", o)
	}
	if !reflect.DeepEqual(o.Affected, []ids.ID{p, q}) || !reflect.DeepEqual(o.Targets, []txn.Ref{ch}) {
		t.Fatalf("affected %v targets %v", o.Affected, o.Targets)
	}
	spec := o.Spec()
	if spec.Action != "grant.create" || spec.Resource != res || !reflect.DeepEqual(spec.Principals, []ids.ID{p, q}) ||
		!reflect.DeepEqual(spec.Targets, []txn.Ref{ch}) {
		t.Fatalf("spec %+v", spec)
	}
}

// TestCovers_TargetsAndAction: a nested unit joins only when the outer declaration names every
// target and the same action.
func TestCovers_TargetsAndAction(t *testing.T) {
	t.Parallel()
	a, b := txn.Ref{Kind: "space", ID: ids.New()}, txn.Ref{Kind: "channel", ID: ids.New()}
	outer := txn.ApplyOptions(txn.DeclareRightsChange(txn.RightsChangeSpec{Action: "channel.archive", Targets: []txn.Ref{a}}))
	if !outer.Covers(txn.ApplyOptions(txn.DeclareRightsChange(txn.RightsChangeSpec{Targets: []txn.Ref{a}}))) {
		t.Error("the same target is not covered")
	}
	if outer.Covers(txn.ApplyOptions(txn.DeclareRightsChange(txn.RightsChangeSpec{Targets: []txn.Ref{b}}))) {
		t.Error("another target is covered")
	}
	if outer.Covers(txn.ApplyOptions(txn.DeclareRightsChange(txn.RightsChangeSpec{Action: "space.archive"}))) {
		t.Error("another action is covered")
	}
}

// TestRightsChangeOf: the hooks read the declaration of the outermost unit; outside a unit or
// without a rights change there is none.
func TestRightsChangeOf(t *testing.T) {
	t.Parallel()
	if _, ok := txn.RightsChangeOf(context.Background()); ok || txn.InRightsChange(context.Background()) {
		t.Fatal("a declaration outside a unit")
	}
	reg := txn.NewRegistry()
	plain, _ := txn.NewUnit(reg, txn.Options{})
	if txn.InRightsChange(txn.WithUnit(context.Background(), plain)) {
		t.Fatal("a unit without RightsChange reports one")
	}
	p := ids.New()
	u, _ := txn.NewUnit(reg, txn.ApplyOptions(txn.DeclareRightsChange(txn.RightsChangeSpec{Action: "group.delete", Principals: []ids.ID{p}})))
	ctx := txn.WithUnit(context.Background(), u)
	spec, ok := txn.RightsChangeOf(ctx)
	if !ok || !txn.InRightsChange(ctx) || spec.Action != "group.delete" || !reflect.DeepEqual(spec.Principals, []ids.ID{p}) {
		t.Fatalf("spec %+v ok %v", spec, ok)
	}
	spec.Principals[0] = ids.New()
	if again, _ := txn.RightsChangeOf(ctx); again.Principals[0] != p {
		t.Fatal("RightsChangeOf returns shared slices")
	}
}
