// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestCreateAgent: the principal and the record exist with the defaults of 03, agent.updated is
// in event_log, the allow entry is written, permission_version is raised; a repeat with the same
// Idempotency-Key returns the stored response and creates nothing.
func TestCreateAgent(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.person("owner", "active", true)
	before := w.permissionVersion()
	body := `{"handle":"lotse","display_name":"Lotse","adapter_kind":"code_runner","description":"Answers","operator_config":{"supports_voice":false}}`
	k := key()
	m := w.want(w.do("owner", http.MethodPost, agentsPath, body, k...), http.StatusCreated, "")
	id := ids.MustParse(m["id"].(string))
	if m["billing_mode"] != "organization" || m["tool_class"] != "chat" || m["status"] != "active" || m["version"] != float64(1) ||
		m["adapter_kind"] != "code_runner" || m["description"] != "Answers" {
		t.Fatalf("created %v", m)
	}
	if p := m["presence"].(map[string]any); p["state"] != "unknown" {
		t.Fatalf("presence %v", p)
	}
	if c := m["operator_config"].(map[string]any); c["supports_voice"] != false || len(c) != 1 {
		t.Fatalf("operator_config %v", c)
	}
	if _, ok := m["credential_holder_id"]; ok {
		t.Fatal("an organization agent with a credential holder")
	}
	if w.text(`SELECT kind || '/' || status || '/' || handle FROM principals WHERE id = $1`, id.String()) != "agent/active/lotse" {
		t.Fatal("principal missing or wrong")
	}
	if w.text(`SELECT billing_mode || '/' || tool_class || '/' || reported_runtime_state || '/' || version FROM agents WHERE principal_id = $1`,
		id.String()) != "organization/chat/unknown/1" {
		t.Fatal("record missing or wrong")
	}
	evs := w.updates()
	if len(evs) != 1 || evs[0].ResourceID != id.String() || evs[0].ReceiveAction != "agent.read" || evs[0].Version != 1 || evs[0].HasScope {
		t.Fatalf("agent.updated %+v", evs)
	}
	if a := evs[0].Payload["agent"].(map[string]any); a["billing_mode"] != "organization" || a["handle"] != "lotse" {
		t.Fatalf("payload %v", evs[0].Payload)
	}
	if allows := w.allows("agent.create"); len(allows) != 1 || allows[0].ResourceID != id.String() {
		t.Fatalf("audit %+v", allows)
	}
	if w.permissionVersion() != before+1 {
		t.Fatalf("permission_version %d, want %d", w.permissionVersion(), before+1)
	}
	again := w.want(w.do("owner", http.MethodPost, agentsPath, body, k...), http.StatusCreated, "")
	if again["id"] != m["id"] || w.count(`SELECT count(*) FROM agents`) != 1 || len(w.updates()) != 1 {
		t.Fatalf("replay %v", again)
	}
}

// TestCreateAgentHandleConflictCaseInsensitive: a handle a person holds is 409 conflict on
// /handle; the API pattern takes only lower case, so "Alex" is refused before the database
// (docs/DECISIONS.md M1-51), and the unique index on lower(handle) guards the rest.
func TestCreateAgentHandleConflictCaseInsensitive(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.person("owner", "active", true)
	w.person("alex", "active", false)
	p := w.want(w.do("owner", http.MethodPost, agentsPath, `{"handle":"alex","display_name":"Alex Bot","adapter_kind":"hermes"}`, key()...),
		http.StatusConflict, "conflict")
	wantPointer(t, p, "/handle")
	w.want(w.do("owner", http.MethodPost, agentsPath, `{"handle":"Alex","display_name":"Alex Bot","adapter_kind":"hermes"}`, key()...),
		http.StatusBadRequest, "validation_failed")
	if w.count(`SELECT count(*) FROM agents`) != 0 || w.count(`SELECT count(*) FROM principals WHERE kind = 'agent'`) != 0 {
		t.Fatal("a refused create left rows")
	}
}

// TestCreateAgentRejectsBidiDisplayName (SEC-T07, part agents): control and bidi characters in
// the display name, an adapter kind outside its pattern and an invalid operator_config are 400
// with the pointer.
func TestCreateAgentRejectsBidiDisplayName(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.person("owner", "active", true)
	cases := []struct{ body, pointer string }{
		{`{"handle":"lotse","display_name":"Lot\u202ese","adapter_kind":"hermes"}`, "/display_name"},
		{`{"handle":"lotse","display_name":"Lot\u0007se","adapter_kind":"hermes"}`, "/display_name"},
		{`{"handle":"lotse","display_name":"Lotse","adapter_kind":"Hermes"}`, "/adapter_kind"},
		{`{"handle":"lotse","display_name":"Lotse","adapter_kind":"hermes","operator_config":{"api_key":"x"}}`, "/operator_config/api_key"},
		{`{"handle":"lotse","display_name":"Lotse","adapter_kind":"hermes","operator_config":{"default_model":"a b"}}`, "/operator_config/default_model"},
	}
	for _, c := range cases {
		p := w.want(w.do("owner", http.MethodPost, agentsPath, c.body, key()...), http.StatusBadRequest, "validation_failed")
		wantPointer(t, p, c.pointer)
	}
	if w.count(`SELECT count(*) FROM principals WHERE kind = 'agent'`) != 0 {
		t.Fatal("a refused create left a principal")
	}
}

// wantPointer checks that the problem p names pointer in its errors.
func wantPointer(t *testing.T, p map[string]any, pointer string) {
	t.Helper()
	list, _ := p["errors"].([]any)
	for _, e := range list {
		if m, _ := e.(map[string]any); m["pointer"] == pointer {
			return
		}
	}
	t.Fatalf("problem %v without pointer %s", p, pointer)
}

// TestUpdateAgentIfMatch: without If-Match 428, a stale one 412 version_conflict with current, a
// fresh one 200 with version + 1 and the ETag; the display name is changed through identity.
func TestUpdateAgentIfMatch(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.person("owner", "active", true)
	id := w.create("lotse")
	path := agentPath(id)
	w.want(w.do("owner", http.MethodPatch, path, `{"display_name":"Lotse Zwei"}`), http.StatusPreconditionRequired, "precondition_required")
	p := w.want(w.do("owner", http.MethodPatch, path, `{"display_name":"Lotse Zwei"}`, ifMatch(7)...), http.StatusPreconditionFailed, "version_conflict")
	if cur, _ := p["current"].(map[string]any); cur["version"] != float64(1) || cur["id"] != id.String() {
		t.Fatalf("current %v", p["current"])
	}
	r := w.do("owner", http.MethodPatch, path, `{"display_name":"Lotse Zwei","description":"Pilot"}`, ifMatch(1)...)
	m := w.want(r, http.StatusOK, "")
	if m["version"] != float64(2) || m["display_name"] != "Lotse Zwei" || m["description"] != "Pilot" || r.Header.Get("ETag") != `"2"` {
		t.Fatalf("updated %v, ETag %s", m, r.Header.Get("ETag"))
	}
	if w.text(`SELECT display_name FROM principals WHERE id = $1`, id.String()) != "Lotse Zwei" {
		t.Fatal("display name not stored")
	}
	allows := w.allows("agent.update")
	if len(allows) != 1 || !slices.Equal(allows[0].Details["changed_fields"].([]any), []any{"display_name", "description"}) {
		t.Fatalf("audit %+v", allows)
	}
	// An update without change: 200, same version, no event, audit no_change.
	events := len(w.updates())
	m = w.want(w.do("owner", http.MethodPatch, path, `{"display_name":"Lotse Zwei"}`, ifMatch(2)...), http.StatusOK, "")
	if m["version"] != float64(2) || len(w.updates()) != events {
		t.Fatalf("no-op update %v", m)
	}
	if allows := w.allows("agent.update"); len(allows) != 2 || allows[1].Details["no_change"] != true {
		t.Fatalf("no-op audit %+v", allows)
	}
	// The operator configuration is replaced as a whole.
	m = w.want(w.do("owner", http.MethodPatch, path, `{"operator_config":{"default_model":"vendor/model-2"}}`, ifMatch(2)...), http.StatusOK, "")
	if c := m["operator_config"].(map[string]any); c["default_model"] != "vendor/model-2" || m["version"] != float64(3) {
		t.Fatalf("config update %v", m)
	}
	p = w.want(w.do("owner", http.MethodPatch, path, `{"operator_config":{"supports_voice":"no"}}`, ifMatch(3)...), http.StatusBadRequest, "validation_failed")
	wantPointer(t, p, "/operator_config/supports_voice")
}

// TestDisableRevokesTokens: an agent token works before and gives 401 after disableAgent, every
// token of the agent is revoked in api_tokens, AgentDisabled was published inside the
// transaction; disabling a disabled agent is 200 without change (06 section 2 "Ohne Wirkung",
// docs/DECISIONS.md M1-51); enabling restores no token.
func TestDisableRevokesTokens(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	id := w.create("lotse")
	w.agentToken("lotse", id, owner)
	w.agentToken("lotse2", id, owner)
	w.want(w.do("lotse", http.MethodGet, agentPath(id), ""), http.StatusOK, "")
	before := w.permissionVersion()
	m := w.want(w.do("owner", http.MethodPost, agentPath(id)+"/disable", `{"reason":"Retired"}`, key()...), http.StatusOK, "")
	if m["status"] != "disabled" || m["version"] != float64(2) {
		t.Fatalf("disabled %v", m)
	}
	if w.count(`SELECT count(*) FROM api_tokens WHERE principal_id = $1 AND revoked_at IS NULL`, id.String()) != 0 {
		t.Fatal("an active token survived")
	}
	if w.count(`SELECT count(*) FROM principals WHERE id = $1 AND disabled_at IS NOT NULL`, id.String()) != 1 {
		t.Fatal("disabled_at not set")
	}
	w.want(w.do("lotse", http.MethodGet, agentPath(id), ""), http.StatusUnauthorized, "")
	w.mu.Lock()
	got := slices.Clone(*w.disabled)
	w.mu.Unlock()
	if len(got) != 1 || got[0].AgentID != id {
		t.Fatalf("AgentDisabled %+v", got)
	}
	if w.permissionVersion() != before+1 {
		t.Fatal("disable is a rights change")
	}
	allows := w.allows("agent.disable")
	if len(allows) != 1 || allows[0].Details["revoked_tokens"] != float64(2) || allows[0].Details["reason"] != "Retired" {
		t.Fatalf("audit %+v", allows)
	}
	events := len(w.updates())
	m = w.want(w.do("owner", http.MethodPost, agentPath(id)+"/disable", `{"reason":"Again"}`, key()...), http.StatusOK, "")
	if m["status"] != "disabled" || m["version"] != float64(2) || len(w.updates()) != events || w.permissionVersion() != before+1 {
		t.Fatalf("repeat disable %v", m)
	}
	if allows := w.allows("agent.disable"); len(allows) != 2 || allows[1].Details["no_change"] != true {
		t.Fatalf("repeat audit %+v", allows)
	}
	m = w.want(w.do("owner", http.MethodPost, agentPath(id)+"/enable", `{"reason":"Back"}`, key()...), http.StatusOK, "")
	if m["status"] != "active" || m["version"] != float64(3) {
		t.Fatalf("enabled %v", m)
	}
	w.want(w.do("lotse", http.MethodGet, agentPath(id), ""), http.StatusUnauthorized, "")
	m = w.want(w.do("owner", http.MethodPost, agentPath(id)+"/enable", `{"reason":"Again"}`, key()...), http.StatusOK, "")
	if m["version"] != float64(3) {
		t.Fatalf("repeat enable %v", m)
	}
	if allows := w.allows("agent.enable"); len(allows) != 2 || allows[1].Details["no_change"] != true {
		t.Fatalf("enable audit %+v", allows)
	}
}

// TestAgentReadsItself: with its own token the agent reads itself including billing_mode and
// credential_holder_id; another agent it does not serve is 404.
func TestAgentReadsItself(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	lotse := w.create("lotse")
	scout := w.create("scout")
	w.setPersonal(lotse, owner)
	w.agentToken("lotse", lotse, owner)
	m := w.want(w.do("lotse", http.MethodGet, agentPath(lotse), ""), http.StatusOK, "")
	if m["billing_mode"] != "personal" || m["credential_holder_id"] != owner.String() {
		t.Fatalf("self read %v", m)
	}
	w.want(w.do("lotse", http.MethodGet, agentPath(scout), ""), http.StatusNotFound, "not_found")
	w.want(w.do("lotse", http.MethodGet, agentPath(ids.New()), ""), http.StatusNotFound, "not_found")
	page := w.want(w.do("lotse", http.MethodGet, agentsPath, ""), http.StatusOK, "")
	if items := page["items"].([]any); len(items) != 1 || items[0].(map[string]any)["id"] != lotse.String() {
		t.Fatalf("the agent lists %v", page)
	}
}

// TestListAgentsVisibility: the owner sees all, page by page with a cursor; Sam with read on a
// channel served by Lotse sees Lotse only; Robin sees none; anonymous is 401.
func TestListAgentsVisibility(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.person("owner", "active", true)
	sam := w.person("sam", "active", false)
	w.person("robin", "active", false)
	created := []ids.ID{w.create("assistent"), w.create("lotse"), w.create("scout")}
	lotse := created[1]
	w.rawGrant(sam, w.channel(lotse), "read")
	first := w.want(w.do("owner", http.MethodGet, agentsPath+"?limit=2", ""), http.StatusOK, "")
	items := first["items"].([]any)
	cursor, _ := first["next_cursor"].(string)
	if len(items) != 2 || cursor == "" || items[0].(map[string]any)["id"] != created[0].String() {
		t.Fatalf("first page %v", first)
	}
	second := w.want(w.do("owner", http.MethodGet, agentsPath+"?limit=2&cursor="+cursor, ""), http.StatusOK, "")
	if items := second["items"].([]any); len(items) != 1 || items[0].(map[string]any)["id"] != created[2].String() {
		t.Fatalf("second page %v", second)
	}
	if _, ok := second["next_cursor"]; ok {
		t.Fatal("next_cursor on the last page")
	}
	w.want(w.do("sam", http.MethodGet, agentsPath+"?cursor="+cursor, ""), http.StatusBadRequest, "cursor_invalid")
	samPage := w.want(w.do("sam", http.MethodGet, agentsPath, ""), http.StatusOK, "")
	if items := samPage["items"].([]any); len(items) != 1 || items[0].(map[string]any)["id"] != lotse.String() {
		t.Fatalf("Sam sees %v", samPage)
	}
	w.want(w.do("sam", http.MethodGet, agentPath(lotse), ""), http.StatusOK, "")
	w.want(w.do("sam", http.MethodGet, agentPath(created[0]), ""), http.StatusNotFound, "not_found")
	if items := w.want(w.do("robin", http.MethodGet, agentsPath, ""), http.StatusOK, "")["items"].([]any); len(items) != 0 {
		t.Fatalf("Robin sees %v", items)
	}
	w.want(w.do("", http.MethodGet, agentsPath, ""), http.StatusUnauthorized, "")
}

// TestAgentUpdatedCarriesBadge: after each switch the payload of agent.updated carries the new
// billing_mode, and authorize(agent, agent.read, itself) allows its delivery to the agent.
func TestAgentUpdatedCarriesBadge(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	id := w.create("lotse")
	w.agentToken("lotse", id, owner)
	w.setPersonal(id, owner)
	w.want(w.do("owner", http.MethodPut, agentPath(id)+"/billing-mode", `{"billing_mode":"organization","reason":"Company key"}`,
		ifMatch(w.version(id))...), http.StatusOK, "")
	evs := w.updates()
	var modes []string
	for _, e := range evs {
		modes = append(modes, e.Payload["agent"].(map[string]any)["billing_mode"].(string))
	}
	if !slices.Equal(modes, []string{"organization", "personal", "organization"}) {
		t.Fatalf("badges %v", modes)
	}
	if evs[1].Payload["agent"].(map[string]any)["credential_holder_id"] != owner.String() || evs[1].Version != 2 {
		t.Fatalf("personal event %+v", evs[1])
	}
	self := w.actors.Get("lotse")
	d, err := w.az.Authorize(context.Background(), self, authorize.ActionAgentRead,
		authorize.Resource{Kind: authorize.ResourceAgent, ID: id, OwnerID: id})
	if err != nil || !d.Allowed() || self.Kind != actor.KindAgent {
		t.Fatalf("delivery to the agent: %+v, %v", d, err)
	}
}

// TestSetToolClass: a change sets the class with version + 1, event, rights version and audit of
// old and new class; the same class is 200 without change; If-Match is required.
func TestSetToolClass(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.person("owner", "active", true)
	id := w.create("lotse")
	path := agentPath(id) + "/tool-class"
	w.want(w.do("owner", http.MethodPut, path, `{"tool_class":"scoped","reason":"Checkout"}`), http.StatusPreconditionRequired, "precondition_required")
	before := w.permissionVersion()
	r := w.do("owner", http.MethodPut, path, `{"tool_class":"scoped","reason":"Checkout"}`, ifMatch(1)...)
	m := w.want(r, http.StatusOK, "")
	if m["tool_class"] != "scoped" || m["version"] != float64(2) || r.Header.Get("ETag") != `"2"` || w.permissionVersion() != before+1 {
		t.Fatalf("set %v", m)
	}
	allows := w.allows("agent.tool_class.set")
	if len(allows) != 1 || allows[0].Details["from_tool_class"] != "chat" || allows[0].Details["to_tool_class"] != "scoped" {
		t.Fatalf("audit %+v", allows)
	}
	pol, ok, err := w.source.AgentPolicy(context.Background(), id)
	if err != nil || !ok || pol.ToolClass != "scoped" {
		t.Fatalf("policy source %+v, %v, %v", pol, ok, err)
	}
	events := len(w.updates())
	m = w.want(w.do("owner", http.MethodPut, path, `{"tool_class":"scoped","reason":"Again"}`, ifMatch(2)...), http.StatusOK, "")
	if m["version"] != float64(2) || len(w.updates()) != events || w.permissionVersion() != before+1 {
		t.Fatalf("same class %v", m)
	}
	if allows := w.allows("agent.tool_class.set"); len(allows) != 2 || allows[1].Details["no_change"] != true {
		t.Fatalf("same class audit %+v", allows)
	}
}
