// SPDX-License-Identifier: Apache-2.0

package definition

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// FieldError is one finding: the JSON Pointer into the definition and the field code. It never
// carries the submitted value.
type FieldError struct {
	Pointer string
	Code    string
}

// The field codes of 26 4.3 (A1-02 requirement 13).
const (
	CodeUnknownMember         = "unknown_member"
	CodeSkillFrontmatter      = "skill_frontmatter"
	CodeInlineShell           = "inline_shell"
	CodeImportOutsideFolder   = "import_outside_folder"
	CodeSecretValueNotAllowed = "secret_value_not_allowed"
	CodeEnvNameReserved       = "env_name_reserved"
	CodeMCPCommandNotAllowed  = "mcp_command_not_allowed"
	CodeMCPPackageUnpinned    = "mcp_package_unpinned"
	CodeToolClassTooLow       = "tool_class_too_low"
	CodeTooLong               = "too_long"
	CodeTooMany               = "too_many"
	CodeInvalidName           = "invalid_name"
	CodeDuplicateName         = "duplicate_name"
)

// The shape codes, the vocabulary of the request validation of platform/httpserver (06 5 item 3),
// so a finding has the same code whether the schema check of the API or this package sees it
// first; the text rules use the codes of kernel/textcheck (invalid_utf8, control_character,
// bidi_control).
const (
	CodeRequired    = "required"
	CodeInvalidType = "invalid_type"
	CodeInvalidEnum = "invalid_enum"
	CodeInvalid     = "invalid"
	CodeTooLarge    = "too_large"
	CodeTooSmall    = "too_small"
)

// Validate checks the JSON text raw as agent_definition.v1 for an agent of tool class class and
// returns every finding; none means the definition passes 26 4.2 and 4.3.
//
// Example:
//
//	for _, f := range definition.Validate(raw, limits, definition.Chat) {
//		fmt.Println(f.Pointer, f.Code) // /skills/0/body skill_frontmatter
//	}
func Validate(raw []byte, lim Limits, class ToolClass) []FieldError {
	_, found := Parse(raw, lim, class)
	return found
}

// Parse checks raw like Validate and returns the typed definition when there is no finding; with
// a finding the Definition is the zero value. A text that is not one JSON value is one
// invalid_type finding on the root.
func Parse(raw []byte, lim Limits, class ToolClass) (Definition, []FieldError) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil || !atEOF(dec) {
		return Definition{}, []FieldError{{Pointer: "", Code: CodeInvalidType}}
	}
	c := &checker{lim: lim, class: class}
	d := c.definition(v)
	if len(c.found) > 0 {
		return Definition{}, c.found
	}
	return d, nil
}

// atEOF reports whether dec holds nothing after the first value.
func atEOF(dec *json.Decoder) bool {
	_, err := dec.Token()
	return errors.Is(err, io.EOF)
}

// checker walks one decoded definition and collects the findings in document order.
type checker struct {
	lim   Limits
	class ToolClass
	found []FieldError
}

// add records a finding.
func (c *checker) add(pointer, code string) {
	c.found = append(c.found, FieldError{Pointer: pointer, Code: code})
}

// pointerEscaper escapes a member name for a JSON Pointer (RFC 6901).
var pointerEscaper = strings.NewReplacer("~", "~0", "/", "~1")

// at returns the pointer of the member name below p.
func at(p, name string) string { return p + "/" + pointerEscaper.Replace(name) }

// index returns the pointer of the item i below p.
func index(p string, i int) string { return p + "/" + strconv.Itoa(i) }

// object returns v as an object and records every member outside allowed as unknown_member, in
// sorted order; a value that is no object is invalid_type and ok is false.
func (c *checker) object(p string, v any, allowed ...string) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		c.add(p, CodeInvalidType)
		return nil, false
	}
	var unknown []string
	for k := range m {
		if !slices.Contains(allowed, k) {
			unknown = append(unknown, k)
		}
	}
	slices.Sort(unknown)
	for _, k := range unknown {
		c.add(at(p, k), CodeUnknownMember)
	}
	return m, true
}

// array returns member name of m as an array; present is false when it is absent (recorded as
// required when required) and ok is false when it is absent or no array.
func (c *checker) array(p string, m map[string]any, name string, required bool) (items []any, ok bool) {
	v, present := m[name]
	if !present {
		if required {
			c.add(at(p, name), CodeRequired)
		}
		return nil, false
	}
	a, isArray := v.([]any)
	if !isArray {
		c.add(at(p, name), CodeInvalidType)
		return nil, false
	}
	return a, true
}

// str returns member name of m as a string; ok is false when it is absent (recorded as required
// when required) or no string (invalid_type).
func (c *checker) str(p string, m map[string]any, name string, required bool) (string, bool) {
	v, present := m[name]
	if !present {
		if required {
			c.add(at(p, name), CodeRequired)
		}
		return "", false
	}
	return c.strValue(at(p, name), v)
}

// strValue returns v as a string or records invalid_type on p.
func (c *checker) strValue(p string, v any) (string, bool) {
	s, ok := v.(string)
	if !ok {
		c.add(p, CodeInvalidType)
	}
	return s, ok
}

// display records the SEC-IN-4 rule a one-line text s breaks (invalid UTF-8, a control or a bidi
// character, line breaks included) and reports whether s passes.
func (c *checker) display(p, s string) bool {
	if err := textcheck.DisplayText(s); err != nil {
		c.add(p, ruleCode(err))
		return false
	}
	return true
}

// markdown is display for Markdown text, where line feed and carriage return are allowed (26 4.2).
func (c *checker) markdown(p, s string) {
	c.display(p, strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, s))
}

// ruleCode returns the textcheck rule of err as the field code.
func ruleCode(err error) string {
	var te *textcheck.Error
	if errors.As(err, &te) {
		return string(te.Rule)
	}
	return CodeInvalid
}

// definition checks the root object (26 4.2) and returns what it could read.
func (c *checker) definition(v any) Definition {
	m, ok := c.object("", v, PartInstructions, PartSkills, PartMCPServers, PartSettings, PartOrigin)
	if !ok {
		return Definition{}
	}
	var d Definition
	if s, ok := c.str("", m, PartInstructions, true); ok {
		p := "/" + PartInstructions
		c.markdown(p, s)
		c.maxBytes(p, s, c.lim.InstructionsMaxBytes)
		c.textRules(p, s, false)
		d.Instructions = s
	}
	d.Skills = c.skills(m)
	d.MCPServers = c.servers(m)
	d.Settings = c.settings(m, d.MCPServers)
	d.Origin = c.origin(m)
	return d
}

// maxBytes records too_long when s has more than limit bytes.
func (c *checker) maxBytes(p, s string, limit int) {
	if len(s) > limit {
		c.add(p, CodeTooLong)
	}
}

// maxItems records too_many on p when n is above limit.
func (c *checker) maxItems(p string, n, limit int) {
	if n > limit {
		c.add(p, CodeTooMany)
	}
}

// textRules applies the text rules of 26 4.3 items 2 to 4 to a Markdown text: frontmatter only
// for a skill body or file content (front), inline shell and imports everywhere.
func (c *checker) textRules(p, s string, front bool) {
	if front && HasFrontmatter(s) {
		c.add(p, CodeSkillFrontmatter)
	}
	if HasInlineShell(s) {
		c.add(p, CodeInlineShell)
	}
	if HasImportOutsideFolder(s) {
		c.add(p, CodeImportOutsideFolder)
	}
}

// names tracks the names of one list for duplicate_name.
type names struct {
	seen map[string]bool
	fold bool
}

// first reports whether n is new in the list and remembers it; with fold the comparison ignores
// ASCII case (header names).
func (ns *names) first(n string) bool {
	if ns.seen == nil {
		ns.seen = map[string]bool{}
	}
	if ns.fold {
		n = strings.ToLower(n)
	}
	if ns.seen[n] {
		return false
	}
	ns.seen[n] = true
	return true
}

// name checks member name of m under pattern ok and the list ns: invalid_name, duplicate_name, a
// text rule, required or invalid_type.
func (c *checker) name(p string, m map[string]any, valid func(string) bool, ns *names) string {
	s, ok := c.str(p, m, memberName, true)
	if !ok {
		return s
	}
	np := at(p, memberName)
	switch {
	case !c.display(np, s):
	case !valid(s):
		c.add(np, CodeInvalidName)
	case !ns.first(s):
		c.add(np, CodeDuplicateName)
	}
	return s
}

// skills checks skills[] (26 4.2 row skills).
func (c *checker) skills(m map[string]any) []Skill {
	items, ok := c.array("", m, PartSkills, true)
	out := []Skill{}
	if !ok {
		return out
	}
	p := "/" + PartSkills
	c.maxItems(p, len(items), c.lim.MaxSkills)
	var ns names
	for i, v := range items {
		ip := index(p, i)
		sm, ok := c.object(ip, v, memberName, "description", "body", "files")
		if !ok {
			continue
		}
		s := Skill{Name: c.name(ip, sm, ValidSkillName, &ns)}
		if d, ok := c.str(ip, sm, "description", true); ok {
			dp := at(ip, "description")
			switch {
			case !c.display(dp, d):
			case strings.TrimSpace(d) == "":
				c.add(dp, CodeRequired)
			case utf8.RuneCountInString(d) > c.lim.SkillDescriptionMaxChars:
				c.add(dp, CodeTooLong)
			}
			s.Description = d
		}
		if b, ok := c.str(ip, sm, "body", true); ok {
			c.skillText(at(ip, "body"), b)
			s.Body = b
		}
		s.Files = c.files(ip, sm)
		out = append(out, s)
	}
	return out
}

// skillText checks a skill body or file content: Markdown, the byte limit and every text rule.
func (c *checker) skillText(p, s string) {
	c.markdown(p, s)
	c.maxBytes(p, s, c.lim.SkillBodyMaxBytes)
	c.textRules(p, s, true)
}

// files checks skills[].files[]; nil when absent.
func (c *checker) files(p string, m map[string]any) []File {
	items, ok := c.array(p, m, "files", false)
	if !ok {
		return nil
	}
	fp := at(p, "files")
	c.maxItems(fp, len(items), c.lim.MaxSkillFiles)
	out := []File{}
	var ns names
	for i, v := range items {
		ip := index(fp, i)
		fm, ok := c.object(ip, v, memberName, "content")
		if !ok {
			continue
		}
		f := File{Name: c.name(ip, fm, ValidFileName, &ns)}
		if s, ok := c.str(ip, fm, "content", true); ok {
			c.skillText(at(ip, "content"), s)
			f.Content = s
		}
		out = append(out, f)
	}
	return out
}

// Member names that several rules read.
const (
	memberName      = "name"
	memberTransport = "transport"
	memberEnv       = "env"
)

// The members of an MCP server per transport; an absent or unknown transport allows all, so only
// the transport itself is reported.
var (
	httpMembers  = []string{memberName, memberTransport, "url", "headers"}
	stdioMembers = []string{memberName, memberTransport, "command", "package", "args", memberEnv}
	allMembers   = []string{memberName, memberTransport, "url", "headers", "command", "package", "args", memberEnv}
)

// servers checks mcp_servers[] (26 4.2 rows mcp_servers).
func (c *checker) servers(m map[string]any) []MCPServer {
	items, ok := c.array("", m, PartMCPServers, true)
	out := []MCPServer{}
	if !ok {
		return out
	}
	p := "/" + PartMCPServers
	c.maxItems(p, len(items), c.lim.MaxMCPServers)
	var ns names
	for i, v := range items {
		ip := index(p, i)
		raw, isObject := v.(map[string]any)
		members := allMembers
		if isObject {
			switch raw[memberTransport] {
			case TransportHTTP:
				members = httpMembers
			case TransportStdio:
				members = stdioMembers
			}
		}
		sm, ok := c.object(ip, v, members...)
		if !ok {
			continue
		}
		s := MCPServer{Name: c.name(ip, sm, ValidServerName, &ns)}
		s.Transport, _ = c.str(ip, sm, memberTransport, true)
		switch s.Transport {
		case TransportHTTP:
			c.httpServer(ip, sm, &s)
		case TransportStdio:
			c.stdioServer(ip, sm, &s)
		default:
			// An absent or non-string transport is already required or invalid_type.
			if _, isString := sm[memberTransport].(string); isString {
				c.add(at(ip, memberTransport), CodeInvalidEnum)
			}
		}
		out = append(out, s)
	}
	return out
}

// httpServer checks url and headers[] of an http server.
func (c *checker) httpServer(p string, m map[string]any, s *MCPServer) {
	if u, ok := c.str(p, m, "url", true); ok {
		up := at(p, "url")
		if c.display(up, u) && !ValidURL(u) {
			c.add(up, CodeInvalid)
		}
		s.URL = u
	}
	s.Headers = c.entries(p, m, "headers", ValidHeaderName, true, nil)
}

// stdioServer checks command, package, args[] and env[] of a stdio server and the class rule.
func (c *checker) stdioServer(p string, m map[string]any, s *MCPServer) {
	s.Command = c.stdioCommand(p, m)
	s.Package = c.stdioPackage(p, m)
	if items, ok := c.array(p, m, "args", false); ok {
		ap := at(p, "args")
		s.Args = []string{}
		for i, v := range items {
			if a, ok := c.strValue(index(ap, i), v); ok {
				c.display(index(ap, i), a)
				s.Args = append(s.Args, a)
			}
		}
	}
	s.Env = c.entries(p, m, memberEnv, ValidEnvName, false, reservedEnv)
	if c.class.Below(Host) {
		c.add(at(p, memberTransport), CodeToolClassTooLow)
	}
}

// stdioCommand checks command: one of agents.mcp_stdio_runners, a missing one included.
func (c *checker) stdioCommand(p string, m map[string]any) string {
	cp := at(p, "command")
	v, present := m["command"]
	if !present {
		c.add(cp, CodeMCPCommandNotAllowed)
		return ""
	}
	cmd, ok := c.strValue(cp, v)
	if ok && c.display(cp, cmd) && !slices.Contains(c.lim.StdioRunners, cmd) {
		c.add(cp, CodeMCPCommandNotAllowed)
	}
	return cmd
}

// stdioPackage checks package: <name>@<exact version>, a missing one included.
func (c *checker) stdioPackage(p string, m map[string]any) string {
	pp := at(p, "package")
	v, present := m["package"]
	if !present {
		c.add(pp, CodeMCPPackageUnpinned)
		return ""
	}
	pkg, ok := c.strValue(pp, v)
	if ok && c.display(pp, pkg) && !PinnedPackage(pkg) {
		c.add(pp, CodeMCPPackageUnpinned)
	}
	return pkg
}

// entries checks headers[] or env[]: each name under valid (header names without case for
// duplicates when fold), reserved names, and exactly one of value and secret_ref, where a name
// that looks like a secret needs secret_ref (26 4.3 item 5).
func (c *checker) entries(p string, m map[string]any, member string, valid func(string) bool, fold bool, reserved func(string) bool) []Entry {
	items, ok := c.array(p, m, member, false)
	if !ok {
		return nil
	}
	lp := at(p, member)
	out := []Entry{}
	ns := names{fold: fold}
	for i, v := range items {
		ip := index(lp, i)
		em, ok := c.object(ip, v, memberName, "value", "secret_ref")
		if !ok {
			continue
		}
		e := Entry{Name: c.name(ip, em, valid, &ns)}
		if reserved != nil && valid(e.Name) && reserved(e.Name) {
			c.add(at(ip, memberName), CodeEnvNameReserved)
		}
		c.entryValue(ip, em, &e)
		out = append(out, e)
	}
	return out
}

// entryValue checks value and secret_ref of one entry.
func (c *checker) entryValue(p string, m map[string]any, e *Entry) {
	_, hasValue := m["value"]
	_, hasRef := m["secret_ref"]
	switch {
	case hasValue && hasRef:
		c.add(p, CodeInvalid)
		return
	case !hasValue && !hasRef:
		c.add(p, CodeRequired)
		return
	}
	if hasValue {
		vp := at(p, "value")
		if v, ok := c.strValue(vp, m["value"]); ok {
			c.display(vp, v)
			if SecretName(e.Name) {
				c.add(vp, CodeSecretValueNotAllowed)
			}
			e.Value, e.HasValue = v, true
		}
		return
	}
	rp := at(p, "secret_ref")
	if r, ok := c.strValue(rp, m["secret_ref"]); ok {
		if c.display(rp, r) && !ValidSecretRef(r) {
			c.add(rp, CodeInvalid)
		}
		e.SecretRef = r
	}
}

// The tool class wishes.
var wishes = []string{string(Chat), string(Scoped), string(Host)}

// settings checks the settings member against the listed servers.
func (c *checker) settings(m map[string]any, servers []MCPServer) Settings {
	v, present := m[PartSettings]
	p := "/" + PartSettings
	if !present {
		c.add(p, CodeRequired)
		return Settings{}
	}
	sm, ok := c.object(p, v, "model", "tool_class_wish", "allowed_tools", "max_cost_usd_per_run")
	if !ok {
		return Settings{}
	}
	var s Settings
	if model, ok := c.str(p, sm, "model", false); ok {
		mp := at(p, "model")
		if c.display(mp, model) && !ValidModel(model) {
			c.add(mp, CodeInvalid)
		}
		s.Model = model
	}
	if w, ok := c.str(p, sm, "tool_class_wish", true); ok {
		if !slices.Contains(wishes, w) {
			c.add(at(p, "tool_class_wish"), CodeInvalidEnum)
		}
		s.ToolClassWish = w
	}
	s.AllowedTools = c.tools(p, sm, servers)
	s.MaxCostUSDPerRun = c.cost(p, sm)
	return s
}

// tools checks settings.allowed_tools[]; nil when absent.
func (c *checker) tools(p string, m map[string]any, servers []MCPServer) *[]string {
	items, ok := c.array(p, m, "allowed_tools", false)
	if !ok {
		return nil
	}
	tp := at(p, "allowed_tools")
	c.maxItems(tp, len(items), c.lim.MaxAllowedTools)
	listed := make([]string, 0, len(servers))
	for _, s := range servers {
		listed = append(listed, s.Name)
	}
	out := []string{}
	var ns names
	for i, v := range items {
		ip := index(tp, i)
		t, ok := c.strValue(ip, v)
		if !ok {
			continue
		}
		switch {
		case !c.display(ip, t):
		case !ValidTool(t, listed):
			c.add(ip, CodeInvalid)
		case !ns.first(t):
			c.add(ip, CodeDuplicateName)
		}
		out = append(out, t)
	}
	return &out
}

// cost checks settings.max_cost_usd_per_run: a number above zero and at most the key.
func (c *checker) cost(p string, m map[string]any) *float64 {
	v, present := m["max_cost_usd_per_run"]
	if !present {
		return nil
	}
	cp := at(p, "max_cost_usd_per_run")
	n, ok := v.(json.Number)
	if !ok {
		c.add(cp, CodeInvalidType)
		return nil
	}
	f, err := n.Float64()
	switch {
	case err != nil:
		c.add(cp, CodeInvalid)
	case f <= 0:
		c.add(cp, CodeTooSmall)
	case f > c.lim.MaxCostUSDPerRun:
		c.add(cp, CodeTooLarge)
	}
	return &f
}

// The origin kinds.
var originKinds = []string{OriginTemplate, OriginInterview, OriginManual}

// origin checks the origin member.
func (c *checker) origin(m map[string]any) Origin {
	v, present := m[PartOrigin]
	p := "/" + PartOrigin
	if !present {
		c.add(p, CodeRequired)
		return Origin{}
	}
	om, ok := c.object(p, v, "kind", "template_slug", "template_version")
	if !ok {
		return Origin{}
	}
	var o Origin
	if k, ok := c.str(p, om, "kind", true); ok {
		if !slices.Contains(originKinds, k) {
			c.add(at(p, "kind"), CodeInvalidEnum)
		}
		o.Kind = k
	}
	if s, ok := c.str(p, om, "template_slug", false); ok {
		sp := at(p, "template_slug")
		if c.display(sp, s) && !ValidSkillName(s) {
			c.add(sp, CodeInvalid)
		}
		o.TemplateSlug = s
	}
	if s, ok := c.str(p, om, "template_version", false); ok {
		vp := at(p, "template_version")
		if c.display(vp, s) && !templateVersionPattern.MatchString(s) {
			c.add(vp, CodeInvalid)
		}
		o.TemplateVersion = s
	}
	return o
}

// marshal is json.Marshal without HTML escaping, so stored text keeps <, > and & as written.
func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// JSON returns the stored form of d: exactly the members of 26 4.2, optional members absent.
//
// Example:
//
//	raw, err := def.JSON() // {"instructions":"...","skills":[],...}
func (d Definition) JSON() ([]byte, error) { return marshal(d) }
