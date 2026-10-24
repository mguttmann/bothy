// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"net/http"
	"time"
)

// costs drives the four cost operations of M2-22 and the three reconciliation operations of M2-28:
// Alex reads the costs of the thread of Support/Allgemein, the owner lists the price table, adds a
// price with its public source (step-up on demand) and closes it again one hour after its start.
// The price lies in the past of the server clock, so no run of the world books against it. Then
// the owner reconciles a run of the thread with a canary provider (step-up on demand; the run has no
// entry of that provider, so it is listed as unmatched), submits the same as a person (403, the
// submission is for apps) and reads the estimates of Lotse (SEC-T10: the ticket that builds a path
// extends the scenario). Every status the rules allow for this world is accepted, the scan checks
// the output.
func (sc *Scenario) costs(thread string) {
	sc.T.Helper()
	owner, alex := sc.W.Owner, sc.W.People["Alex"]
	sc.get(alex, "/threads/"+thread+"/costs", ok...)
	sc.get(owner, "/prices", ok...)
	now := sc.Server.Clock().Now()
	from := now.Add(-48 * time.Hour)
	created := sc.call(owner, req{method: http.MethodPost, path: "/prices", body: map[string]any{
		"provider": "canary-provider", "model": "canary-model-" + newID(), "unit": "input_mtok", "price_micros": 3000000,
		"currency": "USD", "valid_from": timestamp(from), "source_url": "https://provider.example.com/pricing",
		"retrieved_at": timestamp(now.Add(-time.Hour)),
	}, header: header("Idempotency-Key", newID())}, http.StatusCreated)
	id := jsonText(obj(created)["id"])
	sc.call(owner, req{method: http.MethodPost, path: "/prices/" + id + "/close",
		body:   map[string]any{"valid_to": timestamp(from.Add(time.Hour))},
		header: header("Idempotency-Key", newID())}, ok...)
	sc.reconcile(thread)
}

// reconcile drives reconcileCosts, submitCostReconciliation and listCostEstimates with a canary in
// provider (M2-28 design note 15).
func (sc *Scenario) reconcile(thread string) {
	sc.T.Helper()
	owner := sc.W.Owner
	run := newID()
	if runs := items(sc.get(sc.W.People["Alex"], "/threads/"+thread+"/runs", ok...)); len(runs) > 0 {
		run = jsonText(runs[0]["id"])
	}
	now := sc.Server.Clock().Now()
	body := map[string]any{"provider": sc.Reg.New(ClassContent, "", "reconciliation provider"), "period_start": timestamp(now.Add(-24 * time.Hour)),
		"period_end": timestamp(now), "entries": []any{map[string]any{"run_id": run, "amount_micros": 1500000, "currency": "USD"}}}
	sc.call(owner, req{method: http.MethodPost, path: "/costs/reconcile", body: body, header: header("Idempotency-Key", newID())}, ok...)
	sc.call(owner, req{method: http.MethodPost, path: "/costs/reconciliations", body: body, header: header("Idempotency-Key", newID())},
		http.StatusForbidden)
	sc.get(owner, "/costs/estimates?from="+timestamp(now.Add(-24*time.Hour))+"&to="+timestamp(now)+"&agent_id="+sc.W.Agents["Lotse"].PrincipalID, ok...)
}
