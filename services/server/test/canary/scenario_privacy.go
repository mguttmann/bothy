// SPDX-License-Identifier: Apache-2.0

package canary

import "net/http"

// privacyRequests drives the five operations of the privacy requests (M7-06): Alex files an export
// of Alex's own data (approved by the system, so the worker builds the package) and a
// rectification, lists and reads them and asks for the package (200 once the job finished, 409
// before); the owner approves the rectification. The reasons are administrative reasons: they may
// appear in the audit details under reason and in the bodies and events of the request, never in
// a log, trace or metric. No deletion is filed: the scenario keeps Alex for the later steps.
func (sc *Scenario) privacyRequests() {
	sc.T.Helper()
	owner, alex := sc.W.Owner, sc.W.People["Alex"]
	export := obj(sc.post(alex, "/gdpr-requests", map[string]any{
		"kind": "export", "reason": sc.Reg.New(ClassAdminReason, "", "privacy export reason"),
	}, created...))
	id := jsonText(export["id"])
	sc.get(alex, "/gdpr-requests", ok...)
	sc.get(alex, "/gdpr-requests/"+id, ok...)
	sc.get(alex, "/gdpr-requests/"+id+"/export", http.StatusOK, http.StatusConflict)
	rect := obj(sc.post(alex, "/gdpr-requests", map[string]any{
		"kind": "rectification", "reason": sc.Reg.New(ClassAdminReason, "", "privacy rectification reason"),
	}, created...))
	sc.post(owner, "/gdpr-requests/"+jsonText(rect["id"])+"/decide", map[string]any{
		"decision": "approve", "reason": sc.Reg.New(ClassAdminReason, "", "privacy decision reason"),
	}, ok...)
}
