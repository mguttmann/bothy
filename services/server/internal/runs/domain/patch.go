// SPDX-License-Identifier: Apache-2.0

package domain

// The static checks of a live-box patch (M2-10, docs/spec/09-agents.md section 5.3): ParseOps
// parses each RFC 6902 operation, matches its JSON Pointer against the path table and checks the
// value against the schema of the addressed member, without the database. Apply (patch_apply.go)
// then applies the checked operations to the locked state.

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The field error codes of the patch rules. The display text rules add the codes of
// textcheck.Rule (invalid_utf8, control_character, bidi_control); required, unknown_member,
// invalid, invalid_format and invalid_enum are shared with the option values (command_exec.go).
const (
	fieldPathForbidden   = "patch_path_forbidden"
	fieldInvalidType     = "invalid_type"
	fieldTooSmall        = "too_small"
	fieldTooLarge        = "too_large"
	fieldPatternMismatch = "pattern_mismatch"
	fieldTargetExists    = "patch_target_exists"
	fieldTargetMissing   = "patch_target_missing"
	fieldLimitReached    = "limit_reached"
)

// maxSafeInteger is 2^53-1, the largest integer every JSON client reads exactly; the integer
// members of run_metrics.v1, RunApiRetry and Money stay within 0 and it.
const maxSafeInteger = 1<<53 - 1

// stepStatusCompleted is the status of a step or plan item that finished successfully.
const stepStatusCompleted = "completed"

// The member names that several patchable objects share.
const (
	memberStatus = "status"
	memberDetail = "detail"
	memberTitle  = "title"
)

// The enums of the patchable members (04 schemas RunStep, RunPlan, RunFollowup).
var (
	stepKindValues     = []string{"tool_call", summaryKind, "subagent", "workflow", "background_job"}
	stepStatusValues   = []string{"pending", "running", stepStatusCompleted, "failed", "cancelled"}
	planStatusValues   = []string{"pending", "in_progress", stepStatusCompleted}
	followupStateValue = []string{"delivered", "absorbed"}
)

// metricFields are the numeric fields of run_metrics.v1 a patch may set at /metrics/<field>;
// progress_events counts runtime progress without a visible change, and its patch counts as
// progress against run.stall_after (09 5.5, 18.4, M2-51).
var metricFields = []string{
	"context_pct", "tokens_in", "tokens_out", "tokens_cache_read", "tokens_cache_write",
	"tokens_cache_write_1h", "subagents", "background_jobs", "api_retries", "progress_events",
}

// metricContextPct is the one metric that is a percentage (a number from 0 to 100), not a count.
const metricContextPct = "context_pct"

// errPointer is the error of ParsePointer; it never repeats the pointer, which comes from the
// adapter.
var errPointer = errors.New("runs: malformed JSON pointer")

// ParsePointer splits a JSON Pointer (RFC 6901) into its reference tokens: "" is the root (no
// tokens), any other pointer starts with "/", and each token is unescaped "~1" to "/" first, then
// "~0" to "~" (in a single pass, so "~01" is the token "~1"). A "~" followed by anything other
// than "0" or "1", or at the end of a token, is malformed. Empty tokens are kept: "/a/" is
// ["a", ""].
//
// It returns the tokens or an error that does not contain the pointer.
//
// Example:
//
//	toks, _ := domain.ParsePointer("/metrics/a~1b") // ["metrics", "a/b"]
func ParsePointer(p string) ([]string, error) {
	if p == "" {
		return []string{}, nil
	}
	if p[0] != '/' {
		return nil, errPointer
	}
	toks := strings.Split(p[1:], "/")
	for i, tok := range toks {
		if !strings.Contains(tok, "~") {
			continue
		}
		u, err := unescapeToken(tok)
		if err != nil {
			return nil, err
		}
		toks[i] = u
	}
	return toks, nil
}

// unescapeToken unescapes one reference token in a single pass: "~0" is "~" and "~1" is "/"; any
// other "~" is errPointer.
func unescapeToken(tok string) (string, error) {
	var b strings.Builder
	for j := 0; j < len(tok); j++ {
		if tok[j] != '~' {
			b.WriteByte(tok[j])
			continue
		}
		if j+1 == len(tok) {
			return "", errPointer
		}
		switch tok[j+1] {
		case '0':
			b.WriteByte('~')
		case '1':
			b.WriteByte('/')
		default:
			return "", errPointer
		}
		j++
	}
	return b.String(), nil
}

// escapeToken escapes one reference token for a JSON Pointer (RFC 6901 section 3); the replacer
// works in one pass, so "~" and "/" never interfere.
var escapeToken = strings.NewReplacer("~", "~0", "/", "~1").Replace

// route is one row of the path table, resolved for a concrete pointer.
type route struct {
	target Target
	field  string
	id     ids.ID
	ops    []OpKind
}

// resolvePath matches tokens against the path table of 09 5.3. It returns the route, or the field
// code invalid_format (an id token that is not a UUIDv7) or patch_path_forbidden (every path
// outside the table). The id of a step or follow-up is checked before its member, so
// /steps/not-a-uuid/status is invalid_format and not forbidden.
func resolvePath(toks []string) (route, string) {
	switch {
	case len(toks) == 0:
		return route{}, fieldPathForbidden // the root: a patch never replaces the whole document
	case len(toks) == 1:
		return resolveTop(toks[0])
	case toks[0] == "metrics" && len(toks) == 2:
		return resolveMetric(toks[1])
	case toks[0] == "steps" && (len(toks) == 2 || len(toks) == 3):
		return resolveStep(toks[1:])
	case toks[0] == "followups" && len(toks) == 3:
		return resolveFollowup(toks[1], toks[2])
	}
	return route{}, fieldPathForbidden
}

// The operations a row of the path table allows.
var (
	opsAll        = []OpKind{OpAdd, OpReplace, OpRemove}
	opsAddReplace = []OpKind{OpAdd, OpReplace}
	opsReplace    = []OpKind{OpReplace}
)

// resolveTop resolves a path of one token, a top level member of the run document.
func resolveTop(tok string) (route, string) {
	switch tok {
	case "status_line":
		return route{target: TargetStatusLine, ops: opsAll}, ""
	case "last_text_excerpt":
		return route{target: TargetLastTextExcerpt, ops: opsAddReplace}, ""
	case "model":
		return route{target: TargetModel, ops: opsReplace}, ""
	case "cost":
		return route{target: TargetCost, ops: opsReplace}, ""
	case "plan":
		return route{target: TargetPlan, ops: opsAll}, ""
	}
	return route{}, fieldPathForbidden
}

// resolveMetric resolves /metrics/<field>.
func resolveMetric(field string) (route, string) {
	if field == "last_api_retry" {
		return route{target: TargetLastAPIRetry, ops: opsAll}, ""
	}
	if slices.Contains(metricFields, field) {
		return route{target: TargetMetric, field: field, ops: opsAddReplace}, ""
	}
	return route{}, fieldPathForbidden
}

// resolveStep resolves /steps/<id> and /steps/<id>/<member>; toks holds the id and the optional
// member.
func resolveStep(toks []string) (route, string) {
	id, err := ids.Parse(toks[0])
	if err != nil {
		return route{}, fieldInvalidFormat
	}
	if len(toks) == 1 {
		return route{target: TargetStep, id: id, ops: []OpKind{OpAdd}}, ""
	}
	switch toks[1] {
	case memberStatus, memberTitle, memberDetail, "finished_at":
		return route{target: TargetStepMember, field: toks[1], id: id, ops: opsReplace}, ""
	}
	return route{}, fieldPathForbidden
}

// resolveFollowup resolves /followups/<id>/<member>; only the member state is patchable.
func resolveFollowup(idTok, member string) (route, string) {
	id, err := ids.Parse(idTok)
	if err != nil {
		return route{}, fieldInvalidFormat
	}
	if member == "state" {
		return route{target: TargetFollowupState, id: id, ops: opsReplace}, ""
	}
	return route{}, fieldPathForbidden
}

// collector gathers field errors in the order they are found.
type collector struct {
	list []errs.FieldError
}

// The fixed details of the field errors. They never contain the value (SEC-LOG-4): the value
// comes from the adapter and may be anything.
const (
	detailOpEnum        = "the operation is not add, replace or remove"
	detailPointer       = "the path is not a valid JSON Pointer"
	detailPathID        = "a path segment is not a valid UUIDv7"
	detailForbidden     = "this path is not patchable with this operation"
	detailValueRequired = "the operation needs a value"
	detailValueJSON     = "the value is not valid JSON"
	detailNull          = "null is not allowed here"
	detailUnstorable    = "the value holds U+0000 or a number beyond 2^53-1"
	detailType          = "the value has the wrong JSON type"
	detailTooSmall      = "the number is below the minimum"
	detailTooLarge      = "the number is above the maximum"
	detailUnknown       = "this member is not allowed"
	detailRequired      = "this member is required"
	detailEnum          = "the value is not one of the allowed values"
	detailTime          = "the value is not an RFC 3339 time"
	detailUUID          = "the value is not a valid UUIDv7"
	detailCurrency      = "the currency must be three upper case letters"
	detailEstimate      = "a reported cost must be an estimate"
	detailStepID        = "the id must equal the id in the path"
	detailPlanItemID    = "the plan item id is not unique"
	detailOrdinal       = "the server assigns the ordinal"
	detailDisplayText   = "the text contains a character that is not allowed"
)

// add records one field error.
func (c *collector) add(ptr, code, detail string) {
	c.list = append(c.list, errs.FieldError{Pointer: ptr, Code: code, Detail: detail})
}

// ParseOps checks a patch without the database: the operation, the JSON Pointer against the path
// table of 09 5.3 and the value against the schema of the member (types, enums, bounds, closed
// objects, display texts after SEC-IN-4). It returns the checked operations in patch order, or,
// when any operation fails, nil and every field error found, in operation order. Pointers name
// the operation (/ops/<i>), its path (/ops/<i>/path) or a place in its value
// (/ops/<i>/value/...). The details are fixed sentences that never repeat a value.
//
// A remove carries no value; a value sent with it is ignored (RFC 6902 section 4.2). The dynamic
// checks (does the step exist, is the id free) are Apply's.
//
// Example:
//
//	ops, fieldErrs := domain.ParseOps([]domain.RawOp{{Op: "replace", Path: "/status_line", Value: []byte(`"Reading"`)}})
//	// len(fieldErrs) == 0, ops[0].Target == domain.TargetStatusLine, ops[0].Value == "Reading"
func ParseOps(raw []RawOp) ([]Op, []errs.FieldError) {
	var c collector
	ops := make([]Op, 0, len(raw))
	for i, r := range raw {
		if op, ok := parseOp(i, r, &c); ok {
			ops = append(ops, op)
		}
	}
	if len(c.list) > 0 {
		return nil, c.list
	}
	return ops, nil
}

// parseOp checks operation i; it reports false when it added a field error.
func parseOp(i int, r RawOp, c *collector) (Op, bool) {
	at := "/ops/" + strconv.Itoa(i)
	kind := OpKind(r.Op)
	if kind != OpAdd && kind != OpReplace && kind != OpRemove {
		c.add(at+"/op", fieldInvalidEnum, detailOpEnum)
		return Op{}, false
	}
	toks, err := ParsePointer(r.Path)
	if err != nil {
		c.add(at+"/path", fieldInvalidFormat, detailPointer)
		return Op{}, false
	}
	rt, code := resolvePath(toks)
	switch {
	case code == fieldInvalidFormat:
		c.add(at+"/path", fieldInvalidFormat, detailPathID)
		return Op{}, false
	case code != "" || !slices.Contains(rt.ops, kind):
		c.add(at, fieldPathForbidden, detailForbidden)
		return Op{}, false
	}
	op := Op{Index: i, Kind: kind, Path: r.Path, Target: rt.target, Field: rt.field, ID: rt.id}
	if kind == OpRemove {
		return op, true
	}
	vp := at + "/value"
	if r.Value == nil {
		c.add(vp, fieldRequired, detailValueRequired)
		return Op{}, false
	}
	v, err := decodeJSON(r.Value)
	if err != nil {
		c.add(vp, fieldInvalidFormat, detailValueJSON)
		return Op{}, false
	}
	if p, found := findNull(v, vp); found {
		c.add(p, fieldInvalidType, detailNull)
		return Op{}, false
	}
	if p, code, found := findUnstorable(v, vp); found {
		c.add(p, code, detailUnstorable)
		return Op{}, false
	}
	start := len(c.list)
	op.Value = checkValue(rt, v, vp, c)
	return op, len(c.list) == start
}

// decodeJSON decodes exactly one JSON value with json.Number for numbers, so integers keep every
// digit and the canonical form keeps the literal.
func decodeJSON(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("runs: data after the JSON value")
	}
	return v, nil
}

// findNull returns the pointer of the first JSON null in v (members in sorted order), because no
// patchable member and no step detail may hold null.
func findNull(v any, ptr string) (string, bool) {
	switch x := v.(type) {
	case nil:
		return ptr, true
	case map[string]any:
		for _, k := range sortedKeys(x) {
			if p, ok := findNull(x[k], ptr+"/"+escapeToken(k)); ok {
				return p, true
			}
		}
	case []any:
		for i, e := range x {
			if p, ok := findNull(e, ptr+"/"+strconv.Itoa(i)); ok {
				return p, true
			}
		}
	}
	return "", false
}

// findUnstorable returns the pointer and field code of the first part of v that the run document
// cannot carry: a string or member name with U+0000, which PostgreSQL refuses in text and jsonb
// (invalid), and a number whose absolute value exceeds 2^53-1, which no JSON client reads exactly
// and the response writer refuses (ADR 0053; too_large or too_small). It walks members in sorted
// order, like findNull, so step details and every other value obey the same rule.
func findUnstorable(v any, ptr string) (string, string, bool) {
	switch x := v.(type) {
	case string:
		if strings.ContainsRune(x, 0) {
			return ptr, fieldInvalid, true
		}
	case json.Number:
		code, bad := unsafeNumber(x)
		return ptr, code, bad
	case map[string]any:
		for _, k := range sortedKeys(x) {
			if q, code, ok := unstorableMember(k, x[k], ptr+"/"+escapeToken(k)); ok {
				return q, code, true
			}
		}
	case []any:
		for i, e := range x {
			if q, code, ok := findUnstorable(e, ptr+"/"+strconv.Itoa(i)); ok {
				return q, code, true
			}
		}
	}
	return "", "", false
}

// unstorableMember is findUnstorable for the member k with value e at pointer p: the name may not
// hold U+0000 either.
func unstorableMember(k string, e any, p string) (string, string, bool) {
	if strings.ContainsRune(k, 0) {
		return p, fieldInvalid, true
	}
	return findUnstorable(e, p)
}

// unsafeNumber reports a number whose absolute value exceeds 2^53-1 (ADR 0053) with too_large
// or too_small.
func unsafeNumber(n json.Number) (string, bool) {
	f, err := n.Float64()
	switch {
	case err != nil:
		return "", false
	case f > maxSafeInteger:
		return fieldTooLarge, true
	case f < -maxSafeInteger:
		return fieldTooSmall, true
	}
	return "", false
}

// sortedKeys returns the keys of m in byte order, the order encoding/json writes them in.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// checkValue checks v for the member of rt and returns the typed value of Op.Value; errors go to c.
func checkValue(rt route, v any, ptr string, c *collector) any {
	switch rt.target {
	case TargetStatusLine, TargetModel:
		s, _ := c.displayText(v, ptr)
		return s
	case TargetLastTextExcerpt:
		// Agent output: any string, no display text rules (09 5.3).
		s, _ := c.str(v, ptr)
		return s
	case TargetMetric:
		if rt.field == metricContextPct {
			f, _ := c.percent(v, ptr)
			return f
		}
		n, _ := c.integer(v, ptr)
		return n
	case TargetLastAPIRetry:
		return c.apiRetry(v, ptr)
	case TargetCost:
		return c.money(v, ptr)
	case TargetPlan:
		return c.plan(v, ptr)
	case TargetStep:
		return c.stepInput(v, ptr, rt.id)
	case TargetStepMember:
		return c.stepMember(rt.field, v, ptr)
	case TargetFollowupState:
		return c.enum(v, ptr, followupStateValue)
	}
	return nil
}

// stepMember checks v for the step member field (status, title, detail or finished_at).
func (c *collector) stepMember(field string, v any, ptr string) any {
	switch field {
	case memberStatus:
		return c.enum(v, ptr, stepStatusValues)
	case memberTitle:
		s, _ := c.displayText(v, ptr)
		return s
	case memberDetail:
		return c.detail(v, ptr)
	default: // finished_at
		return c.timestamp(v, ptr)
	}
}

// str checks that v is a string.
func (c *collector) str(v any, ptr string) (string, bool) {
	s, ok := v.(string)
	if !ok {
		c.add(ptr, fieldInvalidType, detailType)
	}
	return s, ok
}

// displayText checks that v is a string that passes textcheck.DisplayText; the field code is the
// violated rule.
func (c *collector) displayText(v any, ptr string) (string, bool) {
	s, ok := c.str(v, ptr)
	if !ok {
		return "", false
	}
	if err := textcheck.DisplayText(s); err != nil {
		code := fieldInvalid
		var te *textcheck.Error
		if errors.As(err, &te) {
			code = string(te.Rule)
		}
		c.add(ptr, code, detailDisplayText)
		return "", false
	}
	return s, true
}

// enum checks that v is one of the strings allowed; it returns "" when it is not.
func (c *collector) enum(v any, ptr string, allowed []string) string {
	s, ok := c.str(v, ptr)
	if !ok {
		return ""
	}
	if !slices.Contains(allowed, s) {
		c.add(ptr, fieldInvalidEnum, detailEnum)
		return ""
	}
	return s
}

// timestamp checks that v is an RFC 3339 time; it returns the zero time when it is not.
func (c *collector) timestamp(v any, ptr string) time.Time {
	s, ok := c.str(v, ptr)
	if !ok {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		c.add(ptr, fieldInvalidFormat, detailTime)
		return time.Time{}
	}
	return t
}

// detail checks that v is a JSON object and returns its canonical encoding (members sorted).
// Agent output: no display text rules. It returns nil when v is not an object.
func (c *collector) detail(v any, ptr string) json.RawMessage {
	m, ok := v.(map[string]any)
	if !ok {
		c.add(ptr, fieldInvalidType, detailType)
		return nil
	}
	return encodeCanonical(m)
}

// integer checks that v is an integral JSON number from 0 to 2^53-1. A literal with a fraction or
// exponent counts when its value is integral (1.0 and 1e2 are integers, 1.5 is not), as in JSON
// Schema.
func (c *collector) integer(v any, ptr string) (int64, bool) {
	num, ok := v.(json.Number)
	if !ok {
		c.add(ptr, fieldInvalidType, detailType)
		return 0, false
	}
	n, code := integral(string(num))
	switch code {
	case fieldInvalidType:
		c.add(ptr, code, detailType)
	case fieldTooSmall:
		c.add(ptr, code, detailTooSmall)
	case fieldTooLarge:
		c.add(ptr, code, detailTooLarge)
	default:
		return n, true
	}
	return 0, false
}

// integral classifies a JSON number literal (as the decoder accepted it) without big
// arithmetic, so a literal like 1e999999999 costs nothing: it returns the value and "", or one of
// the codes invalid_type (not integral), too_small (negative) and too_large (above 2^53-1).
func integral(lit string) (int64, string) {
	neg := strings.HasPrefix(lit, "-")
	s := strings.TrimPrefix(lit, "-")
	mant, exp := s, ""
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		mant, exp = s[:i], s[i+1:]
	}
	whole, frac := mant, ""
	if i := strings.IndexByte(mant, '.'); i >= 0 {
		whole, frac = mant[:i], mant[i+1:]
	}
	digits := strings.TrimLeft(whole+frac, "0")
	if digits == "" {
		return 0, "" // zero, also -0 and 0.0e5
	}
	// The value is digits * 10^scale; trailing zeros move into the scale.
	scale := exponent(exp) - len(frac)
	trimmed := strings.TrimRight(digits, "0")
	scale += len(digits) - len(trimmed)
	switch {
	case scale < 0:
		return 0, fieldInvalidType
	case neg:
		return 0, fieldTooSmall
	case len(trimmed)+scale > len(strconv.Itoa(maxSafeInteger)):
		return 0, fieldTooLarge
	}
	n, err := strconv.ParseInt(trimmed+strings.Repeat("0", scale), 10, 64)
	if err != nil || n > maxSafeInteger {
		return 0, fieldTooLarge
	}
	return n, ""
}

// exponent reads the exponent of a number literal and saturates it, because only its sign and
// whether it is far beyond 16 digits matter.
func exponent(s string) int {
	if s == "" {
		return 0
	}
	neg := s[0] == '-'
	s = strings.TrimLeft(strings.TrimLeft(s, "+-"), "0")
	const limit = 1_000_000
	n := limit
	if len(s) < 7 {
		n, _ = strconv.Atoi("0" + s)
	}
	if neg {
		return -n
	}
	return n
}

// percent checks that v is a number from 0 to 100 (context_pct).
func (c *collector) percent(v any, ptr string) (float64, bool) {
	num, ok := v.(json.Number)
	if !ok {
		c.add(ptr, fieldInvalidType, detailType)
		return 0, false
	}
	// ParseFloat returns ±Inf with an error for a literal beyond float64; the sign decides.
	f, _ := strconv.ParseFloat(string(num), 64)
	switch {
	case f < 0:
		c.add(ptr, fieldTooSmall, detailTooSmall)
	case f > 100:
		c.add(ptr, fieldTooLarge, detailTooLarge)
	default:
		return f, true
	}
	return 0, false
}

// object checks that v is a JSON object.
func (c *collector) object(v any, ptr string) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		c.add(ptr, fieldInvalidType, detailType)
	}
	return m, ok
}

// member returns member name of m and its pointer; a missing required member is a field error.
func (c *collector) member(m map[string]any, ptr, name string, required bool) (any, string, bool) {
	p := ptr + "/" + escapeToken(name)
	v, ok := m[name]
	if !ok && required {
		c.add(p, fieldRequired, detailRequired)
	}
	return v, p, ok
}

// closed reports every member of m outside known as unknown_member, in sorted order, because the
// schemas of the patchable objects allow no other members.
func (c *collector) closed(m map[string]any, ptr string, known ...string) {
	for _, k := range sortedKeys(m) {
		if !slices.Contains(known, k) {
			c.add(ptr+"/"+escapeToken(k), fieldUnknownMember, detailUnknown)
		}
	}
}

// optionalInt checks an optional integer member and returns nil when it is absent or invalid.
func (c *collector) optionalInt(m map[string]any, ptr, name string) *int64 {
	v, p, ok := c.member(m, ptr, name, false)
	if !ok {
		return nil
	}
	n, valid := c.integer(v, p)
	if !valid {
		return nil
	}
	return &n
}

// apiRetry checks the closed object RunApiRetry.
func (c *collector) apiRetry(v any, ptr string) APIRetry {
	m, ok := c.object(v, ptr)
	if !ok {
		return APIRetry{}
	}
	var r APIRetry
	if a, p, ok := c.member(m, ptr, "attempt", true); ok {
		r.Attempt, _ = c.integer(a, p)
	}
	r.MaxRetries = c.optionalInt(m, ptr, "max_retries")
	r.RetryDelayMS = c.optionalInt(m, ptr, "retry_delay_ms")
	if e, p, ok := c.member(m, ptr, "error", false); ok {
		if s, valid := c.displayText(e, p); valid {
			r.Error = &s
		}
	}
	c.closed(m, ptr, "attempt", "max_retries", "retry_delay_ms", "error")
	return r
}

// money checks the closed object Money of /cost: a reported cost is always an estimate, the
// reconciled amount comes from the invoice (M2-22).
func (c *collector) money(v any, ptr string) Money {
	m, ok := c.object(v, ptr)
	if !ok {
		return Money{}
	}
	var out Money
	if a, p, ok := c.member(m, ptr, "amount_micros", true); ok {
		out.AmountMicros, _ = c.integer(a, p)
	}
	if cur, p, ok := c.member(m, ptr, "currency", true); ok {
		if s, valid := c.str(cur, p); valid {
			if !isCurrency(s) {
				c.add(p, fieldPatternMismatch, detailCurrency)
			}
			out.Currency = s
		}
	}
	if est, p, ok := c.member(m, ptr, "is_estimate", true); ok {
		switch b, isBool := est.(bool); {
		case !isBool:
			c.add(p, fieldInvalidType, detailType)
		case !b:
			c.add(p, fieldInvalid, detailEstimate)
		default:
			out.IsEstimate = true
		}
	}
	c.closed(m, ptr, "amount_micros", "currency", "is_estimate")
	return out
}

// isCurrency reports three upper case ASCII letters (the form of ISO 4217 codes).
func isCurrency(s string) bool {
	if len(s) != 3 {
		return false
	}
	for i := 0; i < 3; i++ {
		if s[i] < 'A' || s[i] > 'Z' {
			return false
		}
	}
	return true
}

// plan checks the closed object RunPlan; item ids must be unique within the plan.
func (c *collector) plan(v any, ptr string) Plan {
	m, ok := c.object(v, ptr)
	if !ok {
		return Plan{}
	}
	var out Plan
	if raw, p, ok := c.member(m, ptr, "items", true); ok {
		items, isArray := raw.([]any)
		if !isArray {
			c.add(p, fieldInvalidType, detailType)
		}
		seen := map[string]bool{}
		for j, it := range items {
			if item, ok := c.planItem(it, p+"/"+strconv.Itoa(j), seen); ok {
				out.Items = append(out.Items, item)
			}
		}
	}
	c.closed(m, ptr, "items")
	return out
}

// planItem checks one closed item of RunPlan at ip; seen holds the item ids so far and gains this
// one. It reports false when the item is not an object.
func (c *collector) planItem(it any, ip string, seen map[string]bool) (PlanItem, bool) {
	im, ok := c.object(it, ip)
	if !ok {
		return PlanItem{}, false
	}
	var item PlanItem
	if id, idp, ok := c.member(im, ip, "id", true); ok {
		if s, valid := c.str(id, idp); valid {
			if seen[s] {
				c.add(idp, fieldInvalid, detailPlanItemID)
			}
			seen[s] = true
			item.ID = s
		}
	}
	if text, tp, ok := c.member(im, ip, "text", true); ok {
		item.Text, _ = c.displayText(text, tp)
	}
	if st, sp, ok := c.member(im, ip, memberStatus, true); ok {
		item.Status = c.enum(st, sp, planStatusValues)
	}
	c.closed(im, ip, "id", "text", memberStatus)
	return item, true
}

// stepInput checks the value of an add on /steps/<id>: a RunStep without ordinal (09 4.7). The
// server assigns the ordinal, so a sent ordinal is a forbidden path, not an unknown member.
func (c *collector) stepInput(v any, ptr string, pathID ids.ID) StepInput {
	m, ok := c.object(v, ptr)
	if !ok {
		return StepInput{}
	}
	var in StepInput
	c.stepID(m, ptr, pathID)
	if kind, p, ok := c.member(m, ptr, "kind", true); ok {
		in.Kind = c.enum(kind, p, stepKindValues)
	}
	if name, p, ok := c.member(m, ptr, "name", true); ok {
		in.Name, _ = c.displayText(name, p)
	}
	if title, p, ok := c.member(m, ptr, memberTitle, false); ok {
		in.Title, _ = c.displayText(title, p)
	}
	if status, p, ok := c.member(m, ptr, memberStatus, false); ok {
		in.Status = c.enum(status, p, stepStatusValues)
	}
	if detail, p, ok := c.member(m, ptr, memberDetail, false); ok {
		in.Detail = c.detail(detail, p)
	}
	in.ParentStepID = c.parentStepID(m, ptr)
	if started, p, ok := c.member(m, ptr, "started_at", false); ok {
		in.StartedAt = c.timestamp(started, p)
	}
	if finished, p, ok := c.member(m, ptr, "finished_at", false); ok {
		in.FinishedAt = c.timestamp(finished, p)
	}
	if _, p, ok := c.member(m, ptr, "ordinal", false); ok {
		c.add(p, fieldPathForbidden, detailOrdinal)
	}
	c.closed(m, ptr, "id", "kind", "name", memberTitle, memberStatus, memberDetail, "parent_step_id", "started_at", "finished_at", "ordinal")
	return in
}

// stepID checks the optional member id of a new step in m: when present it must equal the id in
// the path.
func (c *collector) stepID(m map[string]any, ptr string, pathID ids.ID) {
	id, p, ok := c.member(m, ptr, "id", false)
	if !ok {
		return
	}
	if s, valid := c.str(id, p); valid {
		if parsed, err := ids.Parse(s); err != nil || parsed != pathID {
			c.add(p, fieldInvalid, detailStepID)
		}
	}
}

// parentStepID checks the optional member parent_step_id of a new step in m and returns it, the
// zero id when it is absent or not a string.
func (c *collector) parentStepID(m map[string]any, ptr string) ids.ID {
	parent, p, ok := c.member(m, ptr, "parent_step_id", false)
	if !ok {
		return ids.ID{}
	}
	s, valid := c.str(parent, p)
	if !valid {
		return ids.ID{}
	}
	id, err := ids.Parse(s)
	if err != nil {
		c.add(p, fieldInvalidFormat, detailUUID)
	}
	return id
}

// encodeCanonical encodes v the way the run document stores and sends values: encoding/json
// with members in sorted order, json.Number literals kept, no HTML escaping and no trailing
// newline. v holds only values decodeJSON returns or plain Go values, which always encode, so
// a failure is a programming error.
func encodeCanonical(v any) json.RawMessage {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		panic("runs: encoding a checked JSON value failed: " + err.Error())
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n"))
}

// canonicalOf returns the canonical encoding of a stored JSON value, or nil when it is not
// valid JSON; equal values have equal canonical encodings, whatever order the store kept.
func canonicalOf(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	v, err := decodeJSON(raw)
	if err != nil {
		return nil
	}
	return encodeCanonical(v)
}
