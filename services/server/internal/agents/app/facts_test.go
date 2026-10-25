// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/adapters/agentstest"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestRunFacts: RunFacts returns handle, status, backend policy and target tool class of the
// agents among the ids (M2-05); the holder is set exactly for a personal agent, and people and
// unknown ids are absent. ComponentsAllowed is false exactly for an agent whose operator_config
// sets supports_components to false (M2-15). VoicePermitted is the owner's supports_voice (nil when
// absent) and VoiceReported the persisted heartbeat report capabilities.voice_input (M3-09).
func TestRunFacts(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	robin := w.person("robin", false)
	lotse := w.create(owner, "lotse")
	scout := w.create(owner, "scout")
	quiet, err := w.svc.Create(ctx, owner, agents.CreateCmd{Handle: "quiet", DisplayName: "quiet", AdapterKind: "hermes",
		OperatorConfig: map[string]any{"supports_components": false, "supports_voice": false}})
	if err != nil {
		t.Fatal(err)
	}
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
	yes := true
	agentstest.ReportCapabilities(t, w.owner, lotse.ID, domain.Capabilities{VoiceInput: &yes})
	agentstest.ReportCapabilities(t, w.owner, quiet.ID, domain.Capabilities{VoiceInput: &yes})
	var facts agents.Facts = w.svc
	got, err := facts.RunFacts(ctx, []ids.ID{lotse.ID, scout.ID, quiet.ID, robin.ID, ids.New()})
	if err != nil {
		t.Fatal(err)
	}
	// The pointer and report of the voice capability, checked apart from the comparable rest.
	for id, c := range map[ids.ID]struct {
		permitted *bool
		reported  bool
	}{lotse.ID: {nil, true}, scout.ID: {nil, false}, quiet.ID: {new(bool), true}} {
		f := got[id]
		if (f.VoicePermitted == nil) != (c.permitted == nil) || (f.VoicePermitted != nil && *f.VoicePermitted != *c.permitted) ||
			f.VoiceReported != c.reported {
			t.Fatalf("RunFacts[%s] voice %v %v, want %v %v", id, f.VoicePermitted, f.VoiceReported, c.permitted, c.reported)
		}
		f.VoicePermitted, f.VoiceReported = nil, false
		got[id] = f
	}
	want := map[ids.ID]agents.RunFacts{
		lotse.ID: {ID: lotse.ID, Handle: "lotse", Status: "active", BillingMode: agents.BillingPersonal, CredentialHolderID: owner.ID, ToolClass: agents.ToolHost,
			ComponentsAllowed: true},
		scout.ID: {ID: scout.ID, Handle: "scout", Status: "disabled", BillingMode: agents.BillingOrganization, ToolClass: agents.ToolChat,
			ComponentsAllowed: true},
		quiet.ID: {ID: quiet.ID, Handle: "quiet", Status: "active", BillingMode: agents.BillingOrganization, ToolClass: agents.ToolChat},
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
