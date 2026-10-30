# Python SDK

Reference of the Python SDK `bothy_sdk` (docs/spec/11-extensions.md section 11, ADR 0026).
`make docs` renders it from the google docstrings of the source with mkdocstrings. Status,
examples and how to run the tests of the SDK are in
[sdk/python/README.md](../../sdk/python/README.md).

## Two layers

The SDK has two layers. The generated core `bothy_sdk.generated` holds the models and functions
of every REST operation and the gateway frames; openapi-python-client writes it from
[openapi.yaml](../api/openapi.yaml), and it is never edited by hand. The hand layer on top of it
calls REST only through the core and adds what every agent adapter needs: credentials and DPoP,
retries, the gateway connection, the check of the invocation token, the inbox, the acceptance
and run loop, the run context and the test support.

The generated core has no section on this page: [openapi.yaml](../api/openapi.yaml) is its
documentation, and the docs site renders it as the pages "REST reference", "Schemas" and
"Gateway reference" under API ([api/README.md](../api/README.md)). Its generated docstrings
would only repeat that.

## Modules of the hand layer

| Module | Content |
|---|---|
| `settings` | Named values the hand layer works with, as defaults; the server is the authority. |
| `clock` | Time source, replaceable in tests. |
| `ids` | UUIDv7 for the identifiers the adapter chooses. |
| `auth` | Credentials: the API token, optional DPoP proofs and the nonce handling. |
| `errors` | Typed errors after the problem format of RFC 9457. |
| `retry` | Retry policy of the REST client and the automatic `Idempotency-Key`. |
| `client` | Asynchronous REST client on top of the generated core. |
| `sync` | Synchronous hull of the REST client for scripts. |
| `gateway` | Gateway connection: `hello`, `identify`, `resume`, heartbeat, reconnect and close codes. |
| `sources` | Event sources of an adapter: the gateway or REST polling. |
| `invocation` | Check of the invocation token, with the JWKS cache and the `jti` store. |
| `inbox` | Accepted events, their newest tokens and the `jti` store. |
| `adapter` | The acceptance and run loop of an agent adapter. |
| `runs` | The live-box patcher and the handle of a running run. |
| `context` | The run context with trust marks for the agent runtime. |
| `components` | Message components of `component.v1`. |
| `commands` | Slash commands: a declared list, reconciliation and autocomplete. |
| `outbox` | Writes that must never go out twice or get lost. |
| `long_answers` | A long answer stays one message: the head as its text, the rest above `message_max_content_bytes` as one attachment `answer.txt` (M4-47). |
| `heartbeat` | The heartbeat building block for `agent_heartbeat.v1`. |
| `tolerance` | Tolerance of the generated core for what a newer server may send (gate G5). |
| `webhooks` | Standard Webhooks verification for apps: `verify_webhook` and the key cache (M2-50). |
| `testing` | Test support (extra `testing`): FakeServer, faults, scenario functions, doubles, the Fake-Agent of the agent conformance suite (M2-31) and the signing of test webhook deliveries (M2-50). |

## Reference

::: bothy_sdk.settings

::: bothy_sdk.clock

::: bothy_sdk.ids

::: bothy_sdk.auth

::: bothy_sdk.errors

::: bothy_sdk.retry

::: bothy_sdk.client

::: bothy_sdk.sync

::: bothy_sdk.gateway

::: bothy_sdk.sources

::: bothy_sdk.invocation

::: bothy_sdk.inbox

::: bothy_sdk.adapter

::: bothy_sdk.runs

::: bothy_sdk.context

::: bothy_sdk.components

::: bothy_sdk.commands

::: bothy_sdk.outbox

::: bothy_sdk.uploads

::: bothy_sdk.long_answers

::: bothy_sdk.heartbeat

::: bothy_sdk.tolerance

::: bothy_sdk.webhooks

::: bothy_sdk.testing
    options:
      # The names it exports are documented in the submodules below.
      members: false

::: bothy_sdk.testing.doubles

::: bothy_sdk.testing.fakeserver

::: bothy_sdk.testing.faults

::: bothy_sdk.testing.scenarios

::: bothy_sdk.testing.sdkserver

::: bothy_sdk.testing.fake_agent

::: bothy_sdk.testing.demo_agent

::: bothy_sdk.testing.control

::: bothy_sdk.testing.token_faults

::: bothy_sdk.testing.webhooks
