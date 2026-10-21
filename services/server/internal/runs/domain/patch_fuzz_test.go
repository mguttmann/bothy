// SPDX-License-Identifier: Apache-2.0

package domain_test

// The fuzz test of ParseOps and Apply on a fixed small run.

import (
	"encoding/json"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// fuzzState is the fixed small run of FuzzParseAndApply: steps tid(1) to tid(4) with a parent, a
// summary step, a follow-up tid(7) and a plan.
func fuzzState() domain.DocumentState {
	st := baseState()
	st.Plan = json.RawMessage(`{"items":[{"id":"a","text":"Reproduce","status":"pending"}]}`)
	st.Metrics = json.RawMessage(`{"tokens_in":5,"context_pct":12.5,"last_api_retry":{"attempt":1}}`)
	st.Steps = []domain.DocStep{
		mkStep(1, 1, "tool_call", "completed", 0),
		mkStep(2, 2, "subagent", "running", 1),
		{ID: tid(3), Ordinal: 4, Kind: "status", Name: domain.SummaryStepName, Status: "completed",
			Detail: json.RawMessage(`{"summarized":{"by_kind":{"tool_call":1},"count":1}}`), StartedAt: testNow},
		mkStep(4, 5, "workflow", "failed", 0),
	}
	st.Followups[tid(7)] = "queued"
	return st
}

// fuzzOp is the JSON form of one fuzzed operation.
type fuzzOp struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

// FuzzParseAndApply feeds random patches (a JSON array of operations) through ParseOps and Apply
// on a fixed small run: neither panics, and a patch either fails with field errors or yields a
// change whose normalized ops satisfy the property of TestNormalizedOpsApplyToBase.
func FuzzParseAndApply(f *testing.F) {
	s1, s2, s9, m := sid(1), sid(2), sid(9), sid(7)
	seeds := []string{
		`[{"op":"replace","path":"/status_line","value":"Reading"}]`,
		`[{"op":"remove","path":"/status_line"},{"op":"add","path":"/last_text_excerpt","value":"Hällo 🙂 wörld"}]`,
		`[{"op":"replace","path":"/model","value":"model-a"},{"op":"add","path":"/metrics/tokens_out","value":1e2}]`,
		`[{"op":"replace","path":"/metrics/context_pct","value":99.5},{"op":"remove","path":"/metrics/last_api_retry"}]`,
		`[{"op":"replace","path":"/cost","value":{"amount_micros":5,"currency":"EUR","is_estimate":true}}]`,
		`[{"op":"replace","path":"/plan","value":{"items":[]}},{"op":"add","path":"/plan","value":{"items":[{"id":"b","text":"Fix","status":"completed"}]}}]`,
		`[{"op":"add","path":"/steps/` + s9 + `","value":{"kind":"tool_call","name":"run","status":"completed","parent_step_id":"` + s2 + `","detail":{"log":"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}}}]`,
		`[{"op":"replace","path":"/steps/` + s1 + `/status","value":"failed"},{"op":"replace","path":"/steps/` + s2 + `/finished_at","value":"2026-10-21T10:00:00.5Z"}]`,
		`[{"op":"replace","path":"/steps/` + s1 + `/detail","value":{"a":[1,"b",{"c":"d"}]}},{"op":"replace","path":"/steps/` + s1 + `/title","value":"Robin"}]`,
		`[{"op":"replace","path":"/followups/` + m + `/state","value":"absorbed"}]`,
		`[{"op":"add","path":"/steps/` + s1 + `","value":{"kind":"status","name":"x"}}]`,
		`[{"op":"replace","path":"/state","value":"running"},{"op":"add","path":"/steps/x","value":null}]`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	lim := domain.Limits{ExcerptMaxChars: 8, StepDetailMaxBytes: 48, MaxStepsPersisted: 4}
	f.Fuzz(func(t *testing.T, patch string) {
		var in []fuzzOp
		if json.Unmarshal([]byte(patch), &in) != nil || len(in) > 64 {
			return
		}
		raws := make([]domain.RawOp, len(in))
		for i, o := range in {
			raws[i] = domain.RawOp{Op: o.Op, Path: o.Path, Value: o.Value}
		}
		ops, fieldErrs := domain.ParseOps(raws)
		if len(fieldErrs) != 0 {
			if ops != nil {
				t.Fatalf("ops with field errors")
			}
			for _, e := range fieldErrs {
				if e.Pointer == "" || e.Code == "" || e.Detail == "" {
					t.Fatalf("incomplete field error %+v", e)
				}
			}
			return
		}
		st := fuzzState()
		ch, norm, err := domain.Apply(st, ops, lim, testEnv(900))
		if err != nil {
			e, ok := errs.As(err)
			if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 {
				t.Fatalf("Apply error %v", err)
			}
			return
		}
		checkProperty(t, st, ops, ch, norm, lim)
	})
}
