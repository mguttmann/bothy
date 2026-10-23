// SPDX-License-Identifier: Apache-2.0

package actingfor_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestSR9_InteractionTokenOfAnotherPerson (ruling D-M2-15-30): a card bound to Alex's run is
// clicked by the owner while the run waits for input, so the token of interaction.created names the
// owner in sub while the run keeps Alex as its triggering person. With that token in the header
// Lotse resumes the run and answers the interaction (not invocation_token_invalid); the binding is
// Alex's, the run's triggering person, never the person who clicked.
func TestSR9_InteractionTokenOfAnotherPerson(t *testing.T) {
	t.Parallel()
	e := newEnv(t, harness.Options{}, nil)
	r := e.startedRun(e.alex, e.lotse, "Support/Allgemein", "Freigabe durch den Owner")

	card := e.ok(e.must(e.lotse, http.MethodPost, "/threads/"+r.thread+"/messages", map[string]any{
		"client_msg_id": ids.New().String(), "kind": "card", "content_md": "Weiter?", "run_id": r.id,
		"components": []any{map[string]any{"custom_id": "go", "type": "button", "definition": map[string]any{"label": "Weiter", "value": "go"}}},
	}, http.StatusCreated, r.token), http.StatusCreated, "createMessage card")
	comps, _ := card["components"].([]any)
	if len(comps) != 1 {
		t.Fatalf("components %v", card["components"])
	}
	component := str(comps[0].(map[string]any)["id"])
	e.must(e.lotse, http.MethodPost, "/runs/"+r.id+"/await-input", map[string]any{"status_line": "Wartet"}, http.StatusOK, r.token)

	ix := e.ok(e.must(e.owner, http.MethodPost, "/components/"+component+"/interactions",
		map[string]any{"client_interaction_id": ids.New().String()}, http.StatusCreated), http.StatusCreated, "createInteraction")
	interaction := str(ix["id"])

	var tok string
	e.eventually("the interaction.created of "+interaction, func() bool {
		for _, ev := range e.page(e.lotse, consumer).Events {
			if ev.Type == "interaction.created" && ev.Resource.ID == interaction && ev.InvocationToken != "" {
				tok = ev.InvocationToken
			}
		}
		return tok != ""
	})

	e.must(e.lotse, http.MethodPost, "/runs/"+r.id+"/resume", startBody(tok), http.StatusOK, tok)
	e.must(e.lotse, http.MethodPost, "/interactions/"+interaction+"/respond", map[string]any{"state": "acknowledged"}, http.StatusOK, tok)
	if got := e.text(`SELECT triggered_by::text FROM runs WHERE id = $1`, r.id); got != e.alex.PrincipalID {
		t.Fatalf("the run's triggering person %s, want Alex", got)
	}
}

// TestSR9_RespondToInteraction (brief M2-14 "TestSR9_ExceptionRoutesEnabled", enabled by M2-15):
// interaction.respond is an own exception of SR9. With the token of Alex's run, Lotse's answer to an
// interaction on a card of another run answers 404 like an unknown interaction (bound) and 200
// without the header; after the owner revokes G-1, Alex's only grant, the answer to the
// interaction of the own run with that token still answers 200.
func TestSR9_RespondToInteraction(t *testing.T) {
	t.Parallel()
	e := newEnv(t, harness.Options{}, map[string]string{"agent.cancel_grace": "30m"})
	main := e.startedRun(e.alex, e.lotse, "Support/Allgemein", "Antwort auf die Karte")
	own := e.cardInteraction(e.alex, main)

	path := "/channels/" + e.w.Channels["Engineering/Werkzeuge"]
	cur := e.must(e.owner, http.MethodGet, path, nil, http.StatusOK)
	h := http.Header{}
	h.Set("If-Match", cur.Header.Get("ETag"))
	set, err := harness.DoWithStepUp(e.t.Context(), e.owner, harness.Request{Method: http.MethodPut, Path: path + "/operator", Header: h,
		Body: map[string]any{"mode": "agent", "agent_id": e.lotse.PrincipalID, "reason": "acting-for test"}})
	if err != nil {
		t.Fatal(err)
	}
	e.ok(set, http.StatusOK, "setChannelOperator")
	foreign := e.cardInteraction(e.owner, e.startedRun(e.owner, e.lotse, "Engineering/Werkzeuge", "Fremde Karte"))

	ack := map[string]any{"state": "acknowledged"}
	got := e.do(e.lotse, http.MethodPost, "/interactions/"+foreign+"/respond", ack, main.token)
	e.wantProblem(got, http.StatusNotFound, "not_found", "", "respondToInteraction of another run with T")
	e.sameAnswer(got, e.do(e.lotse, http.MethodPost, "/interactions/"+ids.New().String()+"/respond", ack, main.token),
		"respondToInteraction of another run with T")
	e.ok(e.do(e.lotse, http.MethodPost, "/interactions/"+foreign+"/respond", ack), http.StatusOK, "respondToInteraction without the header")

	e.revoke(e.w.Grants["G-1"])
	e.ok(e.do(e.lotse, http.MethodPost, "/interactions/"+own+"/respond", ack, main.token), http.StatusOK,
		"respondToInteraction of the own run with T after the revocation")
}

// cardInteraction posts a card of Lotse bound to r with r's token and lets clicker use its button;
// it returns the interaction.
func (e *env) cardInteraction(clicker *harness.Client, r run) string {
	e.t.Helper()
	card := e.ok(e.must(e.lotse, http.MethodPost, "/threads/"+r.thread+"/messages", map[string]any{
		"client_msg_id": ids.New().String(), "kind": "card", "content_md": "Weiter?", "run_id": r.id,
		"components": []any{map[string]any{"custom_id": "go", "type": "button", "definition": map[string]any{"label": "Weiter", "value": "go"}}},
	}, http.StatusCreated, r.token), http.StatusCreated, "createMessage card")
	comps, _ := card["components"].([]any)
	if len(comps) != 1 {
		e.t.Fatalf("components %v", card["components"])
	}
	ix := e.ok(e.must(clicker, http.MethodPost, "/components/"+str(comps[0].(map[string]any)["id"])+"/interactions",
		map[string]any{"client_interaction_id": ids.New().String()}, http.StatusCreated), http.StatusCreated, "createInteraction")
	return str(ix["id"])
}
