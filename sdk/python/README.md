# sdk/python

Status: generated core and tolerance layer (M2-23), the hand layer with the FakeServer (M2-29)
and the `webhooks` module of docs/spec/11-extensions.md 11.2 (M2-50). The package is private and
not published (OQ-17).

## Purpose

Python SDK for agent adapters (among them the Hermes adapter) and apps that use the public API.

## Layout

| Path | Content |
|---|---|
| `src/bothy_sdk/generated/` | Generated core: models and functions of every REST operation of `docs/api/openapi.yaml`, written by openapi-python-client 0.29.1. Never edited by hand. |
| `src/bothy_sdk/tolerance.py` | Tolerance layer (gate G5), installed on import of `bothy_sdk`. |
| `src/bothy_sdk/*.py` | Hand layer (M2-29): `adapter`, `inbox`, `invocation`, `gateway`, `sources`, `runs`, `context`, `components`, `commands`, `outbox`, `heartbeat`, `client`, `sync`, `retry`, `auth`, `errors`, `settings`, `clock`, `ids`, and `webhooks` (M2-50); reference page [docs/sdk/python.md](../../docs/sdk/python.md). |
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
import asyncio
from pathlib import Path

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


async def main() -> None:
    config = AdapterConfig(
        base_url="https://example.com/api/v1",
        instance_id=...,  # the configured instance
        agent_id=...,  # the agent's principal ID
        credentials=Credentials(Secret.from_file("token"), FileDpopKey.load("dpop.pem")),
        consumer="main",
        state_dir=Path("state"),  # inbox.sqlite3 and outbox.sqlite3, mode 0600
    )
    reason = await Adapter(config, Echo()).run()  # returns the CloseReason, for example 4015
    print(reason)


asyncio.run(main())
```

The adapter checks every invocation token itself (09 4.5), acknowledges an event only after it is
stored with its token in the inbox, starts a run at most once with the newest token, renews an
expired one, and ends a refused run with the reason of 09 4.5. `bothy_sdk.context.render_context`
turns the run context into a system part with trust marks and the trigger text.

Stop and follow-ups (M2-35, docs/DECISIONS.md D-M2-35-12): `run.cancel_requested` for a run that
has not started cancels its start and sends `finishRun` `cancelled` with the request's reason; for
a started run a handler that overrides `AgentHandler.on_cancel(run, reason)` keeps `execute`
running, stops its runtime and finishes the run itself, while without the override the adapter
cancels `execute` and finishes. A second request for the same run changes nothing. A follow-up of
a run that this process holds but has not started yet reaches `on_followup` too, so a handler can
keep it until its runtime starts.

## Webhook verification (M2-50)

`bothy_sdk.webhooks` checks a delivery of an outgoing webhook or a request to `interactions_url`
the way docs/api/webhooks.md 3 describes it, with the behaviour of the TypeScript module
(M2-30). `getWebhookKeys` is public and an app token has no scope for it, so the key cache uses
a generated client without a token:

```python
from collections.abc import Mapping

from bothy_sdk.generated.client import Client as CoreClient
from bothy_sdk.webhooks import WebhookKeyCache, WebhookVerificationError, verify_webhook

keys = WebhookKeyCache(CoreClient(base_url="https://chat.example.com/api/v1"))


async def receive(raw_body: bytes, headers: Mapping[str, str]) -> int:
    try:
        # The raw body exactly as received, never parsed and serialized again.
        verified = verify_webhook(raw_body, headers, keys=await keys.keys())
    except WebhookVerificationError:
        return 401
    # The app remembers verified.id against duplicates, then handles verified.envelope.
    return 204
```

`v1` secrets go in as `secrets=["whsec_..."]`. The tolerance is `webhook.receiver_tolerance`
(`SdkSettings.webhook_receiver_tolerance`); the key list is fetched again only after the
`max-age` of its answer (without one `agent.jwks_max_age`), never because a verification failed.
Tests sign deliveries with `bothy_sdk.testing.sign_webhook` and `webhook_public_key`.

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

## Test doubles: the Fake-Agent (M2-31)

`bothy_sdk.testing.fake_agent` is the scriptable adapter of the agent conformance suite
(docs/spec/09-agents.md 16.1, `services/server/test/agents`). The suite starts one process per
agent and consumer with `uv run --project sdk/python --locked --quiet python -m
bothy_sdk.testing.fake_agent` and drives it over JSON lines: one request `{id, op, args, now}`
per line on stdin, one reply `{id, ok, result}` or `{id, ok: false, error: {kind, detail}}` per
line on stdout (`bothy_sdk.testing.control`). The first request is `hello` with the API base, the
instance and agent IDs, consumer, transport (`rest` or `gateway`), the runtime's actual billing
mode, evidence and tool class, SDK settings and the credentials; credentials travel only there,
never in `argv` or the environment. `now` is the harness clock: the Fake-Agent sets its SDK clock
to it before every op (token expiry, DPoP `iat`, the JWKS `max-age`).

The ops use the SDK: the invocation verifier and the ack window for `receive` and `verify`, the
gateway client, the patcher's rebase, the card builders, the command declarations and the context
rendering; requests the hand layer would never send (forged, empty or absent tokens of
`bothy_sdk.testing.token_faults`, a stale `base_version`, a start past the adapter's own checks)
go out as plain requests of the same authenticated client. The ops and their members are listed in
`services/server/test/agents/README.md`. Logs go to stderr at level `WARNING` and never carry a
token, a credential, file content or a transcript; only `export_token` returns a token.
`tests/testing/` tests the Fake-Agent against the FakeServer.

For files and voice messages (M3-09) the Fake-Agent has `read_file` (`downloadFile`; the reply
holds only `status`, the problem members and the byte length `bytes`, never the content) and
`submit_transcript` (`getFile` for the transcript `ETag`, then `submitTranscript` with that
`If-Match`, or the value of `if_match`, `absent` for none). `context` renders the run context with
the texts it read (`render_context(..., file_texts=...)`) and lists per element its `attachments`
with `file_id`, `transcript` (present or not) and where their texts stand in the composed prompt:
`content_marked_only` for a file read with `read_file`, `transcript_marked_only` and
`transcript_in_trigger` for a transcript (ADR 0054 point 4: file content is always data, a
transcript carries the mark of its message). The FakeServer serves `getFile` and
`submitTranscript` with the derived transcript `ETag` (`FakeServer.file_etag`).

## Not built

Helpers for agent sessions, file upload and transcripts (adapters call the generated operations through `Client.call`).
