# sdk/testdata/scenarios

The shared scenario list of both SDKs (docs/decisions.d/M2-30.md D-M2-30-1, ADR 0026 "Confirmation": the
same scenarios for both SDKs). Each scenario is one function of the SDK's `testing` module that
runs unchanged against the SDK's in-process FakeServer and against the real server of
`services/server/test/sdkserver`:

| SDK | Functions | Against the FakeServer | Against the real server |
|---|---|---|---|
| Python | `bothy_sdk.testing.scenarios` | `sdk/python/tests/test_scenarios_fake.py` | `sdk/python/tests/realserver/` |
| TypeScript | `src/testing/scenarios.ts` | `sdk/typescript/test/scenarios.test.ts` | `sdk/typescript/test/live/` |

Parse rule: every level-3 heading of the section "Scenarios" below is one scenario, named
exactly as the Python function. `sdk/python/tests/test_scenario_list.py` holds the Python
functions equal to these headings; `sdk/typescript/test/scenarios.test.ts` holds the TypeScript
functions equal to their camelCase form (`ag08_invalid_tokens` becomes `ag08InvalidTokens`). A
scenario added to one SDK only fails one of the two tests.

Common setup of every scenario: Alex mentions the agent Lotse in a new thread (`trigger`), which
queues a run with an invocation event; Texter is a second agent. Every scenario connects its
adapters with its own consumer names, so a run that one scenario leaves `queued` never reaches
another one (a new consumer starts at `head_seq`, docs/spec/04-sync-protocol.md 15.1 point 2).
Times are named by their configuration keys (`agent.ack_timeout`,
`agent.invocation_token_ttl`, `adapter.invocation_token_leeway`, `auth.clock_leeway`); the
FakeServer lets them pass on its fake clock, the real server in real time. A failed check raises
`ScenarioFailed` (Python) or an error of the scenario (TypeScript) whose message names the
check and never contains a token.

## Scenarios

### ag08_invalid_tokens

AG-08, adapter side (docs/spec/09-agents.md 16.2): no start on a missing or forged token.

Steps:

1. Start an adapter whose event source replaces the `invocation_token` of each `agent.invocation`
   before the adapter sees it.
2. For each variant `missing` (token removed), `bad_signature`, `alg_hs256`, `aud_other_agent`
   and `typ_access` (forged by the server under test): trigger a run and hand the adapter the
   token of the variant.
3. Trigger one more run that no adapter of the scenario receives and call `startRun` and
   `renewInvocationToken` on it directly with the token of each variant (an empty string for
   `missing`).

Expected outcomes:

- Each run of step 2 ends `failed` with `state_reason` `invalid_invocation_token`, was never
  started (`starts` 0), the runtime callbacks were never called, and the consumer cursor is
  acknowledged up to the event's `seq`.
- Each direct call of step 3 answers `403 invocation_token_invalid` with `reason: invalid`, and
  the run stays `queued`.

### ag09_double_delivery

AG-09: two adapter processes of one agent receive the same invocation; exactly one start.

Steps:

1. Start two adapters of Lotse with different consumers (`alpha`, `beta`), each with its own
   inbox and connection, and trigger a run.
2. Wait until both received the `agent.invocation` and until the runtime ran once and neither
   inbox holds an accepted, not started entry of the run.
3. Variant: start an adapter, then a second adapter with the same consumer name.

Expected outcomes:

- Both deliveries carry a token, and the two tokens differ (a fresh token per delivery).
- The runtime ran exactly once over both adapters; the run has `starts` 1 (the other `startRun`
  got `409 already_started` and its entry was closed).
- In the variant the first adapter stops with close code `4015` and does not reconnect.

### ag12_patch_rules

AG-12: forbidden paths and stale versions of run patches, over REST and the gateway; the
patcher rebases once.

Steps:

1. Start an adapter, trigger a run and hold the runtime inside its execution.
2. Patch `/state` through the run's patcher.
3. Send `patchRun` directly with `replace /state` and the current `base_version`, then with
   `replace /model` and a `base_version` one below the current one.
4. Move the version with a concurrent `patchRun` on `/status_line`, then set the model through
   the run's patcher and flush it.
5. Over a gateway connection send `run.update` with `replace /state`, then with a stale
   `base_version`.

Expected outcomes:

- Step 2 fails locally without a request (`PatchPathError` in TypeScript, `ValueError` in
  Python).
- Step 3: `400 validation_failed` with `errors[0].code` `patch_path_forbidden` and pointer
  `/ops/0`; then `409 base_version_conflict` whose `current` is the server's document at its
  current version.
- Step 4: the patcher did not see the concurrent change before its flush; after the flush the
  run holds both the concurrent `status_line` and the rebased `model`.
- Step 5: gateway `error` frames with code `validation_failed` and then
  `base_version_conflict`.

### ag28_slow_start_and_restart

AG-28: acknowledgement after acceptance into the inbox; a redelivery replaces the stored token;
the run starts exactly once.

Steps:

1. Start an adapter, trigger a run and hold the runtime before the start (in its preparation).
2. Let `agent.ack_timeout` pass.
3. Restart the server on the same data with the consumer cursor rewound before the event.
4. Call `startRun` directly with the first delivered token.
5. Let `agent.invocation_token_ttl` plus `auth.clock_leeway` pass, then release the runtime.

Expected outcomes:

- The event is acknowledged while the run is still `queued`, and it is not delivered again
  before the restart.
- After the restart the redelivery arrives; the inbox entry of the event counts at least two
  deliveries and holds the newer token; the runtime was prepared only once.
- Step 4 answers `403 invocation_token_invalid` with `reason: superseded`.
- After step 5 the adapter renews the expired token exactly once (`renewInvocationToken`),
  sends exactly one `startRun` with the renewed token, the run has `starts` 1 and the runtime
  executed once.

### azp6_adapter_never_starts

AZ-P6 for the SDK (docs/spec/07-permissions.md 11.2): the runtime never starts on a mismatching,
replayed or expired token. The missing and forged tokens are covered by `ag08_invalid_tokens`.

Steps:

1. An adapter whose actual billing mode is `unknown`; trigger a run.
2. An adapter that reports the billing mode that is not the claim's; trigger a run.
3. An adapter whose runtime loaded tool class `host`; trigger a run.
4. An adapter that holds the runtime in its preparation until the token has expired by more
   than `adapter.invocation_token_leeway`, then releases it.
5. Only against a server that can deliver a given token again under a new event ID (the
   FakeServer): deliver the token of a started run once more.

Expected outcomes:

- Steps 1 and 2: the run ends `failed` with `billing_mode_mismatch`, never started, the runtime
  never executed.
- Step 3: the run ends `failed` with `tool_class_mismatch`, never started, the runtime never
  executed.
- Step 4: exactly one renewal and one `startRun`; the run started with the renewed token, not
  the expired one, and has `starts` 1.
- Step 5: the replayed delivery is not acknowledged and does not reach the runtime again.

### rag19_allowlist_from_server

R-AG-19 for the SDK (docs/spec/09-agents.md 17.7): an adapter keeps its allow list from the
server's `getAgentTriggerers`, and the list admits the person the server let trigger. The name
carries the requirement ID because 09 16.2 has no scenario row for the allow list.

Steps:

1. Read `getAgentTriggerers` with the agent's own credentials (API token and DPoP).
2. An adapter with the allow list on (Python `AdapterConfig.allowlist`, TypeScript
   `AdapterConfig.allowlist`); trigger a run.

Expected outcomes:

- Step 1: `200` with an `AgentTriggerers` body: at least one person, no repeated ID, neither the
  agent nor the other agent, `permission_version` at least 1.
- Step 2: the adapter read `getAgentTriggerers` at least once; the run started exactly once
  (`starts` 1) and the runtime executed once.
