// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// mutation is one mutating operation of the table: the request for the agent target and the
// audit action whose allow entry it writes.
type mutation struct {
	action, method string
	path           func(target ids.ID) string
	body           func(w *world, target ids.ID) string
	ifMatch, keyed bool
	// onInstance marks the two operations on the instance: there is no invisible agent.
	onInstance bool
}

// mutations are the eight mutating operations of the group Agents.
func mutations(owner ids.ID) []mutation {
	sub := func(s string) func(ids.ID) string { return func(id ids.ID) string { return agentPath(id) + s } }
	fixed := func(s string) func(*world, ids.ID) string { return func(*world, ids.ID) string { return s } }
	return []mutation{
		{action: "agent.update", method: http.MethodPatch, path: sub(""), body: fixed(`{"description":"Pilot"}`), ifMatch: true, keyed: true},
		{action: "agent.tool_class.set", method: http.MethodPut, path: sub("/tool-class"), body: fixed(`{"tool_class":"scoped","reason":"Checkout"}`), ifMatch: true},
		{action: "agent.billing_mode.set", method: http.MethodPut, path: sub("/billing-mode"), ifMatch: true,
			body: fixed(`{"billing_mode":"personal","credential_holder_id":"` + owner.String() + `","reason":"Own use"}`)},
		{action: "agent.credential_holder.set", method: http.MethodPut, path: sub("/credential-holder"), ifMatch: true,
			body: fixed(`{"credential_holder_id":"` + owner.String() + `","reason":"Same holder"}`)},
		{action: "agent.disable", method: http.MethodPost, path: sub("/disable"), body: fixed(`{"reason":"Pause"}`), keyed: true},
		{action: "agent.enable", method: http.MethodPost, path: sub("/enable"), body: fixed(`{"reason":"Back"}`), keyed: true},
		{action: "agent.billing_mode.set_all", method: http.MethodPut, path: func(ids.ID) string { return "/api/v1/instance/agent-billing-mode" },
			body: fixed(`{"billing_mode":"organization","reason":"Company key"}`), onInstance: true},
		{action: "agent.create", method: http.MethodPost, path: func(ids.ID) string { return agentsPath },
			body: fixed(`{"handle":"neu","display_name":"Neu","adapter_kind":"custom"}`), keyed: true, onInstance: true},
	}
}

// send runs m on target as name.
func (w *world) send(name string, m mutation, target ids.ID) int {
	w.t.Helper()
	var hdr []string
	if m.ifMatch {
		hdr = append(hdr, ifMatch(w.version(target))...)
	}
	if m.keyed {
		hdr = append(hdr, key()...)
	}
	return w.do(name, m.method, m.path(target), m.body(w, target), hdr...).StatusCode
}

// TestAgentMutationsOwnerStepUpAudit runs every mutating operation as: a person who sees the agent
// through a served channel (403), a person who sees nothing (404, or 403 on the instance), the
// owner without a fresh step-up (401 step_up_required), the owner with a personal token (SR5: no
// owner role, the agent is invisible, 404, or 403 on the instance; docs/DECISIONS.md M1-51) and
// the owner with a fresh step-up (success with one more allow entry).
func TestAgentMutationsOwnerStepUpAudit(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	sam := w.person("sam", "active", false)
	w.person("robin", "active", false)
	w.session("stale", owner, w.clk.Now().Add(-time.Hour))
	w.personalToken("pat", owner)
	lotse := w.create("lotse")
	w.rawGrant(sam, w.channel(lotse), "read")
	for _, m := range mutations(owner) {
		invisible := http.StatusNotFound
		if m.onInstance {
			invisible = http.StatusForbidden
		}
		visible := http.StatusForbidden
		if got := w.send("sam", m, lotse); got != visible {
			t.Errorf("%s as a reader: %d, want %d", m.action, got, visible)
		}
		if got := w.send("robin", m, lotse); got != invisible {
			t.Errorf("%s as a stranger: %d, want %d", m.action, got, invisible)
		}
		if got := w.send("stale", m, lotse); got != http.StatusUnauthorized {
			t.Errorf("%s without step-up: %d, want 401", m.action, got)
		}
		if got := w.send("pat", m, lotse); got != invisible {
			t.Errorf("%s with the owner's personal token: %d, want %d", m.action, got, invisible)
		}
		before := len(w.allows(m.action))
		if got := w.send("owner", m, lotse); got != http.StatusOK && got != http.StatusCreated {
			t.Errorf("%s as the owner: %d", m.action, got)
		}
		if after := len(w.allows(m.action)); after != before+1 {
			t.Errorf("%s: %d allow entries, want %d", m.action, after, before+1)
		}
	}
	r := w.do("stale", http.MethodGet, agentPath(lotse), "")
	if r.StatusCode != http.StatusOK {
		t.Fatalf("reading needs no step-up: %d", r.StatusCode)
	}
}
