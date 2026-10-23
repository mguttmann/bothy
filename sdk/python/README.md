# sdk/python

Status: generated core and tolerance layer (M2-23) and the hand layer with the FakeServer
(M2-29). The Python `webhooks` module of docs/spec/11-extensions.md 11.2 is not built yet. The
package is private and not published (OQ-17).

## Purpose

Python SDK for agent adapters (among them the Hermes adapter) and apps that use the public API.

## Layout

| Path | Content |
|---|---|
| `src/bothy_sdk/generated/` | Generated core: models and functions of every REST operation of `docs/api/openapi.yaml`, written by openapi-python-client 0.29.1. Never edited by hand. |
| `src/bothy_sdk/tolerance.py` | Tolerance layer (gate G5), installed on import of `bothy_sdk`. |
| `src/bothy_sdk/*.py` | Hand layer (M2-29): `adapter`, `inbox`, `invocation`, `gateway`, `sources`, `runs`, `context`, `components`, `commands`, `outbox`, `heartbeat`, `client`, `sync`, `retry`, `auth`, `errors`, `settings`, `clock`, `ids`; reference page [docs/sdk/python.md](../../docs/sdk/python.md). |
| `src/bothy_sdk/testing/` | FakeServer (extra `testing`), faults, scenario functions, doubles and recorded fixtures. |
| `src/bothy_sdk/__init__.py` | Package entry; calls `tolerance.install()`. |
| `tests/` | G4 and G5 tests (pytest); the fixtures are shared with the TypeScript SDK in [`../testdata/tolerance/`](../testdata/tolerance/README.md). |
| `openapi-python-client.yaml` | Generator configuration: standard settings, only the post hooks are set. |
| `osv-scanner.toml` | License data for three generator dependencies that OSV-Scanner reports as non-standard. |

Distribution `bothy-sdk`, import package `bothy_sdk` (working names, docs/spec/02-architecture.md
"Places that carry the product name"). The classifier `Private :: Do Not Upload` makes an index refuse
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

What a newer server may send is accepted (docs/spec/06-api.md 2 "Unknown values" and "No `null`"):

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
- Runtime dependencies are exactly what the core and the hand layer import: `httpx`, `attrs`,
  `PyJWT`, `cryptography` and `websockets`, as compatible ranges; the extra `testing` adds
  `aiohttp` for the FakeServer. The tested versions are in `uv.lock`.
- ruff, mypy, pytest, pytest-cov and PyYAML have the versions of `tools/pyproject.toml`.
- Coverage of the hand-written code: `quality.coverage.python_min_percent` (`fail_under`);
  generated code is outside coverage, ruff and bandit, but inside mypy.

## Hand layer (M2-29)

An agent adapter subclasses `AgentHandler` and runs an `Adapter`:

```python
from bothy_sdk.adapter import ActualBilling, Adapter, AdapterConfig, AgentHandler, StartOptions
from bothy_sdk.auth import Credentials, FileDpopKey, Secret


class Echo(AgentHandler):
    def actual_billing(self, inv):
        return ActualBilling("organization")  # what the runtime really uses for this run

    async def prepare(self, inv, run):
        await run.load_context()  # every request of the run carries its token (07 SR9)
        return StartOptions(model="model-small")

    async def execute(self, inv, run):  # called only after 200 on run.start
        run.patch.set_status_line("thinking")
        await run.send("Hello from the adapter")


config = AdapterConfig(
    base_url="https://example.com/api/v1",
    instance_id=...,  # the configured instance
    agent_id=...,  # the agent's principal ID
    credentials=Credentials(Secret.from_file("token"), FileDpopKey.load("dpop.pem")),
    consumer="main",
    state_dir=Path("state"),  # inbox.sqlite3 and outbox.sqlite3, mode 0600
)
reason = await Adapter(config, Echo()).run()  # returns the CloseReason, for example 4015
```

The adapter checks every invocation token itself (09 4.5), acknowledges an event only after it is
stored with its token in the inbox, starts a run at most once with the newest token, renews an
expired one, and ends a refused run with the reason of 09 4.5. `bothy_sdk.context.render_context`
turns the run context into a system part with trust marks and the trigger text.

## Tests

```sh
cd sdk/python && uv run --locked pytest
```

- Unit tests use `httpx.MockTransport` on `https://example.com`.
- `bothy_sdk.testing.FakeServer` runs in process on `127.0.0.1` port 0 with a `FakeClock`; it
  offers the faults "stored, then 502", "stored late", rate limit and connection loss
  (`Fault`), scenario helpers and fixtures recorded from the real server.
- The same scenario functions (`bothy_sdk.testing.scenarios`: AG-08, AG-09, AG-12, AG-28,
  AZ-P6) run against the FakeServer (`tests/test_scenarios_fake.py`) and against the real server
  of this tree (`tests/realserver`). The real-server tests build
  `services/server/test/sdkserver` with the Go tool chain and start PostgreSQL in a throwaway
  container; without Go or a Docker-compatible runtime they fail, they never skip.
- `BOTHY_SDK_RECORD_FIXTURES=1 uv run --locked pytest tests/realserver -k record` records the
  fixtures of `src/bothy_sdk/testing/fixtures/` again.

## Not built

The `webhooks` module (Standard Webhooks signatures `v1a`); helpers for agent sessions, file
upload and transcripts (adapters call the generated operations through `Client.call`).
