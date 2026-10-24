---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team agent platform
---

# SDKs: generated core plus hand-written layer (Python, TypeScript)

## Context and Problem Statement

Agent adapters and apps speak the public API. The hard part is not the single REST call but
the protocol around it: gateway with cursor and `ack`, check of the invocation token, live box
patches with throttling and version, idempotency, retries, signatures of outgoing webhooks. How
does the product deliver SDKs that keep pace with the specification and solve these parts
correctly once?

## Decision Drivers

- OpenAPI 3.1 and AsyncAPI 3.0 are normative (B2, ADR 0013); SDKs must not deviate from them.
- Security-critical parts (checking the token, signatures, idempotency) should not be built by
  every adapter itself.
- The Hermes adapter needs Python, integrations mostly TypeScript (Baseline B2: Python for the SDK
  and the Hermes adapter).
- Little maintenance for new endpoints.

## Considered Options

1. Generated core from the spec (Python: openapi-python-client, TypeScript: openapi-typescript)
   plus a hand-written layer for gateway, token, runs, components, commands, idempotency,
   retries, webhook signatures and test doubles.
2. Fully hand-written SDKs.
3. Only generated clients without a hand-written layer.
4. Only one SDK (Python); other languages use the raw API.

## Decision Outcome

Chosen: option 1, because the core grows with the spec without manual work and the hard
protocol parts are built and tested exactly once per language. The generators follow the
lead's decision R-Clients.

### Consequences

- Good, because new operations are in the SDK with the next generation run.
- Good, because adapters do not build the token, the gateway and the patch discipline themselves.
- Good, because a FakeServer in the SDK allows adapter tests without a real server.
- Bad, because two hand-written layers are maintained in parallel; a shared set of
  contract scenarios (docs/spec/09-agents.md section 16) checks both in the same way.
- Bad, because a breaking change of the spec makes the hand-written layer fail to compile; that
  is intended, because the error shows up in the gate instead of at the user.

### Confirmation

- Gate: generate and compile the core from `docs/api/openapi.yaml` (R-Clients, ADR 0038).
- Gate: the hand-written layer tests of both SDKs run the same scenarios against the FakeServer
  and against a real server.
- Lint: the hand-written layer defines no types of its own for objects that the core already
  generates.
- Python (M2-29): the scenario functions of `bothy_sdk.testing.scenarios` (AG-08, AG-09, AG-12,
  AG-28, AZ-P6) run unchanged in `sdk/python/tests/test_scenarios_fake.py` against the FakeServer
  and in `sdk/python/tests/realserver/test_realserver_scenarios.py` against the server of the
  tree (`services/server/test/sdkserver`); `sdk/python/tests/test_hand_layer_types.py` is the type
  lint (no class of the hand layer carries a schema name of `openapi.yaml`).
- TypeScript (M2-30): the ported scenario functions of `sdk/typescript/src/testing/scenarios.ts`
  run in `sdk/typescript/test/scenarios.test.ts` against the TypeScript FakeServer and in
  `sdk/typescript/test/live/scenarios.live.test.ts` against the same `sdkserver`;
  `sdk/typescript/test/types.test.ts` is the type lint (no exported type or interface of the hand
  layer carries a schema name of `openapi.yaml`). Both SDKs check their scenario names against
  `sdk/testdata/scenarios/README.md` and render the shared golden files of `sdk/testdata/context/`.
- Compatibility matrix in `sdk/README.md` with a test per row (docs/spec/11-extensions.md
  section 12).

## Pros and Cons of the Options

### Option 1: Generated core plus hand-written layer

- Good, because fidelity to the spec and convenience come together.
- Neutral, because generators have their own quirks (keep the spec conservative, Baseline R-Stack).
- Bad, because two layers must be explained.

### Option 2: Fully by hand

- Good, because the API becomes as ergonomic as one wants.
- Bad, because every spec change must be followed up by hand, and deviations arise
  silently.

### Option 3: Only generated

- Good, because there is no maintenance effort.
- Bad, because every adapter builds gateway, token, idempotency and patches itself and can
  make security mistakes in the process.

### Option 4: Only Python

- Good, because only one hand-written layer is maintained.
- Bad, because integrations in the webhook environment are often written in TypeScript, and the
  signature check and the gateway would be missing there.

## More Information

- docs/spec/11-extensions.md sections 11 and 12; Baseline B2, R-Clients.
- Package names and registrations at PyPI and npm depend on the final product name (question for
  the owner).
