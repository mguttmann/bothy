# Package `platform/otel/genai`

Status: built (M1-10). Normative: [17-observability.md](../../../../../../docs/spec/17-observability.md)
"Grundregeln" rule 5 and [signals.md](../../../../../../docs/observability/signals.md)
"GenAI-Konventionen (gekapselt)"; on a difference the documents win.

## Purpose

The one place of the server that spells `gen_ai.*` telemetry keys. The OpenTelemetry GenAI
conventions are in status "Development"; when they change, this package, its snapshot, the
collector allowlist and the dashboards change in one change, and no other code does.

## Interface

- 13 constants for the keys the server may send, metadata only: `OperationName`, `ProviderName`,
  `RequestModel`, `ResponseModel`, `ResponseFinishReasons`, `UsageInputTokens`,
  `UsageOutputTokens`, `UsageCacheReadInputTokens`, `UsageCacheWriteInputTokens`, `AgentID`,
  `AgentName`, `ConversationID`, `ToolName`.
- `Keys()` returns them sorted; `Is(key)` reports membership.
- `Forbidden` lists the keys that carry content and are never sent: `gen_ai.input.messages`,
  `gen_ai.output.messages`, `gen_ai.system_instructions`, `gen_ai.tool.definitions`,
  `gen_ai.tool.call.arguments`, `gen_ai.tool.call.result`.
- `ConventionRef` names the convention repository, its status and the date it was read. The
  commit is not recorded yet (signals.md marks it open); a ticket with network access adds it.

## Tests

- `TestGenAIKeysSnapshot` compares the keys, the forbidden keys and `ConventionRef` with
  `testdata/keys.golden`. After a deliberate change run
  `go test ./internal/platform/otel/genai -update` and update the collector allowlist and the
  dashboards in the same change.
- `TestGenAIKeysAllowedOrScrubbed` (in package `attr`, which reads the collector file once):
  every constant is in `redaction/allowlist.allowed_keys` of `infra/otel/collector.yaml`, and
  every forbidden key is deleted by `transform/scrub` (and not allowed).

## Dependencies

Standard library only. Package `platform/otel/attr` uses `Is` for `attr.Allowed`.
