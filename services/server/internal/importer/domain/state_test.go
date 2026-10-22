// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/importer/domain"
)

// TestStateMachine checks every pair of state and operation of design note 3.
func TestStateMachine(t *testing.T) {
	t.Parallel()
	type want struct {
		next     string
		allowed  bool
		noEffect bool
	}
	refused := func(s string) want { return want{next: s} }
	cases := map[domain.Op]map[string]want{
		domain.OpStart: {
			domain.StateDraft: {domain.StateRunning, true, false}, domain.StatePaused: {domain.StateRunning, true, false},
			domain.StateRunning: {domain.StateRunning, true, true}, domain.StateCompleted: refused(domain.StateCompleted),
			domain.StateFailed: refused(domain.StateFailed),
		},
		domain.OpPause: {
			domain.StateRunning: {domain.StatePaused, true, false}, domain.StatePaused: {domain.StatePaused, true, true},
			domain.StateDraft: refused(domain.StateDraft), domain.StateCompleted: refused(domain.StateCompleted),
			domain.StateFailed: refused(domain.StateFailed),
		},
		domain.OpCancel: {
			domain.StateDraft: {domain.StateFailed, true, false}, domain.StateRunning: {domain.StateFailed, true, false},
			domain.StatePaused: {domain.StateFailed, true, false}, domain.StateCompleted: refused(domain.StateCompleted),
			domain.StateFailed: refused(domain.StateFailed),
		},
		domain.OpComplete: {
			domain.StateRunning: {domain.StateCompleted, true, false}, domain.StatePaused: {domain.StateCompleted, true, false},
			domain.StateDraft: refused(domain.StateDraft), domain.StateCompleted: refused(domain.StateCompleted),
			domain.StateFailed: refused(domain.StateFailed),
		},
	}
	for op, states := range cases {
		for state, w := range states {
			got := domain.Transition(state, op)
			if got.Next != w.next || got.Allowed != w.allowed || got.NoEffect != w.noEffect {
				t.Errorf("%s on %s = %+v, want %+v", op, state, got, w)
			}
		}
	}
	if !domain.SetsStartedAt(domain.OpStart) || domain.SetsStartedAt(domain.OpPause) {
		t.Error("only start sets started_at")
	}
	if !domain.SetsFinishedAt(domain.OpCancel) || !domain.SetsFinishedAt(domain.OpComplete) || domain.SetsFinishedAt(domain.OpStart) {
		t.Error("cancel and complete set finished_at")
	}
	if !domain.Ended(domain.StateCompleted) || !domain.Ended(domain.StateFailed) || domain.Ended(domain.StatePaused) {
		t.Error("completed and failed are ended")
	}
}
