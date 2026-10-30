// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Origin is a value of messages.origin (03, 09 3.2). Only OriginHuman triggers.
type Origin string

// The origins of a message.
const (
	// OriginHuman: a message of a person over REST or the gateway.
	OriginHuman Origin = "human"
	// OriginAgent: a message with an agent token, the live box anchor included.
	OriginAgent Origin = "agent"
	// OriginApp: a message with an app token or through an incoming webhook.
	OriginApp Origin = "app"
	// OriginImport: a message from an import.
	OriginImport Origin = "import"
	// OriginSystem: a line created by the server.
	OriginSystem Origin = "system"
)

// ChannelKind is a value of channels.kind (03).
type ChannelKind string

// The kinds of a channel.
const (
	// ChannelText: a text channel with a main thread and threads.
	ChannelText ChannelKind = "text"
	// ChannelForum: a forum channel; it has threads only (R-DM-11).
	ChannelForum ChannelKind = "forum"
	// ChannelDM: a direct message channel; it has no agents (09 3.4 step 2).
	ChannelDM ChannelKind = "dm"
)

// ThreadKind is a value of threads.kind (03).
type ThreadKind string

// The kinds of a thread.
const (
	// ThreadMain: the main thread of a channel; it has no fixed operator (R-DM-21).
	ThreadMain ThreadKind = "main"
	// ThreadThread: a thread with its fixed threads.operator_agent_id (R-DM-12).
	ThreadThread ThreadKind = "thread"
)

// BillingMode is a value of agents.billing_mode (03), the target of the backend policy.
type BillingMode string

// The billing modes of an agent.
const (
	// BillingPersonal: the agent runs on the subscription of one person; it is never a guest.
	BillingPersonal BillingMode = "personal"
	// BillingOrganization: the agent runs on an organization account.
	BillingOrganization BillingMode = "organization"
)

// VoiceMode is a value of the configuration key voice.transcription_mode (12 8.2).
type VoiceMode string

// The transcription modes.
const (
	// VoiceOff: no transcription; a triggering voice message is rejected.
	VoiceOff VoiceMode = "off"
	// VoiceAgent: the candidate transcribes, if its effective feature voice_input is on.
	VoiceAgent VoiceMode = "agent"
	// VoiceService: the server's transcription service transcribes.
	VoiceService VoiceMode = "service"
)

// AgentFacts are the facts about one agent that the decision needs. The caller gathers them from
// agents (Active, BillingMode, VoiceInput) and authz (WriteOnChannel).
type AgentFacts struct {
	// ID is the principal id of the agent; never zero.
	ID ids.ID
	// Active is principals.status = 'active'. Decide does not read it (09 3.4 checks activity in
	// step 7); Settle uses it to tell agent_disabled from agent_not_allowed_here.
	Active bool
	// BillingMode is agents.billing_mode; a personal agent is never a guest (R-AZ-19, R-AG-22).
	BillingMode BillingMode
	// WriteOnChannel is the agent's own effective level on the channel being at least write
	// (05 section 5 point 3); it includes the operator rule (05 2.3).
	WriteOnChannel bool
	// VoiceInput is the effective feature voice_input (09 2.2, R-AG-01).
	VoiceInput bool
}

// Switches are the trigger switches of the channel (03 channels, 09 3.3, channel behaviour of
// M2-02). history_backfill_limit has no influence on the decision and is not here.
type Switches struct {
	// RequireMention: in the main thread the operator answers only when mentioned.
	RequireMention bool
	// ThreadRequireMention: in threads the operator answers only when mentioned.
	ThreadRequireMention bool
	// FreeResponse: the operator answers everywhere without a mention; overrides both above.
	FreeResponse bool
	// AutoThread: a trigger in the main thread creates a new thread.
	AutoThread bool
	// AllowMentionedAgents: further agents may answer when mentioned (09 3.6).
	AllowMentionedAgents bool
}

// ActiveRun is the active run of a thread: a run in queued, running or waiting_input. There is at
// most one per thread (03 runs, R-DM-14).
type ActiveRun struct {
	// RunID is the run; AgentID its agent. Neither is zero.
	RunID, AgentID ids.ID
}

// MessageTrigger are the facts about one new message M that Decide needs. Where each field comes
// from is listed in the runs README, section "Trigger decision".
type MessageTrigger struct {
	// MessageID is M, ThreadID its thread and ChannelID its channel; none is zero.
	MessageID, ThreadID, ChannelID ids.ID
	// Origin is messages.origin.
	Origin Origin
	// Voice is messages.kind = 'voice'.
	Voice bool
	// ChannelKind is channels.kind of the channel of M.
	ChannelKind ChannelKind
	// ThreadKind is threads.kind of the thread of M.
	ThreadKind ThreadKind
	// ThreadOperator is the agent of threads.operator_agent_id; nil when NULL and always nil for
	// a main thread (R-DM-21).
	ThreadOperator *AgentFacts
	// ThreadOperatorOutside reports that ThreadOperator is no longer allowed in the channel (09
	// 3.4 step 4a, addendum of 2026-10-30): it is not the channel's effective operator, has no
	// write of its own on the channel, no active run in the thread and never completed one there.
	// Set only with ThreadOperator.
	ThreadOperatorOutside bool
	// ChannelOperator is the effective operator of the channel now; nil when none.
	ChannelOperator *AgentFacts
	// Switches are the channel switches.
	Switches Switches
	// Mentioned are the resolved agent mentions of M; people are not in it. The same agent may
	// appear twice and counts once.
	Mentioned []AgentFacts
	// AuthorHasTrigger reports effective trigger of the author on the channel (07 2.3).
	AuthorHasTrigger bool
	// ActiveRun is the active run of the thread of M; nil when none.
	ActiveRun *ActiveRun
	// VoiceMode is voice.transcription_mode.
	VoiceMode VoiceMode
}

// NewThread describes the auto thread that a trigger in the main thread creates (09 3.7). The
// caller creates it through the chat port in the transaction of the acceptance.
type NewThread struct {
	// ChannelID is the channel of M.
	ChannelID ids.ID
	// ParentMessageID is M.
	ParentMessageID ids.ID
	// OperatorAgentID is the effective operator of the channel; zero when there is none (a guest
	// was mentioned).
	OperatorAgentID ids.ID
	// Title is always "" until a title is set (M2-18).
	Title string
	// TitleSource is always TitleSourceAuto.
	TitleSource string
}

// Decision is the result of Decide or DecideCommand. The zero value has OutcomeNone and no check.
type Decision struct {
	// Outcome is the result kind.
	Outcome Outcome
	// Step is the row of the decision table that decided; empty only with an error.
	Step Step
	// Reason is set exactly with OutcomeReject.
	Reason Reason
	// TriggerKind is TriggerKindMessage or TriggerKindCommand (03 runs.trigger_kind); empty only
	// with an error.
	TriggerKind string
	// TriggerMessageID is M; it carries trigger_state, also when the target is a new thread
	// (09 3.7). Zero for commands.
	TriggerMessageID ids.ID
	// CommandID is the command of DecideCommand (03 runs.triggering_command_id); zero otherwise.
	CommandID ids.ID
	// Agent is a copy of the facts of the candidate for invoke, followup and control; nil
	// otherwise.
	Agent *AgentFacts
	// Guest marks a guest run, which has no agent session (09 3.6, R-AG-04).
	Guest bool
	// Mentioned reports that the candidate was mentioned explicitly; the caller passes it as
	// InvokeFacts.MentionsAgent (D-M2-04-4).
	Mentioned bool
	// NewThread is the target when it is a new auto thread; nil when the target is the thread
	// of M.
	NewThread *NewThread
	// RunID is the active run for OutcomeFollowUp; zero otherwise.
	RunID ids.ID
	// Check is the authorization that runs next; CheckNone for none and reject.
	Check Check
	// Transcript is the transcript source of a triggering voice message; empty otherwise.
	Transcript TranscriptSource
}

// Decide applies steps 1 to 6 of 09 3.4 and the voice rule of 09 13 to one new message. The rows
// of the decision table (runs README, "Trigger decision") are evaluated in order and the first
// row that decides wins; Decision.Step names it.
//
// Decide is pure: it reads in only, never mutates it and calls no authorize; step 7, the
// agent.invoke check, is the caller's, and Settle maps its verdict.
//
// It returns an error that wraps errs.CodeInternalError, with the zero Decision, when in is
// invalid (D-M2-04-5): an unknown enum value, an operator where 03 allows none, a main thread in a
// forum channel or a zero id. The error names the field, never a value.
//
// Example:
//
//	d, err := domain.Decide(domain.MessageTrigger{
//		MessageID: m, ThreadID: t, ChannelID: c, Origin: domain.OriginHuman,
//		ChannelKind: domain.ChannelText, ThreadKind: domain.ThreadMain,
//		ChannelOperator: &lotse, Switches: domain.Switches{FreeResponse: true, AutoThread: true},
//		AuthorHasTrigger: true, VoiceMode: domain.VoiceOff,
//	})
//	// d.Outcome == OutcomeInvoke, d.NewThread.ParentMessageID == m, d.Check == CheckInvoke
func Decide(in MessageTrigger) (Decision, error) {
	if err := validateMessage(&in); err != nil {
		return Decision{}, err
	}
	d := Decision{TriggerKind: TriggerKindMessage, TriggerMessageID: in.MessageID}

	// Steps 1 and 2: only a human message outside a DM triggers (09 3.2, 07 SR4).
	if in.Origin != OriginHuman {
		d.Step = StepOrigin
		return d, nil
	}
	if in.ChannelKind == ChannelDM {
		d.Step = StepDirectMessage
		return d, nil
	}

	// Step 3: the mentioned agents E choose the candidate.
	op := operatorOf(&in)
	cand, mentioned, guest, reject := chooseCandidate(&in, op)
	if reject != "" {
		return d.reject(StepMentions, reject), nil
	}
	if cand == nil {
		d.Step = StepMentions
		return d, nil
	}

	// Step 4: without trigger only an explicit mention earns an answer (R-AG-05).
	if !in.AuthorHasTrigger {
		if mentioned {
			return d.reject(StepTriggerRight, ReasonNoTriggerRight), nil
		}
		d.Step = StepTriggerRight
		return d, nil
	}

	// Step 4a (addendum of 2026-10-30): a fixed thread operator that is no longer allowed in the
	// channel gets no run; a run would fail at once in its runtime. The person sees why instead.
	if !guest && in.ThreadKind == ThreadThread && in.ThreadOperatorOutside && cand.ID == in.ThreadOperator.ID {
		return d.reject(StepThreadOperator, ReasonOperatorNotInChannel), nil
	}

	// Voice rule (09 13, D-M2-04-2): before the target, because the path depends on the candidate
	// and a message that cannot be understood creates neither an auto thread nor a follow-up.
	if in.Voice {
		src := transcriptPath(in.VoiceMode, cand)
		if src == TranscriptNone {
			return d.reject(StepVoice, ReasonVoiceUnsupported), nil
		}
		d.Transcript = src
	}

	a := *cand
	d.Agent, d.Mentioned, d.Guest = &a, mentioned, guest
	return target(&in, d), nil
}

// target applies steps 5 and 6 of 09 3.4 to a decision with a candidate. A new thread has no
// active run, so step 6 does not apply to it.
func target(in *MessageTrigger, d Decision) Decision {
	if in.ThreadKind == ThreadMain && in.Switches.AutoThread {
		nt := NewThread{ChannelID: in.ChannelID, ParentMessageID: in.MessageID, TitleSource: TitleSourceAuto}
		if in.ChannelOperator != nil {
			nt.OperatorAgentID = in.ChannelOperator.ID
		}
		d.NewThread = &nt
	} else if in.ActiveRun != nil {
		// A run of the candidate takes M as a follow-up; another agent's run rejects.
		if in.ActiveRun.AgentID != d.Agent.ID {
			return Decision{TriggerKind: TriggerKindMessage, TriggerMessageID: in.MessageID}.
				reject(StepActiveRun, ReasonRunActiveOtherAgent)
		}
		d.Outcome, d.Step, d.RunID, d.Check = OutcomeFollowUp, StepActiveRun, in.ActiveRun.RunID, CheckFollowUp
		return d
	}
	d.Outcome, d.Step, d.Check = OutcomeInvoke, StepInvokeCheck, CheckInvoke
	return d
}

// reject returns d as a rejection at step s with reason r: no candidate, no check.
func (d Decision) reject(s Step, r Reason) Decision {
	d.Outcome, d.Step, d.Reason = OutcomeReject, s, r
	return d
}

// operatorOf returns the operator of the thread of M (09 3.1): the fixed operator of a thread,
// the channel's effective operator for the main thread. A later operator change of the channel
// never changes the operator of an existing thread (R-DM-12).
func operatorOf(in *MessageTrigger) *AgentFacts {
	if in.ThreadKind == ThreadThread {
		return in.ThreadOperator
	}
	return in.ChannelOperator
}

// chooseCandidate applies step 3 of 09 3.4. It returns the candidate (nil for no trigger), whether
// it was mentioned and is a guest, or a rejection reason.
func chooseCandidate(in *MessageTrigger, op *AgentFacts) (cand *AgentFacts, mentioned, guest bool, reject Reason) {
	var only *AgentFacts
	for i := range in.Mentioned {
		m := &in.Mentioned[i]
		if only == nil {
			only = m
		} else if m.ID != only.ID {
			return nil, false, false, ReasonMultipleAgentsMentioned
		}
	}
	if only != nil {
		if op != nil && only.ID == op.ID {
			return op, true, false, ""
		}
		if guestRule(in.Switches, only) {
			return only, true, true, ""
		}
		return nil, false, false, ReasonAgentNotAllowedHere
	}
	if op == nil {
		return nil, false, false, ""
	}
	sw := in.Switches
	if sw.FreeResponse ||
		(in.ThreadKind == ThreadMain && !sw.RequireMention) ||
		(in.ThreadKind == ThreadThread && !sw.ThreadRequireMention) {
		return op, false, false, ""
	}
	return nil, false, false, ""
}

// guestRule is 09 3.6 and 05 section 5 point 3 for a mentioned agent that is not the operator:
// guests allowed, at least write on the channel itself, and organization billing (R-AG-22: I1
// does not count guest channels among the targets of a personal agent). Activity is not part of
// it; step 7 checks it.
func guestRule(sw Switches, a *AgentFacts) bool {
	return sw.AllowMentionedAgents && a.WriteOnChannel && a.BillingMode == BillingOrganization
}

// transcriptPath returns the transcription path of a voice message for the candidate, or
// TranscriptNone when there is none (12 8.2).
func transcriptPath(mode VoiceMode, cand *AgentFacts) TranscriptSource {
	switch {
	case mode == VoiceService:
		return TranscriptService
	case mode == VoiceAgent && cand.VoiceInput:
		return TranscriptAgent
	}
	return TranscriptNone
}

// invalid returns the error of D-M2-04-5 for a field of the input. It names the field only.
func invalid(field string) error {
	return errs.New(errs.CodeInternalError, "runs: invalid trigger input: "+field)
}

// validateMessage checks in before any rule runs (D-M2-04-5), so a broken input never yields an
// invocation (fail closed, 09 1 point 4).
func validateMessage(in *MessageTrigger) error {
	switch {
	case in.MessageID.IsZero():
		return invalid("message_id")
	case in.ThreadID.IsZero():
		return invalid("thread_id")
	case in.ChannelID.IsZero():
		return invalid("channel_id")
	}
	switch in.Origin {
	case OriginHuman, OriginAgent, OriginApp, OriginImport, OriginSystem:
	default:
		return invalid("origin")
	}
	switch in.ChannelKind {
	case ChannelText, ChannelForum, ChannelDM:
	default:
		return invalid("channel_kind")
	}
	switch in.ThreadKind {
	case ThreadMain, ThreadThread:
	default:
		return invalid("thread_kind")
	}
	switch in.VoiceMode {
	case VoiceOff, VoiceAgent, VoiceService:
	default:
		return invalid("voice_mode")
	}
	return validateShape(in)
}

// validateShape checks the combinations that 03 rules out and the ids of the agents and the run.
func validateShape(in *MessageTrigger) error {
	switch {
	case in.ThreadKind == ThreadMain && in.ThreadOperator != nil:
		return invalid("thread_operator") // R-DM-21
	case in.ThreadOperatorOutside && in.ThreadOperator == nil:
		return invalid("thread_operator_outside")
	case in.ThreadKind == ThreadMain && in.ChannelKind == ChannelForum:
		return invalid("thread_kind") // R-DM-11
	case in.ChannelKind == ChannelDM && (in.ThreadOperator != nil || in.ChannelOperator != nil):
		return invalid("channel_operator") // 03 channels: a DM has no operator
	}
	if in.ThreadOperator != nil {
		if err := validateAgent(in.ThreadOperator, "thread_operator"); err != nil {
			return err
		}
	}
	if in.ChannelOperator != nil {
		if err := validateAgent(in.ChannelOperator, "channel_operator"); err != nil {
			return err
		}
	}
	for i := range in.Mentioned {
		if err := validateAgent(&in.Mentioned[i], "mentioned"); err != nil {
			return err
		}
	}
	return validateRun(in.ActiveRun)
}

// validateAgent checks the id and the billing mode of one agent.
func validateAgent(a *AgentFacts, field string) error {
	if a.ID.IsZero() {
		return invalid(field + ".id")
	}
	switch a.BillingMode {
	case BillingPersonal, BillingOrganization:
		return nil
	}
	return invalid(field + ".billing_mode")
}

// validateRun checks the ids of an active run; nil is valid.
func validateRun(r *ActiveRun) error {
	if r == nil {
		return nil
	}
	if r.RunID.IsZero() {
		return invalid("active_run.run_id")
	}
	if r.AgentID.IsZero() {
		return invalid("active_run.agent_id")
	}
	return nil
}
