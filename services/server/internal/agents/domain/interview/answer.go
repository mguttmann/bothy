// SPDX-License-Identifier: Apache-2.0

package interview

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/agents/domain/definition"
)

// The kinds of a builder answer (schema AgentInterviewBuilderAnswer).
const (
	KindQuestions = "questions"
	KindDraft     = "draft"
)

// maxQuestions is the most questions of one answer: "one or two questions per turn" is the rule
// of 26 6.1 item 2 and 6.3, not a configurable limit.
const maxQuestions = 2

// The members of the answer, of a cost report and the members a draft adds to a definition.
var (
	answerMembers = []string{"kind", "questions", "draft", "costs"}
	costMembers   = []string{"provider", "model", "currency", "tokens_in", "tokens_out", "tokens_cache_read",
		"tokens_cache_write", "tokens_cache_write_1h", "amount_micros"}
	draftExtras    = []string{memberName, memberHandle, memberDescription, memberChannel}
	channelMembers = []string{"space_name", "channel_name"}
)

const (
	memberName        = "name"
	memberHandle      = "handle"
	memberDescription = "description"
	memberChannel     = "channel_suggestion"
)

// refusingCodes are the findings that refuse a whole answer (builder_invalid) instead of going back
// as a field error of the draft: a member outside the schema (every surface of 26 5.5 that is a
// member: settings files, hooks, permission rules, commands, subagents, plugin parts, frontmatter
// keys, headersHelper), the text surfaces of 26 5.5 (frontmatter, inline shell, @ imports outside
// the folder), a value of the wrong shape that the typed answer of interviewAgent cannot carry, and
// a text that is no display text.
var refusingCodes = []string{
	definition.CodeUnknownMember, definition.CodeSkillFrontmatter, definition.CodeInlineShell, definition.CodeImportOutsideFolder,
	definition.CodeInvalidType, definition.CodeInvalidEnum,
	string(textRuleUTF8), string(textRuleControl), string(textRuleBidi),
}

// The textcheck rules as field codes.
const (
	textRuleUTF8    = "invalid_utf8"
	textRuleControl = "control_character"
	textRuleBidi    = "bidi_control"
)

// Cost is the usage of the builder's model call (schema CostReport).
type Cost struct {
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	Currency         string `json:"currency"`
	TokensIn         int64  `json:"tokens_in,omitempty"`
	TokensOut        int64  `json:"tokens_out,omitempty"`
	TokensCacheRead  int64  `json:"tokens_cache_read,omitempty"`
	TokensCacheWrite int64  `json:"tokens_cache_write,omitempty"`
	CacheWrite1h     int64  `json:"tokens_cache_write_1h,omitempty"`
	// AmountMicros is the adapter's estimate; nil when it reported none.
	AmountMicros *int64 `json:"amount_micros,omitempty"`
}

// Answer is a decoded builder answer that passed the answer schema: questions, or a draft with the
// findings that go back to the person, and the costs.
type Answer struct {
	Kind      string
	Questions []string
	// Draft is the draft object as the builder sent it (only members of schema AgentDraft); nil for
	// questions.
	Draft json.RawMessage
	// Findings are the field errors of the draft (26 4.3 for the class of its tool_class_wish and
	// mcp_not_in_catalogue of 26 7.4); none means the draft passes.
	Findings []definition.FieldError
	Costs    []Cost
}

// InvalidError is the refusal of a whole answer (fallback builder_invalid): its findings name the
// pointers and codes, never a value.
type InvalidError struct {
	Findings []definition.FieldError
}

// Error implements error.
func (e *InvalidError) Error() string {
	parts := make([]string, 0, len(e.Findings))
	for _, f := range e.Findings {
		parts = append(parts, f.Pointer+" "+f.Code)
	}
	return "agents: the builder answer breaks the answer schema: " + strings.Join(parts, ", ")
}

// answerChecker collects the findings of one answer.
type answerChecker struct {
	refuse []definition.FieldError
}

func (c *answerChecker) add(pointer, code string) {
	c.refuse = append(c.refuse, definition.FieldError{Pointer: pointer, Code: code})
}

// ParseAnswer decodes raw, the body of answerAgentInterview, strictly and checks it (26 6.1 items
// 2 and 3, 6.2): exactly one JSON object with the members of AgentInterviewBuilderAnswer; kind
// questions with one or two non-empty display texts of at most lim.QuestionMaxChars characters and
// no draft; kind draft with a draft object and no questions; cost reports with their members and
// no negative count. A draft is a definition (checked with package definition for the class of its
// settings.tool_class_wish) plus name, handle, description and an optional channel_suggestion;
// every drafted MCP server must equal a server of catalogue except its name.
//
// Errors: *InvalidError for an answer that is refused as a whole (see refusingCodes); the draft
// findings that are no refusal are in Answer.Findings.
//
// Example:
//
//	a, err := interview.ParseAnswer([]byte(`{"kind":"questions","questions":["Who reads it?"]}`), lim, nil)
func ParseAnswer(raw []byte, lim Limits, catalogue []CatalogueEntry) (Answer, error) {
	c := &answerChecker{}
	v, ok := decodeOne(raw)
	if !ok {
		c.add("", definition.CodeInvalidType)
		return Answer{}, &InvalidError{Findings: c.refuse}
	}
	m, ok := v.(map[string]any)
	if !ok {
		c.add("", definition.CodeInvalidType)
		return Answer{}, &InvalidError{Findings: c.refuse}
	}
	c.unknown("", m, answerMembers)
	a := Answer{Costs: c.costs(m)}
	kind, _ := m["kind"].(string)
	a.Kind = kind
	_, hasQuestions := m["questions"]
	d, hasDraft := m["draft"]
	switch kind {
	case KindQuestions:
		if hasDraft {
			c.add("/draft", definition.CodeInvalid)
		}
		a.Questions = c.questions(m, lim.QuestionMaxChars)
	case KindDraft:
		if hasQuestions {
			c.add("/questions", definition.CodeInvalid)
		}
		if !hasDraft {
			c.add("/draft", definition.CodeRequired)
			break
		}
		a.Draft, a.Findings = c.draft(d, lim, catalogue)
	default:
		c.add("/kind", definition.CodeInvalidEnum)
	}
	if len(c.refuse) > 0 {
		return Answer{}, &InvalidError{Findings: c.refuse}
	}
	return a, nil
}

// decodeOne decodes raw as exactly one JSON value with numbers kept as json.Number.
func decodeOne(raw []byte) (any, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, false
	}
	_, err := dec.Token()
	return v, errors.Is(err, io.EOF)
}

// unknown records every member of m outside allowed below pointer p.
func (c *answerChecker) unknown(p string, m map[string]any, allowed []string) {
	var keys []string
	for k := range m {
		if !slices.Contains(allowed, k) {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	for _, k := range keys {
		c.add(p+"/"+escape(k), definition.CodeUnknownMember)
	}
}

// pointerEscaper escapes a member name for a JSON Pointer (RFC 6901).
var pointerEscaper = strings.NewReplacer("~", "~0", "/", "~1")

func escape(name string) string { return pointerEscaper.Replace(name) }

// questions checks the questions of kind questions.
func (c *answerChecker) questions(m map[string]any, maxChars int) []string {
	items, ok := m["questions"].([]any)
	if !ok {
		c.add("/questions", definition.CodeRequired)
		return nil
	}
	if len(items) == 0 || len(items) > maxQuestions {
		c.add("/questions", CodeTooManyItems)
		return nil
	}
	out := make([]string, 0, len(items))
	for i, it := range items {
		p := "/questions/" + strconv.Itoa(i)
		q, ok := it.(string)
		switch {
		case !ok:
			c.add(p, definition.CodeInvalidType)
		case strings.TrimSpace(q) == "":
			c.add(p, CodeTooShort)
		case utf8.RuneCountInString(q) > maxChars:
			c.add(p, CodeTooLong)
		default:
			if rule := markdownRule(q); rule != "" {
				c.add(p, rule)
			}
		}
		out = append(out, q)
	}
	return out
}

// costs decodes the cost reports strictly: the members of CostReport, provider, model and currency
// as strings, counts and amount as integers that are not negative.
func (c *answerChecker) costs(m map[string]any) []Cost {
	v, present := m["costs"]
	if !present {
		return nil
	}
	items, ok := v.([]any)
	if !ok {
		c.add("/costs", definition.CodeInvalidType)
		return nil
	}
	out := make([]Cost, 0, len(items))
	for i, it := range items {
		p := "/costs/" + strconv.Itoa(i)
		cm, ok := it.(map[string]any)
		if !ok {
			c.add(p, definition.CodeInvalidType)
			continue
		}
		c.unknown(p, cm, costMembers)
		var cost Cost
		cost.Provider = c.text(p, cm, "provider")
		cost.Model = c.text(p, cm, "model")
		cost.Currency = c.text(p, cm, "currency")
		cost.TokensIn = c.count(p, cm, "tokens_in")
		cost.TokensOut = c.count(p, cm, "tokens_out")
		cost.TokensCacheRead = c.count(p, cm, "tokens_cache_read")
		cost.TokensCacheWrite = c.count(p, cm, "tokens_cache_write")
		cost.CacheWrite1h = c.count(p, cm, "tokens_cache_write_1h")
		if _, has := cm["amount_micros"]; has {
			n := c.count(p, cm, "amount_micros")
			cost.AmountMicros = &n
		}
		out = append(out, cost)
	}
	return out
}

// text returns the required display text member name of m.
func (c *answerChecker) text(p string, m map[string]any, name string) string {
	s, ok := m[name].(string)
	switch {
	case !ok:
		c.add(p+"/"+name, definition.CodeRequired)
	case markdownRule(s) != "" || strings.ContainsAny(s, "\r\n"):
		c.add(p+"/"+name, definition.CodeInvalid)
	}
	return s
}

// count returns the optional non-negative integer member name of m.
func (c *answerChecker) count(p string, m map[string]any, name string) int64 {
	v, present := m[name]
	if !present {
		return 0
	}
	n, ok := v.(json.Number)
	if !ok {
		c.add(p+"/"+name, definition.CodeInvalidType)
		return 0
	}
	i, err := strconv.ParseInt(n.String(), 10, 64)
	if err != nil || i < 0 {
		c.add(p+"/"+name, definition.CodeInvalid)
		return 0
	}
	return i
}

// draft checks a draft object and returns its JSON form and the findings that go back to the
// person; refusing findings land in c.
func (c *answerChecker) draft(v any, lim Limits, catalogue []CatalogueEntry) (json.RawMessage, []definition.FieldError) {
	const p = "/draft"
	m, ok := v.(map[string]any)
	if !ok {
		c.add(p, definition.CodeInvalidType)
		return nil, nil
	}
	def := make(map[string]any, len(m))
	extras := map[string]any{}
	for k, val := range m {
		if slices.Contains(draftExtras, k) {
			extras[k] = val
			continue
		}
		def[k] = val
	}
	defRaw, err := json.Marshal(def)
	if err != nil {
		c.add(p, definition.CodeInvalid)
		return nil, nil
	}
	settings, _ := def["settings"].(map[string]any)
	wish, _ := settings["tool_class_wish"].(string)
	_, found := definition.Parse(defRaw, lim.Definition, definition.ToolClass(wish))
	found = append(found, extraFindings(extras)...)
	if servers, err := definition.Decode(defRaw); err == nil {
		found = append(found, notInCatalogue(servers.MCPServers, catalogue)...)
	}
	var keep []definition.FieldError
	for _, f := range found {
		if slices.Contains(refusingCodes, f.Code) || !resolves(m, f.Pointer) {
			c.add(p+f.Pointer, f.Code)
			continue
		}
		keep = append(keep, f)
	}
	out, err := json.Marshal(m)
	if err != nil {
		c.add(p, definition.CodeInvalid)
		return nil, nil
	}
	return out, keep
}

// extraFindings checks the members a draft adds to a definition: name, handle and description are
// required display texts, name and handle not blank; channel_suggestion, when present, is an object
// with exactly space_name and channel_name, both non-blank display texts. createAgent and
// channel.create check names and handles with their own rules later.
func extraFindings(m map[string]any) []definition.FieldError {
	var out []definition.FieldError
	add := func(p, code string) { out = append(out, definition.FieldError{Pointer: p, Code: code}) }
	line := func(p string, v any, present, blankOK bool) {
		s, ok := v.(string)
		switch {
		case !present:
			add(p, definition.CodeRequired)
		case !ok:
			add(p, definition.CodeInvalidType)
		case lineRule(s) != "":
			add(p, lineRule(s))
		case !blankOK && strings.TrimSpace(s) == "":
			add(p, definition.CodeRequired)
		}
	}
	for _, n := range []string{memberName, memberHandle, memberDescription} {
		v, present := m[n]
		line("/"+n, v, present, n == memberDescription)
	}
	cv, present := m[memberChannel]
	if !present {
		return out
	}
	cm, ok := cv.(map[string]any)
	if !ok {
		add("/"+memberChannel, definition.CodeInvalidType)
		return out
	}
	for k := range cm {
		if !slices.Contains(channelMembers, k) {
			add("/"+memberChannel+"/"+escape(k), definition.CodeUnknownMember)
		}
	}
	for _, n := range channelMembers {
		v, present := cm[n]
		line("/"+memberChannel+"/"+n, v, present, false)
	}
	return out
}

// lineRule returns the textcheck rule a one-line display text breaks, "" when it passes.
func lineRule(s string) string {
	if strings.ContainsAny(s, "\r\n") {
		return textRuleControl
	}
	return markdownRule(s)
}

// resolves reports whether the JSON Pointer p names a value inside doc; a finding on a missing
// member (required) does not, so the typed answer could not carry it.
func resolves(doc any, p string) bool {
	if p == "" {
		return true
	}
	cur := doc
	for _, tok := range strings.Split(p[1:], "/") {
		tok = strings.ReplaceAll(strings.ReplaceAll(tok, "~1", "/"), "~0", "~")
		switch v := cur.(type) {
		case map[string]any:
			next, ok := v[tok]
			if !ok {
				return false
			}
			cur = next
		case []any:
			i, err := strconv.Atoi(tok)
			if err != nil || i < 0 || i >= len(v) {
				return false
			}
			cur = v[i]
		default:
			return false
		}
	}
	return true
}

// notInCatalogue returns mcp_not_in_catalogue for every drafted server that equals no server of
// catalogue except its name (26 7.4).
func notInCatalogue(servers []definition.MCPServer, catalogue []CatalogueEntry) []definition.FieldError {
	var out []definition.FieldError
	for i, s := range servers {
		if !inCatalogue(s, catalogue) {
			out = append(out, definition.FieldError{Pointer: "/mcp_servers/" + strconv.Itoa(i), Code: CodeMCPNotInCatalogue})
		}
	}
	return out
}

// inCatalogue reports whether s equals a server of catalogue except its name.
func inCatalogue(s definition.MCPServer, catalogue []CatalogueEntry) bool {
	for _, t := range catalogue {
		for _, cs := range t.MCPServers {
			if SameServer(s, cs) {
				return true
			}
		}
	}
	return false
}

// SameServer reports whether a and b are the same MCP server except their names (26 7.4): for
// stdio the command, the package, the full argument list in the same order and count and every
// env[] entry; for http the URL and every headers[] entry. An entry is equal with the same name and
// the same value or the same secret_ref; a switch between value and secret_ref is a difference.
// Entries compare by name, so their order does not matter.
//
// Example:
//
//	interview.SameServer(drafted, catalogueServer) // false when one argument differs
func SameServer(a, b definition.MCPServer) bool {
	if a.Transport != b.Transport {
		return false
	}
	switch a.Transport {
	case definition.TransportStdio:
		return a.Command == b.Command && a.Package == b.Package && slices.Equal(a.Args, b.Args) && sameEntries(a.Env, b.Env)
	case definition.TransportHTTP:
		return a.URL == b.URL && sameEntries(a.Headers, b.Headers)
	}
	return false
}

// sameEntries reports whether a and b hold the same entries by name.
func sameEntries(a, b []definition.Entry) bool {
	if len(a) != len(b) {
		return false
	}
	for _, e := range a {
		i := slices.IndexFunc(b, func(o definition.Entry) bool { return o.Name == e.Name })
		if i < 0 || b[i] != e {
			return false
		}
	}
	return true
}

// Estimate sums the amounts of one currency (the estimate of D-IS-8): ok is false without costs,
// when an amount is missing or when the currencies differ, because such an estimate would mislead.
//
// Example:
//
//	micros, currency, ok := interview.Estimate(a.Costs)
func Estimate(costs []Cost) (micros int64, currency string, ok bool) {
	if len(costs) == 0 {
		return 0, "", false
	}
	currency = costs[0].Currency
	for _, c := range costs {
		if c.AmountMicros == nil || *c.AmountMicros < 0 || c.Currency != currency {
			return 0, "", false
		}
		micros += *c.AmountMicros
	}
	return micros, currency, true
}
