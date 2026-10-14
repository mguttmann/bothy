# sdk

Status: generated cores and tolerance layers of both SDKs (M2-23); the hand layers follow with
M2-29 (Python) and M2-30 (TypeScript).

## Purpose

SDKs for agent adapters, apps and tools that use the public API. Per language a core generated
from the specification plus a hand-written layer for gateway, invocation token, runs,
components, commands, idempotency, retries, webhook signatures and test doubles (ADR 0026).

| Folder | Language | State |
|---|---|---|
| `python/` | Python | generated core (openapi-python-client) and tolerance layer; [README](python/README.md) |
| `typescript/` | TypeScript | generated types (openapi-typescript), openapi-fetch client and tolerance layer; [README](typescript/README.md) |
| `testdata/` | - | shared G5 fixtures of both SDKs; [README](testdata/tolerance/README.md) |

The clients of the apps (Dart) are also generated from the specification, but they belong to the
apps (docs/spec/15-client-architecture.md).

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
