// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Commands and cards owned by apps (docs/spec/11-extensions.md section 9, brief M2-25 design notes
// 5 and 8): the target thread of an app command, the payload of its command.invoked and the
// interaction state after a request to the app. An app owns commands and components like an agent
// but never triggers an agent: no run, no token, no trigger state. Pure.

// AppCommandTarget returns the target thread of an app command (design note 5 b): the given
// thread; else the main thread of a text channel (R-DM-11); else zero (a forum without thread).
// There is no auto thread: it exists to give an agent's run a thread. channelKind is text, forum
// or dm.
//
// Example:
//
//	target := domain.AppCommandTarget(cmd.ThreadID, ch.Kind, ch.MainThreadID)
func AppCommandTarget(thread ids.ID, channelKind string, mainThread ids.ID) ids.ID {
	switch {
	case !thread.IsZero():
		return thread
	case channelKind == channelKindForum:
		return ids.ID{}
	}
	return mainThread
}

// AppCommandInvokedInput are the members of the CommandInvokedPayload of an app command.
type AppCommandInvokedInput struct {
	InvocationID, CommandID ids.ID
	CommandName             string
	// Control is commands.control, passed through unchanged; it changes nothing for an app.
	Control bool
	// ChannelID is the channel; ThreadID the target thread, zero without one.
	ChannelID, ThreadID ids.ID
	InvokedBy           PrincipalRefPayload
	Options             []OptionValue
}

// AppCommandInvokedPayload encodes the payload of command.invoked for an app owner (design note
// 13): control as stored, thread_id only with a target thread, never run_id (apps have no runs,
// 11 section 2), no JSON null (ADR 0053). Errors: a zero ID, an empty name or an empty invoker.
func AppCommandInvokedPayload(in AppCommandInvokedInput) ([]byte, error) {
	if in.InvocationID.IsZero() || in.CommandID.IsZero() || in.ChannelID.IsZero() || in.CommandName == "" || in.InvokedBy.ID == "" ||
		in.InvokedBy.Kind == "" {
		return nil, errPayload
	}
	return json.Marshal(commandInvokedPayload{
		InvocationID: in.InvocationID.String(), CommandID: in.CommandID.String(), CommandName: in.CommandName, Control: in.Control,
		ChannelID: in.ChannelID.String(), ThreadID: idText(in.ThreadID), InvokedBy: in.InvokedBy, Options: optionPayloads(in.Options),
	})
}

// The interaction states of 03 interactions that the job runs.app_dispatch reads and writes.
const (
	InteractionReceived     = "received"
	InteractionDispatched   = "dispatched"
	InteractionAcknowledged = "acknowledged"
	InteractionFailed       = "failed"
	InteractionRejected     = "rejected"
)

// AppOutcomeFacts is the outcome of one request to an app as the state rule reads it.
type AppOutcomeFacts struct {
	Answered, Deferred bool
}

// InteractionStateAfterOutcome returns the state an interaction takes after the request to its
// app (design note 8, handler step 5), as a compare-and-swap from dispatched only: answered gives
// acknowledged, deferred keeps dispatched (the app answers with respondToInteraction), every other
// outcome gives failed. change is false when the state stays, and for any current state other than
// dispatched, so respondToInteraction, which may have left dispatched first, wins.
//
// Example:
//
//	to, change := domain.InteractionStateAfterOutcome(domain.InteractionDispatched, domain.AppOutcomeFacts{Answered: true})
func InteractionStateAfterOutcome(current string, o AppOutcomeFacts) (to string, change bool) {
	if current != InteractionDispatched {
		return current, false
	}
	switch {
	case o.Answered:
		return InteractionAcknowledged, true
	case o.Deferred:
		return InteractionDispatched, false
	}
	return InteractionFailed, true
}
