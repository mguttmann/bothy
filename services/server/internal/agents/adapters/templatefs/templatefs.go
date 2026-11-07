// SPDX-License-Identifier: Apache-2.0

// Package templatefs reads agent templates from folders (docs/spec/26-builtin-agents.md 7): the
// shipped templates the server embeds (internal/agents/adapters/templates) and the private templates of the
// folder agents.templates_dir, both once at start. A template folder holds exactly
// template.yaml (schema agent_template.v1), CLAUDE.md and skills/<name>/SKILL.md per listed skill
// with a frontmatter of name and description only; the template becomes a definition draft one to
// one (origin.kind template) and must pass the rules of 26 4.3 for its tool class wish.
//
// These are the structural rules of the template gate (tools/templates/template_gate.py: layout,
// schema, frontmatter, definition); the gate's content rules (hosts, paths, IP addresses, names,
// secret-looking values, deny lists) run in the repository only. Problems name a file and a rule,
// never a value of the template.
//
// Example:
//
//	items, err := templatefs.Load(templates.FS(), cfg.TemplatesDir, cfg.DefinitionLimits(), log)
package templatefs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain/definition"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// The files of the layout (26 7.1).
const (
	templateFile     = "template.yaml"
	instructionsFile = "CLAUDE.md"
	skillFile        = "SKILL.md"
	skillsDir        = "skills"
)

// The log reasons of a skipped private template (26 7.2).
const (
	ReasonSlugTaken = "template_slug_taken"
	ReasonInvalid   = "template_invalid"
)

// msgSkipped is the constant log message of a skipped private template; the reason is the only
// attribute (docs/observability/signals.md 7).
const msgSkipped = "agents: a private template is skipped"

// The members of agent_template.v1 and the required ones; model_hint, required_secrets and
// mcp_servers are optional (docs/decisions.d/A1-06.md).
const (
	keyName        = "name"
	keyDescription = "description"
	keyWish        = "tool_class_wish"
	keyServers     = "mcp_servers"
)

var (
	templateKeys = []string{keyName, keyDescription, "version", keyWish, "model_hint", "required_secrets", keyServers, skillsDir}
	requiredKeys = []string{keyName, keyDescription, "version", keyWish, skillsDir}
	envName      = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	versionForm  = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+-]{0,63}$`)
	toolClasses  = []string{string(definition.Chat), string(definition.Scoped), string(definition.Host)}
)

// InvalidError is the refusal of a template: one problem per finding as "<file>: <rule>", never
// a value of the template.
type InvalidError struct {
	// Slug is the template folder name.
	Slug string
	// Problems are the findings in the order of the checks.
	Problems []string
}

// Error implements error.
func (e *InvalidError) Error() string {
	return fmt.Sprintf("template %s: %s", e.Slug, strings.Join(e.Problems, "; "))
}

// reader collects what the checks read of one template folder.
type reader struct {
	slug     string
	problems []string
	entries  []string
	dirs     map[string]bool
	texts    map[string]string
	meta     map[string]any
	skills   []string
	skillsOK bool
	secrets  []string
}

// add records a problem of the file rel.
func (r *reader) add(rel, detail string) {
	if rel == "" {
		r.problems = append(r.problems, detail)
		return
	}
	r.problems = append(r.problems, rel+": "+detail)
}

// Read checks the template folder fsys (rooted at the folder) named slug and returns it as a
// template of source shipped, its definition draft in the stored form.
//
// Errors: *InvalidError with every problem, or an error of fsys.
//
// Example:
//
//	t, err := templatefs.Read(os.DirFS("/srv/templates/notes"), "notes", lim)
func Read(fsys fs.FS, slug string, lim definition.Limits) (agents.Template, error) {
	r := &reader{slug: slug, dirs: map[string]bool{}, texts: map[string]string{}}
	if !definition.ValidSkillName(slug) {
		r.add("", "slug outside ^[a-z0-9][a-z0-9-]{0,63}$")
	}
	if err := r.readFiles(fsys); err != nil {
		return agents.Template{}, err
	}
	r.schema()
	r.layout()
	def := r.definition(lim)
	if len(r.problems) > 0 {
		return agents.Template{}, &InvalidError{Slug: slug, Problems: r.problems}
	}
	stored, err := def.JSON()
	if err != nil {
		return agents.Template{}, err
	}
	// After a template without problems, name, description and version are strings (schema).
	name, _ := r.meta[keyName].(string)
	description, _ := r.meta[keyDescription].(string)
	version, _ := r.meta["version"].(string)
	return agents.Template{
		Slug: slug, Source: agents.TemplateShipped, Name: name, Description: description,
		Version: version, RequiredSecrets: slices.Clip(append([]string{}, r.secrets...)), Definition: stored,
	}, nil
}

// readFiles walks the folder: symbolic links are problems, regular files are read as UTF-8 text.
func (r *reader) readFiles(fsys fs.FS) error {
	return fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." {
			return nil
		}
		r.entries = append(r.entries, p)
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			r.add(p, "symbolic link")
		case d.IsDir():
			r.dirs[p] = true
		case !d.Type().IsRegular():
			r.add(p, "not a regular file")
		default:
			data, err := fs.ReadFile(fsys, p)
			if err != nil {
				return err
			}
			if !utf8.Valid(data) {
				r.add(p, "not UTF-8 text")
				return nil
			}
			r.texts[p] = string(data)
		}
		return nil
	})
}

// oneLine reports whether v is a non-blank string without control or bidi characters.
func oneLine(v any) bool {
	s, ok := v.(string)
	return ok && strings.TrimSpace(s) != "" && textcheck.DisplayText(s) == nil
}

// loadYAML decodes text; a key twice in one mapping is an error of the decoder.
func loadYAML(text string) (any, error) {
	var v any
	err := yaml.Unmarshal([]byte(text), &v)
	return v, err
}

// schema checks template.yaml against agent_template.v1.
func (r *reader) schema() {
	text, ok := r.texts[templateFile]
	if !ok {
		return
	}
	v, err := loadYAML(text)
	if err != nil {
		r.add(templateFile, "not valid YAML or a key appears twice")
		return
	}
	meta, ok := v.(map[string]any)
	if !ok {
		r.add(templateFile, "not a mapping")
		return
	}
	r.meta = meta
	r.keys()
	r.member(keyName, oneLine, "name is not one line of text")
	r.member(keyDescription, oneLine, "description is not one line of text")
	r.member("version", func(v any) bool { s, ok := v.(string); return ok && versionForm.MatchString(s) }, "version is not a version string")
	r.member(keyWish, func(v any) bool { s, ok := v.(string); return ok && slices.Contains(toolClasses, s) },
		"tool_class_wish is not chat, scoped or host")
	r.member("model_hint", func(v any) bool { _, ok := v.(string); return ok }, "model_hint is not a string")
	r.member(keyServers, func(v any) bool { _, ok := v.([]any); return ok }, "mcp_servers is not a list")
	r.secrets, _ = r.checkNames("required_secrets", envName.MatchString)
	r.skills, r.skillsOK = r.checkNames(skillsDir, definition.ValidSkillName)
	r.secretRefs(r.secrets)
}

// keys records the unknown members of template.yaml in sorted order and the missing required ones.
func (r *reader) keys() {
	var unknown []string
	for k := range r.meta {
		if !slices.Contains(templateKeys, k) {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	for _, k := range unknown {
		r.add(templateFile, "unknown key "+k)
	}
	for _, k := range requiredKeys {
		if _, ok := r.meta[k]; !ok {
			r.add(templateFile, "missing "+k)
		}
	}
}

// member records problem when the member key of template.yaml is present and fails valid.
func (r *reader) member(key string, valid func(any) bool, problem string) {
	if v, ok := r.meta[key]; ok && !valid(v) {
		r.add(templateFile, problem)
	}
}

// checkNames checks the list member key as unique names under valid and returns them; ok is false
// when the member is absent although required, or no list of strings.
func (r *reader) checkNames(key string, valid func(string) bool) ([]string, bool) {
	v, present := r.meta[key]
	if !present {
		return []string{}, !slices.Contains(requiredKeys, key)
	}
	items, isList := v.([]any)
	ok := isList
	for _, it := range items {
		if _, isString := it.(string); !isString {
			ok = false
		}
	}
	if !ok {
		r.add(templateFile, key+" is not a list of names")
		return nil, false
	}
	var out []string
	for _, it := range items {
		s, _ := it.(string)
		switch {
		case !valid(s):
			r.add(templateFile, key+" holds an invalid name")
		case slices.Contains(out, s):
			r.add(templateFile, key+" holds a name twice")
		default:
			out = append(out, s)
		}
	}
	return out, true
}

// secretRefs checks that every secret_ref of the MCP servers is env:<NAME> of a declared secret.
func (r *reader) secretRefs(declared []string) {
	servers, _ := r.meta[keyServers].([]any)
	for _, s := range servers {
		server, _ := s.(map[string]any)
		for _, ref := range refs(server) {
			s, isString := ref.(string)
			name, cut := strings.CutPrefix(s, "env:")
			if !isString || !cut || !slices.Contains(declared, name) {
				r.add(templateFile, "secret_ref is not env:<NAME> of a name in required_secrets")
			}
		}
	}
}

// refs returns the secret_ref values of the headers and env entries of one server.
func refs(server map[string]any) []any {
	var out []any
	for _, member := range []string{"headers", "env"} {
		entries, _ := server[member].([]any)
		for _, e := range entries {
			entry, _ := e.(map[string]any)
			if ref, has := entry["secret_ref"]; has {
				out = append(out, ref)
			}
		}
	}
	return out
}

// layout checks that the folder holds exactly the files of 26 7.1.
func (r *reader) layout() {
	allowed := map[string]bool{templateFile: true, instructionsFile: true}
	if r.skillsOK {
		for _, n := range r.skills {
			allowed[path.Join(skillsDir, n, skillFile)] = true
		}
	}
	for _, f := range slices.Sorted(maps.Keys(allowed)) {
		if !slices.Contains(r.entries, f) {
			r.add(f, "missing")
		}
	}
	for _, rel := range r.entries {
		switch {
		case r.dirs[rel]:
			if !r.layoutDir(rel) && r.empty(rel) {
				r.add(rel, "folder outside the layout")
			}
		case allowed[rel], r.problemOf(rel), !r.skillsOK && r.anySkillFile(rel):
		default:
			r.add(rel, "file outside the layout")
		}
	}
}

// layoutDir reports whether the folder rel belongs to the layout: skills/ and skills/<name>/ of a
// listed skill (any name while the skill list is unreadable, which schema reported).
func (r *reader) layoutDir(rel string) bool {
	parts := strings.Split(rel, "/")
	return rel == skillsDir || (len(parts) == 2 && parts[0] == skillsDir && (!r.skillsOK || slices.Contains(r.skills, parts[1])))
}

// anySkillFile reports whether rel is skills/<name>/SKILL.md for any name.
func (r *reader) anySkillFile(rel string) bool {
	parts := strings.Split(rel, "/")
	return len(parts) == 3 && parts[0] == skillsDir && parts[2] == skillFile
}

// empty reports whether no other entry lies below the folder rel.
func (r *reader) empty(rel string) bool {
	return !slices.ContainsFunc(r.entries, func(e string) bool { return strings.HasPrefix(e, rel+"/") })
}

// problemOf reports whether rel already has a problem of its own (a link, a file that is no text).
func (r *reader) problemOf(rel string) bool {
	_, isText := r.texts[rel]
	return !isText && !r.dirs[rel]
}

// skill returns the description and body of skills/<name>/SKILL.md; ok is false after a problem.
func (r *reader) skill(name string) (description, body string, ok bool) {
	rel := path.Join(skillsDir, name, skillFile)
	text, has := r.texts[rel]
	if !has {
		return "", "", false
	}
	lines := strings.Split(text, "\n")
	if lines[0] != "---" {
		r.add(rel, "does not start with a frontmatter line ---")
		return "", "", false
	}
	end := slices.Index(lines[1:], "---")
	if end < 0 {
		r.add(rel, "the frontmatter has no closing line ---")
		return "", "", false
	}
	end++
	v, err := loadYAML(strings.Join(lines[1:end], "\n"))
	if err != nil {
		r.add(rel, "not valid YAML or a key appears twice")
		return "", "", false
	}
	fm, isMap := v.(map[string]any)
	if !isMap {
		r.add(rel, "not a mapping")
		return "", "", false
	}
	ok = true
	var unknown []string
	for k := range fm {
		if k != keyName && k != keyDescription {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	for _, k := range unknown {
		r.add(rel, "key "+k+" other than name and description")
		ok = false
	}
	if fm[keyName] != name {
		r.add(rel, "name differs from the skill folder")
		ok = false
	}
	if !oneLine(fm[keyDescription]) {
		r.add(rel, "description is not one line of text")
		ok = false
	}
	if !ok {
		return "", "", false
	}
	description, _ = fm[keyDescription].(string)
	return description, strings.Join(lines[end+1:], "\n"), true
}

// draftMeta returns the definition draft of the template as JSON data (26 7.1).
func (r *reader) draftMeta() map[string]any {
	var skills []any
	for _, n := range r.skills {
		if d, body, ok := r.skill(n); ok {
			skills = append(skills, map[string]any{keyName: n, keyDescription: d, "body": body})
		}
	}
	if skills == nil {
		skills = []any{}
	}
	settings := map[string]any{keyWish: r.meta[keyWish]}
	if m, ok := r.meta["model_hint"]; ok {
		settings["model"] = m
	}
	origin := map[string]any{"kind": definition.OriginTemplate, "template_slug": r.slug}
	if v, ok := r.meta["version"].(string); ok {
		origin["template_version"] = v
	}
	servers, ok := r.meta[keyServers].([]any)
	if !ok {
		servers = []any{}
	}
	return map[string]any{
		"instructions": r.texts[instructionsFile], "skills": skills, keyServers: servers,
		"settings": settings, "origin": origin,
	}
}

// definition checks the draft with the rules of 26 4.3 for the tool class wish (chat when the
// wish is no class) and returns it.
func (r *reader) definition(lim definition.Limits) definition.Definition {
	if r.meta == nil {
		return definition.Definition{}
	}
	raw, err := json.Marshal(r.draftMeta())
	if err != nil {
		r.add(templateFile, "mcp_servers is not JSON data")
		return definition.Definition{}
	}
	class := definition.Chat
	if w, ok := r.meta[keyWish].(string); ok && slices.Contains(toolClasses, w) {
		class = definition.ToolClass(w)
	}
	def, findings := definition.Parse(raw, lim, class)
	for _, f := range findings {
		p := f.Pointer
		if p == "" {
			p = "/"
		}
		r.add("", "definition "+p+": "+f.Code)
	}
	return def
}

// Load reads the shipped templates of shipped (one folder per template at its root), then the
// private templates of the folder privateDir (agents.templates_dir; empty means none), and returns
// them in this order, each part by slug. A private template whose slug a shipped one has is
// skipped with the log reason template_slug_taken; a private folder that is no valid template, or
// whose name is no slug, with template_invalid; a file at the root of the private folder is
// ignored. The folder is read once; a change needs a restart.
//
// Errors: a shipped template that fails (the template gate holds them), and a privateDir that is
// no readable folder (the error names agents.templates_dir, never the path).
func Load(shipped fs.FS, privateDir string, lim definition.Limits, log *slog.Logger) ([]agents.Template, error) {
	entries, err := fs.ReadDir(shipped, ".")
	if err != nil {
		return nil, err
	}
	var out []agents.Template
	taken := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub, err := fs.Sub(shipped, e.Name())
		if err != nil {
			return nil, err
		}
		t, err := Read(sub, e.Name(), lim)
		if err != nil {
			return nil, fmt.Errorf("agents: shipped %w", err)
		}
		out = append(out, t)
		taken[t.Slug] = true
	}
	if privateDir == "" {
		return out, nil
	}
	private, err := os.ReadDir(privateDir)
	if err != nil {
		return nil, errors.New("agents.templates_dir does not name a readable folder")
	}
	for _, e := range private {
		if e.Type().IsRegular() {
			continue
		}
		t, reason := readPrivate(privateDir, e, taken, lim)
		if reason != "" {
			log.LogAttrs(context.Background(), slog.LevelWarn, msgSkipped, slog.String(attr.Reason, reason))
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

// readPrivate reads the entry e of the private folder dir as a private template; reason is the log
// reason of a skipped entry and empty for a template.
func readPrivate(dir string, e fs.DirEntry, taken map[string]bool, lim definition.Limits) (t agents.Template, reason string) {
	switch {
	case !e.IsDir() || !definition.ValidSkillName(e.Name()):
		return agents.Template{}, ReasonInvalid
	case taken[e.Name()]:
		return agents.Template{}, ReasonSlugTaken
	}
	t, err := Read(os.DirFS(filepath.Join(dir, e.Name())), e.Name(), lim)
	if err != nil {
		return agents.Template{}, ReasonInvalid
	}
	t.Source = agents.TemplatePrivate
	return t, ""
}
