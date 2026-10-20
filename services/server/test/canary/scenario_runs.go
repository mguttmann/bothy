// SPDX-License-Identifier: Apache-2.0

package canary

import "net/http"

// runs drives the four run reads of M2-05: the first message of the owner in the thread of
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
	sc.get(alex, "/runs/"+run, ok...)
	sc.get(alex, "/runs/"+run+"/steps", ok...)
	sc.get(alex, "/runs", ok...)
	sc.get(sam, "/runs/"+run, ok...)
	sc.get(sam, "/runs/"+run+"/steps?limit=1", ok...)
	// A run of Marketing is invisible to Alex: the same 404 as an unknown run.
	foreign := items(sc.get(sc.W.Owner, "/threads/"+sc.W.Threads["Marketing/Kampagnen"]+"/runs", ok...))
	if len(foreign) > 0 {
		sc.get(alex, "/runs/"+jsonText(foreign[0]["id"]), http.StatusNotFound)
	}
}
