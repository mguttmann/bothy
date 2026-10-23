// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// callWithInvocation is a GET of consumerWorld.call with the Invocation-Token values of an agent
// inside a run (07 guard rule SR9).
func (w *invocationWorld) callWithInvocation(a consumerAgent, path string, tokens ...string) (int, map[string]any) {
	w.t.Helper()
	req, err := http.NewRequestWithContext(testkit.Context(w.t), http.MethodGet, w.base+path, nil)
	if err != nil {
		w.t.Fatal(err)
	}
	req.Header.Set("Authorization", "DPoP "+a.tok)
	req.Header.Set("DPoP", identitytest.SignDPoPProof(a.key, identitytest.ProofClaims{HTM: http.MethodGet, HTU: gwPublicURL + path,
		IAT: time.Now(), Nonce: w.id.DPoP.CurrentNonce(), ATH: dpop.ATH(a.tok)}))
	for _, t := range tokens {
		req.Header.Add("Invocation-Token", t)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return resp.StatusCode, m
}

// TestWire_ActingForSource (brief M2-14 "Tests"): the started server binds the acting-for source
// of runs to the authenticator: the delivered invocation token binds the agent's request to Sam's
// run (Sam's own channel answers 200, a channel Sam cannot read but the agent operates answers 404
// like an unknown ID), and a header that is no token of this agent answers 403
// invocation_token_invalid with reason invalid.
func TestWire_ActingForSource(t *testing.T) {
	t.Parallel()
	w := newInvocationWorld(t)
	ch, a, handle := w.operated("bound", false)
	w.triggerGrant(w.sam, ch)
	// The first poll creates the consumer at the head of the log (M2-07).
	w.poll(w.base, a, "sr9", "")
	m, _ := w.say(w.sam, ch.thread, "@"+handle+" please look")
	r, ok := w.runOf(m)
	if !ok {
		t.Fatal("no run")
	}
	evs, _ := w.pollInvocations(a, "sr9", w.invocationSeq(r.id))
	if len(evs) == 0 || evs[0].InvocationToken == "" {
		t.Fatalf("no invocation token delivered: %+v", evs)
	}
	tok := evs[0].InvocationToken
	// A second channel operated by the same agent, with a message Sam cannot read.
	other := w.newChannel(w.space, "other")
	w.sql(`UPDATE channels SET operator_inherit = false, operator_agent_id = $1 WHERE id = $2`, a.id.String(), other.id)
	hidden, _ := w.say(w.owner, other.thread, "not for Sam")

	if code, b := w.callWithInvocation(a, "/api/v1/messages/"+m, tok); code != http.StatusOK {
		t.Fatalf("own channel with the token = %d %v", code, b)
	}
	if code, b := w.callWithInvocation(a, "/api/v1/messages/"+hidden); code != http.StatusOK {
		t.Fatalf("operated channel without the token = %d %v", code, b)
	}
	if code, b := w.callWithInvocation(a, "/api/v1/messages/"+hidden, tok); code != http.StatusNotFound || b["reason"] != nil {
		t.Fatalf("operated channel with Sam's token = %d %v, want 404 without reason", code, b)
	}
	code, b := w.callWithInvocation(a, "/api/v1/messages/"+m, "not-a-token")
	if code != http.StatusForbidden || b["code"] != "invocation_token_invalid" || b["reason"] != "invalid" {
		t.Fatalf("garbage header = %d %v", code, b)
	}
}
