// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// CancelOutcome is what cancelRun does to a run in a given state.
type CancelOutcome uint8

// The outcomes of CancelDecision (brief M2-09 notes 7b to 7d). The zero value is invalid.
const (
	// CancelImmediate: the run is queued; it becomes cancelled with user_cancelled at once, and
	// run.cancel_requested is written as well, because no delivery marker tells whether an adapter
	// holds the invocation.
	CancelImmediate CancelOutcome = iota + 1
	// CancelRequest: the run is running or waiting_input; write run.cancel_requested and schedule
	// runs.cancel_timeout after agent.cancel_grace, the state stays.
	CancelRequest
	// CancelNoChange: the run is already cancelled; 202 without change.
	CancelNoChange
	// CancelInvalid: the run is completed or failed; 409 invalid_state_transition.
	CancelInvalid
)

// CancelDecision returns the outcome of cancelRun for a run in state current. An unknown state is
// CancelInvalid, so nothing is written for a row outside the state machine.
func CancelDecision(current State) CancelOutcome {
	//exhaustive:ignore // Every other state is outside the cancel rules (CancelInvalid after the switch).
	switch current {
	case StateQueued:
		return CancelImmediate
	case StateRunning, StateWaitingInput:
		return CancelRequest
	case StateCancelled:
		return CancelNoChange
	}
	return CancelInvalid
}

// CancelReasonFor returns the reason of the cancel requests that a rights change with the audit
// action action causes (brief M2-09 note 8c): user.disable gives user_disabled (08 11 point 7,
// 02 4.3), channel.archive and space.archive give channel_read_only (09 5.2 "read-only or
// archived"), every other action gives access_revoked.
//
// Example:
//
//	domain.CancelReasonFor("grant.revoke") // "access_revoked"
func CancelReasonFor(action string) string {
	switch action {
	case "user.disable":
		return RunReasonUserDisabled
	case "channel.archive", "space.archive":
		return RunReasonChannelReadOnly
	}
	return RunReasonAccessRevoked
}

// The kinds of AccessTarget, the values of authorize.PrincipalAccessChange.
const (
	accessKindSpace   = "space"
	accessKindChannel = "channel"
)

// levelTrigger is the level a person needs to start a run (07 2.3); the levels read < write <
// trigger, so any other level, or none, is a loss.
const levelTrigger = "trigger"

// AccessTarget is one entry of an access diff: a domain-local copy of the target of
// authorize.PrincipalAccessChange, because this package must not import kernel/authorize; runs/app
// converts it.
type AccessTarget struct {
	// Kind is "space", "channel" or "host".
	Kind string
	// ID is the space, channel or host.
	ID ids.ID
	// Level is the new level ("read", "write", "trigger"); "" when absent.
	Level string
}

// AccessChange is the access diff of one principal (domain-local copy of
// authorize.PrincipalAccessChange).
type AccessChange struct {
	// PrincipalID is the principal whose access changed.
	PrincipalID ids.ID
	// Removed lists the targets the principal lost entirely.
	Removed []AccessTarget
	// Changed lists the targets whose level or capabilities changed; Level is the new level.
	Changed []AccessTarget
}

// LiveRun is the part of a run row that revocation and the stall check read; ActiveRun of the
// trigger decision is only the run and agent ids. Zero times stand for NULL.
type LiveRun struct {
	// ID is the run.
	ID ids.ID
	// AgentID is runs.agent_id.
	AgentID ids.ID
	// TriggeredBy is runs.triggered_by, the person who started the run.
	TriggeredBy ids.ID
	// ThreadID is runs.thread_id.
	ThreadID ids.ID
	// ChannelID and SpaceID are the thread's current place, resolved by the caller.
	ChannelID, SpaceID ids.ID
	// State is runs.state.
	State State
	// QueuedAt is runs.queued_at; StartedAt is runs.started_at, zero before the first start.
	QueuedAt, StartedAt time.Time
}

// LostTriggerRuns returns the ids of the runs whose starter lost trigger on the run's place
// through changes (R-SY-11, brief M2-09 note 8a), in the order of runs. A run is kept when its
// state is active and its TriggeredBy has a change that
//
//   - removes the run's channel or the run's space, or
//   - changes the run's channel to a level other than trigger (a capability-only change keeps
//     level trigger and loses nothing).
//
// Changes of other principals and of other places keep nothing. For user.disable the caller
// passes a diff that removes every place of the person.
//
// Example:
//
//	lost := domain.LostTriggerRuns(changes, active)
//	// one run.cancel_requested with domain.CancelReasonFor(action) per id
func LostTriggerRuns(changes []AccessChange, runs []LiveRun) []ids.ID {
	var out []ids.ID
	for _, r := range runs {
		if !r.State.Active() {
			continue
		}
		for _, c := range changes {
			if c.PrincipalID == r.TriggeredBy && losesRun(c, r) {
				out = append(out, r.ID)
				break
			}
		}
	}
	return out
}

// losesRun reports whether the change c takes trigger on the place of r away.
func losesRun(c AccessChange, r LiveRun) bool {
	for _, t := range c.Removed {
		if (t.Kind == accessKindChannel && t.ID == r.ChannelID) || (t.Kind == accessKindSpace && t.ID == r.SpaceID) {
			return true
		}
	}
	for _, t := range c.Changed {
		if t.Kind == accessKindChannel && t.ID == r.ChannelID && t.Level != levelTrigger {
			return true
		}
	}
	return false
}
