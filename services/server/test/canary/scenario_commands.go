// SPDX-License-Identifier: Apache-2.0

package canary

import "net/http"

// commands drives the six command operations of M2-16 with a canary command description and a
// canary string option value of class content on Support/Allgemein: Lotse registers a control
// command scoped to the channel, reads it, updates it with If-Match and removes it at the end;
// Alex (trigger on Support) lists the commands of the channel, reads the command and invokes it in
// the main thread with the canary value, and once more with a value outside SEC-IN-4 (400, whose
// body must not repeat it). Sam reads Support/Allgemein, so his frames and pages carry
// command.changed; command.invoked goes to Lotse alone.
func (sc *Scenario) commands() {
	t := sc.T
	t.Helper()
	alex, lotse := sc.W.People["Alex"], sc.W.Agents["Lotse"]
	allgemein := sc.W.Channels["Support/Allgemein"]
	def := func(description string) map[string]any {
		return map[string]any{
			"name": "triage", "description": description, "scope_kind": "channel", "scope_id": allgemein, "control": true,
			"options": []any{map[string]any{"name": "ticket", "description": "Ticket number", "type": "string", "required": true}},
		}
	}
	created := obj(sc.post(lotse, "/commands", def("Triage "+sc.Reg.New(ClassContent, allgemein, "command description")),
		http.StatusCreated))
	id := jsonText(created["id"])
	if id == "" {
		t.Errorf("commands: registerCommand answered no id")
		return
	}
	path := "/commands/" + id
	sc.get(lotse, path, ok...)
	sc.matched(lotse, path, req{method: http.MethodPut, path: path,
		body: def("Triage again "+sc.Reg.New(ClassContent, allgemein, "command description (update)"))}, http.StatusOK)
	sc.get(alex, "/channels/"+allgemein+"/commands", ok...)
	sc.get(alex, path, ok...)
	invoke := "/channels/" + allgemein + "/commands/" + id + "/invoke"
	value := sc.Reg.New(ClassContent, allgemein, "command option value")
	sc.post(alex, invoke, map[string]any{"options": []any{map[string]any{"name": "ticket", "value": value}}}, http.StatusAccepted)
	sc.post(alex, invoke, map[string]any{"options": []any{map[string]any{"name": "ticket", "value": value + "‮"}}},
		http.StatusBadRequest)
	sc.del(lotse, path, http.StatusNoContent)
}
