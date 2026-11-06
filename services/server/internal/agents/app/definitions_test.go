// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// canary is a text that must never leave the definition: not in an event, an audit entry or a
// log line (26 4.1).
const canary = "canary-7f3a-definition-text"

// defJSON returns a definition with instructions text and the given MCP servers.
func defJSON(t *testing.T, text string, servers ...map[string]any) json.RawMessage {
	t.Helper()
	if servers == nil {
		servers = []map[string]any{}
	}
	raw, err := json.Marshal(map[string]any{
		"instructions": text, "skills": []any{map[string]any{"name": "notes", "description": "Notes", "body": canary}},
		"mcp_servers": servers, "settings": map[string]any{"tool_class_wish": "chat"}, "origin": map[string]any{"kind": "manual"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// stdioServer is a stdio MCP server, which needs the class host.
var stdioServer = map[string]any{"name": "files", "transport": "stdio", "command": "npx", "package": "@example/mcp-files@1.4.2"}

// builtin creates a built-in agent as o.
func (w *world) builtin(o actor.Actor, handle string) agents.Agent {
	w.t.Helper()
	// A built-in agent names its hosting runner (A1-03); the ceiling host never limits the class.
	host := w.runner(o, handle+"-host", "host")
	a, err := w.svc.Create(context.Background(), o, agents.CreateCmd{Handle: handle, DisplayName: handle, AdapterKind: agents.AdapterKindBuiltin, HostedBy: host.ID})
	if err != nil {
		w.t.Fatal(err)
	}
	return a
}

// agentActor returns the agent id as a principal with a valid token.
func (w *world) agentActor(id ids.ID) actor.Actor {
	tok := ids.New()
	w.creds.PutToken(tok, fake.Token{PrincipalID: id, ExpiresAt: start.Add(time.Hour)})
	return actor.Actor{ID: id, Kind: actor.KindAgent, TokenID: tok}
}

// count runs a count query as the database owner.
func (w *world) count(query string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.owner.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return n
}

// denials flushes the aggregated denials and returns the deny reasons of action on resource.
func (w *world) denials(action string, resource ids.ID) []string {
	w.t.Helper()
	if err := w.rec.Aggregator().Flush(context.Background()); err != nil {
		w.t.Fatal(err)
	}
	rows, err := w.owner.Query(context.Background(), `SELECT coalesce(deny_reason, '') FROM audit_events
		WHERE action = $1 AND decision = 'deny' AND coalesce(resource_id::text, '') = $2 ORDER BY seq`, action, idText(resource))
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

// TestPutDefinition_Versions: the first version without If-Match, the next with the right one; a
// missing If-Match on an existing definition is 428, a stale one 412 with the current version;
// each write raises agents.version, appends agent.definition_changed with definition_version only
// to the agent and audits agent.update with the version and the changed parts, never the text; an
// equal body stores nothing (26 4.1, BA-T04 definition part).
func TestPutDefinition_Versions(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	a := w.builtin(owner, "writer")

	v1, err := w.svc.PutDefinition(ctx, owner, a.ID, agents.PutDefinitionCmd{Definition: defJSON(t, "First "+canary)})
	if err != nil || v1.Version != 1 || v1.OriginKind != "manual" || v1.CreatedBy != owner.ID || !strings.Contains(string(v1.Definition), "First") {
		t.Fatalf("first put %+v, %v", v1, err)
	}
	if got := w.text(`SELECT version::text FROM agents WHERE principal_id = $1`, a.ID.String()); got != "2" {
		t.Fatalf("agents.version after the first put %s, want 2", got)
	}
	_, err = w.svc.PutDefinition(ctx, owner, a.ID, agents.PutDefinitionCmd{Definition: defJSON(t, "Second")})
	wantCode(t, err, errs.CodePreconditionRequired)

	v2, err := w.svc.PutDefinition(ctx, owner, a.ID, agents.PutDefinitionCmd{Definition: defJSON(t, "Second"), IfMatch: match(1)})
	if err != nil || v2.Version != 2 {
		t.Fatalf("second put %+v, %v", v2, err)
	}
	_, err = w.svc.PutDefinition(ctx, owner, a.ID, agents.PutDefinitionCmd{Definition: defJSON(t, "Third"), IfMatch: match(1)})
	wantCode(t, err, errs.CodeVersionConflict)
	if e, _ := errs.As(err); e.Current == nil || !strings.Contains(mustMarshal(t, e.Current), `"version":2`) {
		t.Fatalf("412 current %v", mustMarshal(t, e.Current))
	}
	same, err := w.svc.PutDefinition(ctx, owner, a.ID, agents.PutDefinitionCmd{Definition: defJSON(t, "Second"), IfMatch: match(2)})
	if err != nil || same.Version != 2 {
		t.Fatalf("equal put %+v, %v", same, err)
	}
	if n := w.count(`SELECT count(*) FROM agent_definitions WHERE agent_id = $1`, a.ID.String()); n != 2 {
		t.Fatalf("%d versions stored, want 2", n)
	}

	events := w.text(`SELECT coalesce(json_agg(json_build_object('p', payload, 'a', audience_principal_id, 'r', receive_action, 'v', resource_version) ORDER BY seq)::text, '[]')
		FROM event_log WHERE type = 'agent.definition_changed' AND resource_id = $1`, a.ID.String())
	want := `[{"p" : {"definition_version": 1}, "a" : "` + a.ID.String() + `", "r" : "agent.definition.read", "v" : null}, ` +
		`{"p" : {"definition_version": 2}, "a" : "` + a.ID.String() + `", "r" : "agent.definition.read", "v" : null}]`
	if events != want {
		t.Fatalf("events\n got %s\nwant %s", events, want)
	}
	if got := w.updates(a.ID); got != "3" {
		t.Fatalf("agent.updated %s, want 3 (create and two versions)", got)
	}
	details := w.auditDetails("agent.update", a.ID)
	if len(details) != 3 || details[0]["definition_version"] != 1.0 || details[1]["definition_version"] != 2.0 {
		t.Fatalf("audit %v", details)
	}
	if parts, _ := details[0]["changed_parts"].([]any); !slices.Equal(parts, []any{"instructions", "skills", "mcp_servers", "settings", "origin"}) {
		t.Fatalf("first changed parts %v", details[0]["changed_parts"])
	}
	if parts, _ := details[1]["changed_parts"].([]any); !slices.Equal(parts, []any{"instructions"}) {
		t.Fatalf("second changed parts %v", details[1]["changed_parts"])
	}
	if details[2]["no_change"] != true {
		t.Fatalf("equal put audit %v", details[2])
	}
	if n := w.count(`SELECT count(*) FROM event_log WHERE payload::text LIKE '%' || $1 || '%'`, canary); n != 0 {
		t.Fatalf("the definition text is in %d events", n)
	}
	if n := w.count(`SELECT count(*) FROM audit_events WHERE details::text LIKE '%' || $1 || '%'`, canary); n != 0 {
		t.Fatalf("the definition text is in %d audit entries", n)
	}
}

// mustMarshal returns v as JSON text.
func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestPutDefinition_Refusals: a definition breaking the rules answers every finding and stores
// nothing (BA-T01, BA-T02); a stdio server below host is tool_class_too_low; another agent is
// not_a_builtin_agent; If-Match without a stored definition is 412; a person who is no owner gets
// the refusal of 07 2.13 with an audit deny.
func TestPutDefinition_Refusals(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	alex := w.person("alex", false)
	a := w.builtin(owner, "writer")

	bad, _ := json.Marshal(map[string]any{
		"instructions": strings.Repeat("x", 65) + " !`id`", "skills": []any{map[string]any{"name": "Bad", "description": "d", "body": "---\nhooks: {}\n"}},
		"mcp_servers": []any{stdioServer}, "settings": map[string]any{"tool_class_wish": "chat", "hooks": map[string]any{}},
		"origin": map[string]any{"kind": "manual"},
	})
	_, err := w.svc.PutDefinition(ctx, owner, a.ID, agents.PutDefinitionCmd{Definition: bad})
	wantCode(t, err, errs.CodeValidationFailed)
	e, _ := errs.As(err)
	var got []string
	for _, f := range e.Fields {
		got = append(got, f.Pointer+" "+f.Code)
		if f.Detail == "" || strings.Contains(f.Detail, "hooks:") {
			t.Errorf("field %s: detail %q", f.Pointer, f.Detail)
		}
	}
	slices.Sort(got)
	want := []string{
		"/instructions inline_shell", "/instructions too_long", "/mcp_servers/0/transport tool_class_too_low",
		"/settings/hooks unknown_member", "/skills/0/body skill_frontmatter", "/skills/0/name invalid_name",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("fields\n got %q\nwant %q", got, want)
	}
	if n := w.count(`SELECT count(*) FROM agent_definitions`); n != 0 {
		t.Fatalf("%d rows stored after a refusal", n)
	}

	other := w.create(owner, "other")
	_, err = w.svc.PutDefinition(ctx, owner, other.ID, agents.PutDefinitionCmd{Definition: defJSON(t, "x")})
	wantReason(t, err, errs.CodeConflict, "not_a_builtin_agent")

	_, err = w.svc.PutDefinition(ctx, owner, a.ID, agents.PutDefinitionCmd{Definition: defJSON(t, "x"), IfMatch: match(1)})
	wantCode(t, err, errs.CodeVersionConflict)

	// Alex cannot read the agent, so the refusal answers like an unknown agent (R-AZ-01).
	_, err = w.svc.PutDefinition(ctx, alex, a.ID, agents.PutDefinitionCmd{Definition: defJSON(t, "x")})
	wantCode(t, err, errs.CodeNotFound)
	if d := w.denials("agent.update", a.ID); len(d) != 1 || d[0] == "" {
		t.Fatalf("deny entries %v", d)
	}
	_, err = w.svc.PutDefinition(ctx, owner, ids.New(), agents.PutDefinitionCmd{Definition: defJSON(t, "x")})
	wantCode(t, err, errs.CodeNotFound)
	if n := w.count(`SELECT count(*) FROM agent_definitions`); n != 0 {
		t.Fatalf("%d rows stored after the refusals", n)
	}
}

// TestPutDefinition_StaleStepUp: agent.update needs a fresh step-up.
func TestPutDefinition_StaleStepUp(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", true)
	a := w.builtin(owner, "writer")
	stale := owner
	stale.StepUpAt = start.Add(-time.Hour)
	_, err := w.svc.PutDefinition(context.Background(), stale, a.ID, agents.PutDefinitionCmd{Definition: defJSON(t, "x")})
	wantCode(t, err, errs.CodeStepUpRequired)
}

// TestGetDefinition: the owner reads the current and an older version; the agent itself reads
// its current version only; another agent, an agent without definition and an agent that is no
// built-in agent get 404; the version list is newest first and paginated without the text.
func TestGetDefinition(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	alex := w.person("alex", false)
	a := w.builtin(owner, "writer")
	empty := w.builtin(owner, "empty")
	plain := w.create(owner, "plain")
	for i, text := range []string{"one", "two", "three"} {
		cmd := agents.PutDefinitionCmd{Definition: defJSON(t, text)}
		if i > 0 {
			cmd.IfMatch = match(int64(i))
		}
		if _, err := w.svc.PutDefinition(ctx, owner, a.ID, cmd); err != nil {
			t.Fatal(err)
		}
	}
	cur, err := w.svc.GetDefinition(ctx, owner, a.ID, 0)
	if err != nil || cur.Version != 3 || !strings.Contains(string(cur.Definition), `"three"`) {
		t.Fatalf("current %+v, %v", cur, err)
	}
	old, err := w.svc.GetDefinition(ctx, owner, a.ID, 1)
	if err != nil || old.Version != 1 || !strings.Contains(string(old.Definition), `"one"`) {
		t.Fatalf("version 1 %+v, %v", old, err)
	}
	_, err = w.svc.GetDefinition(ctx, owner, a.ID, 9)
	wantCode(t, err, errs.CodeNotFound)

	self := w.agentActor(a.ID)
	mine, err := w.svc.GetDefinition(ctx, self, a.ID, 0)
	if err != nil || mine.Version != 3 {
		t.Fatalf("agent-self %+v, %v", mine, err)
	}
	_, err = w.svc.GetDefinition(ctx, self, a.ID, 1)
	wantCode(t, err, errs.CodeNotFound)
	_, err = w.svc.GetDefinition(ctx, w.agentActor(empty.ID), a.ID, 0)
	wantCode(t, err, errs.CodeNotFound)
	_, err = w.svc.GetDefinition(ctx, alex, a.ID, 0)
	if e, ok := errs.As(err); !ok || (e.PublicCode() != errs.CodeNotFound && e.PublicCode() != errs.CodeForbidden) {
		t.Fatalf("a person who is no owner: %v", err)
	}
	for _, id := range []ids.ID{empty.ID, plain.ID} {
		_, err = w.svc.GetDefinition(ctx, owner, id, 0)
		wantCode(t, err, errs.CodeNotFound)
	}

	page, err := w.svc.ListDefinitionVersions(ctx, owner, a.ID, agents.Page{})
	if err != nil || len(page.Items) != 2 || page.Items[0].Version != 3 || page.Items[1].Version != 2 || page.NextCursor == "" ||
		page.Items[0].Definition != nil {
		t.Fatalf("first page %+v, %v", page, err)
	}
	next, err := w.svc.ListDefinitionVersions(ctx, owner, a.ID, agents.Page{Cursor: page.NextCursor})
	if err != nil || len(next.Items) != 1 || next.Items[0].Version != 1 || next.NextCursor != "" {
		t.Fatalf("second page %+v, %v", next, err)
	}
	_, err = w.svc.ListDefinitionVersions(ctx, alex, a.ID, agents.Page{Cursor: page.NextCursor})
	if err == nil {
		t.Fatal("a person who is no owner lists the versions")
	}
	if none, err := w.svc.ListDefinitionVersions(ctx, owner, empty.ID, agents.Page{}); err != nil || len(none.Items) != 0 {
		t.Fatalf("empty list %+v, %v", none, err)
	}
	_, err = w.svc.ListDefinitionVersions(ctx, owner, plain.ID, agents.Page{})
	wantCode(t, err, errs.CodeNotFound)
}

// TestSetToolClass_DefinitionNeedsHost is BA-T03: a stdio server is stored under host; lowering
// the class below host is 409 definition_needs_host with an audit deny and leaves the class;
// after the server is removed the class goes down.
func TestSetToolClass_DefinitionNeedsHost(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	owner := w.person("owner", true)
	a := w.builtin(owner, "files")
	got, err := w.svc.SetToolClass(ctx, owner, a.ID, agents.ToolClassCmd{ToolClass: agents.ToolHost, Reason: "Files", IfMatch: match(a.Version)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.PutDefinition(ctx, owner, a.ID, agents.PutDefinitionCmd{Definition: defJSON(t, "x", stdioServer)}); err != nil {
		t.Fatal(err)
	}
	version := got.Version + 1
	for _, class := range []string{agents.ToolScoped, agents.ToolChat} {
		_, err = w.svc.SetToolClass(ctx, owner, a.ID, agents.ToolClassCmd{ToolClass: class, Reason: "Lower", IfMatch: match(version)})
		wantReason(t, err, errs.CodeConflict, "definition_needs_host")
	}
	if d := w.denials("agent.tool_class.set", a.ID); len(d) == 0 || d[0] != "definition_needs_host" {
		t.Fatalf("deny entries %v", d)
	}
	if c := w.text(`SELECT tool_class FROM agents WHERE principal_id = $1`, a.ID.String()); c != "host" {
		t.Fatalf("class %s after the refusal", c)
	}
	if _, err := w.svc.PutDefinition(ctx, owner, a.ID, agents.PutDefinitionCmd{Definition: defJSON(t, "x"), IfMatch: match(1)}); err != nil {
		t.Fatal(err)
	}
	lowered, err := w.svc.SetToolClass(ctx, owner, a.ID, agents.ToolClassCmd{ToolClass: agents.ToolScoped, Reason: "Lower", IfMatch: match(version + 1)})
	if err != nil || lowered.ToolClass != agents.ToolScoped {
		t.Fatalf("lower after removing the server %+v, %v", lowered, err)
	}
	// An agent without a definition and one that is no built-in agent lower as before.
	plain := w.create(owner, "plain")
	up, err := w.svc.SetToolClass(ctx, owner, plain.ID, agents.ToolClassCmd{ToolClass: agents.ToolHost, Reason: "Up", IfMatch: match(plain.Version)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.SetToolClass(ctx, owner, plain.ID, agents.ToolClassCmd{ToolClass: agents.ToolChat, Reason: "Down", IfMatch: match(up.Version)}); err != nil {
		t.Fatal(err)
	}
}
