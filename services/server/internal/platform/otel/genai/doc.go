// SPDX-License-Identifier: Apache-2.0

// Package genai is the one place of the server that names gen_ai.* telemetry keys
// (docs/spec/17-observability.md "Grundregeln" rule 5, docs/observability/signals.md
// "GenAI-Konventionen (gekapselt)"). The OpenTelemetry GenAI conventions are in status
// "Development"; a change of the convention is a change of this package, of its snapshot
// testdata/keys.golden, of the collector allowlist and of the dashboards, in one change.
//
// Every other package uses the constants of this package and never spells a gen_ai.* key itself.
// The keys in Forbidden carry content (prompts, answers, system instructions, tool definitions,
// tool arguments and results) and are never sent; the collector deletes them as well.
//
// Example:
//
//	span.SetAttributes(attribute.String(genai.ProviderName, "anthropic"))
package genai
