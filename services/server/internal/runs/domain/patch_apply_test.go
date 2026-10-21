// SPDX-License-Identifier: Apache-2.0

package domain_test

// Tests of Apply: set semantics, dynamic checks, the Change contract, and the property test of
// Design note 4 with a renderer of the run document written independently of the package.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// docTime is the time layout of the run document: time.Time as JSON after the cut to
// milliseconds, the form getRun answers.
const docTime = time.RFC3339Nano

// testNow is the transaction time of the tests; its nanoseconds prove the cut to microseconds
// in storage and to milliseconds in the document.
var testNow = time.Date(2026, 10, 21, 10, 0, 0, 123456789, time.UTC)

// testEnv returns an Env at testNow whose ids start at tid(first).
func testEnv(first int) domain.Env {
	n := first
	return domain.Env{Now: testNow, NewID: func() ids.ID { n++; return tid(n - 1) }}
}

// testLimits are generous limits for tests that do not test the limits.
var testLimits = domain.Limits{ExcerptMaxChars: 100, StepDetailMaxBytes: 1000, MaxStepsPersisted: 100}

// baseState is a small run: one running step, one follow-up.
func baseState() domain.DocumentState {
	return domain.DocumentState{
		StatusLine: "Reading",
		Metrics:    json.RawMessage(`{"tokens_in":5}`),
		Plan:       json.RawMessage(`{}`),
		Cost:       domain.Money{AmountMicros: 0, Currency: "EUR", IsEstimate: true},
		Steps: []domain.DocStep{{
			ID: tid(1), Ordinal: 1, Kind: "tool_call", Name: "read_file", Status: "running",
			Detail: json.RawMessage(`{}`), StartedAt: time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC),
		}},
		Followups: map[ids.ID]string{tid(2): "queued"},
	}
}

// mustApply parses raws (failing the test on field errors) and applies them.
func mustApply(t *testing.T, st domain.DocumentState, lim domain.Limits, raws ...domain.RawOp) (domain.Change, []domain.NormOp, error) {
	t.Helper()
	ops, fieldErrs := domain.ParseOps(raws)
	if len(fieldErrs) != 0 {
		t.Fatalf("ParseOps: %v", fieldErrs)
	}
	ch, norm, err := domain.Apply(st, ops, lim, testEnv(900))
	if err == nil {
		checkProperty(t, st, ops, ch, norm, lim)
	}
	return ch, norm, err
}

// nop builds an expected NormOp.
func nop(op domain.OpKind, path, value string) domain.NormOp {
	n := domain.NormOp{Op: op, Path: path}
	if value != "" {
		n.Value = json.RawMessage(value)
	}
	return n
}

// sameNorm compares normalized ops, values as JSON.
func sameNorm(t *testing.T, name string, got, want []domain.NormOp) {
	t.Helper()
	ok := len(got) == len(want)
	for i := 0; ok && i < len(got); i++ {
		ok = got[i].Op == want[i].Op && got[i].Path == want[i].Path &&
			(got[i].Value == nil) == (want[i].Value == nil) &&
			(got[i].Value == nil || jsonEqual(got[i].Value, want[i].Value))
	}
	if !ok {
		t.Errorf("%s: normalized ops\n got %s\nwant %s", name, showNorm(got), showNorm(want))
	}
}

// showNorm renders normalized ops for a failure message.
func showNorm(ops []domain.NormOp) string {
	var b strings.Builder
	for _, o := range ops {
		fmt.Fprintf(&b, "[%s %s %s] ", o.Op, o.Path, o.Value)
	}
	return b.String()
}

// TestSetSemantics covers the set semantics of the patch rules and the dynamic checks.
func TestSetSemantics(t *testing.T) {
	t.Parallel()
	s1, s3, s4, s9 := sid(1), sid(3), sid(4), sid(9)

	t.Run("replace on absent model adds it", func(t *testing.T) {
		ch, norm, err := mustApply(t, baseState(), testLimits, rawOp("replace", "/model", `"model-b"`))
		if err != nil {
			t.Fatal(err)
		}
		sameNorm(t, "model", norm, []domain.NormOp{nop(domain.OpAdd, "/model", `"model-b"`)})
		if ch.Model != (domain.TextColumn{Set: true, Value: "model-b"}) || ch.After.Model != "model-b" {
			t.Errorf("change = %+v", ch.Model)
		}
	})

	t.Run("replace on absent finished_at adds it", func(t *testing.T) {
		ch, norm, err := mustApply(t, baseState(), testLimits, rawOp("replace", "/steps/"+s1+"/finished_at", `"2026-10-21T10:00:01.123456789Z"`))
		if err != nil {
			t.Fatal(err)
		}
		sameNorm(t, "finished_at", norm, []domain.NormOp{nop(domain.OpAdd, "/steps/"+s1+"/finished_at", `"2026-10-21T10:00:01.123Z"`)})
		want := time.Date(2026, 10, 21, 10, 0, 1, 123456000, time.UTC)
		if len(ch.UpdateSteps) != 1 || ch.UpdateSteps[0].ID != tid(1) || ch.UpdateSteps[0].FinishedAt == nil || !ch.UpdateSteps[0].FinishedAt.Equal(want) ||
			ch.UpdateSteps[0].Status != nil || ch.UpdateSteps[0].Title != nil || ch.UpdateSteps[0].Detail != nil || ch.UpdateSteps[0].Ordinal != nil {
			t.Errorf("updates = %+v", ch.UpdateSteps)
		}
	})

	t.Run("remove of absent status_line is dropped", func(t *testing.T) {
		st := baseState()
		st.StatusLine = ""
		ch, norm, err := mustApply(t, st, testLimits, rawOp("remove", "/status_line", ""))
		if err != nil {
			t.Fatal(err)
		}
		sameNorm(t, "remove", norm, nil)
		if ch.StatusLine.Set {
			t.Errorf("status_line = %+v", ch.StatusLine)
		}
	})

	t.Run("equal value is dropped, other value replaces, empty removes", func(t *testing.T) {
		ch, norm, err := mustApply(t, baseState(), testLimits,
			rawOp("replace", "/status_line", `"Reading"`),
			rawOp("add", "/metrics/tokens_in", `5`),
			rawOp("replace", "/status_line", `"Writing"`),
			rawOp("add", "/metrics/tokens_out", `7`),
			rawOp("replace", "/metrics/context_pct", `37.5`),
			rawOp("replace", "/status_line", `""`),
		)
		if err != nil {
			t.Fatal(err)
		}
		sameNorm(t, "texts", norm, []domain.NormOp{
			nop(domain.OpReplace, "/status_line", `"Writing"`),
			nop(domain.OpAdd, "/metrics/tokens_out", `7`),
			nop(domain.OpAdd, "/metrics/context_pct", `37.5`),
			nop(domain.OpRemove, "/status_line", ""),
		})
		if ch.StatusLine != (domain.TextColumn{Set: true, Value: ""}) {
			t.Errorf("status_line = %+v", ch.StatusLine)
		}
		if !jsonEqual(ch.Metrics, []byte(`{"tokens_in":5,"tokens_out":7,"context_pct":37.5}`)) {
			t.Errorf("metrics = %s", ch.Metrics)
		}
	})

	t.Run("cost and last_api_retry", func(t *testing.T) {
		ch, norm, err := mustApply(t, baseState(), testLimits,
			rawOp("replace", "/cost", `{"amount_micros":0,"currency":"EUR","is_estimate":true}`),
			rawOp("replace", "/cost", `{"amount_micros":1250000,"currency":"EUR","is_estimate":true}`),
			rawOp("remove", "/metrics/last_api_retry", ""),
			rawOp("add", "/metrics/last_api_retry", `{"error":"overloaded","attempt":2}`),
			rawOp("replace", "/metrics/last_api_retry", `{"attempt":2,"error":"overloaded"}`),
			rawOp("remove", "/metrics/last_api_retry", ""),
		)
		if err != nil {
			t.Fatal(err)
		}
		sameNorm(t, "cost", norm, []domain.NormOp{
			nop(domain.OpReplace, "/cost", `{"amount_micros":1250000,"currency":"EUR","is_estimate":true}`),
			nop(domain.OpAdd, "/metrics/last_api_retry", `{"attempt":2,"error":"overloaded"}`),
			nop(domain.OpRemove, "/metrics/last_api_retry", ""),
		})
		if ch.Cost == nil || *ch.Cost != (domain.Money{AmountMicros: 1250000, Currency: "EUR", IsEstimate: true}) {
			t.Errorf("cost = %+v", ch.Cost)
		}
		if ch.Metrics != nil {
			t.Errorf("metrics = %s, want unchanged", ch.Metrics)
		}
	})

	t.Run("plan", func(t *testing.T) {
		st := baseState()
		st.Plan = json.RawMessage(`{"items":[{"id":"a","text":"Reproduce","status":"pending"}]}`)
		ch, norm, err := mustApply(t, st, testLimits, rawOp("replace", "/plan", `{"items":[]}`))
		if err != nil {
			t.Fatal(err)
		}
		sameNorm(t, "plan", norm, []domain.NormOp{nop(domain.OpRemove, "/plan", "")})
		if !jsonEqual(ch.Plan, []byte(`{}`)) {
			t.Errorf("plan = %s", ch.Plan)
		}
		ch, norm, err = mustApply(t, baseState(), testLimits, rawOp("replace", "/plan", `{"items":[]}`), rawOp("remove", "/plan", ""))
		if err != nil {
			t.Fatal(err)
		}
		sameNorm(t, "plan no-op", norm, nil)
		if ch.Plan != nil {
			t.Errorf("plan = %s", ch.Plan)
		}
		_, norm, err = mustApply(t, baseState(), testLimits, rawOp("add", "/plan", `{"items":[{"id":"a","text":"Fix","status":"in_progress"}]}`))
		if err != nil {
			t.Fatal(err)
		}
		sameNorm(t, "plan add", norm, []domain.NormOp{nop(domain.OpAdd, "/plan", `{"items":[{"id":"a","text":"Fix","status":"in_progress"}]}`)})
	})

	t.Run("excerpt is cut before it is stored", func(t *testing.T) {
		lim := testLimits
		lim.ExcerptMaxChars = 3
		ch, norm, err := mustApply(t, baseState(), lim, rawOp("replace", "/last_text_excerpt", `"abcdef"`))
		if err != nil {
			t.Fatal(err)
		}
		sameNorm(t, "excerpt", norm, []domain.NormOp{nop(domain.OpAdd, "/last_text_excerpt", `"def"`)})
		if ch.LastTextExcerpt != (domain.TextColumn{Set: true, Value: "def"}) {
			t.Errorf("excerpt = %+v", ch.LastTextExcerpt)
		}
	})

	t.Run("add on an existing step", func(t *testing.T) {
		ch, norm, err := mustApply(t, baseState(), testLimits, rawOp("add", "/steps/"+s1, `{"kind":"status","name":"x"}`))
		if got := patchErrFields(t, err); !reflect.DeepEqual(got, []fe{{"/ops/0", "patch_target_exists"}}) {
			t.Errorf("errors = %v", got)
		}
		if norm != nil || !reflect.DeepEqual(ch, domain.Change{}) {
			t.Errorf("a failed patch returned a change")
		}
		_, _, err = mustApply(t, baseState(), testLimits,
			rawOp("add", "/steps/"+s3, `{"kind":"status","name":"x"}`),
			rawOp("add", "/steps/"+s3, `{"kind":"status","name":"y"}`))
		if got := patchErrFields(t, err); !reflect.DeepEqual(got, []fe{{"/ops/1", "patch_target_exists"}}) {
			t.Errorf("errors = %v", got)
		}
	})

	t.Run("member replace on an unknown step", func(t *testing.T) {
		_, _, err := mustApply(t, baseState(), testLimits, rawOp("replace", "/status_line", `"x"`), rawOp("replace", "/steps/"+s9+"/status", `"completed"`))
		if got := patchErrFields(t, err); !reflect.DeepEqual(got, []fe{{"/ops/1", "patch_target_missing"}}) {
			t.Errorf("errors = %v", got)
		}
	})

	t.Run("unknown parent", func(t *testing.T) {
		_, _, err := mustApply(t, baseState(), testLimits, rawOp("add", "/steps/"+s3, `{"kind":"status","name":"x","parent_step_id":"`+s9+`"}`))
		if got := patchErrFields(t, err); !reflect.DeepEqual(got, []fe{{"/ops/0/value/parent_step_id", "patch_target_missing"}}) {
			t.Errorf("errors = %v", got)
		}
	})

	t.Run("parent added earlier in the patch", func(t *testing.T) {
		ch, norm, err := mustApply(t, baseState(), testLimits,
			rawOp("add", "/steps/"+s3, `{"kind":"workflow","name":"build"}`),
			rawOp("add", "/steps/"+s4, `{"kind":"tool_call","name":"run","parent_step_id":"`+s3+`","title":"Alex","detail":{"b":2,"a":"x"},"started_at":"2026-10-21T08:00:00.987654321Z"}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		sameNorm(t, "steps", norm, []domain.NormOp{
			nop(domain.OpAdd, "/steps/"+s3, `{"id":"`+s3+`","ordinal":2,"kind":"workflow","name":"build","title":"","status":"running","detail":{},"started_at":"2026-10-21T10:00:00.123Z"}`),
			nop(domain.OpAdd, "/steps/"+s4, `{"id":"`+s4+`","ordinal":3,"parent_step_id":"`+s3+`","kind":"tool_call","name":"run","title":"Alex","status":"running","detail":{"a":"x","b":2},"started_at":"2026-10-21T08:00:00.987Z"}`),
		})
		if len(ch.InsertSteps) != 2 || ch.InsertSteps[0].ID != tid(3) || ch.InsertSteps[1].ID != tid(4) ||
			ch.InsertSteps[0].Ordinal != 2 || ch.InsertSteps[1].Ordinal != 3 || ch.InsertSteps[1].ParentStepID != tid(3) {
			t.Errorf("inserts = %+v", ch.InsertSteps)
		}
		if !ch.InsertSteps[0].StartedAt.Equal(time.Date(2026, 10, 21, 10, 0, 0, 123456000, time.UTC)) {
			t.Errorf("started_at = %s", ch.InsertSteps[0].StartedAt)
		}
		if !bytes.Equal(ch.InsertSteps[1].Detail, []byte(`{"a":"x","b":2}`)) {
			t.Errorf("stored detail = %s", ch.InsertSteps[1].Detail)
		}
		if !reflect.DeepEqual(ch.AddedSteps, ch.InsertSteps) {
			t.Errorf("added = %+v", ch.AddedSteps)
		}
	})

	t.Run("added then changed is one insert", func(t *testing.T) {
		ch, norm, err := mustApply(t, baseState(), testLimits,
			rawOp("add", "/steps/"+s3, `{"kind":"status","name":"x"}`),
			rawOp("replace", "/steps/"+s3+"/status", `"completed"`),
			rawOp("replace", "/steps/"+s3+"/title", `"Done"`),
			rawOp("replace", "/steps/"+s3+"/detail", `{"n":1}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(norm) != 4 || len(ch.UpdateSteps) != 0 || len(ch.InsertSteps) != 1 {
			t.Fatalf("norm %d, updates %d, inserts %d", len(norm), len(ch.UpdateSteps), len(ch.InsertSteps))
		}
		in := ch.InsertSteps[0]
		if in.Status != "completed" || in.Title != "Done" || !bytes.Equal(in.Detail, []byte(`{"n":1}`)) {
			t.Errorf("insert = %+v", in)
		}
	})

	t.Run("follow-ups", func(t *testing.T) {
		_, _, err := mustApply(t, baseState(), testLimits, rawOp("replace", "/followups/"+s9+"/state", `"delivered"`))
		if got := patchErrFields(t, err); !reflect.DeepEqual(got, []fe{{"/ops/0", "patch_target_missing"}}) {
			t.Errorf("errors = %v", got)
		}
		ch, norm, err := mustApply(t, baseState(), testLimits,
			rawOp("replace", "/followups/"+sid(2)+"/state", `"delivered"`),
			rawOp("replace", "/followups/"+sid(2)+"/state", `"delivered"`))
		if err != nil {
			t.Fatal(err)
		}
		sameNorm(t, "followups", norm, []domain.NormOp{nop(domain.OpReplace, "/followups/"+sid(2)+"/state", `"delivered"`)})
		if !reflect.DeepEqual(ch.Followups, []domain.FollowupWrite{{MessageID: tid(2), State: "delivered"}}) || ch.After.Followups[tid(2)] != "delivered" {
			t.Errorf("followups = %+v", ch.Followups)
		}
	})

	t.Run("detail replace is cut", func(t *testing.T) {
		lim := testLimits
		lim.StepDetailMaxBytes = 40
		ch, _, err := mustApply(t, baseState(), lim, rawOp("replace", "/steps/"+s1+"/detail", `{"log":"`+strings.Repeat("x", 100)+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		if len(ch.UpdateSteps) != 1 || len(ch.UpdateSteps[0].Detail) > 40 || !strings.Contains(string(ch.UpdateSteps[0].Detail), `"truncated":true`) {
			t.Errorf("updates = %+v", ch.UpdateSteps)
		}
	})

	t.Run("the input state is not changed", func(t *testing.T) {
		st := baseState()
		before := renderDoc(t, st)
		_, _, err := mustApply(t, st, testLimits,
			rawOp("replace", "/steps/"+s1+"/status", `"completed"`),
			rawOp("replace", "/followups/"+sid(2)+"/state", `"absorbed"`),
			rawOp("add", "/metrics/tokens_out", `1`))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, renderDoc(t, st)) {
			t.Errorf("Apply changed its input")
		}
	})
}

// renderDoc renders the patchable part of the run document of state the way getRun encodes it,
// decoded with float64 numbers; it is written independently of the package under test.
func renderDoc(t testing.TB, st domain.DocumentState) any {
	t.Helper()
	doc := map[string]any{}
	if st.StatusLine != "" {
		doc["status_line"] = st.StatusLine
	}
	if st.LastTextExcerpt != "" {
		doc["last_text_excerpt"] = st.LastTextExcerpt
	}
	if st.Model != "" {
		doc["model"] = st.Model
	}
	metrics := map[string]any{}
	if len(st.Metrics) > 0 {
		if err := json.Unmarshal(st.Metrics, &metrics); err != nil {
			t.Fatalf("metrics %s: %v", st.Metrics, err)
		}
	}
	doc["metrics"] = metrics
	doc["cost"] = map[string]any{"amount_micros": st.Cost.AmountMicros, "currency": st.Cost.Currency, "is_estimate": st.Cost.IsEstimate}
	if len(st.Plan) > 0 {
		var plan map[string]any
		if err := json.Unmarshal(st.Plan, &plan); err != nil {
			t.Fatalf("plan %s: %v", st.Plan, err)
		}
		if items, ok := plan["items"].([]any); ok && len(items) > 0 {
			doc["plan"] = plan
		}
	}
	steps := map[string]any{}
	for _, s := range st.Steps {
		o := map[string]any{
			"id": s.ID.String(), "ordinal": s.Ordinal, "kind": s.Kind, "name": s.Name, "title": s.Title,
			"status": s.Status, "started_at": s.StartedAt.UTC().Truncate(time.Millisecond).Format(docTime),
		}
		if !s.ParentStepID.IsZero() {
			o["parent_step_id"] = s.ParentStepID.String()
		}
		if !s.FinishedAt.IsZero() {
			o["finished_at"] = s.FinishedAt.UTC().Truncate(time.Millisecond).Format(docTime)
		}
		detail := map[string]any{}
		if s.Detail != nil {
			if err := json.Unmarshal(s.Detail, &detail); err != nil {
				t.Fatalf("detail %s: %v", s.Detail, err)
			}
		}
		o["detail"] = detail
		if _, dup := steps[s.ID.String()]; dup {
			t.Fatalf("step %s twice", s.ID)
		}
		steps[s.ID.String()] = o
	}
	doc["steps"] = steps
	followups := map[string]any{}
	for id, state := range st.Followups {
		followups[id.String()] = map[string]any{"message_id": id.String(), "state": state}
	}
	doc["followups"] = followups
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	out, err := rfcDecode(b)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// applyChange writes ch onto base the way the store does and returns the stored state; it fails
// the test where ch breaks its own contract (unknown update target, duplicate insert).
func applyChange(t testing.TB, base domain.DocumentState, ch domain.Change) domain.DocumentState {
	t.Helper()
	out := base
	for _, c := range []struct {
		col *domain.TextColumn
		dst *string
	}{{&ch.StatusLine, &out.StatusLine}, {&ch.LastTextExcerpt, &out.LastTextExcerpt}, {&ch.Model, &out.Model}} {
		if c.col.Set {
			*c.dst = c.col.Value
		}
	}
	if ch.Metrics != nil {
		out.Metrics = ch.Metrics
	}
	if ch.Plan != nil {
		out.Plan = ch.Plan
	}
	if ch.Cost != nil {
		out.Cost = *ch.Cost
	}
	steps := map[ids.ID]domain.DocStep{}
	for _, s := range base.Steps {
		steps[s.ID] = s
	}
	if !slices.IsSortedFunc(ch.DeleteSteps, func(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) }) {
		t.Errorf("DeleteSteps not ascending")
	}
	for _, id := range ch.DeleteSteps {
		if _, ok := steps[id]; !ok {
			t.Errorf("delete of unknown step %s", id)
		}
		delete(steps, id)
	}
	for i, u := range ch.UpdateSteps {
		if i > 0 && bytes.Compare(ch.UpdateSteps[i-1].ID[:], u.ID[:]) >= 0 {
			t.Errorf("UpdateSteps not strictly ascending")
		}
		s, ok := steps[u.ID]
		if !ok {
			t.Errorf("update of unknown step %s", u.ID)
			continue
		}
		if u.Status == nil && u.Title == nil && u.Detail == nil && u.FinishedAt == nil && u.Ordinal == nil {
			t.Errorf("empty update of %s", u.ID)
		}
		if u.Status != nil {
			s.Status = *u.Status
		}
		if u.Title != nil {
			s.Title = *u.Title
		}
		if u.Detail != nil {
			s.Detail = u.Detail
		}
		if u.FinishedAt != nil {
			s.FinishedAt = *u.FinishedAt
		}
		if u.Ordinal != nil {
			s.Ordinal = *u.Ordinal
		}
		steps[u.ID] = s
	}
	for i, s := range ch.InsertSteps {
		if i > 0 && ch.InsertSteps[i-1].Ordinal >= s.Ordinal {
			t.Errorf("InsertSteps not in ordinal order")
		}
		if _, ok := steps[s.ID]; ok {
			t.Errorf("insert of existing step %s", s.ID)
		}
		steps[s.ID] = s
	}
	out.Steps = slices.SortedFunc(maps.Values(steps), func(a, b domain.DocStep) int { return a.Ordinal - b.Ordinal })
	out.Followups = maps.Clone(base.Followups)
	for _, w := range ch.Followups {
		if _, ok := out.Followups[w.MessageID]; !ok {
			t.Errorf("write of unknown follow-up %s", w.MessageID)
		}
		out.Followups[w.MessageID] = w.State
	}
	return out
}

// checkProperty checks a successful Apply: the normalized ops turn the base document into the
// document of After (RFC 6902), the Change turns the base state into After, ordinals are unique,
// the step limit holds after a step add, and the added steps are inserted.
func checkProperty(t testing.TB, base domain.DocumentState, ops []domain.Op, ch domain.Change, norm []domain.NormOp, lim domain.Limits) {
	t.Helper()
	before := renderDoc(t, base)
	after := renderDoc(t, ch.After)
	got, err := rfcApply(before, norm)
	if err != nil {
		t.Fatalf("normalized ops do not apply to base: %v\nops %s", err, showNorm(norm))
	}
	if !reflect.DeepEqual(got, after) {
		gb, _ := json.Marshal(got)
		ab, _ := json.Marshal(after)
		t.Fatalf("normalized ops give\n%s\nwant\n%s\nops %s", gb, ab, showNorm(norm))
	}
	for _, n := range norm {
		if (n.Op == domain.OpRemove) != (n.Value == nil) {
			t.Errorf("normalized op %s %s has value %s", n.Op, n.Path, n.Value)
		}
	}
	stored := renderDoc(t, applyChange(t, base, ch))
	if !reflect.DeepEqual(stored, after) {
		sb, _ := json.Marshal(stored)
		ab, _ := json.Marshal(after)
		t.Fatalf("change gives\n%s\nwant\n%s", sb, ab)
	}
	seen := map[int]bool{}
	for i, s := range ch.After.Steps {
		if seen[s.Ordinal] || (i > 0 && ch.After.Steps[i-1].Ordinal > s.Ordinal) {
			t.Errorf("ordinal %d twice or out of order", s.Ordinal)
		}
		seen[s.Ordinal] = true
		if s.Detail != nil && len(s.Detail) > lim.StepDetailMaxBytes && !domain.IsSummaryStep(s) {
			stepAdded := false
			for _, in := range ch.InsertSteps {
				stepAdded = stepAdded || in.ID == s.ID
			}
			if stepAdded {
				t.Errorf("detail of step %s is %d bytes", s.ID, len(s.Detail))
			}
		}
	}
	addsStep := false
	for _, o := range ops {
		addsStep = addsStep || o.Target == domain.TargetStep
	}
	if addsStep && len(ch.After.Steps) > lim.MaxStepsPersisted {
		t.Errorf("%d steps after a step add, limit %d", len(ch.After.Steps), lim.MaxStepsPersisted)
	}
	for _, a := range ch.AddedSteps {
		if !slices.ContainsFunc(ch.InsertSteps, func(s domain.DocStep) bool { return reflect.DeepEqual(s, a) }) {
			t.Errorf("added step %s is not inserted as is", a.ID)
		}
		if domain.IsSummaryStep(a) {
			t.Errorf("summary step among the added steps")
		}
	}
	if utf8Len(ch.After.LastTextExcerpt) > lim.ExcerptMaxChars && ch.LastTextExcerpt.Set {
		t.Errorf("excerpt longer than the limit")
	}
}

// utf8Len counts runes.
func utf8Len(s string) int { return len([]rune(s)) }

// gen draws random states and patches for the property tests.
type gen struct {
	r    *rand.Rand
	next int // next fresh id number for steps added by a patch
}

// pick returns one element of xs.
func pick[T any](g *gen, xs ...T) T { return xs[g.r.IntN(len(xs))] }

var (
	stepKinds    = []string{"tool_call", "status", "subagent", "workflow", "background_job"}
	stepStatuses = []string{"pending", "running", "completed", "failed", "cancelled"}
	metricFields = []string{"context_pct", "tokens_in", "tokens_out", "tokens_cache_read", "tokens_cache_write",
		"tokens_cache_write_1h", "subagents", "background_jobs", "api_retries"}
	texts = []string{"", "Reading", "Writing", "Hällo wörld 🙂", "Sam & Robin <x>", "a/b~c"}
)

// text returns a random string, sometimes long and multi-byte, for excerpts and details.
func (g *gen) text() string {
	if g.r.IntN(3) == 0 {
		return strings.Repeat(pick(g, "ab", "ü", "🙂", " ", `"`, "\n"), g.r.IntN(40))
	}
	return pick(g, texts...)
}

// detail returns a random JSON object.
func (g *gen) detail() string {
	m := map[string]any{}
	for i := g.r.IntN(4); i > 0; i-- {
		switch g.r.IntN(4) {
		case 0:
			m[pick(g, "a", "b", "log", "x/y")] = g.text()
		case 1:
			m[pick(g, "n", "count")] = g.r.IntN(1000)
		case 2:
			m["nested"] = map[string]any{"s": g.text(), "list": []any{g.text(), 1.5, true}}
		default:
			m["truncated"] = pick[any](g, true, "no", map[string]any{"z": g.text()})
		}
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// state returns a random run state with steps 100 and up and follow-ups 200 and up.
func (g *gen) state() domain.DocumentState {
	st := domain.DocumentState{
		StatusLine:      pick(g, texts...),
		LastTextExcerpt: pick(g, "", "Hello Alex"),
		Model:           pick(g, "", "model-a"),
		Plan:            json.RawMessage(pick(g, `{}`, `{"items":[]}`, `{"items":[{"id":"a","text":"Reproduce","status":"pending"}]}`)),
		Cost:            domain.Money{AmountMicros: int64(g.r.IntN(3)), Currency: "EUR", IsEstimate: true},
		Followups:       map[ids.ID]string{},
	}
	metrics := map[string]any{}
	for _, f := range metricFields {
		if g.r.IntN(3) == 0 {
			if f == "context_pct" {
				metrics[f] = pick(g, 12.5, 50.0, 0.1)
			} else {
				metrics[f] = g.r.IntN(3)
			}
		}
	}
	if g.r.IntN(3) == 0 {
		metrics["last_api_retry"] = map[string]any{"error": "overloaded", "attempt": 1}
	}
	b, _ := json.Marshal(metrics)
	st.Metrics = b
	ord := 0
	n := g.r.IntN(7)
	summary := -1
	if n > 0 && g.r.IntN(3) == 0 {
		summary = g.r.IntN(n)
	}
	for i := 0; i < n; i++ {
		ord += 1 + g.r.IntN(2)
		s := domain.DocStep{
			ID: tid(100 + i), Ordinal: ord, Kind: pick(g, stepKinds...), Name: "read_file", Title: pick(g, "", "Reading"),
			Status: pick(g, stepStatuses...), Detail: json.RawMessage(`{"i":` + fmt.Sprint(i) + `}`),
			StartedAt: time.Date(2026, 10, 21, 9, i, 0, 0, time.UTC),
		}
		if i > 0 && g.r.IntN(2) == 0 {
			s.ParentStepID = tid(100 + g.r.IntN(i))
		}
		if g.r.IntN(2) == 0 {
			s.FinishedAt = time.Date(2026, 10, 21, 9, i, 30, 0, time.UTC)
		}
		if i == summary {
			s.Kind, s.Name, s.Title, s.Status = "status", domain.SummaryStepName, "", "completed"
			s.Detail = json.RawMessage(`{"summarized":{"by_kind":{"tool_call":2},"count":2}}`)
		}
		st.Steps = append(st.Steps, s)
	}
	for i := g.r.IntN(4); i > 0; i-- {
		st.Followups[tid(200+i)] = pick(g, "queued", "delivered", "absorbed", "deferred")
	}
	return st
}

// ops returns a random valid patch on st; targets mostly exist so that Apply mostly succeeds.
func (g *gen) ops(st domain.DocumentState) []domain.RawOp {
	var known []string
	for _, s := range st.Steps {
		known = append(known, s.ID.String())
	}
	var followups []string
	for id := range st.Followups {
		followups = append(followups, id.String())
	}
	slices.Sort(followups)
	var out []domain.RawOp
	str := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	for i := 1 + g.r.IntN(6); i > 0; i-- {
		switch g.r.IntN(12) {
		case 0:
			out = append(out, rawOp(pick(g, "add", "replace"), "/status_line", str(pick(g, texts...))))
		case 1:
			out = append(out, rawOp("remove", "/status_line", ""))
		case 2:
			out = append(out, rawOp(pick(g, "add", "replace"), "/last_text_excerpt", str(g.text())))
		case 3:
			out = append(out, rawOp("replace", "/model", str(pick(g, "", "model-a", "model-b"))))
		case 4:
			f := pick(g, metricFields...)
			v := fmt.Sprint(g.r.IntN(3))
			if f == "context_pct" {
				v = pick(g, "12.5", "50", "0.1", "100")
			}
			out = append(out, rawOp(pick(g, "add", "replace"), "/metrics/"+f, v))
		case 5:
			if g.r.IntN(2) == 0 {
				out = append(out, rawOp("remove", "/metrics/last_api_retry", ""))
			} else {
				out = append(out, rawOp(pick(g, "add", "replace"), "/metrics/last_api_retry",
					pick(g, `{"attempt":1,"error":"overloaded"}`, `{"attempt":2,"max_retries":5,"retry_delay_ms":4000}`)))
			}
		case 6:
			out = append(out, rawOp("replace", "/cost", fmt.Sprintf(`{"amount_micros":%d,"currency":"%s","is_estimate":true}`, g.r.IntN(3), pick(g, "EUR", "USD"))))
		case 7:
			if g.r.IntN(3) == 0 {
				out = append(out, rawOp("remove", "/plan", ""))
			} else {
				out = append(out, rawOp(pick(g, "add", "replace"), "/plan", pick(g, `{"items":[]}`,
					`{"items":[{"id":"a","text":"Reproduce","status":"pending"}]}`,
					`{"items":[{"id":"a","text":"Reproduce","status":"completed"},{"id":"b","text":"Fix","status":"in_progress"}]}`)))
			}
		case 8, 9:
			id := sid(g.next)
			g.next++
			v := map[string]any{"kind": pick(g, stepKinds...), "name": "run"}
			if g.r.IntN(2) == 0 {
				v["status"] = pick(g, stepStatuses...)
			}
			if g.r.IntN(2) == 0 {
				v["detail"] = json.RawMessage(g.detail())
			}
			if len(known) > 0 && g.r.IntN(2) == 0 {
				v["parent_step_id"] = pick(g, known...)
			}
			if g.r.IntN(2) == 0 {
				v["started_at"] = "2026-10-21T08:00:00.987654321Z"
			}
			if g.r.IntN(3) == 0 {
				v["finished_at"] = "2026-10-21T08:00:01Z"
			}
			if g.r.IntN(3) == 0 {
				v["title"] = "Sam"
			}
			b, _ := json.Marshal(v)
			out = append(out, rawOp("add", "/steps/"+id, string(b)))
			known = append(known, id)
		case 10:
			if len(known) == 0 {
				continue
			}
			p := "/steps/" + pick(g, known...)
			switch g.r.IntN(4) {
			case 0:
				out = append(out, rawOp("replace", p+"/status", str(pick(g, stepStatuses...))))
			case 1:
				out = append(out, rawOp("replace", p+"/title", str(pick(g, "", "Reading", "Done"))))
			case 2:
				out = append(out, rawOp("replace", p+"/detail", g.detail()))
			default:
				out = append(out, rawOp("replace", p+"/finished_at", str(pick(g, "2026-10-21T09:00:30Z", "2026-10-21T11:00:00.5555Z"))))
			}
		default:
			if len(followups) == 0 {
				continue
			}
			out = append(out, rawOp("replace", "/followups/"+pick(g, followups...)+"/state", str(pick(g, "delivered", "absorbed"))))
		}
	}
	return out
}

// TestNormalizedOpsApplyToBase is the property test of Design note 4: for random valid patches on
// random states, the normalized ops applied with an independent RFC 6902 applier to the document
// at the base version give the document at the new version, and the Change writes the same state.
func TestNormalizedOpsApplyToBase(t *testing.T) {
	t.Parallel()
	for seed := uint64(1); seed <= 6; seed++ {
		g := &gen{r: rand.New(rand.NewPCG(seed, seed*7919)), next: 300} //nolint:gosec // a fixed seed makes the property test reproducible
		applied, summarized, multi := 0, 0, 0
		const rounds = 400
		for i := 0; i < rounds; i++ {
			st := g.state()
			raws := g.ops(st)
			if len(raws) == 0 {
				continue
			}
			lim := domain.Limits{
				ExcerptMaxChars:    1 + g.r.IntN(10),
				StepDetailMaxBytes: 18 + g.r.IntN(60),
				MaxStepsPersisted:  2 + g.r.IntN(7),
			}
			ops, fieldErrs := domain.ParseOps(raws)
			if len(fieldErrs) != 0 {
				t.Fatalf("seed %d round %d: generated patch invalid: %v", seed, i, fieldErrs)
			}
			ch, norm, err := domain.Apply(st, ops, lim, testEnv(5000+i*10))
			if err != nil {
				fields := patchErrFields(t, err)
				if len(fields) != 1 || fields[0].code != "limit_reached" {
					t.Fatalf("seed %d round %d: %v", seed, i, fields)
				}
				continue
			}
			applied++
			if touchedSummary(ch) {
				summarized++
			}
			if len(norm) > 1 {
				multi++
			}
			checkProperty(t, st, ops, ch, norm, lim)
		}
		if applied < rounds/2 || summarized < 20 || multi < rounds/4 {
			t.Errorf("seed %d: weak coverage: applied %d, summarized %d, multi-op %d", seed, applied, summarized, multi)
		}
	}
}

// touchedSummary reports whether the change creates or updates the summary step.
func touchedSummary(ch domain.Change) bool {
	for _, s := range ch.After.Steps {
		if !domain.IsSummaryStep(s) {
			continue
		}
		if slices.ContainsFunc(ch.InsertSteps, func(in domain.DocStep) bool { return in.ID == s.ID }) ||
			slices.ContainsFunc(ch.UpdateSteps, func(u domain.StepUpdate) bool { return u.ID == s.ID }) {
			return true
		}
	}
	return false
}

// TestNormalizedTimesMatchTheAPI checks that the times in normalized ops have the form getRun
// answers (time.Time as JSON after the cut to milliseconds), so a client that applies the ops to
// its snapshot ends with exactly the document of getRun, also when the milliseconds end in zero.
func TestNormalizedTimesMatchTheAPI(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 21, 10, 0, 0, 990_400_000, time.UTC)
	finished := time.Date(2026, 10, 21, 10, 0, 1, 0, time.UTC)
	step := tid(50)
	ops, fieldErrs := domain.ParseOps([]domain.RawOp{
		{Op: "add", Path: "/steps/" + step.String(), Value: json.RawMessage(`{"kind":"tool_call","name":"read"}`)},
		{Op: "replace", Path: "/steps/" + step.String() + "/finished_at", Value: json.RawMessage(`"2026-10-21T10:00:01.000Z"`)},
	})
	if len(fieldErrs) != 0 {
		t.Fatalf("ParseOps: %v", fieldErrs)
	}
	n := 900
	_, norm, err := domain.Apply(baseState(), ops, testLimits, domain.Env{Now: now, NewID: func() ids.ID { n++; return tid(n) }})
	if err != nil {
		t.Fatal(err)
	}
	api := func(t time.Time) string {
		b, _ := json.Marshal(t.UTC().Truncate(time.Millisecond))
		return string(b)
	}
	var added struct {
		StartedAt json.RawMessage `json:"started_at"`
	}
	if len(norm) != 2 || json.Unmarshal(norm[0].Value, &added) != nil {
		t.Fatalf("normalized ops %v", norm)
	}
	if string(added.StartedAt) != api(now) {
		t.Errorf("started_at %s, want %s as getRun answers", added.StartedAt, api(now))
	}
	if string(norm[1].Value) != api(finished) {
		t.Errorf("finished_at %s, want %s as getRun answers", norm[1].Value, api(finished))
	}
}
