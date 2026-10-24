// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
)

// TestMeasure_BillingModeSwitches drives billing_mode_switches of the own measurement (M6-13, 25
// 3.2) through agent.billing_mode.set and .set_all against every real source: a switch to personal
// and one to organization count once each with the new mode and the agent, set_all counts every
// agent it switches, and a call without a change counts nothing.
func TestMeasure_BillingModeSwitches(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	// The world's own setup switched agents already; only the calls of this test count.
	before := len(w.measure.Calls(measure.BillingModeSwitches))
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/grants/"+w.g2.String()+"/revoke", `{"reason":"Moved"}`, key()...))
	w.wantOK(w.setPersonal(w.texter, w.owner))
	w.switchTo(w.assistent, "Company key")
	w.wantOK(w.do("owner", http.MethodPut, "/api/v1/instance/agent-billing-mode", `{"billing_mode":"organization","reason":"Emergency"}`))
	w.switchTo(w.assistent, "Again")
	if inv := w.measure.Invalid(); len(inv) != 0 {
		t.Fatalf("invalid measure calls %v", inv)
	}
	got := map[[2]string]int{}
	for _, c := range w.measure.Calls(measure.BillingModeSwitches)[before:] {
		if !c.Key.ChannelID.IsZero() || c.Value != measure.One() {
			t.Fatalf("call %+v", c)
		}
		got[[2]string{c.Key.AgentID.String(), c.Key.Dimension}]++
	}
	want := map[[2]string]int{
		{w.texter.String(), "personal"}:        1,
		{w.assistent.String(), "organization"}: 1,
		{w.texter.String(), "organization"}:    1,
	}
	if len(got) != len(want) {
		t.Fatalf("switches %v, want %v", got, want)
	}
	for k, n := range want {
		if got[k] != n {
			t.Errorf("%v: %d, want %d", k, got[k], n)
		}
	}
}
