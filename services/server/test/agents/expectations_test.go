// SPDX-License-Identifier: Apache-2.0

package agents_test

import (
	"context"
	"strings"
	"testing"
)

// fakeRows is the database of the unit tests: every query returns the same rows.
type fakeRows struct{ all []map[string]any }

func (f fakeRows) rows(context.Context, string, ...any) ([]map[string]any, error) {
	out := make([]map[string]any, len(f.all))
	for i, r := range f.all {
		c := map[string]any{}
		for k, v := range r {
			c[k] = v
		}
		out[i] = c
	}
	return out, nil
}

// canaryJWS has the form of a compact JWS; no failure message may show it.
const canaryJWS = "canaryheadercanary.canarypayloadcanary.canarysignaturecanary"

// unitConductor is a conductor without a server: kept values and a fake database.
func unitConductor(t *testing.T, rows ...map[string]any) *conductor {
	return &conductor{t: t, sc: &Scenario{ID: "AG-00"}, vars: map[string]any{"tok": canaryJWS, "r1": "run-1"}, db: fakeRows{all: rows},
		asyncs: map[string]chan answer{}}
}

// TestExpectations_Bite (brief Design note 14 b): every kind of expectation fails a step whose
// recorded reply or stored row differs, with a message that names the step, the expected and
// the observed value and holds no token.
func TestExpectations_Bite(t *testing.T) {
	t.Parallel()
	reply := answer{
		"status": float64(409), "code": "conflict", "reason": "run_not_active", "event": "agent.invocation", "verdict": "valid",
		"errors": []any{map[string]any{"code": "patch_path_forbidden", "pointer": "/ops/0"}}, "token": canaryJWS, "run_id": "run-1",
	}
	count := func(n int) *int { return &n }
	for name, e := range map[string]Expect{
		"status":  {Status: 200},
		"code":    {Code: "already_started"},
		"reason":  {Reason: "superseded"},
		"errors":  {Errors: []FieldErr{{Code: "patch_path_forbidden", Pointer: "/ops/1"}}},
		"event":   {Fields: map[string]any{"event": "agent.followup"}},
		"verdict": {Fields: map[string]any{"verdict": "expired"}},
		"none":    {None: true},
		"one_of":  {OneOf: []Expect{{Status: 200}, {Code: "already_started"}}},
		"kept":    {Fields: map[string]any{"run_id": "$tok"}},
		"absent":  {Fields: map[string]any{"token": "<absent>"}},
		"present": {Fields: map[string]any{"interaction_id": "<present>"}},
		"at most": {Fields: map[string]any{"status": "<=400"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := unitConductor(t)
			ch := make(chan answer, 1)
			ch <- reply
			c.asyncs["call"] = ch
			err := c.stepAt("AG-00 case bite step 3", &Step{Await: "call", Expect: &e})
			assertBites(t, err)
		})
	}
	row := map[string]any{"id": "run-1", "state": "running", "state_reason": nil, "version": float64(4), "invocation_jti": "jti-1",
		"details": map[string]any{"stage": "delivery"}, "payload": map[string]any{"token_like": canaryJWS}}
	for _, kind := range checkKinds {
		t.Run("check "+kind, func(t *testing.T) {
			t.Parallel()
			c := unitConductor(t, row)
			st := &Step{Check: kind, Expect: &Expect{Fields: map[string]any{"state": "cancelled"}}}
			if !singleCheck(kind) {
				st.Expect = &Expect{Count: count(2)}
			} else {
				st.ID = "$r1"
			}
			assertBites(t, c.stepAt("AG-00 case bite step 3", st))
			rows := &Step{Check: kind, Where: map[string]any{"state": "running"}, Expect: &Expect{Rows: []map[string]any{{"version": float64(5)}}}}
			if !singleCheck(kind) {
				assertBites(t, c.stepAt("AG-00 case bite step 3", rows))
			}
		})
	}
	t.Run("tally", func(t *testing.T) {
		t.Parallel()
		c := unitConductor(t)
		err := c.parallel(&Step{
			Parallel: []Step{{Await: "a"}, {Await: "b"}},
			Tally:    []Tally{{Match: Expect{Status: 200}, Count: 1}},
		})
		if err == nil {
			t.Fatal("a tally of steps that are not calls passed")
		}
	})
}

// singleCheck reports whether a check kind names one row by id.
func singleCheck(kind string) bool {
	for _, k := range singleChecks {
		if k == kind {
			return true
		}
	}
	return false
}

// assertBites requires a failure that names the step, says expected and observed, and holds no
// token.
func assertBites(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("the wrong expectation passed")
	}
	msg := err.Error()
	if !strings.Contains(msg, "AG-00 case bite step 3") || !strings.Contains(msg, "expected") {
		t.Fatalf("the message does not name step and expectation: %s", msg)
	}
	if !strings.Contains(msg, "observed") && !strings.Contains(msg, "no alternative") {
		t.Fatalf("the message does not name the observed value: %s", msg)
	}
	if strings.Contains(msg, canaryJWS) || strings.Contains(msg, "canarysignaturecanary") {
		t.Fatalf("the message shows a token: %s", msg)
	}
}
