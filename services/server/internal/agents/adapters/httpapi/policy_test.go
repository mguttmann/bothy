// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// wantI1 checks a 409 i1_violation whose conflicts name exactly the agents in order, each with
// holder and the extra principals.
func wantI1(t *testing.T, p map[string]any, holder ids.ID, agentsAndExtra ...[]ids.ID) {
	t.Helper()
	list, _ := p["conflicts"].([]any)
	if len(list) != len(agentsAndExtra) {
		t.Fatalf("conflicts %v, want %d", p["conflicts"], len(agentsAndExtra))
	}
	for i, want := range agentsAndExtra {
		c := list[i].(map[string]any)
		var extra []any
		for _, id := range want[1:] {
			extra = append(extra, id.String())
		}
		if c["agent_id"] != want[0].String() || c["credential_holder_id"] != holder.String() || !slices.Equal(c["principal_ids"].([]any), extra) {
			t.Fatalf("conflict %d = %v, want agent %s extra %v", i, c, want[0], extra)
		}
	}
}

// unchanged checks that the agent has the policy mode and holder ("" for none) and the version.
func (w *world) unchanged(id ids.ID, mode, holder string, version int64) {
	w.t.Helper()
	m, h := w.policy(id)
	if m != mode || h != holder || w.version(id) != version {
		w.t.Fatalf("agent %s: %s %s v%d, want %s %s v%d", id, m, h, w.version(id), mode, holder, version)
	}
}

// TestAZM22_SwitchToPersonalWithForeignTriggerIsRejected (07 11.1 AZ-M22): Alex has trigger on a
// channel Assistent serves (fake structure); the owner switches Assistent to personal: 409
// i1_violation naming Alex, nothing changed, the deny entry written in its own transaction.
func TestAZM22_SwitchToPersonalWithForeignTriggerIsRejected(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	alex := w.person("alex", "active", false)
	assistent := w.create("assistent")
	w.rawGrant(alex, w.channel(assistent), "trigger")
	version, pv, events := w.version(assistent), w.permissionVersion(), len(w.updates())
	p := w.want(w.do("owner", http.MethodPut, agentPath(assistent)+"/billing-mode",
		`{"billing_mode":"personal","credential_holder_id":"`+owner.String()+`","reason":"Own use"}`, ifMatch(version)...),
		http.StatusConflict, "i1_violation")
	wantI1(t, p, owner, []ids.ID{assistent, alex})
	w.unchanged(assistent, "organization", "", version)
	if w.permissionVersion() != pv || len(w.updates()) != events {
		t.Fatal("the refused switch left a rights version or an event")
	}
	if len(w.allows("agent.billing_mode.set")) != 0 || len(w.i1Denies("agent.billing_mode.set")) != 1 {
		t.Fatalf("audit %+v", w.audits("agent.billing_mode.set"))
	}
}

// TestBR01_SwitchToPersonalWithInstanceTriggerGrant (10 BR-01, no channels needed): Sam has
// trigger on the instance; switching to personal fails as above.
func TestBR01_SwitchToPersonalWithInstanceTriggerGrant(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	sam := w.person("sam", "active", false)
	assistent := w.create("assistent")
	w.rawGrant(sam, ids.ID{}, "trigger")
	p := w.want(w.do("owner", http.MethodPut, agentPath(assistent)+"/billing-mode",
		`{"billing_mode":"personal","credential_holder_id":"`+owner.String()+`","reason":"Own use"}`, ifMatch(1)...),
		http.StatusConflict, "i1_violation")
	wantI1(t, p, owner, []ids.ID{assistent, sam})
	w.unchanged(assistent, "organization", "", 1)
	if len(w.i1Denies("agent.billing_mode.set")) != 1 {
		t.Fatal("deny entry missing")
	}
	// Without the foreign grant the switch to the sole owner holds I1.
	w.sql(`UPDATE grants SET revoked_at = now(), revoked_by = $1, revoke_reason = 'fixture' WHERE subject_principal_id = $1`, sam.String())
	m := w.setPersonal(assistent, owner)
	if m["billing_mode"] != "personal" || m["credential_holder_id"] != owner.String() || m["billing_mode_changed_at"] == nil {
		t.Fatalf("switched %v", m)
	}
	allows := w.allows("agent.billing_mode.set")
	if len(allows) != 1 || allows[0].Details["from_billing_mode"] != "organization" || allows[0].Details["to_billing_mode"] != "personal" ||
		allows[0].Details["to_credential_holder_id"] != owner.String() || allows[0].Details["reason"] != "Own use" {
		t.Fatalf("audit %+v", allows)
	}
}

// TestBR01_CredentialHolderChangeIsChecked: the holder of a personal agent changes to a person
// who is not the sole owner; the owner keeps trigger everywhere, so I1 fails (03 section 6.4:
// personal works only for the sole owner, docs/DECISIONS.md M1-51). An organization agent has no
// holder to change (409 conflict); a disabled agent is not counted, so its holder may change.
func TestBR01_CredentialHolderChangeIsChecked(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	sam := w.person("sam", "active", false)
	assistent := w.create("assistent")
	lotse := w.create("lotse")
	w.want(w.do("owner", http.MethodPut, agentPath(lotse)+"/credential-holder", `{"credential_holder_id":"`+sam.String()+`","reason":"Handover"}`,
		ifMatch(1)...), http.StatusConflict, "conflict")
	w.setPersonal(assistent, owner)
	version := w.version(assistent)
	p := w.want(w.do("owner", http.MethodPut, agentPath(assistent)+"/credential-holder",
		`{"credential_holder_id":"`+sam.String()+`","reason":"Handover"}`, ifMatch(version)...), http.StatusConflict, "i1_violation")
	wantI1(t, p, sam, []ids.ID{assistent, owner})
	w.unchanged(assistent, "personal", owner.String(), version)
	if len(w.i1Denies("agent.credential_holder.set")) != 1 {
		t.Fatal("deny entry missing")
	}
	w.want(w.do("owner", http.MethodPost, agentPath(assistent)+"/disable", `{"reason":"Pause"}`, key()...), http.StatusOK, "")
	m := w.want(w.do("owner", http.MethodPut, agentPath(assistent)+"/credential-holder",
		`{"credential_holder_id":"`+sam.String()+`","reason":"Handover"}`, ifMatch(w.version(assistent))...), http.StatusOK, "")
	if m["credential_holder_id"] != sam.String() {
		t.Fatalf("holder of a disabled agent %v", m)
	}
	allows := w.allows("agent.credential_holder.set")
	if len(allows) != 1 || allows[0].Details["from_credential_holder_id"] != owner.String() || allows[0].Details["to_credential_holder_id"] != sam.String() {
		t.Fatalf("audit %+v", allows)
	}
}

// TestBR01_EnableIsChecked: Assistent is personal and disabled; meanwhile Sam gets trigger on the
// instance (allowed, the agent is inactive); enableAgent fails with i1_violation and the agent
// stays disabled.
func TestBR01_EnableIsChecked(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	sam := w.person("sam", "active", false)
	assistent := w.create("assistent")
	w.setPersonal(assistent, owner)
	w.want(w.do("owner", http.MethodPost, agentPath(assistent)+"/disable", `{"reason":"Pause"}`, key()...), http.StatusOK, "")
	if _, err := w.admin.CreateGrant(context.Background(), w.actors.Get("owner"), authz.CreateGrantCmd{
		SubjectKind: "user", SubjectID: sam, TargetKind: "instance", Level: "trigger", Reason: "Team",
	}); err != nil {
		t.Fatalf("grant while the agent is disabled: %v", err)
	}
	version := w.version(assistent)
	p := w.want(w.do("owner", http.MethodPost, agentPath(assistent)+"/enable", `{"reason":"Back"}`, key()...), http.StatusConflict, "i1_violation")
	wantI1(t, p, owner, []ids.ID{assistent, sam})
	if w.text(`SELECT status FROM principals WHERE id = $1`, assistent.String()) != "disabled" || w.version(assistent) != version {
		t.Fatal("the refused enable changed the agent")
	}
	if len(w.i1Denies("agent.enable")) != 1 {
		t.Fatal("deny entry missing")
	}
}

// TestBR03_ParallelChangesSerialize (10 BR-03): a switch to personal and a parallel trigger grant
// on the instance that only together break I1: exactly one succeeds, serialised by
// permission_version.
func TestBR03_ParallelChangesSerialize(t *testing.T) {
	t.Parallel()
	for round := 0; round < 3; round++ {
		w := newWorld(t)
		owner := w.person("owner", "active", true)
		sam := w.person("sam", "active", false)
		assistent := w.create("assistent")
		var wg sync.WaitGroup
		var switchStatus int
		var grantErr error
		ready := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-ready
			r := w.do("owner", http.MethodPut, agentPath(assistent)+"/billing-mode",
				`{"billing_mode":"personal","credential_holder_id":"`+owner.String()+`","reason":"Own use"}`, ifMatch(1)...)
			switchStatus = r.StatusCode
		}()
		go func() {
			defer wg.Done()
			<-ready
			_, grantErr = w.admin.CreateGrant(context.Background(), w.actors.Get("owner"), authz.CreateGrantCmd{
				SubjectKind: "user", SubjectID: sam, TargetKind: "instance", Level: "trigger", Reason: "Team",
			})
		}()
		close(ready)
		wg.Wait()
		switched := switchStatus == http.StatusOK
		granted := grantErr == nil
		if switched == granted {
			t.Fatalf("round %d: switch %d, grant %v: want exactly one", round, switchStatus, grantErr)
		}
		if !switched && switchStatus != http.StatusConflict {
			t.Fatalf("round %d: switch %d, want 409", round, switchStatus)
		}
		if !granted {
			if e, ok := errs.As(grantErr); !ok || e.PublicCode() != errs.CodeI1Violation {
				t.Fatalf("round %d: grant error %v, want i1_violation", round, grantErr)
			}
		}
		mode, _ := w.policy(assistent)
		grants := w.count(`SELECT count(*) FROM grants WHERE subject_principal_id = $1 AND revoked_at IS NULL`, sam.String())
		if (mode == "personal") == (grants == 1) {
			t.Fatalf("round %d: mode %s with %d grants", round, mode, grants)
		}
	}
}

// TestBR08_SetAllPersonalWithOneViolatorChangesNothing (10 BR-08): the list [A, B] where B serves
// a channel Alex may trigger: 409, A and B unchanged, the problem names B only.
func TestBR08_SetAllPersonalWithOneViolatorChangesNothing(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	alex := w.person("alex", "active", false)
	a := w.create("assistent")
	b := w.create("lotse")
	w.rawGrant(alex, w.channel(b), "trigger")
	pv := w.permissionVersion()
	p := w.want(w.do("owner", http.MethodPut, "/api/v1/instance/agent-billing-mode",
		`{"billing_mode":"personal","credential_holder_id":"`+owner.String()+`","agent_ids":["`+b.String()+`","`+a.String()+`"],"reason":"Own use"}`),
		http.StatusConflict, "i1_violation")
	wantI1(t, p, owner, []ids.ID{b, alex})
	w.unchanged(a, "organization", "", 1)
	w.unchanged(b, "organization", "", 1)
	if w.permissionVersion() != pv || len(w.allows("agent.billing_mode.set_all")) != 0 ||
		len(w.i1Denies("agent.billing_mode.set_all")) != 1 {
		t.Fatal("the refused switch left traces")
	}
	// Without B the list switches A.
	m := w.want(w.do("owner", http.MethodPut, "/api/v1/instance/agent-billing-mode",
		`{"billing_mode":"personal","credential_holder_id":"`+owner.String()+`","agent_ids":["`+a.String()+`","`+a.String()+`"],"reason":"Own use"}`),
		http.StatusOK, "")
	if !slices.Equal(m["changed_agent_ids"].([]any), []any{a.String()}) {
		t.Fatalf("changed %v", m)
	}
	w.unchanged(a, "personal", owner.String(), 2)
}

// TestSetAllOrganizationWithoutListSwitchesAll: without agent_ids organization switches every
// agent not yet on organization, disabled ones included, in one transaction with one audit entry
// listing old values; agents already on organization are not reported.
func TestSetAllOrganizationWithoutListSwitchesAll(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	a := w.create("assistent")
	b := w.create("lotse")
	c := w.create("scout")
	w.setPersonal(a, owner)
	w.setPersonal(c, owner)
	w.want(w.do("owner", http.MethodPost, agentPath(c)+"/disable", `{"reason":"Pause"}`, key()...), http.StatusOK, "")
	events := len(w.updates())
	m := w.want(w.do("owner", http.MethodPut, "/api/v1/instance/agent-billing-mode", `{"billing_mode":"organization","reason":"Emergency"}`),
		http.StatusOK, "")
	if !slices.Equal(m["changed_agent_ids"].([]any), []any{a.String(), c.String()}) {
		t.Fatalf("changed %v", m)
	}
	for _, id := range []ids.ID{a, b, c} {
		if mode, holder := w.policy(id); mode != "organization" || holder != "" {
			t.Fatalf("agent %s still %s %s", id, mode, holder)
		}
	}
	if len(w.updates()) != events+2 {
		t.Fatal("one agent.updated per switched agent")
	}
	allows := w.allows("agent.billing_mode.set_all")
	if len(allows) != 1 || !slices.Equal(allows[0].Details["agent_ids"].([]any), []any{a.String(), c.String()}) ||
		!slices.Equal(allows[0].Details["from_billing_modes"].([]any), []any{"personal", "personal"}) ||
		!slices.Equal(allows[0].Details["from_credential_holder_ids"].([]any), []any{owner.String(), owner.String()}) ||
		allows[0].Details["reason"] != "Emergency" {
		t.Fatalf("audit %+v", allows)
	}
	// Nothing left to switch: 200 with an empty list, no rights version, audit no_change.
	pv := w.permissionVersion()
	m = w.want(w.do("owner", http.MethodPut, "/api/v1/instance/agent-billing-mode", `{"billing_mode":"organization","reason":"Again"}`),
		http.StatusOK, "")
	if len(m["changed_agent_ids"].([]any)) != 0 || w.permissionVersion() != pv {
		t.Fatalf("second switch %v", m)
	}
	if allows := w.allows("agent.billing_mode.set_all"); len(allows) != 2 || allows[1].Details["no_change"] != true {
		t.Fatalf("no-op audit %+v", allows)
	}
}

// TestSetAllPersonalRequiresListAndHolder: personal needs agent_ids and a holder, organization
// takes no holder, an unknown id is 400 on its index, the reason is required.
func TestSetAllPersonalRequiresListAndHolder(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	a := w.create("assistent")
	path := "/api/v1/instance/agent-billing-mode"
	holder := `"credential_holder_id":"` + owner.String() + `"`
	cases := []struct{ body, pointer string }{
		{`{"billing_mode":"personal",` + holder + `,"reason":"x"}`, "/agent_ids"},
		{`{"billing_mode":"personal",` + holder + `,"agent_ids":[],"reason":"x"}`, "/agent_ids"},
		{`{"billing_mode":"personal","agent_ids":["` + a.String() + `"],"reason":"x"}`, "/credential_holder_id"},
		{`{"billing_mode":"organization",` + holder + `,"reason":"x"}`, "/credential_holder_id"},
		{`{"billing_mode":"personal",` + holder + `,"agent_ids":["` + a.String() + `","` + ids.New().String() + `"],"reason":"x"}`, "/agent_ids/1"},
		{`{"billing_mode":"organization","reason":" "}`, "/reason"},
	}
	for _, c := range cases {
		p := w.want(w.do("owner", http.MethodPut, path, c.body), http.StatusBadRequest, "validation_failed")
		wantPointer(t, p, c.pointer)
	}
	w.want(w.do("owner", http.MethodPut, path, `{"billing_mode":"organization","reason":""}`), http.StatusBadRequest, "validation_failed")
	w.unchanged(a, "organization", "", 1)
}

// TestSetBillingModeOrganizationClearsHolder: back to organization the holder becomes absent and
// billing_mode_changed_at is set; a holder given with organization is 400.
func TestSetBillingModeOrganizationClearsHolder(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	id := w.create("assistent")
	w.setPersonal(id, owner)
	path := agentPath(id) + "/billing-mode"
	p := w.want(w.do("owner", http.MethodPut, path, `{"billing_mode":"organization",`+`"credential_holder_id":"`+owner.String()+`","reason":"x"}`,
		ifMatch(2)...), http.StatusBadRequest, "validation_failed")
	wantPointer(t, p, "/credential_holder_id")
	r := w.do("owner", http.MethodPut, path, `{"billing_mode":"organization","reason":"Company key"}`, ifMatch(2)...)
	m := w.want(r, http.StatusOK, "")
	if _, ok := m["credential_holder_id"]; ok || m["billing_mode"] != "organization" || m["billing_mode_changed_at"] == nil || r.Header.Get("ETag") != `"3"` {
		t.Fatalf("organization %v", m)
	}
	w.unchanged(id, "organization", "", 3)
	allows := w.allows("agent.billing_mode.set")
	if len(allows) != 2 || allows[1].Details["from_credential_holder_id"] != owner.String() {
		t.Fatalf("audit %+v", allows)
	}
	if _, ok := allows[1].Details["to_credential_holder_id"]; ok {
		t.Fatal("organization with a new holder in the audit")
	}
}

// TestSetBillingModeHolderMustBeActiveHuman: an agent, a disabled person and an unknown id are
// no holder (400 on /credential_holder_id); personal without a holder is 400 too.
func TestSetBillingModeHolderMustBeActiveHuman(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.person("owner", "active", true)
	gone := w.person("gone", "disabled", false)
	id := w.create("assistent")
	other := w.create("lotse")
	path := agentPath(id) + "/billing-mode"
	for _, holder := range []ids.ID{other, gone, ids.New()} {
		p := w.want(w.do("owner", http.MethodPut, path, `{"billing_mode":"personal","credential_holder_id":"`+holder.String()+`","reason":"x"}`,
			ifMatch(1)...), http.StatusBadRequest, "validation_failed")
		wantPointer(t, p, "/credential_holder_id")
	}
	p := w.want(w.do("owner", http.MethodPut, path, `{"billing_mode":"personal","reason":"x"}`, ifMatch(1)...), http.StatusBadRequest, "validation_failed")
	wantPointer(t, p, "/credential_holder_id")
	w.unchanged(id, "organization", "", 1)
}

// TestPolicyReasonRequired: an empty reason is refused by the contract (minLength 1) and a blank
// one by the use case, for the switch, the holder and the switch of all agents.
func TestPolicyReasonRequired(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	id := w.create("assistent")
	for _, reason := range []string{"", "  "} {
		w.want(w.do("owner", http.MethodPut, agentPath(id)+"/billing-mode", `{"billing_mode":"organization","reason":"`+reason+`"}`, ifMatch(1)...),
			http.StatusBadRequest, "validation_failed")
		w.want(w.do("owner", http.MethodPut, agentPath(id)+"/credential-holder", `{"credential_holder_id":"`+owner.String()+`","reason":"`+reason+`"}`,
			ifMatch(1)...), http.StatusBadRequest, "validation_failed")
		w.want(w.do("owner", http.MethodPost, agentPath(id)+"/disable", `{"reason":"`+reason+`"}`, key()...), http.StatusBadRequest, "validation_failed")
		w.want(w.do("owner", http.MethodPut, agentPath(id)+"/tool-class", `{"tool_class":"host","reason":"`+reason+`"}`, ifMatch(1)...),
			http.StatusBadRequest, "validation_failed")
	}
	w.unchanged(id, "organization", "", 1)
}

// TestPolicyUnchangedWritesNothing: the same mode and holder answer 200 with the current agent,
// no new version, no event, no rights version, and an audit entry with no_change.
func TestPolicyUnchangedWritesNothing(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	id := w.create("assistent")
	pv, events := w.permissionVersion(), len(w.updates())
	m := w.want(w.do("owner", http.MethodPut, agentPath(id)+"/billing-mode", `{"billing_mode":"organization","reason":"Check"}`, ifMatch(1)...),
		http.StatusOK, "")
	if m["version"] != float64(1) || w.permissionVersion() != pv || len(w.updates()) != events {
		t.Fatalf("unchanged %v", m)
	}
	w.setPersonal(id, owner)
	pv, events = w.permissionVersion(), len(w.updates())
	w.setPersonal(id, owner)
	if w.version(id) != 2 || w.permissionVersion() != pv || len(w.updates()) != events {
		t.Fatal("the same personal target wrote something")
	}
	allows := w.allows("agent.billing_mode.set")
	if len(allows) != 3 || allows[0].Details["no_change"] != true || allows[2].Details["no_change"] != true {
		t.Fatalf("audit %+v", allows)
	}
}
