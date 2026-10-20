// SPDX-License-Identifier: Apache-2.0

package domain

import "github.com/OWNER/bothy/services/server/internal/kernel/errs"

// The deny reasons of authorize (07 4.1) that Settle maps. The domain keeps them as plain strings
// because kernel/auditing is not part of kernel-core; the values are the ones of
// kernel/auditing.Reason.
const (
	denyAgentNotReachable = "agent_not_reachable"
	denyLevelInsufficient = "level_insufficient"
	denyNoGrant           = "no_grant"
)

// Verdict is the outcome of the check named by Decision.Check, as the caller got it from
// authorize.
type Verdict struct {
	// Allowed is true for allow.
	Allowed bool
	// Reason is the deny reason of 07 4.1; required when Allowed is false, ignored otherwise.
	Reason string
}

// Settlement is what the acceptance records for the trigger (09 3.4 step 7, 09 6).
type Settlement struct {
	// Accepted is true when the check allowed: the caller creates the run, the follow-up or the
	// control delivery.
	Accepted bool
	// State is the new messages.trigger_state: received, failed, or TriggerStateNone (stays NULL;
	// always for command decisions, which have no trigger message).
	State TriggerState
	// Reason is the trigger_state_reason of a rejection or a denial; empty otherwise.
	Reason Reason
}

// Settle turns a Decision and the verdict of its check into the trigger state (D-M2-04-3).
//
// For OutcomeNone and OutcomeReject the verdict is ignored: none keeps trigger_state NULL, reject
// gives failed with the decision's reason. For OutcomeInvoke, OutcomeFollowUp and OutcomeControl an
// allow gives Accepted and, for a message decision, received; a denial gives failed (message
// decisions) and the reason by this mapping, because the closed deny list of 07 4.1 has no
// agent_disabled and the guard answers agent_not_reachable for an inactive agent:
//
//	agent_not_reachable, candidate not Active -> agent_disabled
//	agent_not_reachable, candidate Active     -> agent_not_allowed_here
//	level_insufficient, no_grant              -> no_trigger_right
//	every other reason                        -> unchanged
//
// Command decisions never set a trigger state; their Reason is the answer to the command.
//
// It returns an error that wraps errs.CodeInternalError for a denial with an empty reason, for a
// rejection without reason, for a decision with a check but without candidate, and for an unknown
// outcome. The audit of a denial is written by authz (05 2.4); Settle only names the reason a
// person sees.
//
// Example:
//
//	s, err := domain.Settle(d, domain.Verdict{Reason: "agent_not_reachable"})
//	// with d.Agent.Active == false: s.State == "failed", s.Reason == ReasonAgentDisabled
func Settle(d Decision, v Verdict) (Settlement, error) {
	message := d.TriggerKind != TriggerKindCommand
	switch d.Outcome {
	case OutcomeNone:
		return Settlement{}, nil
	case OutcomeReject:
		if d.Reason == "" {
			return Settlement{}, errs.New(errs.CodeInternalError, "runs: rejection without reason")
		}
		s := Settlement{Reason: d.Reason}
		if message {
			s.State = TriggerStateFailed
		}
		return s, nil
	case OutcomeInvoke, OutcomeFollowUp, OutcomeControl:
	default:
		return Settlement{}, errs.New(errs.CodeInternalError, "runs: unknown decision outcome")
	}
	if d.Agent == nil {
		return Settlement{}, errs.New(errs.CodeInternalError, "runs: decision without candidate")
	}
	if v.Allowed {
		s := Settlement{Accepted: true}
		if message {
			s.State = TriggerStateReceived
		}
		return s, nil
	}
	if v.Reason == "" {
		return Settlement{}, errs.New(errs.CodeInternalError, "runs: denial without reason")
	}
	s := Settlement{Reason: mapDenial(v.Reason, d.Agent.Active)}
	if message {
		s.State = TriggerStateFailed
	}
	return s, nil
}

// mapDenial is the mapping table of Settle.
func mapDenial(reason string, active bool) Reason {
	switch reason {
	case denyAgentNotReachable:
		if !active {
			return ReasonAgentDisabled
		}
		return ReasonAgentNotAllowedHere
	case denyLevelInsufficient, denyNoGrant:
		return ReasonNoTriggerRight
	}
	return Reason(reason)
}
