# sdk

Status: generated cores and tolerance layers of both SDKs (M2-23); the Python hand layer with the
FakeServer (M2-29) and its `webhooks` module (M2-50); the TypeScript hand layer with the webhook
verifier and FakeServer (M2-30). Both run the same scenarios against their FakeServer and the real server.

## Purpose

SDKs for agent adapters, apps and tools that use the public API. Per language a core generated
from the specification plus a hand-written layer for gateway, invocation token, runs,
components, commands, idempotency, retries, webhook signatures and test doubles (ADR 0026).

| Folder | Language | State |
|---|---|---|
| `python/` | Python | generated core (openapi-python-client), tolerance layer and hand layer with FakeServer and scenario tests (M2-29), the module `webhooks` (M2-50); [README](python/README.md) |
| `typescript/` | TypeScript | generated types (openapi-typescript), openapi-fetch client, tolerance layer and hand layer (modules of 11 11.2 including `webhooks`) with FakeServer, scenario tests and typedoc reference (M2-30); [README](typescript/README.md) |
| `testdata/` | - | shared files of both SDKs: G5 fixtures ([README](testdata/tolerance/README.md)), the scenario list ([README](testdata/scenarios/README.md)) and the context golden files ([README](testdata/context/README.md)) |

The clients of the apps (Dart) are also generated from the specification, but they belong to the
apps (docs/spec/15-client-architecture.md).

The guide [Connect your own agent](../docs/guides/connect-agent.md) builds an adapter on the
Python SDK step by step; its example lives in `python/examples/guide/` and runs as a test.

## Contract

- Design: docs/spec/11-extensions.md section 11.
- Agent contract that the hand-written layer implements: docs/spec/09-agents.md.
- Source of truth: `docs/api/openapi.yaml` and `docs/api/asyncapi.yaml`.

## Versioning

SemVer per language, own changelog per folder. Rules: docs/spec/11-extensions.md
section 12; deprecation according to docs/spec/06-api.md.

## Compatibility matrix

No published version yet. Every row gets a test in the gate that runs the contract tests
of the SDK version against the named server version.

| SDK | Version | API | Gateway protocol | Minimum server | Status |
|---|---|---|---|---|---|
| Python | none yet | | | | |
| TypeScript | none yet | | | | |

Status values: `supported`, `deprecated`, `end-of-life`.

## Package names

The names on PyPI and npm depend on the final product name and are not reserved yet
(question to the owner).

Working names until OQ-17 is decided: Python distribution `bothy-sdk` (import package
`bothy_sdk`) and npm package `bothy-sdk`. Both packages are private (`Private :: Do Not Upload`,
`"private": true`) and are not published; they are a listed kind of "places that carry the
product name" in docs/spec/02-architecture.md section 6.
