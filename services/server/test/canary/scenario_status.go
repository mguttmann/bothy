// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"net/http"
	"time"
)

// statusPage drives the seven operations of the status page (M7-09): the owner adds a component
// with a canary name and description, changes the description, opens an incident on it with a
// canary title and text, adds a resolving update with a canary text, reads the page, deletes the
// component and asks for the public page (404 while status.public_enabled is off). The texts are
// content: they may appear in the owner's 2xx bodies only, never in a log, trace, metric or audit
// row (the audit names the operation and the ID). No observer reads the page, because every
// person with status.read may see a standard component and its incidents.
func (sc *Scenario) statusPage() {
	sc.T.Helper()
	owner := sc.W.Owner
	name := sc.Reg.New(ClassContent, "", "status component name")
	comp := obj(sc.call(owner, req{method: http.MethodPost, path: "/status/components", body: map[string]any{
		"name": name, "description": sc.Reg.New(ClassContent, "", "status component description"),
	}}, created...))
	id := jsonText(comp["id"])
	sc.call(owner, req{method: http.MethodPatch, path: "/status/components/" + id, body: map[string]any{
		"name": name, "description": sc.Reg.New(ClassContent, "", "status component description (update)"),
	}}, ok...)
	inc := obj(sc.call(owner, req{method: http.MethodPost, path: "/status/incidents", body: map[string]any{
		"title": sc.Reg.New(ClassContent, "", "status incident title"), "kind": "incident", "impact": "minor",
		"started_at": timestamp(sc.Server.Clock().Now().Add(-time.Minute)), "component_ids": []string{id},
		"body_md": sc.Reg.New(ClassContent, "", "status incident text"),
	}}, created...))
	sc.call(owner, req{method: http.MethodPost, path: "/status/incidents/" + jsonText(inc["id"]) + "/updates", body: map[string]any{
		"state": "resolved", "body_md": sc.Reg.New(ClassContent, "", "status incident update"),
	}}, created...)
	sc.get(owner, "/status", ok...)
	sc.call(owner, req{method: http.MethodDelete, path: "/status/components/" + id}, none...)
	sc.anon(owner, req{method: http.MethodGet, path: "/status/public"}, http.StatusNotFound)
}
