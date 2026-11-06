// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// The tests of A1-03 (hosted agents, docs/spec/26-builtin-agents.md 3 and 9) through the HTTP
// chain with response validation against openapi.yaml: createAgent with hosted_by, the heartbeat
// capability hosts_agents, setAgentHost, listHostedAgents, issueHostedAgentToken with its one-time
// secret, and BA-T08 (a built-in agent never serves a bound forum).

// hostingJKT is a well-formed RFC 7638 thumbprint (43 base64url characters), no real key.
const hostingJKT = "EXAMPLEexampleEXAMPLEexampleEXAMPLEexample0"

// keyedRunner registers the token name "runner-key": a second API token of Runner bound to
// hostingJKT, which a hosted token takes its key from.
func (w *world) keyedRunner() {
	w.t.Helper()
	issued, err := w.id.Tokens.Issue(context.Background(), identity.IssueTokenCmd{
		PrincipalID: w.runner, Name: "Runner key", DPoPJKT: hostingJKT, ExpiresAt: w.clk.Now().Add(time.Hour), CreatedBy: w.owner,
	})
	must(w.t, err)
	w.setActor("runner-key", httpservertest.WithToken(w.runner, actor.KindAgent, issued.Token.ID))
}

// createBuiltIn creates the built-in agent writer hosted by host through the API and returns the
// answer.
func (w *world) createBuiltIn(host ids.ID) *httpservertest.Response {
	w.t.Helper()
	return w.do("owner", http.MethodPost, "/api/v1/agents",
		`{"handle":"writer","display_name":"Writer","adapter_kind":"claude_agent","hosted_by":"`+host.String()+`"}`, key()...)
}

// hostsAgents sends a heartbeat of Runner reporting the ceiling hosts_agents.
func (w *world) hostsAgents(ceiling string) {
	w.t.Helper()
	w.wantOK(w.beat("runner", `{"state":"online","billing_modes_supported":["organization","personal"],
		"details":{"capabilities":{"hosts_agents":"`+ceiling+`"}}}`))
}

// fieldCodes returns the pointer:code pairs of a validation problem.
func fieldCodes(t *testing.T, r *httpservertest.Response) []string {
	t.Helper()
	return problemFields(t, r)
}

// TestHostedAgentsAPI: a runner reports hosts_agents, an owner creates a built-in agent on it, the
// runner lists it and fetches its token once (a replay of the Idempotency-Key is one_time_secret),
// a built-in agent's own heartbeat may not report hosts_agents, and the routes carry the actions
// and classes of the catalogue (AZ-P1).
func TestHostedAgentsAPI(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	r := w.createBuiltIn(w.runner)
	w.want(r, http.StatusBadRequest, "validation_failed")
	if f := fieldCodes(t, r); len(f) != 1 || f[0] != "/hosted_by:host_not_capable" {
		t.Fatalf("before hosts_agents: %v", f)
	}
	w.hostsAgents("scoped")
	if got := w.text(`SELECT reported_details->'capabilities'->>'hosts_agents' FROM agents WHERE principal_id = $1`, w.runner.String()); got != "scoped" {
		t.Fatalf("stored hosts_agents %q", got)
	}
	m := w.want(w.createBuiltIn(w.runner), http.StatusCreated, "")
	writer := ids.MustParse(m["id"].(string))
	if m["hosted_by"] != w.runner.String() || m["adapter_kind"] != "claude_agent" {
		t.Fatalf("created %v", m)
	}
	if m := w.wantOK(w.do("owner", http.MethodGet, "/api/v1/agents/"+w.lotse.String(), "")); m["hosted_by"] != nil {
		t.Fatalf("an agent that is not built in shows hosted_by: %v", m)
	}
	list := w.wantOK(w.do("runner", http.MethodGet, "/api/v1/agents/me/hosted", ""))
	if items, _ := list["items"].([]any); len(items) != 1 || items[0].(map[string]any)["id"] != writer.String() {
		t.Fatalf("hosted list %v", list)
	}
	// A second built-in agent pages the list: limit 1 answers next_cursor, the cursor the rest.
	w.want(w.do("owner", http.MethodPost, "/api/v1/agents",
		`{"handle":"reader","display_name":"Reader","adapter_kind":"claude_agent","hosted_by":"`+w.runner.String()+`"}`, key()...), http.StatusCreated, "")
	first := w.wantOK(w.do("runner", http.MethodGet, "/api/v1/agents/me/hosted?limit=1", ""))
	cursor, _ := first["next_cursor"].(string)
	if items, _ := first["items"].([]any); len(items) != 1 || cursor == "" {
		t.Fatalf("first hosted page %v", first)
	}
	second := w.wantOK(w.do("runner", http.MethodGet, "/api/v1/agents/me/hosted?limit=1&cursor="+url.QueryEscape(cursor), ""))
	if items, _ := second["items"].([]any); len(items) != 1 || items[0].(map[string]any)["id"] == first["items"].([]any)[0].(map[string]any)["id"] {
		t.Fatalf("second hosted page %v after %v", second, first)
	}
	if items, _ := w.wantOK(w.do("lotse", http.MethodGet, "/api/v1/agents/me/hosted", ""))["items"].([]any); len(items) != 0 {
		t.Fatalf("another agent's hosted list %v", items)
	}
	w.want(w.do("owner", http.MethodGet, "/api/v1/agents/me/hosted", ""), http.StatusForbidden, "forbidden")

	w.keyedRunner()
	path := "/api/v1/agents/me/hosted/" + writer.String() + "/token"
	idem := key()
	tok := w.want(w.do("runner-key", http.MethodPost, path, "", idem...), http.StatusCreated, "")
	token, _ := tok["token"].(map[string]any)
	if token["principal_id"] != writer.String() || token["dpop_jkt"] != hostingJKT || tok["secret"] == "" {
		t.Fatalf("hosted token %v", tok)
	}
	w.wantReasonCode(w.do("runner-key", http.MethodPost, path, "", idem...), http.StatusConflict, "conflict", "one_time_secret")
	w.wantReasonCode(w.do("runner", http.MethodPost, path, "", key()...), http.StatusConflict, "conflict", "dpop_required")
	w.want(w.do("lotse", http.MethodPost, path, "", key()...), http.StatusNotFound, "not_found")
	w.want(w.do("owner", http.MethodPost, path, "", key()...), http.StatusNotFound, "not_found")
	if secret, _ := tok["secret"].(string); secret == "" || w.count(`SELECT count(*) FROM idempotency_keys WHERE coalesce(response_body::text, '') LIKE '%' || $1 || '%'`, secret) != 0 {
		t.Fatal("the secret is stored in an idempotency record")
	}

	// A built-in agent's own heartbeat never carries hosts_agents (26 3.2 item 2).
	w.setActor("writer", httpservertest.WithToken(writer, actor.KindAgent, ids.MustParse(token["id"].(string))))
	hb := w.beat("writer", `{"state":"online","details":{"capabilities":{"hosts_agents":"chat"}}}`)
	w.want(hb, http.StatusBadRequest, "validation_failed")
	if f := fieldCodes(t, hb); len(f) != 1 || f[0] != "/details/capabilities/hosts_agents:not_allowed" {
		t.Fatalf("hosted heartbeat with hosts_agents: %v", f)
	}
	w.wantOK(w.beat("writer", `{"state":"online","details":{"capabilities":{"title":true}}}`))

	for op, want := range map[string][2]string{
		"setAgentHost": {"agent.host.set", "rl.admin"}, "listHostedAgents": {"agent.hosted.list", "rl.agent_runtime"},
		"issueHostedAgentToken": {"agent.hosted_token.issue", "rl.agent_runtime"},
	} {
		i := slices.IndexFunc(routes.Enabled(), func(o routes.Operation) bool { return o.ID == op })
		if i < 0 || routes.Enabled()[i].Action != want[0] || routes.Enabled()[i].RateLimitClass != want[1] {
			t.Errorf("route %s not enabled as %s in %s", op, want[0], want[1])
		}
	}
}

// TestSetAgentHostAPI: the owner moves a built-in agent with If-Match; the answer is the agent
// with its new runner and ETag, and the old runner's token of the agent no longer authenticates.
func TestSetAgentHostAPI(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.hostsAgents("host")
	writer := ids.MustParse(w.want(w.createBuiltIn(w.runner), http.StatusCreated, "")["id"].(string))
	w.keyedRunner()
	tok := w.want(w.do("runner-key", http.MethodPost, "/api/v1/agents/me/hosted/"+writer.String()+"/token", "", key()...), http.StatusCreated, "")
	tokenID := ids.MustParse(tok["token"].(map[string]any)["id"].(string))
	w.setActor("writer", httpservertest.WithToken(writer, actor.KindAgent, tokenID))
	w.wantOK(w.beat("writer", `{"state":"online"}`))
	w.wantOK(w.beat("lotse", `{"state":"online","details":{"capabilities":{"hosts_agents":"chat"}}}`))
	body := `{"hosted_by":"` + w.lotse.String() + `","reason":"Move"}`
	w.want(w.do("owner", http.MethodPut, "/api/v1/agents/"+writer.String()+"/host", body, key()...), http.StatusPreconditionRequired, "precondition_required")
	m := w.wantOK(w.do("owner", http.MethodPut, "/api/v1/agents/"+writer.String()+"/host", body, ifMatch(w.agentVersion(writer))...))
	if m["hosted_by"] != w.lotse.String() {
		t.Fatalf("moved %v", m)
	}
	w.want(w.beat("writer", `{"state":"online"}`), http.StatusUnauthorized, "unauthenticated")
	w.want(w.do("alex", http.MethodPut, "/api/v1/agents/"+writer.String()+"/host", body, ifMatch(w.agentVersion(writer))...), http.StatusNotFound, "not_found")
}

// TestBuiltInAgentNeverServesABoundForum (BA-T08, R-AG-39, 26 3.1 item 5): setChannelOperator on a
// bound forum and putProjectBinding on a forum operated by a built-in agent answer
// operator_cannot_serve_project.
func TestBuiltInAgentNeverServesABoundForum(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.hostsAgents("scoped")
	writer := ids.MustParse(w.want(w.createBuiltIn(w.runner), http.StatusCreated, "")["id"].(string))
	w.bind(w.projektB, "website")
	r := w.setChannelOperator(w.projektB, agentBody(writer))
	w.want(r, http.StatusBadRequest, "validation_failed")
	if f := fieldCodes(t, r); len(f) != 1 || f[0] != "/agent_id:operator_cannot_serve_project" {
		t.Fatalf("operator on a bound forum: %v", f)
	}
	forum := w.channel(w.engineering, "forum", "Notes")
	w.wantOK(w.setChannelOperator(forum, agentBody(writer)))
	r = w.put("owner", forum, bindBody("notes"))
	w.want(r, http.StatusBadRequest, "validation_failed")
	if f := fieldCodes(t, r); len(f) != 1 || f[0] != "#path/channel_id:operator_cannot_serve_project" {
		t.Fatalf("binding of a forum operated by a built-in agent: %v", f)
	}
}

// wantReasonCode checks a problem with status, code and reason.
func (w *world) wantReasonCode(r *httpservertest.Response, status int, code, reason string) {
	w.t.Helper()
	p := w.want(r, status, code)
	if p["reason"] != reason {
		w.t.Fatalf("reason %v, want %s: %s", p["reason"], reason, r.Body)
	}
}
