// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// The client errors of FinishDecision. Each is the adapter's mistake; the caller maps it to 400
// validation_failed with the pointer named here. A missing reason of failed or cancelled is
// ErrReasonRequired (pointer /reason, errors[].code required).
var (
	// ErrCancelReason: cancelled with a reason outside CancelReasons; pointer /reason.
	ErrCancelReason = errors.New("runs: the reason is not a reason of a cancel request")
	// ErrFinishTarget: the target is not completed, failed or cancelled; pointer /state.
	ErrFinishTarget = errors.New("runs: finishRun targets completed, failed or cancelled")
)

// cancelReasons are the reasons of a cancel request (09 5.2, 04 15.5), the only reasons an
// adapter may report for cancelled (brief M2-09 note 6c).
var cancelReasons = []string{
	RunReasonUserCancelled, RunReasonAccessRevoked, RunReasonAgentDisabled, RunReasonChannelReadOnly, RunReasonUserDisabled,
}

// CancelReasons returns the reasons of a cancel request in this order: user_cancelled,
// access_revoked, agent_disabled, channel_read_only, user_disabled. They are the values of
// RunCancelRequestedPayload.reason and the only reasons finishRun accepts for cancelled. The
// slice is a copy: the caller may change it.
func CancelReasons() []string { return slices.Clone(cancelReasons) }

// FinishRequest is the part of a RunFinish body that FinishDecision decides on.
type FinishRequest struct {
	// Target is the requested end state.
	Target State
	// Reason is the reported reason; "" when absent.
	Reason string
}

// FinishKind is the kind of a FinishOutcome.
type FinishKind uint8

// The kinds of FinishOutcome. The zero value is invalid.
const (
	// FinishTransition: end the run through the end-of-run function.
	FinishTransition FinishKind = iota + 1
	// FinishNoChange: the run already is in the target state; answer 200 with the current run,
	// no new version, no event (06 2 "No effect").
	FinishNoChange
)

// FinishOutcome is the result of FinishDecision.
type FinishOutcome struct {
	// Kind is what to do.
	Kind FinishKind
	// Reason is the value to store in runs.state_reason; "" for completed and for FinishNoChange.
	Reason string
	// Original is the adapter reason replaced by adapter_error, for the server step of
	// OriginalReasonDetail; "" otherwise.
	Original string
}

// FinishDecision decides finishRun on a run in state current (brief M2-09 notes 6b and 6c). The
// checks run in this order, so a malformed body is refused before any state is read:
//
//  1. a target other than completed, failed or cancelled: ErrFinishTarget.
//  2. failed or cancelled without a reason: ErrReasonRequired.
//  3. cancelled with a reason outside CancelReasons: ErrCancelReason.
//  4. current equals the target: FinishNoChange.
//  5. an ended run, or completed from anything but running: an *errs.Error with
//     errs.CodeInvalidStateTransition (409).
//
// Then FinishTransition: failed stores the reason through NormalizeAdapterReason (an adapter
// reason is kept, every other code becomes adapter_error with Original set); cancelled stores the
// reason as reported; completed never stores a reason.
//
// Example:
//
//	out, err := domain.FinishDecision(domain.StateRunning, domain.FinishRequest{Target: domain.StateFailed, Reason: "stale"})
//	// out.Reason == "adapter_error", out.Original == "stale"
func FinishDecision(current State, req FinishRequest) (FinishOutcome, error) {
	if !req.Target.Ended() {
		return FinishOutcome{}, ErrFinishTarget
	}
	if req.Target != StateCompleted && req.Reason == "" {
		return FinishOutcome{}, ErrReasonRequired
	}
	if req.Target == StateCancelled && !slices.Contains(cancelReasons, req.Reason) {
		return FinishOutcome{}, ErrCancelReason
	}
	if current == req.Target {
		return FinishOutcome{Kind: FinishNoChange}, nil
	}
	if !CanTransition(current, req.Target) {
		// Out of an end state, and completed from queued or waiting_input (03 runs).
		return FinishOutcome{}, errs.New(errs.CodeInvalidStateTransition, "runs: the run cannot reach this end state")
	}
	//exhaustive:ignore // completed carries no reason; FinishDecision allows no other target here.
	switch req.Target {
	case StateFailed:
		reason, original, err := NormalizeAdapterReason(req.Reason)
		if err != nil {
			return FinishOutcome{}, err
		}
		return FinishOutcome{Kind: FinishTransition, Reason: reason, Original: original}, nil
	case StateCancelled:
		return FinishOutcome{Kind: FinishTransition, Reason: req.Reason}, nil
	}
	return FinishOutcome{Kind: FinishTransition}, nil
}
