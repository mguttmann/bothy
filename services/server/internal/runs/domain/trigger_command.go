// SPDX-License-Identifier: Apache-2.0

package domain

import "github.com/OWNER/bothy/services/server/internal/kernel/ids"

// CommandTrigger are the facts about one invocation of a command owned by an agent (09 10.4).
// App commands are not decided here (05 section 5; M2-25).
type CommandTrigger struct {
	// CommandID is the command and ThreadID the thread it is invoked in; neither is zero.
	CommandID, ThreadID ids.ID
	// Owner is the agent that registered the command.
	Owner AgentFacts
	// Control is commands.control: a control command (for example a stop) goes to the agent at
	// once, also during a run.
	Control bool
	// ActiveRun is the active run of the thread; nil when none.
	ActiveRun *ActiveRun
}

// DecideCommand applies 09 10.4 to a command owned by an agent:
//
//   - a control command gives OutcomeControl with CheckInvoke, also during a run of any agent, and
//     is never a follow-up (09 7 last sentence, 05 section 5 point 8, D-AG-12);
//   - a model command during an active run of the thread (any agent) gives OutcomeReject with
//     ReasonRunActive;
//   - otherwise OutcomeInvoke with CheckInvoke.
//
// Every non-error result has StepCommand, TriggerKind TriggerKindCommand and CommandID set, and
// control and invoke carry a copy of Owner as Agent. Like Decide it is pure and returns an error
// that wraps errs.CodeInternalError, with the zero Decision, for a zero id or an unknown billing
// mode (D-M2-04-5).
//
// Example:
//
//	d, err := domain.DecideCommand(domain.CommandTrigger{CommandID: cmd, ThreadID: t, Owner: lotse})
//	// d.Outcome == OutcomeInvoke, d.TriggerKind == "command", d.CommandID == cmd
func DecideCommand(in CommandTrigger) (Decision, error) {
	switch {
	case in.CommandID.IsZero():
		return Decision{}, invalid("command_id")
	case in.ThreadID.IsZero():
		return Decision{}, invalid("thread_id")
	}
	if err := validateAgent(&in.Owner, "owner"); err != nil {
		return Decision{}, err
	}
	if err := validateRun(in.ActiveRun); err != nil {
		return Decision{}, err
	}
	d := Decision{Step: StepCommand, TriggerKind: TriggerKindCommand, CommandID: in.CommandID}
	if !in.Control && in.ActiveRun != nil {
		d.Outcome, d.Reason = OutcomeReject, ReasonRunActive
		return d, nil
	}
	owner := in.Owner
	d.Agent, d.Check = &owner, CheckInvoke
	if in.Control {
		d.Outcome = OutcomeControl
	} else {
		d.Outcome = OutcomeInvoke
	}
	return d, nil
}
