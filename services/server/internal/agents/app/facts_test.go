// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestRunFacts: RunFacts returns handle, status, backend policy and target tool class of the
// agents among the ids (M2-05); the holder is set exactly for a personal agent, and people and
// unknown ids are absent.
func TestRunFacts(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	robin := w.person("robin", false)
	lotse := w.create(owner, "lotse")
	scout := w.create(owner, "scout")
	if _, err := w.svc.SetBillingMode(ctx, owner, lotse.ID, agents.BillingModeCmd{
		BillingMode: agents.BillingPersonal, CredentialHolderID: owner.ID, Reason: "Own use", IfMatch: match(1),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.SetToolClass(ctx, owner, lotse.ID, agents.ToolClassCmd{ToolClass: agents.ToolHost, Reason: "Shell", IfMatch: match(2)}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.Disable(ctx, owner, scout.ID, "Paused"); err != nil {
		t.Fatal(err)
	}
	var facts agents.Facts = w.svc
	got, err := facts.RunFacts(ctx, []ids.ID{lotse.ID, scout.ID, robin.ID, ids.New()})
	if err != nil {
		t.Fatal(err)
	}
	want := map[ids.ID]agents.RunFacts{
		lotse.ID: {ID: lotse.ID, Handle: "lotse", Status: "active", BillingMode: agents.BillingPersonal, CredentialHolderID: owner.ID, ToolClass: agents.ToolHost},
		scout.ID: {ID: scout.ID, Handle: "scout", Status: "disabled", BillingMode: agents.BillingOrganization, ToolClass: agents.ToolChat},
	}
	if len(got) != len(want) {
		t.Fatalf("RunFacts = %+v, want %+v", got, want)
	}
	for id, f := range want {
		if got[id] != f {
			t.Fatalf("RunFacts[%s] = %+v, want %+v", id, got[id], f)
		}
	}
	if empty, err := facts.RunFacts(ctx, nil); err != nil || len(empty) != 0 {
		t.Fatalf("RunFacts empty = %+v, %v", empty, err)
	}
}
