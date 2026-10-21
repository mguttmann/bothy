# Changelog

All notable changes to the Python SDK (`sdk/python`, distribution `bothy-sdk`) are
documented in this file. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this component adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). Its version is the
[`VERSION`](VERSION) file next to this one; release tags are `sdk-python/v<version>`
(docs/spec/18-release.md). Nothing is published yet (OQ-17).

## [Unreleased]

### Security

- The locked `certifi` (through `httpx`) is 2026.10.19, the fixed release of
  [GHSA-6q48-6c7w-9gw2](https://github.com/advisories/GHSA-6q48-6c7w-9gw2).

### Added

- Generated models of the personal event `gdpr.request_due` (R-AZ-35).
- Run loop hooks for the Claude Code runner (M2-34): `RunRefused(reason)` ends a run from
  `prepare` with exactly that reason (or, without one, closes the entry without `finishRun`);
  `AgentHandler.on_not_started` frees what `prepare` took when a run does not reach `execute`;
  `AgentHandler.runtime_status` with `RuntimeStatus` supplies the heartbeat members that change
  at run time (`state`, degraded reasons, `runtime_version`, `billing_modes_supported`,
  `project_readiness`); `Adapter.send_heartbeat` sends one at once; `AgentHandler.on_ready`
  follows every ready source.
- Generated core `bothy_sdk.generated` from `docs/api/openapi.yaml` (openapi-python-client
  0.29.1, standard settings): models and sync and async functions of every REST operation.
- `make generate` and `make generate-check` (gate G4: drift check, compileall, mypy).
- Tolerance layer `bothy_sdk.tolerance`, installed on import (gate G5): unknown enum values are
  kept as pseudo members with the raw value, `null` members count as absent, `EventPage` skips
  events of unknown type; `decode_event`, `UnknownEvent`, `KNOWN_EVENT_TYPES`, `is_unknown`.
- Runtime dependencies `httpx` and `attrs`; Python 3.11 or later.
