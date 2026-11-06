// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The adapter kinds the hosting rules name (03 agents.adapter_kind, R-DM-58).
const (
	// AdapterClaudeAgent marks a built-in agent that a Claude Code runner hosts
	// (docs/spec/26-builtin-agents.md 3.1).
	AdapterClaudeAgent = "claude_agent"
	// AdapterCodeRunner is the Claude Code runner (09 section 18), the only kind that can host.
	AdapterCodeRunner = "code_runner"
)

// PointerHostedBy is the JSON pointer of the hosting runner in createAgent and setAgentHost.
const PointerHostedBy = "/hosted_by"

// FieldHostNotCapable is the field code of a hosted_by that names no capable runner (26 3.1 item 2).
const FieldHostNotCapable = "host_not_capable"

// The reasons of the 409 conflicts of hosting (26 3.1, 3.3, 4.1 and 10 section 10.3).
const (
	// ReasonHostToolClassCeiling refuses a hosted agent's class above its runner's hosts_agents.
	ReasonHostToolClassCeiling = "host_tool_class_ceiling"
	// ReasonHostedHolderMismatch refuses personal for a hosted agent without its runner's holder
	// (BR-14).
	ReasonHostedHolderMismatch = "hosted_holder_mismatch"
	// ReasonNotABuiltinAgent refuses a hosting action on an agent that is not built in.
	ReasonNotABuiltinAgent = "not_a_builtin_agent"
)

// CheckHostedPairing holds R-DM-58 for a new record: hosted_by is set exactly when the adapter
// kind is claude_agent. Errors: validation_failed on /hosted_by with required (claude_agent without
// a runner) or not_allowed (a runner for another kind).
//
// Example:
//
//	err := domain.CheckHostedPairing(cmd.AdapterKind, cmd.HostedBy)
func CheckHostedPairing(adapterKind string, hostedBy ids.ID) error {
	switch {
	case adapterKind == AdapterClaudeAgent && hostedBy.IsZero():
		return fieldError(PointerHostedBy, FieldRequired, "a built-in agent needs its hosting runner")
	case adapterKind != AdapterClaudeAgent && !hostedBy.IsZero():
		return fieldError(PointerHostedBy, FieldNotAllowed, "only a built-in agent (claude_agent) has a hosting runner")
	}
	return nil
}

// CheckHost reports whether host may host the agent self (26 3.1 item 2): found and active, of
// kind code_runner, not hosted itself, not self, and its last stored heartbeat reports
// hosts_agents. active is the status of the host's principal (identity). Errors: validation_failed
// on /hosted_by with host_not_capable, the same for every cause, so the answer names no state of
// another agent.
func CheckHost(self ids.ID, host Agent, found, active bool) error {
	if !found || !active || host.ID == self || host.AdapterKind != AdapterCodeRunner || !host.HostedBy.IsZero() ||
		!host.Reported.HostsAgents.Valid() {
		return fieldError(PointerHostedBy, FieldHostNotCapable, "the hosting runner must be an active Claude Code runner that reports hosts_agents")
	}
	return nil
}

// CheckCeiling refuses a hosted agent's tool class above the ceiling its runner reports in
// hosts_agents (26 3.1 item 3). A runner that reports no ceiling admits only chat, the class every
// new agent starts with (docs/decisions.d/A1-03.md D-A1-03-6). Errors: conflict with
// host_tool_class_ceiling.
func CheckCeiling(class, ceiling ToolClass) error {
	if class == ToolChat || class.rank() <= ceiling.rank() {
		return nil
	}
	return errs.New(errs.CodeConflict, "The tool class is above what the hosting runner loads for a hosted agent.").
		WithReason(ReasonHostToolClassCeiling)
}

// CheckHostedHolder holds 10 section 10.3 item 2 for a hosted agent with the policy p under the
// runner host: personal only when the runner is personal with the same credential holder and
// reports personal in billing_modes_supported, because the turn runs with the login of the
// runner's host user. organization is always allowed. Errors: conflict with
// hosted_holder_mismatch.
func CheckHostedHolder(p Policy, host Agent) error {
	if p.Mode != Personal {
		return nil
	}
	if host.Policy.Mode != Personal || host.Policy.HolderID != p.HolderID || !slices.Contains(host.Reported.BillingModesSupported, Personal) {
		return hostedHolderMismatch()
	}
	return nil
}

// CheckRunnerHolders holds 10 section 10.3 item 2 for a runner whose policy becomes p: every
// personal agent of hosted keeps the runner's holder. Errors: conflict with
// hosted_holder_mismatch.
func CheckRunnerHolders(p Policy, hosted []Agent) error {
	for _, h := range hosted {
		if h.Policy.Mode == Personal && (p.Mode != Personal || p.HolderID != h.Policy.HolderID) {
			return hostedHolderMismatch()
		}
	}
	return nil
}

// hostedHolderMismatch is the refusal of BR-14.
func hostedHolderMismatch() error {
	return errs.New(errs.CodeConflict, "A personal built-in agent needs the credential holder of its hosting runner.").
		WithReason(ReasonHostedHolderMismatch)
}

// NotABuiltinAgent is the refusal of a hosting action on an agent whose adapter kind is not
// claude_agent: conflict with not_a_builtin_agent.
func NotABuiltinAgent() error {
	return errs.New(errs.CodeConflict, "Only a built-in agent has a hosting runner.").WithReason(ReasonNotABuiltinAgent)
}
