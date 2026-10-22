// SPDX-License-Identifier: Apache-2.0

package runs

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// CommandExec executes slash commands (docs/spec/09-agents.md section 10.4, M2-16). The caller is
// the principal of ctx (kernel/actor). A control command goes to its owner at once as the personal
// event command.invoked, also during a run, and never creates a run, a follow-up or a model turn; a
// model command opens a run queued with trigger_kind command like a triggering message and is
// refused with run_active while the thread has an active run. Every invocation authorizes
// command.invoke and, for agent commands, agent.invoke (I2, SR2, SR4).
//
// Example:
//
//	inv, err := exec.Invoke(ctx, runs.InvokeCommand{ChannelID: c, CommandID: cmd, ThreadID: t,
//		Options: []runs.OptionValue{{Name: "ticket", Value: "T-1"}}})
type CommandExec interface {
	// Invoke runs the command (invokeCommand). Errors: 400 validation_failed for a missing
	// thread_id where one is needed or an option value outside the definition, 403
	// agent_invocation_denied with the reason of the refusal, 403 channel_read_only in a read-only
	// or archived channel, 404 for an unknown, invisible, not covered or shadowed command, an
	// unknown thread or a thread of another channel.
	Invoke(ctx context.Context, cmd InvokeCommand) (CommandInvocation, error)
}

// InvokeCommand is the input of CommandExec.Invoke (schema CommandInvoke and the path).
type InvokeCommand struct {
	ChannelID, CommandID ids.ID
	// ThreadID is the target thread; zero when absent.
	ThreadID ids.ID
	// Options are the submitted option values in the order of the request.
	Options []OptionValue
}

// OptionValue is one option value (schema CommandOptionValue).
type OptionValue struct {
	Name, Value string
}

// CommandInvocation is the answer of an accepted invocation (schema CommandInvocation; accepted is
// always true, a refusal is an error).
type CommandInvocation struct {
	// InvocationID is new per accepted invocation and not stored in a table.
	InvocationID, CommandID, ChannelID, ThreadID ids.ID
	// RunID is the run of a model command; zero for a control command.
	RunID ids.ID
}
