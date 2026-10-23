// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// AnswerRun is the part of a runs row the answer check reads (M2-19 design note 4): the run an
// agent names in MessageCreate.run_id.
type AnswerRun struct {
	ID, ThreadID, AgentID ids.ID
	State                 State
}

// The verdicts of AnswerVerdict other than nil.
var (
	// ErrAnswerRunMismatch: the run is unknown, belongs to another thread or to another agent.
	// The caller answers 400 validation_failed with pointer /run_id and errors[].code
	// run_mismatch, with one detail for every case, so run_id cannot probe runs outside the
	// authorized thread (05 rule 1.2).
	ErrAnswerRunMismatch = errors.New("runs: run_id does not name a run of the calling agent in this thread")
	// ErrAnswerRunNotActive: the own run of this thread is in an end state; the caller answers
	// 409 conflict with reason run_not_active.
	ErrAnswerRunNotActive = errors.New("runs: the run of run_id has ended")
)

// AnswerVerdict decides whether an agent message of agentID in threadID may name run (found is
// false for an unknown id). Ownership and thread come first, the state second: only the own run
// of this thread can be ErrAnswerRunNotActive. Active means queued, running or waiting_input
// (State.Active, R-DM-14). It returns nil for an own active run.
//
// Example:
//
//	domain.AnswerVerdict(run, true, run.AgentID, run.ThreadID) // nil while run.State is running
func AnswerVerdict(run AnswerRun, found bool, agentID, threadID ids.ID) error {
	if !found || run.ThreadID != threadID || run.AgentID != agentID {
		return ErrAnswerRunMismatch
	}
	if !run.State.Active() {
		return ErrAnswerRunNotActive
	}
	return nil
}
