// SPDX-License-Identifier: Apache-2.0

package txn_test

import (
	"context"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// TestControlledUnit_Declare: the control moves the phase and replaces the declaration once a
// late rights change begins (M1-31); readers of the unit see the new declaration.
func TestControlledUnit_Declare(t *testing.T) {
	t.Parallel()
	reg := txn.NewRegistry()
	reg.Seal()
	o := txn.ApplyOptions(txn.AllowLateRightsChange())
	if !o.LateRightsChange || o.RightsChange {
		t.Fatalf("options %+v", o)
	}
	u, ctrl := txn.NewControlledUnit(reg, o)
	ctx := txn.WithUnit(context.Background(), u)
	ctrl.SetPhase(txn.PhaseWork)
	if txn.CurrentPhase(ctx) != txn.PhaseWork || txn.InRightsChange(ctx) {
		t.Fatal("phase or rights change before Declare")
	}
	p := ids.New()
	declared := u.Options()
	txn.DeclareRightsChange(txn.RightsChangeSpec{Action: "owner.add", Principals: []ids.ID{p}})(&declared)
	ctrl.Declare(declared)
	spec, ok := txn.RightsChangeOf(ctx)
	if !ok || spec.Action != "owner.add" || !slices.Equal(spec.Principals, []ids.ID{p}) {
		t.Fatalf("RightsChangeOf = %+v, %v", spec, ok)
	}
	if got, ok := txn.AffectedPrincipals(ctx); !ok || !slices.Equal(got, []ids.ID{p}) {
		t.Fatalf("AffectedPrincipals = %v, %v", got, ok)
	}
}
