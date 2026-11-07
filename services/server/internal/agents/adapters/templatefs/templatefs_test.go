// SPDX-License-Identifier: Apache-2.0

package templatefs_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/adapters/templatefs"
	"github.com/OWNER/bothy/services/server/internal/agents/adapters/templates"
	"github.com/OWNER/bothy/services/server/internal/agents/domain/definition"
)

// repoRoot is the repository root seen from this package.
const repoRoot = "../../../../../.."

// limits returns the limits of the corpus, the defaults of the configuration keys (held equal by
// the wire test TestAgentDefinitionLimitsEqualCorpus).
func limits(t *testing.T) definition.Limits {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot, "tests", "agent-definitions", "limits.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	var lim definition.Limits
	for key, dst := range map[string]any{
		"agents.definition.instructions_max_bytes": &lim.InstructionsMaxBytes, "agents.definition.max_skills": &lim.MaxSkills,
		"agents.definition.skill_description_max_chars": &lim.SkillDescriptionMaxChars,
		"agents.definition.skill_body_max_bytes":        &lim.SkillBodyMaxBytes, "agents.definition.max_skill_files": &lim.MaxSkillFiles,
		"agents.definition.max_mcp_servers": &lim.MaxMCPServers, "agents.definition.max_allowed_tools": &lim.MaxAllowedTools,
		"agents.definition.max_cost_usd_per_run": &lim.MaxCostUSDPerRun, "agents.mcp_stdio_runners": &lim.StdioRunners,
	} {
		if err := json.Unmarshal(m[key], dst); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
	return lim
}

// logged returns a logger that writes JSON lines into buf.
func logged(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// TestShippedTemplatesLoad: the four shipped templates load as drafts that pass 26 4.3 for their
// tool class wish, with origin template, slug and version (26 7.1).
func TestShippedTemplatesLoad(t *testing.T) {
	t.Parallel()
	lim := limits(t)
	items, err := templatefs.Load(templates.FS(), "", lim, logged(&bytes.Buffer{}))
	if err != nil {
		t.Fatal(err)
	}
	var slugs []string
	for _, it := range items {
		slugs = append(slugs, it.Slug)
		if it.Source != agents.TemplateShipped || it.Name == "" || it.Description == "" || it.Version == "" {
			t.Errorf("%s: %+v", it.Slug, it)
		}
		def, err := definition.Decode(it.Definition)
		if err != nil {
			t.Fatal(err)
		}
		if f := definition.Validate(it.Definition, lim, definition.ToolClass(def.Settings.ToolClassWish)); len(f) > 0 {
			t.Errorf("%s: %v", it.Slug, f)
		}
		if def.Origin != (definition.Origin{Kind: definition.OriginTemplate, TemplateSlug: it.Slug, TemplateVersion: it.Version}) {
			t.Errorf("%s: origin %+v", it.Slug, def.Origin)
		}
	}
	if want := []string{"code-reviewer", "meeting-notes", "research-summarizer", "writing-helper"}; !slices.Equal(slugs, want) {
		t.Fatalf("slugs %v", slugs)
	}
	research := items[2]
	if !slices.Equal(research.RequiredSecrets, []string{"SEARCH_MCP_TOKEN"}) {
		t.Fatalf("required secrets %v", research.RequiredSecrets)
	}
	def, _ := definition.Decode(research.Definition)
	if len(def.MCPServers) != 1 || def.MCPServers[0].Headers[0].SecretRef != "env:SEARCH_MCP_TOKEN" || len(def.Skills) != 2 {
		t.Fatalf("draft %+v", def)
	}
	if def.Skills[0].Name != "summarize-sources" || strings.HasPrefix(def.Skills[0].Body, "---") {
		t.Fatalf("skill %+v", def.Skills[0])
	}
}

// TestGateFixturesOfTheSchemaAreRefused: every failing fixture of the template gate that breaks
// the layout, the schema, the frontmatter or a rule of 26 4.3 is refused by the server's loader
// too (tools/templates/tests/fixtures), so a private template the gate would refuse never
// reaches the catalogue. The content rules (hosts, paths, names) are the gate's alone.
func TestGateFixturesOfTheSchemaAreRefused(t *testing.T) {
	t.Parallel()
	lim := limits(t)
	root := filepath.Join(repoRoot, "tools", "templates", "tests", "fixtures")
	rules, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range rules {
		cases, err := os.ReadDir(filepath.Join(root, rule.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range cases {
			_, err := templatefs.Read(os.DirFS(filepath.Join(root, rule.Name(), c.Name())), c.Name(), lim)
			switch rule.Name() {
			case "layout", "schema", "frontmatter", "definition":
				if err == nil {
					t.Errorf("%s/%s: accepted", rule.Name(), c.Name())
				}
			default:
				if err != nil {
					t.Errorf("%s/%s: %v", rule.Name(), c.Name(), err)
				}
			}
		}
	}
}

// copyTemplate copies the shipped template slug into dir/name.
func copyTemplate(t *testing.T, slug, dir, name string) {
	t.Helper()
	err := fs.WalkDir(templates.FS(), slug, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dir, name, strings.TrimPrefix(p, slug))
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		data, err := fs.ReadFile(templates.FS(), p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestPrivateFolder is BA-T18: a private template with a slug a shipped one has is skipped with
// template_slug_taken, an invalid one and a folder whose name is no slug with template_invalid; a
// valid one is listed after the shipped templates as private, and a file at the root is ignored.
func TestPrivateFolder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	copyTemplate(t, "writing-helper", dir, "writing-helper")
	copyTemplate(t, "writing-helper", dir, "release-notes")
	copyTemplate(t, "meeting-notes", dir, "broken")
	copyTemplate(t, "meeting-notes", dir, "Bad_Slug")
	if err := os.WriteFile(filepath.Join(dir, "broken", "hooks.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Private\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	items, err := templatefs.Load(templates.FS(), dir, limits(t), logged(&buf))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, it := range items {
		got = append(got, it.Source+":"+it.Slug)
	}
	want := []string{"shipped:code-reviewer", "shipped:meeting-notes", "shipped:research-summarizer", "shipped:writing-helper", "private:release-notes"}
	if !slices.Equal(got, want) {
		t.Fatalf("templates %v, want %v", got, want)
	}
	var reasons []string
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatal(err)
		}
		if rec["msg"] != "agents: a private template is skipped" || rec["level"] != "WARN" {
			t.Errorf("log %v", rec)
		}
		reasons = append(reasons, rec["bothy.reason"].(string))
	}
	slices.Sort(reasons)
	if want := []string{"template_invalid", "template_invalid", "template_slug_taken"}; !slices.Equal(reasons, want) {
		t.Fatalf("reasons %v, want %v", reasons, want)
	}
}

// TestLoadErrors: a private folder that does not exist names its key, and a shipped template that
// fails the rules stops the start (the template gate holds the shipped ones).
func TestLoadErrors(t *testing.T) {
	t.Parallel()
	lim := limits(t)
	_, err := templatefs.Load(templates.FS(), filepath.Join(t.TempDir(), "none"), lim, logged(&bytes.Buffer{}))
	if err == nil || !strings.Contains(err.Error(), "agents.templates_dir") {
		t.Fatalf("missing folder: %v", err)
	}
	bad := fstest.MapFS{"x/template.yaml": {Data: []byte("name: X\n")}}
	if _, err := templatefs.Load(bad, "", lim, logged(&bytes.Buffer{})); err == nil || !strings.Contains(err.Error(), "x") {
		t.Fatalf("invalid shipped template: %v", err)
	}
}

// TestReadNamesEveryProblemWithoutContent: the error of Read lists each problem with its file and
// rule, never a value of the template.
func TestReadNamesEveryProblemWithoutContent(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"template.yaml":             {Data: []byte("name: Notes\ndescription: Takes notes.\nversion: 1.0.0\ntool_class_wish: chat\nskills: [note]\nsecret: hunter2\n")},
		"CLAUDE.md":                 {Data: []byte("Use !`whoami` here.\n")},
		"skills/note/SKILL.md":      {Data: []byte("---\nname: note\ndescription: Notes.\nhooks: {}\n---\nBody.\n")},
		".mcp.json":                 {Data: []byte("{}\n")},
		"skills/note/extra/file.md": {Data: []byte("x\n")},
	}
	_, err := templatefs.Read(fsys, "notes", limits(t))
	var inv *templatefs.InvalidError
	if !errors.As(err, &inv) {
		t.Fatalf("error %v", err)
	}
	joined := strings.Join(inv.Problems, "\n")
	for _, want := range []string{".mcp.json: file outside the layout", "skills/note/extra/file.md: file outside the layout",
		"template.yaml: unknown key secret", "skills/note/SKILL.md: key hooks other than name and description", "/instructions: inline_shell"} {
		if !strings.Contains(joined, want) {
			t.Errorf("problems %q miss %q", joined, want)
		}
	}
	if strings.Contains(joined, "hunter2") || strings.Contains(joined, "whoami") {
		t.Fatalf("a problem shows content: %q", joined)
	}
}
