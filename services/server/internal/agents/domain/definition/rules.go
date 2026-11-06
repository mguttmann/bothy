// SPDX-License-Identifier: Apache-2.0

package definition

import (
	"encoding/json"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
)

// The patterns of 26 4.2 and of docs/decisions.d/A1-02.md. Go's regexp runs in linear time
// (SEC-IN-6), so no input slows a check down beyond its length.
var (
	// skillNamePattern is the name of a skill and of an MCP server, and a template slug (7.1).
	skillNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	// fileNamePattern is a flat Markdown file name of a skill.
	fileNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}\.md$`)
	// envNamePattern is the name of an environment entry.
	envNamePattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	// headerNamePattern is the name of an HTTP header: letters, digits, dash and underscore.
	headerNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)
	// modelPattern is the pattern of default_model (09 2.2).
	modelPattern = regexp.MustCompile(`^[A-Za-z0-9._:/@-]+$`)
	// builtinToolPattern is a tool name of the CLI, optionally with a rule in parentheses as
	// runner.scoped_allowed_tools may name it (for example Bash(git log:*)).
	builtinToolPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(\([^()]*\))?$`)
	// mcpToolPattern is the tool part of mcp__<server>__<tool>.
	mcpToolPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	// packagePattern is <name>@<exact semantic version>: an npm name with an optional scope or a
	// Python package name, then a SemVer 2.0.0 version without range operator, wildcard or tag.
	packagePattern = regexp.MustCompile(`^(@[a-z0-9][a-z0-9._-]*/)?[A-Za-z0-9][A-Za-z0-9._-]*@` +
		`(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)` +
		`(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)
	// templateVersionPattern is origin.template_version.
	templateVersionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+-]{0,63}$`)
)

// jsSpace is the white space of a JavaScript regular expression (\s), which the pinned CLI uses
// in its inline shell pattern; Go's \s knows only ASCII.
const jsSpace = `\t\n\v\f\r \x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}`

var (
	// inlineShellPattern is the pattern (?<=^|\s)!`[^`]+` of the pinned CLI (26 4.3 item 3) with
	// the look-behind written as a group: the start of the text or white space, then !, a
	// backtick, at least one character other than a backtick (line breaks included), a backtick.
	inlineShellPattern = regexp.MustCompile("(?:^|[" + jsSpace + "])!`[^`]+`")
	// importPattern finds a token @<path> at the start of the text or after white space (a line
	// break included, so also at the start of every line).
	importPattern = regexp.MustCompile("(?:^|[" + jsSpace + "])@([^" + jsSpace + "]+)")
	// drivePattern is a Windows drive at the start of a path.
	drivePattern = regexp.MustCompile(`^[A-Za-z]:`)
)

// fenceShell opens a fenced block the CLI runs as shell; any occurrence counts, also inside
// another fence, because the CLI's pattern /```!\s*\n?[\s\S]*?\n?```/ does not look at fences.
const fenceShell = "```!"

// HasFrontmatter reports whether the first line of s, after an optional byte order mark and
// without trailing blanks, is --- (26 4.3 item 2).
func HasFrontmatter(s string) bool {
	s = strings.TrimPrefix(s, "\ufeff")
	line, _, _ := strings.Cut(s, "\n")
	return strings.TrimRight(line, " \t\r") == "---"
}

// HasInlineShell reports whether s holds one of the two inline shell forms of 26 4.3 item 3.
func HasInlineShell(s string) bool {
	return strings.Contains(s, fenceShell) || inlineShellPattern.MatchString(s)
}

// HasImportOutsideFolder reports whether s holds an @ import whose path leaves the agent folder
// (26 4.3 item 4): absolute (/, \ or a drive), starting with ~, or with a .. segment, with / or
// \ as separator; a leading quote is ignored.
func HasImportOutsideFolder(s string) bool {
	for _, m := range importPattern.FindAllStringSubmatch(s, -1) {
		if outsideFolder(strings.TrimLeft(m[1], `"'`)) {
			return true
		}
	}
	return false
}

// outsideFolder reports whether the import path p leaves the agent folder.
func outsideFolder(p string) bool {
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, `\`) || strings.HasPrefix(p, "~") || drivePattern.MatchString(p) {
		return true
	}
	return slices.Contains(strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' }), "..")
}

// ValidSkillName reports whether n is a skill name, an MCP server name or a template slug.
func ValidSkillName(n string) bool { return skillNamePattern.MatchString(n) }

// ValidServerName reports whether n is an MCP server name.
func ValidServerName(n string) bool { return skillNamePattern.MatchString(n) }

// ValidFileName reports whether n is a flat Markdown file name of a skill other than SKILL.md in
// any case (a case-insensitive file system would let skill.md replace the runner's SKILL.md).
func ValidFileName(n string) bool {
	return fileNamePattern.MatchString(n) && !strings.EqualFold(n, "SKILL.md")
}

// ValidEnvName reports whether n is the name of an environment entry.
func ValidEnvName(n string) bool { return envNamePattern.MatchString(n) }

// ValidHeaderName reports whether n is the name of a header.
func ValidHeaderName(n string) bool { return headerNamePattern.MatchString(n) }

// The environment names the runner sets itself (26 4.3 item 5): exact names and prefixes.
var (
	reservedEnvNames    = []string{"PATH", "HOME", "USER", "LOGNAME", "SHELL", "TMPDIR", "CLAUDE_CONFIG_DIR", "CLAUDE_PRODUCT_RUN", "NODE_OPTIONS"}
	reservedEnvPrefixes = []string{"ANTHROPIC_", "CLAUDE_CODE_", "LD_", "DYLD_"}
)

// reservedEnv reports whether n is an environment name the runner sets itself.
func reservedEnv(n string) bool {
	return slices.Contains(reservedEnvNames, n) ||
		slices.ContainsFunc(reservedEnvPrefixes, func(p string) bool { return strings.HasPrefix(n, p) })
}

// secretWords are the words of 26 4.3 item 5; the header names Authorization, Cookie and
// Proxy-Authorization contain AUTH or COOKIE.
var secretWords = []string{"KEY", "TOKEN", "SECRET", "PASSWORD", "PASSWD", "CREDENTIAL", "AUTH", "COOKIE"}

// SecretName reports whether a header or environment entry named n must carry secret_ref
// instead of a plain value: its upper-cased name contains one of the words of 26 4.3 item 5.
func SecretName(n string) bool {
	u := strings.ToUpper(n)
	return slices.ContainsFunc(secretWords, func(w string) bool { return strings.Contains(u, w) })
}

// ValidSecretRef reports whether r has the form the runner's secret provider accepts (the form of
// runner.anthropic_api_key_ref): env:<NAME> or file:<clean absolute path>. The server checks the
// form only and never resolves it.
func ValidSecretRef(r string) bool {
	scheme, rest, ok := strings.Cut(r, ":")
	switch {
	case !ok:
		return false
	case scheme == memberEnv:
		return envNamePattern.MatchString(rest)
	case scheme == "file":
		return rest != "/" && path.IsAbs(rest) && path.Clean(rest) == rest
	}
	return false
}

// ValidURL reports whether u is an https URL with a host, without user information and without a
// fragment (26 4.2).
func ValidURL(u string) bool {
	p, err := url.Parse(u)
	return err == nil && p.Scheme == "https" && p.Opaque == "" && p.Hostname() != "" && p.User == nil &&
		!strings.Contains(u, "#")
}

// PinnedPackage reports whether p is <name>@<exact version> (26 4.3 item 6).
func PinnedPackage(p string) bool { return packagePattern.MatchString(p) }

// ValidModel reports whether m follows the pattern of default_model and does not start with a
// dash, which the runner would read as a flag (09 18.2).
func ValidModel(m string) bool { return modelPattern.MatchString(m) && !strings.HasPrefix(m, "-") }

// mcpPrefix starts the name of a tool of an MCP server.
const mcpPrefix = "mcp__"

// ValidTool reports whether t is a tool name of 26 4.2: a CLI tool name, optionally with a rule in
// parentheses, or mcp__<server>[__<tool>] of a server in listed.
func ValidTool(t string, listed []string) bool {
	rest, isMCP := strings.CutPrefix(t, mcpPrefix)
	if !isMCP {
		return builtinToolPattern.MatchString(t)
	}
	server, tool, hasTool := strings.Cut(rest, "__")
	if !slices.Contains(listed, server) {
		return false
	}
	return !hasTool || mcpToolPattern.MatchString(tool)
}

// UnmarshalJSON reads an entry of the stored form (value or secret_ref).
func (e *Entry) UnmarshalJSON(b []byte) error {
	var in entryJSON
	if err := json.Unmarshal(b, &in); err != nil {
		return err
	}
	*e = Entry{Name: in.Name, SecretRef: in.SecretRef}
	if in.Value != nil {
		e.Value, e.HasValue = *in.Value, true
	}
	return nil
}

// Decode reads a stored definition without checking it again: a version stored under earlier
// limits stays readable after an operator lowers a key. Errors: a text that is not a stored
// definition.
func Decode(raw []byte) (Definition, error) {
	var d Definition
	if err := json.Unmarshal(raw, &d); err != nil {
		return Definition{}, err
	}
	d.Skills, d.MCPServers = nonNil(d.Skills), nonNil(d.MCPServers)
	return d, nil
}
