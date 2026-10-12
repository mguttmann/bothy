# Changelog

All notable changes to the Python SDK (`sdk/python`, distribution `bothy-sdk`) are
documented in this file. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this component adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). Its version is the
[`VERSION`](VERSION) file next to this one; release tags are `sdk-python/v<version>`
(docs/spec/18-release.md). Nothing is published yet (OQ-17).

## [Unreleased]

### Added

- Generated core `bothy_sdk.generated` from `docs/api/openapi.yaml` (openapi-python-client
  0.29.1, standard settings): models and sync and async functions of every REST operation.
- `make generate` and `make generate-check` (gate G4: drift check, compileall, mypy).
- Tolerance layer `bothy_sdk.tolerance`, installed on import (gate G5): unknown enum values are
  kept as pseudo members with the raw value, `null` members count as absent, `EventPage` skips
  events of unknown type; `decode_event`, `UnknownEvent`, `KNOWN_EVENT_TYPES`, `is_unknown`.
- Runtime dependencies `httpx` and `attrs`; Python 3.11 or later.
