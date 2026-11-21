// SPDX-License-Identifier: Apache-2.0

package domain

import "github.com/OWNER/bothy/services/server/internal/kernel/ids"

// The sources of 03 run_feeders: the ways an input reaches a run (FEAT-TEAM-CHANNEL-SERVER, 09
// 3.9.4). The database records the first three when it stores the input (migration 00108); the
// delivery check records every delivered event by FeedSourceOf, and the run context the authors of
// its messages.
const (
	// FeedTrigger is runs.triggered_by, and the delivery of agent.invocation.
	FeedTrigger = "trigger"
	// FeedFollowup is a follow-up: the authority of a team input, the author of a person's
	// message, none for every other message.
	FeedFollowup = "followup"
	// FeedInteraction is an interaction with a component bound to the run.
	FeedInteraction = "interaction"
	// FeedCommand is a control command delivered into the run (command.invoked with run_id).
	FeedCommand = "command"
	// FeedWakeInput is a wake-up that fired into the run.
	FeedWakeInput = "wake_input"
	// FeedContext is a message of the run context the server gave the run (ContextFeeder).
	FeedContext = "context"
)

// FeedSources returns every source of run_feeders, the values of its check.
func FeedSources() []string {
	return []string{FeedTrigger, FeedFollowup, FeedInteraction, FeedCommand, FeedWakeInput, FeedContext}
}

// FeedSourceOf returns the source of run_feeders a delivery of eventType records; "" for an event
// type that bears no invocation. Every type of InvocationBearingTypes has one (held by a test), so
// a new input event cannot reach an agent without being recorded.
func FeedSourceOf(eventType string) string {
	switch eventType {
	case EventAgentInvocation:
		return FeedTrigger
	case EventAgentFollowup:
		return FeedFollowup
	case EventInteractionCreated:
		return FeedInteraction
	case EventCommandInvoked:
		return FeedCommand
	case EventAgentWakeInput:
		return FeedWakeInput
	}
	return ""
}

// principalUser is principals.kind of a person (03 principals).
const principalUser = "user"

// ContextElement is what the record needs of one message a run context gives a run: its author,
// for a team input the input's authority, and for an agent's answer the record of the run it
// answers (03 run_answers).
type ContextElement struct {
	AuthorID   ids.ID
	AuthorKind string
	// Edited reports a message edited after it was written (only its author edits it).
	Edited bool
	// TeamInput reports a team input (03 team_inputs); Authority is its authority then.
	TeamInput bool
	Authority ids.ID
	// Answer reports a message linked in run_answers; AnswerRecord holds the principals recorded
	// for that run (zero for one that speaks for no person) and AnswerTriggered whether its
	// record has its trigger row.
	Answer          bool
	AnswerRecord    []ids.ID
	AnswerTriggered bool
}

// ContextFeeders returns the principals run_feeders records, source context, for an element of a
// run context (09 3.9.4, red team rounds 5 and 6 of FEAT-TEAM-CHANNEL-SERVER). Every element is
// recorded whatever its origin (human, agent, app, import, system), its author (the run's own
// agent included) and its place (trigger, thread or channel window): a team input with its
// authority, a person's message with its author, an agent's answer with the whole record of the
// run it answers (an incomplete record, one without its trigger row, as an input that speaks for
// no person), and every other message (an agent's message written outside a run of its thread,
// an app's, one without a person as author) as an input that speaks for no person (zero, stored
// as NULL), which the owner check refuses. A text an agent wrote thus speaks for whoever fed the
// run that wrote it; an agent's message edited afterwards (a team copy or an answer, red team round
// 7) speaks for no person, because the edit is bound to no run.
func ContextFeeders(e ContextElement) []ids.ID {
	switch {
	case e.Edited && e.AuthorKind != principalUser:
		return []ids.ID{{}}
	case e.TeamInput:
		return []ids.ID{e.Authority}
	case e.AuthorKind == principalUser:
		return []ids.ID{e.AuthorID}
	case e.Answer && e.AnswerTriggered:
		return append([]ids.ID{}, e.AnswerRecord...)
	}
	return []ids.ID{{}}
}
