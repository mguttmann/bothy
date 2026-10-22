// SPDX-License-Identifier: Apache-2.0

package domain

// The trigger states that follow a run (09 6). TriggerStateNone, TriggerStateReceived and
// TriggerStateFailed are declared with the trigger decision in trigger_reasons.go.
const (
	// TriggerStateQueued: the invocation was delivered to the adapter, the start is pending; the
	// delivery sets it, not a run transition.
	TriggerStateQueued TriggerState = "queued"
	// TriggerStateWorking: the run is running, and also while it waits for input.
	TriggerStateWorking TriggerState = "working"
	// TriggerStateDone: the run completed.
	TriggerStateDone TriggerState = "done"
	// TriggerStateCancelled: the run was stopped, with the run's reason.
	TriggerStateCancelled TriggerState = "cancelled"
	// TriggerStateFollowupDelivered: a follow-up was handed over to the runtime; the follow-up
	// path sets it, not a run transition.
	TriggerStateFollowupDelivered TriggerState = "followup_delivered"
)

// TriggerChange is the new trigger_state of the triggering message and its
// trigger_state_reason. Reason is set only for failed and cancelled (03 messages check).
type TriggerChange struct {
	// State is the new messages.trigger_state.
	State TriggerState
	// Reason is the new messages.trigger_state_reason: the run's state_reason, or empty.
	Reason string
}

// TriggerOnOpen returns the trigger state of a message whose run was just created in queued at
// acceptance (09 3.4 point 7): received, without reason.
func TriggerOnOpen() TriggerChange {
	return TriggerChange{State: TriggerStateReceived}
}

// TriggerOnTransition maps a run transition to the change of trigger_state of the triggering
// message (09 6). The second result is false when trigger_state stays as it is:
//
//   - queued -> running and waiting_input -> running give working;
//   - running -> waiting_input leaves it unchanged (working covers waiting_input);
//   - -> completed gives done;
//   - -> failed gives failed and -> cancelled gives cancelled, both with reason, the run's
//     state_reason, which the caller has validated (ValidateServerReason, NormalizeAdapterReason);
//   - a pair CanTransition refuses gives false.
//
// reason is ignored for working and done. Runs without a triggering message change no
// trigger_state; the caller does not call this function for them.
//
// Example:
//
//	c, ok := domain.TriggerOnTransition(domain.StateRunning, domain.StateFailed, domain.RunReasonStale)
//	// c = {failed, stale}, ok = true
func TriggerOnTransition(from, to State, reason string) (TriggerChange, bool) {
	if !CanTransition(from, to) {
		return TriggerChange{}, false
	}
	switch to {
	case StateRunning:
		return TriggerChange{State: TriggerStateWorking}, true
	case StateCompleted:
		return TriggerChange{State: TriggerStateDone}, true
	case StateFailed:
		return TriggerChange{State: TriggerStateFailed, Reason: reason}, true
	case StateCancelled:
		return TriggerChange{State: TriggerStateCancelled, Reason: reason}, true
	case StateWaitingInput, StateQueued:
		// waiting_input stays working; no allowed transition leads to queued.
		return TriggerChange{}, false
	}
	return TriggerChange{}, false
}
