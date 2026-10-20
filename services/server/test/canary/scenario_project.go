// SPDX-License-Identifier: Apache-2.0

package canary

import "net/http"

// projectBinding drives putProjectBinding and deleteProjectBinding (M2-03) on the forum
// Engineering/Projekt-A with canary values of class manage_only in workspace_ref, the path of
// repository_url, default_branch and runner_config.model, and a canary reason of class
// admin_reason: the owner binds and reads the behaviour, Alex (no read on Engineering, 404) and
// Robin (trigger on the forum, 403 owner_only) are refused, and the owner removes the binding
// again. The values may appear only in the success
// bodies (no sink captures them) and the reason in audit_events.details; never in an event, a log,
// a trace, a metric or another audit field.
func (sc *Scenario) projectBinding() {
	sc.T.Helper()
	o, alex, robin := sc.W.Owner, sc.W.People["Alex"], sc.W.People["Robin"]
	forum := sc.W.Channels["Engineering/Projekt-A"]
	path := "/channels/" + forum + "/project-binding"
	body := func(label string) map[string]any {
		return map[string]any{
			"workspace_ref":  sc.Reg.New(ClassManageOnly, forum, "workspace_ref"+label),
			"repository_url": "https://git.example.com/team/" + sc.Reg.New(ClassManageOnly, forum, "repository_url path"+label),
			"default_branch": sc.Reg.New(ClassManageOnly, forum, "default_branch"+label),
			"runner_config":  map[string]any{"model": sc.Reg.New(ClassManageOnly, forum, "runner model"+label), "allowed_tools": []string{"Read"}},
			"reason":         "Bind " + sc.Reg.New(ClassAdminReason, forum, "project binding reason"+label),
		}
	}
	sc.put(o, path, body(""), ok...)
	sc.get(o, "/channels/"+forum+"/behavior", ok...)
	sc.put(alex, path, body(" (404)"), http.StatusNotFound)
	sc.put(robin, path, body(" (403)"), http.StatusForbidden)
	sc.del(robin, path, http.StatusForbidden)
	sc.del(o, path, none...)
}
