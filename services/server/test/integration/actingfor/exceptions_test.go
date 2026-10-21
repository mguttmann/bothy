// SPDX-License-Identifier: Apache-2.0

package actingfor_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestSR9_ExceptionRoutesEndToEnd is the end-to-end case of the SR9 exceptions whose routes the
// integration with M2-10, M2-12 and M2-15 enables (INT-W55/6; brief M2-14 "Out of scope"):
// patchRun, reportAgentSession and respondToInteraction. Lotse's run of Alex keeps working with
// its token T after the owner revoked G-1, Alex's only grant, while the same operation on a run of
// the owner with T answers 404 like an unknown resource and 2xx without the header.
// agent.cancel_grace is long, so the cancel request of the revocation does not end the run.
func TestSR9_ExceptionRoutesEndToEnd(t *testing.T) {
	t.Parallel()
	e := newEnv(t, harness.Options{}, map[string]string{"agent.cancel_grace": "30m"})
	own := e.startedRun(e.alex, e.lotse, "Support/Allgemein", "Eigener Lauf")
	e.operatorLotse("Engineering/Werkzeuge")
	foreign := e.startedRun(e.owner, e.lotse, "Engineering/Werkzeuge", "Fremder Lauf")
	ownIx := e.clickCard(e.alex, own)
	foreignIx := e.clickCard(e.owner, foreign)
	e.revoke(e.w.Grants["G-1"])

	t.Run("patchRun", func(t *testing.T) {
		e := e.in(t)
		e.ok(e.do(e.lotse, http.MethodPatch, "/runs/"+own.id, e.statusPatch(own, "Eigener Lauf arbeitet"), own.token),
			http.StatusOK, "patchRun of the own run with T")
		got := e.do(e.lotse, http.MethodPatch, "/runs/"+foreign.id, e.statusPatch(foreign, "Fremd"), own.token)
		e.wantProblem(got, http.StatusNotFound, "not_found", "", "patchRun of the foreign run with T")
		e.sameAnswer(got, e.do(e.lotse, http.MethodPatch, "/runs/"+ids.New().String(), e.statusPatch(foreign, "Fremd"), own.token),
			"patchRun of the foreign run with T")
		e.ok(e.do(e.lotse, http.MethodPatch, "/runs/"+foreign.id, e.statusPatch(foreign, "Fremd")), http.StatusOK,
			"patchRun of the foreign run without the header")
	})
	t.Run("reportAgentSession", func(t *testing.T) {
		e := e.in(t)
		e.ok(e.do(e.lotse, http.MethodPost, "/threads/"+own.thread+"/agent-sessions/report",
			map[string]any{"external_session_ref": "sess-own", "started": true}, own.token), http.StatusOK, "reportAgentSession of the own run with T")
		got := e.do(e.lotse, http.MethodPost, "/threads/"+foreign.thread+"/agent-sessions/report",
			map[string]any{"external_session_ref": "sess-foreign", "started": true}, own.token)
		e.wantProblem(got, http.StatusNotFound, "not_found", "", "reportAgentSession of the foreign run with T")
		e.sameAnswer(got, e.do(e.lotse, http.MethodPost, "/threads/"+ids.New().String()+"/agent-sessions/report",
			map[string]any{"external_session_ref": "sess-foreign", "started": true}, own.token), "reportAgentSession of the foreign run with T")
		e.ok(e.do(e.lotse, http.MethodPost, "/threads/"+foreign.thread+"/agent-sessions/report",
			map[string]any{"external_session_ref": "sess-foreign", "started": true}), http.StatusOK,
			"reportAgentSession of the foreign run without the header")
	})
	t.Run("respondToInteraction", func(t *testing.T) {
		e := e.in(t)
		ack := map[string]any{"state": "acknowledged"}
		e.ok(e.do(e.lotse, http.MethodPost, "/interactions/"+ownIx+"/respond", ack, own.token), http.StatusOK,
			"respondToInteraction of the own run with T")
		got := e.do(e.lotse, http.MethodPost, "/interactions/"+foreignIx+"/respond", ack, own.token)
		e.wantProblem(got, http.StatusNotFound, "not_found", "", "respondToInteraction of the foreign run with T")
		e.sameAnswer(got, e.do(e.lotse, http.MethodPost, "/interactions/"+ids.New().String()+"/respond", ack, own.token),
			"respondToInteraction of the foreign run with T")
		e.ok(e.do(e.lotse, http.MethodPost, "/interactions/"+foreignIx+"/respond", ack), http.StatusOK,
			"respondToInteraction of the foreign run without the header")
	})
	t.Run("TestSR9_TokenNeverLogged", func(t *testing.T) { e.in(t).noTokenLeak(e.secrets()) })
}

// operatorLotse makes Lotse the operator of the channel key as the owner, with the step-up.
func (e *env) operatorLotse(key string) {
	e.t.Helper()
	path := "/channels/" + e.w.Channels[key]
	cur := e.must(e.owner, http.MethodGet, path, nil, http.StatusOK)
	h := http.Header{}
	h.Set("If-Match", cur.Header.Get("ETag"))
	set, err := harness.DoWithStepUp(e.t.Context(), e.owner, harness.Request{Method: http.MethodPut, Path: path + "/operator", Header: h,
		Body: map[string]any{"mode": "agent", "agent_id": e.lotse.PrincipalID, "reason": "acting-for test"}})
	if err != nil {
		e.t.Fatal(err)
	}
	e.ok(set, http.StatusOK, "setChannelOperator")
}

// clickCard lets Lotse post a card bound to r in its thread, without the header, and lets person
// click its button; it returns the interaction.
func (e *env) clickCard(person *harness.Client, r run) string {
	e.t.Helper()
	card := e.ok(e.do(e.lotse, http.MethodPost, "/threads/"+r.thread+"/messages", map[string]any{
		"client_msg_id": ids.New().String(), "kind": "card", "content_md": "Freigeben?", "run_id": r.id,
		"components": []any{map[string]any{"custom_id": "approve", "type": "button", "restricted_to": "any",
			"definition": map[string]any{"label": "Approve", "value": "approve"}}},
	}), http.StatusCreated, "the card of the run")
	list, _ := card["components"].([]any)
	if len(list) != 1 {
		e.t.Fatalf("the card has %d components", len(list))
	}
	comp := str(list[0].(map[string]any)["id"])
	ix := e.ok(e.do(person, http.MethodPost, "/components/"+comp+"/interactions", map[string]any{"client_interaction_id": ids.New().String()}),
		http.StatusCreated, "createInteraction")
	return str(ix["id"])
}

// statusPatch is the RunPatch body that sets the status line of r on its current version, which
// Lotse reads without the header.
func (e *env) statusPatch(r run, line string) map[string]any {
	e.t.Helper()
	cur := e.ok(e.do(e.lotse, http.MethodGet, "/runs/"+r.id, nil), http.StatusOK, "getRun")
	return map[string]any{"base_version": cur["version"], "ops": []any{map[string]any{"op": "replace", "path": "/status_line", "value": line}}}
}
