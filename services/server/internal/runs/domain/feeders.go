// SPDX-License-Identifier: Apache-2.0

package domain

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
	// FeedContext is a message of the run context the server gave the run.
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
