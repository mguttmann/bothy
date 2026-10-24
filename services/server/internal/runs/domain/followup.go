// SPDX-License-Identifier: Apache-2.0

package domain

import "github.com/OWNER/bothy/services/server/internal/kernel/errs"

// FollowupState is a value of run_followups.state (03 run_followups, schema RunFollowup).
type FollowupState string

// The states of a follow-up (09 section 7).
const (
	// FollowupQueued: accepted, not yet handed to the runtime.
	FollowupQueued FollowupState = "queued"
	// FollowupDelivered: the adapter handed it to the runtime (09 4.7).
	FollowupDelivered FollowupState = "delivered"
	// FollowupAbsorbed: the runtime took it up.
	FollowupAbsorbed FollowupState = "absorbed"
	// FollowupDeferred: the run ended before it was absorbed; it triggers or joins the follow-on run.
	FollowupDeferred FollowupState = "deferred"
	// FollowupDiscarded: it will never be processed (stop by a person, deleted message, refused).
	FollowupDiscarded FollowupState = "discarded"
)

// FollowupStates returns the five states in the order of the CHECK of 03 run_followups.
func FollowupStates() []FollowupState {
	return []FollowupState{FollowupQueued, FollowupDelivered, FollowupAbsorbed, FollowupDeferred, FollowupDiscarded}
}

// Open reports queued and delivered: the follow-ups the end of a run settles (09 section 7).
func (s FollowupState) Open() bool { return s == FollowupQueued || s == FollowupDelivered }

// FollowupMode is the effective capability followups of an agent (09 section 2.2).
type FollowupMode string

// The effective follow-up modes.
const (
	// FollowupSteer: the server sends agent.followup while the run is active.
	FollowupSteer FollowupMode = "steer"
	// FollowupQueue: the server sends nothing; the follow-ups wait for the end of the run.
	FollowupQueue FollowupMode = "queue"
	// FollowupNone: no capability; the follow-ups wait for the end of the run like queue.
	FollowupNone FollowupMode = "none"
)

// EffectiveVoiceInput combines the owner's permission supports_voice (nil when the key is absent
// from operator_config, which allows) with the adapter's reported capabilities.voice_input into the
// effective feature voice_input of the trigger decision (09 sections 2.2 and 13, R-AG-01, M3-09):
// true only when not refused and reported.
//
// Example:
//
//	domain.EffectiveVoiceInput(nil, true) // true
func EffectiveVoiceInput(permitted *bool, reported bool) bool {
	return reported && (permitted == nil || *permitted)
}

// EffectiveFollowupMode combines the owner's permission supports_followups (nil when the key is
// absent from operator_config, which allows) with the adapter's reported capabilities.followups
// (09 section 2.2, R-AG-01): steer or queue when permitted and reported exactly so, none for a
// refusal, an absent and an unknown report (fail closed, 09 section 1 point 4).
//
// Example:
//
//	domain.EffectiveFollowupMode(nil, "steer") // FollowupSteer
func EffectiveFollowupMode(permitted *bool, reported string) FollowupMode {
	if permitted != nil && !*permitted {
		return FollowupNone
	}
	switch FollowupMode(reported) {
	case FollowupSteer:
		return FollowupSteer
	case FollowupQueue:
		return FollowupQueue
	case FollowupNone:
	}
	return FollowupNone
}

// FollowupEventNeeded reports whether a follow-up to a run in runState gets the event
// agent.followup: steer in every active state; every mode in waiting_input, where the follow-up is
// the answer to the open question and only the token of its delivery resumes the run (09 section
// 7, R-AG-08); never for an ended run.
func FollowupEventNeeded(mode FollowupMode, runState State) bool {
	if runState == StateWaitingInput {
		return true
	}
	return mode == FollowupSteer && runState.Active()
}

// ErrFollowupTransition is the error of a follow-up state the adapter may not set: backwards, or
// out of deferred or discarded. It reaches the client as 409 invalid_state_transition (06 section
// 2 "No effect"; ruling D-M2-11-3).
var ErrFollowupTransition = errs.New(errs.CodeInvalidStateTransition, "a follow-up state moves forward only")

// AdvanceFollowup checks the move of a follow-up from -> to that an adapter patch asks for (to is
// delivered or absorbed, the values M2-10 accepts): queued -> delivered, queued -> absorbed and
// delivered -> absorbed change; delivered -> delivered and absorbed -> absorbed are no change;
// every other pair is ErrFollowupTransition.
//
// Example:
//
//	changed, err := domain.AdvanceFollowup(domain.FollowupQueued, domain.FollowupDelivered) // true, nil
func AdvanceFollowup(from, to FollowupState) (changed bool, err error) {
	switch {
	case from == FollowupQueued && (to == FollowupDelivered || to == FollowupAbsorbed):
		return true, nil
	case from == FollowupDelivered && to == FollowupAbsorbed:
		return true, nil
	case from == to && (to == FollowupDelivered || to == FollowupAbsorbed):
		return false, nil
	}
	return false, ErrFollowupTransition
}
