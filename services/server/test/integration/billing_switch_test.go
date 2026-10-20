// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
)

// The switches of I4 (10 section 9, M2-21) against every real source and the heartbeat of M2-20.

// receivedBy returns the types and resources of the durable events name receives through
// catch-up, the read the consumer engine of pollAgentEvents and the gateway deliver with (04
// section 7, M2-07): every event the principal may receive, in seq order.
func (w *world) receivedBy(name string) []string {
	w.t.Helper()
	repo := instancepg.NewRepository(w.pool)
	cu, err := syncapp.NewCatchUp(syncapp.CatchUpConfig{PageSize: 500, MaxEvents: 100000}, syncapp.CatchUpDeps{
		Log: syncpg.NewRepository(w.pool, nil), Head: instanceapp.NewVersions(repo), Epoch: instanceapp.NewState(repo, w.uow), Authorizer: w.az,
	})
	must(w.t, err)
	page, err := cu.PageEvents(actor.WithActor(context.Background(), w.actor(name)), sync.PageInput{After: 0, Head: w.count(`SELECT coalesce(max(seq), 0) FROM event_log`)})
	must(w.t, err)
	var out []string
	for _, e := range page.Events {
		out = append(out, e.Type+":"+e.ResourceID.String())
	}
	return out
}

// switchTo sends setAgentBillingMode to organization for agent as the owner.
func (w *world) switchTo(agent ids.ID, reason string) map[string]any {
	w.t.Helper()
	return w.wantOK(w.do("owner", http.MethodPut, "/api/v1/agents/"+agent.String()+"/billing-mode",
		`{"billing_mode":"organization","reason":"`+reason+`"}`, ifMatch(w.agentVersion(agent))...))
}

const (
	// beatPersonal and beatOrganization are heartbeats of an adapter that supports both modes.
	beatPersonal = `{"state":"online","reported_billing_mode":"personal","billing_modes_supported":["personal","organization"],` +
		`"billing_evidence":{"key_type":"oauth","method":"resolved_credentials"}}`
	beatOrganization = `{"state":"online","reported_billing_mode":"organization","billing_modes_supported":["personal","organization"],` +
		`"billing_evidence":{"key_type":"api_key","method":"resolved_credentials"}}`
)

// TestBR08_SetAllPersonalWithViolatorChangesNothing (BR-08, every real source): set_all to
// personal for Assistent and Lotse, where Alex triggers in Support, answers 409 i1_violation for
// Lotse and changes nothing at all.
func TestBR08_SetAllPersonalWithViolatorChangesNothing(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.switchTo(w.assistent, "Company key")
	assistent, lotse, pv, updates := w.policy(w.assistent), w.policy(w.lotse), w.permissionVersion(), len(w.events("agent.updated"))
	r := w.do("owner", http.MethodPut, "/api/v1/instance/agent-billing-mode", `{"billing_mode":"personal","credential_holder_id":"`+w.owner.String()+
		`","agent_ids":["`+w.assistent.String()+`","`+w.lotse.String()+`"],"reason":"Own subscription"}`)
	w.wantI1(r, w.lotse, w.owner, w.alex)
	if w.policy(w.assistent) != assistent || w.policy(w.lotse) != lotse || w.permissionVersion() != pv || len(w.events("agent.updated")) != updates {
		t.Fatalf("set_all changed something: %s %s", w.policy(w.assistent), w.policy(w.lotse))
	}
	if d := w.denies("agent.billing_mode.set_all", "i1_violation"); len(d) != 1 {
		t.Fatalf("deny rows %+v", d)
	}
}

// TestI4_SwitchReachesClientsAndAdapter (10 section 9.2): a switch to organization is agent.updated
// with receive action agent.read for the owner and for Assistent itself; the card shows the
// difference to the reported personal, the warning comes after billing.mismatch_alert_after and
// ends with a heartbeat reporting organization. set_all to organization without a list switches
// every agent not on organization, a disabled one included, with one entry and one event each.
func TestI4_SwitchReachesClientsAndAdapter(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.beatOK("assistent", beatPersonal)
	m := w.switchTo(w.assistent, "Company key")
	if m["billing_mode"] != "organization" {
		t.Fatalf("answer %v", m)
	}
	if ra := w.text(`SELECT receive_action FROM event_log WHERE type = 'agent.updated' AND resource_id = $1 ORDER BY seq DESC LIMIT 1`,
		w.assistent.String()); ra != "agent.read" {
		t.Fatalf("receive action %s", ra)
	}
	want := "agent.updated:" + w.assistent.String()
	for _, name := range []string{"owner", "assistent"} {
		if !slices.Contains(w.receivedBy(name), want) {
			t.Fatalf("%s does not receive %s", name, want)
		}
	}
	card := w.wantOK(w.do("owner", http.MethodGet, "/api/v1/agents/"+w.assistent.String(), ""))
	if p := card["presence"].(map[string]any); card["billing_mode"] != "organization" || p["billing_mode_matches"] != false {
		t.Fatalf("card %v", card)
	}
	w.keepAlive("assistent", beatPersonal, presenceConfig.MismatchAlertAfter+time.Second)
	if c := w.card(w.assistent); !slices.Equal(warnings(c), []string{"billing_mode_mismatch"}) {
		t.Fatalf("card after billing.mismatch_alert_after %v", c)
	}
	if p := w.beatOK("assistent", beatOrganization); p["billing_mode_matches"] != true || len(warnings(p)) != 0 {
		t.Fatalf("ack after organization %v", p)
	}
	if c := w.card(w.assistent); c["billing_mode_matches"] != true || len(warnings(c)) != 0 {
		t.Fatalf("card after organization %v", c)
	}

	v := newWorld(t)
	v.wantOK(v.do("owner", http.MethodPost, "/api/v1/grants/"+v.g2.String()+"/revoke", `{"reason":"Moved"}`, key()...))
	v.wantOK(v.setPersonal(v.texter, v.owner))
	v.wantOK(v.do("owner", http.MethodPost, "/api/v1/agents/"+v.texter.String()+"/disable", `{"reason":"Pause"}`, key()...))
	updates := len(v.events("agent.updated"))
	all := v.wantOK(v.do("owner", http.MethodPut, "/api/v1/instance/agent-billing-mode", `{"billing_mode":"organization","reason":"Emergency"}`))
	changed := strs(all["changed_agent_ids"])
	wantIDs := []string{v.assistent.String(), v.texter.String()}
	slices.Sort(wantIDs)
	if !slices.Equal(changed, wantIDs) || len(v.events("agent.updated")) != updates+2 {
		t.Fatalf("changed %v, want %v; agent.updated %d", changed, wantIDs, len(v.events("agent.updated"))-updates)
	}
	entries := v.allows("agent.billing_mode.set_all")
	if len(entries) != 1 || !slices.Equal(strs(entries[0].Details["agent_ids"]), wantIDs) {
		t.Fatalf("set_all entries %+v", entries)
	}
	if !strings.HasPrefix(v.policy(v.texter), "organization:-:") {
		t.Fatalf("disabled Texter %s", v.policy(v.texter))
	}
}

// TestGuidedFlowAdapterStateAfterSwitch (10 section 11 point 5 and its sequence diagram): after
// the guided createGrant the consumer of Assistent receives agent.updated, the next heartbeat with
// personal answers billing_mode_matches false, the warning follows billing.mismatch_alert_after,
// and a heartbeat with organization ends both.
func TestGuidedFlowAdapterStateAfterSwitch(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	if p := w.beatOK("assistent", beatPersonal); p["billing_mode_matches"] != true {
		t.Fatalf("ack before %v", p)
	}
	w.want(w.createGrant("owner", with(w.samNotizen(), resolveJSON("Sam triggers the assistant", w.assistent))), http.StatusCreated, "")
	if !slices.Contains(w.receivedBy("assistent"), "agent.updated:"+w.assistent.String()) {
		t.Fatal("Assistent does not receive its agent.updated")
	}
	if c := w.card(w.assistent); c["billing_mode_matches"] != false {
		t.Fatalf("card right after the guided flow %v", c)
	}
	if p := w.beatOK("assistent", beatPersonal); p["billing_mode_matches"] != false {
		t.Fatalf("ack after the guided flow %v", p)
	}
	w.keepAlive("assistent", beatPersonal, presenceConfig.MismatchAlertAfter+time.Second)
	if c := w.card(w.assistent); !slices.Equal(warnings(c), []string{"billing_mode_mismatch"}) {
		t.Fatalf("card after billing.mismatch_alert_after %v", c)
	}
	if p := w.beatOK("assistent", beatOrganization); p["billing_mode_matches"] != true || len(warnings(p)) != 0 {
		t.Fatalf("ack after organization %v", p)
	}
}
