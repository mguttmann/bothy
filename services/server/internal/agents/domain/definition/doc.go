// SPDX-License-Identifier: Apache-2.0

// Package definition is the pure model of the agent definition of a built-in agent: the closed
// schema agent_definition.v1 (docs/spec/26-builtin-agents.md 4.2), its refusals (26 4.3) with one
// field error per finding, the class rule of a stdio MCP server (26 4.4 item 4) and the tool list
// intersection of 26 4.4 item 2. It does no I/O; every limit comes from Limits, which the server
// binds from the configuration keys agents.definition.* and agents.mcp_stdio_runners.
//
// The rules are the conformance set of every writer: the server (A1-02), the runner (A1-05), the
// template gate (A1-06) and the interview (A1-07) hold the same corpus,
// tests/agent-definitions at the repository root.
//
// Example:
//
//	def, findings := definition.Parse(raw, limits, definition.Scoped)
//	if len(findings) > 0 {
//		// 400 validation_failed with one errors[] entry per finding
//	}
package definition
