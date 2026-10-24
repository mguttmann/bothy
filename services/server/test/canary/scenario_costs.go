// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"net/http"
	"time"
)

// costs drives the four cost operations of M2-22: Alex reads the costs of the thread of
// Support/Allgemein, the owner lists the price table, adds a price with its public source (step-up
// on demand) and closes it again one hour after its start. The price lies in the past of the
// server clock, so no run of the world books against it; every status the rules allow for this
// world is accepted, the scan checks the output.
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
}
