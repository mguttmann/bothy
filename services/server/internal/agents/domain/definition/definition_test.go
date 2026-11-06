// SPDX-License-Identifier: Apache-2.0

package definition_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents/domain/definition"
)

// small are limits small enough to test every boundary by hand.
var small = definition.Limits{
	InstructionsMaxBytes: 10, MaxSkills: 2, SkillDescriptionMaxChars: 5, SkillBodyMaxBytes: 8, MaxSkillFiles: 2,
	MaxMCPServers: 2, MaxAllowedTools: 2, MaxCostUSDPerRun: 3, StdioRunners: []string{"npx", "uvx"},
}

// doc builds a definition with the required members; over replaces or adds members.
func doc(over map[string]any) map[string]any {
	d := map[string]any{
		"instructions": "", "skills": []any{}, "mcp_servers": []any{},
		"settings": map[string]any{"tool_class_wish": "chat"}, "origin": map[string]any{"kind": "manual"},
	}
	for k, v := range over {
		d[k] = v
	}
	return d
}

// skill builds a skill with name n and body b.
func skill(n, b string, more ...any) map[string]any {
	s := map[string]any{"name": n, "description": "d", "body": b}
	for i := 0; i+1 < len(more); i += 2 {
		s[more[i].(string)] = more[i+1]
	}
	return s
}

// file builds a skill file.
func file(n, c string) map[string]any { return map[string]any{"name": n, "content": c} }

// stdio builds a stdio server named n with package p.
func stdio(n, p string, more ...any) map[string]any {
	s := map[string]any{"name": n, "transport": "stdio", "command": "npx", "package": p}
	for i := 0; i+1 < len(more); i += 2 {
		s[more[i].(string)] = more[i+1]
	}
	return s
}

// http builds an http server named n with url u.
func http(n, u string, more ...any) map[string]any {
	s := map[string]any{"name": n, "transport": "http", "url": u}
	for i := 0; i+1 < len(more); i += 2 {
		s[more[i].(string)] = more[i+1]
	}
	return s
}

// findings validates d with lim under class and returns "pointer code" lines, sorted.
func findings(t *testing.T, d any, lim definition.Limits, class definition.ToolClass) []string {
	t.Helper()
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range definition.Validate(raw, lim, class) {
		out = append(out, f.Pointer+" "+f.Code)
	}
	slices.Sort(out)
	return out
}

// check runs the table: each case expects exactly want (nil means valid).
func check(t *testing.T, cases []struct {
	name  string
	d     map[string]any
	class definition.ToolClass
	want  []string
}) {
	t.Helper()
	for _, c := range cases {
		class := c.class
		if class == "" {
			class = definition.Host
		}
		got := findings(t, c.d, small, class)
		want := slices.Clone(c.want)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s: got %q, want %q", c.name, got, want)
		}
	}
}

type tcase = struct {
	name  string
	d     map[string]any
	class definition.ToolClass
	want  []string
}

// TestLimits: every limit passes at its value and fails one above it (design note 2).
func TestLimits(t *testing.T) {
	t.Parallel()
	tools := func(n int) []any {
		var out []any
		for i := range n {
			out = append(out, "T"+string(rune('a'+i)))
		}
		return out
	}
	check(t, []tcase{
		{"instructions at the limit", doc(map[string]any{"instructions": strings.Repeat("x", 10)}), "", nil},
		{"instructions one byte above", doc(map[string]any{"instructions": strings.Repeat("x", 9) + "é"}), "", []string{"/instructions too_long"}},
		{"skills at the limit", doc(map[string]any{"skills": []any{skill("a", ""), skill("b", "")}}), "", nil},
		{"skills above", doc(map[string]any{"skills": []any{skill("a", ""), skill("b", ""), skill("c", "")}}), "", []string{"/skills too_many"}},
		{"description chars at the limit", doc(map[string]any{"skills": []any{skill("a", "", "description", "ééééé")}}), "", nil},
		{"description chars above", doc(map[string]any{"skills": []any{skill("a", "", "description", "éééééé")}}), "", []string{"/skills/0/description too_long"}},
		{"body at the limit", doc(map[string]any{"skills": []any{skill("a", "12345678")}}), "", nil},
		{"body above", doc(map[string]any{"skills": []any{skill("a", "123456789")}}), "", []string{"/skills/0/body too_long"}},
		{"files at the limit", doc(map[string]any{"skills": []any{skill("a", "", "files", []any{file("a.md", "12345678"), file("b.md", "")})}}), "", nil},
		{"files above", doc(map[string]any{"skills": []any{skill("a", "", "files", []any{file("a.md", ""), file("b.md", ""), file("c.md", "")})}}), "", []string{"/skills/0/files too_many"}},
		{"file content above", doc(map[string]any{"skills": []any{skill("a", "", "files", []any{file("a.md", "123456789")})}}), "", []string{"/skills/0/files/0/content too_long"}},
		{"servers at the limit", doc(map[string]any{"mcp_servers": []any{http("a", "https://example.com"), http("b", "https://example.com")}}), "", nil},
		{"servers above", doc(map[string]any{"mcp_servers": []any{http("a", "https://example.com"), http("b", "https://example.com"), http("c", "https://example.com")}}), "", []string{"/mcp_servers too_many"}},
		{"tools at the limit", doc(map[string]any{"settings": map[string]any{"tool_class_wish": "chat", "allowed_tools": tools(2)}}), "", nil},
		{"tools above", doc(map[string]any{"settings": map[string]any{"tool_class_wish": "chat", "allowed_tools": tools(3)}}), "", []string{"/settings/allowed_tools too_many"}},
		{"cost at the limit", doc(map[string]any{"settings": map[string]any{"tool_class_wish": "chat", "max_cost_usd_per_run": 3}}), "", nil},
		{"cost just above", doc(map[string]any{"settings": map[string]any{"tool_class_wish": "chat", "max_cost_usd_per_run": 3.0001}}), "", []string{"/settings/max_cost_usd_per_run too_large"}},
		{"cost small", doc(map[string]any{"settings": map[string]any{"tool_class_wish": "chat", "max_cost_usd_per_run": 0.01}}), "", nil},
		{"cost negative", doc(map[string]any{"settings": map[string]any{"tool_class_wish": "chat", "max_cost_usd_per_run": -1}}), "", []string{"/settings/max_cost_usd_per_run too_small"}},
		{"cost a string", doc(map[string]any{"settings": map[string]any{"tool_class_wish": "chat", "max_cost_usd_per_run": "1"}}), "", []string{"/settings/max_cost_usd_per_run invalid_type"}},
	})
}

// TestFrontmatter: the first line of a body or file content is --- (after a byte order mark,
// trailing blanks allowed); anywhere else, or in instructions, it is text.
func TestFrontmatter(t *testing.T) {
	t.Parallel()
	body := func(b string) map[string]any {
		return doc(map[string]any{"skills": []any{skill("a", b)}})
	}
	big := definition.Limits{InstructionsMaxBytes: 100, MaxSkills: 1, SkillDescriptionMaxChars: 5, SkillBodyMaxBytes: 100, MaxSkillFiles: 1, MaxMCPServers: 1, MaxAllowedTools: 1, MaxCostUSDPerRun: 1}
	for _, c := range []struct {
		b    string
		want bool
	}{
		{"---\nx: 1\n---\n", true}, {"---", true}, {"---  \r\n", true}, {"\ufeff---\n", true}, {"--- \t\n", true},
		{"x\n---\n", false}, {"----\n", false}, {"--- x\n", false}, {" ---\n", false}, {"", false}, {"--\n", false},
	} {
		raw, _ := json.Marshal(body(c.b))
		got := slices.ContainsFunc(definition.Validate(raw, big, definition.Host), func(f definition.FieldError) bool {
			return f.Code == definition.CodeSkillFrontmatter
		})
		if got != c.want {
			t.Errorf("body %q: frontmatter %v, want %v", c.b, got, c.want)
		}
	}
	raw, _ := json.Marshal(doc(map[string]any{"instructions": "---\nx\n---\n"}))
	if fs := definition.Validate(raw, big, definition.Host); len(fs) != 0 {
		t.Errorf("instructions with a leading --- line: %v", fs)
	}
}

// TestInlineShell: both forms of 26 4.3 item 3, at least the two patterns of the pinned CLI,
// also inside other fences and across lines; look-alikes pass.
func TestInlineShell(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		text string
		want bool
	}{
		{"!`id`", true}, {"a !`id`", true}, {"a\n!`id`", true}, {"a\t!`id`", true}, {"a\u00a0!`id`", true},
		{"a\u3000!`id`", true}, {"!`multi\nline`", true}, {"```!\nid\n```", true}, {"x ```! id ```", true},
		{"````md\n```!\nid\n```\n````", true}, {"inside `code` then !`id`", true},
		{"a!`id`", false}, {"!``", false}, {"! `id`", false}, {"!`unclosed", false}, {"```bash\nid\n```", false},
		{"`!`", false}, {"``` !\nid\n```", false},
	} {
		raw, _ := json.Marshal(doc(map[string]any{"instructions": c.text}))
		got := slices.ContainsFunc(definition.Validate(raw, definition.Limits{InstructionsMaxBytes: 1000}, definition.Host), func(f definition.FieldError) bool {
			return f.Code == definition.CodeInlineShell
		})
		if got != c.want {
			t.Errorf("text %q: inline shell %v, want %v", c.text, got, c.want)
		}
	}
}

// TestImports: an @ import at the start of a line or after white space whose path is absolute,
// starts with ~ or has a .. segment (also with \ as separator) is refused, also when an escaped
// space (\ ) continues the path the way the pinned CLI reads it; relative paths, mail addresses and
// handles pass.
func TestImports(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		text string
		want bool
	}{
		{"@/etc/hosts", true}, {"x @/a", true}, {"x\n@~/a", true}, {"@~", true}, {"@..", true}, {"@../a", true},
		{"@a/../b", true}, {"@a/..", true}, {"@a\\..\\b", true}, {"@\\\\server\\share", true}, {"@C:\\x", true},
		{"@c:/x", true}, {"see\t@/x", true},
		{"See @x\\ /../../../../etc/passwd", true}, {"@notes\\ /../../../.ssh/id_ed25519", true},
		{"@a/\\ ..\\ /b", true}, {"@\\ /etc/hosts", true}, {"@\"x\\ /../y", true},
		{"@my\\ notes.md", false}, {"@a.md#/../x", false},
		{"@a/b.md", false}, {"@./a", false}, {"alex@example.com", false}, {"@sam", false}, {"@a..b", false},
		{"x@/etc/hosts", false}, {"@", false}, {"@...", false},
	} {
		raw, _ := json.Marshal(doc(map[string]any{"instructions": c.text}))
		got := slices.ContainsFunc(definition.Validate(raw, definition.Limits{InstructionsMaxBytes: 1000}, definition.Host), func(f definition.FieldError) bool {
			return f.Code == definition.CodeImportOutsideFolder
		})
		if got != c.want {
			t.Errorf("text %q: import %v, want %v", c.text, got, c.want)
		}
	}
}

// TestNames: the name patterns of 26 4.2 and the flat file names other than SKILL.md.
func TestNames(t *testing.T) {
	t.Parallel()
	ok := strings.Repeat("a", 64)
	check(t, []tcase{
		{"skill names", doc(map[string]any{"skills": []any{skill("a", ""), skill(ok, "")}}), "", nil},
		{"skill too long", doc(map[string]any{"skills": []any{skill(ok+"a", "")}}), "", []string{"/skills/0/name invalid_name"}},
		{"skill bad", doc(map[string]any{"skills": []any{skill("a_b", ""), skill("", "")}}), "", []string{"/skills/0/name invalid_name", "/skills/1/name invalid_name"}},
		{"file names", doc(map[string]any{"skills": []any{skill("a", "", "files", []any{file("a-b_c.d.md", ""), file(strings.Repeat("a", 64)+".md", "")})}}), "", nil},
		{"file name too long", doc(map[string]any{"skills": []any{skill("a", "", "files", []any{file(strings.Repeat("a", 65)+".md", "")})}}), "", []string{"/skills/0/files/0/name invalid_name"}},
		{"file SKILL.md in any case", doc(map[string]any{"skills": []any{skill("a", "", "files", []any{file("skill.md", "")})}}), "", []string{"/skills/0/files/0/name invalid_name"}},
		{"file not markdown", doc(map[string]any{"skills": []any{skill("a", "", "files", []any{file("a.txt", "")})}}), "", []string{"/skills/0/files/0/name invalid_name"}},
		{"server names", doc(map[string]any{"mcp_servers": []any{http("a-1", "https://example.com"), http("A", "https://example.com")}}), "", []string{"/mcp_servers/1/name invalid_name"}},
		{"duplicate skills", doc(map[string]any{"skills": []any{skill("a", ""), skill("a", "")}}), "", []string{"/skills/1/name duplicate_name"}},
		{"duplicate files", doc(map[string]any{"skills": []any{skill("a", "", "files", []any{file("a.md", ""), file("a.md", "")})}}), "", []string{"/skills/0/files/1/name duplicate_name"}},
	})
}

// TestSecrets: design note 3; the named headers and the words of 26 4.3 item 5 need a
// secret_ref, case-insensitive; the form of secret_ref is file:<clean absolute path> or
// env:<NAME>.
func TestSecrets(t *testing.T) {
	t.Parallel()
	hdr := func(name string, val ...string) map[string]any {
		h := map[string]any{"name": name}
		for i := 0; i+1 < len(val); i += 2 {
			h[val[i]] = val[i+1]
		}
		return doc(map[string]any{"mcp_servers": []any{http("a", "https://example.com", "headers", []any{h})}})
	}
	env := func(name string, val ...string) map[string]any {
		e := map[string]any{"name": name}
		for i := 0; i+1 < len(val); i += 2 {
			e[val[i]] = val[i+1]
		}
		return doc(map[string]any{"mcp_servers": []any{stdio("a", "x@1.0.0", "env", []any{e})}})
	}
	check(t, []tcase{
		{"authorization", hdr("AUTHORIZATION", "value", "x"), "", []string{"/mcp_servers/0/headers/0/value secret_value_not_allowed"}},
		{"word in a header", hdr("X-Session-Token", "value", "x"), "", []string{"/mcp_servers/0/headers/0/value secret_value_not_allowed"}},
		{"authorization by ref", hdr("Authorization", "secret_ref", "env:A_TOKEN"), "", nil},
		{"plain header", hdr("X-Team", "value", "x"), "", nil},
		{"empty plain value", hdr("X-Team", "value", ""), "", nil},
		{"header with line break", hdr("X-Team", "value", "a\r\nX-Other: b"), "", []string{"/mcp_servers/0/headers/0/value control_character"}},
		{"word in env lower case", env("MY_KEY", "value", "x"), "", []string{"/mcp_servers/0/env/0/value secret_value_not_allowed"}},
		{"env by file ref", env("MY_KEY", "secret_ref", "file:/run/secrets/k"), "", nil},
		{"ref root", env("MY_KEY", "secret_ref", "file:/"), "", []string{"/mcp_servers/0/env/0/secret_ref invalid"}},
		{"ref trailing slash", env("MY_KEY", "secret_ref", "file:/a/"), "", []string{"/mcp_servers/0/env/0/secret_ref invalid"}},
		{"ref empty env", env("MY_KEY", "secret_ref", "env:"), "", []string{"/mcp_servers/0/env/0/secret_ref invalid"}},
		{"ref other scheme", env("MY_KEY", "secret_ref", "azkv:x"), "", []string{"/mcp_servers/0/env/0/secret_ref invalid"}},
		{"reserved prefix", env("LD_LIBRARY_PATH", "value", "x"), "", []string{"/mcp_servers/0/env/0/name env_name_reserved"}},
		{"reserved exact only", env("PATHS", "value", "x"), "", nil},
		{"reserved with a secret word", env("ANTHROPIC_API_KEY", "value", "x"), "", []string{
			"/mcp_servers/0/env/0/name env_name_reserved", "/mcp_servers/0/env/0/value secret_value_not_allowed",
		}},
	})
}

// TestMCP: design note 4: the URL rule, the command list, the exact version, the class rule.
func TestMCP(t *testing.T) {
	t.Parallel()
	one := func(s map[string]any) map[string]any { return doc(map[string]any{"mcp_servers": []any{s}}) }
	check(t, []tcase{
		{"https with port and query", one(http("a", "https://example.com:8443/x?y=1")), "", nil},
		{"no scheme", one(http("a", "example.com")), "", []string{"/mcp_servers/0/url invalid"}},
		{"upper-case scheme", one(http("a", "HTTPS://example.com")), "", nil},
		{"empty fragment", one(http("a", "https://example.com/#")), "", []string{"/mcp_servers/0/url invalid"}},
		{"user only", one(http("a", "https://alex@example.com")), "", []string{"/mcp_servers/0/url invalid"}},
		{"scoped http", one(http("a", "https://example.com")), definition.Scoped, nil},
		{"chat http", one(http("a", "https://example.com")), definition.Chat, nil},
		{"host stdio", one(stdio("a", "@scope/pkg.name_x@10.20.30")), definition.Host, nil},
		{"uvx", one(map[string]any{"name": "a", "transport": "stdio", "command": "uvx", "package": "Pkg_Name@1.0.0"}), "", nil},
		{"pre-release and build", one(stdio("a", "p@1.0.0-rc.1+b.2")), "", nil},
		{"leading zero", one(stdio("a", "p@01.0.0")), "", []string{"/mcp_servers/0/package mcp_package_unpinned"}},
		{"v prefix", one(stdio("a", "p@v1.0.0")), "", []string{"/mcp_servers/0/package mcp_package_unpinned"}},
		{"tilde range", one(stdio("a", "p@~1.0.0")), "", []string{"/mcp_servers/0/package mcp_package_unpinned"}},
		{"wildcard", one(stdio("a", "p@*")), "", []string{"/mcp_servers/0/package mcp_package_unpinned"}},
		{"next tag", one(stdio("a", "p@next")), "", []string{"/mcp_servers/0/package mcp_package_unpinned"}},
		{"git source", one(stdio("a", "git+https://example.com/p@1.0.0")), "", []string{"/mcp_servers/0/package mcp_package_unpinned"}},
		{"empty package", one(stdio("a", "")), "", []string{"/mcp_servers/0/package mcp_package_unpinned"}},
		{"command not in the list", one(map[string]any{"name": "a", "transport": "stdio", "command": "node", "package": "p@1.0.0"}), "", []string{"/mcp_servers/0/command mcp_command_not_allowed"}},
		{"command missing", one(map[string]any{"name": "a", "transport": "stdio", "package": "p@1.0.0"}), "", []string{"/mcp_servers/0/command mcp_command_not_allowed"}},
		{"args after the package may be flags", one(stdio("a", "p@1.0.0", "args", []any{"--port", "0"})), "", nil},
		{"args not strings", one(stdio("a", "p@1.0.0", "args", []any{1})), "", []string{"/mcp_servers/0/args/0 invalid_type"}},
		{"stdio under scoped", one(stdio("a", "p@1.0.0")), definition.Scoped, []string{"/mcp_servers/0/transport tool_class_too_low"}},
		{"stdio under chat", one(stdio("a", "p@1.0.0")), definition.Chat, []string{"/mcp_servers/0/transport tool_class_too_low"}},
	})
}

// TestSettings: model pattern, wish, tools of a listed server, distinct entries.
func TestSettings(t *testing.T) {
	t.Parallel()
	set := func(s map[string]any, servers ...any) map[string]any {
		s["tool_class_wish"] = "scoped"
		if servers == nil {
			servers = []any{}
		}
		return doc(map[string]any{"settings": s, "mcp_servers": servers})
	}
	check(t, []tcase{
		{"model pattern", set(map[string]any{"model": "vendor/model-1.5:fast@2"}), "", nil},
		{"model with space", set(map[string]any{"model": "a b"}), "", []string{"/settings/model invalid"}},
		{"model with selector", set(map[string]any{"model": "opus[1m]"}), "", []string{"/settings/model invalid"}},
		{"model empty", set(map[string]any{"model": ""}), "", []string{"/settings/model invalid"}},
		{"tools of a listed server", set(map[string]any{"allowed_tools": []any{"mcp__a__b-c", "mcp__a"}}, http("a", "https://example.com")), "", nil},
		{"tools of the CLI", set(map[string]any{"allowed_tools": []any{"WebFetch", "Bash(npm test:*)"}}), "", nil},
		{"tool of an unknown server", set(map[string]any{"allowed_tools": []any{"mcp__b__x"}}, http("a", "https://example.com")), "", []string{"/settings/allowed_tools/0 invalid"}},
		{"tool with an empty tool part", set(map[string]any{"allowed_tools": []any{"mcp__a__"}}, http("a", "https://example.com")), "", []string{"/settings/allowed_tools/0 invalid"}},
		{"tool starting with a dash", set(map[string]any{"allowed_tools": []any{"-x"}}), "", []string{"/settings/allowed_tools/0 invalid"}},
		{"tool with a control character", set(map[string]any{"allowed_tools": []any{"Bash\n"}}), "", []string{"/settings/allowed_tools/0 control_character"}},
		{"duplicate tool", set(map[string]any{"allowed_tools": []any{"Read", "Read"}}), "", []string{"/settings/allowed_tools/1 duplicate_name"}},
		{"origin template", doc(map[string]any{"origin": map[string]any{"kind": "template", "template_slug": "a-b", "template_version": "2"}}), "", nil},
		{"origin version with a space", doc(map[string]any{"origin": map[string]any{"kind": "template", "template_version": "1 0"}}), "", []string{"/origin/template_version invalid"}},
		{"null member", doc(map[string]any{"instructions": nil}), "", []string{"/instructions invalid_type"}},
	})
}

// TestParseRoundTrip: Parse returns the typed definition, whose JSON holds exactly the members of
// 26 4.2 and reads back equal; NeedsHost, Parts and Intersect follow 26 4.3 item 7 and 4.4.
func TestParseRoundTrip(t *testing.T) {
	t.Parallel()
	in := doc(map[string]any{
		"instructions": "Hi",
		"skills":       []any{skill("a", "b", "files", []any{file("x.md", "y")})},
		"mcp_servers": []any{
			http("h", "https://example.com", "headers", []any{map[string]any{"name": "Authorization", "secret_ref": "env:T"}}),
			stdio("s", "p@1.0.0", "args", []any{"--x"}, "env", []any{map[string]any{"name": "A", "value": "1"}}),
		},
		"settings": map[string]any{"tool_class_wish": "host", "model": "m", "allowed_tools": []any{}, "max_cost_usd_per_run": 2.5},
		"origin":   map[string]any{"kind": "template", "template_slug": "t", "template_version": "1"},
	})
	raw, _ := json.Marshal(in)
	lim := definition.Limits{InstructionsMaxBytes: 10, MaxSkills: 1, SkillDescriptionMaxChars: 5, SkillBodyMaxBytes: 5, MaxSkillFiles: 1, MaxMCPServers: 2, MaxAllowedTools: 1, MaxCostUSDPerRun: 3, StdioRunners: []string{"npx"}}
	d, fs := definition.Parse(raw, lim, definition.Host)
	if len(fs) != 0 {
		t.Fatalf("findings %v", fs)
	}
	if !d.NeedsHost() || d.Origin.Kind != "template" {
		t.Fatalf("NeedsHost %v, origin %q", d.NeedsHost(), d.Origin.Kind)
	}
	out, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var a, b any
	_ = json.Unmarshal(raw, &a)
	_ = json.Unmarshal(out, &b)
	if ja, jb := mustJSON(a), mustJSON(b); ja != jb {
		t.Fatalf("round trip\n in  %s\n out %s", ja, jb)
	}
	again, fs := definition.Parse(out, lim, definition.Host)
	if len(fs) != 0 || !again.Equal(d) {
		t.Fatalf("second parse %v", fs)
	}
	// Without allowed_tools the member stays absent; NeedsHost is false without stdio.
	plain, fs := definition.Parse([]byte(`{"instructions":"","skills":[],"mcp_servers":[],"settings":{"tool_class_wish":"chat"},"origin":{"kind":"manual"}}`), lim, definition.Chat)
	if len(fs) != 0 || plain.NeedsHost() || plain.Settings.AllowedTools != nil {
		t.Fatalf("plain %+v %v", plain, fs)
	}
	if o, _ := json.Marshal(plain); strings.Contains(string(o), "allowed_tools") || strings.Contains(string(o), "null") {
		t.Fatalf("plain JSON %s", o)
	}
	if got := definition.Parts(plain, d); !slices.Equal(got, []string{"instructions", "skills", "mcp_servers", "settings", "origin"}) {
		t.Fatalf("Parts %v", got)
	}
	if got := definition.AllParts(); !slices.Equal(got, definition.Parts(plain, d)) {
		t.Fatalf("AllParts %v", got)
	}
	if got := definition.Parts(d, d); len(got) != 0 {
		t.Fatalf("Parts of equal definitions %v", got)
	}
}

// mustJSON returns v as canonical JSON (sorted keys through map marshaling).
func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// TestIntersect: 26 4.4 item 2: the runner's list in its order, narrowed by the definition;
// without a list of the definition the runner's whole list.
func TestIntersect(t *testing.T) {
	t.Parallel()
	runner := []string{"Read", "Grep", "WebFetch"}
	if got := definition.Intersect(runner, nil); !slices.Equal(got, runner) {
		t.Errorf("nil list: %v", got)
	}
	if got := definition.Intersect(runner, &[]string{}); len(got) != 0 {
		t.Errorf("empty list: %v", got)
	}
	if got := definition.Intersect(runner, &[]string{"WebFetch", "Bash", "Read"}); !slices.Equal(got, []string{"Read", "WebFetch"}) {
		t.Errorf("narrowed: %v", got)
	}
}

// TestNotJSON: a text that is not JSON is one finding on the root.
func TestNotJSON(t *testing.T) {
	t.Parallel()
	if fs := definition.Validate([]byte(`{`), small, definition.Host); len(fs) != 1 || fs[0].Pointer != "" || fs[0].Code != "invalid_type" {
		t.Fatalf("%v", fs)
	}
}
