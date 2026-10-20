// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestTriggerStateMapping checks the mapping of 09 6 (brief note 4) for every pair of states:
// the run events that move trigger_state and the ones that leave it unchanged.
func TestTriggerStateMapping(t *testing.T) {
	if got := domain.TriggerOnOpen(); got != (domain.TriggerChange{State: domain.TriggerStateReceived}) {
		t.Errorf("TriggerOnOpen() = %v, want received without reason", got)
	}
	q, r, w := domain.StateQueued, domain.StateRunning, domain.StateWaitingInput
	c, f, x := domain.StateCompleted, domain.StateFailed, domain.StateCancelled
	working := domain.TriggerChange{State: domain.TriggerStateWorking}
	done := domain.TriggerChange{State: domain.TriggerStateDone}
	failed := domain.TriggerChange{State: domain.TriggerStateFailed, Reason: "stale"}
	cancelled := domain.TriggerChange{State: domain.TriggerStateCancelled, Reason: "stale"}
	want := map[[2]domain.State]domain.TriggerChange{
		{q, r}: working, {w, r}: working,
		{r, c}: done,
		{q, f}: failed, {r, f}: failed, {w, f}: failed,
		{q, x}: cancelled, {r, x}: cancelled, {w, x}: cancelled,
	}
	for _, from := range domain.States() {
		for _, to := range domain.States() {
			got, ok := domain.TriggerOnTransition(from, to, "stale")
			exp, wantOK := want[[2]domain.State{from, to}]
			if ok != wantOK || got != exp {
				t.Errorf("TriggerOnTransition(%s, %s) = %v, %v; want %v, %v", from, to, got, ok, exp, wantOK)
			}
		}
	}
	// running -> waiting_input is allowed but leaves trigger_state at working.
	if !domain.CanTransition(r, w) {
		t.Fatal("running -> waiting_input must be an allowed transition")
	}
	// Without a reason the change to working or done carries none, whatever the caller passes.
	if got, _ := domain.TriggerOnTransition(q, r, ""); got != working {
		t.Errorf("queued -> running without reason = %v", got)
	}
}

// triggerStateConstants parses the package files that declare TriggerState constants and returns
// their non-empty values, so a constant added without a row in 09 6 fails the test.
func triggerStateConstants(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, name := range []string{"trigger_reasons.go", "triggerstate.go"} {
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				if id, ok := vs.Type.(*ast.Ident); !ok || id.Name != "TriggerState" {
					continue
				}
				for _, v := range vs.Values {
					s, err := strconv.Unquote(v.(*ast.BasicLit).Value)
					if err != nil {
						t.Fatal(err)
					}
					if s != "" {
						out = append(out, s)
					}
				}
			}
		}
	}
	return out
}

// TestTriggerStatesMatchSpec compares the TriggerState constants with the rows of the table of
// 09 6 of this repository: every constant is a row and every row is a constant.
func TestTriggerStatesMatchSpec(t *testing.T) {
	var spec []string
	for _, cells := range specTable(t, "## 6. ") {
		spec = append(spec, cells[0])
	}
	consts := triggerStateConstants(t)
	sort.Strings(spec)
	sort.Strings(consts)
	if strings.Join(spec, ",") != strings.Join(consts, ",") {
		t.Fatalf("09 6 states %v differ from the TriggerState constants %v", spec, consts)
	}
}
