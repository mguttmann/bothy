// SPDX-License-Identifier: Apache-2.0

package interview_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents/domain/definition"
	"github.com/OWNER/bothy/services/server/internal/agents/domain/interview"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// limits are the corpus defaults of 26 14 with a question limit for the tests.
var limits = interview.Limits{
	Definition: definition.Limits{
		InstructionsMaxBytes: 65536, MaxSkills: 32, SkillDescriptionMaxChars: 300, SkillBodyMaxBytes: 65536, MaxSkillFiles: 8,
		MaxMCPServers: 16, MaxAllowedTools: 64, MaxCostUSDPerRun: 50, StdioRunners: []string{"npx", "uvx"},
	},
	QuestionMaxChars: 40,
}

// refEntry is an entry that names its value by secret_ref.
func refEntry(name, ref string) definition.Entry { return definition.Entry{Name: name, SecretRef: ref} }

// refMember is the JSON form of refEntry.
func refMember(name, ref string) map[string]any {
	return map[string]any{"name": name, "secret_ref": ref}
}

// catalogueFiles is a catalogue stdio server with an endpoint argument and two env entries.
func catalogueFiles() definition.MCPServer {
	return definition.MCPServer{
		Name: "files", Transport: definition.TransportStdio, Command: "npx", Package: "@example/mcp-files@1.4.2",
		Args: []string{"--endpoint", "https://files.example.com"},
		Env:  []definition.Entry{{Name: "LOG_LEVEL", Value: "info", HasValue: true}, refEntry("FILES_TOKEN", "env:FILES_TOKEN")},
	}
}

// catalogue holds one template with the stdio server above and an http server.
func catalogue() []interview.CatalogueEntry {
	return []interview.CatalogueEntry{{
		Slug: "writing-helper", Name: "Writing helper", Description: "Writes texts for example.com", ToolClassWish: "host",
		MCPServers: []definition.MCPServer{catalogueFiles(), {
			Name: "docs", Transport: definition.TransportHTTP, URL: "https://mcp.example.com/v1",
			Headers: []definition.Entry{refEntry("Authorization", "env:DOCS_TOKEN")},
		}},
	}}
}

// draft builds a valid draft for the class wish; over replaces or adds members.
func draft(wish string, over map[string]any) map[string]any {
	d := map[string]any{
		"instructions": "You help Alex write texts for example.com.\n", "skills": []any{}, "mcp_servers": []any{},
		"settings": map[string]any{"tool_class_wish": wish}, "origin": map[string]any{"kind": "interview"},
		"name": "Writing helper", "handle": "writing-helper", "description": "Writes short texts",
		"channel_suggestion": map[string]any{"space_name": "Team", "channel_name": "writing"},
	}
	for k, v := range over {
		if v == nil {
			delete(d, k)
			continue
		}
		d[k] = v
	}
	return d
}

// filesServer is the catalogue stdio server as a draft member under another name.
func filesServer(name string) map[string]any {
	return map[string]any{
		"name": name, "transport": "stdio", "command": "npx", "package": "@example/mcp-files@1.4.2",
		"args": []any{"--endpoint", "https://files.example.com"},
		"env":  []any{map[string]any{"name": "LOG_LEVEL", "value": "info"}, refMember("FILES_TOKEN", "env:FILES_TOKEN")},
	}
}

// raw encodes v.
func raw(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// answer encodes a draft answer.
func draftAnswer(t *testing.T, d map[string]any) []byte {
	t.Helper()
	return raw(t, map[string]any{"kind": "draft", "draft": d})
}

// codes returns pointer=code of findings, sorted.
func codes(fs []definition.FieldError) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Pointer+"="+f.Code)
	}
	slices.Sort(out)
	return out
}

// mustInvalid asserts that err is the builder_invalid refusal and returns its findings.
func mustInvalid(t *testing.T, err error) []definition.FieldError {
	t.Helper()
	var inv *interview.InvalidError
	if !errors.As(err, &inv) {
		t.Fatalf("err %v, want *InvalidError", err)
	}
	if len(inv.Findings) == 0 {
		t.Fatal("an invalid answer without findings")
	}
	return inv.Findings
}

func TestParseAnswer_Questions(t *testing.T) {
	t.Parallel()
	amount := int64(4200)
	a, err := interview.ParseAnswer(raw(t, map[string]any{
		"kind": "questions", "questions": []any{"Who reads the texts?", "Which tone?"},
		"costs": []any{map[string]any{"provider": "anthropic", "model": "model-medium", "currency": "USD", "tokens_in": 10, "amount_micros": amount}},
	}), limits, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.Kind != interview.KindQuestions || !slices.Equal(a.Questions, []string{"Who reads the texts?", "Which tone?"}) || a.Draft != nil {
		t.Fatalf("answer %+v", a)
	}
	if len(a.Costs) != 1 || a.Costs[0].AmountMicros == nil || *a.Costs[0].AmountMicros != amount {
		t.Fatalf("costs %+v", a.Costs)
	}
}

func TestParseAnswer_QuestionRules(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]map[string]any{
		"none":          {"kind": "questions", "questions": []any{}},
		"three":         {"kind": "questions", "questions": []any{"a?", "b?", "c?"}},
		"empty":         {"kind": "questions", "questions": []any{" "}},
		"too long":      {"kind": "questions", "questions": []any{strings.Repeat("x", 41)}},
		"bidi":          {"kind": "questions", "questions": []any{"a\u202eb"}},
		"absent":        {"kind": "questions"},
		"with a draft":  {"kind": "questions", "questions": []any{"a?"}, "draft": draft("chat", nil)},
		"unknown kind":  {"kind": "plan"},
		"unknown":       {"kind": "questions", "questions": []any{"a?"}, "hooks": map[string]any{}},
		"cost negative": {"kind": "questions", "questions": []any{"a?"}, "costs": []any{map[string]any{"provider": "p", "model": "m", "currency": "USD", "tokens_in": -1}}},
		"cost unknown":  {"kind": "questions", "questions": []any{"a?"}, "costs": []any{map[string]any{"provider": "p", "model": "m", "currency": "USD", "x": 1}}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := interview.ParseAnswer(raw(t, body), limits, nil)
			mustInvalid(t, err)
		})
	}
}

func TestParseAnswer_ValidDraft(t *testing.T) {
	t.Parallel()
	d := draft("host", map[string]any{"mcp_servers": []any{filesServer("my-files")}})
	a, err := interview.ParseAnswer(draftAnswer(t, d), limits, catalogue())
	if err != nil {
		t.Fatal(err)
	}
	if a.Kind != interview.KindDraft || len(a.Findings) != 0 {
		t.Fatalf("answer %+v", a)
	}
	var back map[string]any
	if err := json.Unmarshal(a.Draft, &back); err != nil {
		t.Fatal(err)
	}
	if back["handle"] != "writing-helper" || back["channel_suggestion"] == nil {
		t.Fatalf("draft %v", back)
	}
}

// TestParseAnswer_DraftFieldErrors: a rule of 26 4.3 that is no surface goes back as a field error
// of the draft for the class of its tool_class_wish (BA-T15 "draft validated").
func TestParseAnswer_DraftFieldErrors(t *testing.T) {
	t.Parallel()
	d := draft("chat", map[string]any{
		"mcp_servers": []any{filesServer("files")},
		"skills": []any{
			map[string]any{"name": "Bad Name", "description": "d", "body": "b"},
		},
		"name": " ",
	})
	a, err := interview.ParseAnswer(draftAnswer(t, d), limits, catalogue())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/mcp_servers/0/transport=tool_class_too_low", "/name=required", "/skills/0/name=invalid_name"}
	if got := codes(a.Findings); !slices.Equal(got, want) {
		t.Fatalf("findings %v, want %v", got, want)
	}
}

// TestParseAnswer_Catalogue: a drafted MCP server matches a catalogue server only when its whole
// entry except name is equal (26 7.4); else mcp_not_in_catalogue at its pointer.
func TestParseAnswer_Catalogue(t *testing.T) {
	t.Parallel()
	endpoint := filesServer("files")
	endpoint["args"] = []any{"--endpoint", "https://files.example.net"}
	envValue := filesServer("files")
	envValue["env"] = []any{map[string]any{"name": "LOG_LEVEL", "value": "debug"}, refMember("FILES_TOKEN", "env:FILES_TOKEN")}
	envRef := filesServer("files")
	envRef["env"] = []any{map[string]any{"name": "LOG_LEVEL", "value": "info"}, refMember("FILES_TOKEN", "env:OTHER_TOKEN")}
	envSwitch := filesServer("files")
	envSwitch["env"] = []any{refMember("LOG_LEVEL", "env:LOG_LEVEL"), refMember("FILES_TOKEN", "env:FILES_TOKEN")}
	argOrder := filesServer("files")
	argOrder["args"] = []any{"https://files.example.com", "--endpoint"}
	argMore := filesServer("files")
	argMore["args"] = []any{"--endpoint", "https://files.example.com", "--verbose"}
	pkg := filesServer("files")
	pkg["package"] = "@example/mcp-files@1.4.3"
	envOrder := filesServer("files")
	envOrder["env"] = []any{refMember("FILES_TOKEN", "env:FILES_TOKEN"), map[string]any{"name": "LOG_LEVEL", "value": "info"}}
	httpOther := map[string]any{"name": "docs", "transport": "http", "url": "https://mcp.example.net/v1",
		"headers": []any{refMember("Authorization", "env:DOCS_TOKEN")}}
	httpHeader := map[string]any{"name": "docs", "transport": "http", "url": "https://mcp.example.com/v1",
		"headers": []any{refMember("Authorization", "env:OTHER")}}
	httpSame := map[string]any{"name": "my-docs", "transport": "http", "url": "https://mcp.example.com/v1",
		"headers": []any{refMember("Authorization", "env:DOCS_TOKEN")}}
	for name, tc := range map[string]struct {
		server map[string]any
		in     bool
	}{
		"endpoint argument": {endpoint, false},
		"env value":         {envValue, false},
		"env secret_ref":    {envRef, false},
		"value to ref":      {envSwitch, false},
		"argument order":    {argOrder, false},
		"one more argument": {argMore, false},
		"package version":   {pkg, false},
		"env order":         {envOrder, true},
		"http url":          {httpOther, false},
		"http header":       {httpHeader, false},
		"http equal":        {httpSame, true},
		"stdio equal":       {filesServer("renamed"), true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d := draft("host", map[string]any{"mcp_servers": []any{tc.server}})
			a, err := interview.ParseAnswer(draftAnswer(t, d), limits, catalogue())
			if err != nil {
				t.Fatal(err)
			}
			got := codes(a.Findings)
			if tc.in && len(got) != 0 {
				t.Fatalf("findings %v, want none", got)
			}
			if !tc.in && !slices.Equal(got, []string{"/mcp_servers/0=mcp_not_in_catalogue"}) {
				t.Fatalf("findings %v, want mcp_not_in_catalogue", got)
			}
		})
	}
}

// TestSameServer_EntriesBySet: env[] and headers[] compare as a set by name, the argument list as
// a list (26 7.4); a missing or extra entry is a difference.
func TestSameServer_EntriesBySet(t *testing.T) {
	t.Parallel()
	head := func(entries ...definition.Entry) definition.MCPServer {
		return definition.MCPServer{Name: "docs", Transport: definition.TransportHTTP, URL: "https://mcp.example.com/v1", Headers: entries}
	}
	auth, trace := refEntry("Authorization", "env:DOCS_TOKEN"), definition.Entry{Name: "X-Trace", Value: "on", HasValue: true}
	files := catalogueFiles()
	swapped := catalogueFiles()
	swapped.Env = []definition.Entry{files.Env[1], files.Env[0]}
	fewer := catalogueFiles()
	fewer.Env = files.Env[:1]
	for name, tc := range map[string]struct {
		a, b definition.MCPServer
		same bool
	}{
		"headers in another order": {head(auth, trace), head(trace, auth), true},
		"one header less":          {head(auth, trace), head(auth), false},
		"env in another order":     {files, swapped, true},
		"one env entry less":       {files, fewer, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := interview.SameServer(tc.a, tc.b); got != tc.same {
				t.Fatalf("SameServer = %v, want %v", got, tc.same)
			}
		})
	}
}

// TestParseAnswer_EmptyCatalogue: without templates every drafted server is outside the catalogue.
func TestParseAnswer_EmptyCatalogue(t *testing.T) {
	t.Parallel()
	d := draft("host", map[string]any{"mcp_servers": []any{filesServer("files")}})
	a, err := interview.ParseAnswer(draftAnswer(t, d), limits, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := codes(a.Findings); !slices.Equal(got, []string{"/mcp_servers/0=mcp_not_in_catalogue"}) {
		t.Fatalf("findings %v", got)
	}
}

// TestParseAnswer_Surfaces: a draft that carries a surface of 26 5.5 is refused as a whole
// (builder_invalid), never answered as a draft (BA-T15).
func TestParseAnswer_Surfaces(t *testing.T) {
	t.Parallel()
	skill := func(body string) []any {
		return []any{map[string]any{"name": "notes", "description": "d", "body": body}}
	}
	cases := map[string]struct {
		over map[string]any
		want string
	}{
		"frontmatter":       {map[string]any{"skills": skill("---\nhooks: {}\n---\nWrite.")}, "/skills/0/body=skill_frontmatter"},
		"frontmatter key":   {map[string]any{"skills": []any{map[string]any{"name": "notes", "description": "d", "body": "b", "allowed-tools": "Bash"}}}, "/skills/0/allowed-tools=unknown_member"},
		"inline shell":      {map[string]any{"instructions": "Run !`id` first."}, "/instructions=inline_shell"},
		"fenced shell":      {map[string]any{"skills": skill("```!\nid\n```")}, "/skills/0/body=inline_shell"},
		"absolute import":   {map[string]any{"instructions": "See @/etc/passwd"}, "/instructions=import_outside_folder"},
		"home import":       {map[string]any{"instructions": "See @~/.ssh/id"}, "/instructions=import_outside_folder"},
		"dotdot import":     {map[string]any{"instructions": "See @../x.md"}, "/instructions=import_outside_folder"},
		"settings file":     {map[string]any{"settings_json": map[string]any{"hooks": map[string]any{}}}, "/settings_json=unknown_member"},
		"hooks":             {map[string]any{"hooks": map[string]any{"PreToolUse": []any{}}}, "/hooks=unknown_member"},
		"permissions":       {map[string]any{"settings": map[string]any{"tool_class_wish": "chat", "permissions": map[string]any{"allow": []any{"Bash"}}}}, "/settings/permissions=unknown_member"},
		"env of the CLI":    {map[string]any{"settings": map[string]any{"tool_class_wish": "chat", "env": map[string]any{}}}, "/settings/env=unknown_member"},
		"folder content":    {map[string]any{"files": []any{map[string]any{"path": ".claude/agents/x.md"}}}, "/files=unknown_member"},
		"plugin folder":     {map[string]any{"plugin": map[string]any{"hooks": map[string]any{}}}, "/plugin=unknown_member"},
		"commands":          {map[string]any{"commands": []any{}}, "/commands=unknown_member"},
		"subagents":         {map[string]any{"agents": []any{}}, "/agents=unknown_member"},
		"headers helper":    {map[string]any{"mcp_servers": []any{map[string]any{"name": "docs", "transport": "http", "url": "https://mcp.example.com/v1", "headersHelper": "x.sh"}}}, "/mcp_servers/0/headersHelper=unknown_member"},
		"channel member":    {map[string]any{"channel_suggestion": map[string]any{"space_name": "Team", "channel_name": "w", "operator": "x"}}, "/channel_suggestion/operator=unknown_member"},
		"control character": {map[string]any{"description": "a\u0007b"}, "/description=control_character"},
		"missing settings":  {map[string]any{"settings": nil}, "/settings=required"},
		"wrong type":        {map[string]any{"skills": "notes"}, "/skills=invalid_type"},
		"unknown wish":      {map[string]any{"settings": map[string]any{"tool_class_wish": "root"}}, "/settings/tool_class_wish=invalid_enum"},
		"missing handle":    {map[string]any{"handle": nil}, "/handle=required"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := interview.ParseAnswer(draftAnswer(t, draft("chat", tc.over)), limits, catalogue())
			// A refusal names the pointer into the answer body, so the draft's members sit below /draft.
			if got := codes(mustInvalid(t, err)); !slices.Contains(got, "/draft"+tc.want) {
				t.Fatalf("findings %v, want %s", got, tc.want)
			}
		})
	}
}

func TestParseAnswer_NotOneObject(t *testing.T) {
	t.Parallel()
	for _, body := range []string{``, `[]`, `{"kind":"questions","questions":["a?"]} {}`, `{"kind":"draft","draft":[]}`, `{"kind":"draft"}`} {
		_, err := interview.ParseAnswer([]byte(body), limits, nil)
		mustInvalid(t, err)
	}
}

func TestCheckTurns(t *testing.T) {
	t.Parallel()
	ok := []interview.Turn{{Role: interview.RoleOwner, Text: "A helper.\nShort."}, {Role: interview.RoleBuilder, Text: "Who reads it?"}}
	if err := interview.CheckTurns(ok, 2, 20); err != nil {
		t.Fatal(err)
	}
	if err := interview.CheckTurns(nil, 2, 20); err != nil {
		t.Fatalf("the start of an interview: %v", err)
	}
	for name, tc := range map[string]struct {
		turns []interview.Turn
		want  string
	}{
		"too many": {[]interview.Turn{ok[0], ok[1], ok[0]}, "/turns=too_many_items"},
		"too long": {[]interview.Turn{{Role: interview.RoleOwner, Text: strings.Repeat("ä", 21)}}, "/turns/0/text=too_long"},
		"empty":    {[]interview.Turn{{Role: interview.RoleOwner, Text: ""}}, "/turns/0/text=too_short"},
		"role":     {[]interview.Turn{{Role: "system", Text: "x"}}, "/turns/0/role=invalid_enum"},
		"bidi":     {[]interview.Turn{{Role: interview.RoleOwner, Text: "a\u202eb"}}, "/turns/0/text=bidi_control"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := interview.CheckTurns(tc.turns, 2, 20)
			e, ok := errs.As(err)
			if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer+"="+e.Fields[0].Code != tc.want {
				t.Fatalf("err %v, want %s", err, tc.want)
			}
		})
	}
}

func TestPayload(t *testing.T) {
	t.Parallel()
	req := ids.New()
	exp := time.Date(2026, 11, 6, 12, 0, 0, 123456789, time.UTC)
	in := interview.PayloadInput{
		RequestID: req, Turns: []interview.Turn{{Role: interview.RoleOwner, Text: "A helper."}}, WantDraft: true,
		Catalogue: catalogue(), Limits: limits, ExpiresAt: exp,
	}
	b, err := interview.Payload(in, 1<<16)
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	if p["request_id"] != req.String() || p["expires_at"] != "2026-11-06T12:00:00.123Z" || p["want_draft"] != true {
		t.Fatalf("payload %s", b)
	}
	lim := p["limits"].(map[string]any)
	if lim["max_skills"] != float64(32) || lim["question_max_chars"] != float64(40) || len(lim["mcp_stdio_runners"].([]any)) != 2 {
		t.Fatalf("limits %v", lim)
	}
	cat := p["catalogue"].([]any)[0].(map[string]any)
	srv := cat["mcp_servers"].([]any)[0].(map[string]any)
	if cat["slug"] != "writing-helper" || srv["package"] != "@example/mcp-files@1.4.2" || testkit.HasJSONNull(b) {
		t.Fatalf("catalogue %s", b)
	}
	empty, err := interview.Payload(interview.PayloadInput{RequestID: req, Limits: limits, ExpiresAt: exp}, 1<<16)
	if err != nil || !strings.Contains(string(empty), `"turns":[]`) || !strings.Contains(string(empty), `"catalogue":[]`) {
		t.Fatalf("empty payload %s %v", empty, err)
	}
	if _, err := interview.Payload(in, len(b)); !errors.Is(err, interview.ErrPayloadTooLarge) {
		t.Fatalf("err %v, want ErrPayloadTooLarge", err)
	}
}

func TestEstimate(t *testing.T) {
	t.Parallel()
	a, b := int64(100), int64(250)
	if m, c, ok := interview.Estimate([]interview.Cost{{Currency: "USD", AmountMicros: &a}, {Currency: "USD", AmountMicros: &b}}); !ok || m != 350 || c != "USD" {
		t.Fatalf("estimate %d %s %v", m, c, ok)
	}
	for _, costs := range [][]interview.Cost{nil, {{Currency: "USD"}}, {{Currency: "USD", AmountMicros: &a}, {Currency: "EUR", AmountMicros: &b}}} {
		if _, _, ok := interview.Estimate(costs); ok {
			t.Fatalf("estimate of %+v", costs)
		}
	}
}

// TestRequest: open -> answered | invalid | expired, each exactly once; forgotten after expires_at
// plus the keep time.
func TestRequest(t *testing.T) {
	t.Parallel()
	exp := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	r := interview.NewRequest(exp)
	if r.State() != interview.StateOpen || !r.Answer(interview.Answer{Kind: interview.KindQuestions}) || r.State() != interview.StateAnswered {
		t.Fatal("open -> answered")
	}
	if r.Answer(interview.Answer{}) || r.Refuse() || r.Expire() {
		t.Fatal("a second transition")
	}
	r2 := interview.NewRequest(exp)
	if !r2.Refuse() || r2.State() != interview.StateInvalid || r2.Expire() {
		t.Fatal("open -> invalid")
	}
	r3 := interview.NewRequest(exp)
	if !r3.Expire() || r3.State() != interview.StateExpired || r3.Answer(interview.Answer{}) {
		t.Fatal("open -> expired")
	}
	if r3.Forgotten(exp.Add(time.Minute-time.Nanosecond), time.Minute) || !r3.Forgotten(exp.Add(time.Minute), time.Minute) {
		t.Fatal("forgotten")
	}
}
