// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"cmp"
	"errors"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// RunEnd is the end state and reason of a run whose open follow-ups are settled.
type RunEnd struct {
	State  State
	Reason string
}

// OpenFollowup is one follow-up in queued or delivered at the end of its run, with the facts of
// its message.
type OpenFollowup struct {
	MessageID ids.ID
	// ThreadSeq is the position of the message in its thread (04 section 4).
	ThreadSeq int64
	State     FollowupState
	// Deleted reports a deleted message (a tombstone).
	Deleted bool
}

// RunEndPlan is what the end of a run does with its open follow-ups (09 section 7).
type RunEndPlan struct {
	// Deleted are the follow-ups whose message is deleted: discarded, trigger_state unchanged.
	Deleted []ids.ID
	// Stopped are the follow-ups of a run a person stopped: discarded, trigger_state cancelled
	// with user_cancelled.
	Stopped []ids.ID
	// Candidates are the follow-ups that become deferred, in ascending thread_seq: the first
	// allowed one triggers the follow-on run, the rest become its follow-ups.
	Candidates []ids.ID
}

// errNotEnded is the error of PlanRunEnd for a state that is no end state.
var errNotEnded = errors.New("runs: PlanRunEnd needs an end state")

// PlanRunEnd splits the open follow-ups of a run that ended with end (09 section 7 and 8 point 5;
// ruling D-M2-11-5): a deleted message goes to Deleted; after cancelled with user_cancelled every
// other one goes to Stopped; after every other end they go to Candidates by ascending thread_seq
// (ties by message id). open is not changed.
//
// Errors: errNotEnded when end.State is not completed, failed or cancelled.
//
// Example:
//
//	plan, err := domain.PlanRunEnd(domain.RunEnd{State: domain.StateCompleted}, open)
func PlanRunEnd(end RunEnd, open []OpenFollowup) (RunEndPlan, error) {
	if !end.State.Ended() {
		return RunEndPlan{}, errNotEnded
	}
	stop := end.State == StateCancelled && end.Reason == RunReasonUserCancelled
	var plan RunEndPlan
	var rest []OpenFollowup
	for _, o := range open {
		switch {
		case o.Deleted:
			plan.Deleted = append(plan.Deleted, o.MessageID)
		case stop:
			plan.Stopped = append(plan.Stopped, o.MessageID)
		default:
			rest = append(rest, o)
		}
	}
	slices.SortFunc(rest, func(a, b OpenFollowup) int {
		if c := cmp.Compare(a.ThreadSeq, b.ThreadSeq); c != 0 {
			return c
		}
		return slices.Compare(a.MessageID[:], b.MessageID[:])
	})
	for _, o := range rest {
		plan.Candidates = append(plan.Candidates, o.MessageID)
	}
	return plan, nil
}
