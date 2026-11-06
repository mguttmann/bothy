// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// HostCmd is the body of setAgentHost (schema HostSet).
type HostCmd struct {
	// HostedBy is the new hosting runner.
	HostedBy ids.ID
	Reason   string
	IfMatch  Precondition
}

// Hosting is the incoming port of the hosted agents (A1-03, docs/spec/26-builtin-agents.md 3):
// which Claude Code runner hosts which built-in agent, and the tokens of a built-in agent, which
// only its hosting runner receives. Every method authorizes the caller a with its action of 07
// section 2.13 and runs its writes in one unit of work with audit and events. Errors are
// kernel/errs errors.
type Hosting interface {
	// SetHost moves the built-in agent id to the runner cmd.HostedBy (agent.host.set, owner with
	// step-up) and revokes every token of the agent the old runner holds, in the same transaction.
	// Errors: validation_failed (/hosted_by host_not_capable, /reason), conflict with
	// not_a_builtin_agent, host_tool_class_ceiling or hosted_holder_mismatch, version_conflict.
	SetHost(ctx context.Context, a actor.Actor, id ids.ID, cmd HostCmd) (Agent, error)
	// ListHosted returns one page of the active agents whose hosted_by is the calling agent a
	// (agent.hosted.list, agent-self), ascending by id. Errors: cursor_invalid.
	ListHosted(ctx context.Context, a actor.Actor, p Page) (AgentPage, error)
	// IssueHostedToken issues a token of the built-in agent id to its hosting runner a
	// (agent.hosted_token.issue), bound to the DPoP key of a's token, and revokes the older ones
	// of that runner. Every caller other than the active hosting runner reporting hosts_agents
	// gets not_found, audited as a denial. Errors: not_found; conflict with dpop_required.
	//
	// Example:
	//
	//	issued, err := hosting.IssueHostedToken(ctx, runner, agentID)
	//	// issued.Secret.Reveal() is shown once in the answer, never logged.
	IssueHostedToken(ctx context.Context, a actor.Actor, id ids.ID) (identity.IssuedToken, error)
}
