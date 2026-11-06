// SPDX-License-Identifier: Apache-2.0

package definition

import (
	"reflect"
	"slices"
)

// ToolClass is the tool class of the agent a definition is checked for (07 2.11): chat, scoped
// or host, in this order.
type ToolClass string

// The tool classes of 07 2.11.
const (
	Chat   ToolClass = "chat"
	Scoped ToolClass = "scoped"
	Host   ToolClass = "host"
)

// rank orders the classes; an unknown class ranks below chat, so it never passes a class rule.
func (c ToolClass) rank() int {
	switch c {
	case Chat:
		return 1
	case Scoped:
		return 2
	case Host:
		return 3
	}
	return 0
}

// Below reports whether c is lower than o in the order chat < scoped < host.
//
// Example:
//
//	definition.Scoped.Below(definition.Host) // true
func (c ToolClass) Below(o ToolClass) bool { return c.rank() < o.rank() }

// Limits are the configured bounds of a definition (26 14). The config tags name the keys the
// server binds them from; no bound is a number in code.
type Limits struct {
	// InstructionsMaxBytes bounds instructions in bytes.
	InstructionsMaxBytes int `config:"agents.definition.instructions_max_bytes"`
	// MaxSkills bounds the number of skills.
	MaxSkills int `config:"agents.definition.max_skills"`
	// SkillDescriptionMaxChars bounds a skill description in Unicode characters.
	SkillDescriptionMaxChars int `config:"agents.definition.skill_description_max_chars"`
	// SkillBodyMaxBytes bounds a skill body and every extra skill file in bytes.
	SkillBodyMaxBytes int `config:"agents.definition.skill_body_max_bytes"`
	// MaxSkillFiles bounds the extra files of one skill.
	MaxSkillFiles int `config:"agents.definition.max_skill_files"`
	// MaxMCPServers bounds the number of MCP servers.
	MaxMCPServers int `config:"agents.definition.max_mcp_servers"`
	// MaxAllowedTools bounds settings.allowed_tools.
	MaxAllowedTools int `config:"agents.definition.max_allowed_tools"`
	// MaxCostUSDPerRun is the highest settings.max_cost_usd_per_run.
	MaxCostUSDPerRun float64 `config:"agents.definition.max_cost_usd_per_run"`
	// StdioRunners are the commands a stdio MCP server may use.
	StdioRunners []string `config:"agents.mcp_stdio_runners"`
}

// Definition is a checked agent_definition.v1 (26 4.2). Its JSON form holds exactly the members of
// the schema, optional members absent rather than null (ADR 0053), so storing the marshaled value
// stores nothing outside the schema.
type Definition struct {
	// Instructions is the Markdown that becomes CLAUDE.md in the agent folder.
	Instructions string `json:"instructions"`
	// Skills are the skills; never nil after Parse.
	Skills []Skill `json:"skills"`
	// MCPServers are the MCP servers; never nil after Parse.
	MCPServers []MCPServer `json:"mcp_servers"`
	// Settings are model, class wish, tool list and cost cap.
	Settings Settings `json:"settings"`
	// Origin says where the definition came from (display only).
	Origin Origin `json:"origin"`
}

// Skill is one entry of skills[].
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
	// Files are the extra flat files of the skill; absent when nil.
	Files []File `json:"files,omitempty"`
}

// File is one entry of skills[].files[].
type File struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// The transports of an MCP server.
const (
	TransportHTTP  = "http"
	TransportStdio = "stdio"
)

// MCPServer is one entry of mcp_servers[]: URL and headers for http; command, package, args and
// env for stdio.
type MCPServer struct {
	Name      string `json:"name"`
	Transport string `json:"transport"`
	// URL and Headers belong to http.
	URL     string  `json:"url,omitempty"`
	Headers []Entry `json:"headers,omitempty"`
	// Command, Package, Args and Env belong to stdio.
	Command string   `json:"command,omitempty"`
	Package string   `json:"package,omitempty"`
	Args    []string `json:"args,omitempty"`
	Env     []Entry  `json:"env,omitempty"`
}

// Entry is a header or an environment entry: a name with either a plain value or a secret_ref
// that the hosting runner resolves (the server never does).
type Entry struct {
	Name string `json:"name"`
	// Value is set when HasValue; an empty plain value is allowed.
	Value    string `json:"-"`
	HasValue bool   `json:"-"`
	// SecretRef names a key of the runner's secret provider; empty when HasValue.
	SecretRef string `json:"-"`
}

// entryJSON is the JSON form of Entry with exactly one of value and secret_ref.
type entryJSON struct {
	Name      string  `json:"name"`
	Value     *string `json:"value,omitempty"`
	SecretRef string  `json:"secret_ref,omitempty"`
}

// MarshalJSON writes value or secret_ref, never both and never null.
func (e Entry) MarshalJSON() ([]byte, error) {
	out := entryJSON{Name: e.Name, SecretRef: e.SecretRef}
	if e.HasValue {
		v := e.Value
		out.Value, out.SecretRef = &v, ""
	}
	return marshal(out)
}

// Settings are the settings member of the definition.
type Settings struct {
	// Model is empty for the runner's default model.
	Model string `json:"model,omitempty"`
	// ToolClassWish preselects the class of the preview; it never sets the class.
	ToolClassWish string `json:"tool_class_wish"`
	// AllowedTools is nil when absent (the runner's whole list) and points to an empty list for
	// "no named tool".
	AllowedTools *[]string `json:"allowed_tools,omitempty"`
	// MaxCostUSDPerRun is nil when the definition sets no cap.
	MaxCostUSDPerRun *float64 `json:"max_cost_usd_per_run,omitempty"`
}

// The origin kinds of 26 4.2.
const (
	OriginTemplate  = "template"
	OriginInterview = "interview"
	OriginManual    = "manual"
)

// Origin is the origin member (display only).
type Origin struct {
	Kind            string `json:"kind"`
	TemplateSlug    string `json:"template_slug,omitempty"`
	TemplateVersion string `json:"template_version,omitempty"`
}

// NeedsHost reports whether d needs the tool class host: a stdio MCP server starts a local process
// with the rights of the host user (26 4.4 item 4, R-BA-05).
func (d Definition) NeedsHost() bool {
	return slices.ContainsFunc(d.MCPServers, func(s MCPServer) bool { return s.Transport == TransportStdio })
}

// Equal reports whether d and o are the same definition.
func (d Definition) Equal(o Definition) bool { return reflect.DeepEqual(d, o) }

// The top-level parts of a definition as the audit of agent.update names them (26 4.1 item 2).
const (
	PartInstructions = "instructions"
	PartSkills       = "skills"
	PartMCPServers   = "mcp_servers"
	PartSettings     = "settings"
	PartOrigin       = "origin"
)

// AllParts returns the names of every top-level part in schema order: the changed parts of a
// first version, which has no version to compare with.
func AllParts() []string {
	return []string{PartInstructions, PartSkills, PartMCPServers, PartSettings, PartOrigin}
}

// Parts returns the names of the top-level parts that differ between old and new, in schema
// order; never their content. Absent and empty lists compare equal.
//
// Example:
//
//	definition.Parts(old, new) // ["instructions", "settings"]
func Parts(old, new Definition) []string {
	var out []string
	add := func(name string, same bool) {
		if !same {
			out = append(out, name)
		}
	}
	add(PartInstructions, old.Instructions == new.Instructions)
	add(PartSkills, reflect.DeepEqual(nonNil(old.Skills), nonNil(new.Skills)))
	add(PartMCPServers, reflect.DeepEqual(nonNil(old.MCPServers), nonNil(new.MCPServers)))
	add(PartSettings, reflect.DeepEqual(old.Settings, new.Settings))
	add(PartOrigin, old.Origin == new.Origin)
	return out
}

// nonNil returns s, or an empty slice for nil, so absent and empty compare equal.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// Intersect returns the tools of a scoped turn (26 4.4 item 2, 09 18.1): the entries of the
// runner's runner.scoped_allowed_tools in their order that the definition's list also names; the
// whole runner list when the definition has none (nil). The definition only narrows, the host
// operator keeps the ceiling.
//
// Example:
//
//	definition.Intersect([]string{"Read", "Grep"}, &[]string{"Grep", "Bash"}) // ["Grep"]
func Intersect(runner []string, allowed *[]string) []string {
	if allowed == nil {
		return slices.Clone(runner)
	}
	out := []string{}
	for _, t := range runner {
		if slices.Contains(*allowed, t) {
			out = append(out, t)
		}
	}
	return out
}
