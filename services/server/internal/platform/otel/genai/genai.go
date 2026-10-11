// SPDX-License-Identifier: Apache-2.0

package genai

import "slices"

// ConventionRef names the state of the GenAI conventions these keys follow, as
// docs/observability/signals.md "GenAI-Konventionen" item 1 read it. No commit was recorded
// during planning; a ticket with network access that reads one adds it here and in signals.md.
const ConventionRef = "open-telemetry/semantic-conventions-genai, status Development, read 2026-10-10, commit not recorded"

// The gen_ai.* keys the server may send: metadata only, never content. Each one is in the
// redaction/allowlist of infra/otel/collector.yaml.
//
//nolint:gosec // G101: the *Tokens names are telemetry keys that count model tokens, not credentials
const (
	// OperationName is the kind of operation, for example "invoke_agent" or "execute_tool".
	OperationName = "gen_ai.operation.name"
	// ProviderName is the model provider, "anthropic" for Anthropic.
	ProviderName = "gen_ai.provider.name"
	// RequestModel is the model the request asked for.
	RequestModel = "gen_ai.request.model"
	// ResponseModel is the model that answered.
	ResponseModel = "gen_ai.response.model"
	// ResponseFinishReasons lists why the model stopped.
	ResponseFinishReasons = "gen_ai.response.finish_reasons"
	// UsageInputTokens counts the input tokens.
	UsageInputTokens = "gen_ai.usage.input_tokens"
	// UsageOutputTokens counts the output tokens.
	UsageOutputTokens = "gen_ai.usage.output_tokens"
	// UsageCacheReadInputTokens counts the input tokens read from the prompt cache.
	UsageCacheReadInputTokens = "gen_ai.usage.cache_read.input_tokens"
	// UsageCacheWriteInputTokens counts the input tokens written to the prompt cache.
	UsageCacheWriteInputTokens = "gen_ai.usage.cache_write.input_tokens"
	// AgentID is the id of the agent.
	AgentID = "gen_ai.agent.id"
	// AgentName is the name of the agent (an agent is no person, privacy.md section 3 item 4).
	AgentName = "gen_ai.agent.name"
	// ConversationID is the id of the conversation.
	ConversationID = "gen_ai.conversation.id"
	// ToolName is the name of a tool, never its arguments or result.
	ToolName = "gen_ai.tool.name"
)

// Forbidden lists the gen_ai.* keys that carry content and are never sent
// (docs/observability/signals.md "GenAI-Konventionen" item 4). The collector deletes them in
// transform/scrub as a second line. Callers must not modify the slice.
var Forbidden = []string{
	"gen_ai.input.messages",
	"gen_ai.output.messages",
	"gen_ai.system_instructions",
	"gen_ai.tool.definitions",
	"gen_ai.tool.call.arguments",
	"gen_ai.tool.call.result",
}

// keys is the set of constants above, sorted.
var keys = []string{
	AgentID, AgentName, ConversationID, OperationName, ProviderName, RequestModel,
	ResponseFinishReasons, ResponseModel, ToolName, UsageCacheReadInputTokens,
	UsageCacheWriteInputTokens, UsageInputTokens, UsageOutputTokens,
}

// Keys returns the gen_ai.* keys the server may send, sorted. The result is a copy.
func Keys() []string { return slices.Clone(keys) }

// Is reports whether key is one of the keys the server may send. Forbidden keys are not.
func Is(key string) bool {
	_, found := slices.BinarySearch(keys, key)
	return found
}
