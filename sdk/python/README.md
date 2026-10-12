# sdk/python

Status: generated core and tolerance layer (M2-23). The rest of the hand layer follows with
M2-29. The package is private and not published (OQ-17).

## Purpose

Python SDK for agent adapters (among them the Hermes adapter) and apps that use the public API.

## Layout

| Path | Content |
|---|---|
| `src/bothy_sdk/generated/` | Generated core: models and functions of every REST operation of `docs/api/openapi.yaml`, written by openapi-python-client 0.29.1. Never edited by hand. |
| `src/bothy_sdk/tolerance.py` | Tolerance layer (gate G5), installed on import of `bothy_sdk`. |
| `src/bothy_sdk/__init__.py` | Package entry; calls `tolerance.install()`. |
| `tests/` | G4 and G5 tests (pytest); the fixtures are shared with the TypeScript SDK in [`../testdata/tolerance/`](../testdata/tolerance/README.md). |
| `openapi-python-client.yaml` | Generator configuration: standard settings, only the post hooks are set. |
| `osv-scanner.toml` | License data for three generator dependencies that OSV-Scanner reports as non-standard. |

Distribution `bothy-sdk`, import package `bothy_sdk` (working names, docs/spec/02-architektur.md
"Stellen mit dem Produktnamen"). The classifier `Private :: Do Not Upload` makes an index refuse
an accidental upload.

## Regenerate

After every change of `docs/api/openapi.yaml`:

```sh
make -C sdk/python generate        # rewrite src/bothy_sdk/generated/
make -C sdk/python generate-check  # gate G4: no drift, compileall, mypy
```

`make api-clients` runs `generate-check` in every `make check`. The generator is pinned exactly
in the dev group of `pyproject.toml` and locked in `uv.lock`; Renovate updates it in its own group
("SDK code generators"), and that pull request regenerates the core in the same change.

## Tolerance (gate G5)

What a newer server may send is accepted (docs/spec/06-api.md 2 "Unbekanntes" and "Kein null"):

- Unknown field: ignored; it stays in `additional_properties` of the model.
- Unknown enum value: every generated enum returns a pseudo member that carries the raw string
  (`.value`, `str()`, `to_dict()`); `tolerance.is_unknown(value)` is true for it.
- Unknown event type: `EventPage.from_dict`, and with it the generated `listEvents` and
  `pollAgentEvents`, skips the event; advance with `next_cursor`. `tolerance.decode_event`
  classifies one event by `type` and returns `UnknownEvent` for an unknown type.
- JSON `null`: a member with value `null` counts as absent (`UNSET`).

Not guaranteed: the one-member `type` enum of every event schema stays strict, because the union
parser of the core tries the event models in order and would otherwise accept an unknown event as
the first model whose payload fits. Malformed data (a missing required member, a value of the
wrong JSON type) still raises. Unknown gateway ops and close codes are handled by the gateway
module of M2-29.

Difference to the TypeScript SDK: here the generated page already skips unknown events (the core
would raise otherwise); in TypeScript the page is passed through and `knownEvents` or
`decodeEvent` skip them.

## Runtimes and dependencies

- `requires-python = ">=3.11"`. On 2026-10-12 python.org lists 3.10 as the oldest maintained
  line, but the generated core imports `enum.StrEnum` and parses times ending in `Z` with
  `datetime.fromisoformat`, both new in 3.11.
- Runtime dependencies are exactly what the core and the hand layer import: `httpx` and `attrs`,
  as compatible ranges; the tested versions are in `uv.lock`.
- ruff, mypy, pytest, pytest-cov and PyYAML have the versions of `tools/pyproject.toml`.
- Coverage of the hand-written code: `quality.coverage.python_min_percent` (`fail_under`);
  generated code is outside coverage, ruff and bandit, but inside mypy.

## Next (M2-29)

Hand layer modules `auth`, `gateway`, `invocation`, `runs`, `components`, `commands`, `outbox`,
`retry`, `heartbeat`, `webhooks`, `testing` (docs/spec/11-erweiterungen.md 11.2), the client
defaults from `sdk.*`, the deprecation warning and the API reference in MkDocs.
