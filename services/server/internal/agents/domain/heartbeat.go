// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The field error codes of the heartbeat decoder: the vocabulary of the chain of M1-18
// (platform/httpserver), so both transports name a violation alike (brief M2-20 note 3).
const (
	codeRequired      = "required"
	codeUnknownMember = "unknown_member"
	codeInvalidType   = "invalid_type"
	codeInvalidFormat = "invalid_format"
	codeInvalidEnum   = "invalid_enum"
	codeTooSmall      = "too_small"
	codeTooLarge      = "too_large"
	codePattern       = "pattern_mismatch"
	codeDuplicate     = "duplicate"
)

// readinessReasonPattern and readinessReasonMax are the form of ProjectReadiness.reason, a code of
// 09 section 18.12 (M4-43): lower case letters, digits and underscores.
var readinessReasonPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_]*$`)

const readinessReasonMax = 64

// The forms of agent_heartbeat.v1 (docs/DECISIONS.md D-M2-20-2): names, versions and codes. None
// of them can carry a path separator, whitespace, a control or bidi character or "=", so most key
// formats and every path are unrepresentable (09 section 4.8, I5). Go's regexp runs in linear
// time (SEC-IN-6); lengths are bounded by http.max_json_body_bytes.
var (
	namePattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	versionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+_-]*$`)
	codePatternRE  = regexp.MustCompile(`^[a-z0-9][a-z0-9_.:-]*$`)
)

// The key types of billing_evidence.v1 (schema BillingEvidence).
const (
	// KeyTypeUnknown is the key type of a runtime that cannot determine its mode; it counts as
	// unequal to every configured mode (10 section 7).
	KeyTypeUnknown = "unknown"
)

// keyTypes are the values of BillingEvidence.key_type.
var keyTypes = []string{"api_key", "oauth", "cloud", "none", KeyTypeUnknown}

// followupModes are the values of details.capabilities.followups (09 section 2.2).
var followupModes = []string{"steer", "queue"}

// Evidence is billing_evidence.v1: how the runtime proves its reported mode. It never holds a key
// value or prefix; Provider and Method are codes.
type Evidence struct {
	Provider string `json:"provider,omitempty"`
	KeyType  string `json:"key_type"`
	Method   string `json:"method,omitempty"`
}

// Capabilities are details.capabilities, the features the runtime reports (09 section 2.2); nil
// members are not reported.
type Capabilities struct {
	Followups  string `json:"followups,omitempty"`
	Components *bool  `json:"components,omitempty"`
	VoiceInput *bool  `json:"voice_input,omitempty"`
	Title      *bool  `json:"title,omitempty"`
	FilesOut   *bool  `json:"files_out,omitempty"`
}

// Adapter is details.adapter: the adapter's name and optional version (18 section 9 point 5).
type Adapter struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// Runtime is details.runtime: the name of the agent runtime.
type Runtime struct {
	Name string `json:"name"`
}

// Details is the closed object details of agent_heartbeat.v1 (D-M2-20-2). Nil slices and
// pointers and a zero ProtocolVersion are not reported; an empty non-nil slice is a reported
// empty list.
type Details struct {
	Capabilities    *Capabilities
	ActiveRuns      []ids.ID
	DegradedReasons []string
	Adapter         *Adapter
	Runtime         *Runtime
	ProtocolVersion int
}

// RateLimitState is rate_limit.state of agent_heartbeat.v1 (schema AgentRateLimitState): the
// rate-limit state of the runtime at its provider as the adapter sees it.
type RateLimitState string

// The values of RateLimitState.
const (
	RateLimitOK        RateLimitState = "ok"
	RateLimitNearLimit RateLimitState = "near_limit"
	RateLimitLimited   RateLimitState = "limited"
	RateLimitUnknown   RateLimitState = "unknown"
)

// RateLimit is rate_limit of agent_heartbeat.v1 (09 section 4.8, M6-14): only the state and the
// time it resets, never a quota, key or subscription name (I5), so nothing in it names a runtime
// or an account.
type RateLimit struct {
	State RateLimitState
	// ResetsAt is the reset time in UTC, truncated to milliseconds; zero when not reported.
	ResetsAt time.Time
}

// Readiness is one entry of project_readiness (schema ProjectReadiness, M2-03).
type Readiness struct {
	ChannelID ids.ID
	Ready     bool
	// Reason is the code of 09 section 18.12 sent with Ready false; "" when absent (M4-43).
	Reason string
}

// Report is one decoded heartbeat (agent_heartbeat.v1). Every heartbeat is the whole reported
// state (D-M2-20-3): an empty value is "not reported".
type Report struct {
	// State is online or degraded.
	State RuntimeState
	// RuntimeVersion is the version of the runtime, the column reported_runtime_version.
	RuntimeVersion string
	// BillingMode is reported_billing_mode; "" when the adapter follows the target per run.
	BillingMode BillingMode
	// BillingModesSupported are the modes the adapter can prove; nil when not reported.
	BillingModesSupported []BillingMode
	// Evidence is billing_evidence; nil when not reported.
	Evidence *Evidence
	// ToolClass is reported_tool_class; "" when not reported.
	ToolClass ToolClass
	// RateLimit is rate_limit; nil when not reported (an earlier stored one is cleared, D-M6-14-5).
	RateLimit *RateLimit
	// Details is the object details.
	Details Details
	// Readiness is project_readiness; ReadinessGiven tells an absent member from an empty list.
	Readiness      []Readiness
	ReadinessGiven bool
}

// Reported returns the columns and details of r as the stored form Reported; LastHeartbeatAt is
// left zero.
func (r Report) Reported() Reported {
	out := Reported{State: r.State, BillingMode: r.BillingMode, ToolClass: r.ToolClass, RuntimeVersion: r.RuntimeVersion,
		BillingModesSupported: slices.Clone(r.BillingModesSupported), Evidence: cloneEvidence(r.Evidence), RateLimit: cloneRateLimit(r.RateLimit)}
	if c := r.Details.Capabilities; c != nil {
		out.Followups = c.Followups
		out.VoiceInput = c.VoiceInput != nil && *c.VoiceInput
	}
	return out
}

// cloneEvidence copies e.
func cloneEvidence(e *Evidence) *Evidence {
	if e == nil {
		return nil
	}
	c := *e
	return &c
}

// cloneRateLimit copies l.
func cloneRateLimit(l *RateLimit) *RateLimit {
	if l == nil {
		return nil
	}
	c := *l
	return &c
}

// storedRateLimit is the stored form of rate_limit: exactly {state, resets_at?}, resets_at in the
// form of the schema Timestamp (UTC, milliseconds, suffix Z).
type storedRateLimit struct {
	State    string `json:"state"`
	ResetsAt string `json:"resets_at,omitempty"`
}

// report returns the stored rate limit; nil for none or a state the server does not know, because
// it shows only what it can name.
func (l *storedRateLimit) report() *RateLimit {
	if l == nil || !slices.Contains(rateLimitStates, l.State) {
		return nil
	}
	out := &RateLimit{State: RateLimitState(l.State)}
	if at, err := time.Parse(time.RFC3339Nano, l.ResetsAt); err == nil {
		out.ResetsAt = at.UTC()
	}
	return out
}

// timestampLayout is the form of the schema Timestamp.
const timestampLayout = "2006-01-02T15:04:05.000Z"

// storedDetails is the form of agents.reported_details (brief M2-20 note 6): exactly these
// members, absent ones omitted, never null. rate_limit is the last reported one (M6-14); project
// readiness is stored only in project_bindings.ready (M2-03).
type storedDetails struct {
	Capabilities          *Capabilities    `json:"capabilities,omitempty"`
	BillingModesSupported []string         `json:"billing_modes_supported,omitzero"`
	BillingEvidence       *Evidence        `json:"billing_evidence,omitempty"`
	ActiveRuns            []string         `json:"active_runs,omitzero"`
	DegradedReasons       []string         `json:"degraded_reasons,omitzero"`
	Adapter               *Adapter         `json:"adapter,omitempty"`
	Runtime               *Runtime         `json:"runtime,omitempty"`
	ProtocolVersion       int              `json:"protocol_version,omitempty"`
	RateLimit             *storedRateLimit `json:"rate_limit,omitempty"`
}

// StoredDetails returns the JSON object of agents.reported_details for r. active_runs holds the
// run ids as strings, the shape runs.stall_check of M2-09 reads.
func (r Report) StoredDetails() []byte {
	d := storedDetails{Capabilities: r.Details.Capabilities, BillingEvidence: r.Evidence, Adapter: r.Details.Adapter,
		Runtime: r.Details.Runtime, ProtocolVersion: r.Details.ProtocolVersion, DegradedReasons: r.Details.DegradedReasons}
	if r.BillingModesSupported != nil {
		d.BillingModesSupported = make([]string, 0, len(r.BillingModesSupported))
		for _, m := range r.BillingModesSupported {
			d.BillingModesSupported = append(d.BillingModesSupported, string(m))
		}
	}
	if l := r.RateLimit; l != nil {
		d.RateLimit = &storedRateLimit{State: string(l.State)}
		if !l.ResetsAt.IsZero() {
			d.RateLimit.ResetsAt = l.ResetsAt.UTC().Format(timestampLayout)
		}
	}
	if r.Details.ActiveRuns != nil {
		d.ActiveRuns = make([]string, 0, len(r.Details.ActiveRuns))
		for _, id := range r.Details.ActiveRuns {
			d.ActiveRuns = append(d.ActiveRuns, id.String())
		}
	}
	b, err := json.Marshal(d)
	if err != nil {
		// Only strings, booleans and integers: Marshal cannot fail.
		return []byte(`{}`)
	}
	return b
}

// ReportFromStore rebuilds the report of a stored row: the columns state, version, mode and class
// and reported_details. Values the server does not know are left out, because it shows only what
// it can name. Errors: raw is no JSON object.
func ReportFromStore(state RuntimeState, version string, mode BillingMode, class ToolClass, raw []byte) (Report, error) {
	var d storedDetails
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &d); err != nil {
			return Report{}, err
		}
	}
	r := Report{State: state, RuntimeVersion: version, BillingMode: mode, ToolClass: class, Evidence: d.BillingEvidence,
		Details: Details{Capabilities: d.Capabilities, DegradedReasons: d.DegradedReasons, Adapter: d.Adapter, Runtime: d.Runtime,
			ProtocolVersion: d.ProtocolVersion}}
	r.RateLimit = d.RateLimit.report()
	for _, s := range d.BillingModesSupported {
		if m := BillingMode(s); m.Valid() {
			r.BillingModesSupported = append(r.BillingModesSupported, m)
		}
	}
	if d.ActiveRuns != nil {
		r.Details.ActiveRuns = []ids.ID{}
		for _, s := range d.ActiveRuns {
			if id, err := ids.Parse(s); err == nil {
				r.Details.ActiveRuns = append(r.Details.ActiveRuns, id)
			}
		}
	}
	return r, nil
}

// decoder collects the field errors of one heartbeat.
type decoder struct {
	errs []errs.FieldError
}

// fail records a field error at pointer with code; the detail never repeats the value.
func (d *decoder) fail(pointer, code string) {
	d.errs = append(d.errs, errs.FieldError{Pointer: pointer, Code: code, Detail: fieldDetail(code)})
}

// fieldDetail is the English detail of a field error code.
func fieldDetail(code string) string {
	switch code {
	case codeRequired:
		return "The member is required."
	case codeUnknownMember:
		return "The member is not part of the schema."
	case codeInvalidType:
		return "The value has the wrong type."
	case codeInvalidFormat:
		return "The value does not have the required format."
	case codeInvalidEnum:
		return "The value is not one of the allowed values."
	case codeTooSmall:
		return "The value is too small."
	case codeTooLarge:
		return "The value is too large."
	case codePattern:
		return "The value does not match the required pattern."
	case codeDuplicate:
		return "The value is repeated."
	case codeTooLong:
		return "The value is too long."
	case codeInvalid:
		return "The value is not allowed here."
	}
	return "The value is invalid."
}

// member is "pointer/key" with the escaping of RFC 6901.
func member(pointer, key string) string {
	return pointer + "/" + strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}

// index is "pointer/i".
func index(pointer string, i int) string { return pointer + "/" + strconv.Itoa(i) }

// object reads raw as a JSON object and reports every member outside known as unknown, in key
// order; ok is false when raw is no object.
func (d *decoder) object(pointer string, raw json.RawMessage, known ...string) (map[string]json.RawMessage, bool) {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 || t[0] != '{' {
		d.fail(pointer, codeInvalidType)
		return nil, false
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(t, &m); err != nil {
		d.fail(pointer, codeInvalidType)
		return nil, false
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if !slices.Contains(known, k) {
			d.fail(member(pointer, k), codeUnknownMember)
		}
	}
	return m, true
}

// array reads raw as a JSON array.
func (d *decoder) array(pointer string, raw json.RawMessage) ([]json.RawMessage, bool) {
	t := bytes.TrimSpace(raw)
	var a []json.RawMessage
	if len(t) == 0 || t[0] != '[' || json.Unmarshal(t, &a) != nil {
		d.fail(pointer, codeInvalidType)
		return nil, false
	}
	return a, true
}

// str reads raw as a JSON string.
func (d *decoder) str(pointer string, raw json.RawMessage) (string, bool) {
	t := bytes.TrimSpace(raw)
	var s string
	if len(t) == 0 || t[0] != '"' || json.Unmarshal(t, &s) != nil {
		d.fail(pointer, codeInvalidType)
		return "", false
	}
	return s, true
}

// boolean reads raw as a JSON boolean.
func (d *decoder) boolean(pointer string, raw json.RawMessage) (bool, bool) {
	switch string(bytes.TrimSpace(raw)) {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	d.fail(pointer, codeInvalidType)
	return false, false
}

// optBool reads an optional boolean member into a pointer.
func (d *decoder) optBool(pointer string, raw json.RawMessage, present bool) *bool {
	if !present {
		return nil
	}
	b, ok := d.boolean(pointer, raw)
	if !ok {
		return nil
	}
	return &b
}

// enum reads raw as a string out of values.
func (d *decoder) enum(pointer string, raw json.RawMessage, values []string) (string, bool) {
	s, ok := d.str(pointer, raw)
	if !ok {
		return "", false
	}
	if !slices.Contains(values, s) {
		d.fail(pointer, codeInvalidEnum)
		return "", false
	}
	return s, true
}

// form reads raw as a string of the pattern re.
func (d *decoder) form(pointer string, raw json.RawMessage, re *regexp.Regexp) (string, bool) {
	s, ok := d.str(pointer, raw)
	if !ok {
		return "", false
	}
	if !re.MatchString(s) {
		d.fail(pointer, codePattern)
		return "", false
	}
	return s, true
}

// The top-level members of the schema Heartbeat.
var heartbeatMembers = []string{"state", "runtime_version", "reported_billing_mode", "billing_modes_supported", "billing_evidence",
	"reported_tool_class", "rate_limit", "details", "project_readiness"}

// billingModes and toolClasses are the enumerations of BillingMode and ToolClass.
var (
	billingModes = []string{string(Personal), string(Organization)}
	toolClasses  = []string{string(ToolChat), string(ToolScoped), string(ToolHost)}
	reportStates = []string{string(RuntimeOnline), string(RuntimeDegraded)}
	// rateLimitStates are the values of AgentRateLimitState.state.
	rateLimitStates = []string{string(RateLimitOK), string(RateLimitNearLimit), string(RateLimitLimited), string(RateLimitUnknown)}
)

// DecodeHeartbeat decodes and validates the JSON object raw of schema Heartbeat (the REST body of
// sendHeartbeat or the member agent of the gateway op heartbeat) with every rule of brief M2-20
// note 3: the closed top-level members and details keys, the forms of names, versions and codes,
// no duplicates, protocol_version at least 1 and rate_limit of the schema AgentRateLimitState. It checks the
// rules the openapi schema also expresses itself (SEC-IN-1), so both transports refuse the same
// bodies. The field errors are in document order of the rules, each with its JSON pointer
// relative to raw; nil means the report is valid. It never panics on arbitrary input.
//
// Example:
//
//	r, fes := domain.DecodeHeartbeat([]byte(`{"state":"online","reported_billing_mode":"organization"}`))
//	// fes == nil, r.BillingMode == domain.Organization
func DecodeHeartbeat(raw []byte) (Report, []errs.FieldError) {
	d := &decoder{}
	m, ok := d.object("", raw, heartbeatMembers...)
	if !ok {
		return Report{}, d.errs
	}
	var r Report
	if v, has := m["state"]; has {
		s, _ := d.enum("/state", v, reportStates)
		r.State = RuntimeState(s)
	} else {
		d.fail("/state", codeRequired)
	}
	if v, has := m["runtime_version"]; has {
		r.RuntimeVersion, _ = d.form("/runtime_version", v, versionPattern)
	}
	if v, has := m["reported_billing_mode"]; has {
		s, _ := d.enum("/reported_billing_mode", v, billingModes)
		r.BillingMode = BillingMode(s)
	}
	if v, has := m["billing_modes_supported"]; has {
		r.BillingModesSupported = d.modes(v)
	}
	if v, has := m["billing_evidence"]; has {
		r.Evidence = d.evidence(v)
	}
	if v, has := m["reported_tool_class"]; has {
		s, _ := d.enum("/reported_tool_class", v, toolClasses)
		r.ToolClass = ToolClass(s)
	}
	if v, has := m["rate_limit"]; has {
		r.RateLimit = d.rateLimit(v)
	}
	if v, has := m["details"]; has {
		r.Details = d.details(v)
	}
	if v, has := m["project_readiness"]; has {
		r.Readiness, r.ReadinessGiven = d.readiness(v), true
	}
	if len(d.errs) > 0 {
		return Report{}, d.errs
	}
	return r, nil
}

// modes reads billing_modes_supported: distinct values of BillingMode.
func (d *decoder) modes(raw json.RawMessage) []BillingMode {
	const p = "/billing_modes_supported"
	items, ok := d.array(p, raw)
	if !ok {
		return nil
	}
	out := make([]BillingMode, 0, len(items))
	for i, it := range items {
		s, ok := d.enum(index(p, i), it, billingModes)
		if !ok {
			continue
		}
		if slices.Contains(out, BillingMode(s)) {
			d.fail(index(p, i), codeDuplicate)
			continue
		}
		out = append(out, BillingMode(s))
	}
	return out
}

// rateLimit reads rate_limit (schema AgentRateLimitState, M6-14 design note 4): state from the
// closed list, resets_at optional and an RFC 3339 time, kept in UTC to the millisecond. resets_at
// has no window bound: the server only shows it, and a provider window lasts from a minute to a
// billing month (D-M6-14-5).
func (d *decoder) rateLimit(raw json.RawMessage) *RateLimit {
	const p = "/rate_limit"
	m, ok := d.object(p, raw, "state", "resets_at")
	if !ok {
		return nil
	}
	var l RateLimit
	if v, has := m["state"]; has {
		s, _ := d.enum(p+"/state", v, rateLimitStates)
		l.State = RateLimitState(s)
	} else {
		d.fail(p+"/state", codeRequired)
	}
	if v, has := m["resets_at"]; has {
		if s, ok := d.str(p+"/resets_at", v); ok {
			at, err := time.Parse(time.RFC3339Nano, s)
			if err != nil {
				d.fail(p+"/resets_at", codeInvalidFormat)
			}
			l.ResetsAt = at.UTC().Truncate(time.Millisecond)
		}
	}
	return &l
}

// evidence reads billing_evidence (schema BillingEvidence): key_type required, provider and
// method codes.
func (d *decoder) evidence(raw json.RawMessage) *Evidence {
	const p = "/billing_evidence"
	m, ok := d.object(p, raw, "provider", "key_type", "method")
	if !ok {
		return nil
	}
	var e Evidence
	if v, has := m["key_type"]; has {
		e.KeyType, _ = d.enum(p+"/key_type", v, keyTypes)
	} else {
		d.fail(p+"/key_type", codeRequired)
	}
	if v, has := m["provider"]; has {
		e.Provider, _ = d.form(p+"/provider", v, codePatternRE)
	}
	if v, has := m["method"]; has {
		e.Method, _ = d.form(p+"/method", v, codePatternRE)
	}
	return &e
}

// The keys of details and of its objects (D-M2-20-2).
var (
	detailsKeys      = []string{"capabilities", "active_runs", "degraded_reasons", "adapter", "runtime", "protocol_version"}
	capabilitiesKeys = []string{"followups", "components", "voice_input", "title", "files_out"}
)

// details reads the closed object details.
func (d *decoder) details(raw json.RawMessage) Details {
	const p = "/details"
	var out Details
	m, ok := d.object(p, raw, detailsKeys...)
	if !ok {
		return out
	}
	if v, has := m["capabilities"]; has {
		out.Capabilities = d.capabilities(v)
	}
	if v, has := m["active_runs"]; has {
		out.ActiveRuns = d.runs(v)
	}
	if v, has := m["degraded_reasons"]; has {
		out.DegradedReasons = d.codes(p+"/degraded_reasons", v)
	}
	if v, has := m["adapter"]; has {
		am, ok := d.object(p+"/adapter", v, "name", "version")
		if ok {
			a := Adapter{Name: d.requiredName(p+"/adapter/name", am)}
			if vv, has := am["version"]; has {
				a.Version, _ = d.form(p+"/adapter/version", vv, versionPattern)
			}
			out.Adapter = &a
		}
	}
	if v, has := m["runtime"]; has {
		rm, ok := d.object(p+"/runtime", v, "name")
		if ok {
			out.Runtime = &Runtime{Name: d.requiredName(p+"/runtime/name", rm)}
		}
	}
	if v, has := m["protocol_version"]; has {
		out.ProtocolVersion = d.protocolVersion(p+"/protocol_version", v)
	}
	return out
}

// requiredName reads the required member name of m in the name form.
func (d *decoder) requiredName(pointer string, m map[string]json.RawMessage) string {
	v, has := m["name"]
	if !has {
		d.fail(pointer, codeRequired)
		return ""
	}
	s, _ := d.form(pointer, v, namePattern)
	return s
}

// capabilities reads details.capabilities (09 section 2.2).
func (d *decoder) capabilities(raw json.RawMessage) *Capabilities {
	const p = "/details/capabilities"
	m, ok := d.object(p, raw, capabilitiesKeys...)
	if !ok {
		return nil
	}
	var c Capabilities
	if v, has := m["followups"]; has {
		c.Followups, _ = d.enum(p+"/followups", v, followupModes)
	}
	get := func(k string) (json.RawMessage, bool) { v, has := m[k]; return v, has }
	v, has := get("components")
	c.Components = d.optBool(p+"/components", v, has)
	v, has = get("voice_input")
	c.VoiceInput = d.optBool(p+"/voice_input", v, has)
	v, has = get("title")
	c.Title = d.optBool(p+"/title", v, has)
	v, has = get("files_out")
	c.FilesOut = d.optBool(p+"/files_out", v, has)
	return &c
}

// runs reads details.active_runs: distinct run ids after R-DM-01.
func (d *decoder) runs(raw json.RawMessage) []ids.ID {
	const p = "/details/active_runs"
	items, ok := d.array(p, raw)
	if !ok {
		return nil
	}
	out := make([]ids.ID, 0, len(items))
	for i, it := range items {
		s, ok := d.str(index(p, i), it)
		if !ok {
			continue
		}
		id, err := ids.Parse(s)
		if err != nil {
			d.fail(index(p, i), codeInvalidFormat)
			continue
		}
		if slices.Contains(out, id) {
			d.fail(index(p, i), codeDuplicate)
			continue
		}
		out = append(out, id)
	}
	return out
}

// codes reads a list of distinct codes.
func (d *decoder) codes(p string, raw json.RawMessage) []string {
	items, ok := d.array(p, raw)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for i, it := range items {
		s, ok := d.form(index(p, i), it, codePatternRE)
		if !ok {
			continue
		}
		if slices.Contains(out, s) {
			d.fail(index(p, i), codeDuplicate)
			continue
		}
		out = append(out, s)
	}
	return out
}

// protocolVersion reads an integer of at least 1.
func (d *decoder) protocolVersion(p string, raw json.RawMessage) int {
	t := bytes.TrimSpace(raw)
	n, err := strconv.ParseInt(string(t), 10, 32)
	switch {
	case errors.Is(err, strconv.ErrRange):
		d.fail(p, codeTooLarge)
		return 0
	case err != nil:
		d.fail(p, codeInvalidType)
		return 0
	case n < 1:
		d.fail(p, codeTooSmall)
		return 0
	}
	return int(n)
}

// readiness reads project_readiness; a channel listed twice is one error at /project_readiness,
// as M2-03 expects.
func (d *decoder) readiness(raw json.RawMessage) []Readiness {
	const p = "/project_readiness"
	items, ok := d.array(p, raw)
	if !ok {
		return nil
	}
	out := make([]Readiness, 0, len(items))
	dup := false
	for i, it := range items {
		r, ok := d.readinessItem(index(p, i), it)
		if !ok {
			continue
		}
		if !r.ChannelID.IsZero() && slices.ContainsFunc(out, func(o Readiness) bool { return o.ChannelID == r.ChannelID }) {
			dup = true
		}
		out = append(out, r)
	}
	if dup {
		d.fail(p, codeDuplicate)
	}
	return out
}

// readinessItem reads one entry of project_readiness (schema ProjectReadiness); ok is false when
// it is no object.
func (d *decoder) readinessItem(ip string, raw json.RawMessage) (Readiness, bool) {
	m, ok := d.object(ip, raw, "channel_id", "ready", "reason")
	if !ok {
		return Readiness{}, false
	}
	var r Readiness
	if v, has := m["channel_id"]; has {
		if s, ok := d.str(ip+"/channel_id", v); ok {
			id, err := ids.Parse(s)
			if err != nil {
				d.fail(ip+"/channel_id", codeInvalidFormat)
			}
			r.ChannelID = id
		}
	} else {
		d.fail(ip+"/channel_id", codeRequired)
	}
	if v, has := m["ready"]; has {
		r.Ready, _ = d.boolean(ip+"/ready", v)
	} else {
		d.fail(ip+"/ready", codeRequired)
	}
	if v, has := m["reason"]; has {
		r.Reason = d.readinessReason(ip+"/reason", v, r.Ready)
	}
	return r, true
}

// readinessReason reads the reason of one readiness entry: a code of at most readinessReasonMax
// characters in readinessReasonPattern, allowed only with ready false (09 section 4.8, M4-43).
func (d *decoder) readinessReason(pointer string, raw json.RawMessage, ready bool) string {
	s, ok := d.str(pointer, raw)
	switch {
	case !ok:
		return ""
	case len(s) > readinessReasonMax:
		d.fail(pointer, codeTooLong)
	case !readinessReasonPattern.MatchString(s):
		d.fail(pointer, codePattern)
	case ready:
		d.fail(pointer, codeInvalid)
	default:
		return s
	}
	return ""
}

// AuditDetails are the details of the audit entry agent.heartbeat of a change of the reported
// mode or class (D-M2-20-3): the old and new value of each changed column, each only when it
// exists, and key_type and method of the evidence when reported.
type AuditDetails struct {
	BillingModeOld, BillingModeNew BillingMode
	ToolClassOld, ToolClassNew     ToolClass
	KeyType, Method                string
}

// The keys of AuditDetails in the audit entry.
const (
	AuditBillingModeOld = "billing_mode_old"
	AuditBillingModeNew = "billing_mode_new"
	AuditToolClassOld   = "tool_class_old"
	AuditToolClassNew   = "tool_class_new"
	AuditKeyType        = "key_type"
	AuditMethod         = "method"
)

// Map returns the present details by their audit keys; every value is a code.
func (a AuditDetails) Map() map[string]string {
	out := map[string]string{}
	for k, v := range map[string]string{AuditBillingModeOld: string(a.BillingModeOld), AuditBillingModeNew: string(a.BillingModeNew),
		AuditToolClassOld: string(a.ToolClassOld), AuditToolClassNew: string(a.ToolClassNew), AuditKeyType: a.KeyType, AuditMethod: a.Method} {
		if v != "" {
			out[k] = v
		}
	}
	return out
}

// ReportedChange compares the stored reported mode and class with the next report (D-M2-20-3):
// changed is true when either differs, also from or to "not reported" and for the first report
// ever. The details name only the changed columns and take key_type and method from
// next.Evidence.
func ReportedChange(stored, next Reported) (AuditDetails, bool) {
	var a AuditDetails
	modeChanged, classChanged := stored.BillingMode != next.BillingMode, stored.ToolClass != next.ToolClass
	if !modeChanged && !classChanged {
		return AuditDetails{}, false
	}
	if modeChanged {
		a.BillingModeOld, a.BillingModeNew = stored.BillingMode, next.BillingMode
	}
	if classChanged {
		a.ToolClassOld, a.ToolClassNew = stored.ToolClass, next.ToolClass
	}
	if next.Evidence != nil {
		a.KeyType, a.Method = next.Evidence.KeyType, next.Evidence.Method
	}
	return a, true
}
