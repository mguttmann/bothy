// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"net/http"
	"testing"
)

// TestFollowupAnswerDisablesCard (09 section 11.6, ruling D-M2-15-21): Lotse's run waits for input
// with a card bound to it; Alex answers with a follow-up message instead of a click. Whatever
// arrives first counts, so the follow-up disables the open components of the run in its own
// transaction and a later click answers 409 component_used.
func TestFollowupAnswerDisablesCard(t *testing.T) {
	t.Parallel()
	w := followupWorld(t)
	w.followups(w.lotse, "queue", false)
	r := w.waitingRun()
	thread := w.threadOf(r.ID)
	m, byID := w.card("lotse", thread, r.ID, twoButtons)
	card := m["id"].(string)
	before := w.messageVersion(card)

	answer := w.say("alex", thread, "Approved, go ahead")
	w.wantFollowup(answer, r.ID, "queued", "received")
	if states := w.statesOf(card); states["approve"] != "disabled" || states["deny"] != "disabled" {
		t.Fatalf("states after the follow-up %v", states)
	}
	if after := w.messageVersion(card); after != before+1 {
		t.Fatalf("card version %d, want %d", after, before+1)
	}
	w.want(w.click("alex", byID["approve"]["id"].(string), ""), http.StatusConflict, "component_used")
}
