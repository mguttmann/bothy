// SPDX-License-Identifier: Apache-2.0

package canary

import "net/http"

// runs drives the four run reads of M2-05, the live-box patch of M2-10 and the session operations
// of M2-12: the first message of the owner in the thread of
// Support/Allgemein opened a run of Lotse (BuildMatrixWorld); Alex (reads Support) lists the runs
// of the thread and "Running now", reads the run document and its steps, and Sam, who reads
// Support/Allgemein through Leser, reads it too, so the pages of both carry the run summaries.
func (sc *Scenario) runs() {
	t := sc.T
	t.Helper()
	alex, sam := sc.W.People["Alex"], sc.W.People["Sam"]
	thread := sc.W.Threads["Support/Allgemein"]
	list := items(sc.get(alex, "/threads/"+thread+"/runs", ok...))
	if len(list) == 0 {
		t.Errorf("runs: the thread of Support/Allgemein has no run")
		return
	}
	run := jsonText(list[0]["id"])
	doc := obj(sc.get(alex, "/runs/"+run, ok...))
	// Lotse patches the live box of its run (patchRun, M2-10); an ended run answers 409.
	sc.patch(sc.W.Agents["Lotse"], "/runs/"+run, map[string]any{"base_version": doc["version"],
		"ops": []any{map[string]any{"op": "replace", "path": "/status_line", "value": "Reading the export log"}}},
		http.StatusOK, http.StatusConflict)
	sc.get(alex, "/runs/"+run+"/steps", ok...)
	sc.get(alex, "/runs", ok...)
	sc.get(sam, "/runs/"+run, ok...)
	sc.get(sam, "/runs/"+run+"/steps?limit=1", ok...)
	// A run of Marketing is invisible to Alex: the same 404 as an unknown run.
	foreign := items(sc.get(sc.W.Owner, "/threads/"+sc.W.Threads["Marketing/Kampagnen"]+"/runs", ok...))
	if len(foreign) > 0 {
		sc.get(alex, "/runs/"+jsonText(foreign[0]["id"]), http.StatusNotFound)
	}
	sc.runLifecycle(run)
	sc.agentSessions(thread)
}

// runLifecycle drives the six lifecycle operations of M2-09 on the queued run of Lotse without
// changing it: start, resume and renew present a forged invocation token (a secret canary, which
// no output may carry) and are refused, waiting for input and finishing are no transitions of a
// queued run, and Alex, who did not trigger the run, may not cancel it.
func (sc *Scenario) runLifecycle(run string) {
	sc.T.Helper()
	lotse, alex := sc.W.Agents["Lotse"], sc.W.People["Alex"]
	forged := sc.Reg.New(ClassSecret, "", "forged invocation token")
	start := map[string]any{"invocation_token": forged, "billing_mode_used": "organization", "tool_class_used": "chat"}
	refused := []int{http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound, http.StatusConflict}
	sc.post(lotse, "/runs/"+run+"/start", start, refused...)
	sc.post(lotse, "/runs/"+run+"/resume", start, refused...)
	sc.post(lotse, "/runs/"+run+"/invocation-token", map[string]any{"invocation_token": forged}, refused...)
	sc.post(lotse, "/runs/"+run+"/await-input", map[string]any{"status_line": "waiting"}, http.StatusConflict)
	sc.post(lotse, "/runs/"+run+"/finish", map[string]any{"state": "completed"}, http.StatusConflict)
	sc.call(alex, req{method: http.MethodPost, path: "/runs/" + run + "/cancel", body: map[string]any{},
		header: header("Idempotency-Key", newID())}, http.StatusForbidden, http.StatusNotFound)
}
