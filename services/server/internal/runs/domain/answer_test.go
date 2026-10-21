// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestAnswerVerdict: the verdict of a run named by an agent's answer (M2-19 design note 4):
// ownership and thread first, the state second.
func TestAnswerVerdict(t *testing.T) {
	t.Parallel()
	agent, other, thread, elsewhere := ids.New(), ids.New(), ids.New(), ids.New()
	own := func(s domain.State) domain.AnswerRun {
		return domain.AnswerRun{ID: ids.New(), ThreadID: thread, AgentID: agent, State: s}
	}
	cases := []struct {
		name  string
		run   domain.AnswerRun
		found bool
		want  error
	}{
		{"not found", domain.AnswerRun{}, false, domain.ErrAnswerRunMismatch},
		{"another thread", domain.AnswerRun{ID: ids.New(), ThreadID: elsewhere, AgentID: agent, State: domain.StateRunning}, true, domain.ErrAnswerRunMismatch},
		{"another agent", domain.AnswerRun{ID: ids.New(), ThreadID: thread, AgentID: other, State: domain.StateRunning}, true, domain.ErrAnswerRunMismatch},
		{"queued", own(domain.StateQueued), true, nil},
		{"running", own(domain.StateRunning), true, nil},
		{"waiting_input", own(domain.StateWaitingInput), true, nil},
		{"completed", own(domain.StateCompleted), true, domain.ErrAnswerRunNotActive},
		{"failed", own(domain.StateFailed), true, domain.ErrAnswerRunNotActive},
		{"cancelled", own(domain.StateCancelled), true, domain.ErrAnswerRunNotActive},
		{"ended run of another agent", domain.AnswerRun{ID: ids.New(), ThreadID: thread, AgentID: other, State: domain.StateCompleted}, true,
			domain.ErrAnswerRunMismatch},
		{"ended run in another thread", domain.AnswerRun{ID: ids.New(), ThreadID: elsewhere, AgentID: agent, State: domain.StateFailed}, true,
			domain.ErrAnswerRunMismatch},
	}
	for _, c := range cases {
		if got := domain.AnswerVerdict(c.run, c.found, agent, thread); !errors.Is(got, c.want) || (c.want == nil) != (got == nil) {
			t.Errorf("%s: verdict %v, want %v", c.name, got, c.want)
		}
	}
}
