// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// planEnds are the ends of TestPlanRunEnd and the seeds of FuzzPlanRunEnd.
var planEnds = []domain.RunEnd{
	{State: domain.StateCompleted},
	{State: domain.StateFailed, Reason: "stale"},
	{State: domain.StateFailed, Reason: "interrupted"},
	{State: domain.StateFailed, Reason: "runtime_error"},
	{State: domain.StateFailed, Reason: "agent_unavailable"},
	{State: domain.StateCancelled, Reason: "user_cancelled"},
	{State: domain.StateCancelled, Reason: "cancel_timeout"},
	{State: domain.StateCancelled, Reason: "access_revoked"},
	{State: domain.StateCancelled, Reason: "agent_disabled"},
	{State: domain.StateCancelled, Reason: "channel_read_only"},
	{State: domain.StateCancelled, Reason: "user_disabled"},
}

// openSet returns four open follow-ups in shuffled order: queued and delivered rows, one of them
// with a deleted message.
func openSet() (open []domain.OpenFollowup, deleted ids.ID, alive []ids.ID) {
	a, b, c, d := ids.New(), ids.New(), ids.New(), ids.New()
	open = []domain.OpenFollowup{
		{MessageID: c, ThreadSeq: 30, State: domain.FollowupDelivered},
		{MessageID: a, ThreadSeq: 10, State: domain.FollowupQueued},
		{MessageID: d, ThreadSeq: 40, State: domain.FollowupQueued, Deleted: true},
		{MessageID: b, ThreadSeq: 20, State: domain.FollowupQueued},
	}
	return open, d, []ids.ID{a, b, c}
}

// TestPlanRunEnd: the deleted follow-up is always in Deleted; user_cancelled puts all others into
// Stopped; every other end puts them into Candidates by ascending thread_seq; an empty set gives
// an empty plan; a state that is no end is an error.
func TestPlanRunEnd(t *testing.T) {
	t.Parallel()
	for _, end := range planEnds {
		open, deleted, alive := openSet()
		plan, err := domain.PlanRunEnd(end, open)
		if err != nil {
			t.Fatalf("%v: %v", end, err)
		}
		if !slices.Equal(plan.Deleted, []ids.ID{deleted}) {
			t.Errorf("%v: Deleted %v", end, plan.Deleted)
		}
		if end.State == domain.StateCancelled && end.Reason == "user_cancelled" {
			if len(plan.Candidates) != 0 || len(plan.Stopped) != 3 {
				t.Errorf("%v: plan %+v", end, plan)
			}
			continue
		}
		if len(plan.Stopped) != 0 || !slices.Equal(plan.Candidates, alive) {
			t.Errorf("%v: plan %+v, want candidates %v", end, plan, alive)
		}
	}
	plan, err := domain.PlanRunEnd(domain.RunEnd{State: domain.StateCompleted}, nil)
	if err != nil || len(plan.Deleted)+len(plan.Stopped)+len(plan.Candidates) != 0 {
		t.Fatalf("empty set: %+v %v", plan, err)
	}
	for _, st := range []domain.State{domain.StateQueued, domain.StateRunning, domain.StateWaitingInput, "unknown"} {
		if _, err := domain.PlanRunEnd(domain.RunEnd{State: st}, nil); err == nil {
			t.Errorf("state %s: no error", st)
		}
	}
}
