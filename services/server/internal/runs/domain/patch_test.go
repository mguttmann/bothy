// SPDX-License-Identifier: Apache-2.0

package domain_test

// Tests of the static patch checks (ParseOps, ParsePointer) and helpers shared by the patch
// tests: fixed UUIDv7 ids and field error comparison.

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// tid returns a fixed UUIDv7 for test number n, so tests name steps and messages without a
// generator; the ids sort by n.
func tid(n int) ids.ID {
	return ids.MustParse(fmt.Sprintf("01900000-0000-7000-8000-%012x", n))
}

// sid is tid as the canonical string.
func sid(n int) string { return tid(n).String() }

// rawOp builds a RawOp; value "" means the member value is absent.
func rawOp(op, path, value string) domain.RawOp {
	r := domain.RawOp{Op: op, Path: path}
	if value != "" {
		r.Value = json.RawMessage(value)
	}
	return r
}

// fe is a field error reduced to what the rules fix: pointer and code.
type fe struct{ ptr, code string }

// reduce maps field errors to fe and checks that each has a fixed, non-empty detail.
func reduce(t *testing.T, got []errs.FieldError) []fe {
	t.Helper()
	out := make([]fe, 0, len(got))
	for _, e := range got {
		if e.Detail == "" {
			t.Errorf("field error %s %s has no detail", e.Pointer, e.Code)
		}
		out = append(out, fe{e.Pointer, e.Code})
	}
	return out
}

// patchErrFields returns the field errors of an Apply error and checks its public code.
func patchErrFields(t *testing.T, err error) []fe {
	t.Helper()
	e, ok := errs.As(err)
	if !ok {
		t.Fatalf("error %v is not an *errs.Error", err)
	}
	if e.Code != errs.CodeValidationFailed {
		t.Fatalf("code = %s, want validation_failed", e.Code)
	}
	return reduce(t, e.Fields)
}

// TestPathTable covers every row and every operation of the path table of 09 5.3 and the paths the
// adapter must never patch.
func TestPathTable(t *testing.T) {
	t.Parallel()
	add, rep, rem := domain.OpAdd, domain.OpReplace, domain.OpRemove
	s, m := sid(1), sid(2)
	type row struct {
		path    string
		value   string
		target  domain.Target
		field   string
		id      ids.ID
		allowed []domain.OpKind
	}
	rows := []row{
		{"/status_line", `"Reading"`, domain.TargetStatusLine, "", ids.ID{}, []domain.OpKind{add, rep, rem}},
		{"/last_text_excerpt", `"Hello Alex"`, domain.TargetLastTextExcerpt, "", ids.ID{}, []domain.OpKind{add, rep}},
		{"/model", `"model-a"`, domain.TargetModel, "", ids.ID{}, []domain.OpKind{rep}},
		{"/metrics/last_api_retry", `{"attempt":2}`, domain.TargetLastAPIRetry, "", ids.ID{}, []domain.OpKind{add, rep, rem}},
		{"/cost", `{"amount_micros":1250000,"currency":"USD","is_estimate":true}`, domain.TargetCost, "", ids.ID{}, []domain.OpKind{rep}},
		{"/plan", `{"items":[{"id":"step-1","text":"Reproduce","status":"pending"}]}`, domain.TargetPlan, "", ids.ID{}, []domain.OpKind{add, rep, rem}},
		{"/steps/" + s, `{"kind":"tool_call","name":"read_file"}`, domain.TargetStep, "", tid(1), []domain.OpKind{add}},
		{"/steps/" + s + "/status", `"completed"`, domain.TargetStepMember, "status", tid(1), []domain.OpKind{rep}},
		{"/steps/" + s + "/title", `"Reading export.log"`, domain.TargetStepMember, "title", tid(1), []domain.OpKind{rep}},
		{"/steps/" + s + "/detail", `{"file":"export.log"}`, domain.TargetStepMember, "detail", tid(1), []domain.OpKind{rep}},
		{"/steps/" + s + "/finished_at", `"2026-10-21T10:00:00Z"`, domain.TargetStepMember, "finished_at", tid(1), []domain.OpKind{rep}},
		{"/followups/" + m + "/state", `"delivered"`, domain.TargetFollowupState, "", tid(2), []domain.OpKind{rep}},
	}
	for _, f := range []string{"context_pct", "tokens_in", "tokens_out", "tokens_cache_read", "tokens_cache_write",
		"tokens_cache_write_1h", "subagents", "background_jobs", "api_retries"} {
		v := "5"
		if f == "context_pct" {
			v = "37.5"
		}
		rows = append(rows, row{"/metrics/" + f, v, domain.TargetMetric, f, ids.ID{}, []domain.OpKind{add, rep}})
	}
	lead := rawOp("replace", "/status_line", `"Starting"`)
	for _, r := range rows {
		for _, op := range []domain.OpKind{add, rep, rem} {
			name := string(op) + " " + r.path
			allowed := false
			for _, a := range r.allowed {
				allowed = allowed || a == op
			}
			ops, fieldErrs := domain.ParseOps([]domain.RawOp{lead, rawOp(string(op), r.path, r.value)})
			got := reduce(t, fieldErrs)
			if !allowed {
				want := []fe{{"/ops/1", "patch_path_forbidden"}}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("%s: errors = %v, want %v", name, got, want)
				}
				continue
			}
			if len(got) != 0 {
				t.Errorf("%s: unexpected errors %v", name, got)
				continue
			}
			if len(ops) != 2 {
				t.Fatalf("%s: %d ops, want 2", name, len(ops))
			}
			o := ops[1]
			if o.Index != 1 || o.Kind != op || o.Path != r.path || o.Target != r.target || o.Field != r.field || o.ID != r.id {
				t.Errorf("%s: op = %+v", name, o)
			}
			if (op == rem) != (o.Value == nil) {
				t.Errorf("%s: value = %#v", name, o.Value)
			}
		}
	}

	forbidden := []string{
		"/state", "/version", "/billing_mode_expected", "/billing_mode_used", "/queued_at", "/started_at",
		"/finished_at", "/duration_ms", "/summary", "/id", "", "/", "/cost/currency", "/cost/amount_micros",
		"/plan/items", "/plan/items/0", "/metrics/unknown_field", "/metrics/last_api_retry/attempt",
		"/steps/" + s + "/ordinal", "/steps/" + s + "/kind", "/steps/" + s + "/name",
		"/steps/" + s + "/started_at", "/steps/" + s + "/parent_step_id", "/steps/" + s + "/id",
		"/steps", "/metrics", "/followups", "/followups/" + m, "/followups/" + m + "/message_id",
		"/steps/" + s + "/detail/x", "/followups/" + m + "/state/x", "/status_line/x", "/status_line/",
		"/plan/", "/STATUS_LINE",
	}
	for _, p := range forbidden {
		for _, op := range []domain.OpKind{add, rep, rem} {
			_, fieldErrs := domain.ParseOps([]domain.RawOp{lead, rawOp(string(op), p, `"x"`)})
			got := reduce(t, fieldErrs)
			want := []fe{{"/ops/1", "patch_path_forbidden"}}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s %q: errors = %v, want %v", op, p, got, want)
			}
		}
	}
}

// TestParsePointer covers RFC 6901 parsing and the path errors of ParseOps.
func TestParsePointer(t *testing.T) {
	t.Parallel()
	good := []struct {
		in   string
		want []string
	}{
		{"", []string{}},
		{"/", []string{""}},
		{"//", []string{"", ""}},
		{"/a/", []string{"a", ""}},
		{"/a~1b", []string{"a/b"}},
		{"/a~0b", []string{"a~b"}},
		{"/~01", []string{"~1"}},
		{"/~10", []string{"/0"}},
		{"/m~0n/0", []string{"m~n", "0"}},
	}
	for _, g := range good {
		got, err := domain.ParsePointer(g.in)
		if err != nil || !reflect.DeepEqual(got, g.want) {
			t.Errorf("ParsePointer(%q) = %q, %v; want %q", g.in, got, err, g.want)
		}
	}
	for _, bad := range []string{"a", "a/b", "/~", "/a~", "/a~2", "/~a", "/a/~/b"} {
		if _, err := domain.ParsePointer(bad); err == nil {
			t.Errorf("ParsePointer(%q) has no error", bad)
		}
	}

	s := sid(1)
	cases := []struct {
		name string
		op   domain.RawOp
		want []fe
	}{
		{"escaped slash is one field", rawOp("replace", "/metrics/a~1b", "5"), []fe{{"/ops/0", "patch_path_forbidden"}}},
		{"escaped tilde in id segment", rawOp("add", "/steps/~0", `{"kind":"status","name":"x"}`), []fe{{"/ops/0/path", "invalid_format"}}},
		{"tilde at the end", rawOp("replace", "/status_line~", `"x"`), []fe{{"/ops/0/path", "invalid_format"}}},
		{"tilde with a bad escape", rawOp("replace", "/status_line~2", `"x"`), []fe{{"/ops/0/path", "invalid_format"}}},
		{"no leading slash", rawOp("replace", "status_line", `"x"`), []fe{{"/ops/0/path", "invalid_format"}}},
		{"escaped tilde in a member", rawOp("replace", "/status~0line", `"x"`), []fe{{"/ops/0", "patch_path_forbidden"}}},
		{"trailing empty segment", rawOp("replace", "/status_line/", `"x"`), []fe{{"/ops/0", "patch_path_forbidden"}}},
		{"step id not a uuid", rawOp("replace", "/steps/not-a-uuid/status", `"completed"`), []fe{{"/ops/0/path", "invalid_format"}}},
		{"step id not a uuid, forbidden member", rawOp("replace", "/steps/not-a-uuid/ordinal", `1`), []fe{{"/ops/0/path", "invalid_format"}}},
		{"step id empty", rawOp("add", "/steps/", `{"kind":"status","name":"x"}`), []fe{{"/ops/0/path", "invalid_format"}}},
		{"step id is a UUIDv4", rawOp("replace", "/steps/6ba7b810-9dad-41d1-80b4-00c04fd430c8/status", `"completed"`), []fe{{"/ops/0/path", "invalid_format"}}},
		{"follow-up id not a uuid", rawOp("replace", "/followups/not-a-uuid/state", `"delivered"`), []fe{{"/ops/0/path", "invalid_format"}}},
		{"upper case step id", rawOp("replace", "/steps/"+strings.ToUpper(s)+"/status", `"completed"`), nil},
	}
	for _, c := range cases {
		ops, fieldErrs := domain.ParseOps([]domain.RawOp{c.op})
		got := reduce(t, fieldErrs)
		if len(c.want) == 0 {
			if len(got) != 0 {
				t.Errorf("%s: errors %v", c.name, got)
			} else if ops[0].ID != tid(1) {
				t.Errorf("%s: id = %s", c.name, ops[0].ID)
			}
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: errors = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestValues covers the value rules of every path: types, enums, bounds, closed objects, nulls,
// display texts and the step value rules. The secret marker never appears in a detail.
func TestValues(t *testing.T) {
	t.Parallel()
	s := sid(1)
	st := "/steps/" + s
	step := func(extra string) string { return `{"kind":"tool_call","name":"read_file"` + extra + `}` }
	cost := func(amount, currency, est string) string {
		return `{"amount_micros":` + amount + `,"currency":` + currency + `,"is_estimate":` + est + `}`
	}
	item := func(id, text, status string) string {
		return `{"id":"` + id + `","text":"` + text + `","status":"` + status + `"}`
	}
	const v0 = "/ops/0/value"
	cases := []struct {
		name string
		ops  []domain.RawOp
		want []fe
	}{
		// Texts.
		{"status_line number", []domain.RawOp{rawOp("replace", "/status_line", "5")}, []fe{{v0, "invalid_type"}}},
		{"status_line null", []domain.RawOp{rawOp("replace", "/status_line", "null")}, []fe{{v0, "invalid_type"}}},
		{"status_line without value", []domain.RawOp{rawOp("add", "/status_line", "")}, []fe{{v0, "required"}}},
		{"status_line bidi", []domain.RawOp{rawOp("replace", "/status_line", "\"Robin-secret\u202etxt\"")}, []fe{{v0, "bidi_control"}}},
		{"status_line newline", []domain.RawOp{rawOp("replace", "/status_line", `"Robin-secret\nx"`)}, []fe{{v0, "control_character"}}},
		{"remove ignores a value", []domain.RawOp{rawOp("remove", "/status_line", "5")}, nil},
		{"remove ignores null", []domain.RawOp{rawOp("remove", "/status_line", "null")}, nil},
		{"value not JSON", []domain.RawOp{rawOp("replace", "/status_line", `{`)}, []fe{{v0, "invalid_format"}}},
		{"value with trailing data", []domain.RawOp{rawOp("replace", "/status_line", `"a" "b"`)}, []fe{{v0, "invalid_format"}}},
		{"unknown operation", []domain.RawOp{rawOp("move", "/status_line", `"a"`)}, []fe{{"/ops/0/op", "invalid_enum"}}},
		{"excerpt allows controls", []domain.RawOp{rawOp("replace", "/last_text_excerpt", "\"a\u202eb\\nc\"")}, nil},
		{"excerpt number", []domain.RawOp{rawOp("add", "/last_text_excerpt", "5")}, []fe{{v0, "invalid_type"}}},
		{"model bell", []domain.RawOp{rawOp("replace", "/model", `"m\u0007"`)}, []fe{{v0, "control_character"}}},
		// Metrics.
		{"tokens string", []domain.RawOp{rawOp("add", "/metrics/tokens_in", `"5"`)}, []fe{{v0, "invalid_type"}}},
		{"tokens fraction", []domain.RawOp{rawOp("add", "/metrics/tokens_in", `1.5`)}, []fe{{v0, "invalid_type"}}},
		{"tokens small exponent", []domain.RawOp{rawOp("add", "/metrics/tokens_in", `1e-1`)}, []fe{{v0, "invalid_type"}}},
		{"tokens integral fraction", []domain.RawOp{rawOp("add", "/metrics/tokens_in", `1.0`)}, nil},
		{"tokens integral exponent", []domain.RawOp{rawOp("add", "/metrics/tokens_in", `1e2`)}, nil},
		{"tokens negative", []domain.RawOp{rawOp("add", "/metrics/tokens_out", `-1`)}, []fe{{v0, "too_small"}}},
		{"tokens above 2^53-1", []domain.RawOp{rawOp("add", "/metrics/subagents", `9007199254740992`)}, []fe{{v0, "too_large"}}},
		{"tokens at 2^53-1", []domain.RawOp{rawOp("add", "/metrics/subagents", `9007199254740991`)}, nil},
		{"tokens huge", []domain.RawOp{rawOp("add", "/metrics/api_retries", `1e400`)}, []fe{{v0, "too_large"}}},
		{"tokens huge negative", []domain.RawOp{rawOp("add", "/metrics/api_retries", `-1e400`)}, []fe{{v0, "too_small"}}},
		{"tokens boolean", []domain.RawOp{rawOp("add", "/metrics/background_jobs", `true`)}, []fe{{v0, "invalid_type"}}},
		{"context_pct below 0", []domain.RawOp{rawOp("add", "/metrics/context_pct", `-0.5`)}, []fe{{v0, "too_small"}}},
		{"context_pct above 100", []domain.RawOp{rawOp("add", "/metrics/context_pct", `100.5`)}, []fe{{v0, "too_large"}}},
		{"context_pct huge", []domain.RawOp{rawOp("add", "/metrics/context_pct", `1e400`)}, []fe{{v0, "too_large"}}},
		{"context_pct 100", []domain.RawOp{rawOp("add", "/metrics/context_pct", `100`)}, nil},
		{"context_pct string", []domain.RawOp{rawOp("add", "/metrics/context_pct", `"5"`)}, []fe{{v0, "invalid_type"}}},
		{"retry without attempt", []domain.RawOp{rawOp("add", "/metrics/last_api_retry", `{}`)}, []fe{{v0 + "/attempt", "required"}}},
		{"retry unknown member", []domain.RawOp{rawOp("add", "/metrics/last_api_retry", `{"attempt":1,"x":1}`)}, []fe{{v0 + "/x", "unknown_member"}}},
		{"retry attempt string", []domain.RawOp{rawOp("add", "/metrics/last_api_retry", `{"attempt":"1"}`)}, []fe{{v0 + "/attempt", "invalid_type"}}},
		{"retry error bidi", []domain.RawOp{rawOp("add", "/metrics/last_api_retry", "{\"attempt\":1,\"error\":\"a\u202e\"}")}, []fe{{v0 + "/error", "bidi_control"}}},
		{"retry nested null", []domain.RawOp{rawOp("add", "/metrics/last_api_retry", `{"attempt":1,"max_retries":null}`)}, []fe{{v0 + "/max_retries", "invalid_type"}}},
		{"retry array", []domain.RawOp{rawOp("add", "/metrics/last_api_retry", `[]`)}, []fe{{v0, "invalid_type"}}},
		{"retry negative delay", []domain.RawOp{rawOp("add", "/metrics/last_api_retry", `{"attempt":1,"retry_delay_ms":-1}`)}, []fe{{v0 + "/retry_delay_ms", "too_small"}}},
		{"retry full", []domain.RawOp{rawOp("replace", "/metrics/last_api_retry", `{"attempt":2,"max_retries":5,"retry_delay_ms":4000,"error":"overloaded"}`)}, nil},
		// Cost.
		{"cost not an estimate", []domain.RawOp{rawOp("replace", "/cost", cost("1", `"USD"`, "false"))}, []fe{{v0 + "/is_estimate", "invalid"}}},
		{"cost lower case currency", []domain.RawOp{rawOp("replace", "/cost", cost("1", `"usd"`, "true"))}, []fe{{v0 + "/currency", "pattern_mismatch"}}},
		{"cost short currency", []domain.RawOp{rawOp("replace", "/cost", cost("1", `"US"`, "true"))}, []fe{{v0 + "/currency", "pattern_mismatch"}}},
		{"cost umlaut currency", []domain.RawOp{rawOp("replace", "/cost", cost("1", `"ÜSD"`, "true"))}, []fe{{v0 + "/currency", "pattern_mismatch"}}},
		{"cost currency number", []domain.RawOp{rawOp("replace", "/cost", cost("1", `5`, "true"))}, []fe{{v0 + "/currency", "invalid_type"}}},
		{"cost estimate string", []domain.RawOp{rawOp("replace", "/cost", cost("1", `"USD"`, `"true"`))}, []fe{{v0 + "/is_estimate", "invalid_type"}}},
		{"cost negative", []domain.RawOp{rawOp("replace", "/cost", cost("-1", `"USD"`, "true"))}, []fe{{v0 + "/amount_micros", "too_small"}}},
		{"cost empty", []domain.RawOp{rawOp("replace", "/cost", `{}`)}, []fe{{v0 + "/amount_micros", "required"}, {v0 + "/currency", "required"}, {v0 + "/is_estimate", "required"}}},
		{"cost unknown escaped member", []domain.RawOp{rawOp("replace", "/cost", `{"amount_micros":1,"currency":"EUR","is_estimate":true,"a/b~c":1}`)}, []fe{{v0 + "/a~1b~0c", "unknown_member"}}},
		// Plan.
		{"plan without items", []domain.RawOp{rawOp("add", "/plan", `{}`)}, []fe{{v0 + "/items", "required"}}},
		{"plan items object", []domain.RawOp{rawOp("add", "/plan", `{"items":{}}`)}, []fe{{v0 + "/items", "invalid_type"}}},
		{"plan bad status", []domain.RawOp{rawOp("add", "/plan", `{"items":[`+item("a", "t", "done")+`]}`)}, []fe{{v0 + "/items/0/status", "invalid_enum"}}},
		{"plan duplicate id", []domain.RawOp{rawOp("add", "/plan", `{"items":[`+item("a", "t", "pending")+`,`+item("b", "t", "pending")+`,`+item("a", "u", "completed")+`]}`)}, []fe{{v0 + "/items/2/id", "invalid"}}},
		{"plan item without text", []domain.RawOp{rawOp("add", "/plan", `{"items":[{"id":"a","status":"pending"}]}`)}, []fe{{v0 + "/items/0/text", "required"}}},
		{"plan item unknown member", []domain.RawOp{rawOp("add", "/plan", `{"items":[{"id":"a","text":"t","status":"pending","x":1}]}`)}, []fe{{v0 + "/items/0/x", "unknown_member"}}},
		{"plan item text newline", []domain.RawOp{rawOp("add", "/plan", `{"items":[`+item("a", `t\n`, "pending")+`]}`)}, []fe{{v0 + "/items/0/text", "control_character"}}},
		{"plan item null", []domain.RawOp{rawOp("add", "/plan", `{"items":[null]}`)}, []fe{{v0 + "/items/0", "invalid_type"}}},
		{"plan unknown member", []domain.RawOp{rawOp("add", "/plan", `{"items":[],"x":1}`)}, []fe{{v0 + "/x", "unknown_member"}}},
		{"plan empty items", []domain.RawOp{rawOp("replace", "/plan", `{"items":[]}`)}, nil},
		// Step add.
		{"step ordinal", []domain.RawOp{rawOp("add", st, step(`,"ordinal":3`))}, []fe{{v0 + "/ordinal", "patch_path_forbidden"}}},
		{"step other id", []domain.RawOp{rawOp("add", st, step(`,"id":"`+sid(9)+`"`))}, []fe{{v0 + "/id", "invalid"}}},
		{"step same id", []domain.RawOp{rawOp("add", st, step(`,"id":"`+s+`"`))}, nil},
		{"step without kind", []domain.RawOp{rawOp("add", st, `{"name":"read_file"}`)}, []fe{{v0 + "/kind", "required"}}},
		{"step without name", []domain.RawOp{rawOp("add", st, `{"kind":"status"}`)}, []fe{{v0 + "/name", "required"}}},
		{"step bad kind", []domain.RawOp{rawOp("add", st, `{"kind":"shell","name":"x"}`)}, []fe{{v0 + "/kind", "invalid_enum"}}},
		{"step name newline", []domain.RawOp{rawOp("add", st, `{"kind":"status","name":"a\nb"}`)}, []fe{{v0 + "/name", "control_character"}}},
		{"step title newline", []domain.RawOp{rawOp("add", st, step(`,"title":"a\nb"`))}, []fe{{v0 + "/title", "control_character"}}},
		{"step bad status", []domain.RawOp{rawOp("add", st, step(`,"status":"done"`))}, []fe{{v0 + "/status", "invalid_enum"}}},
		{"step detail array", []domain.RawOp{rawOp("add", st, step(`,"detail":[]`))}, []fe{{v0 + "/detail", "invalid_type"}}},
		{"step detail null member", []domain.RawOp{rawOp("add", st, step(`,"detail":{"a":null}`))}, []fe{{v0 + "/detail/a", "invalid_type"}}},
		{"step detail allows controls", []domain.RawOp{rawOp("add", st, step(",\"detail\":{\"a\":\"\u202e\\n\"}"))}, nil},
		{"step bad parent", []domain.RawOp{rawOp("add", st, step(`,"parent_step_id":"nope"`))}, []fe{{v0 + "/parent_step_id", "invalid_format"}}},
		{"step bad started_at", []domain.RawOp{rawOp("add", st, step(`,"started_at":"yesterday"`))}, []fe{{v0 + "/started_at", "invalid_format"}}},
		{"step unknown member", []domain.RawOp{rawOp("add", st, step(`,"foo":1`))}, []fe{{v0 + "/foo", "unknown_member"}}},
		{"step not an object", []domain.RawOp{rawOp("add", st, `"x"`)}, []fe{{v0, "invalid_type"}}},
		{"step full", []domain.RawOp{rawOp("add", st, step(`,"title":"t","status":"completed","detail":{"a":[1,"b"]},"parent_step_id":"`+sid(5)+`","started_at":"2026-10-21T10:00:00.123456789Z","finished_at":"2026-10-21T10:00:01+02:00"`))}, nil},
		// Step members.
		{"step status", []domain.RawOp{rawOp("replace", st+"/status", `"done"`)}, []fe{{v0, "invalid_enum"}}},
		{"step title number", []domain.RawOp{rawOp("replace", st+"/title", `5`)}, []fe{{v0, "invalid_type"}}},
		{"step title newline member", []domain.RawOp{rawOp("replace", st+"/title", `"\n"`)}, []fe{{v0, "control_character"}}},
		{"step detail string", []domain.RawOp{rawOp("replace", st+"/detail", `"x"`)}, []fe{{v0, "invalid_type"}}},
		{"step detail deep null", []domain.RawOp{rawOp("replace", st+"/detail", `{"x":{"y":[1,null]}}`)}, []fe{{v0 + "/x/y/1", "invalid_type"}}},
		{"step finished_at date", []domain.RawOp{rawOp("replace", st+"/finished_at", `"2026-10-21"`)}, []fe{{v0, "invalid_format"}}},
		{"step finished_at number", []domain.RawOp{rawOp("replace", st+"/finished_at", `5`)}, []fe{{v0, "invalid_type"}}},
		// Follow-ups.
		{"followup queued", []domain.RawOp{rawOp("replace", "/followups/"+sid(2)+"/state", `"queued"`)}, []fe{{v0, "invalid_enum"}}},
		{"followup absorbed", []domain.RawOp{rawOp("replace", "/followups/"+sid(2)+"/state", `"absorbed"`)}, nil},
		// Several ops: errors in op order.
		{"op order", []domain.RawOp{
			rawOp("replace", "/status_line", "5"),
			rawOp("replace", "/model", `"ok"`),
			rawOp("replace", "/state", `"running"`),
			rawOp("add", "/metrics/tokens_in", `-3`),
			rawOp("replace", "/steps/x/status", `"running"`),
		}, []fe{{v0, "invalid_type"}, {"/ops/2", "patch_path_forbidden"}, {"/ops/3/value", "too_small"}, {"/ops/4/path", "invalid_format"}}},
	}
	for _, c := range cases {
		ops, fieldErrs := domain.ParseOps(c.ops)
		for _, e := range fieldErrs {
			if strings.Contains(e.Detail, "Robin-secret") || strings.Contains(e.Detail, "read_file") {
				t.Errorf("%s: detail %q repeats the value", c.name, e.Detail)
			}
		}
		got := reduce(t, fieldErrs)
		if len(c.want) == 0 {
			if len(got) != 0 {
				t.Errorf("%s: errors %v", c.name, got)
			}
			if len(ops) != len(c.ops) {
				t.Errorf("%s: %d ops, want %d", c.name, len(ops), len(c.ops))
			}
			continue
		}
		if ops != nil {
			t.Errorf("%s: ops returned with errors", c.name)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: errors = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestValueTypes checks the decoded value of each target.
func TestValueTypes(t *testing.T) {
	t.Parallel()
	s := sid(1)
	ops, fieldErrs := domain.ParseOps([]domain.RawOp{
		rawOp("replace", "/metrics/context_pct", `37.5`),
		rawOp("replace", "/metrics/tokens_in", `1e2`),
		rawOp("replace", "/metrics/last_api_retry", `{"attempt":2,"error":"overloaded"}`),
		rawOp("replace", "/cost", `{"amount_micros":1250000,"currency":"EUR","is_estimate":true}`),
		rawOp("replace", "/plan", `{"items":[{"id":"a","text":"Reproduce","status":"in_progress"}]}`),
		rawOp("add", "/steps/"+s, `{"kind":"subagent","name":"review","parent_step_id":"`+sid(5)+`","detail":{"b":1,"a":"x"},"finished_at":"2026-10-21T12:00:01+02:00"}`),
		rawOp("replace", "/steps/"+s+"/finished_at", `"2026-10-21T10:00:00.5Z"`),
		rawOp("replace", "/steps/"+s+"/detail", `{"k":true}`),
		rawOp("replace", "/followups/"+sid(2)+"/state", `"delivered"`),
		rawOp("replace", "/last_text_excerpt", `"Hello Sam"`),
	})
	if len(fieldErrs) != 0 {
		t.Fatalf("errors %v", fieldErrs)
	}
	if v, ok := ops[0].Value.(float64); !ok || v != 37.5 {
		t.Errorf("context_pct = %#v", ops[0].Value)
	}
	if v, ok := ops[1].Value.(int64); !ok || v != 100 {
		t.Errorf("tokens_in = %#v", ops[1].Value)
	}
	if v, ok := ops[2].Value.(domain.APIRetry); !ok || v.Attempt != 2 || v.MaxRetries != nil || v.Error == nil || *v.Error != "overloaded" {
		t.Errorf("last_api_retry = %#v", ops[2].Value)
	}
	if v, ok := ops[3].Value.(domain.Money); !ok || v != (domain.Money{AmountMicros: 1250000, Currency: "EUR", IsEstimate: true}) {
		t.Errorf("cost = %#v", ops[3].Value)
	}
	if v, ok := ops[4].Value.(domain.Plan); !ok || !reflect.DeepEqual(v.Items, []domain.PlanItem{{ID: "a", Text: "Reproduce", Status: "in_progress"}}) {
		t.Errorf("plan = %#v", ops[4].Value)
	}
	in, ok := ops[5].Value.(domain.StepInput)
	if !ok || in.Kind != "subagent" || in.Name != "review" || in.Title != "" || in.Status != "" || in.ParentStepID != tid(5) ||
		!in.StartedAt.IsZero() || !in.FinishedAt.Equal(time.Date(2026, 10, 21, 10, 0, 1, 0, time.UTC)) {
		t.Errorf("step = %#v", ops[5].Value)
	}
	if !jsonEqual(in.Detail, json.RawMessage(`{"a":"x","b":1}`)) {
		t.Errorf("step detail = %s", in.Detail)
	}
	if v, ok := ops[6].Value.(time.Time); !ok || !v.Equal(time.Date(2026, 10, 21, 10, 0, 0, 5e8, time.UTC)) {
		t.Errorf("finished_at = %#v", ops[6].Value)
	}
	if v, ok := ops[7].Value.(json.RawMessage); !ok || !jsonEqual(v, json.RawMessage(`{"k":true}`)) {
		t.Errorf("detail = %#v", ops[7].Value)
	}
	if v, ok := ops[8].Value.(string); !ok || v != "delivered" {
		t.Errorf("followup = %#v", ops[8].Value)
	}
	if v, ok := ops[9].Value.(string); !ok || v != "Hello Sam" {
		t.Errorf("excerpt = %#v", ops[9].Value)
	}
}

// jsonEqual reports whether a and b are the same JSON value (numbers compared as float64).
func jsonEqual(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}

// TestValuesUnstorable holds the rule of findUnstorable (code review of M2-10): U+0000 in any
// string or member name, also in last_text_excerpt, a step detail and a plan item id, gives
// invalid, and a number beyond 2^53-1 anywhere, also inside a detail, gives too_large or
// too_small, because PostgreSQL refuses U+0000 and getRun could not answer such a number.
func TestValuesUnstorable(t *testing.T) {
	t.Parallel()
	step := "/steps/" + ids.New().String()
	for _, c := range []struct {
		name, op, path, value, pointer, code string
	}{
		{"nul in excerpt", "replace", "/last_text_excerpt", `"bin\u0000ary"`, "/ops/0/value", "invalid"},
		{"nul in detail value", "replace", step + "/detail", `{"out":"a\u0000b"}`, "/ops/0/value/out", "invalid"},
		{"nul in detail key", "replace", step + "/detail", `{"a\u0000":1}`, "/ops/0/value/a\u0000", "invalid"},
		{"nul in plan id", "replace", "/plan", `{"items":[{"id":"x\u0000","text":"t","status":"pending"}]}`, "/ops/0/value/items/0/id", "invalid"},
		{"huge integer in detail", "replace", step + "/detail", `{"n":9007199254740993}`, "/ops/0/value/n", "too_large"},
		{"huge float in detail", "replace", step + "/detail", `{"n":1.76e18}`, "/ops/0/value/n", "too_large"},
		{"huge negative in detail", "replace", step + "/detail", `{"n":[-9007199254740993.5]}`, "/ops/0/value/n/0", "too_small"},
	} {
		_, fields := domain.ParseOps([]domain.RawOp{{Op: c.op, Path: c.path, Value: json.RawMessage(c.value)}})
		if len(fields) != 1 || fields[0].Pointer != c.pointer || fields[0].Code != c.code {
			t.Errorf("%s: %+v, want %s at %s", c.name, fields, c.code, c.pointer)
		}
	}
	// Numbers within 2^53-1 inside a detail stay as sent.
	if _, fields := domain.ParseOps([]domain.RawOp{{Op: "replace", Path: step + "/detail", Value: json.RawMessage(`{"n":9007199254740991,"f":-1.5}`)}}); len(fields) != 0 {
		t.Errorf("safe numbers: %+v", fields)
	}
}
