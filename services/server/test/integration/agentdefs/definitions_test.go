// SPDX-License-Identifier: Apache-2.0

package agentdefs_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// canary is definition text that must never leave the definition (26 4.1).
const canary = "cnry-a102-definition-text"

// logBuffer collects the JSON log lines of the server.
type logBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

// Write implements io.Writer.
func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

// String returns the lines so far.
func (l *logBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// world is one server with the owner, a person who is no owner and two built-in agents.
type world struct {
	t                    *testing.T
	s                    *harness.Server
	logs                 *logBuffer
	owner, alex          *harness.Client
	writer, other, plain *harness.Client
}

// newWorld starts a server with raised rate limits and creates the principals.
func newWorld(t *testing.T) *world {
	t.Helper()
	logs := &logBuffer{}
	s := harness.Start(t, harness.Options{EgressDenyAll: true, Config: harness.RaisedRateLimits(), LogWriter: logs})
	// The built-in agents are hosted by a runner with the ceiling host (A1-03), so the class host
	// is allowed and their tokens come from the runner.
	runner := s.NewRunner(t, "runner", "host")
	return &world{
		t: t, s: s, logs: logs, owner: s.Owner,
		alex:   s.NewUser(t, harness.UserSpec{Handle: "alex", DisplayName: "Alex", Email: "alex@example.com"}),
		writer: s.NewAgent(t, harness.AgentSpec{Handle: "writer", DisplayName: "Writer", AdapterKind: "claude_agent", HostedBy: runner}),
		other:  s.NewAgent(t, harness.AgentSpec{Handle: "other", DisplayName: "Other", AdapterKind: "claude_agent", HostedBy: runner}),
		plain:  s.NewAgent(t, harness.AgentSpec{Handle: "plain", DisplayName: "Plain"}),
	}
}

// do sends one request as c with the step-up retry of the harness (a real step-up of the owner).
func (w *world) do(c *harness.Client, method, path string, body any, h http.Header) harness.Response {
	w.t.Helper()
	r, err := harness.DoWithStepUp(context.Background(), c, harness.Request{Method: method, Path: path, Body: body, Header: h})
	if err != nil {
		w.t.Fatalf("%s %s: %v", method, path, err)
	}
	return r
}

// want fails unless r has status and, for a problem, code; it returns the decoded body.
func (w *world) want(r harness.Response, status int, code string) map[string]any {
	w.t.Helper()
	if r.Status != status {
		w.t.Fatalf("status %d, want %d: %s", r.Status, status, r.Body)
	}
	var m map[string]any
	if err := json.Unmarshal(r.Body, &m); err != nil {
		w.t.Fatalf("body: %v", err)
	}
	if testkit.HasJSONNull(r.Body) {
		w.t.Fatalf("JSON null in %s", r.Body)
	}
	if code != "" && m["code"] != code {
		w.t.Fatalf("code %v, want %s: %s", m["code"], code, r.Body)
	}
	return m
}

// ifMatch is the If-Match header of version.
func ifMatch(v int64) http.Header {
	h := http.Header{}
	h.Set("If-Match", `"`+strconv.FormatInt(v, 10)+`"`)
	return h
}

// path is the definition path of the agent c.
func path(c *harness.Client) string { return "/agents/" + c.PrincipalID + "/definition" }

// definition is an AgentDefinition body with the canary in the instructions, a skill body and a
// header value.
func definition(text string, servers ...map[string]any) map[string]any {
	//nolint:gosec // G101: a secret_ref names a variable, no credential
	srv := []any{map[string]any{"name": "docs", "transport": "http", "url": "https://mcp.example.com/v1",
		"headers": []any{map[string]any{"name": "X-Team", "value": "team " + canary}, map[string]any{"name": "Authorization", "secret_ref": "env:DOCS_TOKEN"}}}}
	for _, s := range servers {
		srv = append(srv, s)
	}
	return map[string]any{
		"instructions": text + " " + canary,
		"skills":       []any{map[string]any{"name": "notes", "description": "Writes notes", "body": "Write notes. " + canary}},
		"mcp_servers":  srv, "settings": map[string]any{"tool_class_wish": "chat", "max_cost_usd_per_run": 2.5},
		"origin": map[string]any{"kind": "manual"},
	}
}

// poll reads one page of the REST consumer default of agent c and acknowledges it.
func (w *world) poll(c *harness.Client) []map[string]any {
	w.t.Helper()
	r := c.Do(context.Background(), harness.Request{Method: http.MethodGet, Path: "/agents/me/consumers/default/events?limit=100"})
	page := w.want(r, http.StatusOK, "")
	events, _ := page["events"].([]any)
	var out []map[string]any
	for _, e := range events {
		m, _ := e.(map[string]any)
		out = append(out, m)
	}
	if len(out) > 0 {
		w.want(c.Do(context.Background(), harness.Request{Method: http.MethodPost, Path: "/agents/me/consumers/default/ack",
			Body: map[string]any{"seq": page["next_cursor"]}}), http.StatusOK, "")
	}
	return out
}

// TestDefinition_PutGetAndEvent (BA-T01, BA-T04 definition part): the owner writes a definition
// through the API with a real step-up and reads it; the agent itself reads it with its token and
// gets agent.definition_changed with definition_version only on its consumer; another agent and
// a person who is no owner are refused; no definition text reaches an event, an audit entry or a
// log line.
func TestDefinition_PutGetAndEvent(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.poll(w.writer) // the consumer starts at the head of the log

	put := w.do(w.owner, http.MethodPut, path(w.writer), definition("Release notes."), nil)
	body := w.want(put, http.StatusOK, "")
	if put.Header.Get("ETag") != `"1"` || body["version"] != 1.0 || body["origin_kind"] != "manual" {
		t.Fatalf("put: ETag %q, body %v", put.Header.Get("ETag"), body)
	}
	got := w.want(w.do(w.owner, http.MethodGet, path(w.writer), nil, nil), http.StatusOK, "")
	def, _ := got["definition"].(map[string]any)
	if !strings.Contains(def["instructions"].(string), canary) {
		t.Fatalf("owner read %v", got)
	}
	self := w.want(w.writer.Do(context.Background(), harness.Request{Method: http.MethodGet, Path: path(w.writer)}), http.StatusOK, "")
	if self["version"] != 1.0 {
		t.Fatalf("agent-self read %v", self)
	}
	w.want(w.other.Do(context.Background(), harness.Request{Method: http.MethodGet, Path: path(w.writer)}), http.StatusNotFound, "not_found")
	r := w.alex.Do(context.Background(), harness.Request{Method: http.MethodGet, Path: path(w.writer)})
	if r.Status != http.StatusNotFound && r.Status != http.StatusForbidden {
		t.Fatalf("a person who is no owner reads: %d %s", r.Status, r.Body)
	}
	w.want(w.do(w.alex, http.MethodPut, path(w.writer), definition("x"), nil), http.StatusNotFound, "not_found")

	var changed []map[string]any
	deadline := testkit.Deadline()
	for len(changed) == 0 && time.Now().Before(deadline) {
		for _, e := range w.poll(w.writer) {
			if e["type"] == "agent.definition_changed" {
				changed = append(changed, e)
			}
		}
	}
	if len(changed) != 1 {
		t.Fatalf("agent.definition_changed on the consumer: %v", changed)
	}
	payload, _ := changed[0]["payload"].(map[string]any)
	if len(payload) != 1 || payload["definition_version"] != 1.0 {
		t.Fatalf("payload %v, want definition_version only", payload)
	}
	for _, e := range w.poll(w.other) {
		if e["type"] == "agent.definition_changed" {
			t.Fatalf("another agent got %v", e)
		}
	}

	db := w.s.AdminDB()
	var leaks int
	if err := db.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM event_log WHERE payload::text LIKE '%' || $1 || '%')
		+ (SELECT count(*) FROM audit_events WHERE details::text LIKE '%' || $1 || '%')`, canary).Scan(&leaks); err != nil {
		t.Fatal(err)
	}
	if leaks != 0 {
		t.Fatalf("the definition text is in %d events or audit entries", leaks)
	}
	var details string
	if err := db.QueryRow(context.Background(), `SELECT details::text FROM audit_events WHERE action = 'agent.update' AND decision = 'allow'
		AND resource_id = $1 ORDER BY seq DESC LIMIT 1`, w.writer.PrincipalID).Scan(&details); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(details, `"definition_version": 1`) || !strings.Contains(details, `"changed_parts"`) {
		t.Fatalf("audit details %s", details)
	}
	if strings.Contains(w.logs.String(), canary) {
		t.Fatal("the definition text is in a log line")
	}
}

// TestDefinition_RefusalsAndVersions (BA-T01, BA-T02): unknown members at every level and the
// rules of 26 4.3 answer 400 with one field error each and store nothing; If-Match is required
// after the first version and a stale one is 412 with current; another agent is
// not_a_builtin_agent; the version list pages newest first.
func TestDefinition_RefusalsAndVersions(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	bad := definition("x")
	bad["hooks"] = map[string]any{}
	bad["skills"] = []any{map[string]any{"name": "notes", "description": "d", "body": "b", "allowed-tools": "Bash"}}
	p := w.want(w.do(w.owner, http.MethodPut, path(w.writer), bad, nil), http.StatusBadRequest, "validation_failed")
	if got := fields(p); !slices.Equal(got, []string{"/hooks unknown_member", "/skills/0/allowed-tools unknown_member"}) {
		t.Fatalf("unknown members %v", got)
	}
	rules := definition("Read @/etc/hosts and !`id`")
	rules["skills"] = []any{map[string]any{"name": "notes", "description": "d", "body": "---\nhooks: {}\n"}}
	rules["mcp_servers"] = []any{map[string]any{"name": "files", "transport": "stdio", "command": "npx", "package": "mcp-files@latest",
		"env": []any{map[string]any{"name": "API_TOKEN", "value": "t"}, map[string]any{"name": "PATH", "value": "/tmp"}}}}
	p = w.want(w.do(w.owner, http.MethodPut, path(w.writer), rules, nil), http.StatusBadRequest, "validation_failed")
	want := []string{
		"/instructions import_outside_folder", "/instructions inline_shell", "/mcp_servers/0/env/0/value secret_value_not_allowed",
		"/mcp_servers/0/env/1/name env_name_reserved", "/mcp_servers/0/package mcp_package_unpinned",
		"/mcp_servers/0/transport tool_class_too_low", "/skills/0/body skill_frontmatter",
	}
	if got := fields(p); !slices.Equal(got, want) {
		t.Fatalf("rules\n got %v\nwant %v", got, want)
	}
	if strings.Contains(string(mustJSON(t, p)), "hosts") || strings.Contains(string(mustJSON(t, p)), canary) {
		t.Fatalf("the problem repeats the definition: %v", p)
	}
	var stored int
	if err := w.s.AdminDB().QueryRow(context.Background(), `SELECT count(*) FROM agent_definitions`).Scan(&stored); err != nil || stored != 0 {
		t.Fatalf("%d rows stored after refusals, %v", stored, err)
	}

	w.want(w.do(w.owner, http.MethodPut, path(w.plain), definition("x"), nil), http.StatusConflict, "conflict")
	w.want(w.do(w.owner, http.MethodPut, path(w.writer), definition("one"), nil), http.StatusOK, "")
	w.want(w.do(w.owner, http.MethodPut, path(w.writer), definition("two"), nil), http.StatusPreconditionRequired, "precondition_required")
	w.want(w.do(w.owner, http.MethodPut, path(w.writer), definition("two"), ifMatch(1)), http.StatusOK, "")
	stale := w.want(w.do(w.owner, http.MethodPut, path(w.writer), definition("three"), ifMatch(1)), http.StatusPreconditionFailed, "version_conflict")
	if cur, _ := stale["current"].(map[string]any); cur["version"] != 2.0 {
		t.Fatalf("412 current %v", stale["current"])
	}
	page := w.want(w.do(w.owner, http.MethodGet, path(w.writer)+"/versions?limit=1", nil, nil), http.StatusOK, "")
	items, _ := page["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["version"] != 2.0 || page["next_cursor"] == nil {
		t.Fatalf("first page %v", page)
	}
	if _, has := items[0].(map[string]any)["definition"]; has {
		t.Fatal("the version list carries the definition")
	}
	old := w.want(w.do(w.owner, http.MethodGet, path(w.writer)+"?version=1", nil, nil), http.StatusOK, "")
	if !strings.Contains(old["definition"].(map[string]any)["instructions"].(string), "one") {
		t.Fatalf("version 1 %v", old)
	}
}

// TestDefinition_ToolClassNeedsHost is BA-T03 through the API: a stdio server is stored under
// host, and agent.tool_class.set below host answers 409 definition_needs_host.
func TestDefinition_ToolClassNeedsHost(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	agent := w.want(w.do(w.owner, http.MethodGet, "/agents/"+w.writer.PrincipalID, nil, nil), http.StatusOK, "")
	version := int64(agent["version"].(float64))
	w.want(w.do(w.owner, http.MethodPut, "/agents/"+w.writer.PrincipalID+"/tool-class", map[string]any{"tool_class": "host", "reason": "Files"},
		ifMatch(version)), http.StatusOK, "")
	stdio := map[string]any{"name": "files", "transport": "stdio", "command": "uvx", "package": "mcp-files@1.0.0"}
	w.want(w.do(w.owner, http.MethodPut, path(w.writer), definition("x", stdio), nil), http.StatusOK, "")
	p := w.want(w.do(w.owner, http.MethodPut, "/agents/"+w.writer.PrincipalID+"/tool-class", map[string]any{"tool_class": "scoped", "reason": "Lower"},
		ifMatch(version+2)), http.StatusConflict, "conflict")
	if p["reason"] != "definition_needs_host" {
		t.Fatalf("reason %v", p["reason"])
	}
}

// fields returns the errors of a problem as sorted "pointer code" lines.
func fields(p map[string]any) []string {
	var out []string
	list, _ := p["errors"].([]any)
	for _, e := range list {
		m, _ := e.(map[string]any)
		out = append(out, m["pointer"].(string)+" "+m["code"].(string))
	}
	slices.Sort(out)
	return out
}

// mustJSON encodes v.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
