// SPDX-License-Identifier: Apache-2.0

package suite

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// Pointer returns the value at the JSON pointer p (RFC 6901) of doc.
func Pointer(doc any, p string) (any, bool) {
	if p == "" || p == "/" {
		return doc, true
	}
	cur := doc
	for _, part := range strings.Split(strings.TrimPrefix(p, "/"), "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch x := cur.(type) {
		case map[string]any:
			v, ok := x[part]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(x) {
				return nil, false
			}
			cur = x[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

// normalize returns v as JSON would decode it (numbers as float64, maps as map[string]any), so a
// literal of a scenario compares with a value of an answer.
func normalize(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if json.Unmarshal(b, &out) != nil {
		return v
	}
	return out
}

// show renders a scalar expectation or observation for a mismatch; maps and lists by size only.
func show(v any) string {
	switch x := v.(type) {
	case map[string]any:
		return fmt.Sprintf("{%d members}", len(x))
	case []any:
		return fmt.Sprintf("[%d entries]", len(x))
	case nil:
		return "absent"
	}
	return fmt.Sprintf("%v", v)
}

// problem is the part of a Problem body the expectations compare.
type problem struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
	Errors []struct {
		Code    string `json:"code"`
		Pointer string `json:"pointer"`
	} `json:"errors"`
}

// CompareCall compares the answer of a call with exp: status, code, reason, errors and the
// fields at JSON pointers; expand resolves the references of literal fields. The mismatches name
// codes and the values at the named pointers only, never the body.
func CompareCall(exp Expect, obs observedCall, expand func(any) any) []string {
	var out []string
	var p problem
	_ = json.Unmarshal(obs.Body, &p)
	if exp.Status != nil && obs.Status != *exp.Status {
		out = append(out, fmt.Sprintf("status %d (code %q, reason %q), want %d", obs.Status, p.Code, p.Reason, *exp.Status))
	}
	if exp.Code != "" && p.Code != exp.Code {
		out = append(out, fmt.Sprintf("code %q, want %q", p.Code, exp.Code))
	}
	if exp.Reason != "" && p.Reason != exp.Reason {
		out = append(out, fmt.Sprintf("reason %q, want %q", p.Reason, exp.Reason))
	}
	for _, e := range exp.Errors {
		found := false
		var seen []string
		for _, g := range p.Errors {
			seen = append(seen, g.Code+"@"+g.Pointer)
			if g.Code == e.Code && (e.Pointer == "" || g.Pointer == e.Pointer) {
				found = true
			}
		}
		if !found {
			out = append(out, fmt.Sprintf("errors %v lack %s@%s", seen, e.Code, e.Pointer))
		}
	}
	if len(exp.Fields) > 0 || len(exp.Absent) > 0 || len(exp.Lengths) > 0 {
		var doc any
		if err := json.Unmarshal(obs.Body, &doc); err != nil {
			return append(out, "the answer is no JSON")
		}
		for _, ptr := range sortedKeys(exp.Fields) {
			want := normalize(expand(exp.Fields[ptr]))
			got, ok := Pointer(doc, ptr)
			if !ok {
				out = append(out, fmt.Sprintf("%s absent, want %s", ptr, show(want)))
				continue
			}
			if !reflect.DeepEqual(normalize(got), want) {
				out = append(out, fmt.Sprintf("%s is %s, want %s", ptr, show(got), show(want)))
			}
		}
		for _, ptr := range exp.Absent {
			if _, ok := Pointer(doc, ptr); ok {
				out = append(out, fmt.Sprintf("%s present, want absent", ptr))
			}
		}
		for _, ptr := range sortedKeys(exp.Lengths) {
			got, _ := Pointer(doc, ptr)
			list, ok := got.([]any)
			if !ok || len(list) != exp.Lengths[ptr] {
				out = append(out, fmt.Sprintf("%s has %s, want %d entries", ptr, show(got), exp.Lengths[ptr]))
			}
		}
	}
	return out
}

// sortedKeys returns the keys of m in order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// CompareClose compares the close of a stream with exp.
func CompareClose(exp CloseExpect, code int, reason string) []string {
	var out []string
	if code != exp.Code {
		out = append(out, fmt.Sprintf("close code %d (reason %q), want %d", code, reason, exp.Code))
	} else if exp.Reason != "" && reason != exp.Reason {
		out = append(out, fmt.Sprintf("close reason %q, want %q", reason, exp.Reason))
	}
	return out
}

// CompareReplay compares the replay frames of an attach: want true asks for exactly one
// replay_begin and one replay_end, false for none.
func CompareReplay(want bool, begin, end int) []string {
	if want && (begin != 1 || end != 1) {
		return []string{fmt.Sprintf("replay_begin %d and replay_end %d, want one each", begin, end)}
	}
	if !want && (begin != 0 || end != 0) {
		return []string{fmt.Sprintf("replay_begin %d and replay_end %d, want none", begin, end)}
	}
	return nil
}

// ObservedSession is the part of a TerminalSession a session check compares.
type ObservedSession struct {
	State       string `json:"state"`
	CloseReason string `json:"close_reason"`
	CloseDetail string `json:"close_detail"`
	ExitCode    *int   `json:"exit_code"`
}

// CompareSession compares a session with exp.
func CompareSession(exp Expect, obs ObservedSession) []string {
	var out []string
	if exp.State != "" && obs.State != exp.State {
		out = append(out, fmt.Sprintf("state %q, want %q", obs.State, exp.State))
	}
	if exp.CloseReason != "" && obs.CloseReason != exp.CloseReason {
		out = append(out, fmt.Sprintf("close_reason %q, want %q", obs.CloseReason, exp.CloseReason))
	}
	if exp.CloseDetail != "" && obs.CloseDetail != exp.CloseDetail {
		out = append(out, fmt.Sprintf("close_detail %q, want %q", obs.CloseDetail, exp.CloseDetail))
	}
	if exp.ExitCode != nil && (obs.ExitCode == nil || *obs.ExitCode != *exp.ExitCode) {
		out = append(out, fmt.Sprintf("exit_code %s, want %d", intOrAbsent(obs.ExitCode), *exp.ExitCode))
	}
	return out
}

// intOrAbsent renders an optional number.
func intOrAbsent(p *int) string {
	if p == nil {
		return "absent"
	}
	return strconv.Itoa(*p)
}

// ObservedDoctor is one entry of Host.doctor.
type ObservedDoctor struct {
	Check       string `json:"check"`
	OK          bool   `json:"ok"`
	Code        string `json:"code"`
	SessionUser string `json:"session_user"`
}

// ObservedHost is the part of a Host (and its rows) a host check compares.
type ObservedHost struct {
	State          string           `json:"state"`
	Connected      bool             `json:"connected"`
	KeyFingerprint string           `json:"key_fingerprint"`
	Doctor         []ObservedDoctor `json:"doctor"`
	Reported       []struct {
		OSUser   string   `json:"os_user"`
		Workdirs []string `json:"workdirs"`
	} `json:"reported_session_users"`
	Profiles []struct {
		Name string `json:"name"`
	} `json:"profiles"`
	SessionUsers []struct {
		UserID string `json:"user_id"`
		OSUser string `json:"os_user"`
	} `json:"session_users"`
	// DisabledReason is hosts.disabled_reason (read with SQL).
	DisabledReason string `json:"-"`
	// Unit is the state of bothy-host.service (systemctl show), when the check reads it.
	Unit *ObservedUnit `json:"-"`
}

// ObservedUnit is the state of bothy-host.service.
type ObservedUnit struct {
	NRestarts, ExecMainStatus int
	Active                    bool
}

// matchDoctor reports whether entry o matches the expected entry e.
func matchDoctor(e DoctorEntry, o ObservedDoctor) bool {
	if e.Check != o.Check {
		return false
	}
	if e.OK != nil && *e.OK != o.OK {
		return false
	}
	if e.Code != "" && e.Code != o.Code {
		return false
	}
	if len(e.CodeIn) > 0 && !slices.Contains(e.CodeIn, o.Code) {
		return false
	}
	if e.SessionUser != nil && *e.SessionUser != o.SessionUser {
		return false
	}
	return true
}

// doctorSummary renders the entries by check, ok, code and session user.
func doctorSummary(d []ObservedDoctor) string {
	parts := make([]string, 0, len(d))
	for _, e := range d {
		s := fmt.Sprintf("%s/%v", e.Check, e.OK)
		if e.Code != "" {
			s += "/" + e.Code
		}
		if e.SessionUser != "" {
			s += "/" + e.SessionUser
		}
		parts = append(parts, s)
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// CompareHost compares a host with exp; people maps the person names of the assignments to their
// IDs.
func CompareHost(exp Expect, obs ObservedHost, people map[string]string) []string {
	var out []string
	if exp.State != "" && obs.State != exp.State {
		out = append(out, fmt.Sprintf("state %q, want %q", obs.State, exp.State))
	}
	if exp.Connected != nil && obs.Connected != *exp.Connected {
		out = append(out, fmt.Sprintf("connected %v, want %v", obs.Connected, *exp.Connected))
	}
	if exp.NoKeys != nil && (obs.KeyFingerprint == "") != *exp.NoKeys {
		out = append(out, fmt.Sprintf("key fingerprint present %v, want no keys %v", obs.KeyFingerprint != "", *exp.NoKeys))
	}
	for _, e := range exp.Doctor {
		if !slices.ContainsFunc(obs.Doctor, func(o ObservedDoctor) bool { return matchDoctor(e, o) }) {
			out = append(out, fmt.Sprintf("doctor %s lacks %s", doctorSummary(obs.Doctor), describeDoctor(e)))
		}
	}
	for _, e := range exp.DoctorLacks {
		if slices.ContainsFunc(obs.Doctor, func(o ObservedDoctor) bool { return matchDoctor(e, o) }) {
			out = append(out, fmt.Sprintf("doctor %s has %s", doctorSummary(obs.Doctor), describeDoctor(e)))
		}
	}
	if exp.ReportedSessionUsers != nil {
		var got []string
		for _, r := range obs.Reported {
			got = append(got, r.OSUser)
		}
		slices.Sort(got)
		want := slices.Clone(exp.ReportedSessionUsers)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			out = append(out, fmt.Sprintf("reported_session_users %v, want %v", got, want))
		}
	}
	for _, u := range sortedKeys(exp.Workdirs) {
		var got []string
		for _, r := range obs.Reported {
			if r.OSUser == u {
				got = append(got, r.Workdirs...)
			}
		}
		if !slices.Equal(got, exp.Workdirs[u]) {
			out = append(out, fmt.Sprintf("reported workdirs of %s %v, want %v", u, got, exp.Workdirs[u]))
		}
	}
	if exp.Profiles != nil || exp.ProfilesLack != nil {
		var got []string
		for _, p := range obs.Profiles {
			got = append(got, p.Name)
		}
		for _, p := range exp.Profiles {
			if !slices.Contains(got, p) {
				out = append(out, fmt.Sprintf("profiles %v lack %s", got, p))
			}
		}
		for _, p := range exp.ProfilesLack {
			if slices.Contains(got, p) {
				out = append(out, fmt.Sprintf("profiles %v hold %s", got, p))
			}
		}
	}
	if exp.SessionUsers != nil {
		got := map[string]string{}
		for _, s := range obs.SessionUsers {
			got[s.UserID] = s.OSUser
		}
		want := map[string]string{}
		for p, u := range exp.SessionUsers {
			want[people[p]] = u
		}
		if !reflect.DeepEqual(got, want) {
			out = append(out, fmt.Sprintf("%d session_users assignments differ from the %d expected", len(got), len(want)))
		}
	}
	if exp.DisabledReason != "" && obs.DisabledReason != exp.DisabledReason {
		out = append(out, fmt.Sprintf("disabled_reason %q, want %q", obs.DisabledReason, exp.DisabledReason))
	}
	if u := exp.Unit; u != nil {
		if obs.Unit == nil {
			return append(out, "the state of bothy-host.service was not read")
		}
		if u.NRestarts != nil && obs.Unit.NRestarts != *u.NRestarts {
			out = append(out, fmt.Sprintf("NRestarts %d, want %d", obs.Unit.NRestarts, *u.NRestarts))
		}
		if u.ExecMainStatus != nil && obs.Unit.ExecMainStatus != *u.ExecMainStatus {
			out = append(out, fmt.Sprintf("ExecMainStatus %d, want %d", obs.Unit.ExecMainStatus, *u.ExecMainStatus))
		}
		if u.Active != nil && obs.Unit.Active != *u.Active {
			out = append(out, fmt.Sprintf("unit active %v, want %v", obs.Unit.Active, *u.Active))
		}
	}
	return out
}

// describeDoctor renders an expected doctor entry.
func describeDoctor(e DoctorEntry) string {
	s := e.Check
	if e.OK != nil {
		s += fmt.Sprintf(" ok=%v", *e.OK)
	}
	if e.Code != "" {
		s += " code=" + e.Code
	}
	if len(e.CodeIn) > 0 {
		s += fmt.Sprintf(" code in %v", e.CodeIn)
	}
	if e.SessionUser != nil {
		s += " session_user=" + *e.SessionUser
	}
	return s
}

// ObservedAudit is one row of audit_events: action, decision, deny_reason and details.
type ObservedAudit struct {
	Action     string         `json:"action"`
	Decision   string         `json:"decision"`
	DenyReason string         `json:"deny_reason"`
	Details    map[string]any `json:"details"`
}

// matchAudit reports whether row o matches exp: action, decision, deny reason and every expected
// member of details.
func matchAudit(exp Expect, o ObservedAudit) bool {
	if exp.Action != "" && o.Action != exp.Action {
		return false
	}
	if exp.Decision != "" && o.Decision != exp.Decision {
		return false
	}
	if exp.DenyReason != "" && o.DenyReason != exp.DenyReason {
		return false
	}
	for k, v := range exp.Details {
		if !reflect.DeepEqual(normalize(o.Details[k]), normalize(v)) {
			return false
		}
	}
	return true
}

// CompareAudit compares the rows with exp: the number of matching rows equals count (default
// exactly one) or is at least at_least.
func CompareAudit(exp Expect, rows []ObservedAudit) []string {
	n := 0
	for _, r := range rows {
		if matchAudit(exp, r) {
			n++
		}
	}
	switch {
	case exp.AtLeast != nil:
		if n < *exp.AtLeast {
			return []string{fmt.Sprintf("%d audit entries %s, want at least %d (of %d rows)", n, describeAudit(exp), *exp.AtLeast, len(rows))}
		}
	default:
		want := 1
		if exp.Count != nil {
			want = *exp.Count
		}
		if n != want {
			return []string{fmt.Sprintf("%d audit entries %s, want %d (of %d rows: %s)", n, describeAudit(exp), want, len(rows), auditSummary(rows))}
		}
	}
	return nil
}

// describeAudit renders the expected entry by its codes.
func describeAudit(exp Expect) string {
	s := exp.Action + "/" + exp.Decision
	if exp.DenyReason != "" {
		s += "/" + exp.DenyReason
	}
	for _, k := range sortedKeys(exp.Details) {
		s += fmt.Sprintf(" %s=%v", k, exp.Details[k])
	}
	return s
}

// auditSummary renders rows by action, decision and deny reason.
func auditSummary(rows []ObservedAudit) string {
	parts := make([]string, 0, len(rows))
	for _, r := range rows {
		parts = append(parts, r.Action+"/"+r.Decision+"/"+r.DenyReason)
	}
	return strings.Join(parts, " ")
}

// ObservedProcesses are the counts of a processes check.
type ObservedProcesses struct {
	Shells, Markers, Processes int
	// Users are the users of the processes of the sessions (TestSessionRunsAsSessionUser).
	Users []string
}

// CompareProcesses compares the counts with exp.
func CompareProcesses(exp Expect, obs ObservedProcesses) []string {
	var out []string
	if exp.Shells != nil && obs.Shells != *exp.Shells {
		out = append(out, fmt.Sprintf("%d shells, want %d", obs.Shells, *exp.Shells))
	}
	if exp.Markers != nil && obs.Markers != *exp.Markers {
		out = append(out, fmt.Sprintf("%d marker processes, want %d", obs.Markers, *exp.Markers))
	}
	if exp.Processes != nil && obs.Processes != *exp.Processes {
		out = append(out, fmt.Sprintf("%d processes of the session user, want %d", obs.Processes, *exp.Processes))
	}
	if exp.Users != nil {
		got := slices.Compact(slices.Sorted(slices.Values(obs.Users)))
		want := slices.Compact(slices.Sorted(slices.Values(exp.Users)))
		if !slices.Equal(got, want) {
			out = append(out, fmt.Sprintf("process users %v, want %v", got, want))
		}
	}
	return out
}

// CompareMarkers compares the number of marker files (programs that ran) with exp.
func CompareMarkers(exp Expect, n int) []string {
	if exp.Markers != nil && n != *exp.Markers {
		return []string{fmt.Sprintf("%d marker programs ran, want %d", n, *exp.Markers)}
	}
	return nil
}

// CompareMetric compares the rise of a counter with exp.
func CompareMetric(exp Expect, rise float64) []string {
	if exp.Rise != nil && int(rise) != *exp.Rise {
		return []string{fmt.Sprintf("%s %v rose by %v, want %d", exp.Metric, exp.Attrs, rise, *exp.Rise)}
	}
	return nil
}

// CompareRelayNodes compares the number of api nodes that paired a session of the case.
func CompareRelayNodes(exp Expect, nodes int) []string {
	if exp.NodesAtLeast != nil && nodes < *exp.NodesAtLeast {
		return []string{fmt.Sprintf("%d api nodes paired a session, want at least %d", nodes, *exp.NodesAtLeast)}
	}
	return nil
}

// CompareReadyz compares the readiness of the api containers with exp.
func CompareReadyz(exp Expect, ready map[string]bool) []string {
	var out []string
	for _, svc := range sortedKeys(ready) {
		if exp.Ready != nil && ready[svc] != *exp.Ready {
			out = append(out, fmt.Sprintf("readyz of %s %v, want %v", svc, ready[svc], *exp.Ready))
		}
	}
	return out
}

// CompareCanary compares the canary findings and the positive control with exp.
func CompareCanary(exp Expect, findings []string, controlSeen bool) []string {
	var out []string
	if !controlSeen {
		out = append(out, "the positive control did not see the canaries in the output")
	}
	want := 0
	if exp.Findings != nil {
		want = *exp.Findings
	}
	if len(findings) != want {
		out = append(out, fmt.Sprintf("%d canary findings, want %d: %s", len(findings), want, strings.Join(findings, "; ")))
	}
	return out
}

// CompareJournal checks that the journal of the host service holds every code text of want (codes
// only, for example "\"code\":4110"); the journal itself is never printed.
func CompareJournal(want []string, journal []byte) []string {
	var out []string
	for _, w := range want {
		if !strings.Contains(string(journal), w) {
			out = append(out, fmt.Sprintf("the journal of bothy-host lacks %q", w))
		}
	}
	return out
}

// CompareStatuses compares the statuses of parallel openings with the multiset of exp.
func CompareStatuses(exp Expect, got []int) []string {
	want := slices.Sorted(slices.Values(exp.Statuses))
	have := slices.Sorted(slices.Values(got))
	if !slices.Equal(want, have) {
		return []string{fmt.Sprintf("the parallel openings answered %v, want %v in any order", have, want)}
	}
	return nil
}

// CompareExec compares a command in a host container with exp: exit code and the texts its
// output must hold or lack (the scenario names them; the output itself is never printed).
func CompareExec(exp Expect, code int, stdout, stderr []byte) []string {
	var out []string
	if exp.Exit != nil && code != *exp.Exit {
		out = append(out, fmt.Sprintf("exit %d, want %d", code, *exp.Exit))
	}
	if exp.ExitNot != nil && code == *exp.ExitNot {
		out = append(out, fmt.Sprintf("exit %d, want another code", code))
	}
	for _, s := range exp.StdoutContains {
		if !strings.Contains(string(stdout), s) {
			out = append(out, fmt.Sprintf("the output lacks %q", s))
		}
	}
	for _, s := range exp.StdoutLacks {
		if strings.Contains(string(stdout), s) {
			out = append(out, fmt.Sprintf("the output holds %q", s))
		}
	}
	for _, s := range exp.StderrContains {
		if !strings.Contains(string(stderr), s) {
			out = append(out, fmt.Sprintf("the error output lacks %q", s))
		}
	}
	return out
}
