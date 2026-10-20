// SPDX-License-Identifier: Apache-2.0

package domain

// Reason is a value of messages.trigger_state_reason: why a message did not trigger an agent.
// The constants below are exactly the codes of docs/spec/09-agenten.md section 3.5 "Reasons for
// rejection at acceptance"; TestRejectionTableCoversSpec keeps the two in step. Settle passes other
// deny reasons of authorize through unchanged (09 3.5), so a Reason in a Settlement may also hold
// a deny reason of 07 4.1 that has no constant here.
type Reason string

// The codes of 09 3.5. Each names the condition, not the notice: the apps map them to texts of
// their language catalogue (01 6.3).
const (
	// ReasonMultipleAgentsMentioned: more than one agent mentioned (09 3.4 step 3).
	ReasonMultipleAgentsMentioned Reason = "multiple_agents_mentioned"
	// ReasonAgentNotAllowedHere: the mentioned agent is neither the operator nor an allowed guest.
	ReasonAgentNotAllowedHere Reason = "agent_not_allowed_here"
	// ReasonNoTriggerRight: the author lacks effective trigger and mentioned an agent (step 4).
	ReasonNoTriggerRight Reason = "no_trigger_right"
	// ReasonRunActiveOtherAgent: another agent has the active run of the target (step 6).
	ReasonRunActiveOtherAgent Reason = "run_active_other_agent"
	// ReasonAgentDisabled: the candidate is disabled; Settle derives it from agent_not_reachable.
	ReasonAgentDisabled Reason = "agent_disabled"
	// ReasonBillingModePersonalNotHolder: I2 denied a personal agent for another person (05 5.4).
	ReasonBillingModePersonalNotHolder Reason = "billing_mode_personal_not_holder"
	// ReasonProjectNotReady: the project binding of the channel is not ready.
	ReasonProjectNotReady Reason = "project_not_ready"
	// ReasonVoiceUnsupported: a voice message without a transcription path (09 13).
	ReasonVoiceUnsupported Reason = "voice_unsupported"
	// ReasonRunActive: a model command during an active run of the thread (09 10.4).
	ReasonRunActive Reason = "run_active"
)

// Outcome is the result kind of a Decision. The zero value is OutcomeNone, so a zero Decision
// (the one returned with an error) never invokes anything (fail closed, 09 1 point 4).
type Outcome uint8

// The outcomes of Decide and DecideCommand.
const (
	// OutcomeNone: no trigger; trigger_state stays NULL.
	OutcomeNone Outcome = iota
	// OutcomeReject: rejection at acceptance with a Reason; no run is created.
	OutcomeReject
	// OutcomeInvoke: invoke the candidate after the agent.invoke check.
	OutcomeInvoke
	// OutcomeFollowUp: the message becomes a follow-up to the active run of the candidate.
	OutcomeFollowUp
	// OutcomeControl: a control command goes to the agent at once, also during a run.
	OutcomeControl
)

// String returns the lower-case name of the outcome ("none", "reject", "invoke", "followup",
// "control") for logs and test output, or "unknown" for a value outside the list.
func (o Outcome) String() string {
	switch o {
	case OutcomeNone:
		return "none"
	case OutcomeReject:
		return "reject"
	case OutcomeInvoke:
		return "invoke"
	case OutcomeFollowUp:
		return "followup"
	case OutcomeControl:
		return "control"
	}
	return "unknown"
}

// Step names the row of the decision table that decided (design table in the runs README).
type Step string

// The steps of the decision table. Validation and the choice of the target decide no outcome of
// their own, so they have no Step; an error leaves Step empty.
const (
	// StepOrigin: 09 3.4 step 1, origin is not human.
	StepOrigin Step = "origin"
	// StepDirectMessage: 09 3.4 step 2, a DM channel.
	StepDirectMessage Step = "direct_message"
	// StepMentions: 09 3.4 step 3, the mentioned agents and the mention rule.
	StepMentions Step = "mentions"
	// StepTriggerRight: 09 3.4 step 4, the author's trigger permission.
	StepTriggerRight Step = "trigger_right"
	// StepVoice: 09 13, the transcription path of a voice message.
	StepVoice Step = "voice"
	// StepActiveRun: 09 3.4 step 6, an active run in the target.
	StepActiveRun Step = "active_run"
	// StepInvokeCheck: 09 3.4 step 7 follows; the caller runs the check.
	StepInvokeCheck Step = "invoke_check"
	// StepCommand: DecideCommand, 09 10.4.
	StepCommand Step = "command"
)

// Check tells the caller which authorization runs next. The domain keeps no action names of the
// catalogue; the application layer maps a Check to its action constant.
type Check string

// The checks of a Decision.
const (
	// CheckNone: no check, for OutcomeNone and OutcomeReject.
	CheckNone Check = ""
	// CheckInvoke: the agent.invoke check (invoke and control).
	CheckInvoke Check = "invoke"
	// CheckFollowUp: the run.followup.create check, "write and agent.invoke" (followup).
	CheckFollowUp Check = "followup"
)

// TranscriptSource says who transcribes a voice message that triggers (12 8.2).
type TranscriptSource string

// The transcript sources of a voice decision.
const (
	// TranscriptNone: not a voice message, or no trigger.
	TranscriptNone TranscriptSource = ""
	// TranscriptAgent: the candidate transcribes (voice.transcription_mode agent, voice_input).
	TranscriptAgent TranscriptSource = "agent"
	// TranscriptService: the server's transcription service (voice.transcription_mode service).
	TranscriptService TranscriptSource = "service"
)

// TriggerState is a value of messages.trigger_state (09 6). Settle yields only none, received and
// failed; TriggerOnOpen and TriggerOnTransition (triggerstate.go) yield the values that follow a
// run (received, working, done, failed, cancelled). Nothing in this package yields queued or
// followup_delivered: the delivery and the follow-up path set those.
type TriggerState string

// The trigger states of a Settlement.
const (
	// TriggerStateNone keeps trigger_state NULL (no trigger, and every command decision).
	TriggerStateNone TriggerState = ""
	// TriggerStateReceived: the check allowed the trigger.
	TriggerStateReceived TriggerState = "received"
	// TriggerStateFailed: rejected at acceptance or denied by the check, with a Reason.
	TriggerStateFailed TriggerState = "failed"
)

// The values of Decision.TriggerKind, mirroring runs.trigger_kind (03).
const (
	// TriggerKindMessage: the trigger is a message.
	TriggerKindMessage = "message"
	// TriggerKindCommand: the trigger is a command owned by an agent.
	TriggerKindCommand = "command"
)

// The values of NewThread.TitleSource (03 threads.title_source).
const (
	// TitleSourceAuto: the title of an auto thread is set later, by the agent or the client.
	TitleSourceAuto = "auto"
)
