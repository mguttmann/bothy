// SPDX-License-Identifier: Apache-2.0

package canary

import "net/http"

// agentSessions drives the four session operations of M2-12 in the thread of the queued run of
// Lotse in Support/Allgemein (INT-W55/7): Lotse reports a session (the answer depends on the
// state of the queued run, so 404 and 409 are accepted), Alex (trigger in Support) lists the sessions of
// the thread, binds an external session to it and resets it.
func (sc *Scenario) agentSessions(thread string) {
	sc.T.Helper()
	lotse, alex := sc.W.Agents["Lotse"], sc.W.People["Alex"]
	base := "/threads/" + thread + "/agent-sessions"
	sc.post(lotse, base+"/report", map[string]any{"external_session_ref": "canary-session-reported"}, http.StatusOK, http.StatusNotFound,
		http.StatusConflict)
	cur := ""
	if list := items(sc.get(alex, base, ok...)); len(list) > 0 {
		cur = jsonText(list[0]["id"])
	}
	bind := map[string]any{"external_session_ref": "canary-session-bound"}
	if cur != "" {
		bind["expected_current_session_id"] = cur
	}
	sc.post(alex, base+"/bind", bind, http.StatusOK, http.StatusCreated, http.StatusConflict)
	sc.post(alex, base+"/reset", nil, http.StatusOK, http.StatusCreated, http.StatusConflict)
}
