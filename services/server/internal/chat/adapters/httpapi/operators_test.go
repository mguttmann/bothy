// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestOperatorHandlers (M1-54): both handlers convert OperatorSet and answer the object with its
// ETag; the I1 cases with all real modules are in services/server/test/integration.
func TestOperatorHandlers(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	agent := ids.New()
	w.sql(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'agent', 'active', 'lotse', 'Lotse')`, agent.String())
	w.sql(`INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'hermes')`, agent.String())
	// The world's agent policy source is the fake of M1-55 (INT-W17/3): it names the agents row.
	w.policies.Put(agent, authz.AgentPolicy{BillingMode: "organization", ToolClass: "chat"})
	sp := w.create("Support")
	ch := w.channel(sp, "text", "tickets")
	r := w.do("owner", http.MethodPut, spacePath(sp)+"/operator", `{"mode":"agent","agent_id":"`+agent.String()+`","reason":"Support agent"}`,
		append(ifMatch(1), key()...)...)
	m := w.want(r, http.StatusOK, "")
	if m["default_operator_agent_id"] != agent.String() || r.Header.Get("ETag") != `"2"` {
		t.Fatalf("space %v %q", m, r.Header.Get("ETag"))
	}
	r = w.do("owner", http.MethodPut, channelPath(ch)+"/operator", `{"mode":"none","reason":"Quiet"}`, append(ifMatch(int64(w.channelVersion(ch))), key()...)...)
	m = w.want(r, http.StatusOK, "")
	if op := m["operator"].(map[string]any); op["mode"] != "none" || op["effective_agent_id"] != nil {
		t.Fatalf("channel %v", m)
	}
	w.want(w.do("owner", http.MethodPut, channelPath(ch)+"/operator", `{"mode":"agent","reason":"x"}`,
		append(ifMatch(int64(w.channelVersion(ch))), key()...)...), http.StatusBadRequest, "validation_failed")
	w.want(w.do("owner", http.MethodPut, spacePath(sp)+"/operator", `{"mode":"none","reason":"x"}`, append(ifMatch(9), key()...)...),
		http.StatusPreconditionFailed, "version_conflict")
}
