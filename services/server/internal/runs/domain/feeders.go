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

// ContextElement is what the record needs of one message a run context gives a run: its author
// and, for a team input, the input's authority.
type ContextElement struct {
	AuthorID   ids.ID
	AuthorKind string
	// TeamInput reports a team input (03 team_inputs); Authority is its authority then.
	TeamInput bool
	Authority ids.ID
}

// ContextFeeder returns the principal run_feeders records, source context, for an element of the
// run context of a run of runAgent, and whether it records one (09 3.9.4, red team round 5 of
// FEAT-TEAM-CHANNEL-SERVER). Every element is recorded whatever its origin (human, agent, app,
// import, system) and wherever it lies (trigger, thread or channel window): a team input with its
// authority, a person's message with its author, and every other message (another agent's that
// is no team input, an app's, one without a person as author) as an input that speaks for no
// person (zero, stored as NULL), which the owner check refuses. Only a message of runAgent itself
// is left out: what an earlier run of the agent wrote is the limitation of ruling D-TEAM-11.
func ContextFeeder(e ContextElement, runAgent ids.ID) (ids.ID, bool) {
	switch {
	case e.TeamInput:
		return e.Authority, true
	case !e.AuthorID.IsZero() && e.AuthorID == runAgent:
		return ids.ID{}, false
	case e.AuthorKind == principalUser:
		return e.AuthorID, true
	}
	return ids.ID{}, true
}
