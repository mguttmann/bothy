# sdk

Status: planned. This folder contains no code yet.

## Purpose

SDKs for agent adapters, apps and tools that use the public API. Per language a core generated
from the specification plus a hand-written layer for gateway, invocation token, runs,
components, commands, idempotency, retries, webhook signatures and test doubles (ADR 0026).

| Folder | Language | State |
|---|---|---|
| `python/` | Python | planned |
| `typescript/` | TypeScript | planned |

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
