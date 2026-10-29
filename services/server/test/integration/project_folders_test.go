// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// The tests of M4-44 (forums for existing folders): the heartbeat member project_folders, the
// column agents.reported_project_folders and listAgentProjectFolders (03 agents, R-DM-56; 09
// sections 4.8 and 18.13).

// folders reads listAgentProjectFolders of the runner as name.
func (w *pbWorld) folders(name string) *httpservertest.Response {
	w.t.Helper()
	return w.do(name, http.MethodGet, "/api/v1/agents/"+w.runner.String()+"/project-folders", "")
}

// folderNames returns the names of a ProjectFolderList and its member reported.
func folderNames(body map[string]any) ([]string, bool) {
	var out []string
	items, _ := body["items"].([]any)
	for _, it := range items {
		m, _ := it.(map[string]any)
		n, _ := m["name"].(string)
		out = append(out, n)
	}
	reported, _ := body["reported"].(bool)
	return out, reported
}

// TestListAgentProjectFolders: before a report the list is empty and not reported; after a
// heartbeat with project_folders the owner gets the names without the bound ones (compared without
// case); the column holds the reported list (R-DM-56, D-M4-44-4).
func TestListAgentProjectFolders(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	names, reported := folderNames(w.wantOK(w.folders("owner")))
	if len(names) != 0 || reported {
		t.Fatalf("before a report: %v reported %v", names, reported)
	}
	w.wantOK(w.beat("runner", `{"state":"online","project_folders":["api","docs","website"]}`))
	if got := w.text(`SELECT array_to_string(reported_project_folders, ',') FROM agents WHERE principal_id = $1`, w.runner.String()); got != "api,docs,website" {
		t.Fatalf("stored %q", got)
	}
	w.bind(w.projektB, "Website")
	names, reported = folderNames(w.wantOK(w.folders("owner")))
	if !slices.Equal(names, []string{"api", "docs"}) || !reported {
		t.Fatalf("after the binding: %v reported %v", names, reported)
	}
	w.wantOK(w.beat("runner", `{"state":"online","project_folders":[]}`))
	if names, reported := folderNames(w.wantOK(w.folders("owner"))); len(names) != 0 || !reported {
		t.Fatalf("after an empty report: %v reported %v", names, reported)
	}
}

// TestListAgentProjectFolders_RightsAndForm: a person who reads the runner but is not the owner
// gets 403 owner_only, a folder
// name with a path is a field error of the heartbeat, and the route names the action and class of
// the catalogue (AZ-P1).
func TestListAgentProjectFolders_RightsAndForm(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.wantReason("alex", w.folders("alex"), http.StatusForbidden, "forbidden", "owner_only")
	r := w.beat("runner", `{"state":"online","project_folders":["a/b"]}`)
	w.want(r, http.StatusBadRequest, "validation_failed")
	if fields := problemFields(t, r); len(fields) != 1 || fields[0] != "/project_folders/0:pattern_mismatch" {
		t.Fatalf("fields %v", fields)
	}
	i := slices.IndexFunc(routes.Enabled(), func(o routes.Operation) bool { return o.ID == "listAgentProjectFolders" })
	if i < 0 || routes.Enabled()[i].Action != "agent.project_folders.read" || routes.Enabled()[i].RateLimitClass != "rl.read" {
		t.Fatalf("route listAgentProjectFolders not enabled as agent.project_folders.read in rl.read")
	}
}
