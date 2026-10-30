// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// triggerersPath is getAgentTriggerers.
const triggerersPath = agentsPath + "/me/triggerers"

// wantSet checks principal_ids (ascending) and permission_version of a getAgentTriggerers body.
func wantSet(t *testing.T, w *world, m map[string]any, people ...ids.ID) {
	t.Helper()
	want := make([]string, 0, len(people))
	for _, p := range people {
		want = append(want, p.String())
	}
	slices.Sort(want)
	var got []string
	for _, v := range m["principal_ids"].([]any) {
		got = append(got, v.(string))
	}
	if !slices.Equal(got, want) {
		t.Errorf("principal_ids %v, want %v", got, want)
	}
	if v := int(m["permission_version"].(float64)); v != w.permissionVersion() {
		t.Errorf("permission_version %d, want %d", v, w.permissionVersion())
	}
}

// TestGetAgentTriggerers (getAgentTriggerers, agent.triggerers.read, 09 17.7): the agent reads the
// people who may trigger it, the owner and Alex with trigger on the channel it operates, but not
// Sam with only write or Robin with trigger on a channel of another agent; after the switch to
// personal the holder stays. A person is refused (agent-self), an anonymous call is 401.
// A read writes no allow audit entry (05 4.10 "Audit: no"); the refusals are audited as usual. The harness validates every body against openapi.yaml.
func TestGetAgentTriggerers(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	alex := w.person("alex", "active", false)
	sam := w.person("sam", "active", false)
	robin := w.person("robin", "active", false)
	lotse := w.create("lotse")
	texter := w.create("texter")
	w.agentToken("lotse", lotse, owner)
	support := w.channel(lotse)
	w.rawGrant(alex, support, "trigger")
	w.rawGrant(sam, support, "write")
	w.rawGrant(robin, w.channel(texter), "trigger")

	wantSet(t, w, w.want(w.do("lotse", http.MethodGet, triggerersPath, ""), http.StatusOK, ""), owner, alex)

	// I1: a personal agent has no other triggerers than its holder, so Alex's grant goes first.
	w.sql(`DELETE FROM grants WHERE subject_principal_id = $1`, alex.String())
	w.setPersonal(lotse, owner)
	wantSet(t, w, w.want(w.do("lotse", http.MethodGet, triggerersPath, ""), http.StatusOK, ""), owner)

	w.want(w.do("owner", http.MethodGet, triggerersPath, ""), http.StatusForbidden, "forbidden")
	w.want(w.do("", http.MethodGet, triggerersPath, ""), http.StatusUnauthorized, "")
	if rows := w.allows("agent.triggerers.read"); len(rows) != 0 {
		t.Errorf("allow audit rows %v", rows)
	}
}
