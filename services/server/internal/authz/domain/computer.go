// SPDX-License-Identifier: Apache-2.0

package domain

import "github.com/OWNER/bothy/services/server/internal/kernel/ids"

// The rules of computer use (CU-04, 07 section 2.15 and guard rule SR13,
// docs/spec/27-computer-use.md 5.1 step 5). Every function here is pure; the application layer
// gathers the facts (the run from runs, the desktop from terminal, the owner status from the
// owner list) and calls DecideComputer.

// The agent actions of computer use that DecideComputer decides.
const (
	ActionComputerDesktopsList = "computer.desktops.list"
	ActionComputerOpen         = "computer.open"
	ActionComputerAttach       = "computer.attach"
)

// IsComputerAgentAction reports one of the three agent actions of computer use, which the
// Authorizer decides with DecideComputer instead of the generic steps.
func IsComputerAgentAction(action string) bool {
	return action == ActionComputerDesktopsList || action == ActionComputerOpen || action == ActionComputerAttach
}

// The values SR13 compares (09 2.2, 03 runs and run_feeders, 03 messages).
const (
	toolClassHost      = "host"
	triggerKindMessage = "message"
	triggerKindCommand = "command"
	originHumanValue   = "human"
	feedContext        = "context"
)

// ComputerFeeder is one row of run_feeders: the principal (zero for NULL) and the source.
type ComputerFeeder struct {
	PrincipalID ids.ID
	Source      string
}

// ComputerFacts are the facts of one decision of a computer agent action (the domain mirror of
// kernel/authorize.ComputerFacts plus what the Authorizer computes itself).
type ComputerFacts struct {
	// Action is the action id; AgentID the acting agent.
	Action  string
	AgentID ids.ID
	// BindingRunID and BindingUserID are the run and the triggering person of the bound
	// acting-for binding of the request; zero without a bound binding.
	BindingRunID, BindingUserID ids.ID
	// RunFound, RunID, RunAgentID, TriggeredBy, RunActive, ToolClass, TriggerKind, TriggerOrigin,
	// TriggerAuthorID, TriggeringCommand and Feeders are the facts of the run.
	RunFound                       bool
	RunID, RunAgentID, TriggeredBy ids.ID
	RunActive                      bool
	ToolClass, TriggerKind         string
	TriggerOrigin                  string
	TriggerAuthorID                ids.ID
	TriggeringCommand              bool
	Feeders                        []ComputerFeeder
	// PersonOwner reports TriggeredBy as an active owner now, from the current owner list
	// (R-AZ-29).
	PersonOwner bool
	// HostVisible reports that the triggering person may read the host (computer.open and
	// computer.attach; owner, assignee or terminal, 07 2.12 point 5).
	HostVisible bool
	// HostState, HostConnected and HostListed describe the desktop (27 section 4 item 3).
	HostState     string
	HostConnected bool
	HostListed    bool
}

// errComputerRun is the internal error of facts that name another run than the binding.
const errComputerRun = "computer facts of another run than the acting-for binding"

// DecideComputer decides one computer agent action from f in the order of 27 5.1 step 5 (the
// liveness, the pending rule and the status of the acting-for binding ran before, so an ended run
// was already invocation_token_invalid with run_inactive):
//
//  1. SR13 item 1, the binding: without a bound acting-for binding forbidden
//     computer_not_owner_run; facts of another run or agent are an internal error;
//  2. a run that is no longer active: forbidden invocation_token_invalid with run_inactive (SR9);
//  3. visibility (R-AZ-01, 404 before 403): computer.open and computer.attach see only a host the
//     triggering person may read, else not_found no_grant (computer.desktops.list names the bound
//     run itself);
//  4. SR2: a tool class other than host gives forbidden computer_tool_class;
//  5. SR13 items 2, 3 and 5 (OwnRun): forbidden computer_not_owner_run;
//  6. for computer.open and computer.attach: a host that is not active or not connected gives
//     host_not_active (409 host_unavailable) before item 4;
//  7. SR13 item 4: a host that is no listed desktop of the person gives forbidden
//     computer_not_enabled.
//
// The error reports facts that cannot belong to the request; it is never an allow.
//
// Example:
//
//	d, err := domain.DecideComputer(domain.ComputerFacts{Action: "computer.open", ...})
func DecideComputer(f ComputerFacts) (Decision, error) {
	if f.BindingRunID.IsZero() || f.BindingUserID.IsZero() {
		return deny(OutcomeForbidden, ReasonComputerNotOwnerRun), nil
	}
	if !f.RunFound || f.RunID != f.BindingRunID || f.RunAgentID != f.AgentID || f.TriggeredBy != f.BindingUserID {
		return Decision{}, internalError(errComputerRun)
	}
	if !f.RunActive {
		return Decision{Outcome: OutcomeForbidden, Reason: ReasonInvocationTokenInvalid, Detail: actingForRunInactive}, nil
	}
	hostAction := f.Action == ActionComputerOpen || f.Action == ActionComputerAttach
	if hostAction && !f.HostVisible {
		return deny(OutcomeNotFound, ReasonNoGrant), nil
	}
	if f.ToolClass != toolClassHost {
		return deny(OutcomeForbidden, ReasonComputerToolClass), nil
	}
	if !OwnRun(f) {
		return deny(OutcomeForbidden, ReasonComputerNotOwnerRun), nil
	}
	if !hostAction {
		return Allow, nil
	}
	if f.HostState != hostStateActive || !f.HostConnected {
		return deny(OutcomeForbidden, ReasonHostNotActive), nil
	}
	if !f.HostListed {
		return deny(OutcomeForbidden, ReasonComputerNotEnabled), nil
	}
	return Allow, nil
}

// OwnRun reports SR13 items 2, 3 and 5 for the run of f:
//
//   - item 2: trigger_kind message with a triggering message of origin human written by
//     triggered_by, or command with a triggering command (the command form; triggered_by is the
//     person who invoked it);
//   - item 3: triggered_by is an active owner (PersonOwner);
//   - item 5: every row of run_feeders whose source is not context names triggered_by; a row
//     with NULL fails. Rows with source context do not count (T-CU-01).
//
// Example:
//
//	ok := domain.OwnRun(domain.ComputerFacts{TriggeredBy: alex, TriggerKind: "message", TriggerOrigin: "human",
//		TriggerAuthorID: alex, PersonOwner: true})
func OwnRun(f ComputerFacts) bool {
	if f.TriggeredBy.IsZero() || !f.PersonOwner {
		return false
	}
	switch f.TriggerKind {
	case triggerKindMessage:
		if f.TriggerOrigin != originHumanValue || f.TriggerAuthorID != f.TriggeredBy {
			return false
		}
	case triggerKindCommand:
		if !f.TriggeringCommand {
			return false
		}
	default:
		return false
	}
	for _, fd := range f.Feeders {
		if fd.Source == feedContext {
			continue
		}
		if fd.PrincipalID.IsZero() || fd.PrincipalID != f.TriggeredBy {
			return false
		}
	}
	return true
}
